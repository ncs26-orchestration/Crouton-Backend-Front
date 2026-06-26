package agentclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ErrAgentUnavailable is returned when the agent service cannot be reached
// or returns a non-2xx status. Callers use this to trigger deterministic
// fallback logic so a request never stalls.
var ErrAgentUnavailable = errors.New("agent service unavailable")

// PlanNode is a single stage the intake agent planned.
type PlanNode struct {
	Key        string `json:"key"`
	Name       string `json:"name"`
	AgentType  string `json:"agent_type"`
	Department string `json:"department"`
}

// PlanEdge connects two stages.
type PlanEdge struct {
	From     string `json:"from"`
	To       string `json:"to"`
	EdgeType string `json:"type"`
}

// Plan is the intake agent's output: a workflow graph for a request.
type Plan struct {
	Nodes []PlanNode `json:"nodes"`
	Edges []PlanEdge `json:"edges"`
}

// IntakeRequest is sent to POST /agents/intake.
type IntakeRequest struct {
	Request    IntakeRequestBody `json:"request"`
	OrgContext map[string]any    `json:"org_context"`
}

type IntakeRequestBody struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
}

// Client talks to the Python agent service.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New creates a Client pointing at the given base URL.
func New(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Intake calls POST /agents/intake and returns the planned workflow graph.
func (c *Client) Intake(ctx context.Context, ir IntakeRequest) (*Plan, error) {
	body, err := json.Marshal(ir)
	if err != nil {
		return nil, fmt.Errorf("marshal intake request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/agents/intake", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAgentUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: status %d", ErrAgentUnavailable, resp.StatusCode)
	}

	var plan Plan
	if err := json.NewDecoder(resp.Body).Decode(&plan); err != nil {
		return nil, fmt.Errorf("decode plan: %w", err)
	}
	return &plan, nil
}

// ── Department run (POST /agents/run) ───────────────────────────────────────

// Flag is a risk or note a department agent surfaces.
type Flag struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// TaskItem is a unit of work a department agent performed on a node.
type TaskItem struct {
	Title  string `json:"title"`
	Status string `json:"status"`
}

// DependencyDecl is a cross-department dependency an agent declares (F5).
type DependencyDecl struct {
	OnDepartment string `json:"on_department"`
	Reason       string `json:"reason"`
}

// Decision is a department agent's output for one workflow node.
type Decision struct {
	Summary    string          `json:"summary"`
	Flags      []Flag          `json:"flags"`
	Tasks      []TaskItem      `json:"tasks"`
	StatusText string          `json:"status_text"`
	BlockedOn  *DependencyDecl `json:"blocked_on"`
}

// UpstreamItem is a completed predecessor node's summary, passed so a
// department can reason over upstream output.
type UpstreamItem struct {
	Key        string `json:"key"`
	Department string `json:"department"`
	Summary    string `json:"summary"`
}

// RunRequest is sent to POST /agents/run.
type RunRequest struct {
	AgentType       string            `json:"agent_type"`
	Request         IntakeRequestBody `json:"request"`
	UpstreamContext []UpstreamItem    `json:"upstream_context"`
	OrgContext      map[string]any    `json:"org_context"`
}

// Run calls POST /agents/run and returns the department's decision.
func (c *Client) Run(ctx context.Context, rr RunRequest) (*Decision, error) {
	body, err := json.Marshal(rr)
	if err != nil {
		return nil, fmt.Errorf("marshal run request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/agents/run", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAgentUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: status %d", ErrAgentUnavailable, resp.StatusCode)
	}

	var decision Decision
	if err := json.NewDecoder(resp.Body).Decode(&decision); err != nil {
		return nil, fmt.Errorf("decode decision: %w", err)
	}
	return &decision, nil
}

// defaultDecisions mirrors the Python department playbook so the engine and
// the demo seed can produce believable, deterministic decisions when the
// agent service is unavailable. Keyed by agent_type.
var defaultDecisions = map[string]struct {
	summary    string
	statusText string
	tasks      []string
}{
	"intake":         {"Classified the request and identified the departments involved.", "Intake complete — routed to planning.", []string{"Classify the request type", "Identify involved departments", "Set initial priority"}},
	"planning":       {"Outlined the cross-department plan and review sequencing.", "Planning complete — department reviews can begin.", []string{"Draft the execution plan", "Sequence department reviews", "Estimate the timeline"}},
	"finance":        {"Assessed budget feasibility, financial impact, and ROI.", "Finance review complete — the request is financially viable.", []string{"Assess budget feasibility", "Estimate the financial impact", "Project the return on investment", "Confirm funding availability"}},
	"legal":          {"Checked regulatory compliance and contractual requirements.", "Legal review complete — no blocking issues, one item to track.", []string{"Review regulatory compliance", "Check contract requirements", "Flag legal risks"}},
	"it":             {"Evaluated technical feasibility, security, and systems integration.", "IT assessment complete — technically feasible with standard provisioning.", []string{"Assess technical feasibility", "Review security requirements", "Plan systems integration"}},
	"hr":             {"Planned staffing and hiring needs.", "HR planning complete — staffing plan ready.", []string{"Plan staffing needs", "Outline the hiring timeline", "Identify onboarding steps"}},
	"ops":            {"Planned logistics, facilities, and the operational timeline.", "Operations planning complete — execution plan ready.", []string{"Plan logistics", "Arrange facilities", "Set the operational timeline"}},
	"approval":       {"Compiled department decisions and flags for executive review.", "Ready for executive approval.", []string{"Compile department decisions", "Summarize flags and risks", "Prepare the approval packet"}},
	"implementation": {"Executed the approved plan across departments.", "Implementation complete.", []string{"Kick off execution", "Coordinate the departments", "Track delivery milestones"}},
	"report":         {"Produced the final report of decisions, flags, and outcomes.", "Final report generated.", []string{"Summarize approvals", "Compile flags", "Record time taken"}},
}

// DefaultDecision returns a deterministic decision for an agent type, used as
// the never-stall fallback when the agent service errors and to populate the
// demo seed. Unknown types still complete with a generic decision.
func DefaultDecision(agentType string) *Decision {
	spec, ok := defaultDecisions[agentType]
	if !ok {
		spec = struct {
			summary    string
			statusText string
			tasks      []string
		}{
			summary:    "Reviewed the request for the " + agentType + " stage.",
			statusText: "Stage complete.",
			tasks:      []string{"Review the request", "Record the outcome"},
		}
	}
	tasks := make([]TaskItem, 0, len(spec.tasks))
	for _, t := range spec.tasks {
		tasks = append(tasks, TaskItem{Title: t, Status: "completed"})
	}
	return &Decision{Summary: spec.summary, Flags: []Flag{}, Tasks: tasks, StatusText: spec.statusText}
}

// DefaultPlan returns a deterministic fallback plan when the agent service
// is unavailable. This ensures a request always gets a workflow graph.
func DefaultPlan() *Plan {
	return &Plan{
		Nodes: []PlanNode{
			{Key: "intake", Name: "Intake & Classification", AgentType: "intake", Department: "Planning"},
			{Key: "planning", Name: "Strategic Planning", AgentType: "planning", Department: "Planning"},
			{Key: "finance_review", Name: "Finance Review", AgentType: "finance", Department: "Finance"},
			{Key: "legal_review", Name: "Legal Review", AgentType: "legal", Department: "Legal"},
			{Key: "it_assessment", Name: "IT Assessment", AgentType: "it", Department: "IT"},
			{Key: "hr_planning", Name: "HR Planning", AgentType: "hr", Department: "HR"},
			{Key: "ops_planning", Name: "Operations Planning", AgentType: "ops", Department: "Operations"},
			{Key: "exec_approval", Name: "Executive Approval", AgentType: "approval", Department: "Executive"},
			{Key: "implementation", Name: "Implementation", AgentType: "implementation", Department: "Operations"},
			{Key: "report", Name: "Review & Report", AgentType: "report", Department: "Planning"},
		},
		Edges: []PlanEdge{
			{From: "intake", To: "planning", EdgeType: "sequence"},
			{From: "planning", To: "finance_review", EdgeType: "sequence"},
			{From: "planning", To: "legal_review", EdgeType: "sequence"},
			{From: "planning", To: "it_assessment", EdgeType: "sequence"},
			{From: "finance_review", To: "exec_approval", EdgeType: "sequence"},
			{From: "legal_review", To: "exec_approval", EdgeType: "sequence"},
			{From: "it_assessment", To: "exec_approval", EdgeType: "sequence"},
			{From: "exec_approval", To: "hr_planning", EdgeType: "sequence"},
			{From: "exec_approval", To: "ops_planning", EdgeType: "sequence"},
			{From: "hr_planning", To: "implementation", EdgeType: "sequence"},
			{From: "ops_planning", To: "implementation", EdgeType: "sequence"},
			{From: "implementation", To: "report", EdgeType: "sequence"},
		},
	}
}
