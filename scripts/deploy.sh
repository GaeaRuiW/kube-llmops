#!/usr/bin/env bash
set -euo pipefail

# kube-llmops deploy script
# Handles Helm upgrade + workarounds for known Helm SSA issues
#
# Usage:
#   ./scripts/deploy.sh -f values-minimal.yaml
#   ./scripts/deploy.sh -f my-values.yaml --set vllm.models[0].name=my-model

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHART_DIR="${SCRIPT_DIR}/../charts/kube-llmops-stack"
RELEASE_NAME="${KUBE_LLMOPS_RELEASE:-kube-llmops}"
NAMESPACE="${KUBE_LLMOPS_NAMESPACE:-default}"

echo "============================================="
echo "  kube-llmops deploy"
echo "============================================="
echo "  Release:   ${RELEASE_NAME}"
echo "  Namespace: ${NAMESPACE}"
echo "  Chart:     ${CHART_DIR}"
echo ""

# Step 1: Delete conflicting ConfigMaps (Helm SSA workaround)
echo "[1/4] Cleaning up stale ConfigMaps..."
kubectl delete cm "${RELEASE_NAME}-litellm-config" -n "${NAMESPACE}" 2>/dev/null && echo "  Deleted litellm-config" || true

# Step 2: Helm upgrade
echo "[2/4] Running helm upgrade..."
helm upgrade --install "${RELEASE_NAME}" "${CHART_DIR}" \
  -n "${NAMESPACE}" --create-namespace \
  "$@"

echo ""

# Step 3: Force-apply deployments that Helm SSA often fails to update
echo "[3/4] Force-applying deployments (Helm SSA workaround)..."
for SUBPATH in \
  "charts/observability/templates/grafana.yaml" \
  "charts/observability/templates/prometheus.yaml" \
  "charts/langfuse/templates/deployment.yaml" \
  "charts/litellm/templates/configmap.yaml" \
; do
  if helm template "${RELEASE_NAME}" "${CHART_DIR}" "$@" \
    --show-only "${SUBPATH}" 2>/dev/null | grep -q "kind:"; then
    helm template "${RELEASE_NAME}" "${CHART_DIR}" "$@" \
      --show-only "${SUBPATH}" 2>/dev/null | \
      kubectl apply --force -f - -n "${NAMESPACE}" 2>/dev/null && \
      echo "  Applied ${SUBPATH}" || true
  fi
done

# Step 4: Restart deployments that use subPath mounts (no hot-reload)
echo "[4/4] Restarting services with config changes..."
kubectl rollout restart deployment "${RELEASE_NAME}-litellm" -n "${NAMESPACE}" 2>/dev/null || true
# Note: Prometheus and Grafana auto-restart due to pod template changes in step 3

echo ""
echo "============================================="
echo "  Deploy complete!"
echo "============================================="
echo ""
echo "Check status: kubectl get pods -n ${NAMESPACE}"
