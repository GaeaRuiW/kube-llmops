# Fine-tuning Examples

Sample files for testing and demonstrating the kube-llmops fine-tuning pipeline.

## Files

| File | Description |
|------|-------------|
| `sample-training-data.json` | 20 instruction/output pairs in Alpaca format (LLMOps domain) |
| `values-finetune-example.yaml` | Example Helm values overlay to enable fine-tuning |

## Quick Start

### 1. Install prerequisites

```bash
# Argo Workflows operator
helm repo add argo https://argoproj.github.io/argo-helm
helm install argo-workflows argo/argo-workflows -n argo --create-namespace

# Build MLflow image
docker build -t kube-llmops/mlflow:latest images/mlflow/
```

### 2. Upload training data to MinIO

```bash
# Port-forward MinIO
kubectl port-forward svc/kube-llmops-minio 9000:9000 &

# Upload (using mc CLI or curl)
mc alias set minio http://localhost:9000 minioadmin minioadmin
mc mb minio/datasets/sft/ --ignore-existing
mc cp examples/finetune/sample-training-data.json minio/datasets/sft/train.json
```

### 3. Deploy with fine-tuning enabled

```bash
helm upgrade kube-llmops charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  -f examples/finetune/values-finetune-example.yaml
```

### 4. Trigger a fine-tuning run

```bash
argo submit -n default --from workflowtemplate/kube-llmops-finetune
```

### 5. Monitor progress

```bash
# Watch workflow
argo watch -n default @latest

# View logs
argo logs -n default @latest --follow

# Check MLflow UI
kubectl port-forward svc/kube-llmops-mlflow 5000:5000 &
open http://localhost:5000
```

## Data Format

The sample data uses **Alpaca format** — each record has:

```json
{
  "instruction": "The task or question",
  "input": "",
  "output": "The expected response"
}
```

For production fine-tuning, provide hundreds to thousands of high-quality examples.
