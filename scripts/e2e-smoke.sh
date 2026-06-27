#!/usr/bin/env bash
# End-to-end smoke test: hits the live stack and exercises the critical path
# (health, readiness, auth, org creation). Used by .github/workflows/e2e.yml and
# runnable locally after `make up`.
set -euo pipefail

API=${API:-http://localhost:8080}
AGENT=${AGENT:-http://localhost:8000}

say()  { printf "\n=== %s ===\n" "$1"; }
fail() { echo "SMOKE FAIL: $1" >&2; exit 1; }

command -v jq >/dev/null 2>&1 || fail "jq is required"

say "api /healthz"
curl -fsS "$API/healthz" | jq -e '.status == "ok"' >/dev/null || fail "api healthz"

say "api /readyz (db + redis up)"
curl -fsS "$API/readyz" | jq -e '.db == "up" and .redis == "up"' >/dev/null || fail "api readyz"

say "agent /healthz"
curl -fsS "$AGENT/healthz" | jq -e '.status == "ok"' >/dev/null || fail "agent healthz"

ts=$(date +%s)
email="ci+${ts}@example.com"
slug="ci-${ts}"

say "register a user"
token=$(curl -fsS -X POST "$API/auth/register" \
  -H 'content-type: application/json' \
  -d "{\"email\":\"${email}\",\"password\":\"password123\",\"name\":\"CI User\"}" \
  | jq -r '.token')
[ -n "$token" ] && [ "$token" != "null" ] || fail "no token returned from register"

say "login"
curl -fsS -X POST "$API/auth/login" \
  -H 'content-type: application/json' \
  -d "{\"email\":\"${email}\",\"password\":\"password123\"}" \
  | jq -e '.token != null' >/dev/null || fail "login"

say "create org"
org_id=$(curl -fsS -X POST "$API/orgs" \
  -H "authorization: Bearer ${token}" \
  -H 'content-type: application/json' \
  -d "{\"name\":\"CI Org\",\"slug\":\"${slug}\"}" \
  | jq -r '.id')
[ -n "$org_id" ] && [ "$org_id" != "null" ] || fail "create org (no id)"

say "list orgs contains the new org"
curl -fsS "$API/orgs" -H "authorization: Bearer ${token}" \
  | jq -e --arg slug "$slug" '[.. | .slug? // empty] | index($slug) != null' >/dev/null \
  || fail "new org not found in list"

# - F10: a fresh org comes pre-seeded with agents and policies -

say "fresh org has seeded agents (F10)"
curl -fsS "$API/orgs/${org_id}/agents" -H "authorization: Bearer ${token}" \
  | jq -e '(.agents | length >= 5) and (.agents[0].agent_type != null)' >/dev/null \
  || fail "new org should have >= 5 seeded agents"

# F9: the roster carries a live status. Before any request runs every agent is
# idle, and the derived activity counts are present.
say "F9: roster carries live status (all idle before any request)"
curl -fsS "$API/orgs/${org_id}/agents" -H "authorization: Bearer ${token}" \
  | jq -e '([.agents[] | select(.status == "idle")] | length) == (.agents | length)
           and (.agents[0] | has("active") and has("completed") and has("request_count"))' >/dev/null \
  || fail "roster should expose live status, all idle before any request"

say "fresh org has seeded policies (F10)"
curl -fsS "$API/orgs/${org_id}/policies" -H "authorization: Bearer ${token}" \
  | jq -e '(.policies | length >= 1) and (.policies[0].title != null)' >/dev/null \
  || fail "new org should have seeded policies"

# --- F1/F2: submit a request and get an auto-planned workflow graph ---

say "submit a request (Open a new office in Berlin, High)"
req_id=$(curl -fsS -X POST "$API/orgs/${org_id}/requests" \
  -H "authorization: Bearer ${token}" \
  -H 'content-type: application/json' \
  -d '{"title":"Open a new office in Berlin","description":"Expand into the EU market","priority":"high"}' \
  | jq -r '.request.id')
[ -n "$req_id" ] && [ "$req_id" != "null" ] || fail "create request (no id)"

say "new request appears in the org request list"
curl -fsS "$API/orgs/${org_id}/requests" -H "authorization: Bearer ${token}" \
  | jq -e --arg id "$req_id" '.requests | map(.id) | index($id) != null' >/dev/null \
  || fail "new request not found in list"

# The intake planner runs on create (deterministic default plan when no LLM key
# is set), so the request lands in 'draft' carrying a department workflow graph
# of ~10 stages with parallel review branches. It does not run until launched.
say "request detail loads in draft with the planned workflow graph"
curl -fsS "$API/requests/${req_id}" -H "authorization: Bearer ${token}" \
  | jq -e --arg id "$req_id" \
      '.request.id == $id and .request.priority == "high"
       and .request.status == "draft"
       and (.nodes | length) >= 9 and (.edges | length) >= 9
       and ([.nodes[].key] | index("exec_approval")) != null' >/dev/null \
  || fail "request detail / workflow graph shape"

# Launch the draft (no verifiers assigned, so every node runs automatically).
say "launch the draft request"
curl -fsS -X POST "$API/requests/${req_id}/launch" -H "authorization: Bearer ${token}" \
  | jq -e '.request.status == "in_progress"' >/dev/null \
  || fail "launch did not move the request to in_progress"

# F4: start a background SSE listener BEFORE the engine runs so we capture
# live events. We kill it after the request completes.
say "SSE events endpoint: starting background listener"
sse_out=$(mktemp)
curl -fsS -N --max-time 10 "$API/requests/${req_id}/events?token=${token}" >"$sse_out" 2>/dev/null &
sse_pid=$!

# F5: the engine should produce a transient blocked state (Finance blocked on
# IT) because finance_review runs before it_assessment in the default plan and
# the deterministic Python _finance() returns blocked_on without IT output.
say "F5: a blocked node appears while it_assessment is still running"
blocked_seen=""
for _ in $(seq 1 30); do
  detail=$(curl -fsS "$API/requests/${req_id}" -H "authorization: Bearer ${token}")
  blocked_count=$(echo "$detail" | jq '[.nodes[] | select(.status == "blocked")] | length')
  if [ "$blocked_count" -ge 1 ]; then
    blocked_seen="yes"
    blocked_node=$(echo "$detail" | jq -r '.nodes[] | select(.status == "blocked") | .key')
    echo "  Blocked node detected: $blocked_node"
    break
  fi
  sleep 0.5
done
[ -n "$blocked_seen" ] || fail "no blocked node appeared (F5)"

# F9: with a node blocked and others mid-flight, the roster reflects live status
# — at least one agent reports busy (owns an in_progress node) or blocked.
say "F9: an agent shows non-idle live status while the request runs"
curl -fsS "$API/orgs/${org_id}/agents" -H "authorization: Bearer ${token}" \
  | jq -e '[.agents[] | select(.status == "busy" or .status == "blocked")] | length >= 1' >/dev/null \
  || fail "no agent reported busy/blocked live status during the run"

# F3/F7: with no verifiers assigned, the engine runs the review stages through
# their department agents (deterministic with no LLM key), auto-advances the
# executive sign-off gate, and drives the request to completion.
say "engine auto-advances the gate and runs the request to completion"
detail=""
for _ in $(seq 1 90); do
  detail=$(curl -fsS "$API/requests/${req_id}" -H "authorization: Bearer ${token}")
  status=$(echo "$detail" | jq -r '.request.status')
  [ "$status" = "completed" ] && break
  sleep 1
done
echo "$detail" | jq -e \
  '.request.status == "completed" and .request.progress == 100
   and ([.nodes[] | select(.status != "completed")] | length) == 0
   and ([.nodes[] | select(.status_text == "")] | length) == 0' >/dev/null \
  || fail "request did not run to completion"

# Stop the SSE listener and check it received events.
kill "$sse_pid" 2>/dev/null || true
wait "$sse_pid" 2>/dev/null || true
events=$(grep -c "^event: " "$sse_out" || true)
rm "$sse_out"
say "SSE events captured: $events"
[ "$events" -ge 1 ] || fail "SSE endpoint: expected at least 1 event, got $events"

say "a completed node carries the agent's tasks"
node_id=$(echo "$detail" | jq -r '.nodes[0].id')
node_detail=$(curl -fsS "$API/requests/${req_id}/nodes/${node_id}" -H "authorization: Bearer ${token}")
echo "$node_detail" | jq -e '(.tasks | length) >= 1 and (.tasks[0].status == "completed") and (.node.status == "completed")' >/dev/null \
  || fail "node detail / tasks shape"

# F6: verify the audit trail is populated after a completed run.
say "node detail includes audit activity (F6)"
echo "$node_detail" | jq -e '(.activity | length) >= 1 and (.activity[0].actor != null and .activity[0].action != null)' >/dev/null \
  || fail "node detail / audit activity is empty"

say "request audit endpoint returns events (F6)"
curl -fsS "$API/requests/${req_id}/audit" -H "authorization: Bearer ${token}" \
  | jq -e '(.events | length) >= 1' >/dev/null \
  || fail "request audit endpoint returned empty events"

say "org audit endpoint returns events (F6)"
curl -fsS "$API/orgs/${org_id}/audit" -H "authorization: Bearer ${token}" \
  | jq -e '(.events | length) >= 1' >/dev/null \
  || fail "org audit endpoint returned empty events"

# F8: the final report endpoint compiles a structured report for the completed
# request. It must return the request overview, per-stage details, approval
# info, and summary.
say "final report endpoint returns a structured report (F8)"
curl -fsS "$API/requests/${req_id}/report" -H "authorization: Bearer ${token}" \
  | jq -e \
    '.request.id == "'"${req_id}"'"
     and .request.status == "completed"
     and (.stages | length) >= 9
     and .approval.decision == "approve"
     and .approval.justification != ""
     and .summary.total_stages >= 9
     and .summary.completed_stages >= 9
     and .summary.total_time_human != ""
     and (.stages[0].tasks | length) >= 1' >/dev/null \
  || fail "final report shape"

echo
echo "SMOKE OK"
