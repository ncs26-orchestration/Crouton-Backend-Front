# .agents/ — Agent Alignment Directory

This directory contains everything an AI agent (Claude, Codex, Copilot,
Gemini, or a human developer) needs to work on this project and stay
aligned with the MVP goal.

## Files

| File | Purpose |
|---|---|
| `CONTEXT.md` | Project identity, what's built, what's next, key decisions. Start here. |
| `MVP-SPEC.md` | Exact specification of what the MVP demo must look like and do. |
| `BACKEND-GUIDE.md` | Go API patterns, database schema, handler conventions. |
| `FRONTEND-GUIDE.md` | React patterns, component structure, design system rules. |
| `AGENT-GUIDE.md` | Python agent service patterns, department agent logic. |

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
