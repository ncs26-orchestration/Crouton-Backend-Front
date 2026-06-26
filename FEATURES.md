# FEATURES.md — AI Organization OS (Multi-Agent Workflow MVP)

## Goal

Recreate the reference screenshot as a **working** product: a real multi-agent
backend driving a real UI. Not a mockup. When all features below are done,
we should be able to submit one request, watch it move through Intake →
Planning → parallel Reviews → parallel Planning → Approval → Implementation →
Report, with every status, agent, and decision visible and clickable —
exactly like the screenshot.

**Scope rule:** if a feature isn't needed to make the screenshot real and
clickable, it's not in this list. No login system, no notifications backend,
no multi-workflow history, no settings pages. One request, one workflow,
fully alive, fully traceable. That's the MVP.

Each feature below is a **vertical slice**: whoever takes it owns the logic
behind it, the data it produces, and the part of the screen that displays it.

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
- Power two views: a chronological Activity feed (per node) and a full
  Audit Trail (whole workflow).

**Maps to screenshot:** the "Activity" tab in the side panel and the
"Audit Trail" tab in the top nav bar. (Timeline / Conversations / Documents
tabs are nice-to-have, not MVP — skip unless time allows.)

**Depends on:** F2 (only useful once there are events to log).
**Feeds:** F8 (UI renders this directly).

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

**What it must do:**
- Render F2's nodes and edges as the boxes-and-arrows canvas, color-coded
  by status, matching the legend exactly.
- Clicking a node opens the right-side panel with tabs: **Overview**
  (description, agent, progress, tasks, latest update — from F3),
  **Tasks**, **Details**, **Activity** (from F6).
- Top bar: workflow title, request ID, priority badge, status badge.
- Left request-overview card (from F1): progress %, steps completed,
  estimated completion, current status.

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

**Depends on:** F7, F6.

---

## Build Order (suggested, not mandatory)

```
F1 → F2 → F6 (start logging from day one)
         ↓
        F3 → F4
         ↓
        F5
         ↓
        F7
         ↓
        F10

F8 and F9 are read-only views — start them immediately with mocked/sample
data, then swap in real state from F2/F3/F6/F7 as those land. They do not
need to wait for the backend to be finished to start.
```

## Definition of Done (MVP)

- [ ] One request can be submitted and generates a real workflow plan (F1)
- [ ] The graph shows correct branching/merging and live status per node (F2)
- [ ] At least 2 agents run in true parallel with independent task lists (F3)
- [ ] At least one real cross-agent dependency is visible and resolves on
      its own once the blocking agent finishes (F4)
- [ ] Approval stage only unlocks after its real dependencies clear, and
      produces a written decision (F5)
- [ ] Every state change in the run is logged and viewable after the fact,
      node by node (F6)
- [ ] Execution stage runs post-approval and completes (F7)
- [ ] The full graph is visible, clickable, and matches live backend state,
      not a static image (F8)
- [ ] Agent roster reflects real-time status (F9)
- [ ] A final report is generated automatically at the end (F10)

If all ten boxes are checked, the demo *is* the reference screenshot —
running live, clickable, and explainable end to end.
