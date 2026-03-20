#!/usr/bin/env bash
set -euo pipefail

# kube-llmops backup script
# Backs up all stateful data: PostgreSQL (LiteLLM + Langfuse), MinIO models
#
# Usage:
#   ./scripts/backup.sh                    # backup to ./backups/
#   BACKUP_DIR=/mnt/backups ./scripts/backup.sh

NAMESPACE="${KUBE_LLMOPS_NAMESPACE:-default}"
RELEASE="${KUBE_LLMOPS_RELEASE:-kube-llmops}"
BACKUP_DIR="${BACKUP_DIR:-./backups}"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
BACKUP_PATH="${BACKUP_DIR}/${TIMESTAMP}"

mkdir -p "${BACKUP_PATH}"

echo "============================================="
echo "  kube-llmops backup"
echo "============================================="
echo "  Namespace: ${NAMESPACE}"
echo "  Output:    ${BACKUP_PATH}"
echo ""

# 1. PostgreSQL (LiteLLM database)
echo "[1/3] Backing up PostgreSQL (LiteLLM)..."
PG_POD=$(kubectl get pod -l app.kubernetes.io/name=litellm-postgresql -n "${NAMESPACE}" -o jsonpath='{.items[0].metadata.name}')
kubectl exec "${PG_POD}" -n "${NAMESPACE}" -- pg_dumpall -U litellm > "${BACKUP_PATH}/postgresql-all.sql" 2>/dev/null
echo "  Saved: postgresql-all.sql ($(du -h "${BACKUP_PATH}/postgresql-all.sql" | cut -f1))"

# 2. MinIO models (if accessible)
echo "[2/3] Backing up MinIO models..."
if command -v mc &>/dev/null; then
  mc alias set backup-llmops http://localhost:9000 minioadmin minioadmin 2>/dev/null && \
  mc mirror backup-llmops/models "${BACKUP_PATH}/minio-models/" 2>/dev/null && \
  echo "  Saved: minio-models/ ($(du -sh "${BACKUP_PATH}/minio-models/" 2>/dev/null | cut -f1))" || \
  echo "  Skipped: MinIO not accessible (port-forward svc/kube-llmops-minio 9000:9000 first)"
else
  echo "  Skipped: mc (MinIO client) not installed"
fi

# 3. Helm values
echo "[3/3] Backing up Helm values..."
helm get values "${RELEASE}" -n "${NAMESPACE}" > "${BACKUP_PATH}/helm-values.yaml" 2>/dev/null && \
echo "  Saved: helm-values.yaml" || echo "  Skipped: helm release not found"

echo ""
echo "============================================="
echo "  Backup complete: ${BACKUP_PATH}"
echo "============================================="
ls -lh "${BACKUP_PATH}/"
