# PRD — Machine Maintenance & Incidents

Owner: Cross-team (Backend + Agent + Frontend + Mobile) · Extends the AI Organization OS with
equipment tracking, technician-driven incident resolution, and AI-assisted diagnostics.

> This PRD covers a **self-contained feature domain** layered on top of the existing request/workflow
> system. Machines are organizational assets; incidents are a specialized flow that can escalate into
> a full department workflow request when cross-department coordination is needed. The same
> traceability, agent reasoning, and live-update principles from `PRD.md` apply.

---

## Problem Statement

Organizations that operate physical equipment (manufacturing lines, IT infrastructure, facility
systems) need their field technicians to report, diagnose, and resolve machine problems quickly.
Today this lives in paper logs, WhatsApp groups, and spreadsheets — there is no structured trail of
what broke, what the diagnosis was, who fixed it, or whether the fix held. When a problem requires
parts, budget, or cross-department help, the escalation is informal and drops context.

The AI Organization OS already orchestrates cross-department work for business requests. Extending it
to machine maintenance means the same agents, traceability, and workflow engine serve a second
high-value use case — and the mobile app becomes the primary interface, because technicians are on
the floor, not at a desk.

## Solution

Add a **machine registry** and an **incident lifecycle** to the platform. Technicians report issues
(or machines push telemetry that auto-creates incidents). A **Maintenance Agent** diagnoses the
problem in a conversational thread, suggests fix steps, and references the machine's specs, error
codes, and history. The technician works the fix and marks it resolved — or the agent escalates to
a full workflow request when the problem needs budget, parts, or cross-department coordination. Every
action is audited. The mobile app is the primary surface for technicians; the web dashboard gives
managers an operational overview.

---

## Concepts

### Technician Role

A technician is an existing org member with the `technician` role on a team. This is a **team-level
role**, not a new org-level role — one person can be a technician on the Facilities team and a
regular member on another.

```
team_members.role:  lead | member | technician
```

Technicians:
- Are assigned to machines (primary responsibility).
- See the Machines tab and their assigned machines.
- Receive push notifications on machine status changes and new incidents.
- Can report incidents, chat with the maintenance agent, resolve, or escalate.
- Cannot approve budget or parts — that flows through the existing executive approval gate.

Managers (team leads, org admins) see all machines, all incidents, and operational metrics.

### Machine Lifecycle

```
operational ──→ degraded ──→ down ──→ maintenance ──→ operational
     │              │          │                          ▲
     │              │          └── resolved (simple fix) ─┘
     │              └── auto-recovers ───────────────────┘
     └── scheduled maintenance ──→ maintenance ──────────┘
```

Status meanings:
- **operational** — running normally, no open incidents.
- **degraded** — running but with warnings (elevated temperature, minor errors). May have an open
  incident.
- **down** — stopped, not producing. Has an open incident.
- **maintenance** — intentionally taken offline for scheduled or reactive maintenance.

### Incident Lifecycle

```
reported ──→ diagnosing ──→ in_repair ──→ resolved
                │                            ▲
                └──→ escalated ──→ (full workflow request runs)
                                         │
                                         └──→ resolved (when request completes)
```

Status meanings:
- **reported** — technician (or auto-telemetry) created the incident.
- **diagnosing** — maintenance agent is analyzing; conversation active.
- **in_repair** — technician is working the fix.
- **resolved** — fix confirmed, machine back to operational.
- **escalated** — problem requires cross-department help; a linked request was created and the
  existing workflow engine handles it (Finance for budget, Operations for parts, etc.). The incident
  resolves when the linked request completes.

### Two Tiers of Response

**Tier 1 — Quick fix (technician + agent chat).** Most incidents. The maintenance agent diagnoses,
the technician fixes, done. No workflow request, no department reviews. Fast, lightweight.

**Tier 2 — Escalation (becomes a full workflow request).** The agent determines the problem needs
parts ordering, budget approval, or cross-department coordination. It escalates by creating a
request linked to the incident. The existing F1-F8 pipeline runs: Finance reviews cost, Operations
handles logistics, executive approves. The incident tracks the request and resolves when it
completes.

---

## Feature Tracker

Legend: ✅ done · 🟡 in progress · ⬜ not started.

| # | Feature (vertical slice) | DB | BE | AG | FE | Mobile | Link | Overall |
|---|---|----|----|----|----|----|----|--------|
| M-F1 | Machine registry + technician role | ⬜ | ⬜ | – | ⬜ | ⬜ | ⬜ | ⬜ |
| M-F2 | Incident reporting + lifecycle | ⬜ | ⬜ | – | ⬜ | ⬜ | ⬜ | ⬜ |
| M-F3 | Maintenance agent + diagnostic chat | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| M-F4 | Escalation to workflow request | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ |
| M-F5 | Telemetry ingestion + auto-incidents | ⬜ | ⬜ | – | ⬜ | – | ⬜ | ⬜ |
| M-F6 | Mobile incident experience | – | – | – | – | ⬜ | ⬜ | ⬜ |
| M-F7 | Machine dashboard + operational metrics | – | ⬜ | – | ⬜ | ⬜ | ⬜ | ⬜ |
| M-F8 | Seeding + demo scenario | ⬜ | ⬜ | – | – | – | ⬜ | ⬜ |

---

## Data Model

### New Tables

```sql
-- Machine registry
CREATE TABLE machines (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    team_id         UUID REFERENCES teams(id),
    assigned_user_id UUID REFERENCES users(id),
    name            TEXT NOT NULL,
    machine_type    TEXT NOT NULL DEFAULT '',
    location        TEXT NOT NULL DEFAULT '',
    serial_number   TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'operational',
    last_service_at TIMESTAMPTZ,
    next_service_due TIMESTAMPTZ,
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_machines_org ON machines(org_id);
CREATE INDEX idx_machines_assigned ON machines(assigned_user_id);

-- Machine telemetry (append-only, recent window)
CREATE TABLE machine_telemetry (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    machine_id  UUID NOT NULL REFERENCES machines(id) ON DELETE CASCADE,
    metrics     JSONB NOT NULL,
    error_code  TEXT,
    source      TEXT NOT NULL DEFAULT 'manual',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_telemetry_machine ON machine_telemetry(machine_id, created_at DESC);

-- Incidents
CREATE TABLE machine_incidents (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    machine_id      UUID NOT NULL REFERENCES machines(id) ON DELETE CASCADE,
    reported_by     UUID NOT NULL REFERENCES users(id),
    title           TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    severity        TEXT NOT NULL DEFAULT 'medium',
    status          TEXT NOT NULL DEFAULT 'reported',
    error_code      TEXT,
    diagnosed_cause TEXT,
    resolution_notes TEXT,
    request_id      UUID REFERENCES requests(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at     TIMESTAMPTZ
);
CREATE INDEX idx_incidents_machine ON machine_incidents(machine_id, created_at DESC);
CREATE INDEX idx_incidents_reporter ON machine_incidents(reported_by);

-- Incident discussion thread
CREATE TABLE incident_messages (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id UUID NOT NULL REFERENCES machine_incidents(id) ON DELETE CASCADE,
    sender_type TEXT NOT NULL,
    sender_id   UUID,
    body        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_messages_incident ON incident_messages(incident_id, created_at);
```

### Schema Changes to Existing Tables

```sql
-- Add technician role to team_members
-- team_members.role already accepts TEXT; just document that 'technician' is now valid.

-- No structural change needed — the app layer enforces valid roles.
```

---

## Shared Contracts

### UI-facing HTTP API (JWT, extends `internal/http/server.go`)

```
# Machine registry
POST /orgs/:orgId/machines                             {name, machine_type, location, serial_number, team_id, assigned_user_id}  -> {machine}
GET  /orgs/:orgId/machines                             ?status=&team_id=&assigned_to=me  -> {machines:[...]}
GET  /machines/:id                                     -> {machine, latest_telemetry, open_incidents:[...]}
PUT  /machines/:id                                     {name, status, assigned_user_id, ...}  -> {machine}

# Telemetry (machine-to-system, API key or JWT)
POST /machines/:id/telemetry                           {metrics, error_code, source}  -> {telemetry, incident_created: bool}
GET  /machines/:id/telemetry                           ?limit=&since=  -> {telemetry:[...]}

# Incidents
POST /machines/:id/incidents                           {title, description, severity, error_code}  -> {incident}
GET  /machines/:id/incidents                           ?status=  -> {incidents:[...]}
GET  /incidents/:id                                    -> {incident, machine, messages:[...]}
PUT  /incidents/:id                                    {status, resolution_notes, diagnosed_cause}  -> {incident}
POST /incidents/:id/resolve                            {resolution_notes}  -> {incident}
POST /incidents/:id/escalate                           {reason}  -> {incident, request}

# Incident discussion
POST /incidents/:id/messages                           {body}  -> {message, agent_reply: message|null}
GET  /incidents/:id/messages                           -> {messages:[...]}

# Live stream
GET  /incidents/:id/events                             SSE (auth via ?token=)

# Dashboard
GET  /orgs/:orgId/machines/stats                       -> {total, operational, degraded, down, maintenance, open_incidents}
```

### SSE Events (incident stream)

```
event: machine_status    data: {machine_id, status, at}
event: incident_status   data: {incident_id, machine_id, status, at}
event: incident_message  data: {incident_id, message_id, sender_type, body, at}
event: incident_escalated data: {incident_id, request_id, at}
```

### Go <-> Python Agent Service

```
POST {AGENT_URL}/agents/diagnose
  Request:  {machine, incident, telemetry:[...], message_history:[...], org_context}
  Response: {reply, suggested_actions:[{title, description}], diagnosed_cause: str|null,
             should_escalate: bool, escalation_reason: str|null}
```

---

## Features (Vertical Slices)

### M-F1 — Machine Registry + Technician Role

*Machines exist in the system and technicians are assigned to them.*

- **DB:** `machines` table. Extend `team_members` role documentation to include `technician`.
- **BE:** `repo/machines.go` (Create, GetByID, ListByOrg, Update). `handler/machines.go`
  (POST/GET/PUT). Register routes. Validate `technician` role on assignment.
- **FE:** Machines tab in ShellRail. Machine list (name, type, location, status, assigned
  technician). Machine detail view. Add/edit machine modal (admin only).
- **Mobile:** Machine list screen filtered to `assigned_to=me`. Machine detail.
- **Link:** web/mobile -> api.
- **Done-check:** create a machine "CNC Mill #4" assigned to a technician, it appears in the web
  Machines tab and on the technician's mobile machine list with status "operational".

### M-F2 — Incident Reporting + Lifecycle

*Technicians report problems and work through the fix.*

- **DB:** `machine_incidents` table.
- **BE:** `repo/incidents.go` (Create, GetByID, ListByMachine, UpdateStatus, Resolve).
  `handler/incidents.go` (POST/GET/PUT + resolve). On incident create: update machine status to
  `down` or `degraded` based on severity. On resolve: update machine status to `operational`. Write
  audit events for every status transition.
- **FE:** "Report Issue" button on machine detail. Incident list on machine detail. Incident detail
  view (status, severity, timeline). Resolve action with resolution notes.
- **Mobile:** Report issue screen (title, description, severity, error code). Incident detail.
  Resolve button with notes.
- **Link:** web/mobile -> api. Push notification to team lead on new critical/high incident.
- **Done-check:** technician reports "CNC Mill #4 — overheating error E-203" from their phone,
  machine status flips to "down", incident appears on web dashboard, technician resolves it with
  notes, machine goes back to "operational", audit trail shows the full lifecycle.

### M-F3 — Maintenance Agent + Diagnostic Chat

*An AI agent helps diagnose and suggests fix steps in a live conversation.*

- **DB:** `incident_messages` table.
- **AG:** `agents/maintenance.py` — a Pydantic AI agent with `output_type=DiagnosisResponse`.
  System prompt: you are a maintenance diagnostic agent; you have the machine's specs, error code
  database, service history, and telemetry; diagnose the problem and suggest actionable fix steps.
  Tools:
  - `get_machine_specs(ctx)` — reads machine metadata (specs, manual references).
  - `get_service_history(ctx)` — past incidents, last service date, patterns.
  - `lookup_error_code(ctx, code)` — known error codes and common fixes from metadata.
  - `suggest_escalation(ctx, reason)` — flags that the problem needs cross-department help.
  Offline fallback: `FunctionModel` returns deterministic diagnosis for known error codes (E-203 =
  coolant flow, E-401 = bearing wear, etc.).
- **BE:** `POST /incidents/:id/messages` — persist the technician's message, call
  `/agents/diagnose` with machine context + message history, persist the agent's reply, publish SSE
  event. `handler/incident_messages.go`.
- **FE:** Chat thread on incident detail. Messages styled by sender_type (employee / agent /
  system). Auto-scroll on new message. Agent's suggested actions rendered as a checklist.
- **Mobile:** Same chat thread, optimized for mobile keyboard. SSE subscription for live agent
  replies.
- **Link:** mobile/web -> api -> agent -> api (persist reply) -> SSE -> mobile/web.
- **Done-check:** technician sends "Machine showing E-203, temperature at 95C" from their phone,
  maintenance agent replies within seconds with a diagnosis and 3 suggested fix steps, technician
  replies "filter was clogged, cleaned it", agent responds with follow-up guidance. With no LLM key,
  the fallback agent produces a valid diagnosis for E-203.

### M-F4 — Escalation to Workflow Request

*When a problem needs budget, parts, or cross-department help, the agent escalates.*

- **AG:** `suggest_escalation` tool on the maintenance agent. When the agent determines escalation
  is needed (e.g., pump replacement requires parts order + budget), it sets
  `should_escalate=true` with a reason.
- **BE:** `POST /incidents/:id/escalate` — creates a new request (reusing F1's request creation)
  with title derived from the incident, links `machine_incidents.request_id`, sets incident status
  to `escalated`, writes audit event. The request triggers the existing intake -> workflow ->
  department agents pipeline (F2/F3). When the linked request reaches `completed`, a callback
  resolves the incident and sets machine status to `operational`.
- **FE:** "Escalate" button on incident detail. Escalation card showing the linked request with a
  link to the workflow canvas. On the workflow canvas, a note indicating this request originated
  from a machine incident.
- **Mobile:** Escalate action on incident screen. Linked request card with deep-link to the
  workflow status view (M2).
- **Link:** incident -> request creation (F1) -> workflow engine (F2/F3) -> on request completion
  -> incident resolution.
- **Done-check:** maintenance agent suggests escalation for a pump replacement, technician taps
  "Escalate", a request "CNC Mill #4 — Pump Replacement" is created, the workflow runs through
  Finance/Operations/Approval, when the request completes the incident auto-resolves and the
  machine goes back to operational.

### M-F5 — Telemetry Ingestion + Auto-Incidents

*Machines push metrics; the system detects anomalies and auto-creates incidents.*

- **DB:** `machine_telemetry` table.
- **BE:** `POST /machines/:id/telemetry` — accepts metrics payload, persists to
  `machine_telemetry`. Anomaly check: if `error_code` is present OR a metric exceeds a threshold
  defined in `machines.metadata.thresholds`, auto-create an incident (status `reported`), update
  machine status, notify the assigned technician via push + SSE. Auth: accept either JWT or a
  machine API key (a simple shared secret per machine stored in metadata). `GET
  /machines/:id/telemetry` — recent readings.
- **FE:** Telemetry chart on machine detail (temperature, vibration, pressure over time). Alert
  badge when a metric is out of range. Auto-created incidents appear in the incident list.
- **Mobile:** Telemetry summary card on machine detail (latest readings, trend arrows). Push
  notification on auto-created incident.
- **Link:** machine/sensor -> api (telemetry) -> anomaly detection -> incident creation -> push +
  SSE -> mobile/web.
- **Done-check:** simulate a telemetry push with `error_code: "E-203"` and `temperature: 95`, an
  incident is auto-created, the assigned technician receives a push notification, the machine
  status shows "down" on the dashboard.

For the hackathon demo, telemetry is **simulated**: a seed script or a "Simulate Alert" button in
the web UI fires a telemetry POST as if a machine sensor sent it.

### M-F6 — Mobile Incident Experience

*The technician's primary interface, optimized for field use.*

- **Mobile:** Full incident flow on phone:
  1. Push notification: "CNC Mill #4 — Error E-203, Temperature 95C".
  2. Tap opens incident detail with machine status card (latest metrics, location, last service).
  3. Chat thread with maintenance agent (live via SSE).
  4. Agent's suggested fix steps as a tappable checklist.
  5. "Mark Resolved" with resolution notes or "Escalate" with reason.
  6. Pull-to-refresh, offline read cache for recent incidents.
- **Link:** rides on M-F2, M-F3, M-F5 endpoints + SSE.
- **Done-check:** technician receives push on their phone, opens the incident, chats with the
  agent, follows fix steps, marks resolved — all without touching the web dashboard. The full
  interaction takes under 2 minutes.

### M-F7 — Machine Dashboard + Operational Metrics

*Managers see the fleet at a glance.*

- **BE:** `GET /orgs/:orgId/machines/stats` — total machines, count by status, open incidents,
  average resolution time, machines overdue for service.
- **FE:** Machines tab header with stat cards (operational / degraded / down / maintenance). Machine
  list with sort/filter (status, team, location). Incident history timeline per machine. "Overdue
  for service" highlight.
- **Mobile:** Simplified dashboard with stat cards and machine list.
- **Done-check:** with seeded machines and a mix of statuses, the dashboard shows correct counts,
  the overdue machines are highlighted, and clicking a machine shows its incident history.

### M-F8 — Seeding + Demo Scenario

*Demo-ready from a cold start.*

- **DB/BE:** On org creation (or via `make seed`), seed:
  - 5 machines across 2 teams (IT Infrastructure, Facilities): "CNC Mill #4", "Press Line B",
    "Laser Cutter #1", "Server Rack A", "HVAC Unit 3".
  - 2 technicians assigned to machines.
  - 1 completed incident with full message history (diagnosis + resolution).
  - 1 active incident mid-diagnosis (technician + agent messages, status `diagnosing`).
  - 1 escalated incident linked to a completed request.
  - Telemetry history for each machine (last 7 days simulated).
  - Known error codes in machine metadata: E-203 (coolant flow), E-401 (bearing wear), E-105
    (voltage fluctuation), E-302 (pressure drop).
- **Done-check:** fresh org shows machines with realistic history, the dashboard has non-empty
  stats, the active incident has a readable diagnostic conversation, and the escalated incident
  links to a completed workflow request.

---

## Maintenance Agent — Detail

### Agent Type

```
agent_type: "maintenance"
name:       "Maintenance Diagnostic Agent"
team:       assigned to the machine's team
```

### System Prompt (summary)

You are a maintenance diagnostic agent for an organization's equipment fleet. You receive a
machine's specs, telemetry readings, service history, error codes, and a technician's description
of the problem. Your job:

1. **Diagnose** — identify the likely root cause from the evidence.
2. **Suggest actions** — provide 2-4 concrete, ordered fix steps the technician can follow.
3. **Assess severity** — is this a quick fix or does it need parts/budget/other departments?
4. **Escalate when needed** — if the fix requires parts ordering, budget approval, or
   cross-department coordination, call `suggest_escalation` with a clear reason.
5. **Be conversational** — the technician is on a phone in a noisy environment. Keep messages
   short, actionable, and jargon-appropriate (they know the machine).

### Tools

| Tool | Purpose | Available to |
|------|---------|-------------|
| `get_machine_specs(ctx)` | Machine metadata: type, model, specs, manual references, known error codes | maintenance |
| `get_service_history(ctx)` | Past incidents, last service date, recurring patterns | maintenance |
| `lookup_error_code(ctx, code)` | Error code meaning, common causes, fix procedures | maintenance |
| `suggest_escalation(ctx, reason)` | Flag that this incident needs cross-department help | maintenance |

### Output Model

```python
class SuggestedAction(BaseModel):
    title: str          # "Check coolant reservoir level"
    description: str    # "Open the access panel on the left side..."

class DiagnosisResponse(BaseModel):
    reply: str                              # conversational message to the technician
    suggested_actions: list[SuggestedAction] # ordered fix steps
    diagnosed_cause: str | None             # root cause if determined
    should_escalate: bool                   # does this need cross-department help?
    escalation_reason: str | None           # why, if escalating
```

### Offline Fallback

With no LLM key, the `FunctionModel` returns deterministic responses for known error codes:

| Error Code | Diagnosis | Actions |
|------------|-----------|---------|
| E-203 | Coolant flow obstruction | Check reservoir, inspect pump filter, test flow sensor |
| E-401 | Bearing wear detected | Inspect bearing assembly, check lubrication, recommend replacement if worn |
| E-105 | Voltage fluctuation | Check power supply connections, inspect UPS, test voltage regulator |
| E-302 | Pressure drop | Inspect seals and gaskets, check for leaks, verify pressure sensor calibration |
| (unknown) | Generic: "Inspect the machine for visible damage, check recent maintenance logs" | Visual inspection, check logs, contact team lead |

---

## Dependencies & Sequencing

### What this feature rides on

| Dependency | Why | Status |
|------------|-----|--------|
| F1 (requests) | Escalation creates a request | ✅ done |
| F2 (workflow graph) | Escalated request triggers a workflow | ✅ done |
| F3 (agents run) | Workflow agents process the escalated request | ✅ done |
| F4 (SSE) | Live chat + incident status updates | ⬜ not started |
| F7 (approval) | Escalated requests need executive approval | ⬜ not started |
| M0 (mobile scaffold) | Mobile is the primary technician interface | ⬜ not started |

### Internal dependency map

```
M-F1 (machines + role) ──→ M-F2 (incidents) ──→ M-F3 (agent chat) ──→ M-F4 (escalation)
         │                        │                      │
         │                        └── M-F5 (telemetry)   └── M-F6 (mobile experience)
         │
         └── M-F7 (dashboard)

M-F1..M-F7 ──→ M-F8 (seeding)
```

### Build order

- **Wave 1 — now:** M-F1 (machines + technician role). Can start immediately, no blockers.
- **Wave 2 — after M-F1:** M-F2 (incidents) + M-F7 (dashboard) in parallel.
- **Wave 3 — after M-F2:** M-F3 (agent chat) + M-F5 (telemetry) in parallel.
- **Wave 4 — after M-F3:** M-F4 (escalation, needs F1-F3 done which they are) + M-F6 (mobile,
  needs M0 mobile scaffold).
- **Wave 5 — last:** M-F8 (seeding, needs all above for realistic demo data).

M-F4 (escalation) is fully functional only once F7 (approval) is done in the main PRD, but the
request-creation bridge works immediately since F1-F3 are done. M-F6 (mobile) depends on M0 from
`PRD_MOBILE.md`.

### If working solo

Strict order: **M-F1 -> M-F2 -> M-F3 -> M-F5 -> M-F4 -> M-F7 -> M-F6 -> M-F8.**

---

## User Stories

1. As a technician, I want to see which machines I am assigned to with their current status, so
   that I know what I am responsible for.
2. As a technician, I want to report a machine problem from my phone with a title, description,
   severity, and error code, so that the system starts tracking it immediately.
3. As a technician, I want an AI agent to diagnose the problem and suggest fix steps based on the
   machine's specs and history, so that I can resolve it faster than searching manuals.
4. As a technician, I want to discuss the problem with the agent in a chat thread, so that I can
   refine the diagnosis as I inspect the machine.
5. As a technician, I want to mark an incident as resolved with notes about what I did, so that
   the fix is recorded for future reference.
6. As a technician, I want the agent to escalate automatically when the problem needs parts or
   budget, so that I don't have to fill out a separate request form.
7. As a technician, I want a push notification when a machine I am assigned to has a new incident
   or status change, so that I can respond quickly even when I am not watching the dashboard.
8. As a manager, I want a dashboard showing how many machines are operational, degraded, or down,
   so that I have an at-a-glance view of the equipment fleet.
9. As a manager, I want to see each machine's incident history, so that I can identify recurring
   problems and plan preventive maintenance.
10. As the system, I want to auto-create an incident when a machine pushes telemetry with an error
    code or out-of-range metric, so that problems are caught even when no one is watching.
11. As the system, I want every incident status change, agent message, and escalation logged to the
    audit trail, so that the maintenance record is complete and traceable.
12. As a viewer, I want to see that an escalated incident is linked to its workflow request, so
    that I can trace the full resolution path from machine alarm to budget approval to fix.

## Implementation Decisions

1. **Machines belong to teams.** A machine's `team_id` determines which team (and which
   technicians) are responsible. This reuses the existing teams infrastructure.

2. **Technician is a team-level role.** No schema change needed — `team_members.role` is already
   TEXT. The app validates `technician` as a valid value alongside `lead` and `member`.

3. **The maintenance agent is stateless per call.** Each `/agents/diagnose` call receives the full
   context (machine, incident, message history, telemetry). The agent does not maintain session
   state — the message history in `incident_messages` is the memory.

4. **Escalation reuses the existing request pipeline.** `POST /incidents/:id/escalate` calls the
   same request-creation logic as F1, linking the incident. No new workflow engine work needed.

5. **Telemetry is simulated for the hackathon.** No real IoT integration. A seed script and a
   "Simulate Alert" button in the UI fire fake telemetry. The ingestion endpoint is real and
   production-ready, but the data source is simulated.

6. **Machine API key auth is simple.** For telemetry ingestion, a shared secret stored in
   `machines.metadata.api_key` is checked against an `Authorization: Bearer <key>` header. This
   is not production-grade (no rotation, no per-machine scoping) but is sufficient for the MVP
   demo.

7. **SSE reuses the existing bus pattern.** Incident events are published to the same in-process
   pub/sub from F4, keyed by incident ID. The `GET /incidents/:id/events` endpoint follows the
   same pattern as `GET /requests/:id/events`.

## Testing Decisions

1. **Maintenance agent (Python)** — `TestModel`/`FunctionModel`: known error code returns correct
   diagnosis and actions; unknown code returns generic guidance; `suggest_escalation` tool fires
   when context indicates need for parts/budget; offline fallback produces valid
   `DiagnosisResponse`.

2. **Incident lifecycle (Go)** — table-driven tests: create incident -> machine status changes;
   resolve -> machine goes operational; escalate -> request created and linked; request completion
   callback -> incident resolved.

3. **Telemetry anomaly detection (Go)** — telemetry with error code auto-creates incident;
   telemetry within thresholds does not; duplicate error codes within a window do not create
   duplicate incidents.

4. **Escalation bridge (Go)** — escalating an incident creates a request with the correct title
   and link; completing the linked request resolves the incident.

## Out of Scope (MVP)

- Real IoT/MQTT integration. Telemetry endpoint exists but data is simulated.
- Photo/video attachments on incidents (valuable but adds complexity).
- Predictive maintenance / ML anomaly detection. Thresholds are static.
- Spare parts inventory tracking. The escalation mentions "needs parts" but does not track
  inventory.
- Scheduled maintenance calendar. `next_service_due` exists on the machine but there is no
  scheduling UI.
- Multi-site / facility-level grouping. Machines have a `location` text field but no facility
  hierarchy.
- Offline-first mobile with queued actions. Read cache only for MVP.

## Demo Golden Path

The demo for judges walks through:

1. **Dashboard** — Machines tab shows 5 machines: 3 operational, 1 degraded, 1 down.
2. **Auto-alert** — "Simulate Alert" fires telemetry for CNC Mill #4 with E-203. Machine goes
   "down", incident auto-created, technician gets push notification.
3. **Diagnosis chat** — technician opens incident on phone, agent diagnoses "coolant flow
   obstruction", suggests 3 fix steps. Technician replies "filter was clogged, cleaned it". Agent
   responds "monitor temperature, if it drops below 70C mark resolved".
4. **Escalation** — technician reports pump noise. Agent determines pump replacement needed,
   suggests escalation. Technician taps "Escalate". Request "CNC Mill #4 — Pump Replacement"
   created, visible on the workflow canvas.
5. **Full cycle** — the escalated request flows through Finance (budget review), Operations (parts
   ordering), Executive Approval, Implementation. When the request completes, the incident
   auto-resolves and CNC Mill #4 goes back to "operational".
6. **Audit trail** — every step is traceable: incident creation, each agent message, escalation,
   request workflow, resolution.

This demonstrates: realistic workflow, clear separation (technician + maintenance agent + department
agents), meaningful decision-making (diagnosis, escalation judgment), genuine collaboration
(agent chat + cross-department escalation), and full traceability.
