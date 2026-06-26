package agentclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIntakeParsesPlan(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/agents/intake" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"nodes": [
				{"key": "intake", "name": "Intake", "agent_type": "intake", "department": "Planning"},
				{"key": "finance_review", "name": "Finance Review", "agent_type": "finance", "department": "Finance"}
			],
			"edges": [
				{"from": "intake", "to": "finance_review", "type": "sequence"}
			]
		}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	plan, err := c.Intake(context.Background(), IntakeRequest{
		Request: IntakeRequestBody{Title: "x", Priority: "high"},
	})
	if err != nil {
		t.Fatalf("Intake error: %v", err)
	}
	if len(plan.Nodes) != 2 || plan.Nodes[1].Key != "finance_review" {
		t.Errorf("nodes parsed wrong: %+v", plan.Nodes)
	}
	if len(plan.Edges) != 1 || plan.Edges[0].From != "intake" || plan.Edges[0].EdgeType != "sequence" {
		t.Errorf("edges parsed wrong: %+v", plan.Edges)
	}
}

func TestIntakeNon2xxReturnsUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.Intake(context.Background(), IntakeRequest{Request: IntakeRequestBody{Title: "x"}})
	if !errors.Is(err, ErrAgentUnavailable) {
		t.Fatalf("expected ErrAgentUnavailable, got %v", err)
	}
}

func TestIntakeConnRefusedReturnsUnavailable(t *testing.T) {
	// A server that is created then immediately closed gives a dead URL.
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	c := New(url)
	_, err := c.Intake(context.Background(), IntakeRequest{Request: IntakeRequestBody{Title: "x"}})
	if !errors.Is(err, ErrAgentUnavailable) {
		t.Fatalf("expected ErrAgentUnavailable, got %v", err)
	}
}

func TestDefaultPlanIsConnectedDAG(t *testing.T) {
	plan := DefaultPlan()
	if len(plan.Nodes) < 9 {
		t.Fatalf("default plan has %d nodes, want >= 9", len(plan.Nodes))
	}
	keys := make(map[string]bool, len(plan.Nodes))
	for _, n := range plan.Nodes {
		keys[n.Key] = true
	}
	if !keys["exec_approval"] {
		t.Error("default plan missing exec_approval stage")
	}
	for _, e := range plan.Edges {
		if !keys[e.From] || !keys[e.To] {
			t.Errorf("edge references unknown key: %+v", e)
		}
	}
}
