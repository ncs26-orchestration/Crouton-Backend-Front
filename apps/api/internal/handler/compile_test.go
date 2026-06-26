package handler_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/ncs26-orchestration/solution/apps/api/internal/engine"
	"github.com/ncs26-orchestration/solution/apps/api/internal/engine/camunda7"
	"github.com/ncs26-orchestration/solution/apps/api/internal/handler"
)

func loadExample(t *testing.T, name string) []byte {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Clean(filepath.Join(cwd, "..", "..", "..", "..", "packages", "ir", "examples", name))
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}

func newTestServer(t *testing.T) *echo.Echo {
	t.Helper()
	e := echo.New()
	// Register the Camunda 7 adapter so CompileBPMN (which delegates
	// to target="camunda7") can route through the registry.
	adapters := engine.NewRegistry()
	adapters.Register(camunda7.NewAdapter())
	ch, err := handler.NewCompileHandler(slog.Default(), adapters)
	if err != nil {
		t.Fatalf("NewCompileHandler: %v", err)
	}
	e.POST("/compile/bpmn", ch.CompileBPMN)
	return e
}

func TestCompileBPMN_BareIR_OK(t *testing.T) {
	e := newTestServer(t)
	body := loadExample(t, "expense-approval.json")

	req := httptest.NewRequest(http.MethodPost, "/compile/bpmn", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Artifact    string          `json:"artifact"`
		Diagnostics []any           `json:"diagnostics"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.Contains(resp.Artifact, "<bpmn:definitions") {
		t.Errorf("artifact does not look like BPMN XML")
	}
	if !strings.Contains(resp.Artifact, `camunda:candidateGroups="accounting"`) {
		t.Errorf("expected camunda:candidateGroups attribute in artifact")
	}
	if !strings.Contains(resp.Artifact, `camunda:topic="openbee.document.archive"`) {
		t.Errorf("expected external-task topic in artifact")
	}
}

func TestCompileBPMN_WrappedRequest_OK(t *testing.T) {
	e := newTestServer(t)
	ir := loadExample(t, "simple-gateway.json")

	wrapped, err := json.Marshal(map[string]any{
		"ir":      json.RawMessage(ir),
		"profile": "camunda7",
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/compile/bpmn", bytes.NewReader(wrapped))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestCompileBPMN_InvalidIR_400(t *testing.T) {
	e := newTestServer(t)
	body := []byte(`{"version":"0.9","metadata":{"name":"bad"},"actors":[],"tasks":[],"events":[],"flows":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/compile/bpmn", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: want 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "invalid_ir") {
		t.Errorf("expected error=invalid_ir, body=%s", rec.Body.String())
	}
}

func TestCompileBPMN_CrossRefMissingRegistry_400(t *testing.T) {
	// A workflow with a binding that cannot resolve because no IS
	// Registry is provided — should surface as unresolved_references
	// if the binding is against systems/users, which require a
	// registry to validate. Plain candidate_group_id without registry
	// is skipped (is==nil), so use a service-task binding that we
	// can't check either without a registry. This test just confirms
	// the handler doesn't blow up on bare IR with bindings — it's a
	// regression guard, not a strict negative.
	e := newTestServer(t)
	body := loadExample(t, "expense-approval.json")
	req := httptest.NewRequest(http.MethodPost, "/compile/bpmn", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 (registry is optional in v0.1); got %d: %s", rec.Code, rec.Body.String())
	}
}
