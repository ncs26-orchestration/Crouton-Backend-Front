# AI Organization OS — Development Plan

## 1. Current State

The foundation is complete. Auth, organizations, teams, and members are
implemented end to end (API + UI). The project is pivoting from "Pablo"
(a workflow authoring tool for Camunda/Elsa) to an AI Organization OS
(a multi-agent workflow system for processing business requests).

### What exists and is reusable
- JWT auth system (login, register, middleware)
- Organization CRUD with team/member management
- React shell with navigation rail
- PostgreSQL + Redis infrastructure
- Docker Compose dev environment with hot reload
- Python agent service (FastAPI + LangGraph — needs new agent logic)
- TanStack Query data fetching patterns

### What needs to be built
All 10 features from FEATURES.md (F1–F10). None have been started.

## 2. Build Order

The build order follows dependency chains. Features that feed other
features are built first. UI views (F8, F9) can start with mock data
in parallel.

```
F1 (Request Intake) ──→ F2 (Graph Engine) ──→ F6 (Audit Trail)
                                │
                                ▼
                         F3 (Dept Agents) ──→ F4 (Dependencies)
                                │
                                ▼
                         F5 (Exec Approval)
                                │
                                ▼
                         F7 (Execution Stage)
                                │
                                ▼
                         F10 (Final Report)

F8 (Canvas UI) and F9 (Agent Roster): start immediately with
mock/sample data, wire to real backend as features land.
```

## 3. Implementation Strategy Per Feature

### F1 — Request Intake & Workflow Plan
**Backend:** New `requests` table. POST endpoint creates a request and
calls the Intake Agent. The agent returns a workflow plan (list of nodes
with types, agent assignments, and edge definitions).

**Agent:** The Intake Agent uses LLM to analyze the request and decide
which stages are needed. Output: structured JSON with nodes and edges.
For MVP, a deterministic fallback plan is acceptable if LLM is slow.

**Frontend:** Submit Request form. Request Overview panel reads from
the API.

### F2 — Workflow Graph Engine
**Backend:** `workflow_nodes` and `workflow_edges` tables. State machine
for node status (Pending → In Progress → Completed / Blocked). Graph
traversal logic: when a node completes, check which downstream nodes
have all dependencies met and transition them.

**Key logic:** Parallel branches (multiple nodes sharing a parent start
simultaneously). Merge points (a node after parallel branches waits for
all incoming edges). This is the core engine — get it right.

### F3 — Department Agents
**Backend:** `agent_tasks` table. Each node's agent has a fixed task
list. Agent logic progresses through tasks, updating status and writing
plain-language updates.

**Agent:** Each department agent (Finance, Legal, IT) has its own prompt
and decision logic. For MVP, agents can use simple rule-based logic with
LLM-generated status messages. Each agent must produce:
- Per-task status updates with timestamps
- A final verdict (approve/flag/block)
- Plain-language latest update text

### F4 — Cross-Agent Dependencies
**Backend:** `agent_dependencies` table. An agent declares "I need X
from agent Y." The dependent node goes Blocked. When the blocking node
completes, the dependency resolves and the blocked node transitions back
to In Progress.

**Required proof:** Finance Review depends on IT Assessment. The Finance
agent's latest update says "Waiting for data from IT assessment." When
IT completes, Finance automatically unblocks and continues.

### F5 — Executive Approval
**Backend:** The Executive Approval node only transitions from Pending to
In Progress when ALL upstream nodes are Completed. The Executive Approver
agent reviews findings and produces a written decision (approve + reason
or reject + reason).

### F6 — Audit Trail
**Backend:** `audit_events` table (append-only). Every state change from
F1–F5 writes an event: timestamp, actor, action, target node, reason.

**Frontend:** Activity tab in the node detail panel. Audit Trail tab in
the top navigation. Both read from the same events API.

**Start early:** Begin logging events as soon as F1 and F2 are built.

### F7 — Execution Stage
**Backend:** HR Manager and Operations Manager agents, same pattern as F3.
Implementation node with its own tasks. Both HR and Ops must complete
before Implementation starts.

### F8 — Workflow Canvas UI
**Frontend:** React Flow canvas. Nodes are boxes colored by status.
Edges are arrows (solid = sequential, dashed = parallel merge). Click
a node to open the detail panel. Start with hardcoded sample data to
get the layout right, then wire to real API.

**Must match:** The screenshot in mvp.png and the design system in
DESIGN.md (Stripe-inspired: navy headings, purple accents, blue-tinted
shadows, sohne-var typography, 4px border-radius).

### F9 — Agent Roster Panel
**Frontend:** Read-only list of agents with status badges. Pulls from
the workflow nodes API. Group by team.

### F10 — Final Report
**Backend:** When Implementation completes, auto-generate a markdown
summary from audit events. Attach to the Review & Report node.

## 4. Development Method

### For each feature:
1. **Write the migration** — tables, indexes, constraints.
2. **Write the handler** — Go API endpoints.
3. **Write the agent logic** — Python agent if needed.
4. **Write the UI** — React components, API calls.
5. **Test end to end** — submit a request, verify state transitions.
6. **Log audit events** — every state change from day one.

### Verification
```bash
# API health
curl http://localhost:8080/readyz

# Web builds
pnpm --filter web build

# Go tests
cd apps/api && go test ./...

# End-to-end: submit request, check workflow state
curl -X POST http://localhost:8080/api/requests \
  -H 'Authorization: Bearer <token>' \
  -H 'Content-Type: application/json' \
  -d '{"title":"Open a new office in Berlin","priority":"high"}'
```

### Design compliance
Every UI component must follow DESIGN.md:
- Font: sohne-var with `font-feature-settings: "ss01"`
- Headings: `#061b31` (deep navy), weight 300
- Body: `#64748d` (slate)
- Primary CTA: `#533afd` (Stripe purple)
- Shadows: `rgba(50,50,93,0.25)` blue-tinted
- Border-radius: 4px–8px (no pills)
- Spacing: 8px base unit

## 5. Parallelization Opportunities

These can be worked on simultaneously:

| Track A (Backend) | Track B (Frontend) | Track C (Agents) |
|---|---|---|
| F1: Request table + API | F8: Canvas with mock data | Intake Agent prompt |
| F2: Graph engine logic | F9: Agent roster with mock data | Department agent prompts |
| F6: Audit events table | F13: Requests list page | Agent task logic |
| F3: Agent tasks table | F14: Agents page | Cross-agent dependency logic |
| F4: Dependencies table | F11: Home dashboard | — |
| — | F12: My Work inbox | — |
| — | F15: Reports page | — |
| — | F16: Integrations page (display-only) | — |

## 6. Risk Mitigation

| Risk | Mitigation |
|---|---|
| LLM latency for Intake Agent | Deterministic fallback: hardcoded workflow plan for "Open office" request |
| Complex graph traversal bugs | Start with a fixed, known-good graph shape; add dynamic later |
| UI doesn't match screenshot | Build F8 canvas first with mock data; iterate visually before wiring |
| Cross-agent timing issues | Use polling or simple sequential execution for MVP; real-time later |
| Too many features for hackathon | F8 canvas with mock data + F1/F2/F3 backend is the minimum viable demo |
| Sidebar tabs feel empty | Build all tab views with mock data early; wire to real APIs later |

## 7. Minimum Viable Demo

If time is short, the absolute minimum to demo is:

1. **F8 with mock data** — the canvas renders the full workflow graph
   from the screenshot, with clickable nodes showing detail panels.
2. **F1 + F2** — one real request creates a real workflow with state.
3. **F3** — at least Finance and IT agents run with real tasks.
4. **F4** — Finance depends on IT, visible in the UI.
5. **F6** — audit trail shows real events.
6. **All sidebar tabs render** — even with mock/sample data, every tab
   should show meaningful content, not empty states.

Everything else (F5, F7, F9, F10, F11–F16) can use mock data if the
backend isn't ready in time. But every tab must have content.
