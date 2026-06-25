package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/Noussour/aup/apps/api/internal/repo"
)

// AttachmentsHandler owns POST /chats/:id/attachments. It accepts a
// multipart upload, forwards the blob to the agent for normalization
// (PDF -> text, TXT passthrough, voice/image stubs), persists the row
// with the returned text_content, and returns a small envelope the UI
// can chip up immediately.
//
// The binary itself is NOT persisted — text_content is the source of
// truth for everything downstream (extractor prompts, re-extraction).
// Archival of originals is a later concern.
type AttachmentsHandler struct {
	logger   *slog.Logger
	chats    *repo.ChatRepo
	attach   *repo.AttachmentRepo
	agentURL string
	http     *http.Client
}

const maxAttachmentBytes = 8 * 1024 * 1024

func NewAttachmentsHandler(logger *slog.Logger, pg *pgxpool.Pool, agentURL string) *AttachmentsHandler {
	return &AttachmentsHandler{
		logger:   logger,
		chats:    repo.NewChatRepo(pg),
		attach:   repo.NewAttachmentRepo(pg),
		agentURL: strings.TrimRight(agentURL, "/"),
		http:     &http.Client{Timeout: 60 * time.Second},
	}
}

type attachmentResponse struct {
	ID            string    `json:"id"`
	ChatID        string    `json:"chat_id"`
	Kind          string    `json:"kind"`
	Filename      string    `json:"filename"`
	Mime          string    `json:"mime"`
	SizeBytes     int64     `json:"size_bytes"`
	TextPreview   string    `json:"text_preview"`
	TextFull      bool      `json:"text_full"`
	CreatedAt     time.Time `json:"created_at"`
}

// Upload handles POST /chats/:id/attachments. Multipart form:
//
//	file: binary blob (required)
//
// The id is generated server-side so the UI can immediately reference
// it in the next outgoing message's `attachment_ids` field.
func (h *AttachmentsHandler) Upload(c echo.Context) error {
	ctx := c.Request().Context()
	chatID := c.Param("id")
	if _, err := h.chats.Get(ctx, chatID); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "chat_not_found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db_error"})
	}

	file, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "file_required"})
	}
	if file.Size > maxAttachmentBytes {
		return c.JSON(http.StatusRequestEntityTooLarge, map[string]string{"error": "file_too_large"})
	}
	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "open_failed"})
	}
	defer src.Close()
	data, err := io.ReadAll(io.LimitReader(src, maxAttachmentBytes+1))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "read_failed"})
	}
	if int64(len(data)) > maxAttachmentBytes {
		return c.JSON(http.StatusRequestEntityTooLarge, map[string]string{"error": "file_too_large"})
	}

	// Forward to the agent for text extraction. The agent returns
	// kind + filename + text_content — we trust its normalization
	// and persist verbatim.
	mimeType := file.Header.Get("content-type")
	parsed, err := h.callAgentExtractText(ctx, file.Filename, mimeType, data)
	if err != nil {
		h.logger.Warn("agent extract-text", slog.String("err", err.Error()))
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "agent_extract_failed", "detail": err.Error()})
	}

	row := repo.Attachment{
		ID:          makeID("att", parsed.Filename),
		ChatID:      chatID,
		Kind:        parsed.Kind,
		Filename:    parsed.Filename,
		Mime:        parsed.Mime,
		SizeBytes:   parsed.SizeBytes,
		TextContent: parsed.TextContent,
	}
	if err := h.attach.Create(ctx, row); err != nil {
		h.logger.Error("persist attachment", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db_error"})
	}

	preview := parsed.TextContent
	truncated := false
	if len(preview) > 400 {
		preview = preview[:400] + "…"
		truncated = true
	}
	return c.JSON(http.StatusCreated, attachmentResponse{
		ID:          row.ID,
		ChatID:      row.ChatID,
		Kind:        row.Kind,
		Filename:    row.Filename,
		Mime:        row.Mime,
		SizeBytes:   row.SizeBytes,
		TextPreview: preview,
		TextFull:    !truncated,
		CreatedAt:   time.Now(),
	})
}

// List returns every attachment for the chat so the composer can
// rehydrate its chip tray after a page reload.
func (h *AttachmentsHandler) List(c echo.Context) error {
	items, err := h.attach.ListByChat(c.Request().Context(), c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db_error"})
	}
	out := make([]attachmentResponse, 0, len(items))
	for _, a := range items {
		preview := a.TextContent
		truncated := false
		if len(preview) > 400 {
			preview = preview[:400] + "…"
			truncated = true
		}
		out = append(out, attachmentResponse{
			ID:          a.ID,
			ChatID:      a.ChatID,
			Kind:        a.Kind,
			Filename:    a.Filename,
			Mime:        a.Mime,
			SizeBytes:   a.SizeBytes,
			TextPreview: preview,
			TextFull:    !truncated,
			CreatedAt:   a.CreatedAt,
		})
	}
	return c.JSON(http.StatusOK, map[string]any{"attachments": out})
}

type agentExtractTextResponse struct {
	Kind        string `json:"kind"`
	Filename    string `json:"filename"`
	Mime        string `json:"mime"`
	SizeBytes   int64  `json:"size_bytes"`
	TextContent string `json:"text_content"`
}

func (h *AttachmentsHandler) callAgentExtractText(ctx context.Context, filename, mime string, data []byte) (*agentExtractTextResponse, error) {
	if h.agentURL == "" {
		return nil, fmt.Errorf("agent URL not configured")
	}
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	// Use the low-level CreatePart so we can set both filename + MIME
	// (CreateFormFile hardcodes application/octet-stream).
	part, err := mw.CreatePart(partHeader(filename, mime))
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(data); err != nil {
		return nil, err
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.agentURL+"/attachments/extract-text", &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", mw.FormDataContentType())
	resp, err := h.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("agent unreachable: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("agent %d: %s", resp.StatusCode, truncateStr(string(raw), 300))
	}
	var parsed agentExtractTextResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode agent response: %w", err)
	}
	return &parsed, nil
}

func partHeader(filename, mime string) textproto.MIMEHeader {
	if mime == "" {
		mime = "application/octet-stream"
	}
	// mime/multipart's CreateFormFile hardcodes octet-stream; we write
	// the header ourselves to preserve the browser-provided MIME so
	// the agent can route PDFs vs TXT without sniffing twice.
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, filename))
	h.Set("Content-Type", mime)
	return h
}
