// Package orgdir holds the canonical department directory, the standard agents
// and starter policies seeded into every org. It is the single source of truth
// shared by the org-create handler (so a new org is usable immediately) and the
// demo seed command, so the two can never drift.
package orgdir

// AgentSpec is one standard department agent. Department is the team name it
// belongs to; it is used to resolve the agent's team_id at seed time.
type AgentSpec struct {
	Department   string
	AgentType    string
	Name         string
	Capabilities string
}

// PolicySpec is one starter department policy, keyed by department/team name.
type PolicySpec struct {
	Department string
	Title      string
	Body       string
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

// Policies is the starter policy set agents consult. One per department.
var Policies = []PolicySpec{
	{"Finance", "Finance Policy", "All expenditures over $10k require executive approval. Budget allocations must align with quarterly planning. Vendor contracts must include payment terms and cancellation clauses."},
	{"Legal", "Legal Policy", "All contracts must be reviewed for regulatory compliance. Non-disclosure agreements follow the standard template. Data privacy laws (GDPR, CCPA) apply to any cross-border data handling."},
	{"IT", "IT Policy", "New systems must pass a security assessment. Software procurement follows the approved vendor list. Infrastructure changes require change management approval."},
	{"HR", "HR Policy", "New headcount requires approved job descriptions and budget allocation. Onboarding includes equipment provisioning, system access, and compliance training."},
	{"Operations", "Operations Policy", "Project timelines must account for dependencies and buffer time. Vendor onboarding follows the standard integration checklist."},
}
