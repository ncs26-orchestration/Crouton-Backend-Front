package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/ncs26-orchestration/solution/apps/api/internal/agentclient"
	"github.com/ncs26-orchestration/solution/apps/api/internal/repo"
)

// fakeStore is an in-memory Store for driving the engine without a DB.
type fakeStore struct {
	req         *repo.Request
	nodes       []*repo.WorkflowNode
	edges       []repo.WorkflowEdge
	tasks       []repo.AgentTask
	auditEvents []repo.AuditEvent
	reqStatus   string
	reqProgress int
}

func (s *fakeStore) GetRequest(_ context.Context, _ string) (*repo.Request, error) {
	return s.req, nil
}

func (s *fakeStore) byID(id string) *repo.WorkflowNode {
	for _, n := range s.nodes {
		if n.ID == id {
			return n
		}
	}
	return nil
}

func (s *fakeStore) ListNodesByRequest(_ context.Context, _ string) ([]repo.WorkflowNode, error) {
	out := make([]repo.WorkflowNode, 0, len(s.nodes))
	for _, n := range s.nodes {
		out = append(out, *n)
	}
	return out, nil
}

func (s *fakeStore) ListEdgesByRequest(_ context.Context, _ string) ([]repo.WorkflowEdge, error) {
	return s.edges, nil
}

func (s *fakeStore) UpdateNodeStatus(_ context.Context, nodeID, status, statusText string, progress int) error {
	for _, n := range s.nodes {
		if n.ID == nodeID {
			n.Status = status
			n.StatusText = statusText
			n.ProgressPercent = progress
			return nil
		}
	}
	return repo.ErrNotFound
}

func (s *fakeStore) ListInProgressRequestIDs(_ context.Context) ([]string, error) {
	if s.req != nil && s.reqStatus == "in_progress" {
		return []string{s.req.ID}, nil
	}
	return nil, nil
}

func (s *fakeStore) ClearNodeTasks(_ context.Context, nodeID string) error {
	out := s.tasks[:0]
	for _, t := range s.tasks {
		if t.NodeID != nodeID {
			out = append(out, t)
		}
	}
	s.tasks = out
	return nil
}

func (s *fakeStore) InsertTasks(_ context.Context, tasks []repo.AgentTask) error {
	s.tasks = append(s.tasks, tasks...)
	return nil
}

func (s *fakeStore) UpdateRequestProgress(_ context.Context, _ string, status string, progress int) error {
	s.reqStatus = status
	s.reqProgress = progress
	return nil
}

func (s *fakeStore) AppendAuditEvent(_ context.Context, e repo.AuditEvent) error {
	s.auditEvents = append(s.auditEvents, e)
	return nil
}

type fakeAgent struct {
	err error
}

func (f fakeAgent) Run(_ context.Context, rr agentclient.RunRequest) (*agentclient.Decision, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &agentclient.Decision{
		Summary:    "ok",
		StatusText: rr.AgentType + " complete",
		Tasks:      []agentclient.TaskItem{{Title: "did " + rr.AgentType, Status: "completed"}},
	}, nil
}

// graph: intake -> finance -> approval (linear, all pending).
func newGraph() *fakeStore {
	return &fakeStore{
		req: &repo.Request{ID: "req_1", Title: "Open a new office in Berlin", Priority: "high"},
		nodes: []*repo.WorkflowNode{
			{ID: "n_intake", RequestID: "req_1", Key: "intake", AgentType: "intake", Department: "Planning", Status: "pending"},
			{ID: "n_fin", RequestID: "req_1", Key: "finance_review", AgentType: "finance", Department: "Finance", Status: "pending"},
			{ID: "n_appr", RequestID: "req_1", Key: "exec_approval", AgentType: "approval", Department: "Executive", Status: "pending"},
		},
		edges: []repo.WorkflowEdge{
			{ID: "e1", RequestID: "req_1", SourceNodeID: "n_intake", TargetNodeID: "n_fin", EdgeType: "sequence"},
			{ID: "e2", RequestID: "req_1", SourceNodeID: "n_fin", TargetNodeID: "n_appr", EdgeType: "sequence"},
		},
	}
}

func quietEngine(store Store, agent AgentRunner) *Engine {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewEngine(context.Background(), log, store, agent, 0, NewBus())
}

func TestRunCompletesEveryNode(t *testing.T) {
	store := newGraph()
	e := quietEngine(store, fakeAgent{})

	if err := e.run(context.Background(), "req_1"); err != nil {
		t.Fatalf("run: %v", err)
	}

	for _, n := range store.nodes {
		if n.Status != "completed" {
			t.Errorf("node %s status = %q, want completed", n.ID, n.Status)
		}
		if n.StatusText == "" {
			t.Errorf("node %s has no status_text", n.ID)
		}
	}
	if store.reqStatus != "completed" || store.reqProgress != 100 {
		t.Errorf("request = %q/%d, want completed/100", store.reqStatus, store.reqProgress)
	}
	if len(store.tasks) != 3 {
		t.Errorf("inserted %d tasks, want 3 (one per node)", len(store.tasks))
	}
}

func TestRunFallsBackWhenAgentUnavailable(t *testing.T) {
	store := newGraph()
	e := quietEngine(store, fakeAgent{err: agentclient.ErrAgentUnavailable})

	if err := e.run(context.Background(), "req_1"); err != nil {
		t.Fatalf("run: %v", err)
	}

	for _, n := range store.nodes {
		if n.Status != "completed" {
			t.Errorf("node %s not completed under fallback", n.ID)
		}
	}
	if store.reqStatus != "completed" {
		t.Errorf("request status = %q, want completed", store.reqStatus)
	}
	// DefaultDecision provides real tasks for each known stage.
	if len(store.tasks) == 0 {
		t.Error("fallback inserted no tasks")
	}
}

// newDiamond models the shape the default plan relies on: intake fans out to
// finance/legal/it (all eligible in one wave) which merge into approval.
func newDiamond() *fakeStore {
	return &fakeStore{
		req: &repo.Request{ID: "req_1", Title: "Berlin", Priority: "high"},
		nodes: []*repo.WorkflowNode{
			{ID: "n_intake", RequestID: "req_1", Key: "intake", AgentType: "intake", Department: "Planning", Status: "pending"},
			{ID: "n_fin", RequestID: "req_1", Key: "finance_review", AgentType: "finance", Department: "Finance", Status: "pending"},
			{ID: "n_legal", RequestID: "req_1", Key: "legal_review", AgentType: "legal", Department: "Legal", Status: "pending"},
			{ID: "n_it", RequestID: "req_1", Key: "it_assessment", AgentType: "it", Department: "IT", Status: "pending"},
			{ID: "n_appr", RequestID: "req_1", Key: "exec_approval", AgentType: "approval", Department: "Executive", Status: "pending"},
		},
		edges: []repo.WorkflowEdge{
			{ID: "e1", SourceNodeID: "n_intake", TargetNodeID: "n_fin"},
			{ID: "e2", SourceNodeID: "n_intake", TargetNodeID: "n_legal"},
			{ID: "e3", SourceNodeID: "n_intake", TargetNodeID: "n_it"},
			{ID: "e4", SourceNodeID: "n_fin", TargetNodeID: "n_appr"},
			{ID: "e5", SourceNodeID: "n_legal", TargetNodeID: "n_appr"},
			{ID: "e6", SourceNodeID: "n_it", TargetNodeID: "n_appr"},
		},
	}
}

func TestRunCompletesParallelBranches(t *testing.T) {
	store := newDiamond()
	e := quietEngine(store, fakeAgent{})

	if err := e.run(context.Background(), "req_1"); err != nil {
		t.Fatalf("run: %v", err)
	}

	for _, n := range store.nodes {
		if n.Status != "completed" {
			t.Errorf("node %s = %q, want completed", n.ID, n.Status)
		}
	}
	if store.reqStatus != "completed" || store.reqProgress != 100 {
		t.Errorf("request = %q/%d, want completed/100", store.reqStatus, store.reqProgress)
	}
	// The merge node must only run after all three branches complete.
	appr := store.byID("n_appr")
	if appr.Status != "completed" {
		t.Errorf("approval node not completed: %q", appr.Status)
	}
}

func TestRunWritesAuditEvents(t *testing.T) {
	store := newGraph()
	e := quietEngine(store, fakeAgent{})

	if err := e.run(context.Background(), "req_1"); err != nil {
		t.Fatalf("run: %v", err)
	}

	// Every node should have at least a "node.started" and a
	// "node.completed" audit event, plus the request-level
	// "request.completed" event.
	got := len(store.auditEvents)
	// 3 nodes * 2 events (started + completed) + 1 request.completed = 7
	wantMin := 3*2 + 1
	if got < wantMin {
		t.Errorf("audit events = %d, want at least %d (node.started + node.completed per node + request.completed)", got, wantMin)
	}

	// Check that the request.completed event exists.
	hasReqCompleted := false
	for _, ae := range store.auditEvents {
		if ae.Action == "request.completed" {
			hasReqCompleted = true
			break
		}
	}
	if !hasReqCompleted {
		t.Error("no request.completed audit event")
	}

	// All audit events should carry a non-empty actor and action.
	for _, ae := range store.auditEvents {
		if ae.Actor == "" {
			t.Errorf("audit event %s has empty actor", ae.ID)
		}
		if ae.Action == "" {
			t.Errorf("audit event %s has empty action", ae.ID)
		}
	}
}

func TestRunStopsOnCycle(t *testing.T) {
	// A two-node cycle: neither reaches in-degree 0, so nothing is eligible
	// and run stops instead of spinning. The request is left as-is.
	store := &fakeStore{
		req: &repo.Request{ID: "req_1", Title: "x"},
		nodes: []*repo.WorkflowNode{
			{ID: "a", RequestID: "req_1", AgentType: "intake", Status: "pending"},
			{ID: "b", RequestID: "req_1", AgentType: "finance", Status: "pending"},
		},
		edges: []repo.WorkflowEdge{
			{ID: "e1", SourceNodeID: "a", TargetNodeID: "b"},
			{ID: "e2", SourceNodeID: "b", TargetNodeID: "a"},
		},
	}
	e := quietEngine(store, fakeAgent{})
	if err := e.run(context.Background(), "req_1"); err != nil {
		t.Fatalf("run should stop cleanly on a cycle, got: %v", err)
	}
	if store.reqStatus == "completed" {
		t.Error("a cyclic graph should not complete")
	}
}

func TestRunResumesUnfinishedNodeIdempotently(t *testing.T) {
	// Simulate a restart: intake completed, finance left in_progress with a
	// stale task from the interrupted run, approval still pending.
	store := newGraph()
	store.byID("n_intake").Status = "completed"
	store.byID("n_fin").Status = "in_progress"
	store.reqStatus = "in_progress"
	store.tasks = []repo.AgentTask{{ID: "stale", NodeID: "n_fin", Title: "half-done", Status: "in_progress"}}

	e := quietEngine(store, fakeAgent{})
	if err := e.run(context.Background(), "req_1"); err != nil {
		t.Fatalf("resume run: %v", err)
	}

	for _, n := range store.nodes {
		if n.Status != "completed" {
			t.Errorf("node %s = %q, want completed after resume", n.ID, n.Status)
		}
	}
	// The stale finance task was cleared and re-written, not duplicated.
	finTasks := 0
	for _, tk := range store.tasks {
		if tk.NodeID == "n_fin" {
			finTasks++
			if tk.ID == "stale" {
				t.Error("stale task survived the resume")
			}
		}
	}
	if finTasks != 1 {
		t.Errorf("finance has %d tasks after resume, want 1 (cleared + re-run)", finTasks)
	}
}
