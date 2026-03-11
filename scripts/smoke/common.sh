#!/usr/bin/env bash
# scripts/smoke/common.sh
# Smoke test for internal/common package dependencies.
# Verifies Redis, ClickHouse, and Gitea are reachable and functional.
set -euo pipefail

REDIS_PORT=${REDIS_PORT:-6380}
CH_HTTP=${CH_HTTP:-http://localhost:8123}
GITEA_URL=${GITEA_URL:-http://localhost:3000}

pass() { echo "  [PASS] $*"; }
fail() { echo "  [FAIL] $*"; exit 1; }

echo "=== Smoke Test: common package dependencies ==="

# Test Redis (use docker exec since redis-cli may not be installed on host)
echo ""
echo "--- Redis ---"
docker exec aidevops-redis redis-cli ping | grep -q "PONG" && pass "Redis ping" || fail "Redis ping failed"
docker exec aidevops-redis redis-cli XADD test:smoke '*' key val | grep -qE "^[0-9]+-[0-9]+$" && pass "Redis XADD" || fail "Redis XADD failed"
docker exec aidevops-redis redis-cli DEL test:smoke >/dev/null

# Test ClickHouse
echo ""
echo "--- ClickHouse ---"
curl -sf "$CH_HTTP/ping" | grep -q "Ok" && pass "ClickHouse ping" || fail "ClickHouse ping failed"
curl -sf "$CH_HTTP/?query=SELECT%201" | grep -q "1" && pass "ClickHouse query" || fail "ClickHouse query failed"

# Test Gitea
echo ""
echo "--- Gitea ---"
curl -sf "$GITEA_URL/api/v1/version" | grep -q "version" && pass "Gitea API" || fail "Gitea API failed"

echo ""
echo "=== All smoke tests passed ==="
