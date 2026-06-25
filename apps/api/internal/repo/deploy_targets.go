package repo

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DeployTarget is a project-scoped engine endpoint. A project may
// register multiple (e.g., Camunda dev + Camunda prod + Elsa
// staging) and the chat's Deploy button picks one at deploy time.
//
// Replaces the old tenant-wide engine_connections model — the new
// operator-tool pitch is "each client company has its own
// infrastructure", so scoping by project matches how the operator
// thinks about it.
type DeployTarget struct {
	ID         string
	ProjectID  string
	Kind       string // camunda7 | elsa3
	Name       string
	Endpoint   string
	AuthKind   string
	AuthUser   string
	AuthSecret string
	CreatedAt  time.Time
}

type DeployTargetRepo struct{ pg *pgxpool.Pool }

func NewDeployTargetRepo(pg *pgxpool.Pool) *DeployTargetRepo { return &DeployTargetRepo{pg: pg} }

func (r *DeployTargetRepo) Create(ctx context.Context, t DeployTarget) error {
	_, err := r.pg.Exec(ctx, `
		INSERT INTO deploy_targets
		  (id, project_id, kind, name, endpoint, auth_kind, auth_user, auth_secret)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, t.ID, t.ProjectID, t.Kind, t.Name, t.Endpoint, t.AuthKind, t.AuthUser, t.AuthSecret)
	return err
}

func (r *DeployTargetRepo) Get(ctx context.Context, id string) (*DeployTarget, error) {
	row := r.pg.QueryRow(ctx, `
		SELECT id, project_id, kind, name, endpoint, auth_kind, auth_user, auth_secret, created_at
		FROM deploy_targets WHERE id = $1
	`, id)
	var t DeployTarget
	if err := row.Scan(&t.ID, &t.ProjectID, &t.Kind, &t.Name, &t.Endpoint, &t.AuthKind, &t.AuthUser, &t.AuthSecret, &t.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &t, nil
}

func (r *DeployTargetRepo) ListByProject(ctx context.Context, projectID string) ([]DeployTarget, error) {
	rows, err := r.pg.Query(ctx, `
		SELECT id, project_id, kind, name, endpoint, auth_kind, auth_user, auth_secret, created_at
		FROM deploy_targets WHERE project_id = $1 ORDER BY created_at ASC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DeployTarget
	for rows.Next() {
		var t DeployTarget
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.Kind, &t.Name, &t.Endpoint, &t.AuthKind, &t.AuthUser, &t.AuthSecret, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *DeployTargetRepo) Delete(ctx context.Context, id string) error {
	tag, err := r.pg.Exec(ctx, `DELETE FROM deploy_targets WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
