#!/usr/bin/env bash
# scripts/observe.sh — Observe runtime behavior of the platform.
# Designed for Claude Code to diagnose issues and understand system behavior.
#
# Usage:
#   ./scripts/observe.sh logs [component]      # Tail logs (all or specific component)
#   ./scripts/observe.sh errors                # Show recent errors across all components
#   ./scripts/observe.sh events                # Show recent K8s events
#   ./scripts/observe.sh agents                # Show agent CRs and their pod status
#   ./scripts/observe.sh gitea                 # Show Gitea issues/PRs state
#   ./scripts/observe.sh pipeline              # Show data pipeline health (Redis → ClickHouse)
#   ./scripts/observe.sh resources             # Show resource usage
set -euo pipefail

NS=${NAMESPACE:-aidevops}
CMD="${1:-help}"
ARG="${2:-}"

CYAN='\033[0;36m'
NC='\033[0m'
section() { echo -e "\n${CYAN}── $* ──${NC}"; }

case "$CMD" in

logs)
  if [ -n "$ARG" ]; then
    kubectl -n "$NS" logs "deploy/$ARG" --tail=100 -f 2>/dev/null \
      || kubectl -n "$NS" logs "$ARG" --tail=100 -f
  else
    # Interleave recent logs from all components
    for DEPLOY in control-panel-backend devops-operator agent-gateway event-collector; do
      section "$DEPLOY (last 20 lines)"
      kubectl -n "$NS" logs "deploy/$DEPLOY" --tail=20 2>/dev/null || echo "(not found)"
    done
  fi
  ;;

errors)
  section "Errors in last 200 log lines"
  FOUND_ERRORS=false
  for DEPLOY in control-panel-backend devops-operator agent-gateway event-collector litellm; do
    ERRORS=$(kubectl -n "$NS" logs "deploy/$DEPLOY" --tail=200 2>/dev/null \
      | { grep -iE "error|fatal|panic|exception" || true; } \
      | { grep -v "no error\|errors=0\|error_count.*0\|without error" || true; } \
      | tail -10)
    if [ -n "$ERRORS" ]; then
      FOUND_ERRORS=true
      section "$DEPLOY"
      echo "$ERRORS"
    fi
  done
  if ! $FOUND_ERRORS; then
    echo "  (no errors found)"
  fi

  section "Pod Events (warnings only)"
  kubectl get events -n "$NS" --field-selector type=Warning --sort-by='.lastTimestamp' 2>/dev/null | tail -15 || true
  ;;

events)
  section "Recent K8s Events"
  kubectl get events -n "$NS" --sort-by='.lastTimestamp' 2>/dev/null | tail -30
  ;;

agents)
  section "Agent CRs"
  kubectl get agents -n "$NS" -o wide 2>/dev/null || echo "No agents found"

  section "Agent Pods"
  kubectl get pods -n "$NS" -l app.kubernetes.io/component=agent -o wide 2>/dev/null || echo "No agent pods"

  # Show agent details if any exist
  AGENTS=$(kubectl get agents -n "$NS" --no-headers 2>/dev/null | awk '{print $1}')
  if [ -n "$AGENTS" ]; then
    for A in $AGENTS; do
      section "Agent: $A"
      kubectl get agent "$A" -n "$NS" -o jsonpath='{
  "role": "{.spec.role}",
  "phase": "{.status.phase}",
  "paused": "{.spec.paused}",
  "cron": "{.spec.cron.schedule}",
  "pod": "{.status.podName}",
  "currentTask": "{.status.currentTask}"
}' 2>/dev/null
      echo ""
    done
  fi
  ;;

gitea)
  GITEA_TOKEN=$(kubectl -n "$NS" get secret gitea-admin-token -o jsonpath='{.data.token}' 2>/dev/null | base64 -d 2>/dev/null)
  if [ -z "$GITEA_TOKEN" ]; then
    echo "Gitea admin token not available"
    exit 1
  fi

  GITEA_API="kubectl -n $NS exec gitea-0 -- wget -qO- --header=Authorization:\ token\ $GITEA_TOKEN"

  section "Repositories"
  eval $GITEA_API "http://localhost:3000/api/v1/orgs/platform/repos?limit=10" 2>/dev/null \
    | python3 -c "
import sys,json
repos = json.load(sys.stdin)
for r in repos:
    print(f'  {r[\"full_name\"]}  issues={r[\"open_issues_count\"]}  forks={r[\"forks_count\"]}')
" 2>/dev/null || echo "  (failed to list repos)"

  section "Open Issues"
  eval $GITEA_API "http://localhost:3000/api/v1/repos/platform/platform-demo/issues?state=open&type=issues&limit=20" 2>/dev/null \
    | python3 -c "
import sys,json
issues = json.load(sys.stdin)
if not issues:
    print('  (none)')
for i in issues:
    labels = ','.join(l['name'] for l in i.get('labels',[]))
    assignee = i.get('assignee',{})
    assigned = assignee.get('login','unassigned') if assignee else 'unassigned'
    print(f'  #{i[\"number\"]} [{labels or \"no-label\"}] ({assigned}) {i[\"title\"]}')
" 2>/dev/null || echo "  (failed to list issues)"

  section "Open Pull Requests"
  eval $GITEA_API "http://localhost:3000/api/v1/repos/platform/platform-demo/pulls?state=open&limit=20" 2>/dev/null \
    | python3 -c "
import sys,json
prs = json.load(sys.stdin)
if not prs:
    print('  (none)')
for p in prs:
    user = p.get('user',{}).get('login','?')
    print(f'  #{p[\"number\"]} ({user}) {p[\"title\"]}  base={p[\"base\"][\"ref\"]}←{p[\"head\"][\"ref\"]}')
" 2>/dev/null || echo "  (failed to list PRs)"

  section "Gitea Users"
  eval $GITEA_API "http://localhost:3000/api/v1/admin/users?limit=50" 2>/dev/null \
    | python3 -c "
import sys,json
users = json.load(sys.stdin)
for u in users:
    admin = ' (admin)' if u.get('is_admin') else ''
    print(f'  {u[\"login\"]}{admin}  email={u.get(\"email\",\"\")}')
" 2>/dev/null || echo "  (failed to list users)"
  ;;

pipeline)
  section "Redis Streams"
  for STREAM in events:gateway events:gitea events:k8s; do
    LEN=$(kubectl -n "$NS" exec redis-0 -- redis-cli XLEN "$STREAM" 2>/dev/null || echo "?")
    LAST=$(kubectl -n "$NS" exec redis-0 -- redis-cli XREVRANGE "$STREAM" + - COUNT 1 2>/dev/null | head -1 || echo "?")
    echo "  $STREAM: $LEN entries (last: $LAST)"
  done

  # Agent streams
  AGENT_STREAMS=$(kubectl -n "$NS" exec redis-0 -- redis-cli KEYS "events:agent-*" 2>/dev/null | grep -v "^$" || true)
  if [ -n "$AGENT_STREAMS" ]; then
    echo ""
    echo "$AGENT_STREAMS" | while read -r S; do
      [ -z "$S" ] && continue
      LEN=$(kubectl -n "$NS" exec redis-0 -- redis-cli XLEN "$S" 2>/dev/null || echo "?")
      echo "  $S: $LEN entries"
    done
  fi

  section "ClickHouse"
  kubectl -n "$NS" exec clickhouse-0 -- clickhouse-client --query "
    SELECT
      'total' as period, count() as traces, sum(tokens_in + tokens_out) as total_tokens
    FROM audit.traces
    UNION ALL
    SELECT
      'last_1h', count(), sum(tokens_in + tokens_out)
    FROM audit.traces
    WHERE created_at > now() - INTERVAL 1 HOUR
    UNION ALL
    SELECT
      'last_24h', count(), sum(tokens_in + tokens_out)
    FROM audit.traces
    WHERE created_at > now() - INTERVAL 24 HOUR
  " 2>/dev/null || echo "  (ClickHouse query failed)"

  section "Top Agents by Trace Count (last 24h)"
  kubectl -n "$NS" exec clickhouse-0 -- clickhouse-client --query "
    SELECT agent_id, count() as traces, sum(tokens_in) as tokens_in, sum(tokens_out) as tokens_out
    FROM audit.traces
    WHERE created_at > now() - INTERVAL 24 HOUR
    GROUP BY agent_id
    ORDER BY traces DESC
    LIMIT 10
  " 2>/dev/null || echo "  (no data)"
  ;;

resources)
  section "Node Resources"
  kubectl top nodes 2>/dev/null || echo "  (metrics-server may not be ready)"

  section "Pod Resources (namespace=$NS)"
  kubectl top pods -n "$NS" 2>/dev/null || echo "  (metrics-server may not be ready)"

  section "PVC Usage"
  kubectl get pvc -n "$NS" -o custom-columns='NAME:.metadata.name,STATUS:.status.phase,CAPACITY:.status.capacity.storage,STORAGECLASS:.spec.storageClassName' 2>/dev/null
  ;;

*)
  echo "Usage: $0 <command> [args]"
  echo ""
  echo "Commands:"
  echo "  logs [component]   Tail logs (all or: control-panel-backend, devops-operator, etc.)"
  echo "  errors             Show recent errors across all components"
  echo "  events             Show recent K8s events"
  echo "  agents             Show agent CRs and their pod status"
  echo "  gitea              Show Gitea issues/PRs/users state"
  echo "  pipeline           Show data pipeline health (Redis → ClickHouse)"
  echo "  resources          Show resource usage (CPU/memory/PVC)"
  ;;
esac
