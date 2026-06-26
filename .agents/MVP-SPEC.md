# MVP Specification — What the Demo Must Look Like

This file breaks down the target screenshot (`../mvp.png`) into exact
UI requirements. Every component described here must exist and function
in the final demo.

## The Demo Scenario

**Request:** "Open a New Office in Berlin"
**Requester:** Ben Johnson (or similar)
**Priority:** High
**Date:** Jun 25, 2025

The workflow progresses through 9 stages. When the demo runs, agents
process the request in real time and the canvas updates live.

---

## Screenshot Breakdown

### 1. Shell Rail (far left, dark sidebar)

A narrow vertical navigation bar on the far left:

```
[Logo] AI Organization OS

Home
My Work
Requests
Workflows        ← active (highlighted)
Agents
Integrations

─── TEAMS ───
Finance Team
IT Team
HR Team
Operations Team
```

- Dark background (#1c1e54 or similar dark navy)
- White text, active item has purple/blue highlight
- Teams section at bottom with colored dots per team
- Logo at top: "AI Organization OS" text

### 2. Left Panel — Request Overview

A card below the top bar, left side of the canvas area:

```
┌────────────────────────────┐
│ Open a new office          │
│ in Berlin                  │
│                            │
│ Request ID: REQ-2025-042   │
│ Requester: Ben Johnson     │
│ Created: Jun 25, 2025      │
│                            │
│ Priority: ● High           │
│ Status: ● In Progress      │
│                            │
│ ████████░░ 5 of 9 steps    │
│                            │
│ Est. completion:           │
│ Jul 15, 2025               │
│                            │
│ Current Status:            │
│ ● Finance review in        │
│   progress. Waiting for    │
│   IT assessment data.      │
└────────────────────────────┘
```

Below the Request Overview card:

```
┌────────────────────────────┐
│ PARTICIPATING AGENTS       │
│                            │
│ 🟢 Intake Processor       │
│ 🟢 Planning Analyst       │
│ 🔵 Finance Reviewer       │
│ 🔵 Legal Reviewer         │
│ 🟡 IT Manager             │
│ ⚪ HR Manager             │
│ ⚪ Operations Manager     │
│ ⚪ Executive Approver     │
│                            │
│ [View All Agents →]        │
└────────────────────────────┘
```

Agent status dots:
- 🟢 Green = Completed
- 🔵 Blue = In Progress
- 🟡 Yellow = In Progress (partial)
- ⚪ Gray = Pending

### 3. Top Bar

Spans full width above the canvas:

```
← Workflow: Open New Office in Berlin    [In Progress]    [Share] [⋯] [Avatar]
Request ID: REQ-2025-042 · Priority: High
```

- Back arrow or breadcrumb
- Workflow title as heading
- Status badge (In Progress — green/blue)
- Priority badge (High — colored)
- Share button, options menu, user avatar

### 4. Center Canvas — Workflow Graph

The main visual. A directed graph with boxes and arrows:

```
                    ┌──────────────┐
                    │ Request      │
                    │ Intake       │ ✅ Completed
                    └──────┬───────┘
                           │
                    ┌──────┴───────┐
                    │ Planning &   │
                    │ Analysis     │ ✅ Completed
                    └──────┬───────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
       ┌──────┴───┐ ┌─────┴────┐ ┌────┴──────┐
       │ Finance  │ │ Legal    │ │ IT        │
       │ Review   │ │ Review   │ │ Assessment│
       │ 🔵 65%  │ │ 🔵 40%  │ │ 🟡 80%   │
       └──────┬───┘ └─────┬────┘ └────┬──────┘
              │            │            │
              └────────────┼────────────┘
                           │ (dashed — merge point)
              ┌────────────┼────────────┐
              │            │            │
       ┌──────┴───┐ ┌─────┴────────────┘
       │ HR       │ │
       │ Planning │ │
       │ ⚪      │ │
       └──────┬───┘ │
              │     │
              │  ┌──┴──────────┐
              │  │ Operations  │
              │  │ Planning    │
              │  │ ⚪          │
              │  └──────┬──────┘
              │         │
              └────┬────┘
                   │
            ┌──────┴───────┐
            │ Executive    │
            │ Approval     │ ⚪ Pending
            └──────┬───────┘
                   │
            ┌──────┴───────┐
            │Implementation│ ⚪ Pending
            └──────┬───────┘
                   │
            ┌──────┴───────┐
            │ Review &     │
            │ Report       │ ⚪ Pending
            └──────────────┘
```

**Node styling:**
- Each node is a rounded rectangle (4-6px radius)
- Left colored border or icon indicating status
- Node name as title
- Agent name or description below
- Progress indicator if in progress
- Avatar/icon for the assigned agent

**Edge styling:**
- Solid arrows: sequential flow
- Dashed arrows: parallel merge (converging after branches)
- Arrow direction: top to bottom, left to right for parallel

**Legend** (bottom-left of canvas):
```
● Completed   ● In Progress   ● Pending   ● Blocked
```

**Controls** (bottom or top of canvas):
- Zoom in / Zoom out
- Fit to view
- Minimap (optional)

### 5. Right Panel — Node Detail

Opens when a node is clicked. Example: "Finance Review"

```
┌────────────────────────────────────┐
│ Finance Review                 [X] │
│                                    │
│ [Overview] [Tasks] [Activity]      │
│ ─────────────────────────────────  │
│                                    │
│ Description                        │
│ Comprehensive financial review     │
│ of the Berlin office proposal      │
│ including budget analysis and      │
│ ROI projections.                   │
│                                    │
│ Agent                              │
│ 🟦 Finance Reviewer Agent         │
│                                    │
│ Progress                           │
│ ████████░░░░░ 65%                  │
│                                    │
│ Tasks                              │
│ ✅ Budget Feasibility Check        │
│    Completed · Jun 26, 10:30 AM    │
│                                    │
│ 🔵 Financial Impact Analysis      │
│    In Progress · Jun 26, 11:15 AM  │
│                                    │
│ ⚪ ROI Projection Review          │
│    Pending                         │
│                                    │
│ Latest Update                      │
│ "Financial impact analysis is in   │
│  progress. Waiting for data from   │
│  IT assessment."                   │
│                                    │
│ ── Activity Tab ──                 │
│ 11:15 AM  Finance Reviewer         │
│   Started financial impact         │
│   analysis                         │
│                                    │
│ 10:30 AM  Finance Reviewer         │
│   Completed budget feasibility     │
│   check — budget within range      │
│                                    │
│ 10:00 AM  System                   │
│   Finance Review node activated    │
└────────────────────────────────────┘
```

**Tabs:**
- **Overview:** Description, agent, progress, task list, latest update
- **Tasks:** Expanded task list with details (same as overview but more detail)
- **Activity:** Chronological audit events for this node

---

## Status Color Mapping

| Status | Node Color | Badge Color | Dot Color |
|--------|-----------|-------------|-----------|
| Completed | Green border/bg | Green badge | 🟢 |
| In Progress | Blue border/bg | Blue badge | 🔵 |
| Pending | Gray border/bg | Gray badge | ⚪ |
| Blocked | Red/orange border/bg | Red badge | 🔴 |

Use the design system colors from DESIGN.md:
- Completed green: `#15be53` (success)
- In Progress blue: `#533afd` (primary purple) or a blue
- Pending gray: `#64748d` (slate) or `#e5edf5` (border)
- Blocked red: `#ea2261` (ruby)

---

## Workflow Nodes (Exact List)

| # | Node Name | Agent | Comes After | Parallel? |
|---|-----------|-------|-------------|-----------|
| 1 | Request Intake | Intake Processor | — | No |
| 2 | Planning & Analysis | Planning Analyst | 1 | No |
| 3 | Finance Review | Finance Reviewer | 2 | Yes (with 4, 5) |
| 4 | Legal Review | Legal Reviewer | 2 | Yes (with 3, 5) |
| 5 | IT Assessment | IT Manager | 2 | Yes (with 3, 4) |
| 6 | HR Planning | HR Manager | 3, 4, 5 (merge) | Yes (with 7) |
| 7 | Operations Planning | Operations Manager | 3, 4, 5 (merge) | Yes (with 6) |
| 8 | Executive Approval | Executive Approver | 6, 7 (merge) | No |
| 9 | Implementation | — | 8 | No |
| 10 | Review & Report | — | 9 | No |

**Note:** The exact graph shape may vary slightly — the important thing is
that parallel branches exist, merge points exist, and the graph matches the
screenshot layout.

---

## Agent Task Lists

### Finance Reviewer
1. Budget Feasibility Check
2. Financial Impact Analysis
3. ROI Projection Review

### Legal Reviewer
1. Regulatory Compliance Check
2. Contract Requirements Review
3. Risk Assessment

### IT Manager
1. Technical Infrastructure Assessment
2. Security Requirements Review
3. Systems Integration Analysis

### HR Manager
1. Staffing Requirements Analysis
2. Hiring Plan Development
3. Policy Compliance Check

### Operations Manager
1. Logistics Planning
2. Facilities Assessment
3. Operational Timeline

### Executive Approver
1. Review All Department Findings
2. Risk-Benefit Analysis
3. Final Decision & Justification

---

## Cross-Agent Dependencies (Required for Demo)

At minimum, one visible dependency must exist:

**Finance Review depends on IT Assessment:**
- Finance's "Financial Impact Analysis" task requires IT's infrastructure
  cost data.
- While IT Assessment is incomplete, Finance's latest update reads:
  *"Financial impact analysis is in progress. Waiting for data from
  IT assessment."*
- When IT Assessment completes, Finance automatically unblocks and
  continues its analysis.
- This single dependency proves the entire multi-agent collaboration
  concept for the hackathon judges.

---

## Sidebar Tab Specifications

Every tab in the shell rail must show meaningful content.

### Home Tab (F11)
```
┌──────────────────────────────────────────────┐
│ Welcome back, [User Name]                    │
│                                              │
│ ┌─────────┐ ┌─────────┐ ┌─────────┐        │
│ │ 3       │ │ 1       │ │ 85%     │        │
│ │ Active  │ │ Pending │ │ Compl.  │        │
│ │ Requests│ │ Approval│ │ Rate    │        │
│ └─────────┘ └─────────┘ └─────────┘        │
│                                              │
│ Recent Requests                              │
│ ┌────────────────────────────────────────┐   │
│ │ ● Open New Office in Berlin   In Prog │   │
│ │ ● Vendor Onboarding Review   Complete │   │
│ │ ● Q3 Budget Planning       Submitted │   │
│ └────────────────────────────────────────┘   │
│                                              │
│ Recent Activity                              │
│ 11:15  Finance Reviewer started impact...   │
│ 10:30  IT Manager completed security...     │
│ 10:00  Legal Reviewer flagged compliance... │
│                                              │
│ [+ New Request]                              │
└──────────────────────────────────────────────┘
```

### My Work Tab (F12)
```
┌──────────────────────────────────────────────┐
│ My Work                         [Filter ▾]   │
│                                              │
│ Assigned to You (3)                          │
│ ┌────────────────────────────────────────┐   │
│ │ 🔵 Financial Impact Analysis          │   │
│ │    Finance Review · In Progress       │   │
│ │    ⚠️ Waiting for IT Assessment       │   │
│ ├────────────────────────────────────────┤   │
│ │ ⚪ ROI Projection Review              │   │
│ │    Finance Review · Pending           │   │
│ ├────────────────────────────────────────┤   │
│ │ ⚪ Executive Approval                 │   │
│ │    Pending · Waiting for reviews      │   │
│ └────────────────────────────────────────┘   │
│                                              │
│ Recently Completed (2)                       │
│ ┌────────────────────────────────────────┐   │
│ │ ✅ Budget Feasibility Check           │   │
│ │    Completed · Jun 26, 10:30 AM       │   │
│ └────────────────────────────────────────┘   │
└──────────────────────────────────────────────┘
```

### Requests Tab (F13)
```
┌──────────────────────────────────────────────┐
│ Requests                    [+ New Request]  │
│                                              │
│ [All] [In Progress] [Completed] [Submitted]  │
│                                              │
│ ┌──┬──────────────────┬────────┬──────┬────┐│
│ │  │ Title            │Priority│Status│Prog││
│ ├──┼──────────────────┼────────┼──────┼────┤│
│ │🔵│ Open New Office  │ High   │In Pr.│ 5/9││
│ │  │ in Berlin        │        │      │    ││
│ ├──┼──────────────────┼────────┼──────┼────┤│
│ │✅│ Vendor Onboarding│ Medium │Compl.│9/9 ││
│ │  │ Review           │        │      │    ││
│ ├──┼──────────────────┼────────┼──────┼────┤│
│ │⚪│ Q3 Budget        │ Low    │Subm. │ 0/0││
│ │  │ Planning         │        │      │    ││
│ └──┴──────────────────┴────────┴──────┴────┘│
└──────────────────────────────────────────────┘
```

### Agents Tab (F14)
```
┌──────────────────────────────────────────────┐
│ Agents                          8 total      │
│                                              │
│ Finance Team                                 │
│ ┌────────────────────────────────────────┐   │
│ │ 🟦 Finance Reviewer                   │   │
│ │ Budget analysis, ROI, financial impact │   │
│ │ Status: In Progress · 2 tasks done     │   │
│ └────────────────────────────────────────┘   │
│                                              │
│ Legal Team                                   │
│ ┌────────────────────────────────────────┐   │
│ │ 🟪 Legal Reviewer                     │   │
│ │ Compliance, regulatory, contracts      │   │
│ │ Status: In Progress · 1 task done      │   │
│ └────────────────────────────────────────┘   │
│                                              │
│ IT Team                                      │
│ ┌────────────────────────────────────────┐   │
│ │ 🟩 IT Manager                         │   │
│ │ Technical feasibility, security        │   │
│ │ Status: In Progress · 2 tasks done     │   │
│ └────────────────────────────────────────┘   │
│ ...                                          │
└──────────────────────────────────────────────┘
```

### Reports Tab (F15)
```
┌──────────────────────────────────────────────┐
│ Reports                                      │
│                                              │
│ [Reports] [Audit Trail]                      │
│                                              │
│ Completed Reports                            │
│ ┌────────────────────────────────────────┐   │
│ │ Vendor Onboarding Review              │   │
│ │ Completed Jun 24 · 8 agents · 4 days  │   │
│ │ [View Report]                          │   │
│ └────────────────────────────────────────┘   │
│                                              │
│ Audit Trail                                  │
│ ┌────────────────────────────────────────┐   │
│ │ 11:15  Finance Reviewer               │   │
│ │   Started financial impact analysis    │   │
│ │   Reason: Budget check passed          │   │
│ ├────────────────────────────────────────┤   │
│ │ 10:30  IT Manager                      │   │
│ │   Completed security review            │   │
│ │   Reason: No critical vulnerabilities  │   │
│ ├────────────────────────────────────────┤   │
│ │ 10:00  System                          │   │
│ │   Finance Review node activated        │   │
│ └────────────────────────────────────────┘   │
│                                              │
│ [Filter: Agent ▾] [Date ▾] [Action ▾]       │
└──────────────────────────────────────────────┘
```

### Integrations Tab (F16)
```
┌──────────────────────────────────────────────┐
│ Integrations                                 │
│                                              │
│ Connected (3)                                │
│ ┌──────────┐ ┌──────────┐ ┌──────────┐     │
│ │ 🏦       │ │ ⚖️       │ │ 🖥️       │     │
│ │ SAP      │ │ Legal    │ │ AWS      │     │
│ │ Finance  │ │ Comply   │ │ Infra    │     │
│ │ ●Active  │ │ ●Active  │ │ ●Active  │     │
│ │          │ │          │ │          │     │
│ │ Used by: │ │ Used by: │ │ Used by: │     │
│ │ Finance  │ │ Legal    │ │ IT       │     │
│ │ Reviewer │ │ Reviewer │ │ Manager  │     │
│ └──────────┘ └──────────┘ └──────────┘     │
│                                              │
│ Available (3)                                │
│ ┌──────────┐ ┌──────────┐ ┌──────────┐     │
│ │ Workday  │ │ Slack    │ │ Jira     │     │
│ │ HR       │ │ Comms    │ │ Projects │     │
│ │ ○ Avail. │ │ ○ Avail. │ │ ○ Avail. │     │
│ └──────────┘ └──────────┘ └──────────┘     │
└──────────────────────────────────────────────┘
```
