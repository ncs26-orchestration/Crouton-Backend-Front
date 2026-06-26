# PRD — AI Organization OS, Mobile (Flutter)

The mobile companion app. **Built in a separate Flutter repo**; this file is the spec + contract it
works against. Nothing mobile is scaffolded in this monorepo — the only mobile-driven work that lands
*here* is the handful of backend additions called out under "Backend support needed in this repo".

> Same product as `PRD.md`, different client. Read `PRD.md` (the web/feature plan) and
> `PRD-BACKEND.md` (the HTTP + SSE contracts) first — mobile consumes those same contracts. We track
> mobile work by **feature vertical slice** the same way: each feature is done only when it works
> end-to-end on a device against the real backend.

## How this differs from the pasted draft (decisions + why)

The quick draft spec was a strong UX sketch but assumed an architecture this system doesn't have.
Reconciled as follows:

| Draft said | Reality / decision |
|---|---|
| iOS Swift / Android Kotlin + Flutter web | **Flutter (Dart), one codebase, separate repo.** Single stack, iOS + Android (+ web build optional). |
| Event-sourced "edge node", on-device Position Agent, `/sync/batch` | Backend is a Go **state machine + SSE**, not event-sourced. Mobile is a **read/act client** of the existing contracts. On-device agent reasoning, offline event journal, and batch sync are **Phase 2/3**, not MVP. |
| "case / worker / Position Agent / capability contract" framing | Use the **real domain**: request → workflow node → department agent → executive approval → audit. The good mobile UX (task board, "why am I doing this", approver decision context, trace timeline) is mapped onto that. |
| Telemetry, predictive SLA escalation, multi-tenant role swap, camera hard-gates | **Out of MVP scope** (captured under Out of Scope / later phases). |

Net: mobile MVP = a Flutter client that lets a person submit requests, watch the live workflow,
handle their My Work queue, approve/reject with a justification, and read the audit trail — on the
existing backend, plus push notifications.

## Feature tracker

Legend: ✅ done · 🟡 in progress · ⬜ not started. Layers: **Flutter** the app · **BE+** new/extended
work in *this* repo · **Rides on** the backend feature from `PRD.md` it needs.

| # | Feature (vertical slice) | Flutter | BE+ (this repo) | Rides on | Overall |
|---|---|----|----|----|----|
| M0 | App scaffold + auth + API/SSE client | ⬜ | CORS/mobile-token check | foundation | ⬜ |
| M1 | Requests: list, submit, detail | ⬜ | reuse | F1 ✅ | ⬜ |
| M2 | Workflow status view (mobile, live) | ⬜ | reuse | F2 ✅, F4 | ⬜ |
| M3 | My Work queue (tasks + approvals) | ⬜ | `GET /me/work` | F7, F9 | ⬜ |
| M4 | Approval flow + decision context | ⬜ | reuse | F7 | ⬜ |
| M5 | Status / trace timeline (audit) | ⬜ | reuse | F6 | ⬜ |
| M6 | Push notifications | ⬜ | device register + dispatch | F4/F7 | ⬜ |
| M7 | Request-more-info loop (approver) | ⬜ | `request_info` decision | F7 | ⬜ |
| M8 | Offline read cache + optimistic actions | ⬜ | idempotency keys | F1–F7 | ⬜ |

**Rule (same as `PRD.md`):** a layer cell goes ✅ only when merged AND wired; **Overall** goes ✅ only
when the feature's done-check passes on a real device against the deployed backend. A mobile feature
can't go green before the backend feature it "Rides on" is done.

---

## Backend support needed in this repo

These are the only changes mobile requires *here* (everything else reuses existing contracts). Each
should be built as a normal vertical slice via `/build-feature`-style PRs:

- **Mobile auth fit (M0):** confirm JWT works from a native client; add token refresh or a longer-
  lived token; ensure CORS / `Authorization: Bearer` paths are fine for non-browser clients. SSE auth
  already uses `?token=` (works for Flutter SSE too).
- **`GET /me/work` (M3):** a single endpoint returning the signed-in user's pending approvals and
  assigned items across the org (so mobile doesn't stitch it client-side). Backed by the same
  requests/nodes/approval data.
- **Push notifications (M6):** `POST /me/devices` to register a device token (FCM/APNs); a dispatcher
  that, on the engine's existing audit/event hooks, sends a push for: an approval is waiting on you, a
  request you submitted completed, a node you're watching unblocked. Reuses the SSE event points as
  the trigger source.
- **`request_info` decision (M7):** extend `POST /requests/:id/approve` (or a sibling endpoint) to
  accept a `request_info` decision with a question, which parks the request and notifies the
  requester — a small addition to the F7 approval gate.
- **Idempotency keys (M8):** accept an idempotency key on write endpoints so a queued offline action
  replayed on reconnect doesn't double-apply.

## Mobile features (vertical slices)

Each lists the Flutter work, any backend addition, the backend feature it rides on, and a device
**done-check**. Design reference: the Stitch mockups (treat as direction, not spec) —
https://stitch.withgoogle.com/projects/15721631282172105211.

### M0 — App scaffold + auth + API/SSE client *(enabler)*
- **Flutter:** new Flutter app (separate repo); routing/state (Riverpod or Bloc); `dio` REST client +
  generated/handwritten models mirroring the backend contracts; an SSE client (http byte-stream or
  `flutter_client_sse`); `flutter_secure_storage` for the JWT; login/register against `/auth/*`.
- **BE+:** verify native-client auth; token refresh if needed.
- **Done-check:** install on a device, register/login, land on an authenticated home shell that can
  call the API.

### M1 — Requests: list, submit, detail
- **Flutter:** Requests list (title, requester, priority, status, progress); New Request form
  (title/description/priority); request detail screen shell. (`POST/GET /orgs/:id/requests`,
  `GET /requests/:id`.)
- **Rides on:** F1 ✅.
- **Done-check:** submit "Open a new office in Berlin" (High) on the phone → it appears in the list
  and opens its detail.

### M2 — Workflow status view (mobile, live)
- **Flutter:** render the workflow as a **mobile-friendly vertical timeline / stepper** (not a desktop
  canvas) from `GET /requests/:id` — each department stage with status color, agent, progress; tap a
  stage for node detail (tasks, latest status, activity). Subscribe to `GET /requests/:id/events`
  (SSE) so stages update live; poll fallback.
- **Rides on:** F2 ✅ (graph), F4 (SSE).
- **Done-check:** opening a request shows its stages; a status change appears live without manual
  refresh.

### M3 — My Work queue
- **Flutter:** a Kanban-or-list "My Work": pending approvals assigned to me, items I'm watching,
  recently completed; badges and pull-to-refresh; deep-link a card into M2/M4.
- **BE+:** `GET /me/work`.
- **Rides on:** F7, F9.
- **Done-check:** when a request reaches the approval gate, it appears in My Work on the phone.

### M4 — Approval flow + decision context
- **Flutter:** decision-context screen — the upstream department-agent decisions and flags (from node
  detail + audit), then Approve / Reject with a **required justification**; optimistic update.
  (`POST /requests/:id/approve`.)
- **Rides on:** F7.
- **Done-check:** approve with a justification from the phone → the workflow resumes (visible via
  SSE); reject stops it; both are audited with the text.

### M5 — Status / trace timeline
- **Flutter:** a vertical event timeline from `GET /requests/:id/audit` — every transition with
  actor/action/reason/time, expandable; the agent-declared "waiting for IT" reason shows verbatim.
- **Rides on:** F6.
- **Done-check:** a completed request's full audit trail is readable as a timeline on the device.

### M6 — Push notifications
- **Flutter:** FCM/APNs integration; register the device token on login; foreground + background
  handling; tap-through deep links into the relevant request/approval.
- **BE+:** `POST /me/devices` + a dispatcher on the engine's event hooks.
- **Rides on:** F4/F7.
- **Done-check:** with the app backgrounded, the approver gets a push when an approval is waiting, and
  tapping it opens the decision screen.

### M7 — Request-more-info loop *(approver)*
- **Flutter:** a "Request info" action on the decision screen with a question field; a non-blocking
  "awaiting info" state the approver can leave and return to; notified when answered.
- **BE+:** `request_info` decision on the approval gate.
- **Rides on:** F7.
- **Done-check:** an approver asks for info, the request parks, the requester is notified, and on
  answer the approval re-surfaces.

### M8 — Offline read cache + optimistic actions *(Phase 2)*
- **Flutter:** local cache (drift/sqflite) of the user's requests and My Work for read-only offline;
  queue write actions (claim/approve) and replay on reconnect with an idempotency key; an offline
  banner + buffered-action count.
- **BE+:** idempotency keys on writes.
- **Rides on:** F1–F7.
- **Done-check:** with the network off, cached requests/My Work are readable and an approval queued
  offline applies exactly once when connectivity returns.

---

## Dependencies & sequencing

Mobile rides on the shared backend: a mobile feature can only finish once the backend feature it
"Rides on" is done. Within mobile, **M0 is the enabler for everything**.

```
M0 ─▶ M1 ─▶ M2 ─┬─▶ M3 ─▶ M4 ─▶ M7
                ├─▶ M5
                └─▶ M6
M1..M7 ─▶ M8 (offline, Phase 2)
```

- **Critical path:** `M0 → M1 → M2 → M3 → M4`. That is the demoable spine (submit, watch, queue,
  approve).
- **Backend gating:** M2 needs F4 (SSE) for "live"; M3/M4/M7 need F7 (approval); M5 needs F6 (audit);
  M6 needs the event hooks. So mobile MVP completes in lockstep with backend Wave C (F4–F7).

**Optimal plan:**
- **Wave A — now:** M0 (scaffold/auth/client). In parallel, the backend team builds the M-support
  endpoints (`/me/work`, device register, `request_info`) as small slices.
- **Wave B — after M0:** M1 then M2 (both ride on already-done F1/F2; M2's live polish lands when F4
  ships).
- **Wave C — after M2 and backend F7:** M3, M4 (the approval spine), plus M5 (rides on F6) and M6
  (push) in parallel.
- **Wave D:** M7 (info loop), then M8 (offline) as Phase 2.

---

## Problem Statement

The web dashboard is desktop-first. The people who most need to act on a request — approvers between
meetings, managers on the floor, requesters checking status — are on their phones. Without mobile,
approvals stall, status is opaque away from a desk, and the "live, traceable" promise breaks the
moment someone steps away from their laptop.

## Solution

A Flutter app that puts the same orchestration in your pocket: submit a request, watch its workflow
advance live, see your My Work queue, approve or reject with a written justification, and read the
full audit trail — backed by the existing API and SSE stream, with push notifications so action items
find you.

## User Stories

1. As an approver, I want a push when a request needs my decision, so that I can act without watching a dashboard.
2. As an approver, I want to see every upstream department-agent decision and flag before I decide, so that my approval is informed.
3. As an approver, I want to approve or reject with a written justification from my phone, so that the reasoning is captured and the workflow moves.
4. As an approver, I want to request more information instead of deciding, so that I'm not forced to approve on incomplete context.
5. As a requester, I want to submit a request from my phone, so that I can start work from anywhere.
6. As a requester, I want to watch my request's stages advance live, so that I know where it stands.
7. As a viewer, I want each stage color-coded by status with the owning agent and latest update, so that I can read state at a glance on a small screen.
8. As a viewer, I want to read the full audit timeline on my phone, so that I can reconstruct how a decision was made.
9. As a user, I want a My Work queue of what's assigned to or waiting on me, so that nothing falls through.
10. As a user, I want the app to stay in sync via the live event stream, so that what I see is current.
11. As a user on a flaky connection, I want recently viewed requests and my queue available offline and my queued actions to apply once on reconnect, so that field/transit use is reliable. *(Phase 2)*
12. As a user, I want to sign in securely with my existing account and stay signed in safely, so that access is convenient and protected.

## Implementation Decisions

- **Stack:** Flutter (Dart 3), single codebase for iOS + Android (web build optional), in its **own
  repository**. State: Riverpod (or Bloc) — the mobile team's call. HTTP: `dio`. SSE: an http
  byte-stream reader (or `flutter_client_sse`) against `GET /requests/:id/events?token=`. Secure
  storage: `flutter_secure_storage`. Local cache (Phase 2): drift/sqflite. Push: `firebase_messaging`
  (FCM) + APNs.
- **Client, not a second backend.** The app holds no business rules beyond input validation; all
  orchestration, dependency gating, and approval logic stay in the Go engine. The app reads
  projections and posts actions.
- **Contracts are the source of truth.** Mobile binds to the contracts in `PRD-BACKEND.md`. If mobile
  needs a shape that doesn't exist, it's added as a backend slice in this repo (see "Backend support
  needed"), not worked around on-device. Keep a small shared contract doc/types so web and mobile
  don't drift.
- **Mobile-native UX over canvas.** The desktop React Flow canvas becomes a vertical
  timeline/stepper on mobile. My Work is the primary surface (the draft's "Kanban board"), mapped to
  real tasks/approvals.
- **Auth:** reuse JWT login; add refresh/longer-lived tokens for native sessions. SSE auth via query
  token already works for a native client.

## Testing Decisions

- **Widget/golden tests** for the key screens (request list, workflow timeline, decision context, My
  Work) using fake repositories — assert rendered state from a given API payload, not implementation.
- **Repository/client unit tests** against a mocked HTTP layer: request CRUD, approval post, SSE
  event → state reduction, and (Phase 2) the offline queue applying an action exactly once with an
  idempotency key.
- **Backend additions** (`/me/work`, device register, `request_info`, idempotency) get Go tests in
  this repo following the existing handler/table-driven pattern, and an extension of
  `scripts/e2e-smoke.sh` for the new endpoints.
- Determinism: no live LLM calls in tests; the backend's no-key fallback path means a device demo
  works without provider keys.

## Out of Scope (MVP) / later phases

- On-device "Position Agent" reasoning, local policy hard-gates, and AI diagnostic synthesis.
- Event-sourced offline journal and `/sync/batch` (MVP offline is read-cache + a small action queue
  in M8).
- Telemetry ingestion, predictive SLA escalation, similar-case ML.
- Multi-tenant facility/role swapping.
- Camera-based attachment validation and step-by-step execution hard-gates.
- These are captured so the vision isn't lost, but they are explicitly post-MVP and several assume
  backend capabilities (worker-task model, telemetry, event store) that do not exist today.

## Further Notes

- This document lives in this repo so the contracts and the required backend additions are tracked
  next to the code that owns them; the Flutter implementation lives in its own repo and references
  this file.
- Keep mobile and web in sync by treating `PRD-BACKEND.md` contracts as the shared interface; any
  change there is a cross-client concern.
- Demo bar: on a phone, get a push for a pending approval → open decision context with the upstream
  agent decisions → approve with a justification → watch the workflow resume live → read the audit
  timeline.
