package repo

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NodeMessage is one turn in a node's verifier↔agent conversation.
type NodeMessage struct {
	ID           string    `json:"id"`
	RequestID    string    `json:"request_id"`
	NodeID       string    `json:"node_id"`
	AuthorUserID *int64    `json:"author_user_id"`
	AuthorName   string    `json:"author_name"`
	Role         string    `json:"role"` // human | agent | system
	Body         string    `json:"body"`
	CreatedAt    time.Time `json:"created_at"`
}

type NodeMessageRepo struct {
	pg *pgxpool.Pool
}

func NewNodeMessageRepo(pg *pgxpool.Pool) *NodeMessageRepo {
	return &NodeMessageRepo{pg: pg}
}

func (r *NodeMessageRepo) Create(ctx context.Context, m NodeMessage) error {
	_, err := r.pg.Exec(ctx, `
		INSERT INTO node_messages (id, request_id, node_id, author_user_id, author_name, role, body)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, m.ID, m.RequestID, m.NodeID, m.AuthorUserID, m.AuthorName, m.Role, m.Body)
	return err
}

func (r *NodeMessageRepo) ListByNode(ctx context.Context, nodeID string) ([]NodeMessage, error) {
	rows, err := r.pg.Query(ctx, `
		SELECT id, request_id, node_id, author_user_id, author_name, role, body, created_at
		FROM node_messages WHERE node_id = $1 ORDER BY created_at ASC
	`, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]NodeMessage, 0)
	for rows.Next() {
		var m NodeMessage
		if err := rows.Scan(&m.ID, &m.RequestID, &m.NodeID, &m.AuthorUserID, &m.AuthorName, &m.Role, &m.Body, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
