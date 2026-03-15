#!/usr/bin/env bash
#
# migrate-roles-to-pvc.sh — Migrate Role ConfigMaps to PVC directory structure.
#
# Run inside the backend pod (which has the roles-data PVC mounted):
#   kubectl exec -it deploy/control-panel-backend -- /bin/sh -c 'curl -sL <script-url> | bash'
# Or copy and run:
#   kubectl cp scripts/migrate-roles-to-pvc.sh <backend-pod>:/tmp/
#   kubectl exec -it <backend-pod> -- bash /tmp/migrate-roles-to-pvc.sh
#
# Prerequisites:
#   - roles-data PVC mounted at /data/roles in the backend pod
#   - kubectl available (or run inside a pod with the k8s API access)
#
set -euo pipefail

ROLES_DATA_PATH="${ROLES_DATA_PATH:-/data/roles}"
NAMESPACE="${AGENT_NAMESPACE:-aidevops}"

echo "=== Role ConfigMap → PVC Migration ==="
echo "Target directory: ${ROLES_DATA_PATH}"
echo "Namespace: ${NAMESPACE}"
echo

# List all role ConfigMaps
CONFIGMAPS=$(kubectl get configmap -n "${NAMESPACE}" -l entropygen.io/component=role -o jsonpath='{.items[*].metadata.name}')

if [ -z "${CONFIGMAPS}" ]; then
    echo "No role ConfigMaps found. Nothing to migrate."
    exit 0
fi

MIGRATED=0
SKIPPED=0

for CM_NAME in ${CONFIGMAPS}; do
    ROLE_NAME="${CM_NAME#role-}"
    ROLE_DIR="${ROLES_DATA_PATH}/${ROLE_NAME}"

    if [ -d "${ROLE_DIR}" ]; then
        echo "[SKIP] ${ROLE_NAME}: directory already exists at ${ROLE_DIR}"
        SKIPPED=$((SKIPPED + 1))
        continue
    fi

    echo "[MIGRATE] ${ROLE_NAME}"
    mkdir -p "${ROLE_DIR}"

    # Get description from annotation
    DESCRIPTION=$(kubectl get configmap -n "${NAMESPACE}" "${CM_NAME}" -o jsonpath='{.metadata.annotations.entropygen\.io/description}' 2>/dev/null || echo "")
    CREATED_AT=$(kubectl get configmap -n "${NAMESPACE}" "${CM_NAME}" -o jsonpath='{.metadata.creationTimestamp}')
    NOW=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

    # Write .metadata.json
    cat > "${ROLE_DIR}/.metadata.json" <<METAEOF
{
  "description": "${DESCRIPTION}",
  "created_at": "${CREATED_AT}",
  "updated_at": "${NOW}"
}
METAEOF

    # Get all data keys
    KEYS=$(kubectl get configmap -n "${NAMESPACE}" "${CM_NAME}" -o jsonpath='{range .data}{@.key}{"\n"}{end}' 2>/dev/null || true)
    # Alternative: use go-template for keys
    if [ -z "${KEYS}" ]; then
        KEYS=$(kubectl get configmap -n "${NAMESPACE}" "${CM_NAME}" -o go-template='{{range $k, $v := .data}}{{$k}}{{"\n"}}{{end}}')
    fi

    FILE_COUNT=0
    while IFS= read -r KEY; do
        [ -z "${KEY}" ] && continue

        # Translate __ separators back to / for directory structure
        FILE_PATH=$(echo "${KEY}" | sed 's/__/\//g')

        # Create parent directories
        FILE_DIR=$(dirname "${ROLE_DIR}/${FILE_PATH}")
        mkdir -p "${FILE_DIR}"

        # Extract file content
        kubectl get configmap -n "${NAMESPACE}" "${CM_NAME}" -o go-template="{{index .data \"${KEY}\"}}" > "${ROLE_DIR}/${FILE_PATH}"

        echo "  ${KEY} → ${FILE_PATH}"
        FILE_COUNT=$((FILE_COUNT + 1))
    done <<< "${KEYS}"

    echo "  ✓ Migrated ${FILE_COUNT} files"
    MIGRATED=$((MIGRATED + 1))
done

echo
echo "=== Migration Complete ==="
echo "Migrated: ${MIGRATED} roles"
echo "Skipped:  ${SKIPPED} roles (already exist)"
echo
echo "Verify with: ls -la ${ROLES_DATA_PATH}/"

# Verification
echo
echo "=== Verification ==="
for CM_NAME in ${CONFIGMAPS}; do
    ROLE_NAME="${CM_NAME#role-}"
    ROLE_DIR="${ROLES_DATA_PATH}/${ROLE_NAME}"

    CM_KEY_COUNT=$(kubectl get configmap -n "${NAMESPACE}" "${CM_NAME}" -o go-template='{{len .data}}')
    # Count files in directory (excluding .metadata.json)
    FS_FILE_COUNT=$(find "${ROLE_DIR}" -type f ! -name '.metadata.json' | wc -l | tr -d ' ')

    if [ "${CM_KEY_COUNT}" = "${FS_FILE_COUNT}" ]; then
        echo "  ✓ ${ROLE_NAME}: ${CM_KEY_COUNT} files match"
    else
        echo "  ✗ ${ROLE_NAME}: ConfigMap has ${CM_KEY_COUNT} keys, filesystem has ${FS_FILE_COUNT} files"
    fi
done
