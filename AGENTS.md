# AGENTS.md — Project Knowledge for AI Assistants

## Project Overview
kube-llmops is a Kubernetes-native LLMOps platform using Umbrella Helm Charts.
Deploy, manage, monitor, and optimize LLM infrastructure with a single `helm install`.

## Key Commands

```bash
# Deploy (single-node with GPU)
helm install kube-llmops charts/kube-llmops-stack -f charts/kube-llmops-stack/values-single-node.yaml

# Upgrade
helm upgrade kube-llmops charts/kube-llmops-stack -f charts/kube-llmops-stack/values-single-node.yaml

# IMPORTANT: After changing any subchart template, rebuild archives:
cd charts/kube-llmops-stack && rm -f charts/*.tgz Chart.lock && helm dependency update .

# Run Playwright E2E tests
uv run tests/e2e/test_dify_model_provider.py
uv run tests/e2e/test_dify_rag_e2e.py

# Trigger Ragas evaluation
kubectl create job ragas-manual --from=cronjob/kube-llmops-ragas-eval

# Check smoke test
kubectl logs -l app.kubernetes.io/name=rag-smoke-test --tail=30

# Check quality gate
kubectl logs job/kube-llmops-quality-gate
```

## Architecture

```
┌─ Ingress (Traefik) ─────────────────────────────┐
│  *.llmops.local → litellm/grafana/langfuse/dify  │
├──────────────────────────────────────────────────┤
│ LiteLLM (Gateway:4000) → vLLM (LLM:8000)        │
│                        → TEI (Embed:8080)         │
│                        → TEI (Rerank:8080)        │
│ Dify (RAG:5001/3000) → LiteLLM → pgvector        │
│ Langfuse (Trace:3000) ← LiteLLM callbacks         │
│ Prometheus:9090 + Pushgateway:9091 → Grafana:3000 │
│ LLM-Guard (Security:8000)                         │
│ Keycloak (SSO:8080)                               │
│ MinIO (S3:9000) + PostgreSQL:5432                  │
└──────────────────────────────────────────────────┘
```

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
- Plugin daemon `uv sync` runs offline — pre-install deps via shared PVC before triggering install
- Frontend sends base64-encoded passwords in login requests

### LLM-Guard
- Needs ~6GB RAM for PromptInjection scanner model
- Config file at `/home/user/app/config/scanners.yml` — mount via ConfigMap
- Default config enables ALL scanners (Sentiment scanner has ZeroDivisionError bug)
- Auth via `Authorization: Bearer <token>` header, token in config yml

### PostgreSQL
- Image: `pgvector/pgvector:pg16` (NOT `postgres:16-alpine`)
- Init script auto-creates databases: litellm, langfuse, dify, dify_plugin
- Auto-enables `vector` extension in each DB

## Test Coverage

| Test | Tool | Count | What it validates |
|------|------|-------|-------------------|
| Model Provider | Playwright | 5/5 | Login + add LLM + embedding + verify |
| RAG E2E | Playwright | 9/9 | KB → upload → index → chat → verify answer |
| Smoke Test | K8s Job | 5/5 | Embedding + LLM + Langfuse + trace + reranker |
| Ragas Eval | K8s CronJob | 4 metrics | faithfulness, relevancy, precision, recall |
| Quality Gate | Helm Hook | pass/block | Pre-upgrade check on Ragas thresholds |
| LLM-Guard | Manual | 4/4 | Normal + direct injection + subtle + benign |

## File Layout

```
charts/kube-llmops-stack/
  values-single-node.yaml     # Main config for single GPU node
  charts/
    vllm/                     # Model serving (PagedAttention)
    tei/                      # Embedding + reranking
    litellm/                  # AI Gateway
    langfuse/                 # Tracing
    dify/                     # RAG platform + setup Job
    observability/            # Prometheus + Grafana + Pushgateway
    rag-eval/                 # Smoke test + Ragas CronJob + Quality gate
    security/                 # LLM-Guard + NetworkPolicy
    keycloak/                 # SSO
    logging/                  # Loki + Fluent Bit
    fluid/                    # MinIO
tests/e2e/                    # Playwright E2E tests
examples/eval/                # Evaluation datasets
```
