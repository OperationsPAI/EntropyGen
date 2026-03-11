#!/usr/bin/env bash
# scripts/smoke/frontend.sh
# Smoke test for the Frontend (nginx proxy + static assets).
# Verifies page load, API proxy, and basic auth flow through the frontend.
#
# Usage:
#   FRONTEND_URL=http://localhost:18083 ./scripts/smoke/frontend.sh
set -euo pipefail

FRONTEND_URL=${FRONTEND_URL:-http://localhost:3001}
ADMIN_USER=${ADMIN_USER:-admin}
ADMIN_PASS=${ADMIN_PASS:-admin}

pass() { echo "  [PASS] $*"; }
fail() { echo "  [FAIL] $*"; exit 1; }

echo "=== Smoke Test: Frontend ($FRONTEND_URL) ==="

# ── Static Assets ──────────────────────────────────────────────────────────────
echo ""
echo "--- Static Assets ---"
curl -sf "$FRONTEND_URL/" | grep -q '<html' && pass "GET / → HTML page" || fail "index.html"
curl -sf "$FRONTEND_URL/" | grep -q 'EntropyGen' && pass "Page contains 'EntropyGen'" || fail "title missing"

# ── API Proxy ──────────────────────────────────────────────────────────────────
echo ""
echo "--- API Proxy ---"

# Health via proxy
curl -sf "$FRONTEND_URL/api/health" | grep -q '"ok"' \
  && pass "GET /api/health (proxied)" || fail "proxy health"

# Login via proxy
LOGIN_RESP=$(curl -sf -X POST "$FRONTEND_URL/api/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$ADMIN_USER\",\"password\":\"$ADMIN_PASS\"}")
TOKEN=$(echo "$LOGIN_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin).get('token',''))" 2>/dev/null)
[ -n "$TOKEN" ] && pass "POST /api/auth/login (proxied) → token" || fail "proxy login"

# Auth/me via proxy
curl -sf "$FRONTEND_URL/api/auth/me" -H "Authorization: Bearer $TOKEN" \
  | grep -q '"username"' && pass "GET /api/auth/me (proxied)" || fail "proxy auth/me"

# Agents via proxy
curl -sf "$FRONTEND_URL/api/agents" -H "Authorization: Bearer $TOKEN" \
  | grep -q '"success":true' && pass "GET /api/agents (proxied)" || fail "proxy agents"

# Traces via proxy
curl -sf "$FRONTEND_URL/api/audit/traces?limit=3" -H "Authorization: Bearer $TOKEN" \
  | grep -q '"success":true' && pass "GET /api/audit/traces (proxied)" || fail "proxy traces"

# ── SPA Routing ────────────────────────────────────────────────────────────────
echo ""
echo "--- SPA Routing ---"
# All frontend routes should return the same HTML (SPA fallback)
for ROUTE in /dashboard /agents /audit /export /login; do
  curl -sf "$FRONTEND_URL$ROUTE" | grep -q '<html' \
    && pass "GET $ROUTE → HTML (SPA)" || fail "SPA route $ROUTE"
done

echo ""
echo "=== Frontend smoke tests passed ==="
