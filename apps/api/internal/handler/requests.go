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

	"github.com/ncs26-orchestration/solution/apps/api/internal/agentclient"
	"github.com/ncs26-orchestration/solution/apps/api/internal/middleware"
	"github.com/ncs26-orchestration/solution/apps/api/internal/repo"
)

// RequestsHandler owns the request lifecycle endpoints: create (which
// also plans the workflow graph via the intake agent), list, and
// get-with-graph.
type RequestsHandler struct {
	logger   *slog.Logger
	pg       *pgxpool.Pool
	requests *repo.RequestRepo
	workflow *repo.WorkflowRepo
	agent    *agentclient.Client
}

func NewRequestsHandler(logger *slog.Logger, pg *pgxpool.Pool, agentURL string) *RequestsHandler {
	return &RequestsHandler{
		logger:   logger,
		pg:       pg,
		requests: repo.NewRequestRepo(pg),
		workflow: repo.NewWorkflowRepo(pg),
		agent:    agentclient.New(agentURL),
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
// resolved from users so the UI can show who submitted it.
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

// CreateRequest handles POST /orgs/:orgId/requests. It persists the
// request, asks the intake agent to plan a department workflow (falling
// back to a deterministic plan when the agent is unavailable), persists
// that graph, and moves the request to in_progress.
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
	reqID := fmt.Sprintf("req_%s", randomHex(8))
	saved, err := h.requests.Create(ctx, repo.Request{
		ID:              reqID,
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

	// Plan the workflow graph. A deterministic fallback keeps creation
	// working when the agent service has no LLM key or is unreachable.
	plan, err := h.agent.Intake(ctx, agentclient.IntakeRequest{
		Request: agentclient.IntakeRequestBody{
			Title:       title,
			Description: body.Description,
			Priority:    priority,
		},
		OrgContext: map[string]any{},
	})
	if err != nil {
		h.logger.Warn("intake agent unavailable, using default plan", slog.String("err", err.Error()))
		plan = agentclient.DefaultPlan()
	}

	keyToNodeID := make(map[string]string, len(plan.Nodes))
	nodes := make([]repo.WorkflowNode, 0, len(plan.Nodes))
	for _, pn := range plan.Nodes {
		nodeID := fmt.Sprintf("wn_%s", randomHex(8))
		keyToNodeID[pn.Key] = nodeID
		nodes = append(nodes, repo.WorkflowNode{
			ID:         nodeID,
			RequestID:  reqID,
			Key:        pn.Key,
			Name:       pn.Name,
			AgentType:  pn.AgentType,
			Department: pn.Department,
			Status:     "pending",
		})
	}

	edges := make([]repo.WorkflowEdge, 0, len(plan.Edges))
	for _, pe := range plan.Edges {
		srcID, ok1 := keyToNodeID[pe.From]
		tgtID, ok2 := keyToNodeID[pe.To]
		if !ok1 || !ok2 {
			// The planner referenced a stage key with no node. Skip the
			// edge but log it — a dangling key means a malformed plan.
			h.logger.Warn("create request: dropping edge with unknown node key",
				slog.String("request_id", reqID), slog.String("from", pe.From), slog.String("to", pe.To))
			continue
		}
		edges = append(edges, repo.WorkflowEdge{
			ID:           fmt.Sprintf("we_%s", randomHex(8)),
			RequestID:    reqID,
			SourceNodeID: srcID,
			TargetNodeID: tgtID,
			EdgeType:     pe.EdgeType,
		})
	}

	// Persist the graph and advance the request in one transaction so the
	// request never ends up with a half-written graph.
	tx, err := h.pg.Begin(ctx)
	if err != nil {
		h.logger.Error("create request: begin tx", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if err := h.workflow.InsertGraphTx(ctx, tx, nodes, edges); err != nil {
		h.logger.Error("create request: insert graph", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	if err := h.requests.UpdateStatusProgressTx(ctx, tx, reqID, "in_progress", 0); err != nil {
		h.logger.Error("create request: set in_progress", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	if err := tx.Commit(ctx); err != nil {
		h.logger.Error("create request: commit", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	saved.Status = "in_progress"

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
// planned workflow graph (nodes + edges). agents is reserved for later
// stages and returned empty for now.
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

	nodes, err := h.workflow.ListNodesByRequest(ctx, id)
	if err != nil {
		h.logger.Error("get request: list nodes", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	edges, err := h.workflow.ListEdgesByRequest(ctx, id)
	if err != nil {
		h.logger.Error("get request: list edges", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	nodeList := make([]map[string]any, 0, len(nodes))
	for _, n := range nodes {
		nodeList = append(nodeList, map[string]any{
			"id":               n.ID,
			"key":              n.Key,
			"name":             n.Name,
			"agent_type":       n.AgentType,
			"department":       n.Department,
			"status":           n.Status,
			"description":      n.Description,
			"progress_percent": n.ProgressPercent,
			"status_text":      n.StatusText,
			"started_at":       n.StartedAt,
			"completed_at":     n.CompletedAt,
		})
	}
	edgeList := make([]map[string]any, 0, len(edges))
	for _, e := range edges {
		edgeList = append(edgeList, map[string]any{
			"id":             e.ID,
			"source_node_id": e.SourceNodeID,
			"target_node_id": e.TargetNodeID,
			"edge_type":      e.EdgeType,
		})
	}

	names := h.requesterNames(ctx, []int64{req.RequesterUserID})
	return c.JSON(http.StatusOK, map[string]any{
		"request": toRequestResponse(*req, names[req.RequesterUserID]),
		"nodes":   nodeList,
		"edges":   edgeList,
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
