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
}

func (s *fakeStore) GetRequest(_ context.Context, _ string) (*repo.Request, error) {
	return s.req, nil
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

func (s *fakeStore) InsertTasks(_ context.Context, tasks []repo.AgentTask) error {
	s.tasks = append(s.tasks, tasks...)
	return nil
}

func (s *fakeStore) UpdateRequestProgress(_ context.Context, _ string, status string, progress int) error {
	s.reqStatus = status
	s.reqProgress = progress
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
