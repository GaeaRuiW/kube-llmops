# AGENTS.md — Project Knowledge for AI Assistants

## Project Overview
kube-llmops is a Kubernetes-native LLMOps platform using Umbrella Helm Charts.
Deploy, manage, monitor, and optimize LLM infrastructure with a single `helm install`.

## Key Commands

```bash
# Deploy (single-node with GPU + NodePort access)
NODE_IP=$(kubectl get node -o jsonpath='{.items[0].status.addresses[0].address}')
helm install kube-llmops charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  --set global.nodePort.enabled=true \
  --set global.nodePort.host=$NODE_IP

# Upgrade
helm upgrade kube-llmops charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml --no-hooks

# IMPORTANT: After changing any subchart template, rebuild archives:
cd charts/kube-llmops-stack && rm -f charts/*.tgz Chart.lock && helm dependency update .

# Build model-loader image (first time only)
docker build -t kube-llmops/model-loader:latest images/model-loader/

# Run Playwright E2E tests
uv run tests/e2e/test_dify_model_provider.py
uv run tests/e2e/test_dify_rag_e2e.py

# Trigger Ragas evaluation
kubectl create job ragas-manual --from=cronjob/kube-llmops-ragas-eval

# Check smoke test
kubectl logs -l app.kubernetes.io/name=rag-smoke-test --tail=30

# Check quality gate
kubectl logs job/kube-llmops-quality-gate

# Run all Helm template tests (Phase 5 + finetune)
python -m pytest tests/helm/ -v

# Run finetune E2E tests (requires GPU cluster + Argo Workflows)
uv run tests/e2e/test_finetune_e2e.py

# Run Phase 5 Helm template tests
python -m pytest tests/helm/test_phase5_templates.py -v
```

## Architecture

```
┌─ Ingress (Traefik) / NodePort ───────────────────┐
│  *.llmops.local → litellm/grafana/langfuse/dify  │
│  or NODE_IP:304xx                                 │
├──────────────────────────────────────────────────┤
│ LiteLLM (Gateway:4000) → vLLM (LLM:8000)        │
│                        → TEI (Embed:8080)         │
│                        → TEI (Rerank:8080)        │
│                        → llama.cpp (GGUF:8080)    │
│ Dify (RAG:5001/3000) → LiteLLM → pgvector        │
│ Langfuse (Trace:3000) ← LiteLLM callbacks         │
│ Prometheus:9090 + Pushgateway:9091 → Grafana:3000 │
│ Node Exporter:9100 + Kube State Metrics:8080      │
│ LLM-Guard (Security:8000)                         │
│ Keycloak (SSO:8080)                               │
│ Argo Workflows + LLaMA-Factory (Fine-tune)          │
│ MLflow (Experiment Tracking:5000)                    │
│ MinIO (S3:9000) + PostgreSQL:5432                  │
└──────────────────────────────────────────────────┘
```

## Key Features (v0.5.0)

### Engine Auto-Detection
Models are defined in a single `global.models` list. Engine is auto-detected from source name:
- `*GGUF*` → llamacpp
- `*rerank*`, `bge-*`, `e5-*`, `*embedding*` → tei
- everything else → vllm
- Explicit `engine:` field overrides auto-detection

### Unified Model Distribution
```
helm install → model-preload Job → HF download → MinIO cache
                                                    ↓
pod start   → model-loader init  → MinIO hit → local PVC (<1s)
```
- Pre-built `model-loader` image (no runtime pip install)
- hf-transfer multi-threaded downloads (3-5x faster)
- `global.hfToken` for gated models (Llama, Gemma, etc.)

### NodePort Access
```bash
--set global.nodePort.enabled=true --set global.nodePort.host=$NODE_IP
```
Ports: LiteLLM :30400, Grafana :30300, Langfuse :30301, Dify :30500,
       Keycloak :30808, Prometheus :30909, MinIO :30900/:30901

SSO works automatically — OIDC URLs auto-computed from nodePort.host.

### Fine-tuning Pipeline (v0.4.0)
- Argo Workflows DAG: prepare-data → finetune → merge-upload → evaluate → quality-gate → deploy
- LLaMA-Factory: LoRA / QLoRA / Full fine-tuning for all model types
- MLflow: Experiment tracking + Model Registry (reuses PostgreSQL + MinIO)
- Data sources: MinIO (s3://), HuggingFace datasets, PVC
- Quality gate with configurable thresholds
- Canary deployment via LiteLLM weight routing
- Human approval via webhook notifications
- Prerequisite: Argo Workflows operator must be installed separately

### HPA Autoscaling (KEDA)
- Auto-creates HPA for vLLM and llama.cpp Deployments via KEDA ScaledObjects
- Trigger: Prometheus `pending_requests` metric per engine:
  - vLLM: `vllm:num_requests_waiting{model_name="<name>"}`
  - llama.cpp: `llamacpp_requests_processing{model="<name>"}`
- Models auto-detected from `global.models` (no separate list needed)
- Per-model overrides: `keda.models.<name>.{minReplicas,maxReplicas,threshold}`
- Prerequisite: KEDA operator must be installed separately
  ```bash
  helm repo add kedacore https://kedacore.github.io/charts
  helm install keda kedacore/keda -n keda-system --create-namespace
  ```

### Advanced Inference (v0.5.0)
- Latency-based routing (default strategy, configurable per deployment)
- Prefix caching for repeated system prompts
- Multi-trigger KEDA autoscaling (queue + TTFT P95 + TPOT P95)
- Scale-to-zero with LiteLLM fallback for cold start
- Spot/preemptible GPU tolerations (AWS, GCP, Azure, Karpenter)
- MIG GPU device support for model co-location
- Canary deployment with weight-based traffic splitting
- llm-d disaggregated serving (experimental, prefill/decode split)
- Multi-accelerator support (nvidia, amd, gaudi)
- SLO alerts (TTFT/TPOT breach thresholds)

## Critical Gotchas

### Helm .tgz Cache
Helm uses `.tgz` archives in `charts/` over directory sources. After editing any subchart template:
```bash
rm -f charts/kube-llmops-stack/charts/*.tgz charts/kube-llmops-stack/Chart.lock
helm dependency update charts/kube-llmops-stack/
```

### LiteLLM Embedding Config
- Use `huggingface/` prefix (NOT `openai/`) for TEI embedding models
- Set `drop_params: true` in `litellm_settings` (Dify sends `encoding_format` which huggingface provider rejects)
- `api_base` for huggingface provider: NO `/v1` suffix

### Dify 1.x Architecture
- Uses HttpOnly cookies with `SameSite=Lax` — must use single-domain path-based routing
- Requires Plugin Daemon (`dify-plugin-daemon`) for model providers
- Plugin Daemon needs its own DB (`dify_plugin`) + Redis + PVC for persistence
- Frontend sends base64-encoded passwords in login requests

### LLM-Guard
- Needs ~6GB RAM for PromptInjection scanner model
- Config file at `/home/user/app/config/scanners.yml` — mount via ConfigMap
- Default config enables ALL scanners (Sentiment scanner has ZeroDivisionError bug)

### PostgreSQL
- Image: `pgvector/pgvector:pg16` (NOT `postgres:16-alpine`)
- Init script auto-creates databases: litellm, langfuse, dify, dify_plugin
- Auto-enables `vector` extension in each DB

### Model Loader
- Pre-built image: `kube-llmops/model-loader:latest` (must `docker build` before first deploy)
- Flow: check MinIO → fallback HuggingFace → upload back to MinIO
- Uses hf-transfer for multi-threaded downloads
- `HF_HUB_ENABLE_HF_TRANSFER=1` set automatically

## Test Coverage

| Test | Tool | Count | What it validates |
|------|------|-------|-------------------|
| Model Provider | Playwright | 5/5 | Login + add LLM + embedding + verify |
| RAG E2E | Playwright | 9/9 | KB → upload → index → chat → verify answer |
| Smoke Test | K8s Job | 5/5 | Embedding + LLM + Langfuse + trace + reranker |
| Ragas Eval | K8s CronJob | 4 metrics | faithfulness, relevancy, precision, recall |
| Quality Gate | Helm Hook | pass/block | Pre-upgrade check on Ragas thresholds |
| LLM-Guard | Manual | 4/4 | Normal + direct injection + subtle + benign |
| Finetune Helm Templates | pytest | 35+ | ConfigMap, RBAC, MLflow, PDB, LoRA/QLoRA/Full, profiles, validation |
| Finetune E2E | Python+kubectl | ~26 | MLflow health, WorkflowTemplate, Argo run, Registry, QG, Grafana |
| Finetune Sample Data | CI | 1 | Alpaca-format validation (>=10 samples) |
| Phase 5 Templates | pytest | 39 | routing, KEDA multi-trigger, scale-to-zero, canary, llm-d, MIG, accelerator |

## Grafana Dashboards (11)

| UID | Title |
|-----|-------|
| `vllm-overview` | vLLM Model Serving |
| `litellm-gateway` | LiteLLM AI Gateway |
| `gpu-overview` | GPU & Infrastructure |
| `rag-quality` | RAG Quality (Ragas) |
| `cost-usage` | Cost & Usage |
| `slo-overview` | SLO Overview |
| `infra-roi` | Infrastructure ROI |
| `tenant-overview` | Tenant Overview |
| `milvus-overview` | Milvus Vector DB |
| `system-overview` | System CPU/Memory/Disk/Network |
| `finetune-overview` | Fine-tuning Pipeline |

## File Layout

```
charts/kube-llmops-stack/
  values-single-node.yaml     # Main config for single GPU node
  templates/
    _helpers.tpl              # resolveEngine + resolveModelType + modelLoader helpers
    nodeport-services.yaml    # NodePort toggle
    model-preload-job.yaml    # Helm hook: batch HF→MinIO
    secret-hf-token.yaml      # global.hfToken Secret
  charts/
    vllm/                     # Model serving (PagedAttention)
    tei/                      # Embedding + reranking
    llamacpp/                 # GGUF model serving
    litellm/                  # AI Gateway
    langfuse/                 # Tracing
    dify/                     # RAG platform + setup Job
    observability/            # Prometheus + Grafana + Pushgateway + node-exporter + kube-state-metrics
    rag-eval/                 # Smoke test + Ragas CronJob + Quality gate
    security/                 # LLM-Guard + NetworkPolicy
    keycloak/                 # SSO
    logging/                  # Loki + Fluent Bit
    finetune/                 # LLaMA-Factory + Argo Workflows + MLflow
    fluid/                    # MinIO
    keda/                     # KEDA autoscaling (multi-trigger)
images/
  model-loader/               # Pre-built model downloader (minio + huggingface_hub + hf-transfer)
  model-resolver/             # Engine auto-detection logic
tests/e2e/                    # Playwright E2E tests
tests/e2e/test_finetune_e2e.py  # Finetune pipeline E2E (MLflow + Argo + QG)
tests/helm/                   # Helm template unit tests (pytest)
tests/load/                   # Load testing scripts
examples/
  curl/                       # API curl examples
  python/                     # Python SDK examples
  eval/                       # Evaluation datasets
  finetune/                   # Sample training data + example values
docs/
  routing.md                  # Routing strategies + prefix caching
  large-model-deployment.md   # Multi-GPU, quantization, MIG
  speculative-decoding.md     # Draft model configuration
  kserve-integration.md       # KServe coexistence guide
  disaggregated-serving.md    # llm-d architecture + configuration
  model-updates.md            # Canary deployment flow
```
