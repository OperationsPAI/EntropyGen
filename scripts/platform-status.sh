#!/usr/bin/env bash
# scripts/platform-status.sh — Single-command platform health snapshot.
# Designed for Claude Code to quickly assess system state at conversation start.
#
# Usage: ./scripts/platform-status.sh [--full]
#   --full  Include logs and data pipeline checks (slower)
set -euo pipefail

NS=${NAMESPACE:-aidevops}
FULL=false
[ "${1:-}" = "--full" ] && FULL=true

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

ok()   { echo -e "  ${GREEN}[OK]${NC}   $*"; }
warn() { echo -e "  ${YELLOW}[WARN]${NC} $*"; }
fail() { echo -e "  ${RED}[FAIL]${NC} $*"; }
info() { echo -e "  ${CYAN}[INFO]${NC} $*"; }
section() { echo -e "\n${CYAN}── $* ──${NC}"; }

# ── 1. Cluster & Namespace ───────────────────────────────────────────────────
section "Cluster"
if ! kubectl cluster-info --request-timeout=5s &>/dev/null; then
  fail "Cannot reach Kubernetes cluster"
  echo "  Try: ./scripts/minikube-setup.sh start"
  exit 1
fi
CONTEXT=$(kubectl config current-context 2>/dev/null)
ok "Cluster reachable (context: $CONTEXT)"

if ! kubectl get ns "$NS" &>/dev/null; then
  fail "Namespace $NS does not exist"
  echo "  Try: skaffold run -p minikube"
  exit 1
fi

# ── 2. Pod Health ────────────────────────────────────────────────────────────
section "Pods"
PODS=$(kubectl get pods -n "$NS" --no-headers 2>/dev/null | grep -v "Completed")
TOTAL=$(echo "$PODS" | wc -l)
NOT_RUNNING=$(echo "$PODS" | grep -v "Running" | grep -v "^$" || true)
RESTARTS=$(echo "$PODS" | awk '{split($4,a,/[()]/); r=a[1]; if(r+0 > 2) print $1" ("r" restarts)"}')

if [ -z "$NOT_RUNNING" ]; then
  ok "All $TOTAL pods running"
else
  fail "Unhealthy pods:"
  echo "$NOT_RUNNING" | while read -r line; do echo "       $line"; done
fi

if [ -n "$RESTARTS" ]; then
  warn "Pods with restarts:"
  echo "$RESTARTS" | while read -r line; do echo "       $line"; done
fi

# ── 3. Service Endpoints ─────────────────────────────────────────────────────
section "Services"

# Helper: quick port-forward probe. Opens PF, curls, kills PF.
probe_svc() {
  local svc=$1 port=$2 path=$3 expect=$4 label=$5
  local lport=$((29000 + RANDOM % 1000))
  kubectl port-forward "svc/$svc" "$lport:$port" -n "$NS" >/dev/null 2>&1 &
  local pf_pid=$!
  sleep 1
  if curl -sf --max-time 3 "http://localhost:$lport$path" 2>/dev/null | grep -q "$expect"; then
    ok "$label"
  else
    fail "$label"
  fi
  kill $pf_pid 2>/dev/null; wait $pf_pid 2>/dev/null || true
}

probe_svc control-panel-backend 80 /api/health '"ok"' "Backend API healthy"
probe_svc agent-gateway 80 /healthz "ok" "Gateway healthy"

probe_svc gitea 3000 /api/v1/version "version" "Gitea healthy"
probe_svc clickhouse 8123 /ping "Ok" "ClickHouse healthy"

if kubectl -n "$NS" exec redis-0 -- redis-cli ping 2>/dev/null | grep -q "PONG"; then
  ok "Redis healthy"
else
  fail "Redis unreachable"
fi

if kubectl -n "$NS" exec platform-postgres-0 -- pg_isready -U postgres 2>/dev/null | grep -q "accepting"; then
  ok "PostgreSQL healthy"
else
  fail "PostgreSQL unreachable"
fi

# ── 4. CRD & Agents ─────────────────────────────────────────────────────────
section "Agents"
if kubectl get crd agents.aidevops.io &>/dev/null; then
  AGENT_COUNT=$(kubectl get agents -n "$NS" --no-headers 2>/dev/null | wc -l)
  ok "Agent CRD installed ($AGENT_COUNT agents)"
  if [ "$AGENT_COUNT" -gt 0 ]; then
    kubectl get agents -n "$NS" --no-headers 2>/dev/null | while read -r line; do
      NAME=$(echo "$line" | awk '{print $1}')
      ROLE=$(echo "$line" | awk '{print $2}')
      PHASE=$(echo "$line" | awk '{print $3}')
      if [ "$PHASE" = "Running" ]; then
        ok "  $NAME (role=$ROLE, phase=$PHASE)"
      elif [ "$PHASE" = "Paused" ]; then
        info "  $NAME (role=$ROLE, phase=$PHASE)"
      else
        warn "  $NAME (role=$ROLE, phase=$PHASE)"
      fi
    done
  fi
else
  fail "Agent CRD not installed"
fi

# ── 5. Gitea Platform State ──────────────────────────────────────────────────
section "Gitea Platform"
GITEA_TOKEN=$(kubectl -n "$NS" get secret gitea-admin-token -o jsonpath='{.data.token}' 2>/dev/null | base64 -d 2>/dev/null)
if [ -n "$GITEA_TOKEN" ]; then
  # Check org
  if kubectl -n "$NS" exec gitea-0 -- wget -qO- --header="Authorization: token $GITEA_TOKEN" http://localhost:3000/api/v1/orgs/platform 2>/dev/null | grep -q '"username"'; then
    ok "Org 'platform' exists"
  else
    warn "Org 'platform' not found"
  fi

  # Check repo & issues
  REPO_INFO=$(kubectl -n "$NS" exec gitea-0 -- wget -qO- --header="Authorization: token $GITEA_TOKEN" "http://localhost:3000/api/v1/repos/platform/platform-demo" 2>/dev/null || echo "{}")
  if echo "$REPO_INFO" | grep -q '"full_name"'; then
    OPEN_ISSUES=$(echo "$REPO_INFO" | python3 -c "import sys,json; print(json.load(sys.stdin).get('open_issues_count',0))" 2>/dev/null || echo "?")
    ok "Repo 'platform/platform-demo' exists (open_issues=$OPEN_ISSUES)"
  else
    warn "Repo 'platform/platform-demo' not found"
  fi

  # Open PRs
  OPEN_PRS=$(kubectl -n "$NS" exec gitea-0 -- wget -qO- --header="Authorization: token $GITEA_TOKEN" "http://localhost:3000/api/v1/repos/platform/platform-demo/pulls?state=open&limit=50" 2>/dev/null \
    | python3 -c "import sys,json; print(len(json.load(sys.stdin)))" 2>/dev/null || echo "?")
  info "Open PRs: $OPEN_PRS"
else
  warn "Gitea admin token not available"
fi

# ── 6. Data Pipeline (--full only) ───────────────────────────────────────────
if $FULL; then
  section "Data Pipeline"

  # Redis stream lengths
  for STREAM in events:gateway events:gitea events:k8s; do
    LEN=$(kubectl -n "$NS" exec redis-0 -- redis-cli XLEN "$STREAM" 2>/dev/null || echo "?")
    info "Redis $STREAM: $LEN entries"
  done

  # Agent-specific streams
  AGENT_STREAMS=$(kubectl -n "$NS" exec redis-0 -- redis-cli KEYS "events:agent-*" 2>/dev/null || echo "")
  if [ -n "$AGENT_STREAMS" ]; then
    echo "$AGENT_STREAMS" | while read -r S; do
      [ -z "$S" ] && continue
      LEN=$(kubectl -n "$NS" exec redis-0 -- redis-cli XLEN "$S" 2>/dev/null || echo "?")
      info "Redis $S: $LEN entries"
    done
  fi

  # ClickHouse trace count
  TRACE_COUNT=$(kubectl -n "$NS" exec clickhouse-0 -- clickhouse-client --query "SELECT count() FROM audit.traces" 2>/dev/null || echo "?")
  info "ClickHouse audit.traces: $TRACE_COUNT rows"

  # Recent traces (last hour)
  RECENT=$(kubectl -n "$NS" exec clickhouse-0 -- clickhouse-client --query "SELECT count() FROM audit.traces WHERE created_at > now() - INTERVAL 1 HOUR" 2>/dev/null || echo "?")
  info "Traces in last hour: $RECENT"

  # ── 7. Recent Errors ─────────────────────────────────────────────────────
  section "Recent Errors (last 50 log lines)"
  for DEPLOY in control-panel-backend devops-operator agent-gateway event-collector; do
    ERRORS=$(kubectl -n "$NS" logs "deploy/$DEPLOY" --tail=50 2>/dev/null | { grep -iE "error|fatal|panic" || true; } | tail -3)
    if [ -n "$ERRORS" ]; then
      warn "$DEPLOY:"
      echo "$ERRORS" | while read -r line; do echo "       $line"; done
    fi
  done
fi

section "Summary"
FAIL_COUNT=0
while IFS= read -r line; do
  FAIL_COUNT=$((FAIL_COUNT + 1))
done < <(kubectl get pods -n "$NS" --no-headers 2>/dev/null | grep -v "Running\|Completed" | grep . || true)
if [ "$FAIL_COUNT" -eq 0 ]; then
  ok "Platform healthy"
else
  fail "$FAIL_COUNT pod(s) not healthy"
fi
