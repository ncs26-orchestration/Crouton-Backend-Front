package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/ncs26-orchestration/solution/apps/api/internal/middleware"
	"github.com/ncs26-orchestration/solution/apps/api/internal/repo"
)

// DocumentsHandler owns the document lifecycle endpoints: list, download, and
// manual upload. Generated documents are created by the orchestrator engine;
// this handler is for retrieval and external attachment.
type DocumentsHandler struct {
	logger    *slog.Logger
	pg        *pgxpool.Pool
	documents *repo.DocumentRepo
	requests  *repo.RequestRepo
}

func NewDocumentsHandler(logger *slog.Logger, pg *pgxpool.Pool) *DocumentsHandler {
	return &DocumentsHandler{
		logger:    logger,
		pg:        pg,
		documents: repo.NewDocumentRepo(pg),
		requests:  repo.NewRequestRepo(pg),
	}
}

// documentResponse is the JSON shape for a document.
type documentResponse struct {
	ID        string `json:"id"`
	RequestID string `json:"request_id"`
	NodeID    string `json:"node_id,omitempty"`
	Filename  string `json:"filename"`
	Mime      string `json:"mime"`
	Size      int    `json:"size"`
	CreatedAt string `json:"created_at"`
}

func toDocumentResponse(d repo.Document) documentResponse {
	r := documentResponse{
		ID:        d.ID,
		RequestID: d.RequestID,
		Filename:  d.Filename,
		Mime:      d.Mime,
		Size:      len(d.ContentText),
		CreatedAt: d.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if d.NodeID != nil {
		r.NodeID = *d.NodeID
	}
	return r
}

// ListDocuments handles GET /requests/:id/documents.
func (h *DocumentsHandler) ListDocuments(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	requestID := c.Param("id")

	ctx := c.Request().Context()
	req, err := h.requests.GetByID(ctx, requestID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "request not found"})
		}
		h.logger.Error("list documents: get request", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	if _, err := requireOrgMember(c, h.pg, req.OrgID, claims.UserID); err != nil {
		var he *echo.HTTPError
		if errors.As(err, &he) && he.Code == http.StatusForbidden {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "request not found"})
		}
		return handleOrgMemberErr(c, err)
	}

	docs, err := h.documents.ListByRequest(ctx, requestID)
	if err != nil {
		h.logger.Error("list documents: query", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	out := make([]documentResponse, 0, len(docs))
	for _, d := range docs {
		out = append(out, toDocumentResponse(d))
	}
	return c.JSON(http.StatusOK, map[string]any{"documents": out})
}

// DownloadDocument handles GET /documents/:id/download.
// It serves the document's content_text as a file download with the original
// filename and MIME type, enabling both browser and mobile app consumption.
func (h *DocumentsHandler) DownloadDocument(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	docID := c.Param("id")

	ctx := c.Request().Context()
	doc, err := h.documents.GetByID(ctx, docID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "document not found"})
		}
		h.logger.Error("download document: query", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	// Authorize: user must be a member of the document's request org.
	req, err := h.requests.GetByID(ctx, doc.RequestID)
	if err != nil {
		h.logger.Error("download document: get request", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	if _, err := requireOrgMember(c, h.pg, req.OrgID, claims.UserID); err != nil {
		var he *echo.HTTPError
		if errors.As(err, &he) && he.Code == http.StatusForbidden {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "document not found"})
		}
		return handleOrgMemberErr(c, err)
	}

	mime := doc.Mime
	if mime == "" {
		mime = "text/plain"
	}
	c.Response().Header().Set("Content-Type", mime)
	c.Response().Header().Set("Content-Disposition", "attachment; filename=\""+doc.Filename+"\"")
	return c.String(http.StatusOK, doc.ContentText)
}

// UploadDocumentRequest is the JSON body for POST /requests/:id/documents.
type UploadDocumentRequest struct {
	Filename    string `json:"filename"`
	Mime        string `json:"mime"`
	ContentText string `json:"content_text"`
}

// UploadDocument handles POST /requests/:id/documents.
// Attaches a document (text content) to a request.
func (h *DocumentsHandler) UploadDocument(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	requestID := c.Param("id")

	var body UploadDocumentRequest
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	if body.Filename == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "filename is required"})
	}

	ctx := c.Request().Context()
	req, err := h.requests.GetByID(ctx, requestID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "request not found"})
		}
		h.logger.Error("upload document: get request", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	if _, err := requireOrgMember(c, h.pg, req.OrgID, claims.UserID); err != nil {
		var he *echo.HTTPError
		if errors.As(err, &he) && he.Code == http.StatusForbidden {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "request not found"})
		}
		return handleOrgMemberErr(c, err)
	}

	mime := body.Mime
	if mime == "" {
		mime = "text/plain"
	}

	doc := &repo.Document{
		ID:          "doc_" + makeID("", requestID),
		RequestID:   requestID,
		Filename:    body.Filename,
		Mime:        mime,
		ContentText: body.ContentText,
	}
	if err := h.documents.Create(ctx, doc); err != nil {
		h.logger.Error("upload document: create", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	return c.JSON(http.StatusCreated, map[string]any{"document": toDocumentResponse(*doc)})
}
