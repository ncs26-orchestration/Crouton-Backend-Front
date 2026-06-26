package repo

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Document is an immutable record of a generated or attached document for a
// request. Documents store only text content — binary archival is a later
// concern, matching the same design choice as chat_attachments.
type Document struct {
	ID          string
	RequestID   string
	NodeID      *string
	Filename    string
	Mime        string
	ContentText string
	CreatedAt   time.Time
}

type DocumentRepo struct {
	pg *pgxpool.Pool
}

func NewDocumentRepo(pg *pgxpool.Pool) *DocumentRepo {
	return &DocumentRepo{pg: pg}
}

// Create inserts a document and populates CreatedAt with the server-generated
// timestamp from the database.
func (r *DocumentRepo) Create(ctx context.Context, d *Document) error {
	err := r.pg.QueryRow(ctx, `
		INSERT INTO documents (id, request_id, node_id, filename, mime, content_text)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at
	`, d.ID, d.RequestID, d.NodeID, d.Filename, d.Mime, d.ContentText).Scan(&d.CreatedAt)
	return err
}

// GetByID returns a single document or ErrNotFound.
func (r *DocumentRepo) GetByID(ctx context.Context, id string) (*Document, error) {
	row := r.pg.QueryRow(ctx, `
		SELECT id, request_id, node_id, filename, mime, content_text, created_at
		FROM documents WHERE id = $1
	`, id)
	var d Document
	if err := row.Scan(&d.ID, &d.RequestID, &d.NodeID, &d.Filename, &d.Mime, &d.ContentText, &d.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &d, nil
}

// ListByRequest returns all documents for a request, oldest first.
func (r *DocumentRepo) ListByRequest(ctx context.Context, requestID string) ([]Document, error) {
	rows, err := r.pg.Query(ctx, `
		SELECT id, request_id, node_id, filename, mime, content_text, created_at
		FROM documents WHERE request_id = $1 ORDER BY created_at ASC
	`, requestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Document
	for rows.Next() {
		var d Document
		if err := rows.Scan(&d.ID, &d.RequestID, &d.NodeID, &d.Filename, &d.Mime, &d.ContentText, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
