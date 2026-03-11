#!/usr/bin/env bash
# scripts/dev-down.sh
# Stop the local development environment.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_ROOT"

echo "[dev-down] Stopping development services..."
docker compose -f docker-compose.dev.yml down

echo "[dev-down] Services stopped. Data volumes preserved."
echo "[dev-down] To remove volumes: docker compose -f docker-compose.dev.yml down -v"
