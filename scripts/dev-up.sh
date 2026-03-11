#!/usr/bin/env bash
# scripts/dev-up.sh
# Start the local development environment.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

log() { echo "[dev-up] $*"; }

cd "$PROJECT_ROOT"

log "Starting development services..."
docker compose -f docker-compose.dev.yml up -d

log "Waiting for Redis..."
for i in $(seq 1 30); do
  docker exec aidevops-redis redis-cli ping 2>/dev/null | grep -q PONG && break
  sleep 1
done
log "Redis ready"

log "Waiting for ClickHouse..."
for i in $(seq 1 60); do
  curl -sf http://localhost:8123/ping 2>/dev/null | grep -q "Ok" && break
  sleep 2
done
log "ClickHouse ready"

log "Waiting for Gitea..."
for i in $(seq 1 60); do
  curl -sf http://localhost:3000/api/v1/version 2>/dev/null | grep -q "version" && break
  sleep 3
done
log "Gitea ready"

log ""
log "All services are up!"
log "  Redis:      localhost:6380"
log "  ClickHouse: localhost:8123 (HTTP), localhost:9000 (native)"
log "  Gitea:      http://localhost:3000"
log ""
log "Next: Initialize Gitea admin user if first run:"
log "  docker exec -it aidevops-gitea gitea admin user create \\"
log "    --username gitea-admin --password gitea-admin123 \\"
log "    --email admin@devops.local --admin --must-change-password=false"
