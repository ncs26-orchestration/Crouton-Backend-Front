package repo

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DepartmentPolicy struct {
	ID        string
	OrgID     string
	TeamID    string
	Title     string
	Body      string
	CreatedAt time.Time
}

type PolicyRepo struct {
	pg *pgxpool.Pool
}

func NewPolicyRepo(pg *pgxpool.Pool) *PolicyRepo {
	return &PolicyRepo{pg: pg}
}

func (r *PolicyRepo) ListByOrg(ctx context.Context, orgID string) ([]DepartmentPolicy, error) {
	rows, err := r.pg.Query(ctx, `
		SELECT id, org_id, team_id, title, body, created_at
		FROM department_policies
		WHERE org_id = $1
		ORDER BY created_at ASC
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DepartmentPolicy
	for rows.Next() {
		var p DepartmentPolicy
		if err := rows.Scan(&p.ID, &p.OrgID, &p.TeamID, &p.Title, &p.Body, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *PolicyRepo) ListByTeam(ctx context.Context, orgID, teamID string) ([]DepartmentPolicy, error) {
	rows, err := r.pg.Query(ctx, `
		SELECT id, org_id, team_id, title, body, created_at
		FROM department_policies
		WHERE org_id = $1 AND team_id = $2
		ORDER BY created_at ASC
	`, orgID, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DepartmentPolicy
	for rows.Next() {
		var p DepartmentPolicy
		if err := rows.Scan(&p.ID, &p.OrgID, &p.TeamID, &p.Title, &p.Body, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *PolicyRepo) Insert(ctx context.Context, p DepartmentPolicy) error {
	_, err := r.pg.Exec(ctx, `
		INSERT INTO department_policies (id, org_id, team_id, title, body)
		VALUES ($1, $2, $3, $4, $5)
	`, p.ID, p.OrgID, p.TeamID, p.Title, p.Body)
	return err
}
