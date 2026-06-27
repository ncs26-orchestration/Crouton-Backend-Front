package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/ncs26-orchestration/solution/apps/api/internal/agentclient"
	"github.com/ncs26-orchestration/solution/apps/api/internal/graphspec"
	"github.com/ncs26-orchestration/solution/apps/api/internal/middleware"
	"github.com/ncs26-orchestration/solution/apps/api/internal/repo"
)

// WorkflowsHandler owns the reusable workflow definitions (hiring, leave, etc.)
// and running them. A run is an execution (a request row with kind=workflow_run)
// that reuses the whole request engine, verification, and audit.
type WorkflowsHandler struct {
	logger      *slog.Logger
	pg          *pgxpool.Pool
	defs        *repo.WorkflowDefRepo
	requests    *repo.RequestRepo
	workflow    *repo.WorkflowRepo
	assignments *repo.AssignmentRepo
	audit       *repo.AuditRepo
}

func NewWorkflowsHandler(logger *slog.Logger, pg *pgxpool.Pool) *WorkflowsHandler {
	return &WorkflowsHandler{
		logger:      logger,
		pg:          pg,
		defs:        repo.NewWorkflowDefRepo(pg),
		requests:    repo.NewRequestRepo(pg),
		workflow:    repo.NewWorkflowRepo(pg),
		assignments: repo.NewAssignmentRepo(pg),
		audit:       repo.NewAuditRepo(pg),
	}
}

type workflowBody struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Category    string           `json:"category"`
	Scope       string           `json:"scope"`
	TeamID      *string          `json:"team_id"`
	Nodes       []graphspec.Node `json:"nodes"`
	Edges       []graphspec.Edge `json:"edges"`
}

// canManageWorkflow reports whether the caller may create/edit/delete a workflow
// of the given scope/team: an admin or executor for any, or a lead of the team a
// team-scoped workflow belongs to.
func (h *WorkflowsHandler) canManageWorkflow(c echo.Context, role string, userID int64, scope string, teamID *string) bool {
	if role == "admin" || role == "executor" {
		return true
	}
	if scope == "team" && teamID != nil {
		lead, err := h.assignments.IsTeamLead(c.Request().Context(), *teamID, userID)
		return err == nil && lead
	}
	return false
}

// ListWorkflows handles GET /orgs/:orgId/workflows.
func (h *WorkflowsHandler) ListWorkflows(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	orgID := c.Param("orgId")
	role, err := requireOrgMember(c, h.pg, orgID, claims.UserID)
	if err != nil {
		return handleOrgMemberErr(c, err)
	}
	defs, err := h.defs.List(c.Request().Context(), orgID, claims.UserID, role == "admin")
	if err != nil {
		h.logger.Error("list workflows", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	return c.JSON(http.StatusOK, map[string]any{"workflows": defs})
}

// CreateWorkflow handles POST /orgs/:orgId/workflows.
func (h *WorkflowsHandler) CreateWorkflow(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	orgID := c.Param("orgId")
	role, err := requireOrgMember(c, h.pg, orgID, claims.UserID)
	if err != nil {
		return handleOrgMemberErr(c, err)
	}

	var body workflowBody
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	scope, teamID, verr := normalizeScope(body.Scope, body.TeamID)
	if verr != "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": verr})
	}
	if body.Name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "name is required"})
	}
	if !h.canManageWorkflow(c, role, claims.UserID, scope, teamID) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "you can't create this workflow"})
	}
	if err := graphspec.Validate(body.Nodes, body.Edges); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	nodesRaw, _ := json.Marshal(body.Nodes)
	edgesRaw, _ := json.Marshal(body.Edges)
	category := body.Category
	if category == "" {
		category = "general"
	}
	uid := claims.UserID
	def := repo.WorkflowDef{
		ID:          "wf_" + randomHex(8),
		OrgID:       orgID,
		TeamID:      teamID,
		Scope:       scope,
		Name:        body.Name,
		Description: body.Description,
		Category:    category,
		Nodes:       nodesRaw,
		Edges:       edgesRaw,
		CreatedBy:   &uid,
	}
	if err := h.defs.Create(c.Request().Context(), def); err != nil {
		if isUniqueViolation(err) {
			return c.JSON(http.StatusConflict, map[string]string{"error": "a workflow with that name already exists"})
		}
		h.logger.Error("create workflow", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	return c.JSON(http.StatusCreated, map[string]any{"workflow": def})
}

// UpdateWorkflow handles PATCH /orgs/:orgId/workflows/:id.
func (h *WorkflowsHandler) UpdateWorkflow(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	orgID := c.Param("orgId")
	id := c.Param("id")
	role, err := requireOrgMember(c, h.pg, orgID, claims.UserID)
	if err != nil {
		return handleOrgMemberErr(c, err)
	}
	ctx := c.Request().Context()
	existing, err := h.defs.Get(ctx, orgID, id)
	if errors.Is(err, repo.ErrNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "workflow not found"})
	}
	if err != nil {
		h.logger.Error("update workflow: get", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	var body workflowBody
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	scope, teamID, verr := normalizeScope(body.Scope, body.TeamID)
	if verr != "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": verr})
	}
	if body.Name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "name is required"})
	}
	// Must be allowed on both the existing scope and the new scope.
	if !h.canManageWorkflow(c, role, claims.UserID, existing.Scope, existing.TeamID) ||
		!h.canManageWorkflow(c, role, claims.UserID, scope, teamID) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "you can't edit this workflow"})
	}
	if err := graphspec.Validate(body.Nodes, body.Edges); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	nodesRaw, _ := json.Marshal(body.Nodes)
	edgesRaw, _ := json.Marshal(body.Edges)
	category := body.Category
	if category == "" {
		category = "general"
	}
	upd := repo.WorkflowDef{
		Name:        body.Name,
		Description: body.Description,
		Category:    category,
		Scope:       scope,
		TeamID:      teamID,
		Nodes:       nodesRaw,
		Edges:       edgesRaw,
	}
	if err := h.defs.Update(ctx, orgID, id, upd); err != nil {
		if isUniqueViolation(err) {
			return c.JSON(http.StatusConflict, map[string]string{"error": "a workflow with that name already exists"})
		}
		h.logger.Error("update workflow", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	updated, _ := h.defs.Get(ctx, orgID, id)
	return c.JSON(http.StatusOK, map[string]any{"workflow": updated})
}

// DeleteWorkflow handles DELETE /orgs/:orgId/workflows/:id.
func (h *WorkflowsHandler) DeleteWorkflow(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	orgID := c.Param("orgId")
	id := c.Param("id")
	role, err := requireOrgMember(c, h.pg, orgID, claims.UserID)
	if err != nil {
		return handleOrgMemberErr(c, err)
	}
	ctx := c.Request().Context()
	existing, err := h.defs.Get(ctx, orgID, id)
	if errors.Is(err, repo.ErrNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "workflow not found"})
	}
	if err != nil {
		h.logger.Error("delete workflow: get", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	if !h.canManageWorkflow(c, role, claims.UserID, existing.Scope, existing.TeamID) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "you can't delete this workflow"})
	}
	if err := h.defs.Delete(ctx, orgID, id); err != nil {
		h.logger.Error("delete workflow", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// ListWorkflowRuns handles GET /orgs/:orgId/workflows/:id/runs.
func (h *WorkflowsHandler) ListWorkflowRuns(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	orgID := c.Param("orgId")
	id := c.Param("id")
	if _, err := requireOrgMember(c, h.pg, orgID, claims.UserID); err != nil {
		return handleOrgMemberErr(c, err)
	}
	ctx := c.Request().Context()
	if _, err := h.defs.Get(ctx, orgID, id); errors.Is(err, repo.ErrNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "workflow not found"})
	}
	runs, err := h.defs.ListRuns(ctx, id)
	if err != nil {
		h.logger.Error("list workflow runs", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	return c.JSON(http.StatusOK, map[string]any{"runs": runs})
}

// RunWorkflow handles POST /orgs/:orgId/workflows/:id/run. It instantiates the
// definition's graph as a draft execution (kind=workflow_run) so the user can
// assign verifiers and launch it through the normal draft flow.
func (h *WorkflowsHandler) RunWorkflow(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	orgID := c.Param("orgId")
	id := c.Param("id")
	requesterRole, err := requireOrgMember(c, h.pg, orgID, claims.UserID)
	if err != nil {
		return handleOrgMemberErr(c, err)
	}
	ctx := c.Request().Context()
	def, err := h.defs.Get(ctx, orgID, id)
	if errors.Is(err, repo.ErrNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "workflow not found"})
	}
	if err != nil {
		h.logger.Error("run workflow: get", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	var specNodes []graphspec.Node
	var specEdges []graphspec.Edge
	_ = json.Unmarshal(def.Nodes, &specNodes)
	_ = json.Unmarshal(def.Edges, &specEdges)
	if err := graphspec.Validate(specNodes, specEdges); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "this workflow's steps are invalid: " + err.Error()})
	}

	reqID := "req_" + randomHex(8)
	saved, err := h.requests.Create(ctx, repo.Request{
		ID:              reqID,
		OrgID:           orgID,
		Title:           def.Name,
		Description:     def.Description,
		RequesterUserID: claims.UserID,
		RequesterRole:   requesterRole,
		Priority:        "medium",
		Status:          "submitted",
		Kind:            "workflow_run",
		WorkflowID:      &def.ID,
	})
	if err != nil {
		h.logger.Error("run workflow: create request", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	if def.Category != "" {
		if err := h.requests.SetRequestType(ctx, reqID, def.Category); err == nil {
			saved.RequestType = def.Category
		}
	}

	planNodes := make([]agentclient.PlanNode, len(specNodes))
	for i, n := range specNodes {
		planNodes[i] = agentclient.PlanNode{Key: n.Key, Name: n.Name, AgentType: n.AgentType, Department: n.Department}
	}
	planEdges := make([]agentclient.PlanEdge, len(specEdges))
	for i, e := range specEdges {
		et := e.Type
		if et == "" {
			et = "sequence"
		}
		planEdges[i] = agentclient.PlanEdge{From: e.From, To: e.To, EdgeType: et}
	}
	nodes, edges := buildGraph(h.logger, reqID, planNodes, planEdges)

	tx, err := h.pg.Begin(ctx)
	if err != nil {
		h.logger.Error("run workflow: begin tx", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if err := h.workflow.InsertGraphTx(ctx, tx, nodes, edges); err != nil {
		h.logger.Error("run workflow: insert graph", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	if err := h.requests.UpdateStatusProgressTx(ctx, tx, reqID, "draft", 0); err != nil {
		h.logger.Error("run workflow: set draft", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	if err := tx.Commit(ctx); err != nil {
		h.logger.Error("run workflow: commit", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	saved.Status = "draft"

	if err := h.audit.Append(ctx, repo.AuditEvent{
		ID:        "aev_" + randomHex(8),
		RequestID: reqID,
		Actor:     claims.Name,
		Action:    "workflow.run",
		Reason:    claims.Name + " started the " + def.Name + " workflow",
	}); err != nil {
		h.logger.Error("run workflow: audit", slog.String("err", err.Error()))
	}

	return c.JSON(http.StatusCreated, map[string]any{"request": toRequestResponse(*saved, claims.Name)})
}

// normalizeScope validates the scope/team pairing: a team workflow needs a team,
// a global one must not carry one.
func normalizeScope(scope string, teamID *string) (string, *string, string) {
	switch scope {
	case "", "global":
		return "global", nil, ""
	case "team":
		if teamID == nil || *teamID == "" {
			return "", nil, "a team workflow needs a team"
		}
		return "team", teamID, ""
	default:
		return "", nil, "scope must be global or team"
	}
}
