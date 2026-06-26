# AI Organization OS — Diagrams

Static renders of system diagrams for Markdown viewing.

## Architecture (legacy — from Pablo era)

These diagrams were created for the previous "Pablo" workflow authoring
system. They will be updated as the AI Organization OS features land.

![Architecture](01.webp)

## Data Model (legacy)

![Data model](04.webp)

## Target: Multi-Agent Workflow

The target architecture is described in the root documentation:
- `../../SYSTEM.md` — system definition and data flow
- `../../FEATURES.md` — the 10 MVP features
- `../../.agents/MVP-SPEC.md` — screenshot breakdown

The workflow graph for the "Open New Office in Berlin" demo:

```
Request Intake → Planning & Analysis
                        │
            ┌───────────┼───────────┐
            │           │           │
      Finance Rev   Legal Rev   IT Assessment
            │           │           │
            └───────────┼───────────┘
                        │
            ┌───────────┼───────────┐
            │                       │
      HR Planning          Operations Planning
            │                       │
            └───────────┬───────────┘
                        │
               Executive Approval
                        │
                  Implementation
                        │
                 Review & Report
```
