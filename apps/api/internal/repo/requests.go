package repo

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Request is a business request submitted into an org. It is the spine
// of the AI Org OS: planned into a workflow, worked by department
// agents, approved by a human. Status follows the lifecycle enum and
// progress is a 0-100 percentage the engine maintains.
type Request struct {
	ID                  string
	OrgID               string
	Title               string
	Description         string
	RequesterUserID     int64
	RequesterRole       string
	RequestType         string
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

// Create inserts a row and returns it with the DB-populated defaults
// (status, progress, created_at) via RETURNING, so the caller doesn't
// need a follow-up read. The ID is caller-supplied so the handler can
// generate a friendly prefixed id.
func (r *RequestRepo) Create(ctx context.Context, req Request) (*Request, error) {
	row := r.pg.QueryRow(ctx, `
		INSERT INTO requests (id, org_id, title, description, requester_user_id, requester_role, priority, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, org_id, title, description, requester_user_id, requester_role, request_type, priority, status, progress, estimated_completion, created_at
	`, req.ID, req.OrgID, req.Title, req.Description, req.RequesterUserID, req.RequesterRole, req.Priority, req.Status)
	var out Request
	if err := row.Scan(
		&out.ID, &out.OrgID, &out.Title, &out.Description, &out.RequesterUserID, &out.RequesterRole,
		&out.RequestType, &out.Priority, &out.Status, &out.Progress, &out.EstimatedCompletion, &out.CreatedAt,
	); err != nil {
		return nil, err
	}
	return &out, nil
}

// SetRequestType records the intake agent's classification of a request.
func (r *RequestRepo) SetRequestType(ctx context.Context, id, requestType string) error {
	_, err := r.pg.Exec(ctx, `UPDATE requests SET request_type = $2 WHERE id = $1`, id, requestType)
	return err
}

// GetByID returns a single request or ErrNotFound.
func (r *RequestRepo) GetByID(ctx context.Context, id string) (*Request, error) {
	row := r.pg.QueryRow(ctx, `
		SELECT id, org_id, title, description, requester_user_id, requester_role, request_type, priority, status, progress, estimated_completion, created_at
		FROM requests
		WHERE id = $1
	`, id)
	var req Request
	if err := row.Scan(
		&req.ID, &req.OrgID, &req.Title, &req.Description, &req.RequesterUserID, &req.RequesterRole,
		&req.RequestType, &req.Priority, &req.Status, &req.Progress, &req.EstimatedCompletion, &req.CreatedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &req, nil
}

// ListByOrg returns an org's requests, newest first, bounded by limit.
// The cap keeps the response and the FE table from growing unbounded as
// requests pile up; cursor pagination can layer on later.
func (r *RequestRepo) ListByOrg(ctx context.Context, orgID string, limit int) ([]Request, error) {
	rows, err := r.pg.Query(ctx, `
		SELECT id, org_id, title, description, requester_user_id, requester_role, request_type, priority, status, progress, estimated_completion, created_at
		FROM requests
		WHERE org_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, orgID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Request, 0)
	for rows.Next() {
		var req Request
		if err := rows.Scan(
			&req.ID, &req.OrgID, &req.Title, &req.Description, &req.RequesterUserID, &req.RequesterRole,
			&req.RequestType, &req.Priority, &req.Status, &req.Progress, &req.EstimatedCompletion, &req.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, req)
	}
	return out, rows.Err()
}

// ListIDsByStatus returns the ids of requests in a given status. The
// orchestration engine uses it on boot to resume requests left in_progress by
// a restart.
func (r *RequestRepo) ListIDsByStatus(ctx context.Context, status string) ([]string, error) {
	rows, err := r.pg.Query(ctx, `SELECT id FROM requests WHERE status = $1 ORDER BY created_at ASC`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// UpdateStatusProgress moves a request's status and progress forward.
// The orchestration engine (BE-5) drives this as nodes complete.
func (r *RequestRepo) UpdateStatusProgress(ctx context.Context, id, status string, progress int) error {
	return updateStatusProgress(ctx, r.pg, id, status, progress)
}

// UpdateStatusProgressTx is UpdateStatusProgress scoped to a transaction,
// so a status change can be committed atomically with related writes.
func (r *RequestRepo) UpdateStatusProgressTx(ctx context.Context, tx pgx.Tx, id, status string, progress int) error {
	return updateStatusProgress(ctx, tx, id, status, progress)
}

// querier is the subset of the pgx API shared by *pgxpool.Pool and pgx.Tx.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func updateStatusProgress(ctx context.Context, q querier, id, status string, progress int) error {
	tag, err := q.Exec(ctx, `
		UPDATE requests
		SET status = $2, progress = $3
		WHERE id = $1
	`, id, status, progress)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
