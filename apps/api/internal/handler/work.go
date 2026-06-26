package handler

import (
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

// WorkHandler serves the mobile-app endpoints that don't exist yet:
//   - GET  /me/work           — pending work items for the authenticated user
//   - GET  /tasks/:id/detail  — full task detail
//   - POST /tasks/:id/progress — submit a milestone
//   - POST /tasks/:id/complete — mark task done
//   - POST /tasks/:id/decision — approve / request-info / escalate
type WorkHandler struct {
	logger   *slog.Logger
	pg       *pgxpool.Pool
	workflow *repo.WorkflowRepo
	requests *repo.RequestRepo
}

func NewWorkHandler(logger *slog.Logger, pg *pgxpool.Pool) *WorkHandler {
	return &WorkHandler{
		logger:   logger,
		pg:       pg,
		workflow: repo.NewWorkflowRepo(pg),
		requests: repo.NewRequestRepo(pg),
	}
}

// ──────────────────────────────────────────────────────────────────────────
// GET /me/work
// Returns work_items: requests visible to the user (via org membership)
// that are not yet completed, enriched with their current workflow stage.
// ──────────────────────────────────────────────────────────────────────────

func (h *WorkHandler) ListMyWork(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	ctx := c.Request().Context()

	// Fetch agent_tasks scoped to the user's role:
	// - admins/executors see all tasks in their org
	// - team members only see tasks whose department matches their team
	rows, err := h.pg.Query(ctx, `
		SELECT at.id, at.title, r.description, r.priority, at.status,
		       wn.progress_percent AS progress,
		       u.name AS requester_name,
		       wn.status AS stage_status,
		       r.id AS request_id,
		       r.title AS request_title
		FROM agent_tasks at
		JOIN workflow_nodes wn ON wn.id = at.node_id
		JOIN requests r ON r.id = wn.request_id
		JOIN org_members om ON om.org_id = r.org_id AND om.user_id = $1
		JOIN users u ON u.id = r.requester_user_id
		WHERE r.status NOT IN ('rejected')
		  AND (
		    om.role IN ('admin', 'executor')
		    OR EXISTS (
		      SELECT 1 FROM team_members tm
		      JOIN teams t ON t.id = tm.team_id
		      WHERE tm.user_id = $1
		        AND LOWER(t.name) = LOWER(wn.department)
		    )
		  )
		ORDER BY r.created_at DESC
		LIMIT 50
	`, claims.UserID)
	if err != nil {
		h.logger.Error("list my work: query", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	defer rows.Close()

	type workItem struct {
		ID            string `json:"id"`
		Title         string `json:"title"`
		Description   string `json:"description"`
		Priority      string `json:"priority"`
		Status        string `json:"status"`
		Progress      int    `json:"progress"`
		RequesterName string `json:"requester_name"`
		StageStatus   string `json:"stage_status"`
		RequestID     string `json:"request_id"`
		RequestTitle  string `json:"request_title"`
	}

	items := make([]workItem, 0)
	for rows.Next() {
		var w workItem
		if err := rows.Scan(&w.ID, &w.Title, &w.Description, &w.Priority,
			&w.Status, &w.Progress, &w.RequesterName, &w.StageStatus,
			&w.RequestID, &w.RequestTitle); err != nil {
			h.logger.Error("list my work: scan", slog.String("err", err.Error()))
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		}
		items = append(items, w)
	}
	if err := rows.Err(); err != nil {
		h.logger.Error("list my work: rows", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	return c.JSON(http.StatusOK, map[string]any{"work_items": items})
}

// ──────────────────────────────────────────────────────────────────────────
// GET /tasks/:id/detail
// Returns a rich task-detail payload matching the mobile TaskDetailModel.
// :id is an agent_task id.
// ──────────────────────────────────────────────────────────────────────────

func (h *WorkHandler) GetTaskDetail(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	taskID := c.Param("id")
	ctx := c.Request().Context()

	// Look up the agent_task, its parent node, and the parent request.
	var (
		nodeID, nodeName, nodeDept, nodeStatus, nodeStatusText string
		nodeProgress                                           int
		taskTitle, taskStatus                                  string
		taskOrdinal                                            int
		taskStartedAt, taskCompletedAt                         *time.Time
		reqID, reqTitle, reqDesc, reqPriority, reqStatus       string
	)
	err := h.pg.QueryRow(ctx, `
		SELECT at.node_id, at.title, at.status, at.ordinal, at.started_at, at.completed_at,
		       wn.name, wn.department, wn.status, wn.status_text, wn.progress_percent,
		       r.id, r.title, r.description, r.priority, r.status
		FROM agent_tasks at
		JOIN workflow_nodes wn ON wn.id = at.node_id
		JOIN requests r ON r.id = wn.request_id
		WHERE at.id = $1
	`, taskID).Scan(
		&nodeID, &taskTitle, &taskStatus, &taskOrdinal, &taskStartedAt, &taskCompletedAt,
		&nodeName, &nodeDept, &nodeStatus, &nodeStatusText, &nodeProgress,
		&reqID, &reqTitle, &reqDesc, &reqPriority, &reqStatus,
	)
	if err != nil {
		if errors.Is(err, errors.New("no rows")) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
		}
		// pgx returns its own ErrNoRows
		if err.Error() == "no rows in result set" {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
		}
		h.logger.Error("get task detail: query", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	// Fetch sibling tasks for the same node as "steps"
	tasks, err := h.workflow.ListTasksByNode(ctx, nodeID)
	if err != nil {
		h.logger.Error("get task detail: sibling tasks", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	steps := make([]map[string]any, 0, len(tasks))
	for _, t := range tasks {
		steps = append(steps, map[string]any{
			"id":     t.ID,
			"title":  t.Title,
			"status": t.Status,
		})
	}

	// Fetch completed sibling workflow nodes as "upstream decisions"
	upstreamRows, err := h.pg.Query(ctx, `
		SELECT wn.name, wn.department, wn.status, wn.status_text
		FROM workflow_nodes wn
		WHERE wn.request_id = $1 AND wn.id != $2 AND wn.status = 'completed'
		ORDER BY wn.completed_at ASC NULLS LAST
		LIMIT 10
	`, reqID, nodeID)
	upstreamDecisions := make([]map[string]any, 0)
	if err == nil {
		defer upstreamRows.Close()
		for upstreamRows.Next() {
			var uName, uDept, uStatus, uText string
			if scanErr := upstreamRows.Scan(&uName, &uDept, &uStatus, &uText); scanErr == nil {
				upstreamDecisions = append(upstreamDecisions, map[string]any{
					"stage":     uName,
					"verdict":   "approved",
					"agent":     uDept,
					"reasoning": uText,
				})
			}
		}
	}

	// Build richer hints from context
	hints := []map[string]any{
		{"message": fmt.Sprintf("Department: %s", nodeDept), "type": "info"},
		{"message": fmt.Sprintf("Request: %s", reqTitle), "type": "info"},
	}
	if reqPriority == "urgent" || reqPriority == "high" {
		hints = append(hints, map[string]any{
			"message": fmt.Sprintf("This is a %s-priority request — escalate blockers immediately", reqPriority),
			"type":    "warning",
		})
	}
	if nodeStatusText != "" {
		hints = append(hints, map[string]any{
			"message": nodeStatusText,
			"type":    "info",
		})
	}

	// Compute SLA based on priority
	var slaTotalHours int
	switch reqPriority {
	case "urgent":
		slaTotalHours = 4
	case "high":
		slaTotalHours = 12
	case "medium":
		slaTotalHours = 24
	default:
		slaTotalHours = 48
	}
	slaTotalSecs := slaTotalHours * 3600
	// If the task has a started_at, compute remaining from elapsed time
	slaRemaining := slaTotalSecs
	if taskStartedAt != nil {
		elapsed := int(time.Since(*taskStartedAt).Seconds())
		slaRemaining = slaTotalSecs - elapsed
		if slaRemaining < 0 {
			slaRemaining = 0
		}
	}
	slaDeadline := time.Now().Add(time.Duration(slaRemaining) * time.Second).Format(time.RFC3339)

	return c.JSON(http.StatusOK, map[string]any{
		"task_id":               taskID,
		"case_id":               reqID,
		"title":                 taskTitle,
		"description":           fmt.Sprintf("%s — %s", nodeName, reqDesc),
		"priority":              reqPriority,
		"status":                taskStatus,
		"claimed_by":            nil,
		"sla_deadline":          slaDeadline,
		"sla_remaining_seconds": slaRemaining,
		"progress_percent":      float64(nodeProgress),
		"current_milestone":     nil,
		"why_this_task":         fmt.Sprintf("Part of the \"%s\" stage for request: %s. This task involves %s work within the %s department.", nodeName, reqTitle, taskTitle, nodeDept),
		"upstream_decisions":    upstreamDecisions,
		"steps":                 steps,
		"hints":                 hints,
		"team_notes":            []any{},
		"capabilities":          []string{"complete", "escalate", "request_info"},
	})
}

// ──────────────────────────────────────────────────────────────────────────
// POST /tasks/:id/progress — submit a milestone update
// ──────────────────────────────────────────────────────────────────────────

func (h *WorkHandler) PostTaskProgress(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	taskID := c.Param("id")
	ctx := c.Request().Context()

	var body struct {
		WorkerID  string `json:"worker_id"`
		Milestone string `json:"milestone"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}

	// Update the task's status text
	tag, err := h.pg.Exec(ctx, `
		UPDATE agent_tasks SET status = 'in_progress',
		       started_at = COALESCE(started_at, now())
		WHERE id = $1
	`, taskID)
	if err != nil {
		h.logger.Error("task progress: update", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	if tag.RowsAffected() == 0 {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// ──────────────────────────────────────────────────────────────────────────
// POST /tasks/:id/complete — mark a task as done
// ──────────────────────────────────────────────────────────────────────────

func (h *WorkHandler) PostTaskComplete(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	taskID := c.Param("id")
	ctx := c.Request().Context()

	tag, err := h.pg.Exec(ctx, `
		UPDATE agent_tasks SET status = 'completed', completed_at = now()
		WHERE id = $1
	`, taskID)
	if err != nil {
		h.logger.Error("task complete: update", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	if tag.RowsAffected() == 0 {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// ──────────────────────────────────────────────────────────────────────────
// POST /tasks/:id/decision — approve, request_info, or escalate
// ──────────────────────────────────────────────────────────────────────────

func (h *WorkHandler) PostTaskDecision(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	taskID := c.Param("id")
	ctx := c.Request().Context()

	var body struct {
		WorkerID string `json:"worker_id"`
		Verdict  string `json:"verdict"`
		Reason   string `json:"reason"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}

	switch body.Verdict {
	case "approve", "request_info", "escalate":
	default:
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "verdict must be approve, request_info, or escalate"})
	}

	newStatus := "completed"
	if body.Verdict == "escalate" {
		newStatus = "blocked"
	}

	tag, err := h.pg.Exec(ctx, `
		UPDATE agent_tasks SET status = $2, completed_at = now()
		WHERE id = $1
	`, taskID, newStatus)
	if err != nil {
		h.logger.Error("task decision: update", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	if tag.RowsAffected() == 0 {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok", "verdict": body.Verdict})
}
