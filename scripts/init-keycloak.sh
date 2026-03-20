#!/usr/bin/env bash
set -euo pipefail

# Initialize Keycloak realm, clients, roles, and users for kube-llmops
# Run this after Keycloak is deployed and ready.
#
# Usage:
#   ./scripts/init-keycloak.sh
#   KEYCLOAK_URL=http://my-keycloak:8080 ./scripts/init-keycloak.sh

KEYCLOAK_URL="${KEYCLOAK_URL:-http://kube-llmops-keycloak:8080}"
ADMIN_USER="${KEYCLOAK_ADMIN:-admin}"
ADMIN_PASS="${KEYCLOAK_ADMIN_PASSWORD:-admin123!}"
REALM="${KEYCLOAK_REALM:-kube-llmops}"
DEFAULT_USER_EMAIL="${DEFAULT_USER_EMAIL:-admin@kube-llmops.local}"
DEFAULT_USER_PASS="${DEFAULT_USER_PASSWORD:-admin123!}"

echo "============================================="
echo "  Keycloak Initialization"
echo "============================================="
echo "  URL:   ${KEYCLOAK_URL}"
echo "  Realm: ${REALM}"
echo ""

# Helper: run curl inside the cluster if KEYCLOAK_URL is internal
_curl() {
  if [[ "${KEYCLOAK_URL}" == *"kube-llmops-keycloak"* ]]; then
    # Internal URL — exec from a pod
    local POD
    POD=$(kubectl get pod -l kube-llmops/engine=vllm -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || \
          kubectl get pod -l app.kubernetes.io/name=litellm -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    if [ -z "$POD" ]; then
      echo "ERROR: No pod found to exec curl from. Set KEYCLOAK_URL to a host-accessible URL."
      exit 1
    fi
    kubectl exec "$POD" -c "$(kubectl get pod "$POD" -o jsonpath='{.spec.containers[0].name}')" -- curl -s "$@" 2>&1
  else
    curl -s "$@" 2>&1
  fi
}

# Get admin token
echo "Getting admin token..."
TOKEN=$(_curl "${KEYCLOAK_URL}/realms/master/protocol/openid-connect/token" \
  -d "grant_type=password&client_id=admin-cli&username=${ADMIN_USER}&password=${ADMIN_PASS}" | \
  python3 -c "import sys,json;print(json.load(sys.stdin)['access_token'])")

if [ ${#TOKEN} -lt 10 ]; then
  echo "ERROR: Failed to get admin token. Is Keycloak running?"
  exit 1
fi
echo "  Token obtained (${#TOKEN} chars)"

# Create realm
echo "Creating realm '${REALM}'..."
STATUS=$(_curl -o /dev/null -w "%{http_code}" \
  "${KEYCLOAK_URL}/admin/realms" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"realm\":\"${REALM}\",\"enabled\":true}")
echo "  Status: ${STATUS} (201=created, 409=exists)"

# Create OIDC clients
echo "Creating OIDC clients..."
for CLIENT in grafana langfuse minio litellm; do
  STATUS=$(_curl -o /dev/null -w "%{http_code}" \
    "${KEYCLOAK_URL}/admin/realms/${REALM}/clients" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"clientId\":\"${CLIENT}\",\"enabled\":true,\"publicClient\":false,\"secret\":\"${CLIENT}-oidc-secret\",\"redirectUris\":[\"*\"],\"standardFlowEnabled\":true,\"directAccessGrantsEnabled\":true}")
  echo "  ${CLIENT}: ${STATUS}"
done

# Create admin role
echo "Creating 'admin' realm role..."
_curl -o /dev/null -w "  Status: %{http_code}\n" \
  "${KEYCLOAK_URL}/admin/realms/${REALM}/roles" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"admin","description":"Admin role for Grafana SSO"}'

# Create default user
echo "Creating default user (${DEFAULT_USER_EMAIL})..."
_curl -o /dev/null -w "  Status: %{http_code}\n" \
  "${KEYCLOAK_URL}/admin/realms/${REALM}/users" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"admin\",\"email\":\"${DEFAULT_USER_EMAIL}\",\"firstName\":\"Admin\",\"lastName\":\"User\",\"enabled\":true,\"emailVerified\":true,\"credentials\":[{\"type\":\"password\",\"value\":\"${DEFAULT_USER_PASS}\",\"temporary\":false}]}"

# Assign admin role to user
echo "Assigning admin role..."
USER_ID=$(_curl "${KEYCLOAK_URL}/admin/realms/${REALM}/users?username=admin" \
  -H "Authorization: Bearer $TOKEN" | \
  python3 -c "import sys,json;print(json.load(sys.stdin)[0]['id'])")
ROLE=$(_curl "${KEYCLOAK_URL}/admin/realms/${REALM}/roles/admin" \
  -H "Authorization: Bearer $TOKEN")
_curl -o /dev/null -w "  Status: %{http_code}\n" \
  "${KEYCLOAK_URL}/admin/realms/${REALM}/users/${USER_ID}/role-mappings/realm" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "[$ROLE]"

# Add realm_access.roles to userinfo for Grafana role mapping
echo "Adding realm roles mapper to grafana client..."
GRAFANA_UUID=$(_curl "${KEYCLOAK_URL}/admin/realms/${REALM}/clients?clientId=grafana" \
  -H "Authorization: Bearer $TOKEN" | python3 -c "import json,sys; print(json.load(sys.stdin)[0]['id'])")
_curl -o /dev/null -w "  Status: %{http_code}\n" \
  "${KEYCLOAK_URL}/admin/realms/${REALM}/clients/${GRAFANA_UUID}/protocol-mappers/models" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "realm roles",
    "protocol": "openid-connect",
    "protocolMapper": "oidc-usermodel-realm-role-mapper",
    "consentRequired": false,
    "config": {
      "multivalued": "true",
      "userinfo.token.claim": "true",
      "id.token.claim": "true",
      "access.token.claim": "true",
      "claim.name": "realm_access.roles",
      "jsonType.label": "String"
    }
  }'

echo ""
echo "============================================="
echo "  Keycloak initialized!"
echo "============================================="
echo ""
echo "  Realm:     ${REALM}"
echo "  Clients:   grafana, langfuse, minio, litellm"
echo "  User:      admin / ${DEFAULT_USER_PASS}"
echo "  Email:     ${DEFAULT_USER_EMAIL}"
echo ""
echo "  OIDC discovery:"
echo "  ${KEYCLOAK_URL}/realms/${REALM}/.well-known/openid-configuration"
