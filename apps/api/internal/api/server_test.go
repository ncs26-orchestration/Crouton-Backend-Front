package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Router wiring smoke test: a server with no DB pool still builds a router,
// and unknown routes return 404. DB-backed handlers are covered by integration
// tests against a real Postgres in CI.
func TestRouterNotFound(t *testing.T) {
	s := NewServer(nil)
	e := s.Router()

	req := httptest.NewRequest(http.MethodGet, "/does-not-exist", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
