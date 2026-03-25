#!/usr/bin/env bash
# scripts/run-tests.sh — Run all tests in dependency order.
# Designed for Claude Code to verify changes before/after deployment.
#
# Usage:
#   ./scripts/run-tests.sh              # unit tests only (fast, no cluster needed)
#   ./scripts/run-tests.sh --smoke      # unit + smoke tests (requires running cluster)
#   ./scripts/run-tests.sh --all        # unit + smoke + e2e (requires cluster + port-forwards)
set -euo pipefail

# Ensure tools in ~/.local/bin and ~/.local/go/bin are available
export PATH="$HOME/.local/go/bin:$HOME/.local/bin:$PATH"

MODE="${1:-unit}"
NS=${NAMESPACE:-aidevops}
FAILURES=0

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

section() { echo -e "\n${CYAN}══ $* ══${NC}"; }
pass()    { echo -e "  ${GREEN}[PASS]${NC} $*"; }
fail_msg(){ echo -e "  ${RED}[FAIL]${NC} $*"; FAILURES=$((FAILURES + 1)); }
skip()    { echo -e "  ${YELLOW}[SKIP]${NC} $*"; }

# ── 1. Build Check ──────────────────────────────────────────────────────────
section "Build"
if go build ./... 2>&1; then
  pass "go build ./..."
else
  fail_msg "go build failed"
fi

# ── 2. Unit Tests ────────────────────────────────────────────────────────────
section "Unit Tests"
if go test ./internal/... -count=1 -short -timeout 60s 2>&1; then
  pass "go test ./internal/..."
else
  fail_msg "Unit tests failed"
fi

[ "$MODE" = "unit" ] && {
  echo ""
  [ "$FAILURES" -eq 0 ] && echo -e "${GREEN}All checks passed.${NC}" || echo -e "${RED}$FAILURES check(s) failed.${NC}"
  exit "$FAILURES"
}

# ── 3. Smoke Tests (requires cluster) ────────────────────────────────────────
if [ "$MODE" = "--smoke" ] || [ "$MODE" = "--all" ]; then
  section "Smoke Tests"

  if ! kubectl get ns "$NS" &>/dev/null; then
    skip "Cluster not available, skipping smoke tests"
  else
    # Quick pod health check first
    UNHEALTHY=$(kubectl get pods -n "$NS" --no-headers | { grep -v "Running\|Completed" || true; } | { grep -c . || true; })
    if [ "$UNHEALTHY" -gt 0 ]; then
      fail_msg "Cluster has $UNHEALTHY unhealthy pods — fix before smoke testing"
    else
      pass "All pods healthy"

      # Run k8s-platform smoke test
      if bash scripts/smoke/k8s-platform.sh 2>&1; then
        pass "K8s platform smoke test"
      else
        fail_msg "K8s platform smoke test failed"
      fi
    fi
  fi
fi

# ── 4. E2E Tests (requires cluster + test setup) ────────────────────────────
if [ "$MODE" = "--all" ]; then
  section "E2E Tests"

  if ! kubectl get ns "$NS" &>/dev/null; then
    skip "Cluster not available, skipping E2E tests"
  else
    # E2E tests need port-forwards. Set them up.
    cleanup_pf() {
      kill $PF_BACKEND $PF_GITEA $PF_CH 2>/dev/null || true
    }
    trap cleanup_pf EXIT

    kubectl port-forward svc/control-panel-backend 28081:80 -n "$NS" >/dev/null 2>&1 &
    PF_BACKEND=$!
    kubectl port-forward svc/gitea 23000:3000 -n "$NS" >/dev/null 2>&1 &
    PF_GITEA=$!
    kubectl port-forward svc/clickhouse 28123:8123 -n "$NS" >/dev/null 2>&1 &
    PF_CH=$!
    sleep 3

    # Backend E2E
    GITEA_TOKEN=$(kubectl -n "$NS" get secret gitea-admin-token -o jsonpath='{.data.token}' | base64 -d)
    if BACKEND_URL=http://localhost:28081 \
       GITEA_URL=http://localhost:23000 \
       CLICKHOUSE_URL=http://localhost:28123 \
       GITEA_TOKEN="$GITEA_TOKEN" \
       GITEA_TEST_TOKEN="$GITEA_TOKEN" \
       go test ./tests/e2e/... -count=1 -timeout 120s -v 2>&1; then
      pass "E2E tests"
    else
      fail_msg "E2E tests failed"
    fi
  fi
fi

# ── Summary ──────────────────────────────────────────────────────────────────
echo ""
if [ "$FAILURES" -eq 0 ]; then
  echo -e "${GREEN}All checks passed.${NC}"
else
  echo -e "${RED}$FAILURES check(s) failed.${NC}"
fi
exit "$FAILURES"
