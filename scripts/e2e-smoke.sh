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

# The intake planner runs on create (deterministic default plan when no LLM
# key is set), so the request moves to in_progress and carries a department
# workflow graph of ~10 stages with parallel review branches.
say "request detail loads with the auto-planned workflow graph"
curl -fsS "$API/requests/${req_id}" -H "authorization: Bearer ${token}" \
  | jq -e --arg id "$req_id" \
      '.request.id == $id and .request.status == "in_progress" and .request.priority == "high"
       and (.nodes | length) >= 9 and (.edges | length) >= 9
       and ([.nodes[].key] | index("exec_approval")) != null' >/dev/null \
  || fail "request detail / workflow graph shape"

echo
echo "SMOKE OK"
