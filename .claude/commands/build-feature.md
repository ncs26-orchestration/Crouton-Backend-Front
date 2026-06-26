---
description: Build one PRD feature end-to-end (DB + backend + agent + frontend + linking), on a branch, with a green PR to develop.
argument-hint: <feature-id e.g. f1>
---

Build feature **$ARGUMENTS** from `.agents/PRD.md` as a complete vertical slice.

Follow `.agents/build-feature.md` exactly. In short:

1. Read feature `$ARGUMENTS` in `.agents/PRD.md` (its DB/BE/AG/FE/Link tasks, referenced BE-/AG-/FE-
   task ids, and the end-to-end done-check) plus the relevant slice-PRD sections and contracts. If
   `$ARGUMENTS` is empty or not a real feature id, stop and ask.
2. Branch from develop: `git checkout develop && git pull --rebase origin develop && git checkout -b feat/$ARGUMENTS-<slug>`.
3. Implement every layer the feature needs, in dependency order (DB → BE → AG → FE → linking),
   reusing existing patterns and the vendored skills in `.agents/skills`. Commit each piece in small
   logical commits. Never `git add -A` (stage explicit paths). No AI co-author trailer.
4. Verify end-to-end locally — build + test each touched app, walk the done-check (extend
   `scripts/e2e-smoke.sh`), and confirm it works with all LLM keys unset. Update the feature's tracker
   rows in `.agents/PRD.md`.
5. `git pull --rebase` then push the branch and open a PR to develop with `gh pr create` (plain human
   voice: no emojis, no em-dashes, no AI tics, no generated-with footer; include the done-check and a
   test plan).
6. Watch the PR's `ci` and `e2e` runs; if red, fix on the branch and push until both are green. Do
   not merge — report the green PR link and checks. Stop and report if you hit a stop condition in
   `.agents/build-feature.md`.

Non-negotiables: keep `develop` untouched; the feature isn't done until its done-check passes and CI
is green end-to-end.
