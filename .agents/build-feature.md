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
2. **Work in an isolated git worktree, never the main checkout.** Do NOT switch the primary
   `solution` working tree's branch — the user (or another agent) keeps using it while you build.
   Create a dedicated worktree off `develop` and do every step there:
   ```
   git -C <repo> fetch origin develop
   git -C <repo> worktree add -b feat/<fid>-<slug> <repo>/.worktrees/<fid>-<slug> origin/develop
   cd <repo>/.worktrees/<fid>-<slug>
   ```
   `.worktrees/` is gitignored, so the primary checkout's `git status` stays clean and on its own
   branch. All build/test/commit/push commands run from inside the worktree. When the PR is green (or
   you stop), remove it with `git -C <repo> worktree remove .worktrees/<fid>-<slug>` (use `--force`
   only if you have confirmed nothing uncommitted is worth keeping); leave it if the user wants to
   inspect. Branch name: `feat/<fid>-<slug>` (e.g. `feat/f1-submit-request`). Never work on `develop`
   directly.
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
8. **New table ⇒ seed it and test the seed.** Any migration that adds a table must, in the same
   slice, extend the demo seed (`apps/api/cmd/seed/main.go`, run via `make seed`) to populate that
   table with realistic data, AND add a test that the seed runs clean and leaves the table non-empty
   (a Go seed test, or an assertion in `scripts/e2e-smoke.sh`). The seed must be idempotent (safe to
   re-run). A new table that ships empty, unseeded, or untested is an incomplete slice.
9. **Read the skill before you write the layer.** The repo vendors discipline skills in
   `.agents/skills/{frontend,backend,agent}` (catalog in `.agents/skills/README.md`). Before writing
   code in a layer, open the relevant `SKILL.md` (and its `reference/`) and follow it — pick by the
   work in front of you, not just the layer name. This is how the slice stays idiomatic and on-contract;
   skipping it is how slop and contract drift get in. Use the routing table below.

## Skills routing — read the matching `SKILL.md` first

Paths are relative to `.agents/skills/`. Read the ones that match the piece you're about to write; a
feature usually touches several.

| Working on… | Read first |
|---|---|
| DB schema, pgx repos, queries, transactions | `backend/i-golang-database`, `backend/i-golang-naming` |
| Go HTTP handlers, REST/SSE API surface | `backend/i-api-design-principles`, `backend/i-golang-error-handling`, `backend/i-golang-context` |
| Orchestration engine, workers, concurrency | `backend/i-golang-concurrency`, `backend/i-golang-design-patterns`, `backend/i-golang-pro` |
| Go structs/interfaces, project layout | `backend/i-golang-structs-interfaces`, `backend/i-golang-project-layout`, `backend/i-golang-code-style` |
| Go tests | `backend/i-golang-testing`, `backend/i-golang-stretchr-testify` |
| Go lint / logging / metrics | `backend/i-golang-lint`, `backend/i-golang-observability`, `backend/i-golang-security` |
| Pydantic AI agents + typed `Plan`/`Decision` output | `agent/i-pydantic-ai`, `agent/i-pydantic-models-py` |
| Agent tools, `raise_dependency`, multi-agent design | `agent/i-agent-tool-builder`, `agent/i-ai-agents-architect`, `agent/i-multi-agent-patterns`, `agent/i-autonomous-agent-patterns` |
| Agent tests / evals | `agent/i-agent-evaluation` |
| React Flow canvas, department nodes, layout | `frontend/i-react-flow-architect`, `frontend/i-react-flow-node-ts` |
| React components, hooks, server vs client state, SSE | `frontend/i-react-patterns`, `frontend/i-react-state-management`, `frontend/i-frontend-api-integration-patterns`, `frontend/i-vercel-react-best-practices` |
| Tailwind, design tokens, component variants | `frontend/i-tailwind-design-system` |
| UI polish, hierarchy, motion, a11y, design review | `frontend/i-impeccable`, `frontend/emil-design-eng`, `frontend/high-end-visual-design`, `frontend/design-taste-frontend`, `frontend/i-web-design-guidelines` |

Before each frontend commit, run the `frontend/react-doctor` skill and fix its findings (it covers
lint, a11y, bundle, architecture). Honor `../DESIGN.md` and the `mvp.png` target throughout.

## Procedure

### 0. Plan
- Read the feature's section in `.agents/PRD.md`: its DB/BE/AG/FE/Link tasks, the referenced
  slice-PRD task ids (BE-/AG-/FE-), and the done-check.
- Open the referenced sections in `PRD-BACKEND.md` / `PRD-AGENT.md` / `PRD-FRONTEND.md` for step
  detail and the **shared contracts** (HTTP/SSE shapes, the Go↔Python agent contract). Honor the
  contracts exactly — other slices depend on them.
- Read the relevant vendored skills under `.agents/skills/{backend,agent,frontend}` before writing
  code in that layer — use the **Skills routing** table above to pick the matching `SKILL.md` for each
  piece (e.g. `i-golang-concurrency` for the engine, `i-pydantic-ai` + `i-agent-tool-builder` for
  agents, `i-react-flow-node-ts` for canvas nodes).
- Reuse existing patterns: pgx repos like `internal/repo/orgs.go`, handler constructors registered in
  `internal/http/server.go`, the agent provider config in `apps/agent/app/settings.py`, the web
  `lib/api.ts` client and design tokens in `index.css`.

### 1. Worktree (isolated; the main checkout stays put)
- Fetch the latest base without touching the primary checkout's branch:
  `git -C <repo> fetch origin develop`.
- Create the feature worktree off the fresh base and enter it:
  `git -C <repo> worktree add -b feat/<fid>-<slug> <repo>/.worktrees/<fid>-<slug> origin/develop`
  then `cd <repo>/.worktrees/<fid>-<slug>`.
- Everything below runs here. The primary `solution` working tree is never checked out to the feature
  branch and stays usable. Tear the worktree down after the PR is green (golden rule 2).

### 2. Implement, layer by layer, committing each piece
Work in dependency order. Commit after each coherent piece.
- **DB:** create migrations with `dbmate new <name>` (real `YYYYMMDDHHMMSS` UTC prefix, both
  `migrate:up` and `migrate:down`). Apply with `make migrate-up`. **For every new table, in the same
  step extend the demo seed (`apps/api/cmd/seed/main.go`) to populate it with realistic, idempotent
  data, run `make seed` to confirm it applies cleanly (and re-applies without duplicating), and add a
  test that the seed leaves the table non-empty** (Go seed test or an `scripts/e2e-smoke.sh`
  assertion). Commit.
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

### 6. Clean up the worktree
- After reporting the green PR, remove the worktree from the primary checkout:
  `git -C <repo> worktree remove .worktrees/<fid>-<slug>` (the branch stays on the remote and in the
  PR). Keep it only if the user asked to inspect locally. If you stop early, leave the worktree in
  place and say where it is.

## Stop conditions (report, don't push broken work)
- The feature id is unknown or its done-check is ambiguous.
- A required contract would have to change in a way that breaks another slice (raise it first).
- CI cannot be made green after a genuine fix attempt, or a layer can't be verified end-to-end.
In these cases, stop and report what you found and what you need.
