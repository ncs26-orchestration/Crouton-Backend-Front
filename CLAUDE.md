# AI Organization OS

Multi-agent system simulating how real companies coordinate work internally.
AI agents represent departments (Finance, Legal, IT, HR, Operations) that
collaborate on business requests with structured handoffs and traceable decisions.

## Hackathon Judging Criteria (what wins)

1. **Realistic workflow** — matches how a real organization processes requests
2. **Clear separation of responsibilities** — each agent owns a distinct department
3. **Meaningful decision-making** — agents validate, flag, approve, not just pass-through
4. **Genuine collaboration and dependencies** — agents wait for each other, share data
5. **Traceability** — every decision logged with who, what, why, when

## What to build: FEATURES.md (F1–F10)

## What it looks like: mvp.png

## What's done: PHASES.md (Phase 0 — auth, orgs, teams, members)

## Design system: DESIGN.md (Stripe-inspired)

## Agent alignment: .agents/ directory

## Stack

- `apps/web` — React 19 + Vite + React Flow
- `apps/api` — Go 1.25 + Echo + pgx + JWT
- `apps/agent` — Python 3.13 + FastAPI + LangGraph
- PostgreSQL 18, Redis 8, pnpm + Turborepo + Docker Compose

## Commands

```
make up          # start dev
make migrate-up  # apply migrations
make logs        # tail logs
```
