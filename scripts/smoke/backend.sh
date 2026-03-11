#!/usr/bin/env bash
# scripts/smoke/backend.sh
# Smoke test for the Control Panel Backend API.
# Verifies health, auth flow, audit traces, and export.
#
# Usage:
#   ./scripts/smoke/backend.sh                          # local dev (localhost:8080)
#   BACKEND_URL=http://localhost:18081 ./scripts/smoke/backend.sh  # port-forwarded K8s
set -euo pipefail

BACKEND_URL=${BACKEND_URL:-http://localhost:8080}
ADMIN_USER=${ADMIN_USER:-admin}
ADMIN_PASS=${ADMIN_PASS:-admin}

pass() { echo "  [PASS] $*"; }
fail() { echo "  [FAIL] $*"; exit 1; }
warn() { echo "  [WARN] $*"; }

echo "=== Smoke Test: Backend API ($BACKEND_URL) ==="

# ── Health ─────────────────────────────────────────────────────────────────────
echo ""
echo "--- Health ---"
curl -sf "$BACKEND_URL/api/health" | grep -q '"ok"' && pass "GET /api/health" || fail "health check"

# ── Auth ───────────────────────────────────────────────────────────────────────
echo ""
echo "--- Auth ---"

# Login success
TOKEN=$(curl -sf -X POST "$BACKEND_URL/api/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$ADMIN_USER\",\"password\":\"$ADMIN_PASS\"}" \
  | python3 -c "import sys,json; print(json.load(sys.stdin).get('token',''))")
[ -n "$TOKEN" ] && pass "POST /api/auth/login → token obtained" || fail "login: no token"

# Login wrong password
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BACKEND_URL/api/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"WRONG"}')
[ "$HTTP_CODE" = "401" ] && pass "POST /api/auth/login (wrong password) → 401" || fail "expected 401, got $HTTP_CODE"

# /auth/me with token
curl -sf "$BACKEND_URL/api/auth/me" -H "Authorization: Bearer $TOKEN" \
  | grep -q '"username"' && pass "GET /api/auth/me → user info" || fail "auth/me"

# /auth/me without token
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BACKEND_URL/api/auth/me")
[ "$HTTP_CODE" = "401" ] && pass "GET /api/auth/me (no token) → 401" || fail "expected 401, got $HTTP_CODE"

# Logout
curl -sf -X POST "$BACKEND_URL/api/auth/logout" -H "Authorization: Bearer $TOKEN" \
  | grep -q '"success":true' && pass "POST /api/auth/logout" || fail "logout"

# Re-login for subsequent tests
TOKEN=$(curl -sf -X POST "$BACKEND_URL/api/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$ADMIN_USER\",\"password\":\"$ADMIN_PASS\"}" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")

# ── Agents ─────────────────────────────────────────────────────────────────────
echo ""
echo "--- Agents ---"
AGENTS_RESP=$(curl -sf "$BACKEND_URL/api/agents" -H "Authorization: Bearer $TOKEN")
echo "$AGENTS_RESP" | grep -q '"success":true' && pass "GET /api/agents → success" || fail "agents list"

# ── Audit ──────────────────────────────────────────────────────────────────────
echo ""
echo "--- Audit ---"

# List traces (no filter)
TRACES=$(curl -sf "$BACKEND_URL/api/audit/traces?limit=5" -H "Authorization: Bearer $TOKEN")
echo "$TRACES" | grep -q '"success":true' && pass "GET /api/audit/traces → success" || fail "traces list"
COUNT=$(echo "$TRACES" | python3 -c "import sys,json; print(json.load(sys.stdin)['meta']['count'])" 2>/dev/null || echo 0)
echo "    (returned $COUNT traces)"

# Limit cap
curl -sf "$BACKEND_URL/api/audit/traces?limit=999" -H "Authorization: Bearer $TOKEN" \
  | python3 -c "import sys,json; m=json.load(sys.stdin)['meta']; assert m['limit']<=200" \
  && pass "GET /api/audit/traces?limit=999 → capped to ≤200" || fail "limit cap"

# Stats endpoints
for EP in token-usage agent-activity operations; do
  curl -sf "$BACKEND_URL/api/audit/stats/$EP" -H "Authorization: Bearer $TOKEN" \
    | grep -q '"success":true' && pass "GET /api/audit/stats/$EP" || warn "stats/$EP returned unexpected"
done

# Export
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BACKEND_URL/api/audit/export?limit=10" \
  -H "Authorization: Bearer $TOKEN")
[ "$HTTP_CODE" = "200" ] && pass "GET /api/audit/export → 200" || fail "export: got $HTTP_CODE"

# ── Response shape contract ────────────────────────────────────────────────────
echo ""
echo "--- Response Contract ---"

# Success response has { success: true }
curl -sf "$BACKEND_URL/api/health" | python3 -c "
import sys,json
d=json.load(sys.stdin)
assert 'status' in d, 'missing status field'
" && pass "Health response shape" || fail "health shape"

# Error response has { success: false, error, code }
curl -s "$BACKEND_URL/api/auth/me" | python3 -c "
import sys,json
d=json.load(sys.stdin)
assert d.get('success') == False, 'expected success=false'
assert 'error' in d, 'missing error field'
assert 'code' in d, 'missing code field'
" && pass "Error response shape" || fail "error shape"

echo ""
echo "=== Backend smoke tests passed ==="
