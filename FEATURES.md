# FEATURES.md — AI Organization OS (Multi-Agent Workflow MVP)

## Goal

Recreate the reference screenshot as a **working** product: a real multi-agent
backend driving a real UI. Not a mockup. When all features below are done,
we should be able to submit one request, watch it move through Intake →
Planning → parallel Reviews → parallel Planning → Approval → Implementation →
Report, with every status, agent, and decision visible and clickable —
exactly like the screenshot.

Each feature below is a **vertical slice**: whoever takes it owns the logic
behind it, the data it produces, and the part of the screen that displays it.

---

## Hackathon Judging Criteria (every feature targets these)

1. **Realistic workflow** — matches how a real organization processes requests
2. **Clear separation of responsibilities** — each agent owns a distinct department
3. **Meaningful decision-making** — validations, approvals, risk flagging with reasoning
4. **Genuine collaboration and dependencies** — agents wait for each other, share data
5. **Traceability** — every decision logged: who, what, why, when

---

## Sidebar Navigation — What Each Tab Shows

The app has 6 main navigation tabs plus a Teams section. Every tab must
be functional and contribute to the demo story.

| Tab | What It Shows | Key Features |
|-----|--------------|-------------|
| **Home** | Organization dashboard | Recent requests, agent activity summary, quick stats |
| **My Work** | Personal task inbox | Tasks assigned to current user/agent, pending approvals |
| **Requests** | All requests list | Create new, filter by status/priority, click to open workflow |
| **Workflows** | The workflow canvas (main view) | Graph, node detail, request overview, agents panel |
| **Agents** | Agent roster & details | All agents, their departments, status, capabilities |
| **Reports** | Generated reports & audit trail | Completed workflow reports, organization-wide audit log |
| **Integrations** | Connected systems | What data sources/systems agents use for decisions |

The **TEAMS** section in the sidebar shows department teams (Finance, IT,
HR, Operations) with member counts. Clicking a team filters the view.

---

## F1 — Request Intake & Workflow Plan

**What it is:** the starting point. Someone submits a request ("Open a new
office in Berlin"), and the system turns it into a structured workflow with
a defined sequence of stages.

**What it must do:**
- Accept a request: title, description, requester, priority.
- An Intake Agent reads the request and decides the workflow plan — which
  stages are needed, in what order, which run in parallel (this is the
  first real decision point in the system, not a hardcoded template).
- Produce a `Request` object with: ID, status, progress (`X of Y steps
  completed`), estimated completion, current status line.

**Maps to screenshot:** the "REQUEST OVERVIEW" panel (top-left) — title,
requester, date, progress bar, estimated completion, current status badge.

**Maps to tabs:**
- **Requests tab** — the new request appears in the list with status.
- **Home tab** — the request shows in "Recent Requests."

**Depends on:** nothing. **Feeds:** F2.

---

## F2 — Workflow Graph Engine

**What it is:** the backbone that holds the actual graph (nodes + edges) and
tracks state for every stage: `Completed / In Progress / Pending / Blocked`.

**What it must do:**
- Represent stages as nodes with sequential edges (Intake → Planning),
  parallel branches (Planning → Finance + Legal + IT at once), and merge
  points (all three → next stage only when all are done or resolved).
- Each node has: name, owning agent, status, timestamps, short live
  description (e.g. "2/3 tasks").
- Expose a simple way for any agent to update its own node's status, and
  for the engine to unblock the next node(s) once dependencies clear.
- This is the single source of truth every other feature reads from.

**Maps to screenshot:** the whole center canvas — the boxes, the arrows
(solid = sequential, dashed = parallel merge), the legend (Completed /
In Progress / Pending / Blocked), the "Fit View / Zoom" controls.

**Depends on:** F1. **Feeds:** F3, F4, F5, F6, F7, F8 (everything reads this).

---

## F3 — Department Agents (Finance / Legal / IT)

**What it is:** the first wave of specialized agents that run in parallel
right after Planning. Each owns its own domain and its own checklist.

**What it must do:**
- Each agent (Finance Reviewer, Legal Reviewer, IT Manager) has its own
  task list (e.g. Finance: "Budget Feasibility Check", "Financial Impact
  Analysis", "ROI Projection Review") with individual statuses.
- Each agent applies real decision logic to move from task to task and to
  reach an end state (approve / flag / blocked) — not just a timer.
- Each agent writes a short plain-language status update when something
  changes (e.g. "Financial impact analysis is in progress. Waiting for
  data from IT assessment.") — this is what makes the system explainable,
  not just a colored dot.

**Maps to screenshot:** the Finance Review / Legal Review / IT Assessment
boxes, and the right-hand detail panel (Overview tab: Description, Agent,
Progress %, Tasks list with per-task status + timestamp, "Latest Update").

**Maps to tabs:**
- **My Work tab** — tasks from these agents appear as work items.
- **Agents tab** — each agent listed with current status and department.

**Depends on:** F2. **Feeds:** F4.

---

## F4 — Cross-Agent Dependencies & Handoffs

**What it is:** the part that makes this an actual multi-agent system and
not three agents working alone. Proves real collaboration.

**What it must do:**
- Let one agent declare it needs something from another before it can
  finish (Finance needs IT's output before completing its impact analysis
  — this is explicitly what's shown in the screenshot's "Latest Update").
- Model that need as a real, visible dependency: requesting agent stays
  "In Progress" with a stated reason, not silently stuck.
- Once the needed agent completes, automatically unblock the waiting agent.
- Define what happens if agents disagree (e.g. Legal completes clean but
  Finance flags a risk) — does the workflow still proceed, pause, or
  escalate to F5?

**Maps to screenshot:** the dashed connector lines between parallel nodes,
and the literal "Waiting for data from IT assessment" line in the panel —
this single sentence is the proof-of-concept for the whole hackathon theme.

**Maps to tabs:**
- **My Work tab** — blocked tasks show "Waiting for [agent]" indicator.
- **Agents tab** — dependency relationships visible in agent detail view.

**Depends on:** F3. **Feeds:** F2 (unblocks nodes), F6.

---

## F5 — Executive Approval

**What it is:** the convergence/decision gate. Nothing executes until this
stage approves it.

**What it must do:**
- Only becomes active once all required upstream branches
  (Reviews + HR/Operations Planning) report complete or resolved.
- Applies decision logic: approve outright if everything is clean,
  or hold/escalate if any upstream agent flagged a concern.
- Produces a clear, written outcome (approve/reject + short justification),
  not just a checkmark.
- On reject: defines where the workflow goes next (back to a specific
  agent, or halted) — must be a real branch, not a dead end.

**Maps to screenshot:** the "Executive Approval" node, and the Executive
Approver entry in the participating-agents list.

**Maps to tabs:**
- **My Work tab** — approval requests appear as actionable items.

**Depends on:** F2, F3, F4. **Feeds:** F6, F7.

---

## F6 — Audit Trail (Traceability Layer)

**What it is:** the system-wide, append-only log every other feature writes
to. This is the feature judges will scrutinize most directly, since
"transparency and traceability" is an explicit grading criterion.

**What it must do:**
- Every status change, agent decision, handoff, and approval gets logged
  with: timestamp, actor (which agent), action, and a one-line reason.
- Support reconstructing "why is this node in this state" by reading the
  log for that node — not by inferring it from the current snapshot.
- Power three views: a chronological Activity feed (per node), a full
  Audit Trail (whole workflow), and the Reports tab audit log.

**Maps to screenshot:** the "Activity" tab in the side panel.

**Maps to tabs:**
- **Reports tab** — the full audit trail is browsable organization-wide.

**Depends on:** F2 (only useful once there are events to log).
**Feeds:** F8 (UI renders this directly), F10 (report reads this).

---

## F7 — Execution Stage (HR / Operations / Implementation)

**What it is:** what happens after approval — proves the workflow ends in
real action, not just a stack of green checkmarks.

**What it must do:**
- Two more agents (HR Manager, Operations Manager) pick up approved work
  in parallel (HR Planning, Operations Planning), each with their own
  sub-tasks, same pattern as F3.
- Both must complete before a final "Implementation" node can start.
- Implementation has its own simple completion criteria (e.g. all sub-tasks
  done) before moving to reporting.

**Maps to screenshot:** the HR Planning / Operations Planning /
Implementation nodes, and their entries in the participating-agents list.

**Depends on:** F5. **Feeds:** F8, F10.

---

## F8 — Workflow Canvas UI

**What it is:** the visual core of the product — the graph, live, clickable.
This is the **Workflows tab** content.

**What it must do:**
- Render F2's nodes and edges as the boxes-and-arrows canvas, color-coded
  by status, matching the legend exactly.
- Clicking a node opens the right-side panel with tabs: **Overview**
  (description, agent, progress, tasks, latest update — from F3),
  **Tasks**, **Details**, **Activity** (from F6).
- Top bar: workflow title, request ID, priority badge, status badge.
- Left request-overview card (from F1): progress %, steps completed,
  estimated completion, current status.
- Left participating-agents panel (from F9): agent list with live status.

**Maps to screenshot:** literally the entire main canvas + right panel.
This is the feature that makes the whole thing demoable.

**Depends on:** F1, F2, F3, F6 (reads all of them; build with mocked data
in parallel from day one, then wire to real state as it becomes available).

---

## F9 — Agent Roster Panel

**What it is:** the simple list proving this is a multi-agent org, not one
model wearing different hats.

**What it must do:**
- List every agent participating in the current workflow with live status
  (In Progress / Completed / Pending), pulled straight from F2/F3/F7.
- Group label by team if time allows (Finance Team, Legal Team, IT Team,
  HR Team, Operations Team) — matches the left sidebar in the screenshot.

**Maps to screenshot:** "PARTICIPATING AGENTS" list in the left panel,
and the "TEAMS" section in the far-left navigation.

**Maps to tabs:**
- **Agents tab** — full page view of all agents with details.
- **Workflows tab** — compact roster in the left panel.

**Depends on:** F2, F3, F7 (read-only).

---

## F10 — Final Report

**What it is:** closes the loop. The one artifact that proves the whole
pipeline produced something, not just a sequence of statuses.

**What it must do:**
- Once Implementation completes, auto-generate a short summary: what was
  requested, who approved it, what was flagged along the way (pulled from
  F6), what was executed, total time taken.
- Doesn't need to be fancy — a generated text/markdown block tied to the
  "Review & Report" node is enough for MVP.

**Maps to screenshot:** the "Review & Report" node (last step in the graph).

**Maps to tabs:**
- **Reports tab** — completed reports listed with timestamps.

**Depends on:** F7, F6.

---

## F11 — Home Dashboard

**What it is:** the landing page after login. Quick organizational overview
that shows the system is alive and active.

**What it must do:**
- Show recent/active requests with status badges (submitted, in progress,
  completed).
- Show agent activity summary: how many agents active, how many tasks
  completed today, current workload.
- Quick stats: total requests, completion rate, average processing time.
- Quick actions: "New Request" button, link to active workflows.
- Recent audit events (last 5–10) as a live activity feed.

**Maps to tabs:** this IS the **Home** tab content.

**Depends on:** F1 (requests), F2 (workflow state), F6 (audit events).

---

## F12 — My Work (Personal Inbox)

**What it is:** the task inbox for the logged-in user. Shows what needs
their attention right now.

**What it must do:**
- List tasks assigned to the current user or their department.
- Show pending approvals (Executive Approval waiting for decision).
- Show blocked items with clear "Waiting for [agent]" indicators.
- Show recently completed items.
- Each item links to the relevant workflow node.

**Maps to tabs:** this IS the **My Work** tab content.

**Depends on:** F2, F3, F5 (reads node/task state).

---

## F13 — Requests List

**What it is:** the full request management view. Browse, filter, create.

**What it must do:**
- List all requests in the organization with: title, requester, priority,
  status, progress, created date.
- Filter by status (submitted/in progress/completed) and priority.
- "New Request" button opens the submission form.
- Click a request to navigate to its workflow canvas.
- Show request count and status breakdown.

**Maps to tabs:** this IS the **Requests** tab content.

**Depends on:** F1 (request data).

---

## F14 — Agents Management Page

**What it is:** the full agent roster with details and capabilities.

**What it must do:**
- List all AI agents in the organization with: name, department/team,
  avatar, current status, description of capabilities.
- Show what each agent does: its task list template, its decision criteria,
  what it validates.
- Show agent activity: how many tasks completed, current workload.
- Group agents by team (Finance Team, Legal Team, IT Team, etc.).
- Click an agent to see its full detail: description, recent actions,
  current assignments.

**Maps to tabs:** this IS the **Agents** tab content.

**Depends on:** F3, F7 (agent definitions), F9 (roster data).

---

## F15 — Reports Page

**What it is:** the reporting and audit hub. Completed reports and
organization-wide traceability.

**What it must do:**
- List completed workflow reports (from F10) with: request title,
  completion date, summary preview.
- Full audit trail browser: filter by request, agent, date range,
  action type.
- Each audit event shows: timestamp, actor (agent), action, target node,
  reason.
- Export or view a full report for any completed workflow.
- Key metrics: requests processed, average completion time, most active
  agents.

**Maps to tabs:** this IS the **Reports** tab content.

**Depends on:** F6 (audit events), F10 (generated reports).

---

## F16 — Integrations Page

**What it is:** shows what external systems and data sources the agents
connect to for their decision-making.

**What it must do:**
- List available integrations with status (connected / available):
  - Financial systems (SAP, QuickBooks) — used by Finance Reviewer
  - Legal databases (LexisNexis, compliance DBs) — used by Legal Reviewer
  - IT infrastructure (AWS, Azure, internal CMDB) — used by IT Manager
  - HR systems (Workday, BambooHR) — used by HR Manager
  - Communication (Slack, Email) — used for notifications
- Each integration card shows: name, icon, status, which agents use it,
  description.
- For MVP: these can be display-only cards showing the vision. They don't
  need to actually connect to external APIs. The important thing is showing
  that agents would use real data sources in production.

**Maps to tabs:** this IS the **Integrations** tab content.

**Depends on:** nothing (display-only for MVP).

---

## Build Order (suggested, not mandatory)

```
Foundation (done): Auth, Orgs, Teams, Members, Shell

Phase 1 — Backbone:
  F1 → F2 → F6 (start logging from day one)
  F13 (Requests list — thin, just shows F1 data)

Phase 2 — Agents:
  F3 → F4
  F14 (Agents page — reads F3 data)

Phase 3 — Decision gates:
  F5
  F7
  F12 (My Work — reads F3/F5 task data)

Phase 4 — Reporting:
  F10 → F15 (Reports page)

Phase 5 — Polish:
  F11 (Home dashboard — reads everything)
  F16 (Integrations — display-only)

F8 and F9 are the Workflows tab — start them immediately with mocked
data, then swap in real state as F1–F7 land.
```

## Definition of Done (MVP)

**Core workflow (must have):**
- [ ] One request can be submitted and generates a real workflow plan (F1)
- [ ] The graph shows correct branching/merging and live status per node (F2)
- [ ] At least 2 agents run in true parallel with independent task lists (F3)
- [ ] At least one real cross-agent dependency is visible and resolves (F4)
- [ ] Approval only unlocks after dependencies clear, with written decision (F5)
- [ ] Every state change is logged and viewable node by node (F6)
- [ ] Execution stage runs post-approval and completes (F7)
- [ ] The full graph is visible, clickable, and matches live backend state (F8)
- [ ] Agent roster reflects real-time status (F9)
- [ ] A final report is generated automatically at the end (F10)

**All sidebar tabs functional:**
- [ ] Home shows dashboard with recent activity (F11)
- [ ] My Work shows personal task inbox (F12)
- [ ] Requests shows list with create/filter/navigate (F13)
- [ ] Agents shows full roster with details (F14)
- [ ] Reports shows audit trail and completed reports (F15)
- [ ] Integrations shows connected systems (F16)

If all boxes are checked, the demo *is* the reference screenshot —
running live, clickable, and explainable end to end, with every tab
showing meaningful content.
