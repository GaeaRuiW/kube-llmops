#!/bin/bash
# Build the kube-llmops operator Docker image.
# Must be run from the repo root (parent of operator/).
#
# Usage:
#   ./operator/build.sh [IMAGE_TAG]
#   IMAGE_TAG defaults to kube-llmops/operator:latest
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

IMAGE="${1:-kube-llmops/operator:latest}"

echo "=== Building operator image: $IMAGE ==="

# 1. Ensure umbrella chart dependencies are built
echo "[1/3] Building chart dependencies..."
cd charts/kube-llmops-stack
rm -f charts/*.tgz Chart.lock
helm dependency update . 2>/dev/null
cd "$REPO_ROOT"

# 2. Stage the umbrella chart into the operator build context (Docker needs it in context)
#    Use _build_charts/ to avoid colliding with operator/charts/kube-llmops-operator/
echo "[2/3] Staging chart for Docker build..."
rm -rf operator/_build_charts
mkdir -p operator/_build_charts
cp -a charts/kube-llmops-stack operator/_build_charts/

# 3. Build Docker image
echo "[3/3] Building Docker image..."
docker build -t "$IMAGE" operator/

# Cleanup staged chart
rm -rf operator/_build_charts

echo "=== Done: $IMAGE ==="
