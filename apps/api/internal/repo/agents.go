package repo

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Agent struct {
	ID           string
	OrgID        string
	TeamID       string
	AgentType    string
	Name         string
	Avatar       string
	Capabilities string
	CreatedAt    time.Time
}

type AgentRepo struct {
	pg *pgxpool.Pool
}

func NewAgentRepo(pg *pgxpool.Pool) *AgentRepo {
	return &AgentRepo{pg: pg}
}

func (r *AgentRepo) ListByOrg(ctx context.Context, orgID string) ([]Agent, error) {
	rows, err := r.pg.Query(ctx, `
		SELECT id, org_id, team_id, agent_type, name, avatar, capabilities, created_at
		FROM agents
		WHERE org_id = $1
		ORDER BY created_at ASC
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Agent
	for rows.Next() {
		var a Agent
		if err := rows.Scan(&a.ID, &a.OrgID, &a.TeamID, &a.AgentType, &a.Name, &a.Avatar, &a.Capabilities, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *AgentRepo) GetByType(ctx context.Context, orgID, agentType string) (*Agent, error) {
	var a Agent
	err := r.pg.QueryRow(ctx, `
		SELECT id, org_id, team_id, agent_type, name, avatar, capabilities, created_at
		FROM agents
		WHERE org_id = $1 AND agent_type = $2
	`, orgID, agentType).Scan(&a.ID, &a.OrgID, &a.TeamID, &a.AgentType, &a.Name, &a.Avatar, &a.Capabilities, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *AgentRepo) Insert(ctx context.Context, a Agent) error {
	_, err := r.pg.Exec(ctx, `
		INSERT INTO agents (id, org_id, team_id, agent_type, name, avatar, capabilities)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, a.ID, a.OrgID, a.TeamID, a.AgentType, a.Name, a.Avatar, a.Capabilities)
	return err
}
