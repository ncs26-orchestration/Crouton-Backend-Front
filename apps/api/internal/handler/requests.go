package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
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

const (
	maxTitleLen       = 200
	maxDescriptionLen = 5000
	// requestListLimit bounds how many requests a list call returns.
	requestListLimit = 200
)

// validateRequestInput normalizes and validates create-request input. It
// trims the title, defaults an empty priority to "medium", and enforces
// length caps. It returns the normalized title and priority, plus a
// non-empty message describing the first failure (empty means valid).
// Pure so it can be table-tested without a DB or HTTP context.
func validateRequestInput(title, description, priority string) (normTitle, normPriority, errMsg string) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", "", "title is required"
	}
	if len(title) > maxTitleLen {
		return "", "", fmt.Sprintf("title must be at most %d characters", maxTitleLen)
	}
	if len(description) > maxDescriptionLen {
		return "", "", fmt.Sprintf("description must be at most %d characters", maxDescriptionLen)
	}
	if priority == "" {
		priority = "medium"
	}
	if !validPriorities[priority] {
		return "", "", "priority must be low, medium, high, or urgent"
	}
	return title, priority, ""
}

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
	title, priority, verr := validateRequestInput(body.Title, body.Description, body.Priority)
	if verr != "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": verr})
	}

	ctx := c.Request().Context()
	saved, err := h.requests.Create(ctx, repo.Request{
		ID:              fmt.Sprintf("req_%s", randomHex(8)),
		OrgID:           orgID,
		Title:           title,
		Description:     body.Description,
		RequesterUserID: claims.UserID,
		Priority:        priority,
		Status:          "submitted",
	})
	if err != nil {
		h.logger.Error("create request: insert", slog.String("err", err.Error()))
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
	rows, err := h.requests.ListByOrg(ctx, orgID, requestListLimit)
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

	// Authorize via the request's org membership. A non-member is told
	// "not found" rather than "forbidden" so the endpoint can't be used
	// to probe whether a request id exists in some other org.
	if _, err := requireOrgMember(c, h.pg, req.OrgID, claims.UserID); err != nil {
		var he *echo.HTTPError
		if errors.As(err, &he) && he.Code == http.StatusForbidden {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "request not found"})
		}
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
