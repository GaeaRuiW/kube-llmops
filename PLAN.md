# kube-llmops Implementation Plan

> Last updated: 2026-03-18
>
> This plan covers **Phase 1 (MVP)** and **Phase 2 (Production Readiness)** in detail.
> Phase 3-6 are outlined at the end, detailed planning happens when we get there.

---

## CI/CD Strategy

> CI from Day 0, growing with each Milestone. CD (ArgoCD/GitOps) deferred to Phase 3.

### Core Principle

Every Milestone adds components. Every new component gets its own CI checks **in the same PR that adds the component**. CI is not an afterthought -- it's a gate.

### Pipeline Overview

```
PR Opened / Push to main
        |
        v
┌─────────────────────────────────────────────────────────┐
│  Workflow 1: lint.yaml  (fast, <2min)                   │
│  - helm lint (all charts)                               │
│  - ct lint (chart-testing)                              │
│  - yamllint (values, OTel config, alert rules)          │
│  - shellcheck (scripts/)                                │
│  - markdownlint (docs/, README, PLAN)                   │
│  - ruff (Python: model-resolver, model-loader)    [M2+] │
└─────────────────────────────────────────────────────────┘
        |
        v
┌─────────────────────────────────────────────────────────┐
│  Workflow 2: test.yaml  (medium, <5min)                 │
│  - helm template --dry-run (render all profiles)        │
│  - Python unit tests (pytest: format detection)   [M2+] │
│  - OTel config validation (otelcol validate)      [M5+] │
│  - Prometheus rules check (promtool check rules)  [M5+] │
│  - Grafana dashboard JSON schema check            [M5+] │
│  - LiteLLM config YAML schema check              [M4+] │
└─────────────────────────────────────────────────────────┘
        |
        v
┌─────────────────────────────────────────────────────────┐
│  Workflow 3: build.yaml  (medium, <10min)               │
│  - Docker build: model-resolver               [M2+]    │
│  - Docker build: model-loader                 [M1+]    │
│  - Docker build: rag-worker                   [P3+]    │
│  - Trivy scan on all built images                       │
│  - Helm package (build .tgz, don't push)                │
└─────────────────────────────────────────────────────────┘
        |
        v  (only on main branch merge, weekly, or manual)
┌─────────────────────────────────────────────────────────┐
│  Workflow 4: e2e.yaml  (slow, ~15-30min)                │
│  - Create kind cluster (no GPU for CI)                  │
│  - helm install with values-ci.yaml (CPU-only mode)     │
│  - Smoke test: LiteLLM /health, /v1/models              │
│  - Smoke test: Prometheus has scrape targets             │
│  - Smoke test: Grafana dashboards load                  │
│  - Smoke test: Langfuse /api/public/health              │
│  - helm uninstall (clean teardown)                      │
│  - kind delete                                          │
└─────────────────────────────────────────────────────────┘
        |
        v  (only on git tag v*)
┌─────────────────────────────────────────────────────────┐
│  Workflow 5: release.yaml                               │
│  - Build + push Docker images to GHCR                   │
│  - Helm package + push to GitHub Pages / OCI registry   │
│  - Generate CHANGELOG from conventional commits         │
│  - Create GitHub Release with notes                     │
└─────────────────────────────────────────────────────────┘
```

### CI Growth Per Milestone

| Milestone | New CI Checks Added |
|---|---|
| **M0** | `lint.yaml` (helm lint, ct lint, yamllint, shellcheck, markdownlint), `build.yaml` (helm package) |
| **M1** | Docker build `model-loader`, `helm template` with values-minimal |
| **M2** | Docker build `model-resolver`, `ruff` Python lint, `pytest` unit tests for format detection |
| **M3** | `helm template` multi-engine rendering (verify vllm/llamacpp/tei all render) |
| **M4** | LiteLLM config schema validation, `e2e.yaml` foundation (kind + install + smoke test) |
| **M5** | `promtool check rules`, Grafana JSON schema, OTel config validation (`otelcol validate`) |
| **M6** | E2E: Langfuse health check added to smoke test |
| **M8** | KEDA ScaledObject schema validation |
| **MVP (v0.1.0)** | `release.yaml` (GHCR push, Helm package push, GitHub Release) |

### values-ci.yaml (CPU-only mode for CI)

```yaml
# Special values for CI/E2E testing on kind (no GPU)
global:
  gpu: false

models:
  - name: tiny-model
    source: hf-internal-testing/tiny-random-LlamaForCausalLM  # 10MB dummy model
    engine: vllm
    resources:
      gpu: 0
      cpu: 2
      memory: 4Gi
    engineArgs:
      --device: cpu
      --dtype: float32

litellm:
  enabled: true
  masterKey: sk-ci-test-key

observability:
  enabled: true
  dcgmExporter:
    enabled: false     # no GPU in CI

langfuse:
  enabled: true

grafana:
  enabled: true

# Everything else disabled for fast CI
fluid:
  enabled: false
harbor:
  enabled: false
keycloak:
  enabled: false
milvus:
  enabled: false
```

---

## Dependency Graph

```
M0 Project Scaffolding + CI Foundation
 |   (lint.yaml + test.yaml + build.yaml -- run from here on every PR)
 |
 v
M1 Single Model on vLLM        (prove: we can deploy a model)
 |   + CI: Docker build model-loader, helm template test
 |
 ├──> M2 Model Resolver         (prove: auto engine selection works)
 │     |  + CI: ruff lint, pytest unit tests, Docker build model-resolver
 │     v
 │    M3 llama.cpp + TEI charts (prove: multi-engine works)
 │        + CI: multi-engine template rendering test
 |
 v
M4 LiteLLM Gateway              (prove: unified API + key management)
 |   + CI: config validation, e2e.yaml (kind + install + smoke test)
 |
 v
M5 Observability - Metrics      (prove: Prometheus + DCGM + Grafana works)
 |   + CI: promtool, OTel validate, dashboard JSON check
 |
 v
M6 Observability - Tracing      (prove: Langfuse shows full trace)
 |   + CI: Langfuse health in e2e smoke test
 |
 v
M6.5 Release Pipeline           (prove: tag -> automated release)
 |   + CI: release.yaml (GHCR push, Helm publish, GitHub Release)
 |
 v
========== CHECKPOINT: MVP Release (v0.1.0) ==========
 |   All 5 workflows green: lint + test + build + e2e + release
 |
 v
M7  Logging (Fluentbit + Loki)
M8  Autoscaling (KEDA)
M9  Distributed Model Cache (Fluid)
M10 Model Registry (Harbor) + Object Storage (MinIO)
M11 Security (Keycloak + Cilium)
 |
 v
========== CHECKPOINT: Production Release (v0.2.0) ==========
 |
 v
Phase 3+: CD (ArgoCD manifests, ApplicationSet, GitOps workflow)
```

---

## Milestone 0: Project Scaffolding + CI Foundation

**Goal**: Repo structure, CI pipelines, tooling -- `helm lint` passes, all 3 CI workflows run green on empty chart.

### Tasks

| # | Task | Output |
|---|---|---|
| 0.1 | `git init`, LICENSE (Apache 2.0), .gitignore | Clean repo |
| 0.2 | Create directory structure per ARCHITECTURE.md | `charts/`, `dashboards/`, `images/`, `docs/`, `scripts/`, `alerting/`, `otel/`, `manifests/`, `examples/`, `terraform/` |
| 0.3 | Umbrella Helm chart scaffold | `charts/kube-llmops-stack/Chart.yaml`, `values.yaml`, `templates/_helpers.tpl` |
| 0.4 | Makefile | Targets: `lint`, `test`, `template`, `build`, `package`, `e2e`, `clean` |
| 0.5 | README.md skeleton | Project description, architecture diagram (text), quick start placeholder |
| 0.6 | CONTRIBUTING.md | Dev setup, PR process, chart development guide |
| 0.7 | ct (chart-testing) config | `ct.yaml` for `helm/chart-testing` |
| **CI** | | |
| 0.8 | `.github/workflows/lint.yaml` | Triggered on PR + push to main |
|     | - `helm lint charts/kube-llmops-stack/` | |
|     | - `ct lint --config ct.yaml` | |
|     | - `yamllint` on all `.yaml` / `.yml` files | |
|     | - `shellcheck scripts/*.sh` | |
|     | - `markdownlint '**/*.md'` | |
| 0.9 | `.github/workflows/test.yaml` | Triggered on PR + push to main |
|     | - `helm template kube-llmops charts/kube-llmops-stack/ -f values-minimal.yaml` (dry-run render) | |
|     | - Validate rendered YAML is parseable (`kubectl apply --dry-run=client`) | |
| 0.10 | `.github/workflows/build.yaml` | Triggered on PR + push to main |
|     | - `helm package charts/kube-llmops-stack/` (build .tgz, don't push) | |
| 0.11 | `.yamllint.yml` config | Relaxed rules (line-length: 200, allow truthy values) |
| 0.12 | `.markdownlint.json` config | Disable rules that conflict with tables |
| 0.13 | `values-ci.yaml` | CPU-only values for CI E2E (see CI/CD Strategy section above) |

### Validation

```bash
# Local checks:
make lint         # exit 0, no errors
make template     # helm template renders without error
make package      # produces .tgz file

# CI checks (push to GitHub, verify):
# 1. lint.yaml    -> green (helm lint + yamllint + shellcheck + markdownlint)
# 2. test.yaml    -> green (helm template dry-run)
# 3. build.yaml   -> green (helm package)
```

---

## Milestone 1: Single Model on vLLM

**Goal**: `helm install` deploys one vLLM instance, model loads, OpenAI API responds.

### Tasks

| # | Task | Output |
|---|---|---|
| 1.1 | vLLM Helm sub-chart | `charts/kube-llmops-stack/charts/vllm/` |
|     | - Deployment with configurable model, GPU resources, engine args | |
|     | - Service (ClusterIP, port 8000) | |
|     | - Readiness probe (`/health`, `initialDelaySeconds: 120`) | |
|     | - Configurable `nodeSelector`, `tolerations`, `affinity` for GPU | |
|     | - PVC mount point for model weights | |
| 1.2 | model-loader init container | `images/model-loader/` |
|     | - Python script: download from HuggingFace / ModelScope / S3 / OCI | |
|     | - Skip if already cached on PVC | |
|     | - Support `HF_TOKEN` for gated models | |
|     | - Dockerfile (slim, <100MB) | |
| 1.3 | Wire vLLM sub-chart into umbrella chart | `Chart.yaml` dependency, `values.yaml` defaults |
| 1.4 | `values-minimal.yaml` (first draft) | Single model (e.g. `Qwen/Qwen2.5-0.5B-Instruct`), 1 GPU, no monitoring |
| **CI** | | |
| 1.5 | `build.yaml`: add Docker build for `model-loader` | Build image, Trivy scan, don't push |
| 1.6 | `test.yaml`: add `helm template` with `values-minimal.yaml` | Ensure vLLM sub-chart renders correctly |

### Validation: CHECKPOINT-1

```bash
# Prerequisites: K8s cluster with 1 GPU node
helm install kube-llmops charts/kube-llmops-stack/ -f values-minimal.yaml

# Wait for pod ready (may take 2-5 min for model download)
kubectl wait --for=condition=ready pod -l app=vllm --timeout=600s

# Test OpenAI-compatible API
kubectl port-forward svc/vllm 8000:8000 &
curl -s http://localhost:8000/v1/models | jq .

curl -X POST http://localhost:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "Qwen2.5-0.5B-Instruct",
    "messages": [{"role": "user", "content": "Hello!"}],
    "max_tokens": 50
  }'

# Pass criteria:
# 1. Pod STATUS = Running, READY = 1/1
# 2. /v1/models returns model list
# 3. /v1/chat/completions returns valid response with generated text
# 4. helm uninstall cleanly removes all resources
```

---

## Milestone 2: Model Resolver (Engine Auto-Selection)

**Goal**: User specifies model ID, platform auto-detects format and picks engine.

### Tasks

| # | Task | Output |
|---|---|---|
| 2.1 | `format_detector.py` | Given model ID, return: format (gguf/safetensors/etc), quant method, model type (chat/embedding/reranker) |
|     | - Parse `config.json` from HF Hub API (no full download) | |
|     | - Scan file listing for `.gguf` / `.safetensors` | |
|     | - Detect `quantization_config.quant_method` | |
|     | - Detect embedding/reranker via model architecture | |
| 2.2 | `hardware_probe.py` | Detect available GPUs (count, type, VRAM) via `nvidia-smi` |
| 2.3 | `engine_map.yaml` | Configurable mapping rules: format+hardware -> engine+args |
| 2.4 | `resolver.py` (main) | Orchestrate: detect format -> probe hardware -> lookup engine map -> write `/resolve/engine.env` |
| 2.5 | Dockerfile for model-resolver | `images/model-resolver/Dockerfile` |
| 2.6 | Integrate into Helm templates | vLLM Deployment template reads `/resolve/engine.env`, conditionally uses vLLM/llama.cpp/TEI image+args |
| **CI** | | |
| 2.7 | `lint.yaml`: add `ruff check images/` | Python lint for model-resolver + model-loader |
| 2.8 | `test.yaml`: add `pytest images/model-resolver/tests/` | Unit tests for format detection: AWQ->vLLM, GGUF->llamacpp, embedding->TEI, override->skip |
| 2.9 | `build.yaml`: add Docker build for `model-resolver` | Build image, Trivy scan |

### Validation: CHECKPOINT-2

```bash
# Test 1: AWQ model -> should auto-select vLLM
helm install test1 charts/kube-llmops-stack/ \
  --set models[0].name=qwen-awq \
  --set models[0].source=Qwen/Qwen2.5-0.5B-Instruct  # SafeTensors, no quant

kubectl logs <pod> -c model-resolver
# Expected output: ENGINE=vllm, ENGINE_ARGS=--gpu-memory-utilization 0.92

# Test 2: GGUF model -> should auto-select llama.cpp
helm install test2 charts/kube-llmops-stack/ \
  --set models[0].name=llama-gguf \
  --set models[0].source=bartowski/Meta-Llama-3.1-8B-Instruct-GGUF

kubectl logs <pod> -c model-resolver
# Expected output: ENGINE=llamacpp, ENGINE_ARGS=--ctx-size 4096 ...

# Test 3: Explicit override -> should skip detection
helm install test3 charts/kube-llmops-stack/ \
  --set models[0].name=custom \
  --set models[0].source=my-model \
  --set models[0].engine=sglang

# Expected: uses SGLang directly, resolver logs "engine override: sglang"

# Pass criteria:
# 1. AWQ/GPTQ models resolve to vLLM with correct --quantization arg
# 2. GGUF models resolve to llama.cpp
# 3. Embedding models resolve to TEI
# 4. Explicit engine override skips auto-detection
# 5. No GPU available -> resolves to llama.cpp CPU mode
```

---

## Milestone 3: llama.cpp + TEI Sub-charts

**Goal**: Multi-engine serving works -- all engines deployable through the same Helm interface.

### Tasks

| # | Task | Output |
|---|---|---|
| 3.1 | llama.cpp Helm sub-chart | `charts/kube-llmops-stack/charts/llamacpp/` |
|     | - Deployment with `ghcr.io/ggerganov/llama.cpp:server` | |
|     | - Support: `--model`, `--ctx-size`, `--n-gpu-layers`, `--host`, `--port` | |
|     | - OpenAI-compatible endpoint (`/v1/chat/completions`) | |
| 3.2 | TEI Helm sub-chart | `charts/kube-llmops-stack/charts/tei/` |
|     | - Deployment with `ghcr.io/huggingface/text-embeddings-inference` | |
|     | - Support embedding mode and reranker mode | |
|     | - Endpoint: `/embed`, `/rerank`, and OpenAI `/v1/embeddings` | |
| 3.3 | Unified Helm interface | All 3 engines (vllm, llamacpp, tei) use same values.yaml schema: `models[].name`, `models[].source`, `models[].engine`, `models[].resources.gpu` |
| 3.4 | Multi-model deployment | Support deploying N models in one release, each auto-selecting engine |
| **CI** | | |
| 3.5 | `test.yaml`: multi-engine template test | `helm template` with a multi-model values file, verify 3 Deployments rendered with correct images (vllm, llama.cpp, TEI) |

### Validation: CHECKPOINT-3

```bash
# Deploy 3 models simultaneously, each using different engine
helm install multi charts/kube-llmops-stack/ -f test-multi-model.yaml
# values contains:
#   models:
#     - name: chat-model     source: Qwen/Qwen2.5-0.5B-Instruct   (-> vLLM)
#     - name: gguf-model     source: ...some-GGUF-model...          (-> llama.cpp)
#     - name: embed-model    source: BAAI/bge-small-en-v1.5         (-> TEI)

# Pass criteria:
# 1. 3 separate Deployments created, each using correct container image
# 2. chat-model: vLLM image, responds to /v1/chat/completions
# 3. gguf-model: llama.cpp image, responds to /v1/chat/completions
# 4. embed-model: TEI image, responds to /v1/embeddings
# 5. All 3 models accessible via their own Service
```

---

## Milestone 4: LiteLLM Gateway

**Goal**: Unified OpenAI-compatible API fronting all models, with API key management.

### Tasks

| # | Task | Output |
|---|---|---|
| 4.1 | LiteLLM Helm sub-chart | `charts/kube-llmops-stack/charts/litellm/` |
|     | - Deployment with `ghcr.io/berriai/litellm:main-stable` | |
|     | - PostgreSQL dependency (Bitnami sub-chart) | |
|     | - ConfigMap from `litellm_config.yaml` (generated from values) | |
|     | - Service (ClusterIP, port 4000) | |
|     | - Readiness probe (`/health/liveliness`) | |
| 4.2 | Config generation template | Helm template that converts `models[]` from values.yaml into LiteLLM `model_list` config |
|     | - Auto-wire: each deployed model -> LiteLLM backend entry | |
|     | - Virtual model name mapping (user-facing name -> internal service) | |
| 4.3 | Ingress template | NGINX Ingress with LLM-optimized timeouts (read: 120s, proxy-buffering: off for streaming) |
| 4.4 | API key bootstrap Job | K8s Job that creates initial master key + one team + one user key, outputs to logs |
| 4.5 | `values.yaml`: gateway section | `gateway.enabled`, `gateway.masterKey`, `gateway.defaultBudget`, `gateway.rateLimits` |
| **CI** | | |
| 4.6 | `test.yaml`: LiteLLM config validation | Generate config from values, validate YAML schema (model_list required, router_settings valid) |
| 4.7 | `e2e.yaml`: **first E2E workflow** | kind cluster + `helm install -f values-ci.yaml` + wait for pods + `curl /health` + `curl /v1/models` + teardown |

### Validation: CHECKPOINT-4 (First "wow moment")

```bash
helm install kube-llmops charts/kube-llmops-stack/ -f values-minimal.yaml
# Now deploys: vLLM (or auto-selected engine) + LiteLLM + PostgreSQL

# Get LiteLLM service
kubectl port-forward svc/litellm 4000:4000 &

# List available models (via gateway, not direct vLLM)
curl -s http://localhost:4000/v1/models \
  -H "Authorization: Bearer sk-master-xxx" | jq .

# Chat via gateway
curl -X POST http://localhost:4000/v1/chat/completions \
  -H "Authorization: Bearer sk-master-xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen2-5-0-5b",
    "messages": [{"role": "user", "content": "What is Kubernetes?"}],
    "max_tokens": 100
  }'

# Create API key with budget
curl -X POST http://localhost:4000/key/generate \
  -H "Authorization: Bearer sk-master-xxx" \
  -d '{"max_budget": 10.0, "duration": "30d"}'

# Use the generated key
curl -X POST http://localhost:4000/v1/chat/completions \
  -H "Authorization: Bearer sk-user-yyy" \
  -H "Content-Type: application/json" \
  -d '{"model": "qwen2-5-0-5b", "messages": [{"role": "user", "content": "Hi"}]}'

# Pass criteria:
# 1. LiteLLM pod Running, connected to PostgreSQL
# 2. /v1/models returns model list through gateway
# 3. /v1/chat/completions works with master key
# 4. Can generate user key with budget
# 5. User key works for chat, invalid key gets 401
# 6. Streaming (SSE) works: curl with -N flag shows token-by-token output
```

---

## Milestone 5: Observability - Metrics

**Goal**: Prometheus collects vLLM + GPU metrics via OTel Collector. Grafana shows 3 dashboards.

### Tasks

| # | Task | Output |
|---|---|---|
| 5.1 | OTel Collector Helm sub-chart | `charts/kube-llmops-stack/charts/observability/` |
|     | - Deployment (or DaemonSet for logs) | |
|     | - ConfigMap with receiver/processor/exporter config | |
|     | - Prometheus receiver: scrape vLLM `/metrics`, DCGM Exporter | |
|     | - Prometheus remote write exporter | |
| 5.2 | Prometheus sub-chart | Use `kube-prometheus-stack` or standalone Prometheus |
|     | - Configured to receive remote write from OTel Collector | |
|     | - Recording rules for LLM metrics (TTFT percentiles, throughput) | |
| 5.3 | DCGM Exporter | DaemonSet on GPU nodes, expose GPU hardware metrics |
| 5.4 | Grafana sub-chart | Bundled with 3 dashboards as ConfigMaps (provisioned automatically) |
| 5.5 | Dashboard: LLM Cluster Overview | `dashboards/vllm-overview.json` |
|     | - Total requests/sec, active models count, total tokens/sec | |
|     | - Cluster-wide GPU utilization, error rate, model status | |
| 5.6 | Dashboard: GPU Fleet | `dashboards/gpu-overview.json` |
|     | - Per-node GPU util%, VRAM, temperature, power draw | |
|     | - XID error counter, MIG status | |
| 5.7 | Dashboard: Per-Model Deep Dive | `dashboards/litellm-gateway.json` |
|     | - TTFT P50/P95/P99 timeseries | |
|     | - Token throughput (input/output), batch size, KV cache utilization | |
|     | - Pending requests queue depth | |
| 5.8 | vLLM OTel integration | Enable `--otlp-traces-endpoint` on vLLM, point to OTel Collector |
| **CI** | | |
| 5.9 | `test.yaml`: OTel config validation | Run `otelcol validate --config=otel/collector-config.yaml` |
| 5.10 | `test.yaml`: Prometheus rules validation | Run `promtool check rules alerting/*.yaml` |
| 5.11 | `test.yaml`: Grafana dashboard JSON validation | Validate JSON syntax + required fields (title, panels, datasource) for each dashboard |
| 5.12 | `e2e.yaml`: add observability smoke tests | Verify Prometheus `/api/v1/targets` has active targets, Grafana `/api/health` returns OK |

### Validation: CHECKPOINT-5

```bash
helm install kube-llmops charts/kube-llmops-stack/ -f values-minimal.yaml
# Now deploys: Engine + LiteLLM + PG + OTel Collector + Prometheus + DCGM + Grafana

# Send some traffic
for i in $(seq 1 20); do
  curl -s http://localhost:4000/v1/chat/completions \
    -H "Authorization: Bearer sk-master-xxx" \
    -d '{"model":"qwen2-5-0-5b","messages":[{"role":"user","content":"Count to 10"}]}' &
done
wait

# Check Prometheus has metrics
kubectl port-forward svc/prometheus 9090:9090 &
curl -s 'http://localhost:9090/api/v1/query?query=vllm_num_requests_running' | jq .
curl -s 'http://localhost:9090/api/v1/query?query=DCGM_FI_DEV_GPU_UTIL' | jq .

# Check Grafana dashboards
kubectl port-forward svc/grafana 3000:3000 &
# Open http://localhost:3000, login admin/admin

# Pass criteria:
# 1. Prometheus has vllm_* metrics (>0 datapoints)
# 2. Prometheus has DCGM_FI_* metrics (GPU util, VRAM, temp)
# 3. Grafana: "LLM Cluster Overview" dashboard loads, shows request rate > 0
# 4. Grafana: "GPU Fleet" dashboard loads, shows GPU utilization graph
# 5. Grafana: "Per-Model Deep Dive" dashboard loads, shows TTFT histogram
# 6. No OTel Collector errors in logs
```

---

## Milestone 6: Observability - Tracing (Langfuse)

**Goal**: Every LLM request is fully traced in Langfuse with prompt, tokens, cost, latency.

### Tasks

| # | Task | Output |
|---|---|---|
| 6.1 | Langfuse Helm sub-chart | `charts/kube-llmops-stack/charts/langfuse/` |
|     | - Deployment (Langfuse server) | |
|     | - PostgreSQL (reuse LiteLLM's PG, separate database) | |
|     | - Service (port 3000) | |
|     | - Init Job: create default project + API keys | |
| 6.2 | LiteLLM -> Langfuse callback integration | Set `success_callback: ["langfuse"]` in LiteLLM config |
|     | - Inject LANGFUSE_PUBLIC_KEY, SECRET_KEY, HOST from Secret | |
| 6.3 | OTel Collector -> Langfuse exporter | Add `otlphttp/langfuse` exporter for vLLM/Envoy traces |
| 6.4 | values.yaml: tracing section | `tracing.enabled`, `tracing.langfuse.host`, `tracing.langfuse.publicKey` |
| **CI** | | |
| 6.5 | `e2e.yaml`: add Langfuse smoke test | Verify Langfuse `/api/public/health` returns OK after install |

### Validation: CHECKPOINT-6 -- **MVP Gate**

```bash
helm install kube-llmops charts/kube-llmops-stack/ -f values-minimal.yaml
# Full stack: Engine + LiteLLM + PG + OTel + Prometheus + DCGM + Grafana + Langfuse

# Send a request
curl -X POST http://localhost:4000/v1/chat/completions \
  -H "Authorization: Bearer sk-master-xxx" \
  -d '{
    "model": "qwen2-5-0-5b",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "Explain Kubernetes in 3 sentences."}
    ],
    "max_tokens": 200
  }'

# Open Langfuse UI
kubectl port-forward svc/langfuse 3001:3000 &
# Open http://localhost:3001

# Pass criteria:
# 1. Langfuse UI shows the trace
# 2. Trace contains:
#    - Full input prompt ("Explain Kubernetes in 3 sentences.")
#    - Full output text
#    - Input token count + Output token count
#    - TTFT (time to first token)
#    - Total latency
#    - Cost calculation
#    - Model name
# 3. Multiple requests -> Langfuse shows list of all traces
# 4. Can filter traces by model, user, time range
```

---

## Milestone 6.5: Release Pipeline (CI -> Release)

**Goal**: `git tag v0.1.0` triggers full automated release -- images pushed, chart published, GitHub Release created.

### Tasks

| # | Task | Output |
|---|---|---|
| 6.5.1 | `.github/workflows/release.yaml` | Triggered on `v*` tag push |
|       | - Build all Docker images with version tag | |
|       | - Push images to `ghcr.io/xxx/kube-llmops/*` | |
|       | - `helm package` with chart version from tag | |
|       | - Push chart to GitHub Pages (`gh-pages` branch) or OCI registry | |
|       | - Generate changelog (from conventional commits or git log) | |
|       | - Create GitHub Release with changelog + artifact links | |
| 6.5.2 | Helm chart `Chart.yaml`: use `appVersion` from tag | Makefile target `make release VERSION=0.1.0` updates Chart.yaml |
| 6.5.3 | Image tagging strategy | `latest` + `v0.1.0` + git SHA. Helm values default to chart `appVersion`. |
| 6.5.4 | `scripts/install.sh` | One-liner: adds Helm repo + installs chart with `values-minimal.yaml` |
| 6.5.5 | Helm repo index | `gh-pages` branch with `index.yaml`, or OCI push to GHCR |

### Validation

```bash
# Simulate release:
git tag v0.1.0-rc1
git push origin v0.1.0-rc1

# Verify in GitHub Actions:
# 1. release.yaml triggers
# 2. Docker images appear in ghcr.io/xxx/kube-llmops/model-resolver:v0.1.0-rc1
# 3. Docker images appear in ghcr.io/xxx/kube-llmops/model-loader:v0.1.0-rc1
# 4. Helm chart installable: helm repo add kube-llmops https://xxx.github.io/kube-llmops
#    helm install test kube-llmops/kube-llmops-stack --version 0.1.0-rc1
# 5. GitHub Release draft created with changelog

# After validation, tag actual release:
git tag v0.1.0
git push origin v0.1.0
```

---

## ========== MVP Release: v0.1.0 ==========

### What's included

```
helm install kube-llmops charts/kube-llmops-stack/ -f values-minimal.yaml
```

Deploys:
- [x] Model serving (vLLM / llama.cpp / TEI, auto-selected by Model Resolver)
- [x] AI Gateway (LiteLLM + PostgreSQL, API key management)
- [x] Metrics (OTel Collector + Prometheus + DCGM Exporter + Grafana + 3 dashboards)
- [x] Tracing (Langfuse, full prompt/token/cost visibility)

### Release Checklist

| # | Item | Status |
|---|---|---|
| R.1 | All 6 checkpoints (M1-M6) pass | |
| R.2 | `values-minimal.yaml` tested end-to-end on fresh cluster | |
| R.3 | README.md: quick start guide works copy-paste | |
| R.4 | `scripts/install.sh` one-liner works | |
| R.5 | Helm chart passes `ct lint` and `ct install` | |
| R.6 | **All 4 CI workflows green** (lint + test + build + e2e) | |
| R.7 | **`release.yaml` workflow**: Docker images pushed to GHCR | |
| R.8 | **`release.yaml` workflow**: Helm chart pushed to OCI / GitHub Pages | |
| R.9 | **`release.yaml` workflow**: GitHub Release created with changelog | |
| R.10 | `helm uninstall` cleanly removes everything | |
| R.11 | docs/getting-started.md complete | |
| R.12 | CHANGELOG.md for v0.1.0 | |

### MVP Demo Script

```bash
# 0. Prerequisites
# - K8s cluster (minikube/kind with GPU, or cloud with GPU node)
# - helm 3.x installed

# 1. Install (< 1 minute)
helm repo add kube-llmops https://xxx.github.io/kube-llmops
helm install kube-llmops kube-llmops/kube-llmops-stack -f values-minimal.yaml

# 2. Wait for ready (2-5 min, model download)
kubectl get pods -w

# 3. Chat with model
export GATEWAY=$(kubectl get svc litellm -o jsonpath='{.spec.clusterIP}')
curl $GATEWAY:4000/v1/chat/completions \
  -H "Authorization: Bearer sk-master-xxx" \
  -d '{"model":"qwen2-5-0-5b","messages":[{"role":"user","content":"Hello!"}]}'

# 4. See metrics in Grafana
kubectl port-forward svc/grafana 3000:3000
# Open browser -> 3 dashboards with live data

# 5. See traces in Langfuse
kubectl port-forward svc/langfuse 3001:3000
# Open browser -> every request traced with prompt + tokens + cost

# 6. Cleanup
helm uninstall kube-llmops
```

---

## Milestone 7: Logging (Fluentbit + Loki)

**Goal**: All container logs collected, structured, queryable in Grafana.

### Tasks

| # | Task | Output |
|---|---|---|
| 7.1 | Fluentbit DaemonSet config | Parse vLLM / LiteLLM / llama.cpp logs |
|     | - Forward to OTel Collector via fluentforward protocol | |
| 7.2 | Loki sub-chart | Storage backend for logs |
| 7.3 | OTel Collector: add log pipeline | Receive from Fluentbit, export to Loki |
| 7.4 | Grafana: Loki datasource | Auto-provisioned |
| 7.5 | Dashboard: add log panels | "Explore Logs" link from existing dashboards |

### Validation

```bash
# Search for vLLM engine logs in Grafana Explore
# Query: {app="vllm"} |= "error"
# Pass: logs appear with correct timestamps and labels
```

---

## Milestone 8: Autoscaling (KEDA)

**Goal**: vLLM pods scale up/down based on LLM-specific metrics.

### Tasks

| # | Task | Output |
|---|---|---|
| 8.1 | KEDA Helm dependency | Add KEDA operator as dependency |
| 8.2 | ScaledObject template for vLLM | Trigger on `vllm_num_requests_waiting` and `vllm_time_to_first_token_seconds` (P95) |
| 8.3 | values.yaml: autoscaling section | `autoscaling.enabled`, `minReplicas`, `maxReplicas`, `triggers[]` |
| 8.4 | Scale-to-zero support (optional) | `scaleToZero: true` -- requires fast cold start |

### Validation

```bash
# 1. Baseline: 1 replica running
kubectl get pods -l app=vllm   # 1 pod

# 2. Send burst traffic (100 concurrent requests)
hey -n 100 -c 20 -m POST -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{"model":"qwen2-5-0-5b","messages":[{"role":"user","content":"Write a long story"}],"max_tokens":500}' \
  http://localhost:4000/v1/chat/completions

# 3. Observe scale-up
kubectl get pods -l app=vllm -w   # should see new pods spinning up

# 4. Stop traffic, wait 5 min
kubectl get pods -l app=vllm      # should scale back down

# Pass criteria:
# 1. Pod count increases under load
# 2. Pod count decreases after cooldown
# 3. No requests dropped during scale-up
```

---

## Milestone 9: Distributed Model Cache (Fluid)

**Goal**: Model weights cached across nodes. Pod restart doesn't re-download.

### Tasks

| # | Task | Output |
|---|---|---|
| 9.1 | Fluid operator dependency | Add Fluid CRDs + controller |
| 9.2 | MinIO sub-chart | S3-compatible storage for model weights |
| 9.3 | Dataset + AlluxioRuntime templates | Per-model Fluid Dataset pointing to MinIO |
| 9.4 | model-loader: upload to MinIO | After downloading from HF, also upload to MinIO for caching |
| 9.5 | vLLM mount Fluid PVC | Mount cached weights instead of direct download |

### Validation

```bash
# 1. First deploy: model downloads from HuggingFace, uploads to MinIO, caches via Fluid
time kubectl wait --for=condition=ready pod -l app=vllm --timeout=600s
# Record: T_first = Xm Ys

# 2. Delete pod, let it recreate
kubectl delete pod -l app=vllm
time kubectl wait --for=condition=ready pod -l app=vllm --timeout=600s
# Record: T_cached = Xm Ys

# Pass criteria:
# T_cached < T_first / 3   (at least 3x faster on cache hit)
# Pod logs show "loading from cache" not "downloading from HuggingFace"
```

---

## Milestone 10: Model Registry (Harbor) + Object Storage (MinIO)

**Goal**: Models stored and versioned as OCI artifacts in Harbor.

### Tasks

| # | Task | Output |
|---|---|---|
| 10.1 | Harbor Helm dependency | Harbor core + registry |
| 10.2 | MinIO Helm sub-chart | For Fluid backend + general object storage |
| 10.3 | model-loader: support `oci://` source | Pull model weights from Harbor OCI |
| 10.4 | Model push script | `scripts/push-model.sh` -- download from HF, push to Harbor as OCI artifact |
| 10.5 | docs: model registry guide | How to version and manage model artifacts |

### Validation

```bash
# Push a model to Harbor
./scripts/push-model.sh Qwen/Qwen2.5-0.5B-Instruct harbor.local/models/qwen2-5-0-5b:v1

# Deploy model from Harbor
helm install test charts/kube-llmops-stack/ \
  --set models[0].source=oci://harbor.local/models/qwen2-5-0-5b:v1

# Pass: model loads from Harbor, not from HuggingFace
```

---

## Milestone 11: Security (Keycloak + Cilium)

**Goal**: SSO login for Grafana/Langfuse, network isolation between tenants.

### Tasks

| # | Task | Output |
|---|---|---|
| 11.1 | Keycloak Helm sub-chart | Keycloak server + realm auto-config |
| 11.2 | OIDC integration: Grafana | Grafana login via Keycloak SSO |
| 11.3 | OIDC integration: Langfuse | Langfuse login via Keycloak SSO |
| 11.4 | Cilium NetworkPolicy templates | Isolate model serving, gateway, observability namespaces |
| 11.5 | Multi-tenant namespace template | Per-team: Namespace + ResourceQuota + LiteLLM Team |
| 11.6 | docs: security hardening guide | |

### Validation

```bash
# 1. Grafana: login redirects to Keycloak, SSO works
# 2. Langfuse: login redirects to Keycloak, SSO works
# 3. Cross-namespace traffic blocked by Cilium policy
#    (pod in namespace-A cannot curl vLLM in namespace-B)
```

---

## ========== Production Release: v0.2.0 ==========

### What's added on top of MVP

- [x] Structured logging (Fluentbit + Loki, queryable in Grafana)
- [x] Autoscaling (KEDA, scale on pending requests / TTFT)
- [x] Model cache (Fluid + MinIO, 3x faster cold start)
- [x] Model registry (Harbor, OCI-based versioning)
- [x] Security (Keycloak SSO, Cilium NetworkPolicy)
- [x] `values-standard.yaml` profile

### Release Checklist

| # | Item |
|---|---|
| R.1 | All checkpoints M7-M11 pass |
| R.2 | `values-standard.yaml` tested on multi-GPU cluster |
| R.3 | Grafana: all 6 dashboards populated with data |
| R.4 | Alert rules fire correctly (simulate GPU OOM, high TTFT) |
| R.5 | Load test: 50 concurrent users, no request drops |
| R.6 | Scale test: KEDA scales from 1 to 4 replicas under load |
| R.7 | Cold start test: cached restart < 60s |
| R.8 | Security: unauthorized cross-namespace access blocked |
| R.9 | docs/deployment-guide.md complete |
| R.10 | CHANGELOG.md for v0.2.0 |

---

## Phase 3-6 Outline (详细 plan 在接近时制定)

### Phase 3: RAG & Inference Optimization (v0.3.0)
- pgvector on existing PG
- Milvus standalone
- TEI embedding/reranking serving
- RAG ingestion worker
- Envoy AI Gateway + IGW (Tier 2)
- LoRA adapter routing
- Multi-tenancy via LiteLLM Teams
- `values-production.yaml`
- **CD: ArgoCD integration**
  - ArgoCD Application manifest (`manifests/argocd/app-of-apps.yaml`)
  - Sync Waves for ordered deployment
  - ApplicationSet for multi-cluster
  - docs: GitOps deployment guide
- **CHECKPOINT**: RAG demo -- upload docs, ask questions, get answers with sources
- **CHECKPOINT**: `kubectl apply -f argocd-app.yaml` deploys full stack via GitOps

### Phase 4: ML Platform (v0.4.0)
- JupyterHub on K8s
- LLaMA-Factory fine-tuning Jobs
- MLflow experiment tracking
- ArgoCD ApplicationSet (multi-cluster)
- Terraform modules
- **CHECKPOINT**: Fine-tune a model, deploy it, serve traffic -- full lifecycle

### Phase 5: Advanced Inference (v0.5.0)
- llm-d integration
- Disaggregated prefill/decode
- Expert Parallelism (MoE)
- KV cache tiered offloading
- **CHECKPOINT**: DeepSeek-R1 (MoE) serving with P/D split, benchmark vs baseline

### Phase 6: Ecosystem (v1.0.0)
- Kubernetes Operator + CRDs
- CLI tool
- Web Dashboard
- **CHECKPOINT**: `kubectl llmops deploy qwen3.5-122b` works end-to-end

---

## Task Execution Order (Recommended)

For a single developer working full-time:

```
Week 1:     M0 (scaffolding + CI foundation: lint.yaml, test.yaml, build.yaml)
            M1 (vLLM chart + model-loader)
Week 2:     M2 (model resolver + pytest unit tests)
            M3 (llama.cpp + TEI)
Week 3:     M4 (LiteLLM gateway + e2e.yaml on kind)
Week 4:     M5 (OTel + Prometheus + DCGM + 3 Grafana dashboards)
Week 5:     M6 (Langfuse) + M6.5 (release.yaml) + MVP polish + docs
            ---> v0.1.0 MVP Release (git tag v0.1.0, automated release)

Week 6:     M7 (logging) + M8 (KEDA autoscaling)
Week 7:     M9 (Fluid cache) + M10 (Harbor + MinIO)
Week 8:     M11 (Keycloak + Cilium) + production polish
            ---> v0.2.0 Production Release
```

For parallel development (2+ people):

```
Developer A (infra + CI):     M0 -> M1 -> M5 -> M6.5 -> M7 -> M9 -> M10
Developer B (app + features): M0 -> M2 -> M3 -> M4 -> M6 -> M8 -> M11
```

---

## Known Gaps & Backlog

> All identified gaps from CTO / Architect / PM / Project Manager / DevOps review.
> Categorized by when to address. Record now, fix later -- don't block development.

### Before MVP (v0.1.0) -- Must Do

These directly impact whether someone will star the repo.

| # | Gap | What to do | Where |
|---|---|---|---|
| G1 | **README is the storefront** | Write a killer README: one-sentence hook, architecture diagram (image, not ASCII), feature highlights, GIF/screenshot of Grafana dashboard + Langfuse trace, 5-minute quick start. This is the single most important file for earning stars. | `README.md` |
| G2 | **Quick Start needs a no-GPU path** | Provide a `values-quickstart.yaml` that runs on any laptop (CPU-only, tiny model). If someone clones and can't try it in 5 minutes, they leave. | `values-quickstart.yaml` |
| G3 | **3-5 concrete use cases** | Not feature lists. Scenarios: "I want to deploy DeepSeek-R1-0528 and let 5 teams share it with budget limits", "I want to build a RAG chatbot on my company docs", "I want to monitor which team is burning the most GPU hours". Resonates better than tech specs. | `README.md` or `docs/use-cases.md` |
| G4 | **License audit note** | Add a "License Notice" section: list all dependencies and their licenses. Flag AGPL components (Grafana, Loki) with a note: "If AGPL is a concern for your organization, these components are optional. You can bring your own Grafana or use alternatives." Don't block usage, just inform. | `ARCHITECTURE.md` + `README.md` |

### Before v0.2.0 -- Should Do

Becomes important once real users start deploying.

| # | Gap | What to do | Where |
|---|---|---|---|
| G5 | **Upgrade strategy** | Document how `helm upgrade` works. Which components need DB migration? What's the rollback procedure? Add a `scripts/pre-upgrade-check.sh`. | `docs/upgrade-guide.md` |
| G6 | **Failure mode analysis** | Document single points of failure (PostgreSQL is the biggest: LiteLLM + Langfuse both depend on it). Describe degradation behavior. Phase 2 can add PG HA. | `docs/operations-guide.md` |
| G7 | **Backup/restore for stateful components** | CronJob templates for PG dump, MinIO sync, Milvus snapshot. `scripts/backup.sh` and `scripts/restore.sh`. | `scripts/` + `docs/backup-restore.md` |
| G8 | **Compatibility matrix** | Table of tested combinations: K8s versions, GPU types, cloud providers. Start small (tested on kind + 1 cloud), expand over time. | `docs/compatibility.md` |
| G9 | **Resource sizing guide** | "For 10 concurrent users with Qwen-7B, you need: 1x A10G, 4 CPU, 16GB RAM, 100GB storage." Give 3-4 reference configurations. | `docs/sizing-guide.md` |
| G10 | **values.yaml schema versioning** | Define rule: `values.yaml` top-level keys are stable within a major version. Deprecation notice before removal. Document in CONTRIBUTING.md. | `CONTRIBUTING.md` |
| G11 | **Benchmark suite** | Script to run standardized benchmark: deploy model, send N requests, collect TTFT/throughput/overhead. Compare kube-llmops vs raw vLLM. Publish results in README. Numbers earn trust. | `scripts/benchmark.sh` + `docs/benchmarks.md` |

### Before v0.3.0+ -- Nice to Have

Polish items for growing the community.

| # | Gap | What to do | Where |
|---|---|---|---|
| G12 | **Community channels** | Set up GitHub Discussions (free, no extra tool). Add Discord/Slack link if community grows. | `README.md` |
| G13 | **Governance model** | GOVERNANCE.md: how decisions are made, how to become a maintainer. Can be simple initially. | `GOVERNANCE.md` |
| G14 | **Documentation site** | mkdocs-material + GitHub Pages. Auto-deploy in CI. Makes the project look professional. | `docs/` + `.github/workflows/docs.yaml` |
| G15 | **Day-2 operations runbook** | Common issues: vLLM OOM, GPU driver mismatch, PG connection exhaustion, certificate expiry. Troubleshooting guide. | `docs/troubleshooting.md` |
| G16 | **Migration guides from competitors** | "Already using raw vLLM? Here's how to migrate to kube-llmops." "Coming from KAITO?" Lowers switching cost. | `docs/migration/` |
| G17 | **Release cadence** | Define: minor release every 4-6 weeks, patch releases as needed. Write in CONTRIBUTING.md. | `CONTRIBUTING.md` |
| G18 | **Success metrics tracking** | Track GitHub stars, Helm chart downloads (ArtifactHub), Docker pulls. Add badges to README. | `README.md` |
| G19 | **Project sustainability** | If the project gets traction: CNCF Sandbox application, OpenCollective, GitHub Sponsors. Plan when the time comes. | `GOVERNANCE.md` |
| G20 | **SLO framework** | Define platform-level SLOs (API availability, TTFT target). More relevant for production users in Phase 3+. | `docs/slo.md` |
| G21 | **Multi-cloud real testing** | Actually test on EKS, GKE, ACK. Create Terraform quickstart for each. Phase 4 item. | `terraform/` |
| G22 | **Issue/project board setup** | GitHub Issues with labels (bug, feature, good-first-issue, help-wanted), Milestones matching M0-M11. | GitHub repo settings |

### Star-Earning Tactics (for README & Promotion)

> Based on what makes open source projects go viral.

| Tactic | Why it works | When |
|---|---|---|
| **One architecture diagram as a polished image** | ASCII diagrams look amateur. A clean SVG/PNG diagram in README gets shared on Twitter/X. | Before MVP |
| **GIF of Grafana dashboard with live data** | Visual proof > text claims. 3-second GIF showing GPU metrics + token costs = instant credibility. | After M5 |
| **"Deploy LLM in 5 minutes" blog post** | Write a Medium/Dev.to article with step-by-step. Cross-post to Reddit r/kubernetes, r/LocalLLaMA, Hacker News. | At MVP release |
| **Comparison table in README** | The competitive table already in ARCHITECTURE.md is great. Move it to README, it's shareable. | Before MVP |
| **"Good first issue" labels** | Attracts contributors. Even simple tasks (docs typo, add model to engine_map.yaml) count. | After MVP |
| **ArtifactHub listing** | Register Helm chart on ArtifactHub. Free discovery channel for K8s users searching for LLM tools. | At MVP release |
| **CNCF Landscape submission** | Submit to CNCF Cloud Native Landscape under "AI" category. Free visibility to the entire CNCF audience. | After v0.2.0 |
