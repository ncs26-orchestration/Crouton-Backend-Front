package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/ncs26-orchestration/solution/apps/api/internal/agentclient"
	"github.com/ncs26-orchestration/solution/apps/api/internal/middleware"
	"github.com/ncs26-orchestration/solution/apps/api/internal/repo"
)

// RequestsHandler handles request submission, listing, and detail with graph.
type RequestsHandler struct {
	logger   *slog.Logger
	db       *pgxpool.Pool
	requests *repo.RequestRepo
	workflow *repo.WorkflowRepo
	agent    *agentclient.Client
}

// NewRequestsHandler constructs a RequestsHandler.
func NewRequestsHandler(logger *slog.Logger, db *pgxpool.Pool, agentURL string) *RequestsHandler {
	return &RequestsHandler{
		logger:   logger,
		db:       db,
		requests: repo.NewRequestRepo(db),
		workflow: repo.NewWorkflowRepo(db),
		agent:    agentclient.New(agentURL),
	}
}

// CreateRequest handles POST /orgs/:orgId/requests.
func (h *RequestsHandler) CreateRequest(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	orgID := c.Param("orgId")
	if _, err := requireOrgMember(c, h.db, orgID, claims.UserID); err != nil {
		return err
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

	reqID := fmt.Sprintf("req_%s", randomHex(8))
	ctx := c.Request().Context()

	req := repo.Request{
		ID:              reqID,
		OrgID:           orgID,
		Title:           body.Title,
		Description:     body.Description,
		RequesterUserID: claims.UserID,
		Priority:        body.Priority,
		Status:          "submitted",
		Progress:        0,
	}
	if err := h.requests.Create(ctx, req); err != nil {
		h.logger.Error("create request", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	// Call intake agent to plan the workflow graph.
	plan, err := h.agent.Intake(ctx, agentclient.IntakeRequest{
		Request: agentclient.IntakeRequestBody{
			Title:       body.Title,
			Description: body.Description,
			Priority:    body.Priority,
		},
		OrgContext: map[string]any{},
	})
	if err != nil {
		h.logger.Warn("intake agent unavailable, using default plan", slog.String("err", err.Error()))
		plan = agentclient.DefaultPlan()
	}

	// Build a key→nodeID map for edge resolution.
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

	if err := h.workflow.InsertNodes(ctx, nodes); err != nil {
		h.logger.Error("insert workflow nodes", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	edges := make([]repo.WorkflowEdge, 0, len(plan.Edges))
	for _, pe := range plan.Edges {
		srcID, ok1 := keyToNodeID[pe.From]
		tgtID, ok2 := keyToNodeID[pe.To]
		if !ok1 || !ok2 {
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

	if err := h.workflow.InsertEdges(ctx, edges); err != nil {
		h.logger.Error("insert workflow edges", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	// Update status to in_progress now that we have a plan.
	_ = h.requests.UpdateStatusProgress(ctx, reqID, "in_progress", 0)

	return c.JSON(http.StatusCreated, map[string]any{
		"request": map[string]any{
			"id":          reqID,
			"org_id":      orgID,
			"title":       body.Title,
			"description": body.Description,
			"priority":    body.Priority,
			"status":      "in_progress",
			"progress":    0,
			"created_at":  req.CreatedAt,
		},
	})
}

// ListRequests handles GET /orgs/:orgId/requests.
func (h *RequestsHandler) ListRequests(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	orgID := c.Param("orgId")
	if _, err := requireOrgMember(c, h.db, orgID, claims.UserID); err != nil {
		return err
	}

	ctx := c.Request().Context()
	reqs, err := h.requests.ListByOrg(ctx, orgID)
	if err != nil {
		h.logger.Error("list requests", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	out := make([]map[string]any, 0, len(reqs))
	for _, r := range reqs {
		// Fetch requester name.
		var requesterName string
		_ = h.db.QueryRow(ctx, `SELECT name FROM users WHERE id = $1`, r.RequesterUserID).Scan(&requesterName)

		out = append(out, map[string]any{
			"id":          r.ID,
			"title":       r.Title,
			"description": r.Description,
			"requester":   requesterName,
			"priority":    r.Priority,
			"status":      r.Status,
			"progress":    r.Progress,
			"created_at":  r.CreatedAt,
		})
	}

	return c.JSON(http.StatusOK, map[string]any{"requests": out})
}

// GetRequest handles GET /requests/:id — returns the full graph.
func (h *RequestsHandler) GetRequest(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	id := c.Param("id")
	ctx := c.Request().Context()

	req, err := h.requests.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "request not found"})
		}
		h.logger.Error("get request", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	// Verify the caller belongs to the request's org.
	if _, err := requireOrgMember(c, h.db, req.OrgID, claims.UserID); err != nil {
		return err
	}

	// Fetch requester name.
	var requesterName string
	_ = h.db.QueryRow(ctx, `SELECT name FROM users WHERE id = $1`, req.RequesterUserID).Scan(&requesterName)

	nodes, err := h.workflow.ListNodesByRequest(ctx, id)
	if err != nil {
		h.logger.Error("list nodes", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	edges, err := h.workflow.ListEdgesByRequest(ctx, id)
	if err != nil {
		h.logger.Error("list edges", slog.String("err", err.Error()))
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

	return c.JSON(http.StatusOK, map[string]any{
		"request": map[string]any{
			"id":          req.ID,
			"org_id":      req.OrgID,
			"title":       req.Title,
			"description": req.Description,
			"requester":   requesterName,
			"priority":    req.Priority,
			"status":      req.Status,
			"progress":    req.Progress,
			"created_at":  req.CreatedAt,
		},
		"nodes": nodeList,
		"edges": edgeList,
	})
}
