# PRD — Backend (Go API + Orchestration Engine)

Owner: Backend team · Stack: Go 1.25, Echo, pgx, JWT · Skills: `.agents/skills/backend/`

## Context

Pivot from the dormant "AUP" authoring tool to the **AI Organization OS**: a user submits a business
request, an Intake agent plans a department workflow, department agents execute in parallel with
**agent-declared cross-dependencies**, a human approves at the Executive gate, and every transition
is logged to an append-only audit trail and streamed live to the UI.

Backend owns the durable system of record and the deterministic orchestration. This PRD also defines
the **shared contracts** the Frontend (`PRD-FRONTEND.md`) and Agent (`PRD-AGENT.md`) teams build
against. Out of scope: AUP `extract`/`compile`/`copilot`/`deploy` handlers stay registered, untouched.

Build order is the feature order below (each feature depends on the ones above it).

---

## Shared contracts (read first — Frontend and Agent depend on these)

### UI-facing HTTP API (JWT, registered in `apps/api/internal/http/server.go`)
```
POST /orgs/:orgId/requests        {title, description, priority}                 -> {request}
GET  /orgs/:orgId/requests                                                       -> {requests:[...]}
GET  /requests/:id                 full graph                                    -> {request, nodes:[...], edges:[...], agents:[...]}
GET  /requests/:id/nodes/:nodeId   node detail                                  -> {node, tasks:[...], activity:[...]}
POST /requests/:id/approve         {decision:"approve"|"reject", justification}  -> {request}
GET  /requests/:id/events          SSE stream (auth via ?token=)
GET  /orgs/:orgId/agents                                                         -> {agents:[...]}
GET  /orgs/:orgId/audit                                                          -> {events:[...]}
GET  /requests/:id/audit                                                         -> {events:[...]}
GET  /orgs/:orgId/policies                                                       -> {policies:[...]}
```
### SSE events (`text/event-stream`, one JSON object per event)
```
event: node_status    data: {request_id, node_id, key, status, progress_percent, status_text, at}
event: request_status data: {request_id, status, progress, at}
event: task           data: {request_id, node_id, task_id, title, status, at}
event: audit          data: {request_id, node_id, actor, action, reason, at}
```
### Go ↔ Python agent service
```
POST {AGENT_URL}/agents/intake  {request, org_context}                              -> {nodes:[{key,name,agent_type,department}], edges:[{from,to,type}]}
POST {AGENT_URL}/agents/run     {agent_type, request, upstream_context, org_context} -> {summary, flags:[...], tasks:[...], status_text, blocked_on: null|{on_department, reason}}
```

---

## BE-1 — Schema & migrations
Goal: the tables the whole system reads/writes. Skills: `i-golang-database`, `i-golang-project-layout`.
Steps:
1. `dbmate new add_requests` — `requests` (id, org_id, title, description, requester_user_id, priority, status, progress, estimated_completion, created_at).
2. `dbmate new add_workflow_graph` — `workflow_nodes` (id, request_id, key, name, agent_id, team_id, status, description, progress_percent, started_at, completed_at) and `workflow_edges` (id, request_id, source_node_id, target_node_id, edge_type).
3. `dbmate new add_agent_tasks_deps` — `agent_tasks` (id, node_id, title, status, started_at, completed_at) and `node_dependencies` (id, dependent_node_id, blocking_node_id, reason, resolved_at).
4. `dbmate new add_audit_events` — `audit_events` (id, request_id, node_id NULL, actor, action, reason, created_at). Index on (request_id, created_at).
5. `dbmate new add_agents_policies` — `agents` (id, org_id, team_id, agent_type, name, avatar, capabilities) and `department_policies` (id, org_id, team_id, title, body).
6. Use real UTC timestamps in filenames; every file has `migrate:up` and `migrate:down`. Run `make migrate-up`.
Acceptance: `make migrate-up` is clean; `make psql` shows all tables; `migrate-down` of the latest works.
Status enums — node: `pending|in_progress|completed|blocked`; request: `submitted|in_progress|awaiting_approval|approved|rejected|completed`.

## BE-2 — Repositories
Goal: pgx data access mirroring `apps/api/internal/repo/orgs.go`. Skills: `i-golang-database`, `i-golang-naming`, `i-golang-structs-interfaces`.
Steps:
1. `repo/requests.go` — Create, GetByID, ListByOrg, UpdateStatusProgress.
2. `repo/workflow.go` — InsertNodes/Edges (batch), ListNodes/Edges by request, GetNode, UpdateNodeStatus, Insert/UpdateTasks.
3. `repo/dependencies.go` — InsertDependency, ListBlockedBy(nodeID), ResolveDependenciesBlockedBy(nodeID).
4. `repo/audit.go` — Append(event), ListByRequest, ListByOrg. Append-only (no update/delete).
5. `repo/agents.go` — ListByOrg, GetByType. `repo/policies.go` — ListByOrg, GetByTeam.
6. Every method takes `context.Context` and returns wrapped errors (`i-golang-error-handling`).
Acceptance: a `go test ./internal/repo/...` round-trips each aggregate against a test DB.

## BE-3 — Agent service client
Goal: typed client for the Python service. Skills: `i-golang-context`, `i-golang-error-handling`.
Steps:
1. `internal/agentclient/client.go` with `Intake(ctx, req) (Plan, error)` and `Run(ctx, req) (Decision, error)` over `AGENT_URL`.
2. Define request/response structs matching the shared contract exactly.
3. Per-call timeout via context; on any error return a typed `ErrAgentUnavailable` so callers can fall back.
Acceptance: unit test with a stub HTTP server returns parsed `Plan`/`Decision`; timeout path returns the typed error.

## BE-4 — Request intake + plan persistence
Goal: submitting a request produces a persisted graph. Skills: `i-api-design-principles`.
Steps:
1. `handler/requests.go NewRequestsHandler(...)`; register `POST/GET /orgs/:orgId/requests`.
2. On create: insert `requests` row (status `submitted`); build `org_context` (IS snapshot + policies); call `agentclient.Intake`.
3. Map the returned plan to `workflow_nodes` (resolve `agent_type`→`agents.id`, `department`→`team_id`) and `workflow_edges`; set every node `pending`.
4. Write an `audit_events` "request.created" entry; set request `in_progress`; launch the worker (BE-5).
5. Implement `GET /requests/:id` returning the full graph (request + nodes + edges + agents).
Acceptance: `POST` "Open a new office in Berlin" returns a request; `GET /requests/:id` shows ~9–10 nodes with edges, all `pending`; audit has `request.created`.

## BE-5 — Orchestration worker (state machine)
Goal: nodes advance automatically. Skills: `i-golang-concurrency`, `i-golang-design-patterns`.
Steps:
1. `internal/orchestrator/engine.go`: a goroutine per request keyed in a registry; graceful shutdown on server stop.
2. Loop: find `pending` nodes whose predecessor edges all point from `completed` nodes → eligible.
3. For each eligible node: set `in_progress` (audit + publish), build `upstream_context` from completed predecessors, call `agentclient.Run`.
4. On success (no `blocked_on`): persist `agent_tasks` + `status_text`, set node `completed` (audit + publish), recompute eligibility.
5. On `agentclient` error: use a deterministic fallback decision so the node still completes (never stall).
6. Per-stage pacing via `ORCH_STEP_DELAY_MS` so progression is watchable.
7. When all terminal nodes are `completed`, set request `completed` + audit.
Acceptance: a request with no dependencies runs intake→…→report, every node ending `completed`, with an audit entry per transition.

## BE-6 — Agent-declared cross-dependencies + re-run on unblock
Goal: the Finance→IT "Waiting for IT" behavior, emergent from the agent. Skills: `i-golang-concurrency`.
Steps:
1. When `Run` returns `blocked_on`: resolve `on_department`→the blocking node; insert a `node_dependencies` row with the agent's `reason`; set the node `blocked` and `status_text` to the reason; audit `node.blocked` + publish.
2. A node is eligible only if predecessors are `completed` AND it has no unresolved `node_dependencies`.
3. On any node completion: call `ResolveDependenciesBlockedBy(nodeID)`; re-queue nodes whose deps are now all resolved.
4. Re-invoke a re-queued agent with the blocker's output added to `upstream_context`; cap re-runs per node (e.g. 3) to prevent loops.
Acceptance: Finance enters `blocked` with the agent's reason while IT runs; when IT completes, `node_dependencies.resolved_at` is set, Finance re-runs and completes; audit shows block→unblock with the agent-authored reason.

## BE-7 — Executive approval gate (human)
Goal: pause for a human decision. Skills: `i-api-design-principles`.
Steps:
1. When the `exec_approval` node becomes eligible: set it `in_progress`, request `awaiting_approval`, audit `approval.requested` + publish; the worker parks (does not auto-complete).
2. `POST /requests/:id/approve {decision, justification}`: on `approve` → complete the node (audit `approval.granted` with justification), resume the worker into execution (HR/Ops/Implementation/Report). On `reject` → request `rejected`, audit `approval.rejected`.
3. Validate the caller has an approver role.
Acceptance: workflow halts at exec_approval; approve resumes to completion; reject stops the request; both write audit with the justification text.

## BE-8 — SSE streaming
Goal: live UI updates. Skills: `i-golang-concurrency`, `i-golang-context`.
Steps:
1. `internal/orchestrator/bus.go`: in-process pub/sub, channel fan-out keyed by request_id; `Publish(event)` + `Subscribe(requestID)`.
2. Replace the "audit + publish" calls in BE-5..7 with bus publishes carrying the SSE event shapes.
3. `handler/events.go GET /requests/:id/events`: authenticate via `?token=` (EventSource can't set headers), set SSE headers, stream events, and unsubscribe on `c.Request().Context().Done()`.
Acceptance: `curl -N /requests/:id/events?token=...` during a run streams node_status/audit/task events in order; closing the client unsubscribes cleanly (no goroutine leak).

## BE-9 — Read endpoints for the tabs
Goal: data for My Work, Agents, Reports, Policies, node detail.
Steps:
1. `GET /requests/:id/nodes/:nodeId` → node + tasks + activity (its audit slice).
2. `GET /orgs/:orgId/agents` → roster with derived live status (busy if owning an in_progress node).
3. `GET /orgs/:orgId/audit` and `GET /requests/:id/audit` → ordered events.
4. `GET /orgs/:orgId/policies` → seeded department policies.
Acceptance: each returns shapes matching the contract; node detail's `activity` matches that node's audit entries.

## BE-10 — Seeding
Goal: usable from a cold start. Skills: `i-golang-project-layout`.
Steps:
1. In `handler/orgs.go CreateOrg`: seed standard department `teams` (Finance/Legal/IT/HR/Operations/Planning/Executive), one `agents` row per team, a starter `department_policies` set, creator = approver.
2. A boot seed behind `DEMO_SEED=true` (or a dedicated migration): one demo org + approver login + one already-completed sample request with full audit history.
Acceptance: a fresh org has departments/agents/policies; with `DEMO_SEED=true`, Home/Reports/Agents are populated before any new request runs.

---

## Definition of done (backend)
- `go build ./...` and `go test ./...` pass; lint clean (`i-golang-lint`).
- End-to-end: submit "Open a new office in Berlin" → worker drives to approval gate → approve → completion; audit is append-only with actor/action/reason/at for every transition.
- The Finance→IT dependency reason in `node_dependencies` is the agent's own text.
- With all LLM keys unset, the flow still completes via the fallback path.
