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
	"github.com/ncs26-orchestration/solution/apps/api/internal/orchestrator"
	"github.com/ncs26-orchestration/solution/apps/api/internal/repo"
)

// RequestsHandler owns the request lifecycle endpoints: create (which
// plans the workflow graph and launches the orchestration engine), list,
// get-with-graph, node detail, and audit reads.
type RequestsHandler struct {
	logger   *slog.Logger
	pg       *pgxpool.Pool
	requests *repo.RequestRepo
	workflow *repo.WorkflowRepo
	deps     *repo.DependencyRepo
	audit    *repo.AuditRepo
	agent    *agentclient.Client
	engine   *orchestrator.Engine
}

func NewRequestsHandler(logger *slog.Logger, pg *pgxpool.Pool, agent *agentclient.Client, engine *orchestrator.Engine) *RequestsHandler {
	return &RequestsHandler{
		logger:   logger,
		pg:       pg,
		requests: repo.NewRequestRepo(pg),
		workflow: repo.NewWorkflowRepo(pg),
		deps:     repo.NewDependencyRepo(pg),
		audit:    repo.NewAuditRepo(pg),
		agent:    agent,
		engine:   engine,
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
	ID                  string         `json:"id"`
	OrgID               string         `json:"org_id"`
	Title               string         `json:"title"`
	Description         string         `json:"description"`
	RequesterUserID     int64          `json:"requester_user_id"`
	RequesterName       string         `json:"requester_name"`
	RequesterRole       string         `json:"requester_role"`
	RequestType         string         `json:"request_type"`
	Details             map[string]any `json:"details"`
	Priority            string         `json:"priority"`
	Status              string         `json:"status"`
	Progress            int            `json:"progress"`
	EstimatedCompletion *time.Time     `json:"estimated_completion"`
	CreatedAt           time.Time      `json:"created_at"`
}

func toRequestResponse(r repo.Request, requesterName string) requestResponse {
	return requestResponse{
		ID:                  r.ID,
		OrgID:               r.OrgID,
		Title:               r.Title,
		Description:         r.Description,
		RequesterUserID:     r.RequesterUserID,
		RequesterName:       requesterName,
		RequesterRole:       r.RequesterRole,
		RequestType:         r.RequestType,
		Details:             r.Details,
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
// standardAgentTypes are the agent types the intake planner already knows from
// its built-in catalog. Any other agent in the org is a custom department the
// org created, which we surface to the planner so it can route to it.
var standardAgentTypes = map[string]bool{
	"finance": true, "legal": true, "it": true, "hr": true,
	"ops": true, "planning": true, "approval": true,
}

// intakeOrgContext builds the org_context for the intake planner: the custom
// departments this org has created (beyond the standard catalog), each with a
// node key the planner may use. Errors are non-fatal — intake just plans from
// the standard catalog. This is what lets a department created in the app
// actually take part in a workflow.
func (h *RequestsHandler) intakeOrgContext(ctx context.Context, orgID string) map[string]any {
	rows, err := h.pg.Query(ctx, `
		SELECT a.agent_type, COALESCE(t.name, '') AS department
		FROM agents a LEFT JOIN teams t ON t.id = a.team_id
		WHERE a.org_id = $1
		ORDER BY a.created_at ASC
	`, orgID)
	if err != nil {
		h.logger.Warn("intake org context: query agents", slog.String("err", err.Error()))
		return map[string]any{}
	}
	defer rows.Close()

	var extra []map[string]string
	for rows.Next() {
		var agentType, department string
		if err := rows.Scan(&agentType, &department); err != nil {
			h.logger.Warn("intake org context: scan", slog.String("err", err.Error()))
			return map[string]any{}
		}
		if standardAgentTypes[agentType] || department == "" {
			continue
		}
		extra = append(extra, map[string]string{
			"key":        agentType + "_review",
			"agent_type": agentType,
			"department": department,
		})
	}
	if len(extra) == 0 {
		return map[string]any{}
	}
	return map[string]any{"additional_departments": extra}
}

func (h *RequestsHandler) CreateRequest(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	orgID := c.Param("orgId")

	requesterRole, err := requireOrgMember(c, h.pg, orgID, claims.UserID)
	if err != nil {
		return handleOrgMemberErr(c, err)
	}

	var body struct {
		Title       string         `json:"title"`
		Description string         `json:"description"`
		Priority    string         `json:"priority"`
		Category    string         `json:"category"`
		Details     map[string]any `json:"details"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	title, priority, verr := validateRequestInput(body.Title, body.Description, body.Priority)
	if verr != "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": verr})
	}
	// The picked category is a soft hint for the request type; intake may refine
	// it. An empty/General category leaves it for intake to classify.
	requestTypeHint := slugify(body.Category)
	if requestTypeHint == "" || requestTypeHint == "general" {
		requestTypeHint = "general"
	}

	ctx := c.Request().Context()
	reqID := fmt.Sprintf("req_%s", randomHex(8))
	saved, err := h.requests.Create(ctx, repo.Request{
		ID:              reqID,
		OrgID:           orgID,
		Title:           title,
		Description:     body.Description,
		RequesterUserID: claims.UserID,
		RequesterRole:   requesterRole,
		Details:         body.Details,
		Priority:        priority,
		Status:          "submitted",
	})
	if err != nil {
		h.logger.Error("create request: insert", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	if requestTypeHint != "general" {
		if err := h.requests.SetRequestType(ctx, reqID, requestTypeHint); err == nil {
			saved.RequestType = requestTypeHint
		}
	}

	// Plan the workflow graph. A deterministic fallback keeps creation
	// working when the agent service has no LLM key or is unreachable.
	plan, err := h.agent.Intake(ctx, agentclient.IntakeRequest{
		Request: agentclient.IntakeRequestBody{
			Title:       title,
			Description: body.Description,
			Priority:    priority,
			Details:     body.Details,
		},
		OrgContext: h.intakeOrgContext(ctx, orgID),
	})
	if err != nil {
		h.logger.Warn("intake agent unavailable, using default plan", slog.String("err", err.Error()))
		plan = agentclient.DefaultPlan()
	}
	if plan.RequestType != "" {
		if err := h.requests.SetRequestType(ctx, reqID, plan.RequestType); err != nil {
			h.logger.Warn("create request: set request_type", slog.String("err", err.Error()))
		} else {
			saved.RequestType = plan.RequestType
		}
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

	if err := h.audit.Append(ctx, repo.AuditEvent{
		ID:        "aev_" + randomHex(8),
		RequestID: reqID,
		Actor:     claims.Name,
		Action:    "request.created",
		Reason:    claims.Name + " submitted: " + title,
	}); err != nil {
		h.logger.Error("create request: append audit event", slog.String("err", err.Error()))
	}

	// Hand the request off to the orchestration engine, which runs each node
	// through its department agent on a background worker (F3).
	if h.engine != nil {
		h.engine.Start(reqID)
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
	deps, err := h.deps.ListUnresolvedByRequest(ctx, id)
	if err != nil {
		h.logger.Error("get request: list deps", slog.String("err", err.Error()))
	}

	nodeList := make([]map[string]any, 0, len(nodes))
	for _, n := range nodes {
		// Include dependency info for blocked nodes (F5).
		var blockedBy map[string]any
		if n.Status == "blocked" {
			for _, d := range deps {
				if d.DependentNodeID == n.ID {
					blockedBy = map[string]any{
						"reason":     d.Reason,
						"blocked_at": d.CreatedAt,
					}
					break
				}
			}
		}
		nodeEntry := map[string]any{
			"id":               n.ID,
			"key":              n.Key,
			"name":             n.Name,
			"agent_type":       n.AgentType,
			"department":       n.Department,
			"status":           n.Status,
			"description":      n.Description,
			"progress_percent": n.ProgressPercent,
			"status_text":      n.StatusText,
			"decision_outcome": n.DecisionOutcome,
			"decision_summary": n.DecisionSummary,
			"started_at":       n.StartedAt,
			"completed_at":     n.CompletedAt,
		}
		if blockedBy != nil {
			nodeEntry["blocked_by"] = blockedBy
		}
		nodeList = append(nodeList, nodeEntry)
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

// GetNode handles GET /requests/:id/nodes/:nodeId, returning a node plus its
// agent tasks and audit activity.
func (h *RequestsHandler) GetNode(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	requestID := c.Param("id")
	nodeID := c.Param("nodeId")

	ctx := c.Request().Context()
	req, err := h.requests.GetByID(ctx, requestID)
	if errors.Is(err, repo.ErrNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "request not found"})
	}
	if err != nil {
		h.logger.Error("get node: request", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	if _, err := requireOrgMember(c, h.pg, req.OrgID, claims.UserID); err != nil {
		var he *echo.HTTPError
		if errors.As(err, &he) && he.Code == http.StatusForbidden {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "request not found"})
		}
		return handleOrgMemberErr(c, err)
	}

	node, err := h.workflow.GetNode(ctx, nodeID)
	if errors.Is(err, repo.ErrNotFound) || (node != nil && node.RequestID != requestID) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "node not found"})
	}
	if err != nil {
		h.logger.Error("get node: query", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	tasks, err := h.workflow.ListTasksByNode(ctx, nodeID)
	if err != nil {
		h.logger.Error("get node: tasks", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	taskList := make([]map[string]any, 0, len(tasks))
	for _, t := range tasks {
		taskList = append(taskList, map[string]any{
			"id":           t.ID,
			"title":        t.Title,
			"status":       t.Status,
			"started_at":   t.StartedAt,
			"completed_at": t.CompletedAt,
		})
	}

	flags, err := h.workflow.ListFlagsByNode(ctx, nodeID)
	if err != nil {
		h.logger.Error("get node: flags", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	flagList := make([]map[string]any, 0, len(flags))
	for _, f := range flags {
		flagList = append(flagList, map[string]any{
			"severity": f.Severity,
			"message":  f.Message,
		})
	}

	// F5: include dependency info for blocked nodes.
	var blockedBy map[string]any
	if node.Status == "blocked" {
		deps, err := h.deps.ListUnresolvedByDependent(ctx, nodeID)
		if err == nil && len(deps) > 0 {
			blockedBy = map[string]any{
				"reason":     deps[0].Reason,
				"blocked_at": deps[0].CreatedAt,
			}
		}
	}

	nodeResp := map[string]any{
		"id":               node.ID,
		"key":              node.Key,
		"name":             node.Name,
		"agent_type":       node.AgentType,
		"department":       node.Department,
		"status":           node.Status,
		"description":      node.Description,
		"progress_percent": node.ProgressPercent,
		"status_text":      node.StatusText,
		"decision_outcome": node.DecisionOutcome,
		"decision_summary": node.DecisionSummary,
		"flags":            flagList,
		"started_at":       node.StartedAt,
		"completed_at":     node.CompletedAt,
	}
	if blockedBy != nil {
		nodeResp["blocked_by"] = blockedBy
	}

	// F6: node-scoped audit activity.
	activity, err := h.audit.ListByNode(ctx, nodeID)
	if err != nil {
		h.logger.Error("get node: activity audit", slog.String("err", err.Error()))
	}
	activityList := make([]map[string]any, 0, len(activity))
	for _, a := range activity {
		activityList = append(activityList, map[string]any{
			"id":         a.ID,
			"actor":      a.Actor,
			"action":     a.Action,
			"reason":     a.Reason,
			"created_at": a.CreatedAt,
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"node":     nodeResp,
		"tasks":    taskList,
		"activity": activityList,
	})
}

// maxJustificationLen caps the written approval reason.
const maxJustificationLen = 2000

// validateApprovalInput normalizes and validates an approval decision. It
// accepts only "approve" or "reject", and requires a non-empty justification
// (the written reason is the point of the gate). Pure so it can be table-tested
// without a DB or HTTP context. Returns the trimmed justification and a
// non-empty message describing the first failure (empty means valid).
func validateApprovalInput(decision, justification string) (normJustification, errMsg string) {
	switch decision {
	case "approve", "reject":
	default:
		return "", "decision must be approve or reject"
	}
	justification = strings.TrimSpace(justification)
	if justification == "" {
		return "", "justification is required"
	}
	if len(justification) > maxJustificationLen {
		return "", fmt.Sprintf("justification must be at most %d characters", maxJustificationLen)
	}
	return justification, ""
}

// ApproveRequest handles POST /requests/:id/approve. An org approver (admin)
// decides a request parked at the executive gate, with a required written
// justification. Approve completes the gate and resumes the workflow into the
// execution stages; reject stops the request. The caller must belong to the
// request's org and hold the approver role.
func (h *RequestsHandler) ApproveRequest(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	if h.engine == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	id := c.Param("id")

	ctx := c.Request().Context()
	req, err := h.requests.GetByID(ctx, id)
	if errors.Is(err, repo.ErrNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "request not found"})
	}
	if err != nil {
		h.logger.Error("approve request: query", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	// Authorize via the request's org. A non-member is told "not found" (same
	// probe protection as GetRequest); a member without the approver role gets
	// a 403.
	role, err := requireOrgMember(c, h.pg, req.OrgID, claims.UserID)
	if err != nil {
		var he *echo.HTTPError
		if errors.As(err, &he) && he.Code == http.StatusForbidden {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "request not found"})
		}
		return handleOrgMemberErr(c, err)
	}
	if role != "admin" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "only an approver can decide this request"})
	}

	var body struct {
		Decision      string `json:"decision"`
		Justification string `json:"justification"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	justification, verr := validateApprovalInput(body.Decision, body.Justification)
	if verr != "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": verr})
	}

	if req.Status != "awaiting_approval" {
		return c.JSON(http.StatusConflict, map[string]string{"error": "request is not awaiting approval"})
	}

	if err := h.engine.Approve(ctx, id, orchestrator.ApprovalDecision(body.Decision), justification, claims.Name); err != nil {
		if errors.Is(err, orchestrator.ErrNotAwaitingApproval) {
			return c.JSON(http.StatusConflict, map[string]string{"error": "request is not awaiting approval"})
		}
		h.logger.Error("approve request: engine", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	// On approve, resume the worker into the execution stages — the same way
	// CreateRequest launches it after persisting the plan.
	if body.Decision == string(orchestrator.ApprovalApprove) {
		h.engine.Start(id)
	}

	updated, err := h.requests.GetByID(ctx, id)
	if err != nil {
		h.logger.Error("approve request: reload", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	names := h.requesterNames(ctx, []int64{updated.RequesterUserID})
	return c.JSON(http.StatusOK, map[string]any{"request": toRequestResponse(*updated, names[updated.RequesterUserID])})
}

// ListRequestAudit handles GET /requests/:id/audit, returning all audit
// events for a request, newest first.
func (h *RequestsHandler) ListRequestAudit(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	requestID := c.Param("id")

	ctx := c.Request().Context()
	req, err := h.requests.GetByID(ctx, requestID)
	if errors.Is(err, repo.ErrNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "request not found"})
	}
	if err != nil {
		h.logger.Error("list request audit: get request", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	if _, err := requireOrgMember(c, h.pg, req.OrgID, claims.UserID); err != nil {
		var he *echo.HTTPError
		if errors.As(err, &he) && he.Code == http.StatusForbidden {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "request not found"})
		}
		return handleOrgMemberErr(c, err)
	}

	events, err := h.audit.ListByRequest(ctx, requestID)
	if err != nil {
		h.logger.Error("list request audit: query", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	eventList := make([]map[string]any, 0, len(events))
	for _, e := range events {
		eventList = append(eventList, map[string]any{
			"id":          e.ID,
			"request_id":  e.RequestID,
			"node_id":     e.NodeID,
			"actor":       e.Actor,
			"action":      e.Action,
			"reason":      e.Reason,
			"document_id": e.DocumentID,
			"created_at":  e.CreatedAt,
		})
	}
	return c.JSON(http.StatusOK, map[string]any{"events": eventList})
}

// ListOrgAudit handles GET /orgs/:orgId/audit, returning audit events across
// all requests in an org, newest first.
func (h *RequestsHandler) ListOrgAudit(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	orgID := c.Param("orgId")

	if _, err := requireOrgMember(c, h.pg, orgID, claims.UserID); err != nil {
		return handleOrgMemberErr(c, err)
	}

	ctx := c.Request().Context()
	events, err := h.audit.ListByOrg(ctx, orgID)
	if err != nil {
		h.logger.Error("list org audit: query", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	eventList := make([]map[string]any, 0, len(events))
	for _, e := range events {
		eventList = append(eventList, map[string]any{
			"id":          e.ID,
			"request_id":  e.RequestID,
			"node_id":     e.NodeID,
			"actor":       e.Actor,
			"action":      e.Action,
			"reason":      e.Reason,
			"document_id": e.DocumentID,
			"created_at":  e.CreatedAt,
		})
	}
	return c.JSON(http.StatusOK, map[string]any{"events": eventList})
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
