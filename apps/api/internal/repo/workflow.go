package repo

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WorkflowNode struct {
	ID              string
	RequestID       string
	Key             string
	Name            string
	AgentType       string
	Department      string
	Status          string
	Description     string
	ProgressPercent int
	StatusText      string
	StartedAt       *time.Time
	CompletedAt     *time.Time
	CreatedAt       time.Time
}

type WorkflowEdge struct {
	ID           string
	RequestID    string
	SourceNodeID string
	TargetNodeID string
	EdgeType     string
}

type WorkflowRepo struct {
	pg *pgxpool.Pool
}

func NewWorkflowRepo(pg *pgxpool.Pool) *WorkflowRepo {
	return &WorkflowRepo{pg: pg}
}

// InsertGraphTx inserts all nodes then all edges inside the given
// transaction in one round trip via a batch. Nodes are queued before
// edges so the edge foreign keys resolve. Use this so the workflow graph
// is written all-or-nothing.
func (r *WorkflowRepo) InsertGraphTx(ctx context.Context, tx pgx.Tx, nodes []WorkflowNode, edges []WorkflowEdge) error {
	batch := &pgx.Batch{}
	for _, n := range nodes {
		batch.Queue(`
			INSERT INTO workflow_nodes (id, request_id, key, name, agent_type, department, status, description)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, n.ID, n.RequestID, n.Key, n.Name, n.AgentType, n.Department, n.Status, n.Description)
	}
	for _, e := range edges {
		batch.Queue(`
			INSERT INTO workflow_edges (id, request_id, source_node_id, target_node_id, edge_type)
			VALUES ($1, $2, $3, $4, $5)
		`, e.ID, e.RequestID, e.SourceNodeID, e.TargetNodeID, e.EdgeType)
	}

	br := tx.SendBatch(ctx, batch)
	defer func() { _ = br.Close() }()
	for range len(nodes) + len(edges) {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return nil
}

func (r *WorkflowRepo) InsertNodes(ctx context.Context, nodes []WorkflowNode) error {
	for _, n := range nodes {
		_, err := r.pg.Exec(ctx, `
			INSERT INTO workflow_nodes (id, request_id, key, name, agent_type, department, status, description)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, n.ID, n.RequestID, n.Key, n.Name, n.AgentType, n.Department, n.Status, n.Description)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *WorkflowRepo) InsertEdges(ctx context.Context, edges []WorkflowEdge) error {
	for _, e := range edges {
		_, err := r.pg.Exec(ctx, `
			INSERT INTO workflow_edges (id, request_id, source_node_id, target_node_id, edge_type)
			VALUES ($1, $2, $3, $4, $5)
		`, e.ID, e.RequestID, e.SourceNodeID, e.TargetNodeID, e.EdgeType)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *WorkflowRepo) ListNodesByRequest(ctx context.Context, requestID string) ([]WorkflowNode, error) {
	rows, err := r.pg.Query(ctx, `
		SELECT id, request_id, key, name, agent_type, department, status, description,
		       progress_percent, status_text, started_at, completed_at, created_at
		FROM workflow_nodes WHERE request_id = $1
		ORDER BY created_at ASC
	`, requestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []WorkflowNode
	for rows.Next() {
		var n WorkflowNode
		if err := rows.Scan(&n.ID, &n.RequestID, &n.Key, &n.Name, &n.AgentType, &n.Department,
			&n.Status, &n.Description, &n.ProgressPercent, &n.StatusText,
			&n.StartedAt, &n.CompletedAt, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (r *WorkflowRepo) ListEdgesByRequest(ctx context.Context, requestID string) ([]WorkflowEdge, error) {
	rows, err := r.pg.Query(ctx, `
		SELECT id, request_id, source_node_id, target_node_id, edge_type
		FROM workflow_edges WHERE request_id = $1
	`, requestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []WorkflowEdge
	for rows.Next() {
		var e WorkflowEdge
		if err := rows.Scan(&e.ID, &e.RequestID, &e.SourceNodeID, &e.TargetNodeID, &e.EdgeType); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *WorkflowRepo) GetNode(ctx context.Context, nodeID string) (*WorkflowNode, error) {
	row := r.pg.QueryRow(ctx, `
		SELECT id, request_id, key, name, agent_type, department, status, description,
		       progress_percent, status_text, started_at, completed_at, created_at
		FROM workflow_nodes WHERE id = $1
	`, nodeID)
	var n WorkflowNode
	if err := row.Scan(&n.ID, &n.RequestID, &n.Key, &n.Name, &n.AgentType, &n.Department,
		&n.Status, &n.Description, &n.ProgressPercent, &n.StatusText,
		&n.StartedAt, &n.CompletedAt, &n.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &n, nil
}
