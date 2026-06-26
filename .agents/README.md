# .agents/ — Agent Alignment Directory

This directory contains everything an AI agent (Claude, Codex, Copilot,
Gemini, or a human developer) needs to work on this project and stay
aligned with the MVP goal.

## Files

| File | Purpose |
|---|---|
| `CONTEXT.md` | Project identity, what's built, what's next, key decisions. Start here. |
| `MVP-SPEC.md` | Exact specification of what the MVP demo must look like and do. |
| `PRD.md` | **Umbrella PRD** for the AI Organization OS, with the live **feature status tracker** (built vs not). Start here for status. |
| `PRD_MOBILE.md` | PRD for the **Flutter mobile app** (separate repo). Client of this backend's contracts; lists the few backend additions mobile needs here. |
| `PRD-BACKEND.md` | **PRD** for the Go team: schema, HTTP + SSE contract, orchestration engine, seeding. Owns the shared contract. |
| `PRD-AGENT.md` | **PRD** for the Agent team: the Pydantic AI agent layer, tools, agent-declared dependencies. |
| `PRD-FRONTEND.md` | **PRD** for the Frontend team: shell nav, the 3-panel live canvas, SSE client, design. |
| `BACKEND-GUIDE.md` | Go API patterns, database schema, handler conventions. |
| `FRONTEND-GUIDE.md` | React patterns, component structure, design system rules. |
| `AGENT-GUIDE.md` | Python agent service patterns, department agent logic. |
| `skills/` | Vendored engineering + design skills (frontend / backend / agent). Read the relevant `SKILL.md` before working. See `skills/README.md`. |
| `build-feature.md` | Agent-agnostic procedure for shipping one PRD feature end-to-end. Invokable as `/build-feature <id>` in Claude (`.claude/commands/`) and OpenCode (`.opencode/command/`). |

## Work split (PRDs)

Three teams build in parallel against one contract. `PRD-BACKEND.md` defines the HTTP + SSE + agent
contracts; `PRD-FRONTEND.md` and `PRD-AGENT.md` consume them, so the teams don't block each other.

## How to Use

1. **Starting work?** Read `CONTEXT.md` first — 2 minutes to full alignment.
2. **Building a feature?** Read the relevant guide (backend/frontend/agent).
3. **Unsure about design?** Check `MVP-SPEC.md` for the screenshot breakdown.
4. **Need the full picture?** Read `../FEATURES.md` for all 10 features.

## Rules

- Every feature is a **vertical slice** — own the logic, data, and UI.
- The UI follows `../DESIGN.md` — Stripe-inspired, sohne-var, navy/purple.
- Every state change gets an **audit event** — traceability is not optional.
- Match the **screenshot** in `../mvp.png` — that's the target.
