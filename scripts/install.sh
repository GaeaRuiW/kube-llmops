#!/usr/bin/env bash
set -euo pipefail

# kube-llmops installer
# Usage: curl -sfL https://raw.githubusercontent.com/GaeaRuiW/kube-llmops/main/scripts/install.sh | bash

REPO="GaeaRuiW/kube-llmops"
BRANCH="${KUBE_LLMOPS_BRANCH:-main}"
PROFILE="${KUBE_LLMOPS_PROFILE:-minimal}"
RELEASE_NAME="${KUBE_LLMOPS_RELEASE:-kube-llmops}"
NAMESPACE="${KUBE_LLMOPS_NAMESPACE:-default}"

echo "============================================="
echo "  kube-llmops installer"
echo "============================================="
echo "  Profile:   ${PROFILE}"
echo "  Namespace: ${NAMESPACE}"
echo ""

# Check prerequisites
command -v helm >/dev/null 2>&1 || { echo "ERROR: helm is required but not installed."; exit 1; }
command -v kubectl >/dev/null 2>&1 || { echo "ERROR: kubectl is required but not installed."; exit 1; }
kubectl cluster-info >/dev/null 2>&1 || { echo "ERROR: Cannot connect to Kubernetes cluster."; exit 1; }

# Check GPU availability (optional)
GPU_COUNT=$(kubectl get nodes -o jsonpath='{.items[*].status.capacity.nvidia\.com/gpu}' 2>/dev/null | tr ' ' '\n' | awk '{s+=$1} END {print s+0}')
echo "  GPUs detected: ${GPU_COUNT}"
if [ "${GPU_COUNT}" -eq 0 ] && [ "${PROFILE}" != "ci" ]; then
    echo "  WARNING: No GPUs detected. Use KUBE_LLMOPS_PROFILE=ci for CPU-only demo."
fi
echo ""

# Clone and install
TMPDIR=$(mktemp -d)
trap 'rm -rf "${TMPDIR}"' EXIT

echo "Downloading kube-llmops..."
git clone --depth 1 -b "${BRANCH}" "https://github.com/${REPO}.git" "${TMPDIR}/kube-llmops" 2>&1 | tail -1

VALUES_FILE="${TMPDIR}/kube-llmops/charts/kube-llmops-stack/values-${PROFILE}.yaml"
if [ ! -f "${VALUES_FILE}" ]; then
    echo "ERROR: Profile '${PROFILE}' not found. Available: ci, minimal, standard"
    exit 1
fi

echo "Installing kube-llmops (profile: ${PROFILE})..."
helm upgrade --install "${RELEASE_NAME}" \
    "${TMPDIR}/kube-llmops/charts/kube-llmops-stack" \
    -f "${VALUES_FILE}" \
    -n "${NAMESPACE}" \
    --create-namespace \
    --wait --timeout 10m

echo ""
echo "============================================="
echo "  kube-llmops installed successfully!"
echo "============================================="
echo ""
echo "Access the services:"
echo "  kubectl port-forward svc/${RELEASE_NAME}-litellm 4000:4000 -n ${NAMESPACE} &"
echo "  kubectl port-forward svc/${RELEASE_NAME}-grafana 3000:3000 -n ${NAMESPACE} &"
echo ""
echo "Chat with your model:"
echo "  curl http://localhost:4000/v1/chat/completions \\"
echo "    -H 'Authorization: Bearer sk-kube-llmops-dev' \\"
echo "    -H 'Content-Type: application/json' \\"
echo "    -d '{\"model\":\"qwen2-5-0-5b\",\"messages\":[{\"role\":\"user\",\"content\":\"Hello!\"}]}'"
echo ""
echo "Default credentials:"
echo "  LiteLLM:  http://localhost:4000/ui  (any user / sk-kube-llmops-dev)"
echo "  Grafana:  http://localhost:3000     (admin / admin)"
