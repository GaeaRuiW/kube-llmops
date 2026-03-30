# LLaMA-Factory Fine-tuning Pipeline — Design Spec

> **Date**: 2026-03-30
> **Phase**: v0.4.0 (ML Platform) — Sub-project 1 of 6
> **Status**: Approved, ready for implementation planning

---

## 1. Overview

Add a production-grade fine-tuning pipeline to kube-llmops. Users define a fine-tuning task in values, and the system handles the full lifecycle: data preparation → training → model registration → evaluation → approval → canary deployment.

**Key decisions:**
- Orchestration: Argo Workflows (DAG-based, production-grade)
- Training: LLaMA-Factory (LoRA / QLoRA / Full, all model types)
- Experiment tracking: MLflow (Experiment Tracking + Model Registry)
- Deployment: LiteLLM weight-based canary routing (no service mesh)
- Storage: Reuse existing PostgreSQL + MinIO

---

## 2. User Configuration

All parameters in a single `finetune:` section of values:

```yaml
finetune:
  enabled: false
  
  # Base model (reuses global.models source naming, engine auto-detected)
  baseModel: Qwen/Qwen2.5-0.5B-Instruct
  outputName: qwen2-5-0-5b-lora-v1
  
  # Training method
  method: lora              # lora | qlora | full
  loraRank: 8
  loraAlpha: 16
  epochs: 3
  batchSize: 4
  learningRate: 2e-4
  
  # Data source
  dataSource:
    type: minio             # minio | huggingface | pvc
    path: s3://datasets/my-sft-data/
    format: alpaca          # alpaca | sharegpt | custom
  
  # GPU resources
  resources:
    gpu: 1
    memory: 16Gi
    cpu: "4"
  
  # Node scheduling (optional)
  nodeSelector: {}
  tolerations: []
  
  # Evaluation + quality gate
  evaluation:
    enabled: true
    benchmarks:
      - ragas
    thresholds:
      faithfulness: 0.7
      answer_relevancy: 0.7
  
  # Deployment strategy
  deploy:
    auto: false             # true = auto canary after eval pass
    canaryPercent: 20
    notifyWebhook: ""       # Slack/webhook for approval notification
  
  # Schedule (empty = manual trigger, cron = periodic)
  schedule: ""

mlflow:
  enabled: false            # auto-enabled when finetune.enabled=true
  image:
    repository: ghcr.io/mlflow/mlflow
    tag: "2.21.3"
  resources:
    requests:
      cpu: 250m
      memory: 512Mi
    limits:
      cpu: "1"
      memory: 1Gi
```

---

## 3. Argo Workflow DAG

### 3.1 Pipeline Overview

```
prepare-data ──▶ finetune ──▶ merge-upload ──▶ evaluate ──▶ quality-gate ──┬──▶ deploy
                     │                              │                       │
                     └── MLflow: log params/metrics  └── MLflow: log eval    └── canary or notify
```

### 3.2 Step Specifications

| Step | Image | GPU | Input | Output | Logic |
|------|-------|-----|-------|--------|-------|
| **prepare-data** | `kube-llmops/model-loader:latest` | No | `dataSource` config | `/workspace/data/train.json` | Branch by `dataSource.type`: MinIO→mc cp, HF→`datasets.load_dataset`, PVC→mount. Convert to LLaMA-Factory format |
| **finetune** | `llamafactory/llamafactory:latest` | **Yes** | Base model + train data + ConfigMap | LoRA adapter at `/workspace/output/` | Read ConfigMap-generated `train_config.yaml`, run `llamafactory-cli train`. Auto-logs loss/lr/epoch to MLflow via `MLFLOW_TRACKING_URI` env |
| **merge-upload** | `kube-llmops/model-loader:latest` | Depends | LoRA adapter + base model | `s3://models/<outputName>/` + MLflow Model Registry | LoRA→merge weights (needs GPU); Full→direct upload. Register in MLflow as new version, state=`Staging` |
| **evaluate** | `python:3.13-slim + ragas` | No | Fine-tuned model (via temp vLLM instance) | Metrics → MLflow + Prometheus | Create a temporary vLLM Deployment (`vllm-<outputName>-eval`) pointing to the merged model in MinIO, wait for readiness, run Ragas 4 metrics + custom benchmarks, write to MLflow run and Pushgateway, then delete the temp Deployment |
| **quality-gate** | `python:3.13-slim` | No | Eval metrics | pass / fail | Compare against `evaluation.thresholds`. On fail: mark MLflow version as `Archived`, send webhook |
| **deploy** | `bitnami/kubectl:latest` | No | Approved model version | Updated Deployment | `deploy.auto=true`→canary deploy; `deploy.auto=false`→webhook notify only |

### 3.3 Shared Volume

```yaml
volumeClaimTemplates:
  - name: workspace         # Shared across all DAG steps
    size: 100Gi
    accessModes: [ReadWriteOnce]
```

Directory layout:
- `/workspace/data/` — Training data
- `/workspace/base-model/` — Base model (downloaded by model-loader)
- `/workspace/output/` — Fine-tune output (adapter / merged weights)
- `/workspace/eval/` — Evaluation results

### 3.4 Trigger Mechanism

Helm renders a **WorkflowTemplate** (not a direct Workflow). Users trigger on demand:

```bash
# Manual
argo submit -n default --from workflowtemplate/kube-llmops-finetune

# Or via kubectl
kubectl create -f <(helm template kube-llmops charts/kube-llmops-stack \
  -f values-single-node.yaml -s charts/finetune/templates/workflow.yaml)

# Periodic (via values)
finetune:
  schedule: "0 2 * * 0"    # CronWorkflow: every Sunday 2AM
```

---

## 4. MLflow Integration

### 4.1 Deployment

```yaml
Deployment: kube-llmops-mlflow
  image: ghcr.io/mlflow/mlflow:2.21.3
  port: 5000
  args:
    - mlflow server
    - --backend-store-uri postgresql://mlflow:<pw>@kube-llmops-litellm-pg:5432/mlflow
    - --default-artifact-root s3://mlflow/
    - --host 0.0.0.0
  env:
    MLFLOW_S3_ENDPOINT_URL: http://kube-llmops-minio:9000
    AWS_ACCESS_KEY_ID: minioadmin
    AWS_SECRET_ACCESS_KEY: minioadmin
```

### 4.2 Storage Backend (reuses existing infra)

| Storage | Component | New |
|---------|-----------|-----|
| Metadata (experiments, runs, metrics) | PostgreSQL → `mlflow` database | DB only |
| Artifacts (model files, charts) | MinIO → `s3://mlflow/` bucket | Bucket only |
| Training datasets | MinIO → `s3://datasets/` bucket | Bucket only |

### 4.3 LLaMA-Factory Integration

LLaMA-Factory natively supports MLflow. Set env vars on the finetune step:

```yaml
env:
  - name: MLFLOW_TRACKING_URI
    value: http://kube-llmops-mlflow:5000
  - name: MLFLOW_EXPERIMENT_NAME
    value: "{{ .Values.finetune.outputName }}"
```

Auto-records: loss, learning_rate, epoch, eval_loss — no code changes needed.

### 4.4 Model Registry Workflow

```python
# In merge-upload step
mlflow.register_model("s3://models/<outputName>/", "<model-name>")
# → Auto-increments: Version 1, 2, 3...
# → State set to "Staging"

# After quality-gate pass
client.transition_model_version_stage("<model-name>", version, "Production")

# On quality-gate fail
client.transition_model_version_stage("<model-name>", version, "Archived")
```

### 4.5 NodePort Access

MLflow UI exposed at `:30505` when NodePort enabled.

---

## 5. Canary Deployment

### 5.1 Mechanism (LiteLLM weight routing, no service mesh)

```yaml
# LiteLLM config during canary
model_list:
  - model_name: qwen2-5-0-5b          # Original
    litellm_params:
      model: openai/qwen2-5-0-5b
      api_base: http://vllm-qwen2-5-0-5b:8000/v1
      weight: 80
  - model_name: qwen2-5-0-5b          # Fine-tuned (same name!)
    litellm_params:
      model: openai/qwen2-5-0-5b
      api_base: http://vllm-qwen2-5-0-5b-ft:8000/v1
      weight: 20
```

### 5.2 Deploy Step Actions

1. Create new vLLM Deployment (`vllm-<outputName>`) with fine-tuned model from MinIO
2. Wait for readiness probe to pass
3. Patch LiteLLM ConfigMap with canary weight routing
4. Restart LiteLLM pod to load new config
5. Tag MLflow model: `deployment_stage: canary`

### 5.3 Promote (manual trigger)

```bash
kubectl create job promote-ft --from=cronjob/kube-llmops-ft-promote
```

- Update LiteLLM ConfigMap: new model weight=100, remove old
- MLflow: Staging → Production, old Production → Archived
- Optional: delete old vLLM Deployment to free GPU

### 5.4 Rollback (one command)

```bash
kubectl create job rollback-ft --from=cronjob/kube-llmops-ft-rollback
```

- Restore LiteLLM ConfigMap to pre-canary state
- MLflow version state rollback
- Delete fine-tuned vLLM Deployment

### 5.5 Approval Flow (when deploy.auto=false)

```
quality-gate PASS → webhook notification:
{
  "model": "<outputName>",
  "version": N,
  "metrics": {"faithfulness": 0.82, "relevancy": 0.85},
  "mlflow_url": "http://<NODE_IP>:30505/#/models/<name>/versions/N",
  "approve_cmd": "kubectl create job deploy-ft-vN --from=cronjob/kube-llmops-ft-deploy"
}
```

Human reviews metrics in MLflow UI → manually runs approve command.

---

## 6. Argo Workflows Operator

### 6.1 Prerequisite (not bundled)

```bash
kubectl create ns argo
kubectl apply -n argo -f https://github.com/argoproj/argo-workflows/releases/download/v3.6.5/install.yaml
```

### 6.2 Auto-Detection

```go
{{- if not (lookup "apiextensions.k8s.io/v1" "CustomResourceDefinition" "" "workflows.argoproj.io") }}
{{- fail "Argo Workflows operator required. Install: ..." }}
{{- end }}
```

Helm fails with actionable error if Argo CRD not present.

---

## 7. New Components Summary

| Component | Type | Image | New? |
|-----------|------|-------|------|
| Argo Workflows | Operator (cluster-level) | argoproj/workflow-controller | Pre-installed |
| MLflow | Deployment | ghcr.io/mlflow/mlflow:2.21.3 | **New** |
| LLaMA-Factory | Workflow Step | llamafactory/llamafactory:latest | **New (ephemeral)** |
| PostgreSQL | Existing | pgvector/pgvector:pg16 | +mlflow DB |
| MinIO | Existing | minio/minio | +mlflow, +datasets buckets |

## 8. Grafana Dashboard (11th)

`finetune-overview.json`:
- Training loss curve (MLflow metrics via Prometheus export)
- Fine-tune Job status (running / succeeded / failed)
- Model version list (Production / Staging / Archived)
- Canary traffic split real-time

---

## 9. File Structure

```
charts/kube-llmops-stack/charts/finetune/
  Chart.yaml
  values.yaml
  templates/
    workflow.yaml              # Argo Workflow DAG (WorkflowTemplate)
    cronworkflow.yaml          # CronWorkflow (if schedule set)
    mlflow.yaml                # MLflow Deployment + Service
    configmap-train.yaml       # LLaMA-Factory training config
    rbac.yaml                  # ServiceAccount for kubectl patch
    pdb.yaml                   # PodDisruptionBudget for MLflow
charts/kube-llmops-stack/charts/observability/dashboards/
    finetune-overview.json     # New Grafana dashboard
```

---

## 10. Supported Model Types

| Model Type | Example | Method | Engine |
|------------|---------|--------|--------|
| LLM (small) | Qwen2.5-0.5B | LoRA / Full | vllm |
| LLM (large) | Llama-3.1-70B | QLoRA (4-bit) | vllm |
| Embedding | bge-small-en-v1.5 | Full fine-tune | tei |
| GGUF | — | Export after LoRA merge | llamacpp |

Engine auto-detection (`resolveEngine`) applies to fine-tuned model deployment — user doesn't need to specify engine for the output model.
