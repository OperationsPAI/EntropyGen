#!/bin/bash
set -euo pipefail

# Copy config files from read-only ConfigMap mount to writable home directory
[ -d /agent/config ] && cp -f /agent/config/* ~/.openclaw/ 2>/dev/null || true

# Copy skills from read-only ConfigMap mount
[ -d /agent/skills ] && mkdir -p ~/.openclaw/skills && cp -rf /agent/skills/* ~/.openclaw/skills/ 2>/dev/null || true

# Copy role-specific extra files
[ -d /agent/role ] && cp -f /agent/role/* ~/.openclaw/ 2>/dev/null || true

# Override openclaw workspace files with platform templates
# This prevents openclaw defaults (SOUL.md, AGENTS.md, USER.md, etc.) from
# conflicting with our injected prompt system.
if [ -d /agent/workspace-templates ]; then
    mkdir -p ~/.openclaw/workspace
    for f in /agent/workspace-templates/*; do
        [ -f "$f" ] && cp -f "$f" ~/.openclaw/workspace/
    done
    # Template substitution for workspace files
    # Use | as sed delimiter because values contain / (e.g. org/repo, http://...)
    for f in ~/.openclaw/workspace/*.md; do
        [ -f "$f" ] && sed -i \
            "s|{{AGENT_ID}}|${AGENT_ID:-unknown}|g; \
             s|{{AGENT_ROLE}}|${AGENT_ROLE:-agent}|g; \
             s|{{REPOS}}|${AGENT_REPOS:-}|g; \
             s|{{GITEA_URL}}|${GITEA_BASE_URL:-http://gitea.aidevops.svc:3000}|g" \
            "$f"
    done
    # Remove BOOTSTRAP.md to skip openclaw onboarding flow
    rm -f ~/.openclaw/workspace/BOOTSTRAP.md
fi

# Substitute template variables in SOUL.md
[ -f ~/.openclaw/SOUL.md ] && sed -i "s|{{AGENT_ID}}|${AGENT_ID:-unknown}|g; s|{{AGENT_ROLE}}|${AGENT_ROLE:-agent}|g" ~/.openclaw/SOUL.md

# Substitute template variables in PROMPT.md
[ -f ~/.openclaw/PROMPT.md ] && sed -i \
    "s|{{AGENT_ID}}|${AGENT_ID:-unknown}|g; \
     s|{{AGENT_ROLE}}|${AGENT_ROLE:-agent}|g; \
     s|{{REPOS}}|${AGENT_REPOS:-}|g; \
     s|{{GITEA_URL}}|${GITEA_BASE_URL:-http://gitea.aidevops.svc:3000}|g" \
    ~/.openclaw/PROMPT.md

# Configure git identity
git config --global user.name "${AGENT_ID:-agent}"
git config --global user.email "${AGENT_ID:-agent}@platform.local"

# Configure git credentials for Gitea access
# Uses git credential store with the agent's Gitea token for HTTP push/pull.
if [ -f /agent/secrets/gitea-token ]; then
    GITEA_TOKEN=$(cat /agent/secrets/gitea-token)
    GITEA_URL="${GITEA_BASE_URL:-http://gitea.aidevops.svc:3000}"
    GITEA_URL=$(echo "$GITEA_URL" | sed 's|/api/v1$||')

    # Write credentials in git-credential-store format: http://user:pass@host
    CRED_URL=$(echo "$GITEA_URL" | sed "s|://|://${AGENT_ID:-agent}:${GITEA_TOKEN}@|")
    echo "$CRED_URL" > ~/.git-credentials
    chmod 600 ~/.git-credentials
    git config --global credential.helper store
fi

# Generate gateway token
OPENCLAW_GATEWAY_TOKEN="${OPENCLAW_GATEWAY_TOKEN:-$(head -c 16 /dev/urandom | base64)}"
export OPENCLAW_GATEWAY_TOKEN

# Start observer (message poller + session viewer) in background
agent-observer &

# Start openclaw gateway (cron is handled externally via observer)
exec openclaw gateway run --port 8080 --bind lan --allow-unconfigured --token "$OPENCLAW_GATEWAY_TOKEN"
