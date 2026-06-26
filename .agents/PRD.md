# PRD — AI Organization OS

Requests, department agents, a live workflow canvas, agent-declared dependencies, human executive
approval, and an append-only audit trail.

> **We track work by FEATURE, not by layer.** Every feature is a vertical slice that carries its own
> DB, backend, agent, frontend, and linking. A feature is **done only when the whole slice works
> end-to-end** — not when "the backend part" is finished. The per-layer step detail and shared
> contracts live in `PRD-BACKEND.md`, `PRD-AGENT.md`, `PRD-FRONTEND.md` and are referenced by task
> id (BE-/AG-/FE-) from each feature below; those files are reference, this file is the plan.

## Feature tracker

Legend: ✅ done · 🟡 in progress · ⬜ not started. Layers: **DB** schema/migrations · **BE** Go API/engine ·
**AG** Python agent · **FE** React · **Link** the wiring that makes the layers talk.

| # | Feature (vertical slice) | DB | BE | AG | FE | Link | Overall |
|---|---|----|----|----|----|----|----|
| F0 | App shell & navigation | – | – | – | ✅ | – | ✅ |
| F1 | Submit & track a request | ✅ | ✅ | – | ✅ | ✅ | ✅ |
| F2 | Auto-planned workflow graph | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| F3 | Agents do the work (status progression) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| F4 | Live canvas (real-time SSE) | – | ✅ | – | ✅ | ✅ | 🟡 |
| F5 | Cross-department dependencies | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| F6 | Traceability / audit trail | ✅ | ✅ | – | ✅ | ✅ | 🟡 |
| F7 | Human executive approval | – | ✅ | – | ✅ | ✅ | 🟡 |
| F8 | Execution stage & final report | – | ✅ | 🟡 | ✅ | ✅ | 🟡 |
| F9 | Roster, Policies & Integrations views | ⬜ | ⬜ | – | ⬜ | ⬜ | ⬜ |
| F10 | Seeding & demo data | ✅ | ✅ | – | – | – | ✅ |

Groundwork already done (not features, but the platform features sit on): auth/orgs/teams/members
foundation ✅ · Docker Compose smooth boot ✅ · CI (lint/test/race/migrations/e2e) ✅ · rebrand
`aup`→`aios` ✅ · vendored skills ✅ · these PRDs ✅.

**Rule:** a layer cell goes ✅ only when its part is merged AND wired to the others; the **Overall**
column goes ✅ only when every non-`–` cell is ✅ and the feature's done-check passes. Update the row
in the same PR that lands the work.

---

## Features (vertical slices)

Each feature lists what each layer must do, the wiring between them, the slice-PRD task ids that hold
the step detail, and a single end-to-end **done-check**. Ordered by dependency; F0→F3 are the spine,
the rest layer on.

### F0 — App shell & navigation
*Enabler. No data; makes every later UI reachable.*
- **FE:** `ShellSection` = `home | my-work | requests | workflows | agents | reports | policies | integrations | teams`; refactor `App.tsx` location state to `{section, requestId, nodeId}`; route each section to a real or stub view; keep existing working views. (FE-1)
- **Link:** none.
- **Done-check:** every nav item renders without a type error; `pnpm --filter web build` passes; deep state survives reload. *(This is the half-done edit that broke `make up`; finishing it is F0.)*

### F1 — Submit & track a request
*The spine: a request exists and is visible.*
- **DB:** `requests`. (BE-1)
- **BE:** create/list/get request endpoints. (BE-2, BE-4 minus planning)
- **FE:** Requests tab + New Request modal + request detail shell; Home lists requests. (FE-3, part of FE-8)
- **Link:** web → api (`lib/api.ts` + types). (FE-2)
- **Done-check:** submit "Open a new office in Berlin" (High) → it appears in the Requests list and opens its (still empty) detail.

### F2 — Auto-planned workflow graph
*Intake turns a request into a department workflow you can see.*
- **DB:** `workflow_nodes`, `workflow_edges`. (BE-1)
- **AG:** pydantic-ai scaffold + `Plan` model + intake agent `/agents/intake`. (AG-1, AG-2, AG-6, AG-8)
- **BE:** agent client; create handler calls intake, persists graph; `GET /requests/:id` returns the graph. (BE-3, BE-4)
- **FE:** Workflows 3-panel shell + `DepartmentNode` + `request-to-flow` mapper + auto-layout. (FE-4, FE-5)
- **Link:** api → agent (intake), web → api (graph).
- **Done-check:** submitting a request renders ~9–10 department stages with parallel review branches on the canvas.

### F3 — Agents do the work (status progression)
*Department agents reason and stages advance.*
- **DB:** `agent_tasks` + node status. (BE-1)
- **AG:** department agent factory + tools (registry/policy/calcs) + `Decision` model + `/agents/run` + FunctionModel fallback. (AG-2, AG-3, AG-4, AG-5, AG-7, AG-8)
- **BE:** orchestration worker — eligibility, run each node, persist tasks + status + status_text. (BE-5)
- **FE:** node-detail panel (agent, tasks, progress, latest status); status colors. (FE-4)
- **Link:** api ↔ agent (run per node).
- **Done-check:** with no LLM key, a request runs intake→…→report; every node ends `completed` with real tasks and plain-language status.

### F4 — Live canvas (real-time)
*You watch it happen.*
- **BE:** in-process SSE bus + `GET /requests/:id/events`. (BE-8)
- **FE:** `useRequestStream` hook patches the query cache; canvas + panels update live; poll fallback. (FE-2, FE-6)
- **Link:** browser ⇇ SSE ⇇ engine.
- **Done-check:** node status changes appear on the canvas with no manual refresh; `curl -N …/events` streams events.

### F5 — Cross-department dependencies *(the differentiator)*
*An agent declares it's blocked; the system gates and auto-unblocks.*
- **DB:** `node_dependencies`. (BE-1)
- **AG:** `raise_dependency(on_department, reason)` tool surfacing `Decision.blocked_on`. (AG-4)
- **BE:** gate on unresolved deps, mark `blocked` with the agent's reason, re-run on unblock (capped). (BE-6)
- **FE:** blocked node shows "waiting for [department]"; live unblock animation. (FE-6)
- **Link:** agent declares → engine records/gates → SSE → canvas.
- **Done-check:** Finance enters `blocked` with the agent's own reason while IT runs; when IT completes, Finance auto-unblocks and completes — visible live.

### F6 — Traceability / audit trail
*Every change is explainable.*
- **DB:** `audit_events` (append-only). (BE-1)
- **BE:** append on every transition; org-wide and per-request audit read endpoints. (BE-9)
- **FE:** node Activity tab + Reports tab audit browser with filters. (FE-4, FE-10)
- **Link:** engine writes → api reads → FE renders.
- **Done-check:** a completed run's audit shows actor/action/reason/timestamp for every transition, browsable and filterable; the F5 block reason is the agent's own text.

### F7 — Human executive approval
*A person makes the call, with a written reason.*
- **BE:** approval node pauses the request at `awaiting_approval`; `POST /requests/:id/approve {decision, justification}` resumes/stops. (BE-7)
- **FE:** My Work pending-approval queue + Approve/Reject with a required justification. (FE-7)
- **Link:** FE approve → api resume → engine continues → SSE.
- **Done-check:** the workflow halts at approval and appears in My Work; approving with a justification resumes execution; rejecting stops the request; both are audited with the text.
- **Status note (Overall 🟡):** the gate, the `POST /requests/:id/approve` endpoint (approver-only, justification required), and the My Work queue ship and are wired; the e2e walks park → reject-empty-justification → approve → completion with no LLM keys. The "audited with the text" clause is the one piece deferred: durable audit persistence is F6's `audit_events`, which isn't built yet, so for now the justification is required and validated at the API and logged. Overall flips ✅ when F6 lands and the gate transitions append the reason.

### F8 — Execution stage & final report
*Post-approval work runs and a report is produced.*
- **BE:** HR/Ops/Implementation stages run after approval; generate a final report on completion. (BE-5 cont.)
- **AG:** execution-stage agents reuse the F3 factory. 🟡 *partially covered by F3's agent work.*
- **FE:** report display on the Review & Report node / Reports tab. (FE-10)
- **Link:** engine → report → FE.
- **Done-check:** after approval, execution stages complete and a final report (request, approvals, flags, time taken) is generated and viewable.

### F9 — Roster, Policies & Integrations views
*The supporting tabs show real data.*
- **DB:** `department_policies` (seeded). (BE-1)
- **BE:** agents roster + policies read endpoints. (BE-9)
- **FE:** Agents roster grouped by team with live status; Policies read-only browser; Integrations display-only cards. (FE-9, FE-11)
- **Link:** web → api.
- **Done-check:** Agents shows seeded agents with status that goes busy while they own an in-progress node; Policies shows the policies agents consult; Integrations renders cleanly.

### F10 — Seeding & demo data
*Usable from a cold start; demo-ready.*
- **DB:** `department_policies`. (BE-1)
- **BE:** on org creation seed departments(teams)+agents+policies+approver; demo-seed path: one org + approver login + a completed sample request + audit history. (BE-10)
- **Link:** runs inside org-create + a boot/seed path.
- **Done-check:** a fresh org has departments/agents/policies; with the demo seed, Home/Reports/Agents are populated before any new request runs.

---

## Dependencies & optimal sequencing

What must come before what, so we know what can run in parallel and what is strictly sequential.

### Dependency map

`A → B` = A must be **done** before B's done-check can pass. *(soft)* = nicer-with but can be
developed concurrently and integrated.

| Feature | Must come after | Nicer-with (soft) | Can start |
|---|---|---|---|
| F0 shell | — | — | ✅ done |
| F1 request spine | F0 | — | ✅ done |
| F2 workflow graph | F1 | — | **now** |
| F3 agents run | F2 | — | after F2 |
| F4 live SSE | F3 | — | plumbing can start mid-F3; done-check after F3 |
| F5 cross-deps | F3 | F4 *(to see unblock live)* | after F3 |
| F6 audit trail | F3 | — | after F3 |
| F7 approval | F3 | F4 | after F3 |
| F8 execution + report | F7 (and F3) | — | after F7 |
| F9 roster/policies/integrations | F0 | F3 *(live status)*, F10 *(policy data)* | read-views after F0; finalize after F3 |
| F10 seeding | agents/policies tables (created in F2/F3) | F1–F8 *(for the demo request)* | basic seed early; full demo seed last |

```
F0 ✅ ─▶ F1 ✅ ─▶ F2 ─▶ F3 ─┬─▶ F4
                            ├─▶ F5     (after F4 for live unblock)
                            ├─▶ F6
                            └─▶ F7 ─▶ F8
F0 ─────────────────▶ F9   (read-views early; live status needs F3)
(agents/policies tables) ─▶ F10-basic ;  F1..F8 ─▶ F10-demo
```

**Critical path:** `F0 → F1 → F2 → F3 → F7 → F8`. That chain is the floor on wall-clock time;
everything else (F4, F5, F6, F9, F10) overlaps alongside it.

### Optimal plan (wave by wave)

F0 and F1 are done, so we are entering Wave A. **Tip that unlocks parallelism:** front-load the
schema — create *all* remaining tables (`workflow_nodes`/`edges`, `agent_tasks`,
`node_dependencies`, `audit_events`, `agents`, `department_policies`) in one early migration at the
start of F2. That single DB step lets F9 and F10-basic proceed in parallel instead of waiting.

- **Wave A — now.**
  - *Critical track:* **F2** (the graph — nothing downstream can start until it lands).
  - *Parallel (spare hands):* front-load the schema migration; scaffold **F9** read-views (Agents /
    Policies / Integrations) against seeded/stub data; start **F10-basic** (seed
    departments/agents/policies on org create).
- **Wave B — after F2.**
  - *Critical track:* **F3** (the orchestration run loop).
  - *Parallel:* finish **F9** (wire live status once F3 lands); finish **F10-basic**.
- **Wave C — after F3 (the fan-out: up to 4 parallel tracks).**
  - **F4** (live SSE), **F5** (cross-deps), **F6** (audit), **F7** (approval) are independent
    concerns built on F3's loop. Land **F4 first** so F5 and F7 get live visuals; integrate the rest
    as they finish.
- **Wave D — after F7.**
  - **F8** (execution + final report). Then **F10-demo** (the demo org with a completed sample
    request + audit) — it needs the whole pipeline, so it goes last.

So after F0/F1, the remaining minimum is **4 waves** (A→B→C→D) versus 9 if done strictly one at a
time — the win comes from fanning out F4/F5/F6/F7 in Wave C and overlapping F9/F10 into A/B.

### If working solo / one track at a time

Strict sequential order that respects every dependency and keeps each step demoable:
**F2 → F3 → F4 → F6 → F5 → F7 → F8 → F9 → F10.** (F4 before F5/F7 so they're live; F6 early so the
audit trail exists as soon as agents run; F9/F10 last as polish + demo dressing.)

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

**Work is organized as vertical feature slices** (above): each feature owns its DB, backend, agent, frontend, and linking, and is done only end-to-end. The per-layer files (`PRD-BACKEND.md`, `PRD-AGENT.md`, `PRD-FRONTEND.md`) remain as the contract + step reference, addressed by task id.

**Orchestrator placement.** Go owns the durable system of record and deterministic orchestration; the Python service does reasoning only. The web app talks only to Go.

**Data model (Go / Postgres).** New entities: `requests`, `workflow_nodes`, `workflow_edges`, `agent_tasks`, `node_dependencies`, `audit_events`, `agents`, `department_policies`. A request belongs to an organization; departments reuse the existing `teams` table; one agent is seeded per department and linked to its team. Node status: `pending | in_progress | completed | blocked`. Request status: `submitted | in_progress | awaiting_approval | approved | rejected | completed`. `audit_events` is append-only.

**Deep module: Orchestration engine (Go).** A request runs on a background worker. Public interface stays small: start a request, and submit an approval decision. Internally it computes node eligibility (all predecessor edges originate from completed nodes AND the node has no unresolved dependencies), advances statuses, writes audit events, and publishes events to the bus. The executive-approval node parks the request at `awaiting_approval` until a human decides. Per-stage pacing is configurable. If the agent service errors, a deterministic fallback decision is used so a run never stalls.

**Agent-declared cross-dependencies (the differentiator).** Dependencies are NOT hardcoded by the planner. When the engine runs a department agent and the returned decision carries a `blocked_on` declaration (produced by the agent's `raise_dependency` tool), the engine marks the node `blocked`, records a `node_dependencies` row with the agent's own reason, and writes audit + event. When the blocking node completes, the engine resolves the dependency and re-invokes the blocked agent with the blocker's output added to its upstream context (capped re-runs to prevent loops).

**Deep module: Agent service client (Go).** Typed `Intake(request, org_context) -> Plan` and `Run(agent_type, request, upstream_context, org_context) -> Decision` over the Python service. Go injects an org-context snapshot (information systems + policies) and upstream summaries so agent tools read injected data rather than calling back into Go. A typed "agent unavailable" error triggers the fallback path.

**Deep module: SSE event bus (Go).** In-process publish/subscribe with channel fan-out keyed by request id. The engine publishes; the SSE endpoint subscribes and streams to the browser, unsubscribing on client disconnect.

**UI-facing API (Go).** Create/list requests under an org; get a request's full graph; get node detail; approve a request; an SSE stream per request authenticated via a token query parameter; list agents, audit (org-wide and per-request), and policies.

**SSE event contract.** Event types `node_status`, `request_status`, `task`, and `audit`, each a JSON object carrying the request id, the changed entity, and a timestamp. The frontend patches its query cache from these.

**Agent layer (Python, Pydantic AI).** Retire the raw HTTP + manual JSON parsing for this flow. Each department agent and the intake planner are Pydantic AI agents with typed output models (`Plan`, `Decision`), tool-calling, dependency injection for org/upstream context, and automatic validation + retries. Tools: read the information-system registry, fetch the department policy, domain calculations (e.g. budget assessment for Finance, compliance lookup for Legal), and `raise_dependency(on_department, reason)` surfacing on `Decision.blocked_on`. The intake planner chooses stages from a fixed department catalog and falls back to a default template if validation fails.

**Offline fallback.** When no provider key is set, model selection returns a Pydantic AI `FunctionModel` that emits deterministic `Plan`/`Decision` output (including a Finance `blocked_on` case), so the full flow runs with no network.

**Deep module: request-to-flow mapper (TS).** A pure function transforming the request graph into React Flow nodes and edges with an auto-layout derived from stage order and edges. No side effects.

**Frontend.** Shell navigation: Home, My Work, Requests, Workflows, Agents, Reports, Policies, Integrations, Teams. The Workflows view is a three-panel layout: left request overview, center canvas, right node detail (Overview / Tasks / Activity). `useRequestStream` subscribes to SSE and patches the cache, with an interval-poll fallback. Policies is a read-only browser; Integrations is display-only.

**Seeding.** On organization creation, seed the standard department teams, one agent per team, starter department policies, and mark the creator as approver. A demo-seed path additionally creates one organization with an approver login and a completed sample request plus audit history.

## Testing Decisions

A good test asserts **external behavior through a module's public interface**, not its internals. Tests must be deterministic: no live LLM calls, no real time-of-day dependence, no flaky network.

Deep modules to test (test the slice's logic, not its glue):

1. **Orchestration engine (Go)** [F3/F5/F7] — drive it through its public interface with an in-memory repo and a fake agent client: a no-dependency request advances every node to completed; a `blocked_on: IT` decision blocks Finance with the agent's reason, then completes it after IT (dependency resolved); the approval node parks at `awaiting_approval` and approve/reject produce the right terminal status; every transition writes an audit event; re-run capping holds.
2. **Department agents + raise_dependency (Python)** [F3/F5] — via Pydantic AI `TestModel`/`FunctionModel`: typed `Decision` shape; Finance lacking IT output returns `blocked_on: IT` and the tool fired; re-run with IT's summary completes; intake returns a valid `Plan`, and invalid model output still yields the default template.
3. **Agent client + SSE bus (Go)** [F2/F4] — client parses `Plan`/`Decision` from a stub server and maps a timeout to the typed unavailable error; the bus delivers to current subscribers and stops after unsubscribe.
4. **request-to-flow mapper (TS)** [F2] — pure vitest unit test: a known request graph maps to the expected React Flow node/edge set with correct statuses and a layout placing parallel branches at the same rank.

Prior art: Go table-driven tests exist in the compiler/handler packages; Python uses pytest (a smoke test exists); the web app has vitest wired (the mapper is the first real unit test).

## Out of Scope

- Real external integrations (SAP, LexisNexis, Workday, Slack, etc.). The Integrations tab is display-only.
- A Policies authoring UI. Policies are seeded and read-only; agents consult them.
- Multi-workflow history depth, request pagination at scale, and notification backends beyond the canvas and audit feed.
- OAuth/SSO and fine-grained RBAC beyond the existing JWT + org roles.
- Mobile-responsive layouts; the canvas is desktop-first.
- Removing or rewriting the dormant authoring-tool code; it stays in the repo, off this path.
- Multi-replica orchestration (the event bus is in-process for now).

## Further Notes

- The demo golden path is the acceptance bar and it walks through the features in order: Home (F1) → submit (F1) → live Workflows canvas (F2/F3/F4) → Finance "waiting for IT" → unblock (F5) → My Work approval (F7) → execution + report (F8) → Reports/audit (F6). Seed (F10) makes it non-empty.
- CI enforces lint/typecheck/test/race across all three apps, a migration up/rollback/re-apply check, and a docker-compose e2e smoke; each feature should extend these (its unit tests, and the e2e driving the request further along the path).
- Reliability bar: with all LLM keys unset, the entire flow must still complete via the `FunctionModel` / deterministic-fallback paths.
- A copy of an earlier slice-organized PRD was published as GitHub issue #4; this file (feature-organized, with the live tracker) is the canonical source.
