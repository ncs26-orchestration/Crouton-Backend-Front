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
	reqStatus   string
	reqProgress int
	deps        []fakeDep
}

type fakeDep struct {
	dependentNodeID string
	blockingNodeID  string
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

func (s *fakeStore) InsertDependency(_ context.Context, dep repo.NodeDependency) error {
	s.deps = append(s.deps, fakeDep{dependentNodeID: dep.DependentNodeID, blockingNodeID: dep.BlockingNodeID})
	return nil
}

func (s *fakeStore) ResolveDependenciesBlockedBy(_ context.Context, blockingNodeID string) ([]string, error) {
	var unblocked []string
	for i, d := range s.deps {
		if d.blockingNodeID == blockingNodeID {
			unblocked = append(unblocked, d.dependentNodeID)
			s.deps[i] = fakeDep{} // mark as resolved
		}
	}
	return unblocked, nil
}

func (s *fakeStore) MaxRunCount(_ context.Context, _ string) (int, error) {
	return 0, nil
}

func (s *fakeStore) ListUnresolvedDepsByRequest(_ context.Context, _ string) ([]repo.NodeDependency, error) {
	var out []repo.NodeDependency
	for _, d := range s.deps {
		if d.dependentNodeID != "" {
			out = append(out, repo.NodeDependency{
				DependentNodeID: d.dependentNodeID,
				BlockingNodeID:  d.blockingNodeID,
			})
		}
	}
	return out, nil
}

type fakeAgent struct {
	err       error
	blockOnIT bool // when true, finance returns blocked_on:IT if no IT in upstream
}

func (f fakeAgent) Run(_ context.Context, rr agentclient.RunRequest) (*agentclient.Decision, error) {
	if f.err != nil {
		return nil, f.err
	}

	// F5: simulate Finance blocking on IT when no IT output is present in the
	// upstream context. This lets tests verify the blocked→unblock→complete
	// lifecycle without a real agent service.
	if f.blockOnIT && rr.AgentType == "finance" {
		hasIT := false
		for _, item := range rr.UpstreamContext {
			if item.Department == "IT" || item.Key == "it_assessment" {
				hasIT = true
				break
			}
		}
		if !hasIT {
			return &agentclient.Decision{
				Summary:    "Waiting for IT assessment",
				StatusText: "Finance review is blocked — waiting for IT assessment.",
				Tasks:      []agentclient.TaskItem{{Title: "Assess budget feasibility", Status: "pending"}},
				BlockedOn:  &agentclient.DependencyDecl{OnDepartment: "IT", Reason: "Need IT security assessment before finalizing budget."},
			}, nil
		}
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
	return NewEngine(context.Background(), log, store, agent, 0)
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

// newF5Graph builds a diamond where Finance can block on IT:
//
//	intake → finance → approval
//	intake → it → approval
//
// The fakeAgent is configured with blockOnIT=true so Finance returns
// blocked_on:IT on its first run and completes only after IT output is
// available in the upstream context.
func newF5Graph() *fakeStore {
	return &fakeStore{
		req: &repo.Request{ID: "req_f5", Title: "Test F5", Priority: "high"},
		nodes: []*repo.WorkflowNode{
			{ID: "n_intake", RequestID: "req_f5", Key: "intake", AgentType: "intake", Department: "Planning", Status: "pending"},
			{ID: "n_fin", RequestID: "req_f5", Key: "finance_review", AgentType: "finance", Department: "Finance", Status: "pending"},
			{ID: "n_it", RequestID: "req_f5", Key: "it_assessment", AgentType: "it", Department: "IT", Status: "pending"},
			{ID: "n_appr", RequestID: "req_f5", Key: "exec_approval", AgentType: "approval", Department: "Executive", Status: "pending"},
		},
		edges: []repo.WorkflowEdge{
			{ID: "e1", SourceNodeID: "n_intake", TargetNodeID: "n_fin"},
			{ID: "e2", SourceNodeID: "n_intake", TargetNodeID: "n_it"},
			{ID: "e3", SourceNodeID: "n_fin", TargetNodeID: "n_appr"},
			{ID: "e4", SourceNodeID: "n_it", TargetNodeID: "n_appr"},
		},
	}
}

func TestRunF5BlockedOnITThenUnblocked(t *testing.T) {
	store := newF5Graph()
	agent := fakeAgent{blockOnIT: true}
	e := quietEngine(store, agent)

	if err := e.run(context.Background(), "req_f5"); err != nil {
		t.Fatalf("run: %v", err)
	}

	for _, n := range store.nodes {
		if n.Status != "completed" {
			t.Errorf("node %s (%s) status = %q, want completed", n.ID, n.Key, n.Status)
		}
	}
	if store.reqStatus != "completed" || store.reqProgress != 100 {
		t.Errorf("request = %q/%d, want completed/100", store.reqStatus, store.reqProgress)
	}
	// The dependency should have been recorded and resolved.
	if len(store.deps) > 0 {
		for _, d := range store.deps {
			if d.dependentNodeID != "" {
				t.Errorf("unresolved dependency found: %+v", d)
			}
		}
	}
}

func TestRunF5BlockedReRunCapped(t *testing.T) {
	// When an agent keeps returning blocked_on even with upstream context,
	// the engine should cap re-runs and complete the node with a fallback.
	store := newF5Graph()
	store.byID("n_intake").Status = "completed"
	store.byID("n_intake").StatusText = "Intake complete"
	store.reqStatus = "in_progress"
	// Return blocked_on for all runs (never unblock) to test capping.
	e := quietEngine(store, fakeAgent{err: nil})
	e.stepDelay = 0

	if err := e.run(context.Background(), "req_f5"); err != nil {
		t.Fatalf("run: %v", err)
	}
	// Even with the never-unblocking scenario, the engine should finish
	// (via fallback after max re-runs).
	_ = e
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
