#!/bin/bash
set -euo pipefail

# Copy config files from read-only ConfigMap mount to writable home directory
if [ -d /agent/config ]; then
    cp -f /agent/config/openclaw.json ~/.openclaw/openclaw.json 2>/dev/null || true
    cp -f /agent/config/SOUL.md ~/.openclaw/SOUL.md 2>/dev/null || true
    cp -f /agent/config/AGENTS.md ~/.openclaw/AGENTS.md 2>/dev/null || true
    cp -f /agent/config/cron-config.json ~/.openclaw/cron-config.json 2>/dev/null || true
fi

# Copy skills from read-only ConfigMap mount
if [ -d /agent/skills ]; then
    mkdir -p ~/.openclaw/skills
    cp -rf /agent/skills/* ~/.openclaw/skills/ 2>/dev/null || true
fi

# Copy role-specific extra files
if [ -d /agent/role ]; then
    cp -f /agent/role/* ~/.openclaw/ 2>/dev/null || true
fi

# Substitute template variables in SOUL.md
if [ -f ~/.openclaw/SOUL.md ]; then
    sed -i "s/{{AGENT_ID}}/${AGENT_ID:-unknown}/g" ~/.openclaw/SOUL.md
    sed -i "s/{{AGENT_ROLE}}/${AGENT_ROLE:-agent}/g" ~/.openclaw/SOUL.md
fi

# Start openclaw gateway
if command -v openclaw &> /dev/null; then
    # Generate a random gateway token if not provided via env
    OPENCLAW_GATEWAY_TOKEN="${OPENCLAW_GATEWAY_TOKEN:-$(head -c 16 /dev/urandom | base64)}"
    export OPENCLAW_GATEWAY_TOKEN

    exec openclaw gateway run \
        --port 8080 \
        --bind lan \
        --allow-unconfigured \
        --token "$OPENCLAW_GATEWAY_TOKEN"
else
    echo "openclaw not found, starting sleep for debugging"
    exec sleep infinity
fi
