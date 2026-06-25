package repo

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// WorkflowVersion is one snapshot of a chat's IR. Every extraction
// and every Copilot patch writes one; the chat's latest pointer is
// the freshest row. Full history enables rewind + audit.
type WorkflowVersion struct {
	ID              string
	ChatID          string
	IRJSON          json.RawMessage
	Stage           string // drafting | ready | approved
	DiagnosticsJSON json.RawMessage
	SourceMessageID *string
	CreatedAt       time.Time
}

type WorkflowVersionRepo struct{ pg *pgxpool.Pool }

func NewWorkflowVersionRepo(pg *pgxpool.Pool) *WorkflowVersionRepo {
	return &WorkflowVersionRepo{pg: pg}
}

func (r *WorkflowVersionRepo) Create(ctx context.Context, v WorkflowVersion) error {
	diags := v.DiagnosticsJSON
	if len(diags) == 0 {
		diags = json.RawMessage("[]")
	}
	_, err := r.pg.Exec(ctx, `
		INSERT INTO workflow_versions
		  (id, chat_id, ir_json, stage, diagnostics_json, source_message_id)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, v.ID, v.ChatID, v.IRJSON, v.Stage, diags, v.SourceMessageID)
	return err
}

func (r *WorkflowVersionRepo) Get(ctx context.Context, id string) (*WorkflowVersion, error) {
	row := r.pg.QueryRow(ctx, `
		SELECT id, chat_id, ir_json, stage, diagnostics_json, source_message_id, created_at
		FROM workflow_versions WHERE id = $1
	`, id)
	var v WorkflowVersion
	if err := row.Scan(&v.ID, &v.ChatID, &v.IRJSON, &v.Stage, &v.DiagnosticsJSON, &v.SourceMessageID, &v.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &v, nil
}

// LatestForChat is the canvas-rendering fast path: skip the history
// index, just return the most recent row for this chat. Returns
// ErrNotFound if the chat has never produced an IR.
func (r *WorkflowVersionRepo) LatestForChat(ctx context.Context, chatID string) (*WorkflowVersion, error) {
	row := r.pg.QueryRow(ctx, `
		SELECT id, chat_id, ir_json, stage, diagnostics_json, source_message_id, created_at
		FROM workflow_versions
		WHERE chat_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`, chatID)
	var v WorkflowVersion
	if err := row.Scan(&v.ID, &v.ChatID, &v.IRJSON, &v.Stage, &v.DiagnosticsJSON, &v.SourceMessageID, &v.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &v, nil
}

// SetStage flips one version's stage in place. Used by the Approve
// flow (ready -> approved). Callers must pre-check the transition
// they're making is legal — the repo only enforces that the row
// exists.
func (r *WorkflowVersionRepo) SetStage(ctx context.Context, id, stage string) error {
	tag, err := r.pg.Exec(ctx, `UPDATE workflow_versions SET stage = $2 WHERE id = $1`, id, stage)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListByChat returns the full version history, newest first. Wired
// to an eventual "rewind" feature in the UI.
func (r *WorkflowVersionRepo) ListByChat(ctx context.Context, chatID string) ([]WorkflowVersion, error) {
	rows, err := r.pg.Query(ctx, `
		SELECT id, chat_id, ir_json, stage, diagnostics_json, source_message_id, created_at
		FROM workflow_versions WHERE chat_id = $1
		ORDER BY created_at DESC, id DESC
	`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WorkflowVersion
	for rows.Next() {
		var v WorkflowVersion
		if err := rows.Scan(&v.ID, &v.ChatID, &v.IRJSON, &v.Stage, &v.DiagnosticsJSON, &v.SourceMessageID, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// Fork creates a new workflow version from an existing one, linked to a new or existing chat.
// If targetChatID is empty, creates a new chat with the given title. Returns the new version ID.
func (r *WorkflowVersionRepo) Fork(ctx context.Context, sourceVersionID, targetChatID, newVersionID, title string) (string, error) {
	tx, err := r.pg.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	// Get source version
	var sourceIR json.RawMessage
	var sourceStage string
	var sourceDiags json.RawMessage
	var sourceMsgID *string
	var sourceChatID string
	err = tx.QueryRow(ctx, `
		SELECT chat_id, ir_json, stage, diagnostics_json, source_message_id
		FROM workflow_versions WHERE id = $1
	`, sourceVersionID).Scan(&sourceChatID, &sourceIR, &sourceStage, &sourceDiags, &sourceMsgID)
	if err != nil {
		return "", err
	}

	// If no target chat, create one
	if targetChatID == "" {
		if title == "" {
			title = "Forked workflow"
		}
		targetChatID = "c_fork_" + newVersionID
		_, err = tx.Exec(ctx, `
			INSERT INTO chats (id, project_id, title, summary, latest_workflow_version_id, created_at, updated_at)
			SELECT $1, project_id, $2, 'Forked from ' || $3, NULL, now(), now()
			FROM chats WHERE id = $4
		`, targetChatID, title, sourceVersionID, sourceChatID)
		if err != nil {
			return "", err
		}
	}

	// Insert new version
	_, err = tx.Exec(ctx, `
		INSERT INTO workflow_versions (id, chat_id, ir_json, stage, diagnostics_json, source_message_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, now())
	`, newVersionID, targetChatID, sourceIR, sourceStage, sourceDiags, sourceMsgID)
	if err != nil {
		return "", err
	}

	// Update chat's latest pointer
	_, err = tx.Exec(ctx, `
		UPDATE chats SET latest_workflow_version_id = $1, updated_at = now() WHERE id = $2
	`, newVersionID, targetChatID)
	if err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}

	return targetChatID, nil
}

// Restore creates a new version from an existing one in the same chat.
// This effectively "restores" an old version by copying it to a new version.
func (r *WorkflowVersionRepo) Restore(ctx context.Context, sourceVersionID, newVersionID string) error {
	tx, err := r.pg.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Get source version
	var sourceIR json.RawMessage
	var sourceStage string
	var sourceDiags json.RawMessage
	var sourceMsgID *string
	var chatID string
	err = tx.QueryRow(ctx, `
		SELECT chat_id, ir_json, stage, diagnostics_json, source_message_id
		FROM workflow_versions WHERE id = $1
	`, sourceVersionID).Scan(&chatID, &sourceIR, &sourceStage, &sourceDiags, &sourceMsgID)
	if err != nil {
		return err
	}

	// Insert new version (restored from source)
	_, err = tx.Exec(ctx, `
		INSERT INTO workflow_versions (id, chat_id, ir_json, stage, diagnostics_json, source_message_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, now())
	`, newVersionID, chatID, sourceIR, sourceStage, sourceDiags, sourceMsgID)
	if err != nil {
		return err
	}

	// Update chat's latest pointer
	_, err = tx.Exec(ctx, `
		UPDATE chats SET latest_workflow_version_id = $1, updated_at = now() WHERE id = $2
	`, newVersionID, chatID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}
