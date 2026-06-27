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

// ReportHandler serves the compiled final report for a completed request (F8).
type ReportHandler struct {
	logger   *slog.Logger
	pg       *pgxpool.Pool
	requests *repo.RequestRepo
	workflow *repo.WorkflowRepo
	audit    *repo.AuditRepo
}

func NewReportHandler(logger *slog.Logger, pg *pgxpool.Pool) *ReportHandler {
	return &ReportHandler{
		logger:   logger,
		pg:       pg,
		requests: repo.NewRequestRepo(pg),
		workflow: repo.NewWorkflowRepo(pg),
		audit:    repo.NewAuditRepo(pg),
	}
}

// reportStage holds the per-node data for the final report.
type reportStage struct {
	Key             string           `json:"key"`
	Name            string           `json:"name"`
	Department      string           `json:"department"`
	Status          string           `json:"status"`
	StatusText      string           `json:"status_text"`
	DecisionOutcome string           `json:"decision_outcome"`
	DecisionSummary string           `json:"decision_summary"`
	StartedAt       *time.Time       `json:"started_at"`
	CompletedAt     *time.Time       `json:"completed_at"`
	DurationSeconds int64            `json:"duration_seconds"`
	Tasks           []reportTaskItem `json:"tasks"`
}

type reportTaskItem struct {
	Title  string `json:"title"`
	Status string `json:"status"`
}

// reportApproval captures the human gate decision.
type reportApproval struct {
	Decision      string    `json:"decision"`
	Justification string    `json:"justification"`
	ApprovedBy    string    `json:"approved_by"`
	ApprovedAt    time.Time `json:"approved_at"`
}

// reportFlag is a notable event during execution.
type reportFlag struct {
	StageKey  string `json:"stage_key"`
	StageName string `json:"stage_name"`
	Severity  string `json:"severity"`
	Message   string `json:"message"`
}

// reportSummary holds aggregated report metadata.
type reportSummary struct {
	TotalStages     int    `json:"total_stages"`
	CompletedStages int    `json:"completed_stages"`
	TotalTimeHuman  string `json:"total_time_human"`
}

// reportRequest is the request overview section of the report.
type reportRequest struct {
	ID            string     `json:"id"`
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	Priority      string     `json:"priority"`
	RequesterName string     `json:"requester_name"`
	Status        string     `json:"status"`
	CreatedAt     time.Time  `json:"created_at"`
	CompletedAt   *time.Time `json:"completed_at"`
}

// finalReport is the top-level response shape for GET /requests/:id/report.
type finalReport struct {
	Request  reportRequest   `json:"request"`
	Summary  reportSummary   `json:"summary"`
	Stages   []reportStage   `json:"stages"`
	Approval *reportApproval `json:"approval,omitempty"`
	Flags    []reportFlag    `json:"flags"`
}

// GetReport handles GET /requests/:id/report. It compiles a structured report
// from the request, its nodes, agent tasks, and audit trail. The report is
// generated on-the-fly — there is no separate report table — so it reflects
// whatever state the request is in (in-progress, completed, or rejected).
func (h *ReportHandler) GetReport(c echo.Context) error {
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
		h.logger.Error("get report: request", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	if _, err := requireOrgMember(c, h.pg, req.OrgID, claims.UserID); err != nil {
		var he *echo.HTTPError
		if errors.As(err, &he) && he.Code == http.StatusForbidden {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "request not found"})
		}
		return handleOrgMemberErr(c, err)
	}

	nodes, err := h.workflow.ListNodesByRequest(ctx, id)
	if err != nil {
		h.logger.Error("get report: list nodes", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	tasks, err := h.workflow.ListTasksByRequest(ctx, id)
	if err != nil {
		h.logger.Error("get report: list tasks", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	// Group tasks by node ID.
	tasksByNode := make(map[string][]repo.AgentTask, len(nodes))
	for _, t := range tasks {
		tasksByNode[t.NodeID] = append(tasksByNode[t.NodeID], t)
	}

	auditEvents, err := h.audit.ListByRequest(ctx, id)
	if err != nil {
		h.logger.Error("get report: list audit", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	// Resolve requester name.
	names := h.requesterNames(ctx, []int64{req.RequesterUserID})

	stages := make([]reportStage, 0, len(nodes))
	var flags []reportFlag
	var firstStart, lastEnd time.Time

	for _, n := range nodes {
		nodeTasks := tasksByNode[n.ID]
		taskItems := make([]reportTaskItem, 0, len(nodeTasks))
		for _, t := range nodeTasks {
			taskItems = append(taskItems, reportTaskItem{
				Title:  t.Title,
				Status: t.Status,
			})
		}

		var dur time.Duration
		if n.StartedAt != nil && n.CompletedAt != nil {
			dur = n.CompletedAt.Sub(*n.StartedAt)
		}

		stages = append(stages, reportStage{
			Key:             n.Key,
			Name:            n.Name,
			Department:      n.Department,
			Status:          n.Status,
			StatusText:      n.StatusText,
			DecisionOutcome: n.DecisionOutcome,
			DecisionSummary: n.DecisionSummary,
			StartedAt:       n.StartedAt,
			CompletedAt:     n.CompletedAt,
			DurationSeconds: int64(dur.Seconds()),
			Tasks:           taskItems,
		})

		if n.StartedAt != nil && (firstStart.IsZero() || n.StartedAt.Before(firstStart)) {
			firstStart = *n.StartedAt
		}
		if n.CompletedAt != nil && (lastEnd.IsZero() || n.CompletedAt.After(lastEnd)) {
			lastEnd = *n.CompletedAt
		}

	}

	var completedAt *time.Time
	var totalDuration time.Duration
	if !firstStart.IsZero() && !lastEnd.IsZero() {
		totalDuration = lastEnd.Sub(firstStart)
		completedAt = &lastEnd
	}

	// Agent fallback and blocked flags from durable audit events (not live
	// node status, since blocked nodes are transient — they complete later).
	for _, ae := range auditEvents {
		if ae.Action == "agent.fallback" || ae.Action == "node.blocked" {
			var stageKey, stageName string
			if ae.NodeID != nil {
				for _, n := range nodes {
					if n.ID == *ae.NodeID {
						stageKey = n.Key
						stageName = n.Name
						break
					}
				}
			}
			severity := "info"
			if ae.Action == "node.blocked" {
				severity = "warning"
			}
			flags = append(flags, reportFlag{
				StageKey:  stageKey,
				StageName: stageName,
				Severity:  severity,
				Message:   ae.Reason,
			})
		}
	}

	// Extract approval info from audit events.
	var approval *reportApproval
	for _, ae := range auditEvents {
		if ae.Action == "approval.granted" {
			app := &reportApproval{
				Decision:      "approve",
				Justification: ae.Reason,
				ApprovedBy:    ae.Actor,
				ApprovedAt:    ae.CreatedAt,
			}
			approval = app
			break
		}
		if ae.Action == "approval.rejected" {
			app := &reportApproval{
				Decision:      "reject",
				Justification: ae.Reason,
				ApprovedBy:    ae.Actor,
				ApprovedAt:    ae.CreatedAt,
			}
			approval = app
			break
		}
	}

	completed := 0
	for _, n := range nodes {
		if n.Status == "completed" {
			completed++
		}
	}

	report := finalReport{
		Request: reportRequest{
			ID:            req.ID,
			Title:         req.Title,
			Description:   req.Description,
			Priority:      req.Priority,
			RequesterName: names[req.RequesterUserID],
			Status:        req.Status,
			CreatedAt:     req.CreatedAt,
			CompletedAt:   completedAt,
		},
		Summary: reportSummary{
			TotalStages:     len(nodes),
			CompletedStages: completed,
			TotalTimeHuman:  formatDuration(totalDuration),
		},
		Stages:   stages,
		Approval: approval,
		Flags:    flags,
	}

	return c.JSON(http.StatusOK, report)
}

// requesterNames resolves user ids to display names in one query.
func (h *ReportHandler) requesterNames(ctx context.Context, ids []int64) map[int64]string {
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

// formatDuration returns a human-readable string like "2m 34s" or "1h 5m".
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	switch {
	case h > 0 && m > 0:
		return fmt.Sprintf("%dh %dm", h, m)
	case h > 0:
		return fmt.Sprintf("%dh", h)
	case m > 0:
		return fmt.Sprintf("%dm %ds", m, s)
	default:
		return fmt.Sprintf("%ds", s)
	}
}
