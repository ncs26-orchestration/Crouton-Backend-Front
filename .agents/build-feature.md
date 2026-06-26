# build-feature — vertical-slice feature delivery procedure

Canonical, agent-agnostic playbook for shipping one feature from `.agents/PRD.md` end-to-end. Any
coding agent (Claude `/build-feature`, Cursor, etc.) should follow this exactly. Invoked with a
feature id, e.g. `build-feature f1` or `build-feature F5`.

## Input

A single feature id (`f0`..`f10`, case-insensitive). If missing or unknown, stop and ask which
feature; do not guess.

## Golden rules (non-negotiable)

1. **One feature = one vertical slice.** Deliver every layer the feature's row in `.agents/PRD.md`
   marks as needed — DB, BE (Go API/engine), AG (Python agent), FE (React), and the **Link** wiring
   that makes them talk. A feature is NOT done until its end-to-end **done-check** passes.
2. **Branch from `develop`.** Never work on `develop` directly. Branch name: `feat/<fid>-<slug>`
   (e.g. `feat/f1-submit-request`).
3. **Commit each piece.** Small, logical commits per layer/step — not one big commit. Never
   `git add -A`; stage explicit paths (a prior incident committed build artifacts that way). No AI
   co-author trailer.
4. **Keep it green.** Build and test locally before pushing. Open a PR to `develop` and drive CI to
   **green** — lint, typecheck, unit tests (race for Go), the migration up/rollback/re-apply check,
   and the docker-compose e2e smoke. If CI is red, fix and push until green. Do not stop on red.
5. **End-to-end, with no keys.** The feature must work with all LLM keys unset (deterministic /
   `FunctionModel` fallbacks). The e2e job must exercise the feature's path.
6. **Update the tracker.** Flip the feature's layer cells and Overall column in `.agents/PRD.md`
   (⬜→🟡 while building, →✅ when merged + wired + done-check passes), in the same branch/PR.
7. **Do not auto-merge.** Leave the merge to a human once CI is green, unless explicitly told to
   merge. Report the green PR link.

## Procedure

### 0. Plan
- Read the feature's section in `.agents/PRD.md`: its DB/BE/AG/FE/Link tasks, the referenced
  slice-PRD task ids (BE-/AG-/FE-), and the done-check.
- Open the referenced sections in `PRD-BACKEND.md` / `PRD-AGENT.md` / `PRD-FRONTEND.md` for step
  detail and the **shared contracts** (HTTP/SSE shapes, the Go↔Python agent contract). Honor the
  contracts exactly — other slices depend on them.
- Read the relevant vendored skills under `.agents/skills/{backend,agent,frontend}` before writing
  code in that layer (e.g. `i-golang-concurrency` for the engine, `i-pydantic-ai` + `i-agent-tool-builder`
  for agents, `i-react-flow-node-ts` for canvas nodes).
- Reuse existing patterns: pgx repos like `internal/repo/orgs.go`, handler constructors registered in
  `internal/http/server.go`, the agent provider config in `apps/agent/app/settings.py`, the web
  `lib/api.ts` client and design tokens in `index.css`.

### 1. Branch
- Ensure local `develop` is current and green: `git checkout develop && git pull --rebase origin develop`.
- `git checkout -b feat/<fid>-<slug>`.

### 2. Implement, layer by layer, committing each piece
Work in dependency order. Commit after each coherent piece.
- **DB:** create migrations with `dbmate new <name>` (real `YYYYMMDDHHMMSS` UTC prefix, both
  `migrate:up` and `migrate:down`). Apply with `make migrate-up`. Commit.
- **BE (Go):** repos → handlers → engine/wiring; register routes in `internal/http/server.go`.
  Keep `gofmt` clean, `go vet` and `go build ./...` passing; add table-driven tests for deep modules.
  Commit per piece.
- **AG (Python):** Pydantic AI typed models, tools, agents, FastAPI routes in `app/api`; keep `ruff`,
  `ruff format`, and `mypy app` clean; add `TestModel`/`FunctionModel` tests. Commit per piece.
- **FE (React):** types + `lib/api.ts` + SSE client, then views/components. Never commit generated
  output (`apps/web/.output`, `routeTree.gen.*`, compiled `src/**/*.js`). Keep `pnpm --filter web build`
  and `eslint` clean; add vitest tests for pure modules (e.g. the mapper). Commit per piece.
- **Link:** wire the layers per the contracts (web→api, api→agent, SSE browser⇇engine) and verify the
  data actually flows, not just that each layer compiles.

### 3. Verify end-to-end locally
- Build each touched app; run its tests. Run `make up` (or the relevant compose slice) and walk the
  feature's done-check by hand or via `scripts/e2e-smoke.sh` (extend it for this feature).
- Confirm the done-check passes with **no LLM keys set**.
- Update `.agents/PRD.md` tracker rows. Commit.

### 4. Push + PR
- `git pull --rebase origin develop` then `git push -u origin feat/<fid>-<slug>`.
- Open a PR to `develop` with `gh pr create`. Title and body in a plain human voice — no emojis, no
  em-dashes, no "comprehensive/robust/seamless", no generated-with footer. Body: what the slice does,
  the done-check, and a short test plan.

### 5. Drive CI to green
- Watch the PR's `ci` and `e2e` runs (`gh run watch`). If anything fails, read the log, fix on the
  branch, commit, push, re-watch. Repeat until both are green.
- Only when green: report the PR link and the passing checks. Do not merge unless told to.

## Stop conditions (report, don't push broken work)
- The feature id is unknown or its done-check is ambiguous.
- A required contract would have to change in a way that breaks another slice (raise it first).
- CI cannot be made green after a genuine fix attempt, or a layer can't be verified end-to-end.
In these cases, stop and report what you found and what you need.
