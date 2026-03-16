#!/bin/bash
set -euo pipefail

# Import a GitHub repository into Gitea via the migration API.
#
# Usage:
#   ./scripts/import-github-repo.sh <github-url> [gitea-org]
#
# Examples:
#   ./scripts/import-github-repo.sh https://github.com/GoogleCloudPlatform/microservices-demo
#   ./scripts/import-github-repo.sh https://github.com/GoogleCloudPlatform/microservices-demo myorg
#
# Environment:
#   GITEA_URL          Gitea base URL         (default: http://10.10.10.240:30030)
#   GITEA_ADMIN_TOKEN  Admin API token        (default: read from k8s secret)
#   GITEA_ORG          Target organization    (default: platform)

GITHUB_URL="${1:?Usage: $0 <github-url> [gitea-org]}"
GITEA_ORG="${2:-${GITEA_ORG:-platform}}"

# Strip trailing slashes and .git suffix for parsing
GITHUB_URL="${GITHUB_URL%/}"
GITHUB_URL="${GITHUB_URL%.git}"

# Extract owner/repo from GitHub URL
# Supports: https://github.com/owner/repo or github.com/owner/repo
REPO_PATH="${GITHUB_URL#*github.com/}"
GITHUB_OWNER="${REPO_PATH%%/*}"
REPO_NAME="${REPO_PATH#*/}"

if [[ -z "$GITHUB_OWNER" || -z "$REPO_NAME" || "$REPO_PATH" == "$GITHUB_URL" ]]; then
    echo "Error: could not parse GitHub URL: $GITHUB_URL" >&2
    echo "Expected format: https://github.com/owner/repo" >&2
    exit 1
fi

# Resolve Gitea connection
GITEA_URL="${GITEA_URL:-http://10.10.10.220:30030}"
if [[ -z "${GITEA_ADMIN_TOKEN:-}" ]]; then
    GITEA_ADMIN_TOKEN=$(kubectl get secret gitea-admin-token -n aidevops -o jsonpath='{.data.token}' | base64 -d)
fi

GITEA_API="${GITEA_URL}/api/v1"

echo "GitHub:  ${GITHUB_OWNER}/${REPO_NAME}"
echo "Gitea:   ${GITEA_ORG}/${REPO_NAME}"
echo "API:     ${GITEA_API}"
echo ""

# Check if repo already exists
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Authorization: token ${GITEA_ADMIN_TOKEN}" \
    "${GITEA_API}/repos/${GITEA_ORG}/${REPO_NAME}")

if [[ "$HTTP_CODE" == "200" ]]; then
    echo "Repository ${GITEA_ORG}/${REPO_NAME} already exists in Gitea, skipping."
    exit 0
fi

# Ensure org exists
ORG_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Authorization: token ${GITEA_ADMIN_TOKEN}" \
    "${GITEA_API}/orgs/${GITEA_ORG}")

if [[ "$ORG_CODE" != "200" ]]; then
    echo "Creating organization: ${GITEA_ORG}"
    curl -sf -X POST \
        -H "Authorization: token ${GITEA_ADMIN_TOKEN}" \
        -H "Content-Type: application/json" \
        -d "{\"username\": \"${GITEA_ORG}\", \"visibility\": \"public\"}" \
        "${GITEA_API}/orgs" > /dev/null
fi

# Migrate (import) from GitHub
echo "Importing ${GITHUB_OWNER}/${REPO_NAME} → ${GITEA_ORG}/${REPO_NAME} ..."

RESPONSE=$(curl -sf -X POST \
    -H "Authorization: token ${GITEA_ADMIN_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "{
        \"clone_addr\": \"https://github.com/${GITHUB_OWNER}/${REPO_NAME}.git\",
        \"repo_name\": \"${REPO_NAME}\",
        \"repo_owner\": \"${GITEA_ORG}\",
        \"service\": \"github\",
        \"mirror\": false,
        \"issues\": false,
        \"labels\": false,
        \"milestones\": false,
        \"pull_requests\": false,
        \"releases\": false,
        \"wiki\": false
    }" \
    "${GITEA_API}/repos/migrate" 2>&1) || {
    echo "Error: migration failed" >&2
    echo "$RESPONSE" >&2
    exit 1
}

CLONE_URL=$(echo "$RESPONSE" | python3 -c "import sys,json; print(json.loads(sys.stdin.read()).get('clone_url',''))" 2>/dev/null || true)

echo ""
echo "Done! Repository imported:"
echo "  ${GITEA_URL}/${GITEA_ORG}/${REPO_NAME}"
if [[ -n "$CLONE_URL" ]]; then
    echo "  Clone: ${CLONE_URL}"
fi
