#!/bin/bash
# Common git setup for all runtime adapters.
# Sources: AGENT_ID, GITEA_BASE_URL, GITEA_TOKEN_FILE env vars.
set -euo pipefail

git config --global user.name "${AGENT_ID:-agent}"
git config --global user.email "${AGENT_ID:-agent}@platform.local"

if [ -f "${GITEA_TOKEN_FILE:-/agent/secrets/gitea-token}" ]; then
    TOKEN=$(cat "${GITEA_TOKEN_FILE:-/agent/secrets/gitea-token}")
    GITEA_URL="${GITEA_BASE_URL:-http://gitea.aidevops.svc:3000}"
    GITEA_URL=$(echo "$GITEA_URL" | sed 's|/api/v1$||')
    CRED_URL=$(echo "$GITEA_URL" | sed "s|://|://${AGENT_ID:-agent}:${TOKEN}@|")
    echo "$CRED_URL" > ~/.git-credentials
    chmod 600 ~/.git-credentials
    git config --global credential.helper store
fi
