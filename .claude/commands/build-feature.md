---
description: Build one PRD feature end-to-end (DB + backend + agent + frontend + linking), on a branch, with a green PR to develop.
argument-hint: <feature-id e.g. f1>
---

Build feature **$ARGUMENTS** from `.agents/PRD.md` as a complete vertical slice.

Follow `.agents/build-feature.md` exactly. In short:

1. Read feature `$ARGUMENTS` in `.agents/PRD.md` (its DB/BE/AG/FE/Link tasks, referenced BE-/AG-/FE-
   task ids, and the end-to-end done-check) plus the relevant slice-PRD sections and contracts. If
   `$ARGUMENTS` is empty or not a real feature id, stop and ask.
2. Work in an isolated git worktree so the primary `solution` checkout stays on its current branch,
   untouched and usable. From the repo root:
   `git fetch origin develop && git worktree add -b feat/$ARGUMENTS-<slug> .worktrees/$ARGUMENTS-<slug> origin/develop`,
   then `cd .worktrees/$ARGUMENTS-<slug>` and run every step there. `.worktrees/` is gitignored. Never
   switch the main checkout's branch or work on `develop` directly.
3. Implement every layer the feature needs, in dependency order (DB → BE → AG → FE → linking),
   reusing existing patterns. **Before writing a layer, read the matching `SKILL.md` under
   `.agents/skills/{backend,agent,frontend}`** per the Skills routing table in `.agents/build-feature.md`
   (e.g. `i-golang-database` for repos/queries, `i-api-design-principles` for handlers,
   `i-pydantic-ai` for agents, `i-react-flow-node-ts` for canvas nodes, `i-tailwind-design-system` for
   UI); run `react-doctor` before each frontend commit. **Any migration that adds a table must also
   extend the demo seed (`apps/api/cmd/seed/main.go`, run via `make seed`) with realistic idempotent
   data and add a test that the seed leaves the table non-empty.** Commit each piece in small logical
   commits. Never `git add -A` (stage explicit paths). No AI co-author trailer.
4. Verify end-to-end locally — build + test each touched app, walk the done-check (extend
   `scripts/e2e-smoke.sh`), and confirm it works with all LLM keys unset. Update the feature's tracker
   rows in `.agents/PRD.md`.
5. `git pull --rebase` then push the branch and open a PR to develop with `gh pr create` (plain human
   voice: no emojis, no em-dashes, no AI tics, no generated-with footer; include the done-check and a
   test plan).
6. Watch the PR's `ci` and `e2e` runs; if red, fix on the branch and push until both are green. Do
   not merge — report the green PR link and checks. Stop and report if you hit a stop condition in
   `.agents/build-feature.md`.
7. When green, remove the worktree (`git worktree remove .worktrees/$ARGUMENTS-<slug>` from the repo
   root); the branch and PR stay on the remote. Leave it only if asked to inspect, and say where.

Non-negotiables: work in a worktree and keep the main `solution` checkout on its own branch; keep
`develop` untouched; read the relevant `.agents/skills` `SKILL.md` before writing each layer; the
feature isn't done until its done-check passes and CI is green end-to-end.
