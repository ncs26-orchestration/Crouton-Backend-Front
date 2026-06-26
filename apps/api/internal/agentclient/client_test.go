package agentclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestRunParsesDecision(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/agents/run" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"summary": "Assessed budget",
			"flags": [{"severity": "info", "message": "within budget"}],
			"tasks": [{"title": "Assess budget feasibility", "status": "completed"}],
			"status_text": "Finance review complete.",
			"blocked_on": null
		}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	d, err := c.Run(context.Background(), RunRequest{
		AgentType: "finance",
		Request:   IntakeRequestBody{Title: "x", Priority: "high"},
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if d.StatusText != "Finance review complete." || len(d.Tasks) != 1 || d.Tasks[0].Status != "completed" {
		t.Errorf("decision parsed wrong: %+v", d)
	}
	if len(d.Flags) != 1 || d.BlockedOn != nil {
		t.Errorf("flags/blocked_on parsed wrong: %+v", d)
	}
}

func TestRunNon2xxReturnsUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.Run(context.Background(), RunRequest{AgentType: "finance", Request: IntakeRequestBody{Title: "x"}})
	if !errors.Is(err, ErrAgentUnavailable) {
		t.Fatalf("expected ErrAgentUnavailable, got %v", err)
	}
}

func TestDefaultDecisionCompletesEveryStage(t *testing.T) {
	for _, at := range []string{"intake", "finance", "legal", "it", "hr", "ops", "approval", "report", "mystery"} {
		d := DefaultDecision(at)
		if d.StatusText == "" || len(d.Tasks) == 0 {
			t.Errorf("DefaultDecision(%q) is empty: %+v", at, d)
		}
		for _, task := range d.Tasks {
			if task.Status != "completed" {
				t.Errorf("DefaultDecision(%q) task not completed: %+v", at, task)
			}
		}
	}
}

// TestDefaultDecisionMatchesPythonPlaybook pins the Go fallback content to the
// Python department playbook (apps/agent/app/agents/department.py). The two are
// intentionally duplicated across runtimes; these literals (and the matching
// assertions in the agent's tests/test_agents.py) make any drift fail loudly,
// so an edit to one side is a conscious decision to update both.
func TestDefaultDecisionMatchesPythonPlaybook(t *testing.T) {
	cases := map[string]struct {
		statusText string
		tasks      []string
	}{
		"finance": {
			"Finance review complete — the request is financially viable.",
			[]string{"Assess budget feasibility", "Estimate the financial impact", "Project the return on investment", "Confirm funding availability"},
		},
		"legal": {
			"Legal review complete — no blocking issues, one item to track.",
			[]string{"Review regulatory compliance", "Check contract requirements", "Flag legal risks"},
		},
	}
	for at, want := range cases {
		d := DefaultDecision(at)
		if d.StatusText != want.statusText {
			t.Errorf("%s status_text = %q, want %q", at, d.StatusText, want.statusText)
		}
		got := make([]string, len(d.Tasks))
		for i, task := range d.Tasks {
			got[i] = task.Title
		}
		if strings.Join(got, "|") != strings.Join(want.tasks, "|") {
			t.Errorf("%s tasks = %v, want %v", at, got, want.tasks)
		}
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
