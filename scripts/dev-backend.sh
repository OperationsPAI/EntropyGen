#!/usr/bin/env bash
# scripts/dev-backend.sh
# Start the Control Panel backend for local development.
#
# Usage:
#   ./scripts/dev-backend.sh          # foreground
#   ./scripts/dev-backend.sh &        # background
#
# Prerequisites:
#   ./scripts/dev-up.sh               # Redis + ClickHouse must be running
#
# Login: admin / admin
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# ── Credentials ────────────────────────────────────────────────────────────────
# Password hash for "admin" (bcrypt cost 10)
# To change password, regenerate with:
#   go run scripts/genhash.go <new-password>
export ADMIN_USERNAME=admin
export ADMIN_PASSWORD_HASH='$2a$10$pUQzGZFY0DvC1GfO.RYjmeX79uXUL7mqYruyEwVjGzp4RdtGWxgIe'
export JWT_SECRET=MX3qEB4EPUJ/GsifHZSp3TriwnTqB+yp5xugPCHy17+mrmSgPXQo/7V8MGVgACRJ

# ── Infrastructure ─────────────────────────────────────────────────────────────
export REDIS_ADDR=localhost:6380
export CLICKHOUSE_ADDR=localhost:9000
export CLICKHOUSE_DB=audit
export CLICKHOUSE_USER=default
export CLICKHOUSE_PASS=

# ── Optional / disabled locally ────────────────────────────────────────────────
export LITELLM_ADDR=http://localhost:4000   # not required; LLM health check will fail gracefully
export GITEA_ADDR=http://localhost:3000

# ── Server ─────────────────────────────────────────────────────────────────────
export LISTEN_ADDR=:8080
export GIN_MODE=debug

cd "$PROJECT_ROOT"
echo "[dev-backend] starting on http://localhost:8080  (admin / admin)"
exec go run ./cmd/backend/...
