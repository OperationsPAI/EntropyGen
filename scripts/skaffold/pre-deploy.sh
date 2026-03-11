#!/usr/bin/env bash
# scripts/skaffold/pre-deploy.sh
# Skaffold pre-deploy hook: ensure namespace, infra, secrets, CRDs are ready.
set -euo pipefail

NS=${NAMESPACE:-aidevops}
GITEA_ADMIN_PASS=${GITEA_ADMIN_PASS:-gitea-admin123}
ADMIN_HASH='$2a$10$pUQzGZFY0DvC1GfO.RYjmeX79uXUL7mqYruyEwVjGzp4RdtGWxgIe'

log() { echo "[pre-deploy] $*"; }

# ── Namespace + Helm labels ────────────────────────────────────────────────────
log "Ensuring namespace $NS..."
kubectl create ns "$NS" 2>/dev/null || true
kubectl label ns "$NS" \
  app.kubernetes.io/managed-by=Helm \
  app.kubernetes.io/name=aidevops-platform \
  app.kubernetes.io/instance=aidevops \
  --overwrite > /dev/null 2>&1
kubectl annotate ns "$NS" \
  meta.helm.sh/release-name=aidevops \
  meta.helm.sh/release-namespace="$NS" \
  --overwrite > /dev/null 2>&1

# ── Infrastructure (Redis, ClickHouse, Gitea) ─────────────────────────────────
log "Deploying infrastructure..."
kubectl apply -f k8s/infra/all-in-one.yaml 2>&1 | grep -cE "created|configured|unchanged" | xargs -I{} echo "  {} resources applied"

# Gitea Service (may be lost due to volumeClaimTemplates YAML separator issue)
kubectl get svc gitea -n "$NS" > /dev/null 2>&1 || \
  kubectl apply -n "$NS" -f - <<'EOF'
apiVersion: v1
kind: Service
metadata:
  name: gitea
spec:
  selector:
    app: gitea
  ports:
  - name: http
    port: 3000
    targetPort: 3000
  - name: ssh
    port: 22
    targetPort: 22
EOF

log "Waiting for infra pods..."
for POD in redis-0 clickhouse-0 gitea-0; do
  kubectl wait --for=condition=Ready "pod/$POD" -n "$NS" --timeout=180s 2>/dev/null \
    && echo "  $POD ready" || echo "  WARNING: $POD not ready"
done

# ── Gitea admin + token ───────────────────────────────────────────────────────
log "Initializing Gitea admin..."
kubectl exec gitea-0 -n "$NS" -- su -c \
  "gitea admin user create --username gitea-admin --password $GITEA_ADMIN_PASS --email admin@devops.local --admin --must-change-password=false 2>/dev/null || true" \
  git 2>/dev/null

GITEA_TOKEN=$(kubectl exec redis-0 -n "$NS" -- wget -q -O- \
  --header="Content-Type: application/json" \
  --post-data='{"name":"platform-token","scopes":["write:admin","write:notification","write:organization","write:issue","write:repository","write:user"]}' \
  "http://gitea-admin:${GITEA_ADMIN_PASS}@gitea:3000/api/v1/users/gitea-admin/tokens" 2>/dev/null \
  | python3 -c "import sys,json; print(json.load(sys.stdin).get('sha1',''))" 2>/dev/null || echo "")
[ -n "$GITEA_TOKEN" ] && log "Gitea token: ${GITEA_TOKEN:0:8}..." || log "WARNING: token may already exist"

# ── Secrets ───────────────────────────────────────────────────────────────────
log "Creating secrets..."
JWT_SECRET=$(openssl rand -base64 64 | tr -d '\n')

for NAME_ARGS in \
  "agent-gateway-jwt-secret --from-literal=jwt-secret=$JWT_SECRET" \
  "backend-admin-secret --from-literal=password-hash=$ADMIN_HASH" \
  "gitea-admin-token --from-literal=token=${GITEA_TOKEN:-placeholder}" \
  "gitea-webhook-secret --from-literal=secret=$(openssl rand -base64 32 | tr -d '\n')"; do
  # shellcheck disable=SC2086
  kubectl create secret generic $NAME_ARGS \
    -n "$NS" --dry-run=client -o yaml | kubectl apply -f - > /dev/null 2>&1
done

# ── CRDs ──────────────────────────────────────────────────────────────────────
log "Applying CRDs..."
kubectl apply -f k8s/crds/ > /dev/null 2>&1

# ── Backend DLQ PVC (needs StorageClass) ──────────────────────────────────────
kubectl get pvc backend-dlq -n "$NS" > /dev/null 2>&1 || \
  kubectl apply -n "$NS" -f - <<'EOF'
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: backend-dlq
spec:
  accessModes: [ReadWriteOnce]
  storageClassName: openebs-hostpath
  resources:
    requests:
      storage: 1Gi
EOF

log "Pre-deploy complete"
