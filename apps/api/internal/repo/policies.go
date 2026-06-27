package repo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DepartmentPolicy is a written guideline an agent checks a request against.
// Department is the owning team's name, so the engine can hand each department
// agent only the policies that apply to it.
type DepartmentPolicy struct {
	ID         string
	Department string
	Title      string
	Body       string
}

type PolicyRepo struct {
	pg *pgxpool.Pool
}

func NewPolicyRepo(pg *pgxpool.Pool) *PolicyRepo {
	return &PolicyRepo{pg: pg}
}

// ListByOrg returns every department policy in an org, joined to the owning
// team's name so callers can group by department.
func (r *PolicyRepo) ListByOrg(ctx context.Context, orgID string) ([]DepartmentPolicy, error) {
	rows, err := r.pg.Query(ctx, `
		SELECT dp.id, COALESCE(t.name, '') AS department, dp.title, dp.body
		FROM department_policies dp
		LEFT JOIN teams t ON t.id = dp.team_id
		WHERE dp.org_id = $1
		ORDER BY dp.created_at ASC
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]DepartmentPolicy, 0)
	for rows.Next() {
		var p DepartmentPolicy
		if err := rows.Scan(&p.ID, &p.Department, &p.Title, &p.Body); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
