package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/ncs26-orchestration/solution/apps/api/internal/compiler"
	"github.com/ncs26-orchestration/solution/apps/api/internal/engine"
	"github.com/ncs26-orchestration/solution/apps/api/internal/ir"
)

// CompileHandler owns the validator and the adapter registry. The
// registry lets the HTTP layer route /compile/:target to the right
// engine backend without importing camunda7 / elsa3 directly.
type CompileHandler struct {
	logger    *slog.Logger
	validator *ir.Validator
	adapters  *engine.Registry
}

func NewCompileHandler(logger *slog.Logger, adapters *engine.Registry) (*CompileHandler, error) {
	v, err := ir.NewValidator()
	if err != nil {
		return nil, err
	}
	return &CompileHandler{logger: logger, validator: v, adapters: adapters}, nil
}

// compileRequest accepts two shapes so callers can either send a bare
// IR at the top level or wrap it as { "ir": ..., "profile": "camunda7",
// "is_registry": ... }. The wrapped form is preferred; the bare form
// stays supported because curl-ing an example file is the fastest
// smoke test.
type compileRequest struct {
	IR         json.RawMessage `json:"ir,omitempty"`
	Profile    string          `json:"profile,omitempty"`
	Target     string          `json:"target,omitempty"`
	ISRegistry json.RawMessage `json:"is_registry,omitempty"`
}

type compileResponse struct {
	Artifact       string                     `json:"artifact"`
	Mime           string                     `json:"mime,omitempty"`
	Diagnostics    []ir.Diagnostic            `json:"diagnostics"`
	Target         string                     `json:"target,omitempty"`
	DecisionTables []compiler.DecisionTable   `json:"decision_tables,omitempty"`
}

type compileError struct {
	Error       string          `json:"error"`
	Diagnostics []ir.Diagnostic `json:"diagnostics,omitempty"`
}

// CompileBPMN is the legacy endpoint. Kept for backward compat and
// for curl-style smoke tests; delegates to the generic path with
// target="camunda7".
func (h *CompileHandler) CompileBPMN(c echo.Context) error {
	return h.compile(c, "camunda7")
}

// CompileForTarget is POST /compile/:target. The target path
// parameter selects the adapter from the registry; everything else
// (schema validation, cross-ref, lowering) is target-neutral.
func (h *CompileHandler) CompileForTarget(c echo.Context) error {
	target := c.Param("target")
	if target == "" {
		return c.JSON(http.StatusBadRequest, compileError{Error: "missing_target"})
	}
	return h.compile(c, target)
}

// ListAdapters powers GET /engines/adapters. The web target
// selector reads this to populate its dropdown and toggle Deploy
// button states from each adapter's Capabilities.
func (h *CompileHandler) ListAdapters(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{
		"adapters": h.adapters.List(),
	})
}

// AnalyzeDecisionTables powers POST /analyze/decision-tables. The
// Inspector calls this when the user clicks a gateway node so the
// consolidated-table view is available without waiting for a full
// compile. Accepts the same envelope as /compile ({ir} or bare IR).
func (h *CompileHandler) AnalyzeDecisionTables(c echo.Context) error {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "read_body: " + err.Error()})
	}
	var req compileRequest
	irBytes := body
	if err := json.Unmarshal(body, &req); err == nil && len(req.IR) > 0 {
		irBytes = req.IR
	}
	wf, diags, err := h.validator.ValidateWorkflowJSON(irBytes)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_json: " + err.Error()})
	}
	if len(diags) > 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid_ir", "diagnostics": diags})
	}
	tables := compiler.AnalyzeDecisionTables(wf)
	return c.JSON(http.StatusOK, map[string]any{"decision_tables": tables})
}

// compile is the shared pipeline for all targets: read body → validate
// schema → validate cross-refs against IS → Lower → adapter.Compile.
// Lowering + compile diagnostics merge into the response so the UI's
// Suggestions tab surfaces them uniformly across targets.
func (h *CompileHandler) compile(c echo.Context, target string) error {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, compileError{Error: "read_body: " + err.Error()})
	}

	// Peek at the envelope. If no "ir" field is present, treat the
	// whole body as the IR (bare form) — this is the shape of every
	// file under packages/ir/examples/.
	var req compileRequest
	irBytes := body
	var isBytes []byte
	if err := json.Unmarshal(body, &req); err == nil && len(req.IR) > 0 {
		irBytes = req.IR
		isBytes = req.ISRegistry
		// An explicit Target in the body wins over the path param —
		// lets the old-style /compile/bpmn route accept a different
		// target via body. Unusual but harmless.
		if req.Target != "" {
			target = req.Target
		}
	}

	adapter, err := h.adapters.Get(target)
	if err != nil {
		return c.JSON(http.StatusBadRequest, compileError{Error: "unknown_target: " + target})
	}

	wf, diags, err := h.validator.ValidateWorkflowJSON(irBytes)
	if err != nil {
		return c.JSON(http.StatusBadRequest, compileError{Error: "invalid_json: " + err.Error()})
	}
	if len(diags) > 0 {
		return c.JSON(http.StatusBadRequest, compileError{Error: "invalid_ir", Diagnostics: diags})
	}

	var registry *ir.ISRegistry
	if len(isBytes) > 0 {
		r, isDiags, err := h.validator.ValidateISRegistryJSON(isBytes)
		if err != nil {
			return c.JSON(http.StatusBadRequest, compileError{Error: "invalid_is_registry_json: " + err.Error()})
		}
		if len(isDiags) > 0 {
			return c.JSON(http.StatusBadRequest, compileError{Error: "invalid_is_registry", Diagnostics: isDiags})
		}
		registry = r
	}
	if crossDiags := h.validator.CrossRef(wf, registry); len(crossDiags) > 0 {
		return c.JSON(http.StatusBadRequest, compileError{Error: "unresolved_references", Diagnostics: crossDiags})
	}

	// ProcessIR → ExecutableIR. Lower's warnings flow through to the
	// response so the UI's Suggestions tab surfaces them.
	exe, loweringDiags, err := compiler.Lower(wf, registry)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, compileError{Error: "lowering_failed: " + err.Error()})
	}

	artifact, mime, compDiags, err := adapter.Compile(exe)
	allDiags := append(loweringDiags, compDiags...)
	if err != nil {
		return c.JSON(http.StatusBadRequest, compileError{Error: "compilation_refused: " + err.Error(), Diagnostics: allDiags})
	}

	// Analyze the executable IR for decision-table patterns so the
	// UI can render a consolidated table view for any exclusive
	// gateway that cascades on shared variables. Analysis is
	// read-only and target-agnostic — a single call here benefits
	// every adapter.
	tables := compiler.AnalyzeDecisionTables(exe)

	return c.JSON(http.StatusOK, compileResponse{
		Artifact:       string(artifact),
		Mime:           mime,
		Diagnostics:    allDiags,
		Target:         target,
		DecisionTables: tables,
	})
}
