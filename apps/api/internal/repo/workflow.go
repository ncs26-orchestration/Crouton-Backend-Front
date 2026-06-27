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
	DecisionOutcome string
	DecisionSummary string
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

// AgentTask is a unit of work a department agent reported for a node.
type AgentTask struct {
	ID          string
	NodeID      string
	Title       string
	Status      string
	Ordinal     int
	StartedAt   *time.Time
	CompletedAt *time.Time
	CreatedAt   time.Time
}

// NodeFlag is a risk or note an agent raised on a node (severity + message,
// which may cite a policy).
type NodeFlag struct {
	ID        string
	RequestID string
	NodeID    string
	Severity  string
	Message   string
	Ordinal   int
	CreatedAt time.Time
}

// NodeCheck is the result of evaluating one policy rule against a request on a
// node: pass | warn | fail, with the cited policy.
type NodeCheck struct {
	ID          string `json:"id"`
	RequestID   string `json:"request_id"`
	NodeID      string `json:"node_id"`
	Label       string `json:"label"`
	Status      string `json:"status"`
	Detail      string `json:"detail"`
	PolicyTitle string `json:"policy_title"`
	Ordinal     int    `json:"ordinal"`
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
		       progress_percent, status_text, decision_outcome, decision_summary, started_at, completed_at, created_at
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
			&n.Status, &n.Description, &n.ProgressPercent, &n.StatusText, &n.DecisionOutcome, &n.DecisionSummary,
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
		       progress_percent, status_text, decision_outcome, decision_summary, started_at, completed_at, created_at
		FROM workflow_nodes WHERE id = $1
	`, nodeID)
	var n WorkflowNode
	if err := row.Scan(&n.ID, &n.RequestID, &n.Key, &n.Name, &n.AgentType, &n.Department,
		&n.Status, &n.Description, &n.ProgressPercent, &n.StatusText, &n.DecisionOutcome, &n.DecisionSummary,
		&n.StartedAt, &n.CompletedAt, &n.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &n, nil
}

// UpdateNodeStatus advances a node's status, status_text, and progress. It
// stamps started_at the first time the node leaves pending and completed_at
// when it reaches completed. Used by the orchestration engine (F3).
func (r *WorkflowRepo) UpdateNodeStatus(ctx context.Context, nodeID, status, statusText string, progressPercent int) error {
	tag, err := r.pg.Exec(ctx, `
		UPDATE workflow_nodes
		SET status = $2,
		    status_text = $3,
		    progress_percent = $4,
		    started_at = COALESCE(started_at, CASE WHEN $2 IN ('in_progress', 'completed') THEN now() END),
		    completed_at = CASE WHEN $2 = 'completed' THEN now() ELSE completed_at END
		WHERE id = $1
	`, nodeID, status, statusText, progressPercent)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateNodeDecisionOutcome records the call a department agent reached on a
// node (approve, approve_with_conditions, flag, reject, block) for traceability.
func (r *WorkflowRepo) UpdateNodeDecisionOutcome(ctx context.Context, nodeID, outcome string) error {
	tag, err := r.pg.Exec(ctx, `
		UPDATE workflow_nodes SET decision_outcome = $2 WHERE id = $1
	`, nodeID, outcome)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetNodeDecisionSummary stores the agent's full reasoning for a node.
func (r *WorkflowRepo) SetNodeDecisionSummary(ctx context.Context, nodeID, summary string) error {
	tag, err := r.pg.Exec(ctx, `UPDATE workflow_nodes SET decision_summary = $2 WHERE id = $1`, nodeID, summary)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteFlagsByNode clears a node's flags so a re-run is idempotent.
func (r *WorkflowRepo) DeleteFlagsByNode(ctx context.Context, nodeID string) error {
	_, err := r.pg.Exec(ctx, `DELETE FROM node_flags WHERE node_id = $1`, nodeID)
	return err
}

// InsertFlags writes a node's flags in one batch.
func (r *WorkflowRepo) InsertFlags(ctx context.Context, flags []NodeFlag) error {
	if len(flags) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, f := range flags {
		batch.Queue(`
			INSERT INTO node_flags (id, request_id, node_id, severity, message, ordinal)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, f.ID, f.RequestID, f.NodeID, f.Severity, f.Message, f.Ordinal)
	}
	br := r.pg.SendBatch(ctx, batch)
	defer func() { _ = br.Close() }()
	for range flags {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return nil
}

// DeleteChecksByNode clears a node's checks so a re-run is idempotent.
func (r *WorkflowRepo) DeleteChecksByNode(ctx context.Context, nodeID string) error {
	_, err := r.pg.Exec(ctx, `DELETE FROM node_checks WHERE node_id = $1`, nodeID)
	return err
}

// InsertChecks writes a node's policy checks in one batch.
func (r *WorkflowRepo) InsertChecks(ctx context.Context, checks []NodeCheck) error {
	if len(checks) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, c := range checks {
		batch.Queue(`
			INSERT INTO node_checks (id, request_id, node_id, label, status, detail, policy_title, ordinal)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, c.ID, c.RequestID, c.NodeID, c.Label, c.Status, c.Detail, c.PolicyTitle, c.Ordinal)
	}
	br := r.pg.SendBatch(ctx, batch)
	defer func() { _ = br.Close() }()
	for range checks {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return nil
}

// ListChecksByNode returns a node's policy checks in display order.
func (r *WorkflowRepo) ListChecksByNode(ctx context.Context, nodeID string) ([]NodeCheck, error) {
	rows, err := r.pg.Query(ctx, `
		SELECT id, request_id, node_id, label, status, detail, policy_title, ordinal
		FROM node_checks WHERE node_id = $1 ORDER BY ordinal ASC, created_at ASC
	`, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]NodeCheck, 0)
	for rows.Next() {
		var c NodeCheck
		if err := rows.Scan(&c.ID, &c.RequestID, &c.NodeID, &c.Label, &c.Status, &c.Detail, &c.PolicyTitle, &c.Ordinal); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListFlagsByNode returns a node's flags in display order.
func (r *WorkflowRepo) ListFlagsByNode(ctx context.Context, nodeID string) ([]NodeFlag, error) {
	rows, err := r.pg.Query(ctx, `
		SELECT id, request_id, node_id, severity, message, ordinal, created_at
		FROM node_flags WHERE node_id = $1 ORDER BY ordinal ASC, created_at ASC
	`, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]NodeFlag, 0)
	for rows.Next() {
		var f NodeFlag
		if err := rows.Scan(&f.ID, &f.RequestID, &f.NodeID, &f.Severity, &f.Message, &f.Ordinal, &f.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// InsertTasks writes a node's agent tasks in one batch.
func (r *WorkflowRepo) InsertTasks(ctx context.Context, tasks []AgentTask) error {
	if len(tasks) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, t := range tasks {
		batch.Queue(`
			INSERT INTO agent_tasks (id, node_id, title, status, ordinal, started_at, completed_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, t.ID, t.NodeID, t.Title, t.Status, t.Ordinal, t.StartedAt, t.CompletedAt)
	}
	br := r.pg.SendBatch(ctx, batch)
	defer func() { _ = br.Close() }()
	for range tasks {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return nil
}

// DeleteTasksByNode removes a node's tasks. The engine calls it before
// (re)writing tasks so re-running a node (after a restart) stays idempotent.
func (r *WorkflowRepo) DeleteTasksByNode(ctx context.Context, nodeID string) error {
	_, err := r.pg.Exec(ctx, `DELETE FROM agent_tasks WHERE node_id = $1`, nodeID)
	return err
}

// ListTasksByRequest returns all tasks for all nodes in a request, joined
// through workflow_nodes. Used by the report endpoint (F8) to assemble
// per-stage task lists without N+1 queries.
func (r *WorkflowRepo) ListTasksByRequest(ctx context.Context, requestID string) ([]AgentTask, error) {
	rows, err := r.pg.Query(ctx, `
		SELECT at.id, at.node_id, at.title, at.status, at.ordinal, at.started_at, at.completed_at, at.created_at
		FROM agent_tasks at
		JOIN workflow_nodes wn ON wn.id = at.node_id
		WHERE wn.request_id = $1
		ORDER BY wn.created_at ASC, at.ordinal ASC
	`, requestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]AgentTask, 0)
	for rows.Next() {
		var t AgentTask
		if err := rows.Scan(&t.ID, &t.NodeID, &t.Title, &t.Status, &t.Ordinal, &t.StartedAt, &t.CompletedAt, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ListTasksByNode returns a node's tasks in creation order.
func (r *WorkflowRepo) ListTasksByNode(ctx context.Context, nodeID string) ([]AgentTask, error) {
	rows, err := r.pg.Query(ctx, `
		SELECT id, node_id, title, status, ordinal, started_at, completed_at, created_at
		FROM agent_tasks
		WHERE node_id = $1
		ORDER BY ordinal ASC, created_at ASC
	`, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]AgentTask, 0)
	for rows.Next() {
		var t AgentTask
		if err := rows.Scan(&t.ID, &t.NodeID, &t.Title, &t.Status, &t.Ordinal, &t.StartedAt, &t.CompletedAt, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// UpdateRequestProgress sets a request's status and progress. Mirrors
// RequestRepo.UpdateStatusProgress but lives here so the engine can drive both
// node and request transitions through one store. Kept thin on purpose.
func (r *WorkflowRepo) UpdateRequestProgress(ctx context.Context, requestID, status string, progress int) error {
	tag, err := r.pg.Exec(ctx, `UPDATE requests SET status = $2, progress = $3 WHERE id = $1`, requestID, status, progress)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
