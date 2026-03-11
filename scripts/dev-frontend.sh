#!/usr/bin/env bash
# scripts/dev-frontend.sh
# Start the Control Panel frontend dev server.
#
# Usage:
#   ./scripts/dev-frontend.sh
#
# Opens: http://localhost:3000
# (proxies /api/* → http://localhost:8080)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
FRONTEND_DIR="$PROJECT_ROOT/frontend"

if [ ! -d "$FRONTEND_DIR/node_modules" ]; then
  echo "[dev-frontend] node_modules not found, running npm install..."
  cd "$FRONTEND_DIR"
  npm install --legacy-peer-deps
fi

cd "$FRONTEND_DIR"
echo "[dev-frontend] starting on http://localhost:3000"
exec npm run dev -- --port 3000
