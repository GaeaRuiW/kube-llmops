#!/usr/bin/env bash
set -euo pipefail

# kube-llmops restore script
# Restores from a backup created by backup.sh
#
# Usage:
#   ./scripts/restore.sh ./backups/20260319-120000

BACKUP_PATH="${1:?Usage: $0 <backup-path>}"
NAMESPACE="${KUBE_LLMOPS_NAMESPACE:-default}"

if [ ! -d "${BACKUP_PATH}" ]; then
  echo "ERROR: Backup path not found: ${BACKUP_PATH}"
  exit 1
fi

echo "============================================="
echo "  kube-llmops restore"
echo "============================================="
echo "  Namespace: ${NAMESPACE}"
echo "  Source:    ${BACKUP_PATH}"
echo ""

# 1. Restore PostgreSQL
if [ -f "${BACKUP_PATH}/postgresql-all.sql" ]; then
  echo "[1/3] Restoring PostgreSQL..."
  PG_POD=$(kubectl get pod -l app.kubernetes.io/name=litellm-postgresql -n "${NAMESPACE}" -o jsonpath='{.items[0].metadata.name}')
  kubectl exec -i "${PG_POD}" -n "${NAMESPACE}" -- psql -U litellm -d litellm < "${BACKUP_PATH}/postgresql-all.sql" 2>/dev/null
  echo "  Restored: postgresql-all.sql"
else
  echo "[1/3] Skipped: postgresql-all.sql not found"
fi

# 2. Restore MinIO
if [ -d "${BACKUP_PATH}/minio-models" ]; then
  echo "[2/3] Restoring MinIO models..."
  if command -v mc &>/dev/null; then
    mc alias set restore-llmops http://localhost:9000 minioadmin minioadmin 2>/dev/null && \
    mc mb restore-llmops/models 2>/dev/null || true
    mc mirror "${BACKUP_PATH}/minio-models/" restore-llmops/models/ 2>/dev/null && \
    echo "  Restored: minio-models/" || echo "  Failed: MinIO not accessible"
  else
    echo "  Skipped: mc (MinIO client) not installed"
  fi
else
  echo "[2/3] Skipped: minio-models/ not found"
fi

# 3. Restore Helm values
if [ -f "${BACKUP_PATH}/helm-values.yaml" ]; then
  echo "[3/3] Helm values saved at: ${BACKUP_PATH}/helm-values.yaml"
  echo "  To re-deploy: helm upgrade kube-llmops charts/kube-llmops-stack -f ${BACKUP_PATH}/helm-values.yaml"
else
  echo "[3/3] Skipped: helm-values.yaml not found"
fi

echo ""
echo "============================================="
echo "  Restore complete!"
echo "============================================="
echo ""
echo "Restart services to pick up restored data:"
echo "  kubectl rollout restart deployment -l app.kubernetes.io/part-of=kube-llmops -n ${NAMESPACE}"
