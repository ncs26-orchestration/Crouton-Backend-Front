# PRD — Backend (Go API + Orchestration Engine)

Owner: Backend team · Stack: Go 1.25, Echo, pgx, JWT · Skills: `.agents/skills/backend/`

## Context

We are pivoting from the dormant "AUP" authoring tool to the **AI Organization OS**: a user submits
a business request, an Intake agent plans a department workflow, department agents execute in
parallel with **agent-declared cross-dependencies**, a human approves at the Executive gate, and
every transition is logged to an append-only audit trail and streamed live to the UI.

Backend owns the durable system of record and the deterministic orchestration. The Python service
(see `PRD-AGENT.md`) does the reasoning; the web app (see `PRD-FRONTEND.md`) renders it. This PRD
also defines the **shared contracts** the other two teams build against.

## Scope / ownership

- New schema (migrations + pgx repos) for requests, workflow graph, tasks, dependencies, audit, agents, policies.
- The orchestration engine: a background worker per request that walks the graph, gates on edges and
  dependencies, calls the agent service per node, persists results, and streams SSE.
- The UI-facing HTTP API and the SSE stream.
- Seeding (departments, agents, policies on org creation + one demo org with history).
- Out of scope: the AUP extract/compile/copilot/deploy handlers stay registered but untouched.

## Data model (new migrations — use `dbmate new`, real UTC timestamps)

- `requests` (id, org_id, title, description, requester_user_id, priority, status, progress, estimated_completion, created_at)
- `workflow_nodes` (id, request_id, key, name, agent_id, team_id, status, description, progress_percent, started_at, completed_at)
- `workflow_edges` (id, request_id, source_node_id, target_node_id, edge_type `sequential|parallel`)
- `agent_tasks` (id, node_id, title, status, started_at, completed_at)
- `node_dependencies` (id, dependent_node_id, blocking_node_id, reason, resolved_at) — `reason` is the agent-declared text
- `audit_events` (id, request_id, node_id NULL, actor, action, reason, created_at) — append-only
- `agents` (id, org_id, team_id, agent_type, name, avatar, capabilities)
- `department_policies` (id, org_id, team_id, title, body)

Node status enum: `pending | in_progress | completed | blocked`. Request status: `submitted |
in_progress | awaiting_approval | approved | rejected | completed`.

Repos mirror the existing pgx pattern in `apps/api/internal/repo/orgs.go` / `projects.go`. Follow
`i-golang-database`, `i-golang-project-layout`, `i-golang-naming`.

## HTTP API contract (UI-facing, JWT, registered in `apps/api/internal/http/server.go`)

```
POST /orgs/:orgId/requests        {title, description, priority}            -> {request}
GET  /orgs/:orgId/requests                                                  -> {requests:[...]}
GET  /requests/:id                  full graph                              -> {request, nodes:[...], edges:[...], agents:[...]}
GET  /requests/:id/nodes/:nodeId    node detail                            -> {node, tasks:[...], activity:[...]}
POST /requests/:id/approve         {decision:"approve"|"reject", justification} -> {request}
GET  /requests/:id/events          SSE stream (auth via ?token=)
GET  /orgs/:orgId/agents           roster + live status                    -> {agents:[...]}
GET  /orgs/:orgId/audit            org-wide audit                          -> {events:[...]}
GET  /requests/:id/audit           per-request audit                       -> {events:[...]}
GET  /orgs/:orgId/policies         seeded department policies              -> {policies:[...]}
```

### SSE event contract (`text/event-stream`)

One JSON object per event; `event:` is the type. Frontend patches its query cache from these.

```
event: node_status   data: {request_id, node_id, key, status, progress_percent, status_text, at}
event: request_status data: {request_id, status, progress, at}
event: task           data: {request_id, node_id, task_id, title, status, at}
event: audit          data: {request_id, node_id, actor, action, reason, at}
```

Use an in-process pub/sub (channel fan-out keyed by request_id); SSE handlers subscribe. Honor
`context.Context` cancellation on client disconnect (`i-golang-context`, `i-golang-concurrency`).

## Agent service contract (Go ↔ Python — also in `PRD-AGENT.md`)

```
POST {AGENT_URL}/agents/intake
  req:  {request:{title,description,priority}, org_context:{...}}
  resp: {nodes:[{key,name,agent_type,department}], edges:[{from,to,type}]}

POST {AGENT_URL}/agents/run
  req:  {agent_type, request, upstream_context:[{node_key,summary,...}], org_context:{...}}
  resp: {summary, flags:[{severity,message}], tasks:[{title,status}], status_text,
         blocked_on: null | {on_department, reason}}
```

Go injects `org_context` (IS registry snapshot + policies) and `upstream_context` so the agent's
tools read from injected data — no callback into Go.

## Orchestration engine (`apps/api/internal/orchestrator/`)

A goroutine per request, launched from the create handler. Follow `i-golang-concurrency`,
`i-golang-design-patterns` (graceful shutdown), `i-golang-error-handling`.

1. On submit: call `/agents/intake`, persist nodes+edges, set request `in_progress`, audit + SSE, start worker.
2. Loop: for each `pending` node whose predecessors are all `completed`, set `in_progress` (audit+SSE),
   call `/agents/run`.
   - If `blocked_on` set → node `blocked`, insert `node_dependencies` (agent reason), audit+SSE, leave for re-run.
   - Else → persist `agent_tasks` + `status_text`, node `completed`, resolve dependencies this node blocked, audit+SSE.
3. **Re-run on unblock**: when a node completes, any node blocked on it is re-queued and its agent
   re-invoked with the blocker's output in `upstream_context` (cap re-runs to prevent loops).
4. Executive Approval node: set `in_progress`, request `awaiting_approval`, emit a pending-approval
   audit+SSE, then pause. `POST /requests/:id/approve` resumes (approve → execution stage; reject → request `rejected`).
5. Pacing: configurable per-stage delay (`ORCH_STEP_DELAY_MS`) so the canvas progression is watchable.
6. Resilience: any agent error → use the agent service's fallback decision so the run always completes.

## Seeding

- `handler/orgs.go CreateOrg`: seed standard department `teams` (Finance/Legal/IT/HR/Operations/
  Planning/Executive), one `agents` row per team, a starter `department_policies` set, creator = approver.
- A boot seed routine behind `DEMO_SEED=true` (or a dedicated migration): one demo org with an
  approver login and one already-completed sample request + its audit history, so Home/Reports/Agents
  are populated on first load.

## Acceptance criteria

- `make migrate-up` applies cleanly; `make psql` shows seeded departments, agents, policies, demo request.
- `POST /orgs/:id/requests` for "Open a new office in Berlin" returns a graph; the worker drives it to
  the approval gate; `GET /requests/:id/audit` shows an append-only entry (actor/action/reason/at) for
  every transition.
- `node_dependencies.reason` for the Finance→IT block contains the agent's `raise_dependency` text.
- `curl -N /requests/:id/events?token=...` streams node_status/audit events live.
- With all LLM keys unset, the flow still completes end to end via the agent fallback.
- `go build ./...` and `go test ./...` pass; lint clean (`i-golang-lint`).
