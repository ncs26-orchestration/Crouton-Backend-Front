package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/ncs26-orchestration/solution/apps/api/internal/auth"
	"github.com/ncs26-orchestration/solution/apps/api/internal/middleware"
	"github.com/ncs26-orchestration/solution/apps/api/internal/repo"
)

type MachinesHandler struct {
	logger    *slog.Logger
	db        *pgxpool.Pool
	mach      *repo.MachineRepo
	telemetry *repo.TelemetryRepo
	inc       *repo.IncidentRepo
	jwtSecret string
}

func NewMachinesHandler(logger *slog.Logger, db *pgxpool.Pool, jwtSecret string) *MachinesHandler {
	return &MachinesHandler{
		logger:    logger,
		db:        db,
		mach:      repo.NewMachineRepo(db),
		telemetry: repo.NewTelemetryRepo(db),
		inc:       repo.NewIncidentRepo(db),
		jwtSecret: jwtSecret,
	}
}

type machineResponse struct {
	ID             string     `json:"id"`
	OrgID          string     `json:"org_id"`
	AssignedUserID *int64     `json:"assigned_user_id"`
	Name           string     `json:"name"`
	MachineType    string     `json:"machine_type"`
	Location       string     `json:"location"`
	SerialNumber   string     `json:"serial_number"`
	Status         string     `json:"status"`
	Metadata       any        `json:"metadata"`
	LastServiceAt  *time.Time `json:"last_service_at"`
	NextServiceDue *time.Time `json:"next_service_due"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func toMachineResponse(m repo.Machine) machineResponse {
	var meta any
	if len(m.Metadata) > 0 {
		_ = json.Unmarshal(m.Metadata, &meta)
	}
	return machineResponse{
		ID:             m.ID,
		OrgID:          m.OrgID,
		AssignedUserID: m.AssignedUserID,
		Name:           m.Name,
		MachineType:    m.MachineType,
		Location:       m.Location,
		SerialNumber:   m.SerialNumber,
		Status:         m.Status,
		Metadata:       meta,
		LastServiceAt:  m.LastServiceAt,
		NextServiceDue: m.NextServiceDue,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
}

func toMachineListResponse(ms []repo.Machine) []machineResponse {
	out := make([]machineResponse, 0, len(ms))
	for _, m := range ms {
		out = append(out, toMachineResponse(m))
	}
	return out
}

// isTechnician returns true when the user has the technician role on any
// team in the org. Technicians see only their assigned machines; admins
// and executors see all.
func (h *MachinesHandler) isTechnician(ctx echo.Context, orgID string, userID int64) (bool, error) {
	var role string
	err := h.db.QueryRow(ctx.Request().Context(),
		`SELECT tm.role
		   FROM team_members tm
		   JOIN teams t ON t.id = tm.team_id
		  WHERE t.org_id = $1 AND tm.user_id = $2 AND tm.role = 'technician'
		  LIMIT 1`,
		orgID, userID,
	).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// callerOrgRole returns the caller's org-level role (admin/executor/employee).
func (h *MachinesHandler) callerOrgRole(ctx echo.Context, orgID string, userID int64) (string, error) {
	return requireOrgMember(ctx, h.db, orgID, userID)
}

// ── Create ──────────────────────────────────────────────────────────

func (h *MachinesHandler) CreateMachine(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	orgID := c.Param("orgId")

	role, err := h.callerOrgRole(c, orgID, claims.UserID)
	if err != nil {
		return handleOrgMemberErr(c, err)
	}
	if role != "admin" && role != "executor" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "only admins and executors can create machines"})
	}

	var body struct {
		Name           string     `json:"name"`
		MachineType    string     `json:"machine_type"`
		Location       string     `json:"location"`
		SerialNumber   string     `json:"serial_number"`
		AssignedUser   *int64     `json:"assigned_user_id"`
		Status         string     `json:"status"`
		LastServiceAt  *time.Time `json:"last_service_at"`
		NextServiceDue *time.Time `json:"next_service_due"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "name is required"})
	}
	if body.Status == "" {
		body.Status = "operational"
	}

	machineID := fmt.Sprintf("mach_%s", randomHex(8))
	saved, err := h.mach.Create(c.Request().Context(), repo.Machine{
		ID:             machineID,
		OrgID:          orgID,
		AssignedUserID: body.AssignedUser,
		Name:           body.Name,
		MachineType:    body.MachineType,
		Location:       body.Location,
		SerialNumber:   body.SerialNumber,
		Status:         body.Status,
		Metadata:       []byte("{}"),
		LastServiceAt:  body.LastServiceAt,
		NextServiceDue: body.NextServiceDue,
	})
	if err != nil {
		h.logger.Error("create machine", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	return c.JSON(http.StatusCreated, map[string]any{"machine": toMachineResponse(*saved)})
}

// ── List ────────────────────────────────────────────────────────────

func (h *MachinesHandler) ListMachines(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	orgID := c.Param("orgId")

	if _, err := h.callerOrgRole(c, orgID, claims.UserID); err != nil {
		return handleOrgMemberErr(c, err)
	}

	filter := repo.ListMachinesFilter{
		Status: c.QueryParam("status"),
	}

	// ?assigned_to_me=true filters to machines assigned to the caller.
	if c.QueryParam("assigned_to_me") == "true" {
		filter.AssignedToMe = &claims.UserID
	}

	machines, err := h.mach.ListByOrg(c.Request().Context(), orgID, filter)
	if err != nil {
		h.logger.Error("list machines", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	return c.JSON(http.StatusOK, map[string]any{"machines": toMachineListResponse(machines)})
}

// ── Get ─────────────────────────────────────────────────────────────

func (h *MachinesHandler) GetMachine(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	machineID := c.Param("id")

	ctx := c.Request().Context()
	m, err := h.mach.GetByID(ctx, machineID)
	if errors.Is(err, repo.ErrNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "machine not found"})
	}
	if err != nil {
		h.logger.Error("get machine", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	role, err := h.callerOrgRole(c, m.OrgID, claims.UserID)
	if err != nil {
		return handleOrgMemberErr(c, err)
	}

	// Technicians can only view machines assigned to them.
	tech, _ := h.isTechnician(c, m.OrgID, claims.UserID)
	if tech && role != "admin" && role != "executor" {
		if m.AssignedUserID == nil || *m.AssignedUserID != claims.UserID {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "machine not found"})
		}
	}

	return c.JSON(http.StatusOK, map[string]any{"machine": toMachineResponse(*m)})
}

// ── Update ──────────────────────────────────────────────────────────

func (h *MachinesHandler) UpdateMachine(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	machineID := c.Param("id")

	ctx := c.Request().Context()
	m, err := h.mach.GetByID(ctx, machineID)
	if errors.Is(err, repo.ErrNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "machine not found"})
	}
	if err != nil {
		h.logger.Error("update machine: get", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	role, err := h.callerOrgRole(c, m.OrgID, claims.UserID)
	if err != nil {
		return handleOrgMemberErr(c, err)
	}
	if role != "admin" && role != "executor" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "only admins and executors can update machines"})
	}

	var body struct {
		Name           *string    `json:"name"`
		MachineType    *string    `json:"machine_type"`
		Location       *string    `json:"location"`
		SerialNumber   *string    `json:"serial_number"`
		Status         *string    `json:"status"`
		AssignedUser   *int64     `json:"assigned_user_id"`
		LastServiceAt  *time.Time `json:"last_service_at"`
		NextServiceDue *time.Time `json:"next_service_due"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	updated := *m
	if body.Name != nil {
		updated.Name = *body.Name
	}
	if body.MachineType != nil {
		updated.MachineType = *body.MachineType
	}
	if body.Location != nil {
		updated.Location = *body.Location
	}
	if body.SerialNumber != nil {
		updated.SerialNumber = *body.SerialNumber
	}
	if body.Status != nil {
		updated.Status = *body.Status
	}
	if body.AssignedUser != nil {
		updated.AssignedUserID = body.AssignedUser
	}
	if body.LastServiceAt != nil {
		updated.LastServiceAt = body.LastServiceAt
	}
	if body.NextServiceDue != nil {
		updated.NextServiceDue = body.NextServiceDue
	}

	if err := h.mach.Update(ctx, updated); err != nil {
		h.logger.Error("update machine", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	return c.JSON(http.StatusOK, map[string]any{"machine": toMachineResponse(updated)})
}

// ── Telemetry webhook ───────────────────────────────────────────────

// bearerToken extracts the token from an Authorization: Bearer header.
func bearerToken(c echo.Context) string {
	auth := c.Request().Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(auth, "Bearer ")
}

// PostTelemetry handles POST /machines/:id/telemetry — the webhook that
// machines/sensors call to push readings. Auth: machine API key or JWT
// in the Authorization: Bearer header. Since this endpoint is meant for
// both sensors and users, it does NOT require the echo auth middleware
// (the handler parses auth itself).
func (h *MachinesHandler) PostTelemetry(c echo.Context) error {
	machineID := c.Param("id")
	ctx := c.Request().Context()
	token := bearerToken(c)

	if token == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "missing authorization"})
	}

	// Get the machine first — we need it for both auth paths.
	machine, err := h.mach.GetByID(ctx, machineID)
	if errors.Is(err, repo.ErrNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "machine not found"})
	}
	if err != nil {
		h.logger.Error("post telemetry: get machine", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	// Try machine API key auth (for sensor/webhook use cases).
	var meta struct {
		APIKey string `json:"api_key"`
	}
	if len(machine.Metadata) > 0 {
		_ = json.Unmarshal(machine.Metadata, &meta)
	}
	authenticated := meta.APIKey != "" && meta.APIKey == token

	// Fall back to JWT auth for authenticated users.
	if !authenticated {
		claims, err := auth.ParseToken(h.jwtSecret, token)
		if err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid or expired token"})
		}
		if _, err := requireOrgMember(c, h.db, machine.OrgID, claims.UserID); err != nil {
			return handleOrgMemberErr(c, err)
		}
	}

	var body struct {
		Metrics   any    `json:"metrics"`
		ErrorCode string `json:"error_code"`
		Source    string `json:"source"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	if body.Source == "" {
		body.Source = "api"
	}

	metricsJSON, err := json.Marshal(body.Metrics)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid metrics"})
	}

	saved, err := h.telemetry.Insert(ctx, repo.Telemetry{
		ID:        fmt.Sprintf("tel_%s", randomHex(8)),
		MachineID: machineID,
		Metrics:   metricsJSON,
		ErrorCode: body.ErrorCode,
		Source:    body.Source,
	})
	if err != nil {
		h.logger.Error("post telemetry: insert", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	incidentCreated := false

	// Auto-create an incident when the machine reports an error code.
	// This ensures the maintenance team gets a task in their work inbox.
	if body.ErrorCode != "" {
		incidentCreated = true

		// Check if there's already an unresolved incident for this machine.
		existing, err := h.inc.ListByMachine(ctx, machineID)
		if err != nil {
			h.logger.Error("post telemetry: check existing incidents", slog.String("err", err.Error()))
		}

		hasOpen := false
		for _, e := range existing {
			if e.Status != "resolved" {
				hasOpen = true
				break
			}
		}

		if !hasOpen {
			incidentID := fmt.Sprintf("inc_%s", randomHex(8))
			title := fmt.Sprintf("%s reported error: %s", machine.Name, body.ErrorCode)
			desc := fmt.Sprintf("Auto-created from telemetry error_code=%q on %s", body.ErrorCode, machine.Name)

			if _, err := h.inc.Create(ctx, repo.Incident{
				ID:          incidentID,
				MachineID:   machineID,
				OrgID:       machine.OrgID,
				ReportedBy:  nil,
				Title:       title,
				Description: desc,
				Severity:    "high",
				Status:      "open",
			}); err != nil {
				h.logger.Error("post telemetry: create incident", slog.String("err", err.Error()))
			} else {
				// Flip machine status to "down".
				machine.Status = "down"
				if err := h.mach.Update(ctx, *machine); err != nil {
					h.logger.Error("post telemetry: update machine status", slog.String("err", err.Error()))
				}

				// Create agent_task linked to the incident so it shows up in
				// the maintenance team's work inbox (GET /me/work).
				taskID := "enpanne:machine:" + incidentID
				if _, err := h.db.Exec(ctx,
					`INSERT INTO agent_tasks (id, title, status, incident_id)
					 VALUES ($1, $2, 'pending', $3)
					 ON CONFLICT (id) DO NOTHING`,
					taskID, title, incidentID,
				); err != nil {
					h.logger.Error("post telemetry: insert agent_task", slog.String("err", err.Error()))
				}
			}
		}
	}

	return c.JSON(http.StatusCreated, map[string]any{
		"telemetry": map[string]any{
			"id":         saved.ID,
			"machine_id": saved.MachineID,
			"metrics":    body.Metrics,
			"error_code": saved.ErrorCode,
			"source":     saved.Source,
			"created_at": saved.CreatedAt,
		},
		"incident_created": incidentCreated,
	})
}

// ListTelemetry handles GET /machines/:id/telemetry (JWT auth required).
func (h *MachinesHandler) ListTelemetry(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	machineID := c.Param("id")

	ctx := c.Request().Context()
	machine, err := h.mach.GetByID(ctx, machineID)
	if errors.Is(err, repo.ErrNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "machine not found"})
	}
	if err != nil {
		h.logger.Error("list telemetry: get machine", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	if _, err := requireOrgMember(c, h.db, machine.OrgID, claims.UserID); err != nil {
		return handleOrgMemberErr(c, err)
	}

	limit := 50
	items, err := h.telemetry.ListByMachine(ctx, machineID, limit)
	if err != nil {
		h.logger.Error("list telemetry", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	out := make([]map[string]any, 0, len(items))
	for _, t := range items {
		var metrics any
		if len(t.Metrics) > 0 {
			_ = json.Unmarshal(t.Metrics, &metrics)
		}
		out = append(out, map[string]any{
			"id":         t.ID,
			"machine_id": t.MachineID,
			"metrics":    metrics,
			"error_code": t.ErrorCode,
			"source":     t.Source,
			"created_at": t.CreatedAt,
		})
	}

	return c.JSON(http.StatusOK, map[string]any{"telemetry": out})
}
