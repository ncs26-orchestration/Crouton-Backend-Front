# Agent Guide — Python Agent Service

## Architecture

The agent service (`apps/agent`) handles LLM-powered decision making for
department agents. The Go API is the orchestrator — it calls the Python
service when it needs an agent to think.

```
apps/agent/
  app/
    main.py                    → FastAPI entry point
    nodes/
      extract.py               → existing LLM extraction (legacy, adaptable)
    config.py                  → settings (pydantic-settings)
```

## Role in the System

The Go API owns:
- Workflow state (nodes, edges, status)
- State transitions (Pending → In Progress → Completed)
- Dependency resolution
- Audit event logging

The Python agent owns:
- LLM calls for agent reasoning
- Generating the workflow plan from a request (Intake Agent)
- Generating agent decisions and status updates
- Domain-specific logic per department

## Communication Pattern

The Go API calls the Python agent via HTTP:

```
Go API                          Python Agent
  │                                │
  │  POST /agents/intake           │
  │  {request_title, description}  │
  │ ─────────────────────────────→ │
  │                                │  LLM generates workflow plan
  │  {nodes: [...], edges: [...]}  │
  │ ←───────────────────────────── │
  │                                │
  │  POST /agents/execute          │
  │  {agent_type, node, context}   │
  │ ─────────────────────────────→ │
  │                                │  LLM generates agent decision
  │  {tasks_update, status_msg,    │
  │   dependencies, verdict}       │
  │ ←───────────────────────────── │
```

## Endpoints to Build

### POST /agents/intake
Called when a new request is submitted. The Intake Agent analyzes the
request and returns a workflow plan.

**Input:**
```json
{
  "title": "Open a new office in Berlin",
  "description": "We need to establish a new office...",
  "priority": "high"
}
```

**Output:**
```json
{
  "nodes": [
    {
      "name": "Request Intake",
      "agent_type": "intake",
      "agent_name": "Intake Processor",
      "description": "Initial request processing and workflow planning",
      "tasks": ["Validate request", "Determine workflow stages"],
      "sort_order": 1
    },
    {
      "name": "Planning & Analysis",
      "agent_type": "planning",
      "agent_name": "Planning Analyst",
      "description": "Break down request into analysis areas",
      "tasks": ["Scope assessment", "Resource identification", "Timeline planning"],
      "sort_order": 2
    },
    {
      "name": "Finance Review",
      "agent_type": "finance",
      "agent_name": "Finance Reviewer",
      "description": "Budget and financial impact analysis",
      "tasks": ["Budget Feasibility Check", "Financial Impact Analysis", "ROI Projection Review"],
      "sort_order": 3
    }
  ],
  "edges": [
    { "source": "Request Intake", "target": "Planning & Analysis", "type": "sequential" },
    { "source": "Planning & Analysis", "target": "Finance Review", "type": "sequential" },
    { "source": "Planning & Analysis", "target": "Legal Review", "type": "sequential" },
    { "source": "Planning & Analysis", "target": "IT Assessment", "type": "sequential" }
  ],
  "dependencies": [
    {
      "dependent": "Finance Review",
      "blocking": "IT Assessment",
      "reason": "Financial impact analysis requires IT infrastructure cost data"
    }
  ]
}
```

**Deterministic fallback:** For MVP, if LLM is slow, return a hardcoded
workflow plan for the "Open office" scenario. The plan should match the
screenshot exactly.

### POST /agents/execute
Called when a node transitions to "In Progress." The agent processes its
tasks and returns updates.

**Input:**
```json
{
  "agent_type": "finance",
  "node_name": "Finance Review",
  "request_context": {
    "title": "Open a new office in Berlin",
    "description": "...",
    "priority": "high"
  },
  "current_tasks": [
    { "id": "t1", "title": "Budget Feasibility Check", "status": "pending" },
    { "id": "t2", "title": "Financial Impact Analysis", "status": "pending" },
    { "id": "t3", "title": "ROI Projection Review", "status": "pending" }
  ],
  "dependencies": [
    { "blocking": "IT Assessment", "resolved": false }
  ]
}
```

**Output:**
```json
{
  "task_updates": [
    { "id": "t1", "status": "completed" },
    { "id": "t2", "status": "in_progress" }
  ],
  "latest_update": "Financial impact analysis is in progress. Waiting for data from IT assessment.",
  "verdict": null,
  "blocked_by": "IT Assessment"
}
```

### POST /agents/approve
Called for the Executive Approval stage.

**Input:** All upstream results and findings.
**Output:** Approve/reject with written justification.

## Agent Types and Their Logic

### Intake Processor
- Reads the request title and description
- Decides which departments need to review
- Generates the full workflow plan (nodes + edges)
- For MVP: deterministic plan matching the screenshot

### Finance Reviewer
- Tasks: Budget Feasibility Check, Financial Impact Analysis, ROI Projection
- Depends on IT Assessment for infrastructure cost data
- Produces: budget analysis findings, approval/flag recommendation

### Legal Reviewer
- Tasks: Regulatory Compliance Check, Contract Requirements, Risk Assessment
- Independent (no cross-dependencies for MVP)
- Produces: compliance findings, approval/flag recommendation

### IT Manager
- Tasks: Technical Infrastructure Assessment, Security Review, Systems Integration
- Independent (others depend on IT, not the other way)
- Produces: technical feasibility findings, cost estimates

### HR Manager
- Tasks: Staffing Requirements, Hiring Plan, Policy Compliance
- Runs after reviews complete
- Produces: staffing plan, timeline

### Operations Manager
- Tasks: Logistics Planning, Facilities Assessment, Operational Timeline
- Runs after reviews complete
- Produces: operational plan, cost breakdown

### Executive Approver
- Tasks: Review All Findings, Risk-Benefit Analysis, Final Decision
- Only runs when all upstream nodes complete
- Produces: written approval or rejection with justification

## LLM Provider Configuration

The agent service supports multiple LLM providers:

```python
# Environment variables
AGENT_EXTRACTOR_PROVIDER=ollama   # ollama | gemini | anthropic
AGENT_EXTRACTOR_MODEL=qwen2.5:7b  # model name
```

For MVP, agents can use a hybrid approach:
- **Deterministic logic** for task progression and state
- **LLM** for generating natural-language status updates and decisions

This avoids waiting 60s per agent for a local model.

## Existing Code to Adapt

The existing `apps/agent/app/nodes/extract.py` contains:
- LLM provider dispatch (Ollama, Gemini, Anthropic)
- JSON-mode output parsing
- Prompt construction patterns

This can be adapted for department agent prompts. The key change:
instead of extracting a Workflow IR from conversation, agents now
produce task updates and status messages from a request context.
