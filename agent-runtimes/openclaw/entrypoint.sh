#!/bin/bash
set -euo pipefail

# ---- Common layer ----
source /common/setup-git.sh

# ---- Adapter layer: OpenClaw-specific ----

# Copy prompt files to openclaw home
[ -d /agent/prompt ] && cp -f /agent/prompt/* ~/.openclaw/ 2>/dev/null || true

# Also support legacy /agent/config mount path for backward compat
[ -d /agent/config ] && cp -f /agent/config/* ~/.openclaw/ 2>/dev/null || true

# Copy skills
[ -d /agent/skills ] && mkdir -p ~/.openclaw/skills && cp -rf /agent/skills/* ~/.openclaw/skills/ 2>/dev/null || true

# Copy role-specific extra files
[ -d /agent/role ] && cp -f /agent/role/* ~/.openclaw/ 2>/dev/null || true

# Override openclaw workspace files with platform templates
if [ -d /agent/workspace-templates ]; then
    mkdir -p ~/.openclaw/workspace
    for f in /agent/workspace-templates/*; do
        [ -f "$f" ] && cp -f "$f" ~/.openclaw/workspace/
    done
    for f in ~/.openclaw/workspace/*.md; do
        [ -f "$f" ] && sed -i \
            "s|{{AGENT_ID}}|${AGENT_ID:-unknown}|g; \
             s|{{AGENT_ROLE}}|${AGENT_ROLE:-agent}|g; \
             s|{{REPOS}}|${GITEA_REPOS:-${AGENT_REPOS:-}}|g; \
             s|{{GITEA_URL}}|${GITEA_BASE_URL:-http://gitea.aidevops.svc:3000}|g" \
            "$f"
    done
    rm -f ~/.openclaw/workspace/BOOTSTRAP.md
fi

# Template variable substitution in prompt files
[ -f ~/.openclaw/SOUL.md ] && sed -i "s|{{AGENT_ID}}|${AGENT_ID:-unknown}|g; s|{{AGENT_ROLE}}|${AGENT_ROLE:-agent}|g" ~/.openclaw/SOUL.md
[ -f ~/.openclaw/PROMPT.md ] && sed -i \
    "s|{{AGENT_ID}}|${AGENT_ID:-unknown}|g; \
     s|{{AGENT_ROLE}}|${AGENT_ROLE:-agent}|g; \
     s|{{REPOS}}|${GITEA_REPOS:-${AGENT_REPOS:-}}|g; \
     s|{{GITEA_URL}}|${GITEA_BASE_URL:-http://gitea.aidevops.svc:3000}|g" \
    ~/.openclaw/PROMPT.md

# Generate openclaw.json from generic env vars
LLM_PROVIDER="litellm"
LLM_MODEL_ID="${LLM_MODEL:-gpt-4o}"
# Extract model ID after provider prefix (e.g. "litellm/MiniMax-M2.5" -> "MiniMax-M2.5")
if [[ "$LLM_MODEL_ID" == *"/"* ]]; then
    LLM_PROVIDER="${LLM_MODEL_ID%%/*}"
    LLM_MODEL_ID="${LLM_MODEL_ID#*/}"
fi
FULL_MODEL="${LLM_PROVIDER}/${LLM_MODEL_ID}"

cat > ~/.openclaw/openclaw.json <<EOCFG
{
  "models": {
    "providers": {
      "${LLM_PROVIDER}": {
        "baseUrl": "${LLM_BASE_URL:-${GATEWAY_URL:-http://agent-gateway.control-plane.svc}/v1}",
        "apiKey": "${LLM_API_KEY:-sk-placeholder}",
        "api": "openai-completions",
        "models": [{
          "id": "${LLM_MODEL_ID}",
          "name": "${LLM_MODEL_ID}",
          "reasoning": false,
          "input": ["text"],
          "contextWindow": 128000,
          "maxTokens": 32000
        }]
      }
    }
  },
  "agents": {
    "defaults": {
      "model": { "primary": "${FULL_MODEL}" }
    }
  },
  "gateway": {
    "controlUi": {
      "dangerouslyAllowHostHeaderOriginFallback": true
    },
    "http": {
      "endpoints": {
        "chatCompletions": { "enabled": true }
      }
    }
  }
}
EOCFG

# Generate gateway token
OPENCLAW_GATEWAY_TOKEN="${OPENCLAW_GATEWAY_TOKEN:-$(head -c 16 /dev/urandom | base64)}"
export OPENCLAW_GATEWAY_TOKEN

# Start observer in background
agent-observer &

# Start openclaw gateway
exec openclaw gateway run --port 8080 --bind lan --allow-unconfigured --token "$OPENCLAW_GATEWAY_TOKEN"
