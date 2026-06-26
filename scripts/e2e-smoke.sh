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
curl -fsS -X POST "$API/orgs" \
  -H "authorization: Bearer ${token}" \
  -H 'content-type: application/json' \
  -d "{\"name\":\"CI Org\",\"slug\":\"${slug}\"}" \
  | jq -e '.id != null or .org != null' >/dev/null || fail "create org"

say "list orgs contains the new org"
curl -fsS "$API/orgs" -H "authorization: Bearer ${token}" \
  | jq -e --arg slug "$slug" '[.. | .slug? // empty] | index($slug) != null' >/dev/null \
  || fail "new org not found in list"

echo
echo "SMOKE OK"
