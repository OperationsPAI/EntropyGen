#!/bin/bash
set -euo pipefail

# Read JWT token from secret file and configure openclaw
if [ -f /agent/secrets/jwt-token ]; then
    JWT_TOKEN=$(cat /agent/secrets/jwt-token)
    if [ -f ~/.openclaw/openclaw.json ]; then
        sed -i "s|__JWT_PLACEHOLDER__|${JWT_TOKEN}|g" ~/.openclaw/openclaw.json
    fi
fi

# Substitute template variables in SOUL.md
if [ -f ~/.openclaw/SOUL.md ]; then
    sed -i "s/{{AGENT_ID}}/${AGENT_ID:-unknown}/g" ~/.openclaw/SOUL.md
    sed -i "s/{{AGENT_ROLE}}/${AGENT_ROLE:-agent}/g" ~/.openclaw/SOUL.md
fi

# Start openclaw if available, otherwise just sleep (for testing)
if command -v openclaw &> /dev/null; then
    exec openclaw gateway --headless --port 18789 --health-port 8080
else
    echo "openclaw not found, starting sleep for debugging"
    exec sleep infinity
fi
