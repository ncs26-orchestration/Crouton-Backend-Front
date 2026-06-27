package handler

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/ncs26-orchestration/solution/apps/api/internal/agentclient"
	"github.com/ncs26-orchestration/solution/apps/api/internal/middleware"
	"github.com/ncs26-orchestration/solution/apps/api/internal/repo"
)

type DiagnosisHandler struct {
	logger *slog.Logger
	db     *pgxpool.Pool
	diag   *repo.DiagnosisRepo
	mach   *repo.MachineRepo
	inc    *repo.IncidentRepo
	agent  *agentclient.Client
}

func NewDiagnosisHandler(logger *slog.Logger, db *pgxpool.Pool, agentClient *agentclient.Client) *DiagnosisHandler {
	return &DiagnosisHandler{
		logger: logger,
		db:     db,
		diag:   repo.NewDiagnosisRepo(db),
		mach:   repo.NewMachineRepo(db),
		inc:    repo.NewIncidentRepo(db),
		agent:  agentClient,
	}
}

// ── Upload Machine Document ────────────────────────────────────────

// UploadMachineDocument handles POST /machines/:id/documents.
// Accepts multipart form with fields: doc_type, and a file upload.
func (h *DiagnosisHandler) UploadMachineDocument(c echo.Context) error {
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
		h.logger.Error("upload doc: get machine", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	if _, err := requireOrgMember(c, h.db, machine.OrgID, claims.UserID); err != nil {
		return handleOrgMemberErr(c, err)
	}

	docType := c.FormValue("doc_type")
	if docType == "" {
		docType = "manual"
	}

	file, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "file is required"})
	}

	const maxSize = 8 << 20 // 8 MB
	if file.Size > maxSize {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "file too large (max 8MB)"})
	}

	src, err := file.Open()
	if err != nil {
		h.logger.Error("upload doc: open file", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	defer src.Close()

	content, err := io.ReadAll(io.LimitReader(src, maxSize))
	if err != nil {
		h.logger.Error("upload doc: read file", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	// Extract text based on file extension.
	ext := strings.ToLower(filepath.Ext(file.Filename))
	var extractedText string
	switch ext {
	case ".txt", ".md":
		extractedText = string(content)
	default:
		extractedText = fmt.Sprintf("[Document uploaded: %s — text extraction pending]", file.Filename)
	}

	doc, err := h.diag.InsertDocument(ctx, repo.MachineDocument{
		ID:            fmt.Sprintf("mdoc_%s", randomHex(8)),
		MachineID:     machineID,
		OrgID:         machine.OrgID,
		UploadedBy:    claims.UserID,
		Filename:      file.Filename,
		ContentType:   file.Header.Get("Content-Type"),
		FileSizeBytes: file.Size,
		DocType:       docType,
		ExtractedText: extractedText,
	})
	if err != nil {
		h.logger.Error("upload doc: db insert", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	return c.JSON(http.StatusCreated, map[string]any{"document": docToJSON(doc)})
}

// ── List Machine Documents ─────────────────────────────────────────

// ListMachineDocuments handles GET /machines/:id/documents.
func (h *DiagnosisHandler) ListMachineDocuments(c echo.Context) error {
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
		h.logger.Error("list docs: get machine", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	if _, err := requireOrgMember(c, h.db, machine.OrgID, claims.UserID); err != nil {
		return handleOrgMemberErr(c, err)
	}

	docs, err := h.diag.ListDocumentsByMachine(ctx, machineID)
	if err != nil {
		h.logger.Error("list docs: db", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	out := make([]map[string]any, 0, len(docs))
	for _, d := range docs {
		out = append(out, docToJSON(&d))
	}

	return c.JSON(http.StatusOK, map[string]any{"documents": out})
}

// ── Request Diagnosis ──────────────────────────────────────────────

// RequestDiagnosis handles POST /incidents/:id/diagnose.
func (h *DiagnosisHandler) RequestDiagnosis(c echo.Context) error {
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
		h.logger.Error("diagnose: get incident", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	if _, err := requireOrgMember(c, h.db, inc.OrgID, claims.UserID); err != nil {
		return handleOrgMemberErr(c, err)
	}

	// Get the machine linked to the incident.
	machine, err := h.mach.GetByID(ctx, inc.MachineID)
	if err != nil {
		h.logger.Error("diagnose: get machine", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	// Collect all machine documents (manuals) and concatenate their text.
	docs, err := h.diag.ListDocumentsByMachine(ctx, inc.MachineID)
	if err != nil {
		h.logger.Error("diagnose: list docs", slog.String("err", err.Error()))
		// Non-fatal — proceed without manual text.
		docs = nil
	}
	var manualParts []string
	for _, d := range docs {
		if d.ExtractedText != "" {
			manualParts = append(manualParts, fmt.Sprintf("--- %s ---\n%s", d.Filename, d.ExtractedText))
		}
	}
	manualText := strings.Join(manualParts, "\n\n")

	// Call the AI diagnostic agent.
	diagReq := agentclient.DiagnoseRequest{
		IncidentTitle:       inc.Title,
		IncidentDescription: inc.Description,
		Severity:            inc.Severity,
		MachineName:         machine.Name,
		MachineType:         machine.MachineType,
		ManualText:          manualText,
		Telemetry:           map[string]any{},
	}

	result, err := h.agent.Diagnose(ctx, diagReq)
	if err != nil {
		h.logger.Warn("diagnose: agent unavailable, using fallback", slog.String("err", err.Error()))
		result = agentclient.DefaultDiagnosis(inc.Severity)
	}

	// Persist the diagnosis record.
	diagID := fmt.Sprintf("diag_%s", randomHex(8))
	diagRecord, err := h.diag.CreateDiagnosis(ctx, repo.DiagnosisRecord{
		ID:         diagID,
		IncidentID: incidentID,
		MachineID:  inc.MachineID,
		AgentModel: "diagnostic-agent",
		Summary:    result.Summary,
		RootCause:  result.RootCause,
		Status:     "in_progress",
	})
	if err != nil {
		h.logger.Error("diagnose: create diagnosis", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	// Persist the diagnosis steps.
	steps := make([]repo.DiagnosisStep, 0, len(result.Steps))
	for i, s := range result.Steps {
		steps = append(steps, repo.DiagnosisStep{
			ID:              fmt.Sprintf("ds_%s", randomHex(8)),
			DiagnosisID:     diagID,
			StepOrder:       i + 1,
			Title:           s.Title,
			Description:     s.Description,
			ActionType:      s.ActionType,
			ExpectedOutcome: s.ExpectedOutcome,
			Warning:         s.Warning,
			Status:          "pending",
		})
	}
	if err := h.diag.InsertSteps(ctx, steps); err != nil {
		h.logger.Error("diagnose: insert steps", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	return c.JSON(http.StatusCreated, map[string]any{
		"diagnosis": diagToJSON(diagRecord),
		"steps":     stepsToJSON(steps),
	})
}

// ── Get Diagnosis ──────────────────────────────────────────────────

// GetDiagnosis handles GET /incidents/:id/diagnosis.
func (h *DiagnosisHandler) GetDiagnosis(c echo.Context) error {
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
		h.logger.Error("get diagnosis: get incident", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	if _, err := requireOrgMember(c, h.db, inc.OrgID, claims.UserID); err != nil {
		return handleOrgMemberErr(c, err)
	}

	diag, err := h.diag.GetDiagnosisByIncident(ctx, incidentID)
	if errors.Is(err, repo.ErrNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "no diagnosis found for this incident"})
	}
	if err != nil {
		h.logger.Error("get diagnosis: db", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	steps, err := h.diag.ListSteps(ctx, diag.ID)
	if err != nil {
		h.logger.Error("get diagnosis: list steps", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"diagnosis": diagToJSON(diag),
		"steps":     stepsToJSON(steps),
	})
}

// ── Complete Step ──────────────────────────────────────────────────

// CompleteStep handles POST /diagnosis/steps/:stepId/complete.
// Body: { notes: "..." }
func (h *DiagnosisHandler) CompleteStep(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	stepID := c.Param("stepId")
	ctx := c.Request().Context()

	var body struct {
		Notes string `json:"notes"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	// Look up the step to find its diagnosis_id.
	var diagnosisID string
	err := h.db.QueryRow(ctx,
		`SELECT diagnosis_id FROM diagnosis_steps WHERE id = $1`, stepID,
	).Scan(&diagnosisID)
	if errors.Is(err, pgx.ErrNoRows) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "step not found"})
	}
	if err != nil {
		h.logger.Error("complete step: lookup step", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	// Mark the step as completed.
	if err := h.diag.CompleteStep(ctx, stepID, body.Notes); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "step not found or already completed"})
		}
		h.logger.Error("complete step: db", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	// Check if all steps for this diagnosis are now completed.
	var pending int
	err = h.db.QueryRow(ctx,
		`SELECT count(*) FROM diagnosis_steps WHERE diagnosis_id = $1 AND status != 'completed'`, diagnosisID,
	).Scan(&pending)
	if err != nil {
		h.logger.Error("complete step: count pending", slog.String("err", err.Error()))
	}
	if err == nil && pending == 0 {
		if err := h.diag.CompleteDiagnosis(ctx, diagnosisID); err != nil {
			h.logger.Error("complete step: complete diagnosis", slog.String("err", err.Error()))
		}
	}

	// Return the updated step.
	var step repo.DiagnosisStep
	row := h.db.QueryRow(ctx,
		`SELECT id, diagnosis_id, step_order, title, description, action_type, expected_outcome, warning, status, notes, completed_at, created_at
		 FROM diagnosis_steps WHERE id = $1`, stepID)
	if err := row.Scan(
		&step.ID, &step.DiagnosisID, &step.StepOrder, &step.Title,
		&step.Description, &step.ActionType, &step.ExpectedOutcome, &step.Warning,
		&step.Status, &step.Notes, &step.CompletedAt, &step.CreatedAt,
	); err != nil {
		h.logger.Error("complete step: read updated step", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	return c.JSON(http.StatusOK, map[string]any{"step": stepToJSON(step)})
}

// ── JSON helpers ───────────────────────────────────────────────────

func docToJSON(d *repo.MachineDocument) map[string]any {
	return map[string]any{
		"id":              d.ID,
		"machine_id":      d.MachineID,
		"org_id":          d.OrgID,
		"uploaded_by":     d.UploadedBy,
		"filename":        d.Filename,
		"content_type":    d.ContentType,
		"file_size_bytes": d.FileSizeBytes,
		"doc_type":        d.DocType,
		"extracted_text":  d.ExtractedText,
		"created_at":      d.CreatedAt,
	}
}

func diagToJSON(d *repo.DiagnosisRecord) map[string]any {
	m := map[string]any{
		"id":          d.ID,
		"incident_id": d.IncidentID,
		"machine_id":  d.MachineID,
		"agent_model": d.AgentModel,
		"summary":     d.Summary,
		"root_cause":  d.RootCause,
		"status":      d.Status,
		"created_at":  d.CreatedAt,
	}
	if d.CompletedAt != nil {
		m["completed_at"] = d.CompletedAt
	}
	return m
}

func stepToJSON(s repo.DiagnosisStep) map[string]any {
	m := map[string]any{
		"id":               s.ID,
		"diagnosis_id":     s.DiagnosisID,
		"step_order":       s.StepOrder,
		"title":            s.Title,
		"description":      s.Description,
		"action_type":      s.ActionType,
		"expected_outcome": s.ExpectedOutcome,
		"warning":          s.Warning,
		"status":           s.Status,
		"notes":            s.Notes,
		"created_at":       s.CreatedAt,
	}
	if s.CompletedAt != nil {
		m["completed_at"] = s.CompletedAt
	}
	return m
}

func stepsToJSON(steps []repo.DiagnosisStep) []map[string]any {
	out := make([]map[string]any, 0, len(steps))
	for _, s := range steps {
		out = append(out, stepToJSON(s))
	}
	return out
}
