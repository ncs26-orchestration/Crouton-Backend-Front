package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/ncs26-orchestration/solution/apps/api/internal/auth"
	"github.com/ncs26-orchestration/solution/apps/api/internal/middleware"
	"github.com/ncs26-orchestration/solution/apps/api/internal/repo"
)

type IncidentsHandler struct {
	logger    *slog.Logger
	db        *pgxpool.Pool
	mach      *repo.MachineRepo
	inc       *repo.IncidentRepo
	jwtSecret string
}

func NewIncidentsHandler(logger *slog.Logger, db *pgxpool.Pool, jwtSecret string) *IncidentsHandler {
	return &IncidentsHandler{
		logger:    logger,
		db:        db,
		mach:      repo.NewMachineRepo(db),
		inc:       repo.NewIncidentRepo(db),
		jwtSecret: jwtSecret,
	}
}

// incidentResponse is the JSON shape returned for an incident.
type incidentResponse struct {
	ID              string     `json:"id"`
	MachineID       string     `json:"machine_id"`
	OrgID           string     `json:"org_id"`
	ReportedBy      int64      `json:"reported_by"`
	Title           string     `json:"title"`
	Description     string     `json:"description"`
	Severity        string     `json:"severity"`
	Status          string     `json:"status"`
	ResolvedAt      *time.Time `json:"resolved_at,omitempty"`
	ResolutionNotes string     `json:"resolution_notes,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

func toIncidentResponse(inc repo.Incident) incidentResponse {
	return incidentResponse{
		ID:              inc.ID,
		MachineID:       inc.MachineID,
		OrgID:           inc.OrgID,
		ReportedBy:      inc.ReportedBy,
		Title:           inc.Title,
		Description:     inc.Description,
		Severity:        inc.Severity,
		Status:          inc.Status,
		ResolvedAt:      inc.ResolvedAt,
		ResolutionNotes: inc.ResolutionNotes,
		CreatedAt:       inc.CreatedAt,
	}
}

type incMessageResponse struct {
	ID         string    `json:"id"`
	IncidentID string    `json:"incident_id"`
	SenderID   *int64    `json:"sender_id"`
	SenderName string    `json:"sender_name"`
	SenderRole string    `json:"sender_role"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}

func toIncMessageResponse(m repo.IncidentMessage) incMessageResponse {
	return incMessageResponse{
		ID:         m.ID,
		IncidentID: m.IncidentID,
		SenderID:   m.SenderID,
		SenderName: m.SenderName,
		SenderRole: m.SenderRole,
		Content:    m.Content,
		CreatedAt:  m.CreatedAt,
	}
}

// ── Create ──────────────────────────────────────────────────────────

// CreateIncident handles POST /incidents.
// Body: { machine_id, title, description, severity }
// Auto-flips the machine status to "down".
func (h *IncidentsHandler) CreateIncident(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	var body struct {
		MachineID   string `json:"machine_id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Severity    string `json:"severity"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	body.MachineID = strings.TrimSpace(body.MachineID)
	body.Title = strings.TrimSpace(body.Title)
	if body.MachineID == "" || body.Title == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "machine_id and title are required"})
	}
	if body.Severity == "" {
		body.Severity = "medium"
	}
	if body.Severity != "low" && body.Severity != "medium" && body.Severity != "high" && body.Severity != "critical" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "severity must be low, medium, high, or critical"})
	}

	ctx := c.Request().Context()

	// Look up the machine so we can get its org_id and verify access.
	machine, err := h.mach.GetByID(ctx, body.MachineID)
	if errors.Is(err, repo.ErrNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "machine not found"})
	}
	if err != nil {
		h.logger.Error("create incident: get machine", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	if _, err := requireOrgMember(c, h.db, machine.OrgID, claims.UserID); err != nil {
		return handleOrgMemberErr(c, err)
	}

	incidentID := fmt.Sprintf("inc_%s", randomHex(8))
	inc, err := h.inc.Create(ctx, repo.Incident{
		ID:          incidentID,
		MachineID:   body.MachineID,
		OrgID:       machine.OrgID,
		ReportedBy:  claims.UserID,
		Title:       body.Title,
		Description: body.Description,
		Severity:    body.Severity,
		Status:      "open",
	})
	if err != nil {
		h.logger.Error("create incident: db insert", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	// Auto-flip machine status to "down".
	machine.Status = "down"
	if err := h.mach.Update(ctx, *machine); err != nil {
		h.logger.Error("create incident: update machine status", slog.String("err", err.Error()))
	}

	// Create an agent_task so the incident appears in the technician's
	// inbox (GET /me/work). The "enpanne:machine:" prefix signals to the
	// mobile client that this task routes to the incident-detail screen
	// with AI diagnosis support.
	taskID := "enpanne:machine:" + incidentID
	if _, err := h.db.Exec(ctx,
		`INSERT INTO agent_tasks (id, title, status, incident_id)
		 VALUES ($1, $2, 'pending', $3)
		 ON CONFLICT (id) DO NOTHING`,
		taskID, body.Title, incidentID,
	); err != nil {
		h.logger.Error("create incident: insert agent_task", slog.String("err", err.Error()))
	}

	return c.JSON(http.StatusCreated, map[string]any{
		"incident": toIncidentResponse(*inc),
	})
}

// ── List Messages ───────────────────────────────────────────────────

// ListMessages handles GET /incidents/:id/messages.
func (h *IncidentsHandler) ListMessages(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	incidentID := c.Param("id")
	ctx := c.Request().Context()

	inc, err := h.inc.GetByID(ctx, incidentID)
	if errors.Is(err, repo.ErrNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "incident not found"})
	}
	if err != nil {
		h.logger.Error("list messages: get incident", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	if _, err := requireOrgMember(c, h.db, inc.OrgID, claims.UserID); err != nil {
		return handleOrgMemberErr(c, err)
	}

	msgs, err := h.inc.ListMessages(ctx, incidentID)
	if err != nil {
		h.logger.Error("list messages: db", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	out := make([]incMessageResponse, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, toIncMessageResponse(m))
	}

	return c.JSON(http.StatusOK, map[string]any{"messages": out})
}

// ── Append Message ──────────────────────────────────────────────────

// AppendMessage handles POST /incidents/:id/messages.
// Body: { content }
func (h *IncidentsHandler) AppendMessage(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	incidentID := c.Param("id")
	ctx := c.Request().Context()

	inc, err := h.inc.GetByID(ctx, incidentID)
	if errors.Is(err, repo.ErrNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "incident not found"})
	}
	if err != nil {
		h.logger.Error("append message: get incident", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	if _, err := requireOrgMember(c, h.db, inc.OrgID, claims.UserID); err != nil {
		return handleOrgMemberErr(c, err)
	}

	var body struct {
		Content string `json:"content"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	body.Content = strings.TrimSpace(body.Content)
	if body.Content == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "content is required"})
	}

	msg, err := h.inc.AppendMessage(ctx, repo.IncidentMessage{
		ID:         fmt.Sprintf("imsg_%s", randomHex(8)),
		IncidentID: incidentID,
		SenderID:   &claims.UserID,
		SenderName: claims.Name,
		SenderRole: "technician",
		Content:    body.Content,
	})
	if err != nil {
		h.logger.Error("append message: db", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	return c.JSON(http.StatusCreated, map[string]any{
		"message": toIncMessageResponse(*msg),
	})
}

// ── Resolve ─────────────────────────────────────────────────────────

// ResolveIncident handles POST /incidents/:id/resolve.
// Body: { notes }
// Auto-flips the machine status back to "operational".
func (h *IncidentsHandler) ResolveIncident(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	incidentID := c.Param("id")
	ctx := c.Request().Context()

	inc, err := h.inc.GetByID(ctx, incidentID)
	if errors.Is(err, repo.ErrNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "incident not found"})
	}
	if err != nil {
		h.logger.Error("resolve: get incident", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	if _, err := requireOrgMember(c, h.db, inc.OrgID, claims.UserID); err != nil {
		return handleOrgMemberErr(c, err)
	}

	var body struct {
		Notes string `json:"notes"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	if err := h.inc.Resolve(ctx, incidentID, body.Notes); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "incident not found or already resolved"})
		}
		h.logger.Error("resolve: db", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	// Auto-flip machine back to "operational".
	machine, err := h.mach.GetByID(ctx, inc.MachineID)
	if err == nil {
		machine.Status = "operational"
		if err := h.mach.Update(ctx, *machine); err != nil {
			h.logger.Error("resolve: update machine status", slog.String("err", err.Error()))
		}
	}

	return c.JSON(http.StatusOK, map[string]any{"status": "resolved"})
}

// ── SSE Events ──────────────────────────────────────────────────────

// StreamEvents handles GET /incidents/:id/events?token=.
// It polls for new messages every 3 seconds and streams them as SSE.
func (h *IncidentsHandler) StreamEvents(c echo.Context) error {
	token := c.QueryParam("token")
	if token == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "missing token query parameter"})
	}

	claims, err := auth.ParseToken(h.jwtSecret, token)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid or expired token"})
	}

	incidentID := c.Param("id")
	ctx := c.Request().Context()

	inc, err := h.inc.GetByID(ctx, incidentID)
	if errors.Is(err, repo.ErrNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "incident not found"})
	}
	if err != nil {
		h.logger.Error("events: get incident", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	// Verify access.
	if _, err := requireOrgMember(c, h.db, inc.OrgID, claims.UserID); err != nil {
		return handleOrgMemberErr(c, err)
	}

	// Get last message timestamp for incremental polling.
	var lastCreatedAt time.Time
	msgs, err := h.inc.ListMessages(ctx, incidentID)
	if err == nil && len(msgs) > 0 {
		lastCreatedAt = msgs[len(msgs)-1].CreatedAt
	}

	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().WriteHeader(http.StatusOK)

	flusher, ok := c.Response().Writer.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}
	flusher.Flush()

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			newMsgs, err := h.inc.ListMessages(ctx, incidentID)
			if err != nil {
				continue
			}
			for _, m := range newMsgs {
				if m.CreatedAt.After(lastCreatedAt) {
					lastCreatedAt = m.CreatedAt
					data, _ := json.Marshal(toIncMessageResponse(m))
					_, err := fmt.Fprintf(c.Response().Writer, "event: message\ndata: %s\n\n", data)
					if err != nil {
						return nil
					}
					flusher.Flush()
				}
			}
		}
	}
}
