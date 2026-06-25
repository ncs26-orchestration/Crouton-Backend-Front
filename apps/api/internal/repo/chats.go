package repo

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Chat is one design-conversation inside a project. Two kinds today:
//
//	"workflow"  — the standard workflow-authoring loop. The canvas
//	              renders the latest workflow_version's IR.
//	"interview" — onboarding interview. The right panel renders the
//	              project's organisation overview instead.
//
// The kind is fixed at creation time. A project may have many
// workflow chats but at most one interview chat (DB unique index
// chats_one_interview_per_project enforces this).
type Chat struct {
	ID                      string
	ProjectID               string
	Kind                    string // workflow | interview
	Title                   string
	Summary                 string
	LatestWorkflowVersionID *string
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

type ChatRepo struct{ pg *pgxpool.Pool }

func NewChatRepo(pg *pgxpool.Pool) *ChatRepo { return &ChatRepo{pg: pg} }

func (r *ChatRepo) Create(ctx context.Context, c Chat) error {
	kind := c.Kind
	if kind == "" {
		kind = "workflow"
	}
	_, err := r.pg.Exec(ctx, `
		INSERT INTO chats (id, project_id, kind, title, summary)
		VALUES ($1, $2, $3, $4, $5)
	`, c.ID, c.ProjectID, kind, c.Title, c.Summary)
	return err
}

func (r *ChatRepo) Get(ctx context.Context, id string) (*Chat, error) {
	row := r.pg.QueryRow(ctx, `
		SELECT id, project_id, kind, title, summary, latest_workflow_version_id, created_at, updated_at
		FROM chats
		WHERE id = $1
	`, id)
	var c Chat
	if err := row.Scan(&c.ID, &c.ProjectID, &c.Kind, &c.Title, &c.Summary, &c.LatestWorkflowVersionID, &c.CreatedAt, &c.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

// ListByProject returns chats for one project, most-recently-updated
// first — matches the left-rail rendering order.
func (r *ChatRepo) ListByProject(ctx context.Context, projectID string) ([]Chat, error) {
	rows, err := r.pg.Query(ctx, `
		SELECT id, project_id, kind, title, summary, latest_workflow_version_id, created_at, updated_at
		FROM chats
		WHERE project_id = $1
		ORDER BY updated_at DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Chat
	for rows.Next() {
		var c Chat
		if err := rows.Scan(&c.ID, &c.ProjectID, &c.Kind, &c.Title, &c.Summary, &c.LatestWorkflowVersionID, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// FindInterview returns the project's interview chat, or ErrNotFound
// if no interview has been started yet. There can be at most one
// (enforced by the unique partial index in migrations).
func (r *ChatRepo) FindInterview(ctx context.Context, projectID string) (*Chat, error) {
	row := r.pg.QueryRow(ctx, `
		SELECT id, project_id, kind, title, summary, latest_workflow_version_id, created_at, updated_at
		FROM chats
		WHERE project_id = $1 AND kind = 'interview'
		LIMIT 1
	`, projectID)
	var c Chat
	if err := row.Scan(&c.ID, &c.ProjectID, &c.Kind, &c.Title, &c.Summary, &c.LatestWorkflowVersionID, &c.CreatedAt, &c.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

// Rename updates the user-visible title.
func (r *ChatRepo) Rename(ctx context.Context, id, title string) error {
	tag, err := r.pg.Exec(ctx, `
		UPDATE chats SET title = $2, updated_at = NOW() WHERE id = $1
	`, id, title)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetLatestWorkflowVersion is called after every successful
// extraction or Copilot patch so the chat's canvas pointer stays
// in sync with the newest IR snapshot.
func (r *ChatRepo) SetLatestWorkflowVersion(ctx context.Context, chatID, versionID string) error {
	tag, err := r.pg.Exec(ctx, `
		UPDATE chats SET latest_workflow_version_id = $2, updated_at = NOW() WHERE id = $1
	`, chatID, versionID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *ChatRepo) Delete(ctx context.Context, id string) error {
	tag, err := r.pg.Exec(ctx, `DELETE FROM chats WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// --- messages ---

// ChatMessage mirrors one row from chat_messages. Body is stored as
// a raw JSONB blob — the shape varies by role (user text, assistant
// text + optional ir-diff refs, system event markers) and living
// inside JSON means we can evolve without migrations.
type ChatMessage struct {
	ID        string
	ChatID    string
	Role      string          // user | assistant | system
	Body      json.RawMessage // flexible envelope
	CreatedAt time.Time
}

type ChatMessageRepo struct{ pg *pgxpool.Pool }

func NewChatMessageRepo(pg *pgxpool.Pool) *ChatMessageRepo { return &ChatMessageRepo{pg: pg} }

func (r *ChatMessageRepo) Append(ctx context.Context, m ChatMessage) error {
	_, err := r.pg.Exec(ctx, `
		INSERT INTO chat_messages (id, chat_id, role, body)
		VALUES ($1, $2, $3, $4)
	`, m.ID, m.ChatID, m.Role, m.Body)
	if err != nil {
		return err
	}
	// Touch the chat so the rail sort order updates.
	_, err = r.pg.Exec(ctx, `UPDATE chats SET updated_at = NOW() WHERE id = $1`, m.ChatID)
	return err
}

func (r *ChatMessageRepo) ListByChat(ctx context.Context, chatID string) ([]ChatMessage, error) {
	rows, err := r.pg.Query(ctx, `
		SELECT id, chat_id, role, body, created_at
		FROM chat_messages
		WHERE chat_id = $1
		ORDER BY created_at ASC, id ASC
	`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ChatMessage
	for rows.Next() {
		var m ChatMessage
		if err := rows.Scan(&m.ID, &m.ChatID, &m.Role, &m.Body, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Recent returns the last N messages oldest-first for the extractor
// prompt. A bounded window keeps Gemini's context stable.
func (r *ChatMessageRepo) Recent(ctx context.Context, chatID string, limit int) ([]ChatMessage, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.pg.Query(ctx, `
		SELECT * FROM (
			SELECT id, chat_id, role, body, created_at
			FROM chat_messages
			WHERE chat_id = $1
			ORDER BY created_at DESC, id DESC
			LIMIT $2
		) t
		ORDER BY created_at ASC, id ASC
	`, chatID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ChatMessage
	for rows.Next() {
		var m ChatMessage
		if err := rows.Scan(&m.ID, &m.ChatID, &m.Role, &m.Body, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
