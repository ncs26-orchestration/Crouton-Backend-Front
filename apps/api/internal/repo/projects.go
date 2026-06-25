package repo

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Project is a client-company workspace. All chats + deploy targets
// hang off one. The operator (single organization using AUP) may
// have many projects, one per client engagement.
type Project struct {
	ID          string
	Name        string
	Description string
	CreatedAt   time.Time
	ArchivedAt  *time.Time
}

type ProjectRepo struct {
	pg *pgxpool.Pool
}

func NewProjectRepo(pg *pgxpool.Pool) *ProjectRepo {
	return &ProjectRepo{pg: pg}
}

// Create inserts a row. ID is caller-supplied so the HTTP handler
// can generate a friendly slug+suffix without a follow-up read.
func (r *ProjectRepo) Create(ctx context.Context, p Project) error {
	_, err := r.pg.Exec(ctx, `
		INSERT INTO projects (id, name, description)
		VALUES ($1, $2, $3)
	`, p.ID, p.Name, p.Description)
	return err
}

func (r *ProjectRepo) Get(ctx context.Context, id string) (*Project, error) {
	row := r.pg.QueryRow(ctx, `
		SELECT id, name, description, created_at, archived_at
		FROM projects
		WHERE id = $1
	`, id)
	var p Project
	if err := row.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.ArchivedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

// List returns all active projects (archived excluded), newest
// first. The operator UI calls this to render the project grid; a
// small filter on archived_at keeps the query cheap with the
// partial index from the migration.
func (r *ProjectRepo) List(ctx context.Context) ([]Project, error) {
	rows, err := r.pg.Query(ctx, `
		SELECT id, name, description, created_at, archived_at
		FROM projects
		WHERE archived_at IS NULL
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.ArchivedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Update changes name + description. Archival is a separate method
// so callers don't accidentally un-archive by passing a zero time.
func (r *ProjectRepo) Update(ctx context.Context, id, name, description string) error {
	tag, err := r.pg.Exec(ctx, `
		UPDATE projects
		SET name = $2, description = $3
		WHERE id = $1
	`, id, name, description)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetOverview returns the project's organisation overview JSON. May
// be empty (nil RawMessage) when no interview has been run yet.
func (r *ProjectRepo) GetOverview(ctx context.Context, id string) (json.RawMessage, error) {
	row := r.pg.QueryRow(ctx, `SELECT overview_json FROM projects WHERE id = $1`, id)
	var raw *json.RawMessage
	if err := row.Scan(&raw); err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}
	return *raw, nil
}

// SetOverview stores the latest organisation overview snapshot. We
// keep one snapshot per project — older versions live in chat_messages
// history (each interview turn references the snapshot it produced).
func (r *ProjectRepo) SetOverview(ctx context.Context, id string, overview json.RawMessage) error {
	tag, err := r.pg.Exec(ctx, `UPDATE projects SET overview_json = $2 WHERE id = $1`, id, overview)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Archive sets archived_at to now. We don't hard-delete so chats +
// messages stay intact for any compliance review the operator
// might need.
func (r *ProjectRepo) Archive(ctx context.Context, id string) error {
	tag, err := r.pg.Exec(ctx, `
		UPDATE projects
		SET archived_at = NOW()
		WHERE id = $1 AND archived_at IS NULL
	`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
