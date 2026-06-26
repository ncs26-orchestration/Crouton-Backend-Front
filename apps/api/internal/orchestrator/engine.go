// Package orchestrator runs a request's workflow graph: it advances each
// node through the department agents, persists the work, and drives the
// request to completion. It is deliberately small on the outside — start a
// request — and depends on narrow interfaces so it can be tested with an
// in-memory store and a fake agent.
package orchestrator

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/ncs26-orchestration/solution/apps/api/internal/agentclient"
	"github.com/ncs26-orchestration/solution/apps/api/internal/repo"
)

// approvalAgentType marks the executive-approval gate. A node with this agent
// type is a human decision point, not an agent step: the engine parks the
// request there instead of running an agent, and a human resumes it via Approve.
const approvalAgentType = "approval"

// ApprovalDecision is a human's call on the executive gate.
type ApprovalDecision string

const (
	ApprovalApprove ApprovalDecision = "approve"
	ApprovalReject  ApprovalDecision = "reject"
)

var (
	// ErrNotAwaitingApproval means the request is not parked at the gate, so
	// there is nothing to approve or reject.
	ErrNotAwaitingApproval = errors.New("request is not awaiting approval")
	// ErrApprovalNodeMissing means a request is awaiting approval but has no
	// executive-approval node — a malformed graph.
	ErrApprovalNodeMissing = errors.New("approval node not found")
)

// AgentRunner runs one department node. *agentclient.Client satisfies it.
type AgentRunner interface {
	Run(ctx context.Context, rr agentclient.RunRequest) (*agentclient.Decision, error)
}

// Store is the persistence the engine drives. A real implementation wraps the
// request + workflow repos; tests use an in-memory fake.
type Store interface {
	GetRequest(ctx context.Context, requestID string) (*repo.Request, error)
	ListNodesByRequest(ctx context.Context, requestID string) ([]repo.WorkflowNode, error)
	ListEdgesByRequest(ctx context.Context, requestID string) ([]repo.WorkflowEdge, error)
	ListInProgressRequestIDs(ctx context.Context) ([]string, error)
	UpdateNodeStatus(ctx context.Context, nodeID, status, statusText string, progressPercent int) error
	ClearNodeTasks(ctx context.Context, nodeID string) error
	InsertTasks(ctx context.Context, tasks []repo.AgentTask) error
	UpdateRequestProgress(ctx context.Context, requestID, status string, progressPercent int) error
}

// Engine advances requests on background goroutines.
type Engine struct {
	rootCtx   context.Context
	log       *slog.Logger
	store     Store
	agent     AgentRunner
	stepDelay time.Duration
	bus       *Bus

	mu      sync.Mutex
	running map[string]bool
}

// NewEngine builds an engine. rootCtx ties worker goroutines to the server
// lifetime so they stop on shutdown (not to any one HTTP request).
func NewEngine(rootCtx context.Context, log *slog.Logger, store Store, agent AgentRunner, stepDelay time.Duration, bus *Bus) *Engine {
	return &Engine{
		rootCtx:   rootCtx,
		log:       log,
		store:     store,
		agent:     agent,
		stepDelay: stepDelay,
		bus:       bus,
		running:   make(map[string]bool),
	}
}

// Start launches the worker for a request if one isn't already running. It
// returns immediately; the work happens on a background goroutine.
func (e *Engine) Start(requestID string) {
	e.mu.Lock()
	if e.running[requestID] {
		e.mu.Unlock()
		return
	}
	e.running[requestID] = true
	e.mu.Unlock()

	go func() {
		defer func() {
			e.mu.Lock()
			delete(e.running, requestID)
			e.mu.Unlock()
		}()
		if err := e.run(e.rootCtx, requestID); err != nil {
			e.log.Error("orchestrator run", slog.String("request_id", requestID), slog.String("err", err.Error()))
		}
	}()
}

// ResumeInProgress re-starts any request left in_progress by a prior run, so a
// restart (deploy or crash) doesn't strand a request mid-orchestration. The
// run loop is idempotent: it re-derives eligibility from node status, so
// already-completed nodes are skipped and only the unfinished ones run.
func (e *Engine) ResumeInProgress() {
	ids, err := e.store.ListInProgressRequestIDs(e.rootCtx)
	if err != nil {
		e.log.Error("orchestrator resume: list in_progress", slog.String("err", err.Error()))
		return
	}
	for _, id := range ids {
		e.log.Info("orchestrator: resuming in_progress request", slog.String("request_id", id))
		e.Start(id)
	}
}

// run drives one request to completion: repeatedly find eligible nodes (all
// predecessors completed), run each, and persist. It stops when every node is
// completed, when nothing is eligible (a malformed graph), or on cancellation.
func (e *Engine) run(ctx context.Context, requestID string) error {
	req, err := e.store.GetRequest(ctx, requestID)
	if err != nil {
		return fmt.Errorf("get request: %w", err)
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		nodes, err := e.store.ListNodesByRequest(ctx, requestID)
		if err != nil {
			return fmt.Errorf("list nodes: %w", err)
		}
		edges, err := e.store.ListEdgesByRequest(ctx, requestID)
		if err != nil {
			return fmt.Errorf("list edges: %w", err)
		}
		if len(nodes) == 0 {
			return nil
		}

		status := make(map[string]string, len(nodes))
		byID := make(map[string]repo.WorkflowNode, len(nodes))
		completed := 0
		for _, n := range nodes {
			status[n.ID] = n.Status
			byID[n.ID] = n
			if n.Status == "completed" {
				completed++
			}
		}
		if completed == len(nodes) {
			if err := e.store.UpdateRequestProgress(ctx, requestID, "completed", 100); err != nil {
				return fmt.Errorf("update request progress: %w", err)
			}
			e.publishRequestEvent(requestID, "completed", 100)
			return nil
		}

		eligible := eligibleNodes(nodes, edges, status)
		if len(eligible) == 0 {
			// No completed-all and nothing runnable: a cycle or an
			// already-in-flight stage we don't own. Stop rather than spin.
			e.log.Warn("orchestrator: no eligible nodes, stopping", slog.String("request_id", requestID))
			return nil
		}

		// The executive-approval node is a human gate, not an agent step. Run
		// every other eligible node first; if the gate is among them, park the
		// request at awaiting_approval and stop. A human resumes it via Approve.
		var gate *repo.WorkflowNode
		for i := range eligible {
			if eligible[i].AgentType == approvalAgentType {
				node := eligible[i]
				gate = &node
				continue
			}
			if err := e.runNode(ctx, req, eligible[i], byID, edges); err != nil {
				return err
			}
			completed++
			progress := completed * 100 / len(nodes)
			if err := e.store.UpdateRequestProgress(ctx, requestID, "in_progress", progress); err != nil {
				return fmt.Errorf("update request progress: %w", err)
			}
			e.publishRequestEvent(requestID, "in_progress", progress)
		}
		if gate != nil {
			return e.parkForApproval(ctx, requestID, *gate, completed*100/len(nodes))
		}
	}
}

// parkForApproval marks the executive-approval gate in_progress and parks the
// whole request at awaiting_approval, then returns so the worker goroutine
// exits. A human resumes the request through Approve. Restart recovery only
// re-launches in_progress requests, so a parked request correctly keeps waiting
// across a restart instead of auto-advancing.
func (e *Engine) parkForApproval(ctx context.Context, requestID string, gate repo.WorkflowNode, progress int) error {
	const statusText = "Awaiting executive approval."
	if err := e.store.UpdateNodeStatus(ctx, gate.ID, "in_progress", statusText, 50); err != nil {
		return fmt.Errorf("mark approval in_progress: %w", err)
	}
	if err := e.store.UpdateRequestProgress(ctx, requestID, "awaiting_approval", progress); err != nil {
		return fmt.Errorf("park for approval: %w", err)
	}
	e.publishNodeEvent(requestID, "in_progress", gate.ID, gate.Key, 50, statusText, time.Now())
	e.publishRequestEvent(requestID, "awaiting_approval", progress)
	e.log.Info("orchestrator: parked for executive approval",
		slog.String("request_id", requestID), slog.String("node_id", gate.ID))
	return nil
}

// Approve records a human decision on a request parked at the executive gate.
// On approve it completes the gate node and moves the request back to
// in_progress; on reject it stops the request at rejected. It performs only the
// durable state transition — the caller resumes the worker by calling Start
// after a successful approve, the same way CreateRequest launches it. (Keeping
// the goroutine launch out of Approve makes the transition synchronous and
// testable.) justification is the human's written reason and is required by the
// caller; durable audit of the reason arrives with the audit trail (F6), so for
// now it is logged.
func (e *Engine) Approve(ctx context.Context, requestID string, decision ApprovalDecision, justification string) error {
	req, err := e.store.GetRequest(ctx, requestID)
	if err != nil {
		return fmt.Errorf("get request: %w", err)
	}
	if req.Status != "awaiting_approval" {
		return ErrNotAwaitingApproval
	}

	nodes, err := e.store.ListNodesByRequest(ctx, requestID)
	if err != nil {
		return fmt.Errorf("list nodes: %w", err)
	}
	var gate *repo.WorkflowNode
	for i := range nodes {
		if nodes[i].AgentType == approvalAgentType {
			node := nodes[i]
			gate = &node
			break
		}
	}
	if gate == nil {
		return ErrApprovalNodeMissing
	}

	switch decision {
	case ApprovalApprove:
		const statusText = "Approved by the executive."
		if err := e.store.UpdateNodeStatus(ctx, gate.ID, "completed", statusText, 100); err != nil {
			return fmt.Errorf("complete approval node: %w", err)
		}
		// Keep the current progress; the resumed run loop recomputes it as the
		// execution stages complete.
		if err := e.store.UpdateRequestProgress(ctx, requestID, "in_progress", req.Progress); err != nil {
			return fmt.Errorf("resume request: %w", err)
		}
		e.publishNodeEvent(requestID, "completed", gate.ID, gate.Key, 100, statusText, time.Now())
		e.publishRequestEvent(requestID, "in_progress", req.Progress)
		e.log.Info("orchestrator: request approved",
			slog.String("request_id", requestID), slog.String("justification", justification))
		return nil
	case ApprovalReject:
		const statusText = "Rejected by the executive."
		if err := e.store.UpdateNodeStatus(ctx, gate.ID, "completed", statusText, 100); err != nil {
			return fmt.Errorf("close approval node: %w", err)
		}
		if err := e.store.UpdateRequestProgress(ctx, requestID, "rejected", req.Progress); err != nil {
			return fmt.Errorf("reject request: %w", err)
		}
		e.publishNodeEvent(requestID, "completed", gate.ID, gate.Key, 100, statusText, time.Now())
		e.publishRequestEvent(requestID, "rejected", req.Progress)
		e.log.Info("orchestrator: request rejected",
			slog.String("request_id", requestID), slog.String("justification", justification))
		return nil
	default:
		return fmt.Errorf("invalid approval decision %q", decision)
	}
}

// runNode marks a node in_progress, runs its agent (falling back to a
// deterministic decision on error so it never stalls), persists the tasks, and
// marks the node completed with the agent's plain-language status.
func (e *Engine) runNode(ctx context.Context, req *repo.Request, node repo.WorkflowNode, byID map[string]repo.WorkflowNode, edges []repo.WorkflowEdge) error {
	now := time.Now()
	if err := e.store.UpdateNodeStatus(ctx, node.ID, "in_progress", node.Department+" reviewing the request…", 25); err != nil {
		return fmt.Errorf("mark in_progress: %w", err)
	}
	e.publishNodeEvent(req.ID, "in_progress", node.ID, node.Key, 25, node.Department+" reviewing the request…", now)
	e.pace(ctx)

	upstream := upstreamContext(node.ID, byID, edges)
	decision, err := e.agent.Run(ctx, agentclient.RunRequest{
		AgentType: node.AgentType,
		Request: agentclient.IntakeRequestBody{
			Title:       req.Title,
			Description: req.Description,
			Priority:    req.Priority,
		},
		UpstreamContext: upstream,
		OrgContext:      map[string]any{},
	})
	if err != nil {
		e.log.Warn("orchestrator: agent unavailable, using fallback decision",
			slog.String("node_id", node.ID), slog.String("agent_type", node.AgentType), slog.String("err", err.Error()))
		decision = agentclient.DefaultDecision(node.AgentType)
	}

	tasks := make([]repo.AgentTask, 0, len(decision.Tasks))
	for i, t := range decision.Tasks {
		started, completedAt := now, now
		tasks = append(tasks, repo.AgentTask{
			ID:          "at_" + shortID(),
			NodeID:      node.ID,
			Title:       t.Title,
			Status:      "completed",
			Ordinal:     i,
			StartedAt:   &started,
			CompletedAt: &completedAt,
		})
	}
	// Clear any tasks from a prior (interrupted) run of this node so a resume
	// doesn't double up.
	if err := e.store.ClearNodeTasks(ctx, node.ID); err != nil {
		return fmt.Errorf("clear tasks: %w", err)
	}
	if err := e.store.InsertTasks(ctx, tasks); err != nil {
		return fmt.Errorf("insert tasks: %w", err)
	}

	statusText := decision.StatusText
	if statusText == "" {
		statusText = node.Name + " complete"
	}
	if err := e.store.UpdateNodeStatus(ctx, node.ID, "completed", statusText, 100); err != nil {
		return fmt.Errorf("mark completed: %w", err)
	}
	e.publishNodeEvent(req.ID, "completed", node.ID, node.Key, 100, statusText, now)
	return nil
}

// pace waits stepDelay (so progression is watchable) but returns early if the
// context is cancelled.
func (e *Engine) pace(ctx context.Context) {
	if e.stepDelay <= 0 {
		return
	}
	t := time.NewTimer(e.stepDelay)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

// eligibleNodes returns the runnable nodes whose every predecessor is
// completed. Both pending and in_progress nodes are runnable: an in_progress
// node is one a prior run started but didn't finish (a restart), so it is
// re-run. runNode clears the node's tasks first, keeping re-runs idempotent.
func eligibleNodes(nodes []repo.WorkflowNode, edges []repo.WorkflowEdge, status map[string]string) []repo.WorkflowNode {
	preds := make(map[string][]string, len(nodes))
	for _, e := range edges {
		preds[e.TargetNodeID] = append(preds[e.TargetNodeID], e.SourceNodeID)
	}
	var out []repo.WorkflowNode
	for _, n := range nodes {
		if n.Status != "pending" && n.Status != "in_progress" {
			continue
		}
		ready := true
		for _, p := range preds[n.ID] {
			if status[p] != "completed" {
				ready = false
				break
			}
		}
		if ready {
			out = append(out, n)
		}
	}
	return out
}

// upstreamContext gathers completed predecessor summaries for a node.
func upstreamContext(nodeID string, byID map[string]repo.WorkflowNode, edges []repo.WorkflowEdge) []agentclient.UpstreamItem {
	var out []agentclient.UpstreamItem
	for _, e := range edges {
		if e.TargetNodeID != nodeID {
			continue
		}
		src, ok := byID[e.SourceNodeID]
		if !ok || src.Status != "completed" {
			continue
		}
		out = append(out, agentclient.UpstreamItem{
			Key:        src.Key,
			Department: src.Department,
			Summary:    src.StatusText,
		})
	}
	return out
}

func (e *Engine) publishNodeEvent(requestID, status, nodeID, key string, progress int, statusText string, at time.Time) {
	if e.bus == nil {
		return
	}
	e.bus.Publish(Event{
		Type:       "node_status",
		RequestID:  requestID,
		NodeID:     nodeID,
		Key:        key,
		Status:     status,
		Progress:   progress,
		StatusText: statusText,
		At:         at,
	})
}

func (e *Engine) publishRequestEvent(requestID, status string, progress int) {
	if e.bus == nil {
		return
	}
	e.bus.Publish(Event{
		Type:      "request_status",
		RequestID: requestID,
		Status:    status,
		Progress:  progress,
		At:        time.Now(),
	})
}

func shortID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(b)
}
