#!/usr/bin/env bash
set -euo pipefail

# kube-llmops deploy script
# Handles Helm upgrade + Ingress URL configuration + Helm SSA workarounds
#
# Usage:
#   ./scripts/deploy.sh -f values-minimal.yaml
#   ./scripts/deploy.sh -f values-minimal.yaml --set ingress.enabled=true --set ingress.host=llmops.local
#   INGRESS_HOST=llmops.local ./scripts/deploy.sh -f values-minimal.yaml

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

# Detect ingress host from args or env
INGRESS_HOST="${INGRESS_HOST:-}"
for arg in "$@"; do
  if [[ "$arg" == *"ingress.host="* ]]; then
    INGRESS_HOST="${arg#*ingress.host=}"
  fi
done

# Build Ingress-aware --set args
INGRESS_SETS=""
if [ -n "$INGRESS_HOST" ]; then
  echo "  Ingress:   *.${INGRESS_HOST}"
  INGRESS_SETS="
    --set ingress.enabled=true
    --set ingress.host=${INGRESS_HOST}
    --set langfuse.externalUrl=http://langfuse.${INGRESS_HOST}
    --set langfuse.oidc.issuerUrl=http://keycloak.${INGRESS_HOST}/realms/kube-llmops
    --set observability.grafana.oidc.issuerUrl=http://keycloak.${INGRESS_HOST}/realms/kube-llmops
    --set observability.grafana.oidc.grafanaRootUrl=http://grafana.${INGRESS_HOST}
    --set dify.web.consoleApiUrl=http://dify-api.${INGRESS_HOST}
    --set dify.web.consoleWebUrl=http://dify.${INGRESS_HOST}
    --set dify.web.appApiUrl=http://dify-api.${INGRESS_HOST}
  "
fi
echo ""

# Step 1: Delete conflicting ConfigMaps (Helm SSA workaround)
echo "[1/5] Cleaning up stale ConfigMaps..."
kubectl delete cm "${RELEASE_NAME}-litellm-config" -n "${NAMESPACE}" 2>/dev/null && echo "  Deleted litellm-config" || true

# Step 2: Helm upgrade
echo "[2/5] Running helm upgrade..."
# shellcheck disable=SC2086
helm upgrade --install "${RELEASE_NAME}" "${CHART_DIR}" \
  -n "${NAMESPACE}" --create-namespace \
  ${INGRESS_SETS} \
  "$@"

echo ""

# Step 3: Force-apply deployments that Helm SSA often fails to update
echo "[3/5] Force-applying deployments (Helm SSA workaround)..."
for SUBPATH in \
  "charts/observability/templates/grafana.yaml" \
  "charts/observability/templates/prometheus.yaml" \
  "charts/langfuse/templates/deployment.yaml" \
  "charts/litellm/templates/configmap.yaml" \
  "charts/dify/templates/dify.yaml" \
  "templates/ingress.yaml" \
; do
  # shellcheck disable=SC2086
  if helm template "${RELEASE_NAME}" "${CHART_DIR}" ${INGRESS_SETS} "$@" \
    --show-only "${SUBPATH}" 2>/dev/null | grep -q "kind:"; then
    # shellcheck disable=SC2086
    helm template "${RELEASE_NAME}" "${CHART_DIR}" ${INGRESS_SETS} "$@" \
      --show-only "${SUBPATH}" 2>/dev/null | \
      kubectl apply --force -f - -n "${NAMESPACE}" 2>/dev/null && \
      echo "  Applied ${SUBPATH}" || true
  fi
done

# Step 4: Restart deployments that use subPath mounts (no hot-reload)
echo "[4/5] Restarting services with config changes..."
kubectl rollout restart deployment "${RELEASE_NAME}-litellm" -n "${NAMESPACE}" 2>/dev/null || true

# Step 5: Setup Keycloak hostname if ingress is enabled
if [ -n "$INGRESS_HOST" ]; then
  echo "[5/5] Configuring Keycloak for Ingress..."
  KC_DEPLOY=$(kubectl get deploy -l app.kubernetes.io/name=keycloak -n "${NAMESPACE}" -o name 2>/dev/null | head -1)
  if [ -n "$KC_DEPLOY" ]; then
    kubectl set env "${KC_DEPLOY}" -n "${NAMESPACE}" \
      KC_HOSTNAME="keycloak.${INGRESS_HOST}" \
      KC_HOSTNAME_PORT="" \
      KC_HOSTNAME_STRICT=true \
      KC_HOSTNAME_STRICT_HTTPS=false \
      2>/dev/null && echo "  Keycloak hostname set to keycloak.${INGRESS_HOST}" || true
  fi
else
  echo "[5/5] No Ingress host — skipping Keycloak config"
fi

echo ""
echo "============================================="
echo "  Deploy complete!"
echo "============================================="

if [ -n "$INGRESS_HOST" ]; then
  NODE_IP=$(kubectl get node -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}' 2>/dev/null || echo "<node-ip>")
  echo ""
  echo "  Add to /etc/hosts (if no real DNS):"
  echo "  ${NODE_IP} litellm.${INGRESS_HOST} grafana.${INGRESS_HOST} langfuse.${INGRESS_HOST} dify.${INGRESS_HOST} dify-api.${INGRESS_HOST} keycloak.${INGRESS_HOST} prometheus.${INGRESS_HOST} minio.${INGRESS_HOST}"
  echo ""
  echo "  Services:"
  echo "    LiteLLM:    http://litellm.${INGRESS_HOST}"
  echo "    Grafana:    http://grafana.${INGRESS_HOST}"
  echo "    Langfuse:   http://langfuse.${INGRESS_HOST}"
  echo "    Dify:       http://dify.${INGRESS_HOST}"
  echo "    Keycloak:   http://keycloak.${INGRESS_HOST}"
  echo "    Prometheus: http://prometheus.${INGRESS_HOST}"
  echo "    MinIO:      http://minio.${INGRESS_HOST}"
fi
echo ""
echo "Check status: kubectl get pods -n ${NAMESPACE}"
