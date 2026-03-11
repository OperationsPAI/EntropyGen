#!/usr/bin/env bash
# scripts/skaffold/post-deploy.sh
# Skaffold post-deploy hook: verify platform, run smoke test.
set -euo pipefail

NS=${NAMESPACE:-aidevops}

log() { echo "[post-deploy] $*"; }

log "Waiting for deployments..."
for DEPLOY in agent-gateway control-panel-backend control-panel-frontend devops-operator event-collector; do
  kubectl rollout status "deploy/$DEPLOY" -n "$NS" --timeout=120s 2>&1 | tail -1
done

log "Pod status:"
kubectl get pods -n "$NS" --no-headers | awk '{printf "  %-45s %s\n", $1, $2}'

log "Running backend smoke test..."
# Port-forward, run smoke, cleanup
kubectl port-forward svc/control-panel-backend 28081:80 -n "$NS" > /dev/null 2>&1 &
PF=$!
sleep 3

if BACKEND_URL=http://localhost:28081 ./scripts/smoke/backend.sh; then
  log "Smoke test passed!"
else
  log "WARNING: Smoke test failed"
fi

kill $PF 2>/dev/null || true
log "Post-deploy complete"
