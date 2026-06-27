package repo

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// WorkflowDef is a reusable internal process (hiring, leave, onboarding): a named
// graph of department steps, scoped org-wide ('global') or to one team. It is run
// on demand to create an execution (a request row with kind='workflow_run').
type WorkflowDef struct {
	ID          string          `json:"id"`
	OrgID       string          `json:"org_id"`
	TeamID      *string         `json:"team_id"`
	TeamName    string          `json:"team_name"`
	Scope       string          `json:"scope"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Category    string          `json:"category"`
	Nodes       json.RawMessage `json:"nodes"`
	Edges       json.RawMessage `json:"edges"`
	CreatedBy   *int64          `json:"created_by"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// WorkflowRun is a compact view of one execution of a workflow definition.
type WorkflowRun struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	Progress  int       `json:"progress"`
	CreatedAt time.Time `json:"created_at"`
}

type WorkflowDefRepo struct {
	pg *pgxpool.Pool
}

func NewWorkflowDefRepo(pg *pgxpool.Pool) *WorkflowDefRepo {
	return &WorkflowDefRepo{pg: pg}
}

const workflowDefCols = `w.id, w.org_id, w.team_id, COALESCE(t.name, ''), w.scope, w.name,
	w.description, w.category, w.nodes, w.edges, w.created_by, w.created_at, w.updated_at`

func scanWorkflowDef(row pgx.Row) (*WorkflowDef, error) {
	var d WorkflowDef
	if err := row.Scan(&d.ID, &d.OrgID, &d.TeamID, &d.TeamName, &d.Scope, &d.Name,
		&d.Description, &d.Category, &d.Nodes, &d.Edges, &d.CreatedBy, &d.CreatedAt, &d.UpdatedAt); err != nil {
		return nil, err
	}
	return &d, nil
}

// List returns the workflows visible to a user: every global workflow, the
// workflows of the teams they belong to, and (for an admin) all of them.
func (r *WorkflowDefRepo) List(ctx context.Context, orgID string, userID int64, isAdmin bool) ([]WorkflowDef, error) {
	rows, err := r.pg.Query(ctx, `
		SELECT `+workflowDefCols+`
		FROM workflows w
		LEFT JOIN teams t ON t.id = w.team_id
		WHERE w.org_id = $1
		  AND (
			w.scope = 'global'
			OR $3
			OR w.team_id IN (SELECT tm.team_id FROM team_members tm WHERE tm.user_id = $2)
		  )
		ORDER BY w.scope ASC, w.name ASC
	`, orgID, userID, isAdmin)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]WorkflowDef, 0)
	for rows.Next() {
		d, err := scanWorkflowDef(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *d)
	}
	return out, rows.Err()
}

// Get returns one workflow scoped to its org, or ErrNotFound.
func (r *WorkflowDefRepo) Get(ctx context.Context, orgID, id string) (*WorkflowDef, error) {
	row := r.pg.QueryRow(ctx, `
		SELECT `+workflowDefCols+`
		FROM workflows w
		LEFT JOIN teams t ON t.id = w.team_id
		WHERE w.org_id = $1 AND w.id = $2
	`, orgID, id)
	d, err := scanWorkflowDef(row)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	return d, err
}

// Create inserts a workflow definition.
func (r *WorkflowDefRepo) Create(ctx context.Context, d WorkflowDef) error {
	_, err := r.pg.Exec(ctx, `
		INSERT INTO workflows (id, org_id, team_id, scope, name, description, category, nodes, edges, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, d.ID, d.OrgID, d.TeamID, d.Scope, d.Name, d.Description, d.Category,
		rawOrEmpty(d.Nodes), rawOrEmpty(d.Edges), d.CreatedBy)
	return err
}

// Update changes a workflow's editable fields, scoped to its org.
func (r *WorkflowDefRepo) Update(ctx context.Context, orgID, id string, d WorkflowDef) error {
	tag, err := r.pg.Exec(ctx, `
		UPDATE workflows
		SET name = $3, description = $4, category = $5, scope = $6, team_id = $7,
		    nodes = $8, edges = $9, updated_at = now()
		WHERE org_id = $1 AND id = $2
	`, orgID, id, d.Name, d.Description, d.Category, d.Scope, d.TeamID,
		rawOrEmpty(d.Nodes), rawOrEmpty(d.Edges))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a workflow scoped to its org.
func (r *WorkflowDefRepo) Delete(ctx context.Context, orgID, id string) error {
	tag, err := r.pg.Exec(ctx, `DELETE FROM workflows WHERE org_id = $1 AND id = $2`, orgID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListRuns returns the executions of a workflow, newest first.
func (r *WorkflowDefRepo) ListRuns(ctx context.Context, workflowID string) ([]WorkflowRun, error) {
	rows, err := r.pg.Query(ctx, `
		SELECT id, title, status, progress, created_at
		FROM requests
		WHERE workflow_id = $1 AND kind = 'workflow_run'
		ORDER BY created_at DESC
		LIMIT 100
	`, workflowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]WorkflowRun, 0)
	for rows.Next() {
		var run WorkflowRun
		if err := rows.Scan(&run.ID, &run.Title, &run.Status, &run.Progress, &run.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

func rawOrEmpty(r json.RawMessage) []byte {
	if len(r) == 0 {
		return []byte("[]")
	}
	return r
}
