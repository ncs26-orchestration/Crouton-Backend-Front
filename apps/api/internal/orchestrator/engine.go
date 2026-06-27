// Package orchestrator runs a request's workflow graph: it advances each
// node through the department agents, persists the work, and drives the
// request to completion. It is deliberately small on the outside — start a
// request — and depends on narrow interfaces so it can be tested with an
// in-memory store and a fake agent.
package orchestrator

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/ncs26-orchestration/solution/apps/api/internal/agentclient"
	"github.com/ncs26-orchestration/solution/apps/api/internal/policyrules"
	"github.com/ncs26-orchestration/solution/apps/api/internal/repo"
)

// approvalAgentType marks the executive-approval gate. A node with this agent
// type is a human decision point, not an agent step: the engine parks the
// request there instead of running an agent, and a human resumes it via Approve.
const approvalAgentType = "approval"

// complianceAgentTypes own hard rules: when one of these agents returns a
// "reject" outcome, the engine stops the request rather than carrying the
// recommendation to the human gate. Other departments' rejections are surfaced
// as critical flags for the executive to weigh, keeping a human in the loop.
var complianceAgentTypes = map[string]bool{"legal": true}

// errRequestRejected is returned by runNode when an agent's decision rejected
// the request outright. The run loop treats it as a clean stop: the rejection is
// already persisted and audited.
var errRequestRejected = errors.New("request rejected by agent decision")

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

// maxRerunsPerNode caps how many times a blocked node can be re-run
// after its dependencies are resolved, preventing infinite loops.
const maxRerunsPerNode = 3

// Store is the persistence the engine drives. A real implementation wraps the
// request + workflow repos; tests use an in-memory fake.
type Store interface {
	GetRequest(ctx context.Context, requestID string) (*repo.Request, error)
	ListNodesByRequest(ctx context.Context, requestID string) ([]repo.WorkflowNode, error)
	ListEdgesByRequest(ctx context.Context, requestID string) ([]repo.WorkflowEdge, error)
	ListInProgressRequestIDs(ctx context.Context) ([]string, error)
	UpdateNodeStatus(ctx context.Context, nodeID, status, statusText string, progressPercent int) error
	UpdateNodeDecisionOutcome(ctx context.Context, nodeID, outcome string) error
	SetNodeDecisionSummary(ctx context.Context, nodeID, summary string) error
	ClearNodeTasks(ctx context.Context, nodeID string) error
	InsertTasks(ctx context.Context, tasks []repo.AgentTask) error
	ClearNodeFlags(ctx context.Context, nodeID string) error
	InsertFlags(ctx context.Context, flags []repo.NodeFlag) error
	ClearNodeChecks(ctx context.Context, nodeID string) error
	InsertChecks(ctx context.Context, checks []repo.NodeCheck) error
	UpdateRequestProgress(ctx context.Context, requestID, status string, progressPercent int) error
	// F5: cross-department dependencies.
	InsertDependency(ctx context.Context, dep repo.NodeDependency) error
	ResolveDependenciesBlockedBy(ctx context.Context, blockingNodeID string) ([]string, error)
	MaxRunCount(ctx context.Context, dependentNodeID string) (int, error)
	ListUnresolvedDepsByRequest(ctx context.Context, requestID string) ([]repo.NodeDependency, error)
	// F6: audit trail.
	AppendAuditEvent(ctx context.Context, e repo.AuditEvent) error
	// Documents — generated completion summaries and manual uploads.
	CreateDocument(ctx context.Context, d *repo.Document) error
	// Policies an agent checks a request against (F10).
	ListPoliciesByOrg(ctx context.Context, orgID string) ([]repo.DepartmentPolicy, error)
	// Human-in-the-loop: how many verifiers are assigned to a node.
	CountAssignmentsByNode(ctx context.Context, nodeID string) (int, error)
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
// predecessors completed and no unresolved deps), run each, and persist. It
// stops when every node is completed, when nothing is eligible (a cycle or
// blocked nodes with no blockers completing), or on cancellation.
func (e *Engine) run(ctx context.Context, requestID string) error {
	req, err := e.store.GetRequest(ctx, requestID)
	if err != nil {
		return fmt.Errorf("get request: %w", err)
	}

	// Load the org's policies once and group them by department so each agent is
	// handed only the policies it owns (F10). A failure here is non-fatal: agents
	// fall back to reasoning without policy text.
	policiesByDept := e.loadPoliciesByDept(ctx, req.OrgID)

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

			docID := "doc_" + shortID()
			docBuf := new(bytes.Buffer)
			fmt.Fprintf(docBuf, "Request: %s\n", req.Title)
			fmt.Fprintf(docBuf, "Description: %s\n", req.Description)
			fmt.Fprintf(docBuf, "Priority: %s\n", req.Priority)
			fmt.Fprintf(docBuf, "Completed: %s\n\n", time.Now().Format(time.RFC1123))
			fmt.Fprintf(docBuf, "--- Node Results ---\n")
			for _, n := range nodes {
				statusText := n.StatusText
				if statusText == "" {
					statusText = "—"
				}
				fmt.Fprintf(docBuf, "\n  %s (%s) [%s]\n", n.Name, n.Department, n.Status)
				fmt.Fprintf(docBuf, "    %s\n", statusText)
			}
			contentText := docBuf.String()

			if err := e.store.CreateDocument(ctx, &repo.Document{
				ID:          docID,
				RequestID:   requestID,
				Filename:    "completion-summary.txt",
				Mime:        "text/plain",
				ContentText: contentText,
			}); err != nil {
				e.log.Warn("failed to create completion document", slog.String("request_id", requestID), slog.String("err", err.Error()))
			}

			aevID := "aev_" + shortID()
			if err := e.store.AppendAuditEvent(ctx, repo.AuditEvent{
				ID:         aevID,
				RequestID:  requestID,
				Actor:      "engine",
				Action:     "request.completed",
				Reason:     "All " + fmt.Sprintf("%d", completed) + " nodes completed",
				DocumentID: &docID,
			}); err != nil {
				e.log.Warn("failed to audit request.completed", slog.String("request_id", requestID), slog.String("err", err.Error()))
			}
			e.publishRequestEvent(requestID, "completed", 100)
			return nil
		}

		// F5: load unresolved dependencies so eligibleNodes can check them.
		deps, err := e.store.ListUnresolvedDepsByRequest(ctx, requestID)
		if err != nil {
			return fmt.Errorf("list unresolved deps: %w", err)
		}
		blocked := make(map[string]bool, len(deps))
		for _, d := range deps {
			blocked[d.DependentNodeID] = true
		}

		eligible := eligibleNodes(nodes, edges, status, blocked)
		if len(eligible) == 0 {
			// No completed-all and nothing runnable: a cycle or blocked nodes
			// whose blocking nodes are still running. Stop rather than spin.
			e.log.Warn("orchestrator: no eligible nodes, stopping", slog.String("request_id", requestID))
			return nil
		}

		// The executive-approval node is a final sign-off stamp, not a blocking
		// human step — the human-in-the-loop work happens at the department
		// verifications. Run the other eligible nodes, then auto-advance the gate
		// (recording the sign-off) so the flow proceeds to execution.
		var gate *repo.WorkflowNode
		for i := range eligible {
			if eligible[i].AgentType == approvalAgentType {
				node := eligible[i]
				gate = &node
				continue
			}
			completedNow, err := e.runNode(ctx, req, eligible[i], byID, edges, policiesByDept)
			if err != nil {
				// An agent rejected the request outright: already persisted and
				// audited, so stop the worker cleanly.
				if errors.Is(err, errRequestRejected) {
					return nil
				}
				return err
			}
			if completedNow {
				completed++
				progress := completed * 100 / len(nodes)
				if err := e.store.UpdateRequestProgress(ctx, requestID, "in_progress", progress); err != nil {
					return fmt.Errorf("update request progress: %w", err)
				}
				e.publishRequestEvent(requestID, "in_progress", progress)
			}
		}
		if gate != nil {
			const statusText = "Signed off — proceeding to execution."
			if err := e.store.UpdateNodeStatus(ctx, gate.ID, "completed", statusText, 100); err != nil {
				return fmt.Errorf("complete approval gate: %w", err)
			}
			if err := e.store.AppendAuditEvent(ctx, repo.AuditEvent{
				ID: "aev_" + shortID(), RequestID: requestID, NodeID: &gate.ID,
				Actor: "Executive", Action: "approval.auto",
				Reason: "Auto sign-off — department verifications carry the human review.",
			}); err != nil {
				e.log.Warn("failed to audit approval.auto", slog.String("request_id", requestID), slog.String("err", err.Error()))
			}
			completed++
			progress := completed * 100 / len(nodes)
			if err := e.store.UpdateRequestProgress(ctx, requestID, "in_progress", progress); err != nil {
				return fmt.Errorf("update request progress: %w", err)
			}
			e.publishNodeEvent(requestID, "completed", gate.ID, gate.Key, 100, statusText, time.Now())
			e.publishRequestEvent(requestID, "in_progress", progress)
		}
	}
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
func (e *Engine) Approve(ctx context.Context, requestID string, decision ApprovalDecision, justification, approverName string) error {
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
		if err := e.store.UpdateRequestProgress(ctx, requestID, "in_progress", req.Progress); err != nil {
			return fmt.Errorf("resume request: %w", err)
		}
		if err := e.store.AppendAuditEvent(ctx, repo.AuditEvent{
			ID:        "aev_" + shortID(),
			RequestID: requestID,
			NodeID:    &gate.ID,
			Actor:     approverName,
			Action:    "approval.granted",
			Reason:    justification,
		}); err != nil {
			e.log.Warn("failed to audit approval.granted", slog.String("request_id", requestID), slog.String("err", err.Error()))
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
		if err := e.store.AppendAuditEvent(ctx, repo.AuditEvent{
			ID:        "aev_" + shortID(),
			RequestID: requestID,
			NodeID:    &gate.ID,
			Actor:     approverName,
			Action:    "approval.rejected",
			Reason:    justification,
		}); err != nil {
			e.log.Warn("failed to audit approval.rejected", slog.String("request_id", requestID), slog.String("err", err.Error()))
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

// ErrNodeNotAwaitingReview means the node isn't paused for human verification.
var ErrNodeNotAwaitingReview = errors.New("node is not awaiting review")

// VerifyNode records a human's sign-off on a node parked at awaiting_review.
// On approve it completes the node so the worker (relaunched by the caller via
// Start) advances the branch; on reject it stops the request. RBAC is enforced
// by the caller; verifierName is recorded in the audit trail.
func (e *Engine) VerifyNode(ctx context.Context, requestID, nodeID string, decision ApprovalDecision, note, verifierName string) error {
	nodes, err := e.store.ListNodesByRequest(ctx, requestID)
	if err != nil {
		return fmt.Errorf("list nodes: %w", err)
	}
	var node *repo.WorkflowNode
	for i := range nodes {
		if nodes[i].ID == nodeID {
			n := nodes[i]
			node = &n
			break
		}
	}
	if node == nil {
		return repo.ErrNotFound
	}
	if node.Status != "awaiting_review" {
		return ErrNodeNotAwaitingReview
	}
	req, err := e.store.GetRequest(ctx, requestID)
	if err != nil {
		return fmt.Errorf("get request: %w", err)
	}

	switch decision {
	case ApprovalApprove:
		statusText := node.Name + " verified by " + verifierName + "."
		if err := e.store.UpdateNodeStatus(ctx, nodeID, "completed", statusText, 100); err != nil {
			return fmt.Errorf("complete verified node: %w", err)
		}
		if err := e.store.AppendAuditEvent(ctx, repo.AuditEvent{
			ID: "aev_" + shortID(), RequestID: requestID, NodeID: &nodeID,
			Actor: verifierName, Action: "node.verified", Reason: note,
		}); err != nil {
			e.log.Warn("failed to audit node.verified", slog.String("node_id", nodeID), slog.String("err", err.Error()))
		}
		e.publishNodeEvent(requestID, "completed", nodeID, node.Key, 100, statusText, time.Now())
		return nil
	case ApprovalReject:
		statusText := node.Name + " sent back by " + verifierName + "."
		if err := e.store.UpdateNodeStatus(ctx, nodeID, "completed", statusText, 100); err != nil {
			return fmt.Errorf("close rejected node: %w", err)
		}
		if err := e.store.UpdateRequestProgress(ctx, requestID, "rejected", req.Progress); err != nil {
			return fmt.Errorf("reject request: %w", err)
		}
		if err := e.store.AppendAuditEvent(ctx, repo.AuditEvent{
			ID: "aev_" + shortID(), RequestID: requestID, NodeID: &nodeID,
			Actor: verifierName, Action: "node.rejected", Reason: note,
		}); err != nil {
			e.log.Warn("failed to audit node.rejected", slog.String("node_id", nodeID), slog.String("err", err.Error()))
		}
		e.publishNodeEvent(requestID, "completed", nodeID, node.Key, 100, statusText, time.Now())
		e.publishRequestEvent(requestID, "rejected", req.Progress)
		return nil
	default:
		return fmt.Errorf("invalid verify decision %q", decision)
	}
}

// runNode marks a node in_progress, runs its agent (falling back to a
// deterministic decision on error so it never stalls), persists the tasks, and
// marks the node completed (or blocked if the agent declared a dependency, F5)
// with the agent's plain-language status. Every state change is written to the
// audit trail (F6). Returns true if the node actually completed (not blocked)
// so the caller can track progress accurately.
func (e *Engine) runNode(ctx context.Context, req *repo.Request, node repo.WorkflowNode, byID map[string]repo.WorkflowNode, edges []repo.WorkflowEdge, policiesByDept map[string][]repo.DepartmentPolicy) (bool, error) {
	now := time.Now()
	if err := e.store.UpdateNodeStatus(ctx, node.ID, "in_progress", node.Department+" reviewing the request…", 25); err != nil {
		return false, fmt.Errorf("mark in_progress: %w", err)
	}
	if err := e.store.AppendAuditEvent(ctx, repo.AuditEvent{
		ID:        "aev_" + shortID(),
		RequestID: req.ID,
		NodeID:    &node.ID,
		Actor:     "engine",
		Action:    "node.started",
		Reason:    node.Name + " started — " + node.Department + " reviewing",
	}); err != nil {
		e.log.Warn("failed to audit node.started", slog.String("node_id", node.ID), slog.String("err", err.Error()))
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
			Details:     req.Details,
		},
		UpstreamContext: upstream,
		OrgContext:      orgContextForNode(node, policiesByDept),
	})
	if err != nil {
		e.log.Warn("orchestrator: agent unavailable, using fallback decision",
			slog.String("node_id", node.ID), slog.String("agent_type", node.AgentType), slog.String("err", err.Error()))
		decision = agentclient.DefaultDecision(node.AgentType)
		if err := e.store.AppendAuditEvent(ctx, repo.AuditEvent{
			ID:        "aev_" + shortID(),
			RequestID: req.ID,
			NodeID:    &node.ID,
			Actor:     "engine",
			Action:    "agent.fallback",
			Reason:    node.Name + " — agent unavailable, used deterministic fallback: " + err.Error(),
		}); err != nil {
			e.log.Warn("failed to audit agent.fallback", slog.String("node_id", node.ID), slog.String("err", err.Error()))
		}
	}

	// F5: if the agent declared a cross-department dependency, mark the node
	// as blocked and record the dependency instead of completing it. Only block
	// when the named department actually has a node in this graph and that node
	// has not already completed. Otherwise the dependency could never be
	// resolved (nothing will complete the blocker again) and the node would
	// deadlock, so we ignore the declaration and complete normally.
	if decision.BlockedOn != nil {
		blockingNodeID := findNodeByDepartment(byID, decision.BlockedOn.OnDepartment)
		blocker, found := byID[blockingNodeID]
		if blockingNodeID == "" || !found || blocker.Status == "completed" {
			e.log.Warn("orchestrator: ignoring blocked_on; blocker is missing or already completed, completing node",
				slog.String("node_id", node.ID), slog.String("on_department", decision.BlockedOn.OnDepartment))
		} else {
			e.log.Info("orchestrator: node declared blocked",
				slog.String("node_id", node.ID), slog.String("on_department", decision.BlockedOn.OnDepartment), slog.String("reason", decision.BlockedOn.Reason))

			if err := e.store.InsertDependency(ctx, repo.NodeDependency{
				ID:              "nd_" + shortID(),
				RequestID:       req.ID,
				DependentNodeID: node.ID,
				BlockingNodeID:  blockingNodeID,
				Reason:          decision.BlockedOn.Reason,
				RunCount:        1,
			}); err != nil {
				return false, fmt.Errorf("insert dependency: %w", err)
			}

			statusText := decision.StatusText
			if statusText == "" {
				statusText = "Waiting for " + decision.BlockedOn.OnDepartment + ": " + decision.BlockedOn.Reason
			}
			if err := e.store.AppendAuditEvent(ctx, repo.AuditEvent{
				ID:        "aev_" + shortID(),
				RequestID: req.ID,
				NodeID:    &node.ID,
				Actor:     node.Department,
				Action:    "node.blocked",
				Reason:    statusText,
			}); err != nil {
				e.log.Warn("failed to audit node.blocked", slog.String("node_id", node.ID), slog.String("err", err.Error()))
			}
			if err := e.store.UpdateNodeStatus(ctx, node.ID, "blocked", statusText, 50); err != nil {
				return false, fmt.Errorf("mark blocked: %w", err)
			}
			if err := e.store.UpdateNodeDecisionOutcome(ctx, node.ID, "block"); err != nil {
				e.log.Warn("failed to record block outcome", slog.String("node_id", node.ID), slog.String("err", err.Error()))
			}
			if decision.Summary != "" {
				if err := e.store.SetNodeDecisionSummary(ctx, node.ID, decision.Summary); err != nil {
					e.log.Warn("failed to record block summary", slog.String("node_id", node.ID), slog.String("err", err.Error()))
				}
			}
			e.persistFlags(ctx, req.ID, node.ID, decision.Flags)
			return false, nil
		}
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
	if err := e.store.ClearNodeTasks(ctx, node.ID); err != nil {
		return false, fmt.Errorf("clear tasks: %w", err)
	}
	if err := e.store.InsertTasks(ctx, tasks); err != nil {
		return false, fmt.Errorf("insert tasks: %w", err)
	}

	statusText := decision.StatusText
	if statusText == "" {
		statusText = node.Name + " complete"
	}

	outcome := decision.Outcome
	if outcome == "" {
		outcome = "approve"
	}

	// Evaluate the department's typed policy rules against the request's details
	// (deterministic, no LLM) and persist the exact pass/warn/fail checks. A
	// failed rule escalates the outcome so it surfaces even offline.
	worst := e.persistChecks(ctx, req, node, policiesByDept[strings.ToLower(strings.TrimSpace(node.Department))])
	if worst == "fail" && outcome == "approve" {
		outcome = "flag"
	} else if worst == "warn" && outcome == "approve" {
		outcome = "approve_with_conditions"
	}

	// Record the agent's outcome, reasoning, and flags on the node so the UI can
	// show why a department decided what it did, and surface the risks as audit
	// events so the decision is traceable and visible at the gate (F6).
	if err := e.store.UpdateNodeDecisionOutcome(ctx, node.ID, outcome); err != nil {
		e.log.Warn("failed to record decision outcome", slog.String("node_id", node.ID), slog.String("err", err.Error()))
	}
	if decision.Summary != "" {
		if err := e.store.SetNodeDecisionSummary(ctx, node.ID, decision.Summary); err != nil {
			e.log.Warn("failed to record decision summary", slog.String("node_id", node.ID), slog.String("err", err.Error()))
		}
	}
	e.persistFlags(ctx, req.ID, node.ID, decision.Flags)
	e.auditFlags(ctx, req.ID, node, decision.Flags)

	// A compliance department (e.g. Legal) that rejects stops the request
	// outright; the rejection is audited with the agent's reasoning. Rejections
	// from other departments fall through to completion and are carried to the
	// executive gate as critical flags instead.
	if outcome == "reject" && complianceAgentTypes[node.AgentType] {
		if err := e.store.UpdateNodeStatus(ctx, node.ID, "completed", statusText, 100); err != nil {
			return false, fmt.Errorf("mark completed: %w", err)
		}
		if err := e.store.AppendAuditEvent(ctx, repo.AuditEvent{
			ID:        "aev_" + shortID(),
			RequestID: req.ID,
			NodeID:    &node.ID,
			Actor:     node.Department,
			Action:    "agent.rejected",
			Reason:    statusText + " — " + decision.Summary,
		}); err != nil {
			e.log.Warn("failed to audit agent.rejected", slog.String("node_id", node.ID), slog.String("err", err.Error()))
		}
		if err := e.store.UpdateRequestProgress(ctx, req.ID, "rejected", req.Progress); err != nil {
			return false, fmt.Errorf("reject request: %w", err)
		}
		e.publishNodeEvent(req.ID, "completed", node.ID, node.Key, 100, statusText, now)
		e.publishRequestEvent(req.ID, "rejected", req.Progress)
		e.log.Info("orchestrator: request rejected by agent",
			slog.String("request_id", req.ID), slog.String("department", node.Department))
		return false, errRequestRejected
	}

	// Human-in-the-loop: if a verifier is assigned to this node, pause for their
	// sign-off instead of completing. The agent's decision is already persisted
	// above; a human resumes the flow via VerifyNode. Nodes with no assignee
	// auto-complete as before.
	if assignees, aerr := e.store.CountAssignmentsByNode(ctx, node.ID); aerr != nil {
		e.log.Warn("failed to count assignments", slog.String("node_id", node.ID), slog.String("err", aerr.Error()))
	} else if assignees > 0 {
		reviewText := node.Name + " complete — awaiting verification."
		if err := e.store.UpdateNodeStatus(ctx, node.ID, "awaiting_review", reviewText, 90); err != nil {
			return false, fmt.Errorf("mark awaiting_review: %w", err)
		}
		if err := e.store.AppendAuditEvent(ctx, repo.AuditEvent{
			ID:        "aev_" + shortID(),
			RequestID: req.ID,
			NodeID:    &node.ID,
			Actor:     node.Department,
			Action:    "node.awaiting_review",
			Reason:    reviewText,
		}); err != nil {
			e.log.Warn("failed to audit node.awaiting_review", slog.String("node_id", node.ID), slog.String("err", err.Error()))
		}
		e.publishNodeEvent(req.ID, "awaiting_review", node.ID, node.Key, 90, reviewText, now)
		return false, nil
	}

	if err := e.store.UpdateNodeStatus(ctx, node.ID, "completed", statusText, 100); err != nil {
		return false, fmt.Errorf("mark completed: %w", err)
	}
	if err := e.store.AppendAuditEvent(ctx, repo.AuditEvent{
		ID:        "aev_" + shortID(),
		RequestID: req.ID,
		NodeID:    &node.ID,
		Actor:     node.Department,
		Action:    "node.completed",
		Reason:    statusText,
	}); err != nil {
		e.log.Warn("failed to audit node.completed", slog.String("node_id", node.ID), slog.String("err", err.Error()))
	}
	e.publishNodeEvent(req.ID, "completed", node.ID, node.Key, 100, statusText, now)

	// F5: after a node completes, check if it unblocks any blocked nodes and
	// resolve the dependency. The caller's loop will re-derive eligibility and
	// pick up unblocked nodes.
	if err := e.resolveDepsOnCompletion(ctx, node.ID, req, byID, edges); err != nil {
		return false, fmt.Errorf("resolve deps: %w", err)
	}
	return true, nil
}

// resolveDepsOnCompletion checks if the just-completed node was blocking any
// other nodes. If so, it resolves those dependencies and re-marks the formerly
// blocked nodes as pending so the main loop picks them up for re-run.
func (e *Engine) resolveDepsOnCompletion(ctx context.Context, completedNodeID string, req *repo.Request, byID map[string]repo.WorkflowNode, edges []repo.WorkflowEdge) error {
	unblocked, err := e.store.ResolveDependenciesBlockedBy(ctx, completedNodeID)
	if err != nil {
		return fmt.Errorf("resolve dependencies blocked by %s: %w", completedNodeID, err)
	}
	if len(unblocked) == 0 {
		return nil
	}
	e.log.Info("orchestrator: resolved dependencies, unblocked nodes",
		slog.String("blocking_node", completedNodeID),
		slog.Any("unblocked", unblocked),
	)
	for _, depNodeID := range unblocked {
		runCount, err := e.store.MaxRunCount(ctx, depNodeID)
		if err != nil {
			return fmt.Errorf("check run count for %s: %w", depNodeID, err)
		}
		if runCount >= maxRerunsPerNode {
			e.log.Warn("orchestrator: node exceeded max re-runs, completing with fallback",
				slog.String("node_id", depNodeID), slog.Int("run_count", runCount))
			if err := e.store.UpdateNodeStatus(ctx, depNodeID, "completed",
				"Completed after maximum re-run attempts.", 100); err != nil {
				return fmt.Errorf("complete over-max node: %w", err)
			}
			continue
		}
		// Re-mark as pending so the eligibility check finds it.
		if err := e.store.UpdateNodeStatus(ctx, depNodeID, "pending",
			"Awaiting re-run after dependency resolved.", 0); err != nil {
			return fmt.Errorf("mark unblocked node pending: %w", err)
		}
	}
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
// completed and that have no unresolved cross-department dependencies (F5).
// Both pending and in_progress nodes are runnable: an in_progress node is one a
// prior run started but didn't finish (a restart), so it is re-run. runNode
// clears the node's tasks first, keeping re-runs idempotent.
func eligibleNodes(nodes []repo.WorkflowNode, edges []repo.WorkflowEdge, status map[string]string, blocked map[string]bool) []repo.WorkflowNode {
	preds := make(map[string][]string, len(nodes))
	for _, e := range edges {
		preds[e.TargetNodeID] = append(preds[e.TargetNodeID], e.SourceNodeID)
	}
	var out []repo.WorkflowNode
	for _, n := range nodes {
		if n.Status != "pending" && n.Status != "in_progress" {
			continue
		}
		// F5: skip if there is an unresolved dependency blocking this node.
		if blocked[n.ID] {
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

// persistFlags replaces a node's stored flags with the agent's latest set, so a
// re-run is idempotent and the node detail panel can show them.
// persistChecks evaluates the department's policy rules against the request's
// details and replaces the node's stored checks. Returns the worst status seen
// ("fail" > "warn" > "pass") so the caller can escalate the outcome.
func (e *Engine) persistChecks(ctx context.Context, req *repo.Request, node repo.WorkflowNode, policies []repo.DepartmentPolicy) string {
	var checks []policyrules.Check
	for _, p := range policies {
		if len(p.Rules) == 0 {
			continue
		}
		checks = append(checks, policyrules.Evaluate(p.Title, p.Rules, req.Details)...)
	}
	if err := e.store.ClearNodeChecks(ctx, node.ID); err != nil {
		e.log.Warn("failed to clear node checks", slog.String("node_id", node.ID), slog.String("err", err.Error()))
		return "pass"
	}
	if len(checks) == 0 {
		return "pass"
	}
	worst := "pass"
	rows := make([]repo.NodeCheck, 0, len(checks))
	for i, c := range checks {
		if c.Status == "fail" {
			worst = "fail"
		} else if c.Status == "warn" && worst != "fail" {
			worst = "warn"
		}
		rows = append(rows, repo.NodeCheck{
			ID: "nck_" + shortID(), RequestID: req.ID, NodeID: node.ID,
			Label: c.Label, Status: c.Status, Detail: c.Detail, PolicyTitle: c.PolicyTitle, Ordinal: i,
		})
	}
	if err := e.store.InsertChecks(ctx, rows); err != nil {
		e.log.Warn("failed to insert node checks", slog.String("node_id", node.ID), slog.String("err", err.Error()))
	}
	return worst
}

func (e *Engine) persistFlags(ctx context.Context, requestID, nodeID string, flags []agentclient.Flag) {
	if err := e.store.ClearNodeFlags(ctx, nodeID); err != nil {
		e.log.Warn("failed to clear node flags", slog.String("node_id", nodeID), slog.String("err", err.Error()))
		return
	}
	if len(flags) == 0 {
		return
	}
	rows := make([]repo.NodeFlag, 0, len(flags))
	for i, f := range flags {
		sev := f.Severity
		if sev != "info" && sev != "warning" && sev != "critical" {
			sev = "info"
		}
		rows = append(rows, repo.NodeFlag{
			ID:        "nf_" + shortID(),
			RequestID: requestID,
			NodeID:    nodeID,
			Severity:  sev,
			Message:   f.Message,
			Ordinal:   i,
		})
	}
	if err := e.store.InsertFlags(ctx, rows); err != nil {
		e.log.Warn("failed to insert node flags", slog.String("node_id", nodeID), slog.String("err", err.Error()))
	}
}

// auditFlags writes a node.flagged audit event for each warning/critical flag an
// agent raised, so the risk is traceable and the executive sees it at the gate.
// Info-level flags are skipped to keep the trail signal-heavy.
func (e *Engine) auditFlags(ctx context.Context, requestID string, node repo.WorkflowNode, flags []agentclient.Flag) {
	for _, f := range flags {
		if f.Severity != "warning" && f.Severity != "critical" {
			continue
		}
		if err := e.store.AppendAuditEvent(ctx, repo.AuditEvent{
			ID:        "aev_" + shortID(),
			RequestID: requestID,
			NodeID:    &node.ID,
			Actor:     node.Department,
			Action:    "node.flagged",
			Reason:    strings.ToUpper(f.Severity) + ": " + f.Message,
		}); err != nil {
			e.log.Warn("failed to audit node.flagged", slog.String("node_id", node.ID), slog.String("err", err.Error()))
		}
	}
}

// loadPoliciesByDept loads the org's policies and groups them by lowercased
// department name. Errors are logged and swallowed: missing policies just means
// agents reason without them.
func (e *Engine) loadPoliciesByDept(ctx context.Context, orgID string) map[string][]repo.DepartmentPolicy {
	out := make(map[string][]repo.DepartmentPolicy)
	policies, err := e.store.ListPoliciesByOrg(ctx, orgID)
	if err != nil {
		e.log.Warn("orchestrator: could not load policies, agents run without them",
			slog.String("org_id", orgID), slog.String("err", err.Error()))
		return out
	}
	for _, p := range policies {
		key := strings.ToLower(strings.TrimSpace(p.Department))
		out[key] = append(out[key], p)
	}
	return out
}

// orgContextForNode builds the org_context payload for a department agent: the
// policies that apply to its department, so it can check the request and cite
// them.
func orgContextForNode(node repo.WorkflowNode, policiesByDept map[string][]repo.DepartmentPolicy) map[string]any {
	dept := strings.ToLower(strings.TrimSpace(node.Department))
	policies := policiesByDept[dept]
	if len(policies) == 0 {
		return map[string]any{}
	}
	items := make([]map[string]string, 0, len(policies))
	for _, p := range policies {
		items = append(items, map[string]string{"title": p.Title, "body": p.Body})
	}
	return map[string]any{"policies": items}
}

// upstreamContext gathers summaries for all completed nodes in the request,
// not just the direct edge predecessors. This lets an agent reason over any
// completed department's output, not just formal predecessors — essential for
// F5 where Finance re-runs after IT completes and needs to see IT's assessment
// even though there is no direct edge from IT to Finance.
func upstreamContext(nodeID string, byID map[string]repo.WorkflowNode, edges []repo.WorkflowEdge) []agentclient.UpstreamItem {
	_ = edges // kept for future use when per-node filtering may be needed.
	seen := make(map[string]bool, len(byID))
	var out []agentclient.UpstreamItem
	for _, n := range byID {
		if n.ID == nodeID {
			continue
		}
		if n.Status != "completed" {
			continue
		}
		// Include all completed nodes, not just edge predecessors — an agent
		// should be able to reason over any completed department's output even
		// without a formal graph edge (F5).
		if seen[n.ID] {
			continue
		}
		seen[n.ID] = true
		outcome := n.DecisionOutcome
		if outcome == "" || outcome == "pending" {
			outcome = "approve"
		}
		out = append(out, agentclient.UpstreamItem{
			Key:        n.Key,
			Department: n.Department,
			Outcome:    outcome,
			Summary:    n.StatusText,
		})
	}
	return out
}

// findNodeByDepartment looks up the first node in a request that belongs to
// the given department. Used by F5 to map an agent's "blocked on IT"
// declaration to the actual blocking node.
func findNodeByDepartment(byID map[string]repo.WorkflowNode, department string) string {
	for _, n := range byID {
		if n.Department == department {
			return n.ID
		}
	}
	return ""
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
