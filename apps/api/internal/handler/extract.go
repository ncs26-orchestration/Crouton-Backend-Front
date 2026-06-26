package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/ncs26-orchestration/solution/apps/api/internal/ir"
	"github.com/ncs26-orchestration/solution/apps/api/internal/repo"
	"github.com/ncs26-orchestration/solution/apps/api/internal/service"
)

// ExtractHandler forwards text + the tenant's projected IS Registry to
// the Python agent's /extract endpoint, then re-validates the returned
// IR with the Go cross-ref validator before handing it back.
//
// Orchestration shape:
//
//	client ──POST /extract──▶ Go API
//	                            │
//	                            │  1. Read tenant IS projection from Postgres
//	                            │  2. Forward {text, is_registry} to agent
//	                            │  3. Cross-ref validate returned IR
//	                            │
//	                            ▼
//	                          Go API ──────▶ client { ir, diagnostics, error? }
type ExtractHandler struct {
	logger    *slog.Logger
	engines   *repo.EngineRepo
	validator *ir.Validator
	agentURL  string
	client    *http.Client
}

func NewExtractHandler(logger *slog.Logger, pg *pgxpool.Pool, agentURL string) (*ExtractHandler, error) {
	v, err := ir.NewValidator()
	if err != nil {
		return nil, err
	}
	return &ExtractHandler{
		logger:    logger,
		engines:   repo.NewEngineRepo(pg),
		validator: v,
		agentURL:  strings.TrimRight(agentURL, "/"),
		client:    &http.Client{Timeout: 45 * time.Second},
	}, nil
}

type extractRequest struct {
	TenantID string `json:"tenant_id,omitempty"`
	Text     string `json:"text"`
}

type extractResponse struct {
	IR          json.RawMessage `json:"ir,omitempty"`
	Diagnostics []ir.Diagnostic `json:"diagnostics"`
	Error       string          `json:"error,omitempty"`
}

// Extract runs the end-to-end flow and returns either a validated IR,
// a schema/cross-ref error with diagnostics, or an upstream failure.
func (h *ExtractHandler) Extract(c echo.Context) error {
	var req extractRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_json"})
	}
	tenantID := orDefault(strings.TrimSpace(req.TenantID), "demo")
	text := strings.TrimSpace(req.Text)
	if text == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "empty_text"})
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 60*time.Second)
	defer cancel()

	// 1. Read the projection and build an IS Registry in the shape the
	//    agent's prompt expects.
	proj, err := h.engines.ReadTenantProjection(ctx, tenantID)
	if err != nil {
		h.logger.Error("read tenant projection", "err", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db_error"})
	}
	registry := service.BuildISRegistry(proj)
	if registry.Users == nil {
		registry.Users = []ir.ISUser{}
	}
	if registry.Groups == nil {
		registry.Groups = []ir.ISGroup{}
	}
	if registry.Systems == nil {
		registry.Systems = []ir.ISSystem{}
	}

	// 2. Forward to the agent.
	agentReqBody, _ := json.Marshal(map[string]any{
		"text":        text,
		"is_registry": registry,
	})
	agentResp, err := h.callAgent(ctx, agentReqBody)
	if err != nil {
		h.logger.Error("call agent /extract", "err", err)
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "agent_unreachable", "details": err.Error()})
	}

	if agentResp.IR == nil {
		return c.JSON(http.StatusBadGateway, extractResponse{
			Error:       firstNonEmpty(agentResp.Error, "agent returned no IR"),
			Diagnostics: []ir.Diagnostic{},
		})
	}

	// 3. Cross-ref-validate the IR against the IS we just handed out.
	rawIR, err := json.Marshal(agentResp.IR)
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "agent_returned_unserializable_ir"})
	}
	wf, schemaDiags, err := h.validator.ValidateWorkflowJSON(rawIR)
	if err != nil {
		return c.JSON(http.StatusBadGateway, extractResponse{
			IR:          rawIR,
			Diagnostics: []ir.Diagnostic{{Severity: "error", Message: err.Error()}},
			Error:       "agent_ir_unparseable",
		})
	}
	if len(schemaDiags) > 0 {
		return c.JSON(http.StatusUnprocessableEntity, extractResponse{
			IR:          rawIR,
			Diagnostics: schemaDiags,
			Error:       "ir_schema_violation",
		})
	}
	crossDiags := h.validator.CrossRef(wf, registry)
	if len(crossDiags) > 0 {
		// IR is structurally fine but references unknown ids. The UI
		// surfaces these as gap cards; we still return 200 so the
		// caller can render the IR and walk the user through fixes.
		return c.JSON(http.StatusOK, extractResponse{
			IR:          rawIR,
			Diagnostics: crossDiags,
		})
	}
	return c.JSON(http.StatusOK, extractResponse{
		IR:          rawIR,
		Diagnostics: []ir.Diagnostic{},
	})
}

// agentExtractEnvelope matches apps/agent/app/api/extract.py's response.
type agentExtractEnvelope struct {
	IR    map[string]any `json:"ir"`
	Error string         `json:"error,omitempty"`
}

func (h *ExtractHandler) callAgent(ctx context.Context, body []byte) (*agentExtractEnvelope, error) {
	url := h.agentURL + "/extract"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/json")
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("agent %d: %s", resp.StatusCode, truncate(string(raw), 400))
	}
	var env agentExtractEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("decode agent response: %w (raw: %s)", err, truncate(string(raw), 400))
	}
	return &env, nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
