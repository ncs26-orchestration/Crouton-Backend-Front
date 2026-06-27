package repo

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ncs26-orchestration/solution/apps/api/internal/policyrules"
)

// DepartmentPolicy is a written guideline an agent checks a request against.
// Department is the owning team's name, so the engine can hand each department
// agent only the policies that apply to it. Rules are typed, machine-checkable
// conditions evaluated against a request's details.
type DepartmentPolicy struct {
	ID         string
	Department string
	Title      string
	Body       string
	Rules      []policyrules.Rule
}

// scanRules unmarshals a jsonb rules column into typed rules.
func scanRules(raw []byte) []policyrules.Rule {
	if len(raw) == 0 {
		return nil
	}
	var rules []policyrules.Rule
	if err := json.Unmarshal(raw, &rules); err != nil {
		return nil
	}
	return rules
}

type PolicyRepo struct {
	pg *pgxpool.Pool
}

func NewPolicyRepo(pg *pgxpool.Pool) *PolicyRepo {
	return &PolicyRepo{pg: pg}
}

// Create inserts a policy and returns its id. rules is raw JSON ("[]" if none).
func (r *PolicyRepo) Create(ctx context.Context, id, orgID, teamID, title, body string, rules []byte) error {
	if len(rules) == 0 {
		rules = []byte("[]")
	}
	_, err := r.pg.Exec(ctx, `
		INSERT INTO department_policies (id, org_id, team_id, title, body, rules)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, id, orgID, teamID, title, body, rules)
	return err
}

// Update changes a policy's title, body, and rules, scoped to its org.
func (r *PolicyRepo) Update(ctx context.Context, orgID, id, title, body string, rules []byte) error {
	if len(rules) == 0 {
		rules = []byte("[]")
	}
	_, err := r.pg.Exec(ctx, `
		UPDATE department_policies SET title = $3, body = $4, rules = $5
		WHERE id = $1 AND org_id = $2
	`, id, orgID, title, body, rules)
	return err
}

// Delete removes a policy scoped to its org.
func (r *PolicyRepo) Delete(ctx context.Context, orgID, id string) error {
	_, err := r.pg.Exec(ctx, `DELETE FROM department_policies WHERE id = $1 AND org_id = $2`, id, orgID)
	return err
}

// ListByOrg returns every department policy in an org, joined to the owning
// team's name so callers can group by department.
func (r *PolicyRepo) ListByOrg(ctx context.Context, orgID string) ([]DepartmentPolicy, error) {
	rows, err := r.pg.Query(ctx, `
		SELECT dp.id, COALESCE(t.name, '') AS department, dp.title, dp.body, dp.rules
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
		var rulesRaw []byte
		if err := rows.Scan(&p.ID, &p.Department, &p.Title, &p.Body, &rulesRaw); err != nil {
			return nil, err
		}
		p.Rules = scanRules(rulesRaw)
		out = append(out, p)
	}
	return out, rows.Err()
}
