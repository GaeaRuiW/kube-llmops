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

### 4b. Alternative: Trigger via the Kubernetes Operator (v0.5.0)

Starting with v0.5.0, kube-llmops ships a Kubernetes Operator that exposes
fine-tuning as a first-class CRD (`FineTuneRun`). This is the declarative
alternative to the `argo submit` CLI above — the Operator reconciles the CR
into the same Argo `Workflow` under the hood, so all existing artifacts
(LLaMA-Factory job, MLflow run, quality gate, canary deploy) are produced
the same way.

Prerequisite: the operator chart is installed (see top-level `AGENTS.md` or
`operator/docs/user-guide/`):

```bash
helm install kube-llmops-operator operator/charts/kube-llmops-operator
```

Submit a run by applying a `FineTuneRun` CR:

```yaml
apiVersion: llmops.kubellmops.io/v1alpha1
kind: FineTuneRun
metadata:
  name: my-finetune-run
spec:
  baseModel: Qwen/Qwen2.5-0.5B-Instruct
  method: lora
  dataset:
    type: huggingface
    ref: tatsu-lab/alpaca
```

```bash
kubectl apply -f my-finetune-run.yaml
kubectl get finetunerun my-finetune-run -w
```

The Operator writes the underlying Argo `Workflow` name and phase into
`status.workflowName` / `status.phase`, so you can still use `argo logs`
and the MLflow UI to monitor progress.

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
