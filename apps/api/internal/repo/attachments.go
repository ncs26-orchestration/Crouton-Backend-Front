package repo

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Attachment is one uploaded file feeding the chat's context. We
// don't retain the binary payload in v0.1 — only the extracted
// text, so Gemini has something to read and the UI has something to
// echo back. Large archival of originals is a later concern.
type Attachment struct {
	ID          string
	ChatID      string
	MessageID   *string // filled when attached to a sent message
	Kind        string  // document | voice | image
	Filename    string
	Mime        string
	SizeBytes   int64
	TextContent string
	CreatedAt   time.Time
}

type AttachmentRepo struct{ pg *pgxpool.Pool }

func NewAttachmentRepo(pg *pgxpool.Pool) *AttachmentRepo { return &AttachmentRepo{pg: pg} }

func (r *AttachmentRepo) Create(ctx context.Context, a Attachment) error {
	_, err := r.pg.Exec(ctx, `
		INSERT INTO chat_attachments
		  (id, chat_id, message_id, kind, filename, mime, size_bytes, text_content)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, a.ID, a.ChatID, a.MessageID, a.Kind, a.Filename, a.Mime, a.SizeBytes, a.TextContent)
	return err
}

func (r *AttachmentRepo) Get(ctx context.Context, id string) (*Attachment, error) {
	row := r.pg.QueryRow(ctx, `
		SELECT id, chat_id, message_id, kind, filename, mime, size_bytes, text_content, created_at
		FROM chat_attachments WHERE id = $1
	`, id)
	var a Attachment
	if err := row.Scan(&a.ID, &a.ChatID, &a.MessageID, &a.Kind, &a.Filename, &a.Mime, &a.SizeBytes, &a.TextContent, &a.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

// ListByChat returns every attachment for a chat, oldest first so
// the extractor sees them in upload order (which typically matches
// narrative order too).
func (r *AttachmentRepo) ListByChat(ctx context.Context, chatID string) ([]Attachment, error) {
	rows, err := r.pg.Query(ctx, `
		SELECT id, chat_id, message_id, kind, filename, mime, size_bytes, text_content, created_at
		FROM chat_attachments WHERE chat_id = $1 ORDER BY created_at ASC
	`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Attachment
	for rows.Next() {
		var a Attachment
		if err := rows.Scan(&a.ID, &a.ChatID, &a.MessageID, &a.Kind, &a.Filename, &a.Mime, &a.SizeBytes, &a.TextContent, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// BindToMessage is called when a message is sent that references
// previously uploaded orphan attachments. The call is idempotent so
// sending twice is safe.
func (r *AttachmentRepo) BindToMessage(ctx context.Context, attachmentID, messageID string) error {
	tag, err := r.pg.Exec(ctx, `
		UPDATE chat_attachments SET message_id = $2 WHERE id = $1
	`, attachmentID, messageID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
