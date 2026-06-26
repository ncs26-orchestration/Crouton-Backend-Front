package orchestrator

import (
	"context"

	"github.com/ncs26-orchestration/solution/apps/api/internal/repo"
)

// dbStore implements Store over the request + workflow + audit repos.
type dbStore struct {
	requests *repo.RequestRepo
	workflow *repo.WorkflowRepo
	audit    *repo.AuditRepo
}

// NewDBStore adapts the concrete repos to the engine's Store interface.
func NewDBStore(requests *repo.RequestRepo, workflow *repo.WorkflowRepo, audit *repo.AuditRepo) Store {
	return &dbStore{requests: requests, workflow: workflow, audit: audit}
}

func (s *dbStore) GetRequest(ctx context.Context, requestID string) (*repo.Request, error) {
	return s.requests.GetByID(ctx, requestID)
}

func (s *dbStore) ListNodesByRequest(ctx context.Context, requestID string) ([]repo.WorkflowNode, error) {
	return s.workflow.ListNodesByRequest(ctx, requestID)
}

func (s *dbStore) ListEdgesByRequest(ctx context.Context, requestID string) ([]repo.WorkflowEdge, error) {
	return s.workflow.ListEdgesByRequest(ctx, requestID)
}

func (s *dbStore) ListInProgressRequestIDs(ctx context.Context) ([]string, error) {
	return s.requests.ListIDsByStatus(ctx, "in_progress")
}

func (s *dbStore) UpdateNodeStatus(ctx context.Context, nodeID, status, statusText string, progressPercent int) error {
	return s.workflow.UpdateNodeStatus(ctx, nodeID, status, statusText, progressPercent)
}

func (s *dbStore) ClearNodeTasks(ctx context.Context, nodeID string) error {
	return s.workflow.DeleteTasksByNode(ctx, nodeID)
}

func (s *dbStore) InsertTasks(ctx context.Context, tasks []repo.AgentTask) error {
	return s.workflow.InsertTasks(ctx, tasks)
}

func (s *dbStore) UpdateRequestProgress(ctx context.Context, requestID, status string, progressPercent int) error {
	return s.workflow.UpdateRequestProgress(ctx, requestID, status, progressPercent)
}

func (s *dbStore) AppendAuditEvent(ctx context.Context, e repo.AuditEvent) error {
	return s.audit.Append(ctx, e)
}
