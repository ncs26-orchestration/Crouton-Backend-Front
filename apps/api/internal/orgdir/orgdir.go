// Package orgdir holds the canonical department directory, the standard agents
// and starter policies seeded into every org. It is the single source of truth
// shared by the org-create handler (so a new org is usable immediately) and the
// demo seed command, so the two can never drift.
package orgdir

import (
	"github.com/ncs26-orchestration/solution/apps/api/internal/graphspec"
	"github.com/ncs26-orchestration/solution/apps/api/internal/policyrules"
)

// AgentSpec is one standard department agent. Department is the team name it
// belongs to; it is used to resolve the agent's team_id at seed time.
type AgentSpec struct {
	Department   string
	AgentType    string
	Name         string
	Capabilities string
}

// PolicySpec is one starter department policy, keyed by department/team name.
// Rules are typed, machine-checkable conditions evaluated against a request's
// structured details to produce exact per-node checks.
type PolicySpec struct {
	Department string
	Title      string
	Body       string
	Rules      []policyrules.Rule
}

// Agents is the standard roster: one agent per agent_type the pipeline runs.
// Order is the pipeline order, which is also how the roster reads top to bottom.
var Agents = []AgentSpec{
	{"Finance", "finance", "Finance Agent", "Budget analysis, spend approval, financial risk assessment, ROI calculation"},
	{"Legal", "legal", "Legal Agent", "Contract review, regulatory compliance, risk flagging, policy advisory"},
	{"IT", "it", "IT Agent", "Technical feasibility, security assessment, infrastructure planning, systems integration"},
	{"HR", "hr", "HR Agent", "Staffing assessment, hiring plan, onboarding logistics, people ops"},
	{"Operations", "ops", "Operations Agent", "Logistics planning, facilities, timeline management, execution coordination"},
	{"Planning", "planning", "Planning Agent", "Workflow planning, dependency mapping, timeline estimation"},
	{"Executive", "approval", "Executive Approver", "Strategic decision-making, cross-functional review, approval authority"},
}

// Policies is the starter policy set agents consult. One per department. Some
// carry typed rules that are checked against a request's structured details.
var Policies = []PolicySpec{
	{"Finance", "Finance Policy", "Expenditures over $10k require executive approval. A single purchase order may not exceed $50k without CFO sign-off. Budget allocations must align with quarterly planning.", []policyrules.Rule{
		{Label: "Within single-PO limit", Field: "total_cost", Op: "lte", Value: 50000, Severity: "warning", Message: "Total exceeds the $50k single-PO limit — CFO sign-off required."},
	}},
	{"Legal", "Legal Policy", "All contracts must be reviewed for regulatory compliance. Data privacy laws (GDPR, CCPA) apply to any cross-border data handling.", nil},
	{"IT", "IT Policy", "New systems must pass a security assessment. Infrastructure changes require change management approval. Monthly infra cost over $3k needs review.", []policyrules.Rule{
		{Label: "Infra cost within review threshold", Field: "est_cost", Op: "lte", Value: 3000, Severity: "warning", Message: "Estimated monthly cost exceeds $3k — needs infrastructure review."},
	}},
	{"HR", "HR Policy", "New headcount requires approved job descriptions and budget. A single request may not add more than 10 roles without VP approval.", []policyrules.Rule{
		{Label: "Headcount within limit", Field: "headcount", Op: "lte", Value: 10, Severity: "warning", Message: "More than 10 roles in one request needs VP of People approval."},
	}},
	{"Operations", "Operations Policy", "Project timelines must account for dependencies and buffer time. Vendor onboarding follows the standard integration checklist.", nil},
}

// WorkflowSpec is one starter internal workflow seeded into every org so the
// Workflows area is useful immediately. All starters are global (org-wide).
type WorkflowSpec struct {
	Name        string
	Description string
	Category    string
	Nodes       []graphspec.Node
	Edges       []graphspec.Edge
}

// Workflows are the reusable processes seeded on org creation. The step keys map
// to the standard agents above so they run out of the box.
var Workflows = []WorkflowSpec{
	{
		Name:        "Hiring",
		Description: "Open a new role: HR plans it, Finance confirms budget, Legal checks compliance, the executive signs off.",
		Category:    "hiring",
		Nodes: []graphspec.Node{
			{Key: "hr_planning", Name: "HR Planning", AgentType: "hr", Department: "HR"},
			{Key: "finance_review", Name: "Finance Review", AgentType: "finance", Department: "Finance"},
			{Key: "legal_review", Name: "Legal Review", AgentType: "legal", Department: "Legal"},
			{Key: "exec_approval", Name: "Executive Approval", AgentType: "approval", Department: "Executive"},
		},
		Edges: []graphspec.Edge{
			{From: "hr_planning", To: "finance_review", Type: "sequence"},
			{From: "finance_review", To: "legal_review", Type: "sequence"},
			{From: "legal_review", To: "exec_approval", Type: "sequence"},
		},
	},
	{
		Name:        "Time off",
		Description: "Request leave: Operations checks coverage, HR validates the balance and policy, the executive approves.",
		Category:    "timeoff",
		Nodes: []graphspec.Node{
			{Key: "ops_planning", Name: "Operations Review", AgentType: "ops", Department: "Operations"},
			{Key: "hr_planning", Name: "HR Review", AgentType: "hr", Department: "HR"},
			{Key: "exec_approval", Name: "Executive Approval", AgentType: "approval", Department: "Executive"},
		},
		Edges: []graphspec.Edge{
			{From: "ops_planning", To: "hr_planning", Type: "sequence"},
			{From: "hr_planning", To: "exec_approval", Type: "sequence"},
		},
	},
}
