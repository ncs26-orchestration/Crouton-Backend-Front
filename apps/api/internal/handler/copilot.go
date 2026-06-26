package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/ncs26-orchestration/solution/apps/api/internal/ir"
	"github.com/ncs26-orchestration/solution/apps/api/internal/repo"
)

// CopilotHandler proxies Ask/Clarify/Apply requests to the agent
// service, re-validating Apply's output against the IR schema and
// cross-refs before returning. The pattern mirrors ExtractHandler —
// the Go API owns validation; the agent owns the LLM surface.
type CopilotHandler struct {
	logger    *slog.Logger
	validator *ir.Validator
	pg        *pgxpool.Pool
	engines   *repo.EngineRepo
	agentURL  string
	http      *http.Client
}

func NewCopilotHandler(logger *slog.Logger, pg *pgxpool.Pool, agentURL string) (*CopilotHandler, error) {
	v, err := ir.NewValidator()
	if err != nil {
		return nil, err
	}
	return &CopilotHandler{
		logger:    logger,
		validator: v,
		pg:        pg,
		engines:   repo.NewEngineRepo(pg),
		agentURL:  agentURL,
		http:      &http.Client{},
	}, nil
}

// copilotRequest shape matches what the web sends. For Ask + Clarify
// the Go API injects the tenant's IS Registry server-side so the
// browser never has to pass it in.
type copilotRequest struct {
	IR         json.RawMessage `json:"ir"`
	TenantID   string          `json:"tenant_id,omitempty"`
	Question   string          `json:"question,omitempty"`
	Kind       string          `json:"kind,omitempty"`
	ElementID  string          `json:"element_id,omitempty"`
	Current    json.RawMessage `json:"current,omitempty"`
	Evidence   string          `json:"evidence,omitempty"`
	Confidence *float64        `json:"confidence,omitempty"`
	Patch      json.RawMessage `json:"patch,omitempty"`
}

func (h *CopilotHandler) Ask(c echo.Context) error {
	return h.proxyWithIS(c, "/copilot/ask")
}

func (h *CopilotHandler) Clarify(c echo.Context) error {
	return h.proxyWithIS(c, "/copilot/clarify")
}

// proxyWithIS reads the tenant's IS Registry from the DB and forwards
// the request to the agent with is_registry pre-populated.
func (h *CopilotHandler) proxyWithIS(c echo.Context, path string) error {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "read_body: " + err.Error()})
	}
	var req copilotRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_json: " + err.Error()})
	}
	if len(req.IR) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "missing_ir"})
	}
	tenant := req.TenantID
	if tenant == "" {
		tenant = "demo"
	}

	proj, err := h.engines.ReadTenantProjection(c.Request().Context(), tenant)
	if err != nil {
		h.logger.Error("read projection", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db_error"})
	}
	reg := buildRegistry(proj)
	regBytes, _ := json.Marshal(reg)

	// Re-serialize with is_registry merged in — avoids the agent
	// needing to know about AIOS's tenant model.
	var envelope map[string]any
	_ = json.Unmarshal(body, &envelope)
	if envelope == nil {
		envelope = map[string]any{}
	}
	envelope["is_registry"] = json.RawMessage(regBytes)
	out, err := json.Marshal(envelope)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "repack: " + err.Error()})
	}

	return h.forwardJSON(c, path, out)
}

// Apply is different from Ask/Clarify — it ends with validation. The
// agent applies the JSON-Patch; we then validate the returned IR
// against the schema + the tenant's IS before echoing it back.
func (h *CopilotHandler) Apply(c echo.Context) error {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "read_body: " + err.Error()})
	}
	var req copilotRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_json: " + err.Error()})
	}
	if len(req.IR) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "missing_ir"})
	}
	if len(req.Patch) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "missing_patch"})
	}

	// Forward verbatim; agent returns { ir, error? }.
	resp, err := h.forwardRaw("/copilot/apply", body)
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "agent_unreachable", "details": err.Error()})
	}
	var result struct {
		IR         json.RawMessage `json:"ir"`
		Error      string          `json:"error,omitempty"`
		Normalized bool            `json:"normalized,omitempty"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "agent_bad_response", "details": err.Error()})
	}
	if result.Error != "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": result.Error})
	}
	if len(result.IR) == 0 {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "agent_returned_no_ir"})
	}

	// Validate schema + cross-refs. Any violation rejects the patch.
	wf, diags, err := h.validator.ValidateWorkflowJSON(result.IR)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "patched_ir_invalid_json", "details": err.Error()})
	}
	if len(diags) > 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "patched_ir_schema_violation", "diagnostics": diags})
	}

	tenant := req.TenantID
	if tenant == "" {
		tenant = "demo"
	}
	proj, err := h.engines.ReadTenantProjection(c.Request().Context(), tenant)
	if err == nil {
		if cross := h.validator.CrossRef(wf, buildRegistry(proj)); len(cross) > 0 {
			// Cross-ref issues are warnings (the user may be patching
			// toward an intentionally unresolved state); include them
			// but let the patch land.
			return c.JSON(http.StatusOK, map[string]any{
				"ir":          wf,
				"diagnostics": cross,
				"normalized":  result.Normalized,
			})
		}
	}
	return c.JSON(http.StatusOK, map[string]any{"ir": wf, "normalized": result.Normalized})
}

// forwardJSON is used by Ask/Clarify — it sends `body` to the agent
// and echoes the agent's response 1:1 back to the caller.
func (h *CopilotHandler) forwardJSON(c echo.Context, path string, body []byte) error {
	resp, err := h.forwardRaw(path, body)
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "agent_unreachable", "details": err.Error()})
	}
	return c.JSONBlob(http.StatusOK, resp)
}

// forwardRaw is the lowest-level HTTP client call. Returns the agent's
// response body bytes when status is 2xx, else a synthetic error.
func (h *CopilotHandler) forwardRaw(path string, body []byte) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, h.agentURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/json")
	resp, err := h.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("agent %d: %s", resp.StatusCode, string(raw))
	}
	return raw, nil
}
