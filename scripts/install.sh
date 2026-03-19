#!/usr/bin/env bash
set -euo pipefail

CHART_DIR="$(cd "$(dirname "$0")/.." && pwd)/charts/kube-llmops-stack"
RELEASE_NAME="${1:-kube-llmops}"
PROFILE="${2:-minimal}"
NAMESPACE="${3:-default}"

echo "============================================="
echo "  kube-llmops installer"
echo "============================================="
echo "  Release:   $RELEASE_NAME"
echo "  Profile:   $PROFILE"
echo "  Namespace: $NAMESPACE"
echo "============================================="

# Check prerequisites
command -v helm >/dev/null 2>&1 || { echo "ERROR: helm is required but not installed."; exit 1; }
command -v kubectl >/dev/null 2>&1 || { echo "ERROR: kubectl is required but not installed."; exit 1; }

# Check cluster connectivity
kubectl cluster-info >/dev/null 2>&1 || { echo "ERROR: Cannot connect to Kubernetes cluster."; exit 1; }

VALUES_FILE="$CHART_DIR/values-${PROFILE}.yaml"
if [ ! -f "$VALUES_FILE" ]; then
    echo "ERROR: Profile '$PROFILE' not found. Available: minimal, standard, ci"
    exit 1
fi

echo ""
echo "Updating Helm dependencies..."
helm dependency update "$CHART_DIR" --skip-refresh 2>/dev/null || helm dependency update "$CHART_DIR"

echo ""
echo "Installing kube-llmops with '$PROFILE' profile..."
helm upgrade --install "$RELEASE_NAME" "$CHART_DIR" \
    -f "$VALUES_FILE" \
    --namespace "$NAMESPACE" \
    --create-namespace \
    --wait \
    --timeout 10m

echo ""
echo "============================================="
echo "  kube-llmops installed successfully!"
echo "============================================="
echo ""
echo "Check status:"
echo "  kubectl get pods -n $NAMESPACE"
echo ""
echo "Quick access:"
echo "  kubectl port-forward svc/${RELEASE_NAME}-litellm 4000:4000 -n $NAMESPACE"
echo "  kubectl port-forward svc/${RELEASE_NAME}-grafana 3000:3000 -n $NAMESPACE"
