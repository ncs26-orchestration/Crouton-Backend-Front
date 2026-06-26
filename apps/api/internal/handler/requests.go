package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/ncs26-orchestration/solution/apps/api/internal/middleware"
	"github.com/ncs26-orchestration/solution/apps/api/internal/repo"
)

// RequestsHandler owns the request lifecycle endpoints: create, list,
// and get-with-graph. F1 ships create/list/get; the planner and engine
// (F2/F3) hang the workflow graph off these later.
type RequestsHandler struct {
	logger   *slog.Logger
	pg       *pgxpool.Pool
	requests *repo.RequestRepo
}

func NewRequestsHandler(logger *slog.Logger, pg *pgxpool.Pool) *RequestsHandler {
	return &RequestsHandler{
		logger:   logger,
		pg:       pg,
		requests: repo.NewRequestRepo(pg),
	}
}

var validPriorities = map[string]bool{"low": true, "medium": true, "high": true, "urgent": true}

// requestResponse is the wire shape for a request. requester_name is
// resolved from users so the UI table can show who submitted it.
type requestResponse struct {
	ID                  string     `json:"id"`
	OrgID               string     `json:"org_id"`
	Title               string     `json:"title"`
	Description         string     `json:"description"`
	RequesterUserID     int64      `json:"requester_user_id"`
	RequesterName       string     `json:"requester_name"`
	Priority            string     `json:"priority"`
	Status              string     `json:"status"`
	Progress            int        `json:"progress"`
	EstimatedCompletion *time.Time `json:"estimated_completion"`
	CreatedAt           time.Time  `json:"created_at"`
}

func toRequestResponse(r repo.Request, requesterName string) requestResponse {
	return requestResponse{
		ID:                  r.ID,
		OrgID:               r.OrgID,
		Title:               r.Title,
		Description:         r.Description,
		RequesterUserID:     r.RequesterUserID,
		RequesterName:       requesterName,
		Priority:            r.Priority,
		Status:              r.Status,
		Progress:            r.Progress,
		EstimatedCompletion: r.EstimatedCompletion,
		CreatedAt:           r.CreatedAt,
	}
}

// CreateRequest handles POST /orgs/:orgId/requests.
func (h *RequestsHandler) CreateRequest(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	orgID := c.Param("orgId")

	if _, err := requireOrgMember(c, h.pg, orgID, claims.UserID); err != nil {
		return handleOrgMemberErr(c, err)
	}

	var body struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Priority    string `json:"priority"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	if body.Title == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "title is required"})
	}
	if body.Priority == "" {
		body.Priority = "medium"
	}
	if !validPriorities[body.Priority] {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "priority must be low, medium, high, or urgent"})
	}

	ctx := c.Request().Context()
	req := repo.Request{
		ID:              fmt.Sprintf("req_%s", randomHex(8)),
		OrgID:           orgID,
		Title:           body.Title,
		Description:     body.Description,
		RequesterUserID: claims.UserID,
		Priority:        body.Priority,
		Status:          "submitted",
	}
	if err := h.requests.Create(ctx, req); err != nil {
		h.logger.Error("create request: insert", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	// Re-read so the response carries DB defaults (status, progress, created_at).
	saved, err := h.requests.GetByID(ctx, req.ID)
	if err != nil {
		h.logger.Error("create request: reload", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	names := h.requesterNames(ctx, []int64{saved.RequesterUserID})
	return c.JSON(http.StatusCreated, map[string]any{"request": toRequestResponse(*saved, names[saved.RequesterUserID])})
}

// ListRequests handles GET /orgs/:orgId/requests.
func (h *RequestsHandler) ListRequests(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	orgID := c.Param("orgId")

	if _, err := requireOrgMember(c, h.pg, orgID, claims.UserID); err != nil {
		return handleOrgMemberErr(c, err)
	}

	ctx := c.Request().Context()
	rows, err := h.requests.ListByOrg(ctx, orgID)
	if err != nil {
		h.logger.Error("list requests: query", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.RequesterUserID)
	}
	names := h.requesterNames(ctx, ids)

	out := make([]requestResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, toRequestResponse(r, names[r.RequesterUserID]))
	}
	return c.JSON(http.StatusOK, map[string]any{"requests": out})
}

// GetRequest handles GET /requests/:id, returning the request plus its
// (currently empty) workflow graph. The planner fills nodes/edges/agents
// in F2; F1 returns empty arrays so the detail shell renders.
func (h *RequestsHandler) GetRequest(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	id := c.Param("id")

	ctx := c.Request().Context()
	req, err := h.requests.GetByID(ctx, id)
	if errors.Is(err, repo.ErrNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "request not found"})
	}
	if err != nil {
		h.logger.Error("get request: query", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	// Authorize via the request's org membership.
	if _, err := requireOrgMember(c, h.pg, req.OrgID, claims.UserID); err != nil {
		return handleOrgMemberErr(c, err)
	}

	names := h.requesterNames(ctx, []int64{req.RequesterUserID})
	return c.JSON(http.StatusOK, map[string]any{
		"request": toRequestResponse(*req, names[req.RequesterUserID]),
		"nodes":   []any{},
		"edges":   []any{},
		"agents":  []any{},
	})
}

// requesterNames resolves user ids to display names in one query. A
// missing id simply maps to the empty string.
func (h *RequestsHandler) requesterNames(ctx context.Context, ids []int64) map[int64]string {
	out := make(map[int64]string, len(ids))
	if len(ids) == 0 {
		return out
	}
	rows, err := h.pg.Query(ctx, `SELECT id, name FROM users WHERE id = ANY($1)`, ids)
	if err != nil {
		h.logger.Error("resolve requester names", slog.String("err", err.Error()))
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id   int64
			name string
		)
		if err := rows.Scan(&id, &name); err != nil {
			h.logger.Error("resolve requester names: scan", slog.String("err", err.Error()))
			return out
		}
		out[id] = name
	}
	return out
}
