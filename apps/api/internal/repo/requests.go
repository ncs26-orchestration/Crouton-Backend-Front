package repo

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Request struct {
	ID                  string
	OrgID               string
	Title               string
	Description         string
	RequesterUserID     int64
	Priority            string
	Status              string
	Progress            int
	EstimatedCompletion *time.Time
	CreatedAt           time.Time
}

type RequestRepo struct {
	pg *pgxpool.Pool
}

func NewRequestRepo(pg *pgxpool.Pool) *RequestRepo {
	return &RequestRepo{pg: pg}
}

func (r *RequestRepo) Create(ctx context.Context, req Request) error {
	_, err := r.pg.Exec(ctx, `
		INSERT INTO requests (id, org_id, title, description, requester_user_id, priority, status, progress)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, req.ID, req.OrgID, req.Title, req.Description, req.RequesterUserID, req.Priority, req.Status, req.Progress)
	return err
}

func (r *RequestRepo) GetByID(ctx context.Context, id string) (*Request, error) {
	row := r.pg.QueryRow(ctx, `
		SELECT id, org_id, title, description, requester_user_id, priority, status, progress,
		       estimated_completion, created_at
		FROM requests WHERE id = $1
	`, id)
	var req Request
	if err := row.Scan(&req.ID, &req.OrgID, &req.Title, &req.Description, &req.RequesterUserID,
		&req.Priority, &req.Status, &req.Progress, &req.EstimatedCompletion, &req.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &req, nil
}

func (r *RequestRepo) ListByOrg(ctx context.Context, orgID string) ([]Request, error) {
	rows, err := r.pg.Query(ctx, `
		SELECT id, org_id, title, description, requester_user_id, priority, status, progress,
		       estimated_completion, created_at
		FROM requests WHERE org_id = $1
		ORDER BY created_at DESC
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Request
	for rows.Next() {
		var req Request
		if err := rows.Scan(&req.ID, &req.OrgID, &req.Title, &req.Description, &req.RequesterUserID,
			&req.Priority, &req.Status, &req.Progress, &req.EstimatedCompletion, &req.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, req)
	}
	return out, rows.Err()
}

func (r *RequestRepo) UpdateStatusProgress(ctx context.Context, id string, status string, progress int) error {
	tag, err := r.pg.Exec(ctx, `
		UPDATE requests SET status = $2, progress = $3 WHERE id = $1
	`, id, status, progress)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
