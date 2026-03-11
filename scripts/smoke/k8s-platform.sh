#!/usr/bin/env bash
# scripts/smoke/k8s-platform.sh
# Smoke test for the full platform running in Kubernetes.
# Verifies all pods are ready, services respond, and the E2E user flow works.
#
# Prerequisites:
#   kubectl configured with cluster access
#   Platform deployed in the target namespace
#
# Usage:
#   ./scripts/smoke/k8s-platform.sh                    # default: namespace=aidevops
#   NAMESPACE=my-ns ./scripts/smoke/k8s-platform.sh
set -euo pipefail

NS=${NAMESPACE:-aidevops}

pass() { echo "  [PASS] $*"; }
fail() { echo "  [FAIL] $*"; exit 1; }
warn() { echo "  [WARN] $*"; }

cleanup() {
  echo ""
  echo "Cleaning up port-forwards..."
  kill $PF_BACKEND $PF_FRONTEND $PF_GATEWAY $PF_GITEA $PF_CH 2>/dev/null || true
}
trap cleanup EXIT

echo "=== Smoke Test: K8s Platform (namespace=$NS) ==="

# ── Pod Readiness ──────────────────────────────────────────────────────────────
echo ""
echo "--- Pod Readiness ---"
EXPECTED_PODS=("redis-0" "clickhouse-0" "gitea-0" "agent-gateway" "control-panel-backend" "control-panel-frontend" "devops-operator" "event-collector")
for POD_PREFIX in "${EXPECTED_PODS[@]}"; do
  READY=$(kubectl get pods -n "$NS" --no-headers 2>/dev/null | grep "$POD_PREFIX" | awk '{print $2}' | head -1)
  if echo "$READY" | grep -qE "^[0-9]+/[0-9]+$"; then
    WANT=$(echo "$READY" | cut -d/ -f2)
    GOT=$(echo "$READY" | cut -d/ -f1)
    [ "$GOT" = "$WANT" ] && pass "$POD_PREFIX ($READY)" || fail "$POD_PREFIX not ready ($READY)"
  else
    fail "$POD_PREFIX not found"
  fi
done

# ── Setup Port-Forwards ───────────────────────────────────────────────────────
echo ""
echo "--- Setting up port-forwards ---"
kubectl port-forward svc/control-panel-backend 28081:80 -n "$NS" > /dev/null 2>&1 &
PF_BACKEND=$!
kubectl port-forward svc/frontend 28080:80 -n "$NS" > /dev/null 2>&1 &
PF_FRONTEND=$!
kubectl port-forward svc/agent-gateway 28082:80 -n "$NS" > /dev/null 2>&1 &
PF_GATEWAY=$!
kubectl port-forward svc/gitea 23000:3000 -n "$NS" > /dev/null 2>&1 &
PF_GITEA=$!
kubectl port-forward svc/clickhouse 28123:8123 -n "$NS" > /dev/null 2>&1 &
PF_CH=$!
sleep 4
pass "Port-forwards established"

# ── Service Health ─────────────────────────────────────────────────────────────
echo ""
echo "--- Service Health ---"
curl -sf http://localhost:28081/api/health | grep -q '"ok"' && pass "Backend health" || fail "Backend unreachable"
curl -sf http://localhost:28082/healthz | grep -q "ok" && pass "Gateway health" || fail "Gateway unreachable"
curl -sf http://localhost:23000/api/v1/version | grep -q "version" && pass "Gitea health" || fail "Gitea unreachable"
curl -sf http://localhost:28123/ping | grep -q "Ok" && pass "ClickHouse health" || fail "ClickHouse unreachable"
curl -sf http://localhost:28080/ | grep -q '<html' && pass "Frontend serves HTML" || fail "Frontend unreachable"

# ── Auth Flow ──────────────────────────────────────────────────────────────────
echo ""
echo "--- Auth Flow ---"
TOKEN=$(curl -sf -X POST http://localhost:28081/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin).get('token',''))")
[ -n "$TOKEN" ] && pass "Login → token" || fail "Login failed"

curl -sf http://localhost:28081/api/auth/me -H "Authorization: Bearer $TOKEN" \
  | grep -q '"admin"' && pass "/auth/me → admin" || fail "auth/me"

# ── Audit Pipeline ─────────────────────────────────────────────────────────────
echo ""
echo "--- Audit Pipeline (ClickHouse → Backend API) ---"

# Insert test trace directly into ClickHouse
curl -sf http://localhost:28123/ --data "
INSERT INTO audit.traces
  (trace_id, span_id, agent_id, agent_role, request_type, method, path, status_code, latency_ms, tokens_in, tokens_out, model)
VALUES
  (generateUUIDv4(), generateUUIDv4(), 'smoke-test-agent', 'developer', 'llm_api', 'POST', '/v1/chat/completions', 200, 1500, 800, 400, 'claude-3-sonnet')
" && pass "ClickHouse INSERT" || fail "ClickHouse insert"

# Query back through Backend API
TRACES=$(curl -sf "http://localhost:28081/api/audit/traces?agent_id=smoke-test-agent&limit=5" \
  -H "Authorization: Bearer $TOKEN")
TRACE_COUNT=$(echo "$TRACES" | python3 -c "import sys,json; print(json.load(sys.stdin)['meta']['count'])" 2>/dev/null || echo 0)
[ "$TRACE_COUNT" -ge 1 ] && pass "Backend returns trace (count=$TRACE_COUNT)" || fail "Trace not found via API"

# Verify field values
echo "$TRACES" | python3 -c "
import sys,json
d=json.load(sys.stdin)
t=d['data'][0]
assert t['AgentID'] == 'smoke-test-agent', f'wrong agent: {t[\"AgentID\"]}'
assert t['Model'] == 'claude-3-sonnet', f'wrong model: {t[\"Model\"]}'
assert t['TokensIn'] == 800, f'wrong tokens_in: {t[\"TokensIn\"]}'
" && pass "Trace fields correct" || fail "Trace field mismatch"

# Export
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:28081/api/audit/export?limit=5" \
  -H "Authorization: Bearer $TOKEN")
[ "$HTTP_CODE" = "200" ] && pass "Export NDJSON → 200" || fail "Export: $HTTP_CODE"

# ── Frontend Proxy ─────────────────────────────────────────────────────────────
echo ""
echo "--- Frontend → Backend Proxy ---"
curl -sf http://localhost:28080/api/health | grep -q '"ok"' \
  && pass "Frontend /api/health proxied" || fail "Frontend proxy broken"

curl -sf -X POST http://localhost:28080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}' \
  | grep -q '"token"' && pass "Frontend /api/auth/login proxied" || fail "Frontend proxy login"

# ── Agents CRD ─────────────────────────────────────────────────────────────────
echo ""
echo "--- Agents CRD ---"
kubectl get crd agents.aidevops.io > /dev/null 2>&1 && pass "Agent CRD installed" || fail "Agent CRD missing"
curl -sf http://localhost:28081/api/agents -H "Authorization: Bearer $TOKEN" \
  | grep -q '"success":true' && pass "GET /api/agents → success" || fail "agents API"

# ── Cleanup test data ──────────────────────────────────────────────────────────
echo ""
echo "--- Cleanup ---"
curl -sf http://localhost:28123/ --data "ALTER TABLE audit.traces DELETE WHERE agent_id = 'smoke-test-agent'" \
  && pass "Cleaned up smoke-test-agent traces" || warn "Cleanup failed (non-critical)"

echo ""
echo "========================================"
echo "  All K8s platform smoke tests passed!"
echo "========================================"
