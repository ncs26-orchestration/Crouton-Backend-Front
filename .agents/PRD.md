# PRD — AI Organization OS

Requests, department agents, a live workflow canvas, agent-declared dependencies, human executive
approval, and an append-only audit trail.

> This is the umbrella PRD. Per-team, step-by-step execution detail lives in `PRD-BACKEND.md`,
> `PRD-AGENT.md`, and `PRD-FRONTEND.md`. The status tracker below is the single source of truth for
> what is built vs not.

## Status tracker

Legend: ✅ done · 🟡 in progress · ⬜ not started

### Platform & groundwork

| Item | Status | Notes |
|---|---|---|
| Auth, organizations, teams, members | ✅ | Pre-existing foundation; reused as-is |
| Docker Compose smooth boot (auto-migrate, healthchecks, ordering) | ✅ | `compose.yaml` + `compose.override.yaml` |
| CI pipeline (lint · typecheck · test · race · migrations up/rollback · e2e smoke) | ✅ | `.github/workflows/ci.yml`, `e2e.yml` |
| Rebrand `aup` → `aios` | ✅ | Codename/infra/schema/storage keys |
| Vendored engineering + design skills | ✅ | `.agents/skills/{frontend,backend,agent}` |
| Slice PRDs authored (backend / agent / frontend) | ✅ | `.agents/PRD-*.md` |

### Backend slice (Go API + orchestration) — see `PRD-BACKEND.md`

| ID | Feature | Status |
|---|---|---|
| BE-1 | Schema & migrations (requests, workflow_nodes/edges, agent_tasks, node_dependencies, audit_events, agents, department_policies) | ⬜ |
| BE-2 | Repositories (pgx) | ⬜ |
| BE-3 | Agent service client (typed Intake/Run) | ⬜ |
| BE-4 | Request intake + plan persistence | ⬜ |
| BE-5 | Orchestration worker (state machine) | ⬜ |
| BE-6 | Agent-declared cross-dependencies + re-run on unblock | ⬜ |
| BE-7 | Executive approval gate (human) | ⬜ |
| BE-8 | SSE streaming | ⬜ |
| BE-9 | Read endpoints for the tabs | ⬜ |
| BE-10 | Seeding (departments, agents, policies, demo org) | ⬜ |

### Agent slice (Python, Pydantic AI) — see `PRD-AGENT.md`

| ID | Feature | Status |
|---|---|---|
| AG-1 | Dependency + package scaffold (pydantic-ai) | ⬜ |
| AG-2 | Output models (Plan, Decision) | ⬜ |
| AG-3 | Injected context (deps) | ⬜ |
| AG-4 | Tools incl. `raise_dependency` | ⬜ |
| AG-5 | Model selection + FunctionModel offline fallback | ⬜ |
| AG-6 | Intake planner agent | ⬜ |
| AG-7 | Department agent factory | ⬜ |
| AG-8 | FastAPI surface (/agents/intake, /agents/run) | ⬜ |
| AG-9 | Tests (TestModel/FunctionModel) | ⬜ |

### Frontend slice (React) — see `PRD-FRONTEND.md`

| ID | Feature | Status |
|---|---|---|
| FE-1 | Shell nav + routing refactor | ⬜ |
| FE-2 | API client, types, SSE client | ⬜ |
| FE-3 | Requests tab + New Request | ⬜ |
| FE-4 | Workflows canvas: 3-panel shell | ⬜ |
| FE-5 | Department node + auto-layout | ⬜ |
| FE-6 | Live SSE wiring | ⬜ |
| FE-7 | My Work + approval | ⬜ |
| FE-8 | Home dashboard | ⬜ |
| FE-9 | Agents roster | ⬜ |
| FE-10 | Reports | ⬜ |
| FE-11 | Policies + Integrations | ⬜ |
| FE-12 | Design pass | ⬜ |

**Update this table as features land.** When you finish a feature, flip its row to ✅ (or 🟡 while in
progress) in the same PR that implements it.

---

## Problem Statement

People inside an organization constantly kick off cross-department requests — "open a new office in Berlin", "onboard a new enterprise vendor", "approve this large purchase". Today that coordination lives in email threads, chat channels, and spreadsheets. Nobody can see, at a glance, where a request is, which departments are still working on it, what each one decided and why, or what is blocking what. When Finance is waiting on IT's security assessment, that dependency is invisible until someone chases it. When an executive approves, the reasoning evaporates into an inbox. There is no single, trustworthy record of how a decision was actually made.

The current product in this repo (a dormant workflow-authoring tool) does not solve this — it lets you draft BPMN, not run a real multi-department request.

## Solution

An **AI Organization OS**: the user submits a **request**, an **Intake agent** plans a **workflow** of department stages, and specialized **department agents** (Finance, Legal, IT, HR, Operations) execute in parallel. Agents do real work — they validate, flag risks, and produce plain-language status updates. Crucially, an agent that needs another department's output **declares a cross-dependency itself** ("Finance is waiting for IT's security assessment"), and the system blocks that stage and auto-unblocks it when the blocking department finishes. When all reviews converge, the request lands in a human **approver's** queue for **executive approval** with a written justification, then execution stages run and a final report is produced.

The whole thing is **live, clickable, and traceable**: a workflow canvas updates in real time as agents progress; clicking any stage shows its agent, tasks, latest status, and activity; and every state change is written to an append-only **audit trail** with who, what, why, and when. Three words: Collaborative → Traceable → Live.

## User Stories

1. As a requester, I want to submit a request with a title, description, and priority, so that the organization can start processing it.
2. As a requester, I want the system to automatically plan the right department workflow for my request, so that I don't have to know who needs to be involved.
3. As a requester, I want to see overall progress and an estimated completion for my request, so that I can set expectations.
4. As a requester, I want to see which departments are participating and their live status, so that I know who is currently working on my request.
5. As a requester, I want to open a request and see its full workflow as a graph, so that I understand the path it will take.
6. As a viewer, I want each workflow stage to be color-coded by status (pending, in progress, completed, blocked), so that I can read the state of the request at a glance.
7. As a viewer, I want to click a workflow stage and see the owning department agent, its task list, progress, and latest status, so that I can understand what that stage is doing.
8. As a viewer, I want to click a stage and see its activity log, so that I can trace what happened and why.
9. As a department agent (Finance), I want to analyze the request for budget feasibility, financial impact, and ROI, so that I produce a meaningful financial decision rather than a rubber stamp.
10. As a department agent (Legal), I want to check regulatory compliance and contract requirements and flag risks, so that legal exposure is surfaced early.
11. As a department agent (IT), I want to assess technical feasibility, security requirements, and systems integration, so that technical constraints are known before approval.
12. As a department agent (HR), I want to plan staffing and hiring needs, so that the people side of the request is accounted for.
13. As a department agent (Operations), I want to plan logistics, facilities, and operational timeline, so that the request can actually be executed.
14. As a department agent, I want to consult my department's policy and the organization's information systems while reasoning, so that my decision is grounded in real constraints, not invented ones.
15. As a department agent, I want to declare a dependency on another department when I need its output first, so that the system knows I am genuinely blocked rather than slow.
16. As a requester, I want a blocked stage to clearly show "waiting for [department]" with the agent's own reason, so that the bottleneck is obvious.
17. As the system, I want a blocked stage to automatically resume when the department it was waiting on completes, so that no human has to manually unblock it.
18. As a viewer, I want to watch a stage complete and a dependent stage unblock live on the canvas, so that I can see the collaboration happen in real time.
19. As an approver, I want completed-but-unapproved requests to appear in my personal work queue, so that I know an approval is waiting on me.
20. As an approver, I want to read every department agent's decision and flags before I decide, so that my approval is informed.
21. As an approver, I want to approve or reject with a written justification, so that the reasoning behind the decision is captured permanently.
22. As the system, I want the workflow to pause at the executive approval stage until a human decides, so that the gate is a real decision, not an automated pass-through.
23. As the system, I want approval to resume execution stages (HR, Operations, Implementation) and rejection to stop the request, so that the human decision actually drives the outcome.
24. As the system, I want to generate a final report when the request completes, so that there is a durable summary of approvals, flags, execution, and time taken.
25. As an auditor, I want every status change, agent action, dependency, handoff, and approval logged with actor, action, reason, and timestamp, so that any state can be explained after the fact.
26. As an auditor, I want to browse the audit trail filtered by request, agent, date, or action, so that I can reconstruct how a decision was made.
27. As a manager, I want a home dashboard with active/recent requests, quick stats, agent activity, and a recent audit feed, so that I have an at-a-glance view of the organization.
28. As a manager, I want to see the full list of requests with requester, priority, status, and progress, and filter it, so that I can find and triage requests.
29. As a manager, I want an agent roster grouped by department showing each agent's live status and capabilities, so that I can see the workforce of agents.
30. As an integrator, I want a new organization to be seeded with standard departments, one agent per department, and starter policies, so that it is usable immediately.
31. As a presenter, I want a demo organization pre-populated with a completed sample request and its audit history, so that the dashboard and reports are not empty on first load.
32. As a frontend developer, I want the canvas to stay in sync via a live event stream with a polling fallback, so that updates are timely and resilient to dropped connections.
33. As a developer, I want department agents to produce typed, validated output, so that the orchestration never has to parse loose JSON.
34. As an operator, I want the system to still complete a request end-to-end when no LLM API key is configured, so that demos and local runs are reliable.
35. As a viewer, I want to read the policies that agents actually consult, so that I understand the rules behind their decisions.
36. As a requester, I want submitting a request to take me straight to its live workflow canvas, so that I immediately see it being processed.

## Implementation Decisions

**Product framing.** Full pivot to the AI Organization OS. The existing workflow-authoring code stays in the repo but is off this feature's path (rebrand already complete: codename `aios`). Reuse the existing auth, organizations, teams, and members foundation, the React Flow canvas plumbing, and the agent service's provider configuration.

**Orchestrator placement.** Go owns the durable system of record and deterministic orchestration; the Python service does reasoning only. The web app talks only to Go.

**Data model (Go / Postgres).** New entities: `requests`, `workflow_nodes`, `workflow_edges`, `agent_tasks`, `node_dependencies`, `audit_events`, `agents`, `department_policies`. A request belongs to an organization; departments reuse the existing `teams` table; one agent is seeded per department and linked to its team. Node status: `pending | in_progress | completed | blocked`. Request status: `submitted | in_progress | awaiting_approval | approved | rejected | completed`. `audit_events` is append-only.

**Deep module: Orchestration engine (Go).** A request runs on a background worker. Public interface stays small: start a request, and submit an approval decision. Internally it computes node eligibility (all predecessor edges originate from completed nodes AND the node has no unresolved dependencies), advances statuses, writes audit events, and publishes events to the bus. The executive-approval node enters `in_progress`, sets the request to `awaiting_approval`, and parks until a human decision arrives. Per-stage pacing is configurable so progression is watchable. If the agent service errors, a deterministic fallback decision is used so a run never stalls.

**Agent-declared cross-dependencies (the differentiator).** Dependencies are NOT hardcoded by the planner. When the engine runs a department agent and the returned decision carries a `blocked_on` declaration (produced by the agent's `raise_dependency` tool), the engine marks the node `blocked`, records a `node_dependencies` row with the agent's own reason, and writes audit + event. When the blocking node completes, the engine resolves the dependency and re-invokes the blocked agent with the blocker's output added to its upstream context (capped re-runs to prevent loops); the second pass typically completes.

**Deep module: Agent service client (Go).** Typed `Intake(request, org_context) -> Plan` and `Run(agent_type, request, upstream_context, org_context) -> Decision` over the Python service. Go injects an org-context snapshot (information systems + policies) and upstream summaries so agent tools read injected data rather than calling back into Go. A typed "agent unavailable" error triggers the fallback path.

**Deep module: SSE event bus (Go).** In-process publish/subscribe with channel fan-out keyed by request id. The engine publishes; the SSE endpoint subscribes and streams to the browser, unsubscribing on client disconnect.

**UI-facing API (Go).** Create/list requests under an org; get a request's full graph (request + nodes + edges + agents); get node detail (node + tasks + activity); approve a request (decision + justification); an SSE stream per request authenticated via a token query parameter because EventSource cannot set headers; list agents, audit (org-wide and per-request), and policies.

**SSE event contract.** Event types `node_status`, `request_status`, `task`, and `audit`, each a JSON object carrying the request id, the changed entity, and a timestamp. The frontend patches its query cache from these.

**Agent layer (Python, Pydantic AI).** Retire the raw HTTP + manual JSON parsing for this flow. Each department agent and the intake planner are Pydantic AI agents with typed output models (`Plan`, `Decision`), tool-calling, dependency injection for org/upstream context, and automatic validation + retries. Tools include reading the information-system registry, fetching the department policy, domain calculations (e.g. budget assessment for Finance, compliance lookup for Legal), and `raise_dependency(on_department, reason)` which surfaces on `Decision.blocked_on`. The intake planner chooses stages from a fixed department catalog and falls back to a default template if validation fails.

**Offline fallback.** When no provider key is set, model selection returns a Pydantic AI `FunctionModel` that emits deterministic `Plan`/`Decision` output (including a Finance `blocked_on` case), so the full flow runs with no network.

**Deep module: request-to-flow mapper (TS).** A pure function transforming the request graph (nodes + edges) into React Flow nodes and edges with an auto-layout derived from stage order and edges (parallel branches side by side, merge points). No side effects.

**Frontend.** Expand the shell navigation to Home, My Work, Requests, Workflows, Agents, Reports, Policies, Integrations, Teams. The Workflows view is a three-panel layout: left request overview (progress, ETA, participating agents with live status), center canvas, right node detail (Overview / Tasks / Activity). A `useRequestStream` hook subscribes to the SSE endpoint and patches the cache, with an interval-poll fallback. Policies is a read-only browser of the seeded policies agents consult; Integrations is display-only.

**Seeding.** On organization creation, seed the standard department teams, one agent per team, and starter department policies, and mark the creator as approver. A demo-seed path additionally creates one organization with an approver login and a completed sample request plus audit history.

## Testing Decisions

A good test asserts **external behavior through a module's public interface**, not its internals — given inputs and observable outputs/effects, never private state or call order. Tests must be deterministic: no live LLM calls, no real time-of-day dependence, no flaky network.

Modules to test (all four agreed):

1. **Orchestration engine (Go)** — the highest-value target. Drive it through its public interface with an in-memory repo and a fake agent client, and assert observable outcomes: a no-dependency request advances every node to completed; a request where the Finance agent returns `blocked_on: IT` marks Finance `blocked` with the agent's reason, then completes Finance after IT completes (dependency resolved); the executive-approval node parks the request at `awaiting_approval` until an approval decision arrives, and approve vs reject produce the right terminal status; every transition writes a corresponding audit event. Re-run capping is asserted so a pathological agent cannot loop forever.
2. **Department agents + raise_dependency (Python)** — run each agent against Pydantic AI `TestModel`/`FunctionModel` and assert the typed `Decision` shape, that the Finance agent given a context lacking IT output returns a `blocked_on` for IT (and that the `raise_dependency` tool fired), and that re-running with IT's summary returns a completed decision. The intake planner returns a valid `Plan` over the fixed catalog, and an invalid model output still yields the default template.
3. **Agent client + SSE bus (Go)** — the client parses `Plan`/`Decision` from a stub HTTP server and maps a timeout to the typed unavailable error; the bus delivers published events to all current subscribers for a request id and stops delivering after unsubscribe.
4. **request-to-flow mapper (TS)** — a pure unit test with vitest: a known request graph maps to the expected React Flow node/edge set with correct statuses and a layout that places parallel branches at the same rank and connects merge points.

Prior art: Go table-driven tests already exist for the compiler and handler packages and should be the pattern for the engine and client. Python testing uses pytest (a smoke test already exists); department-agent tests follow the Pydantic AI test-model approach. The web app has vitest wired (currently no tests); the mapper is the first real unit test.

## Out of Scope

- Implementing real external integrations (SAP, LexisNexis, Workday, Slack, etc.). The Integrations tab is display-only, matching the product's stated non-goal.
- A Policies authoring UI. Policies are seeded and read-only for this feature; agents consult them.
- Multi-workflow history depth, request pagination at scale, and notification backends beyond what the canvas and audit feed show.
- OAuth/SSO and fine-grained RBAC beyond the existing JWT + org roles, and approver-role gating.
- Mobile-responsive layouts; the canvas is desktop-first.
- Removing or rewriting the dormant authoring-tool code; it stays in the repo, off this path.
- Multi-replica orchestration (the event bus is in-process for now; a shared broker is a later concern).

## Further Notes

- Execution order is the slice order in the three detailed PRDs in `.agents/` (`PRD-BACKEND.md`, `PRD-AGENT.md`, `PRD-FRONTEND.md`), which break this feature into numbered, step-by-step tasks per team and define the shared contracts. This file is the umbrella; those are the execution detail.
- The demo golden path is the acceptance bar: Home → Requests → submit "Open a new office in Berlin" (High) → live Workflows canvas → Finance shows "waiting for IT" → IT completes, Finance unblocks → My Work approval with justification → execution → Reports with the completed report and full audit trail.
- CI already enforces lint/typecheck/test/race across all three apps, a migration up/rollback/re-apply check, and a docker-compose e2e smoke; new modules should extend these (engine + agent unit tests, the TS mapper test, and an extended e2e that drives a request to the approval gate).
- Reliability bar: with all LLM keys unset, the entire flow must still complete via the `FunctionModel` / deterministic-fallback paths.
- A copy of this PRD was also published as GitHub issue #4; this file is the canonical, version-controlled source and carries the live status tracker.
