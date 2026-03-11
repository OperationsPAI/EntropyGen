#!/usr/bin/env bash
# scripts/init-secrets.sh
# Initialize all required K8S secrets for the AI DevOps Platform.
# Run this once before the first helm install.
# Usage: ./scripts/init-secrets.sh [--namespace NAMESPACE]
set -euo pipefail

NAMESPACE=${NAMESPACE:-control-plane}

log() {
  echo "[init-secrets] $*"
}

# Create namespace if not exists
kubectl get namespace "$NAMESPACE" >/dev/null 2>&1 || \
  kubectl create namespace "$NAMESPACE"

# 1. Gateway JWT signing secret (random 64 bytes)
log "Creating agent-gateway-jwt-secret..."
kubectl create secret generic agent-gateway-jwt-secret \
  --from-literal=jwt-secret="$(openssl rand -base64 64 | tr -d '\n')" \
  -n "$NAMESPACE" \
  --dry-run=client -o yaml | kubectl apply -f -

# 2. Gitea webhook HMAC secret
log "Creating gitea-webhook-secret..."
kubectl create secret generic gitea-webhook-secret \
  --from-literal=secret="$(openssl rand -base64 32 | tr -d '\n')" \
  -n "$NAMESPACE" \
  --dry-run=client -o yaml | kubectl apply -f -

# 3. Gitea admin token (prompt user)
if [ -z "${GITEA_ADMIN_TOKEN:-}" ]; then
  echo ""
  echo "Please provide the Gitea admin API token."
  echo "You can create one in Gitea: Settings -> Applications -> Generate Token"
  read -rsp "Gitea Admin Token: " GITEA_ADMIN_TOKEN
  echo ""
fi
log "Creating gitea-admin-token..."
kubectl create secret generic gitea-admin-token \
  --from-literal=token="$GITEA_ADMIN_TOKEN" \
  -n "$NAMESPACE" \
  --dry-run=client -o yaml | kubectl apply -f -

# 4. ClickHouse credentials
CH_PASSWORD=${CH_PASSWORD:-$(openssl rand -base64 16 | tr -d '\n')}
log "Creating clickhouse-creds..."
kubectl create secret generic clickhouse-creds \
  --from-literal=username="default" \
  --from-literal=password="$CH_PASSWORD" \
  -n "$NAMESPACE" \
  --dry-run=client -o yaml | kubectl apply -f -

log "All secrets initialized successfully in namespace: $NAMESPACE"
log ""
log "Next steps:"
log "  1. Apply CRDs:  kubectl apply -f k8s/crds/"
log "  2. Apply RBAC:  kubectl apply -f k8s/rbac/"
log "  3. Deploy:      helm install aidevops-platform k8s/helm/ -n $NAMESPACE"
