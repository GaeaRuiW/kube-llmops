# kube-llmops - Architecture (v2)

**English** | [中文](ARCHITECTURE.zh-CN.md)

> **Kubernetes-native LLMOps Platform**
> One command to deploy, manage, monitor, and optimize your entire LLM infrastructure on Kubernetes.

---

## Core Design Principles

| # | Principle | What it means |
|---|---|---|
| 1 | **Best solution first, CNCF preferred** | Choose the best tool for the job. When multiple equally good options exist, prefer CNCF (Graduated > Incubating > Sandbox). Never add complexity just for a CNCF badge. |
| 2 | **Don't reinvent, integrate** | vLLM, LiteLLM, OpenTelemetry, Envoy... already battle-tested. Our value is the **glue + defaults + one-click experience**. |
| 3 | **Smart defaults, full override** | Model format auto-detection picks the right engine. 4 deployment presets (`ci`/`minimal`/`standard`/`production`). Everything can be manually overridden. |
| 4 | **IaC + GitOps native** | Everything declarative. ArgoCD Sync Waves handle deployment ordering. |
| 5 | **LLM-specific, not generic** | Token-based metering, GPU scheduling, model weight caching, TTFT monitoring, prefix-cache-aware routing. |

---

## Architecture Overview

```
                            External Clients (OpenAI SDK / LangChain / curl)
                                          |
                                          v
                   +----------------------------------------------+
                   |  Tier 1: AI Gateway (LiteLLM)                |
                   |  Key Mgmt / Multi-provider / Cost Tracking   |
                   +----------------------------------------------+
                                          |
                   +----------------------------------------------+
                   |  Tier 2: Inference Gateway                   |
                   |  Envoy AI Gateway + Gateway API Inference    |
                   |  Extension (IGW)                             |
                   |  KV-cache-aware routing / Prefix scheduling  |
                   +----------------------------------------------+
                        /         |         \
              +---------+   +---------+   +----------+
              | vLLM    |   | vLLM    |   | TEI      |     Engine auto-selected
              | DS-R1   |   | DS-V3   |   | bge-m3   |     by Model Resolver
              | (BF16)  |   | (FP16)  |   |          |     based on model format
              +---------+   +---------+   +----------+
              | llama.cpp|
              | Llama-8B |
              | (GGUF)   |
              +----------+
                        \         |         /
              +----------------------------------------------+
              |  Infrastructure & Scheduling                  |
              |  GPU Operator / Fluid Cache / KEDA / Karpenter|
              +----------------------------------------------+

   ┌──────────────────────────────────────────────────────────────┐
   │  Unified Observability (OpenTelemetry Collector)             │
   │  ┌──────────┐  ┌─────────┐  ┌──────┐                       │
   │  │Prometheus │  │ Langfuse│  │ Loki │                       │
   │  │(Metrics)  │  │(Traces) │  │(Logs)│                       │
   │  └────┬─────┘  └────┬────┘  └──┬───┘                       │
   │       └──────────────┴──────────┘                           │
   │                Grafana (Dashboards)                          │
   └──────────────────────────────────────────────────────────────┘

   ┌──────────────────────────────────────────────────────────────┐
   │  Data & Vector Layer: Milvus / MinIO / Harbor                │
   │  Dev & Finetune: JupyterHub / LLaMA-Factory / MLflow        │
   │  Security: Keycloak / NetworkPolicy / LLM-Guard             │
   └──────────────────────────────────────────────────────────────┘

   ┌──────────────────────────────────────────────────────────────┐
   │  Layer 0: GitOps (ArgoCD + Helm Umbrella Chart)             │
   └──────────────────────────────────────────────────────────────┘
```

---

## CNCF Alignment Map

Every technology choice with its CNCF status:

| Component | Technology | CNCF Status | Alternatives |
|---|---|---|---|
| **Orchestration** | Kubernetes | **Graduated** | - |
| **Service Proxy / AI Gateway Tier 2** | Envoy (AI Gateway + IGW) | **Graduated** | - |
| **Observability Pipeline** | OpenTelemetry | **Graduated** | - |
| **Metrics** | Prometheus | **Graduated** | - |
| **Tracing (LLM-specific)** | Langfuse (via OTel OTLP) | Community OSS | No CNCF tool handles prompt/token/cost tracing |
| **Log Collector** | Fluentbit | **Graduated** (Fluentd project) | Promtail (non-CNCF) |
| **Pod Autoscaling** | KEDA | **Graduated** | HPA with custom metrics |
| **GitOps** | Argo CD | **Graduated** | Flux (CNCF Graduated) |
| **Package Manager** | Helm | **Graduated** | - |
| **Container Registry / Model Registry** | Harbor | **Graduated** | Docker Registry |
| **Network Policy** | Cilium | **Graduated** | Calico |
| **ML Serving (optional)** | KServe | **Incubating** | Raw Deployment |
| **Auth / SSO** | Keycloak | **Incubating** | Dex |
| **Data Cache** | Fluid | **Sandbox** | JuiceFS (non-CNCF) |
| **Vector DB** | Milvus | LF AI & Data Foundation | pgvector, Qdrant |
| **AI Gateway Tier 1** | LiteLLM | Community OSS | No CNCF equivalent |
| **Inference Engine** | vLLM / llama.cpp / TEI | Community OSS | No CNCF equivalent |
| **Dashboards** | Grafana | Community OSS (AGPL) | No CNCF equivalent |
| **Object Storage** | MinIO | Community OSS | SeaweedFS |
| **Log Storage** | Loki | Community OSS (AGPL) | OpenSearch |
| **Inference Scheduling** | llm-d | Community OSS (K8s SIG adjacent) | - |

**Decision rule**: Best tool for the job wins. When two tools are equally good, pick the CNCF one. Example: Jaeger (CNCF) vs Langfuse -- Langfuse wins because it does everything Jaeger does for our use case PLUS prompt/token/cost/session tracking. We don't add Jaeger just for a CNCF badge.

---

## NEW: Model Resolver (Engine Auto-Selection)

> **Core idea: User specifies the model, platform automatically picks the optimal engine.**

### Problem

Users shouldn't need to know that AWQ models run best on vLLM and GGUF models need llama.cpp. The platform should figure this out.

### Detection Algorithm

```
                   User provides model ID
                   (e.g. "deepseek-ai/DeepSeek-R1-0528")
                              |
                              v
                   ┌─────────────────────┐
                   │  Model Resolver      │
                   │  (init-container)    │
                   └──────────┬──────────┘
                              |
              1. Fetch metadata (config.json / file listing)
                              |
              2. Detect format & quantization
                              |
              3. Match hardware constraints
                              |
              4. Select engine + optimal args
                              |
                              v
                   ┌─────────────────────┐
                   │  Deploy with auto-   │
                   │  selected engine     │
                   └─────────────────────┘
```

### Format -> Engine Mapping Rules

```
Priority: User explicit override > Auto-detection

┌────────────────────────────┬─────────────────────────────┬──────────────────────────────────────┐
│ Model Characteristics      │ Auto-Selected Engine        │ Auto-Applied Args                    │
├────────────────────────────┼─────────────────────────────┼──────────────────────────────────────┤
│ *.gguf files               │ llama.cpp (llama-server)    │ --ctx-size auto                      │
│ *.gguf + GPU available     │ llama.cpp (GPU offload)     │ --n-gpu-layers max                   │
├────────────────────────────┼─────────────────────────────┼──────────────────────────────────────┤
│ SafeTensors + AWQ quant    │ vLLM                        │ --quantization awq                   │
│ SafeTensors + GPTQ quant   │ vLLM                        │ --quantization gptq                  │
│ SafeTensors + FP8 quant    │ vLLM                        │ --quantization fp8                   │
│ SafeTensors + BitsAndBytes │ vLLM                        │ --quantization bitsandbytes          │
│ SafeTensors + no quant     │ vLLM                        │ (default FP16/BF16)                  │
├────────────────────────────┼─────────────────────────────┼──────────────────────────────────────┤
│ model_type = embedding     │ TEI                         │ (embedding mode)                     │
│ model_type = reranker      │ TEI                         │ (reranker mode)                      │
├────────────────────────────┼─────────────────────────────┼──────────────────────────────────────┤
│ TensorRT-LLM engine files  │ TensorRT-LLM / NIM         │ (pre-compiled engine)                │
├────────────────────────────┼─────────────────────────────┼──────────────────────────────────────┤
│ ONNX format                │ vLLM (ONNX backend)         │ (experimental)                       │
├────────────────────────────┼─────────────────────────────┼──────────────────────────────────────┤
│ No GPU available           │ llama.cpp (CPU)             │ Auto-convert to GGUF if needed       │
│ GPU VRAM < model size      │ llama.cpp (partial offload) │ --n-gpu-layers calculated            │
│ Multi-GPU required         │ vLLM (tensor parallel)      │ --tensor-parallel-size auto          │
└────────────────────────────┴─────────────────────────────┴──────────────────────────────────────┘
```

### Hardware-Aware Logic

```python
# Pseudocode for hardware-aware engine selection
def resolve_engine(model_meta, available_gpus):
    # Step 1: Format-based selection
    if model_meta.has_gguf_files():
        engine = "llama.cpp"
    elif model_meta.is_embedding() or model_meta.is_reranker():
        engine = "tei"
    elif model_meta.has_safetensors():
        engine = "vllm"
    else:
        engine = "vllm"  # safe default

    # Step 2: Hardware constraints
    model_size_gb = model_meta.estimate_vram()
    total_gpu_vram = sum(gpu.vram for gpu in available_gpus)

    if len(available_gpus) == 0:
        engine = "llama.cpp"  # CPU-only fallback
        quantization = "Q4_K_M"  # auto-quantize for CPU
    elif model_size_gb > total_gpu_vram and engine == "vllm":
        if model_size_gb > total_gpu_vram * 2:
            engine = "llama.cpp"  # partial GPU offload
        else:
            args["tensor-parallel-size"] = len(available_gpus)

    # Step 3: Quantization args
    if model_meta.quant_method:
        args["quantization"] = model_meta.quant_method

    # Step 4: Engine-specific optimizations
    if engine == "vllm":
        args["enable-prefix-caching"] = True
        args["gpu-memory-utilization"] = 0.92
    
    return engine, args
```

### User Interface

```yaml
# Helm values.yaml - User just specifies model, engine auto-detected
models:
  - name: deepseek-r1-0528
    source: deepseek-ai/DeepSeek-R1-0528
    # engine: auto                # default, auto-detect (resolves to vLLM)
    replicas: 2
    resources:
      gpu: 4

  - name: llama3-8b-q4
    source: bartowski/Meta-Llama-3-8B-Instruct-GGUF
    # engine: auto                # auto-detect (resolves to llama.cpp)
    resources:
      gpu: 1

  - name: bge-m3
    source: BAAI/bge-m3
    # engine: auto                # auto-detect (resolves to TEI embedding mode)
    resources:
      gpu: 1

  - name: custom-model
    source: my-registry/my-model
    engine: sglang               # explicit override, skip auto-detection
    engineArgs:
      --tp: "2"
```

### Implementation

The Model Resolver is an **init-container** that runs before the inference engine:

```yaml
initContainers:
  - name: model-resolver
    image: kube-llmops/model-resolver:latest
    env:
      - name: MODEL_SOURCE
        value: "deepseek-ai/DeepSeek-R1-0528"
      - name: ENGINE_OVERRIDE
        value: ""                  # empty = auto-detect
    volumeMounts:
      - name: resolver-output
        mountPath: /resolve
    # Outputs: /resolve/engine.env
    # ENGINE=vllm
    # ENGINE_IMAGE=vllm/vllm-openai:latest
    # ENGINE_ARGS=--enable-prefix-caching --gpu-memory-utilization 0.92
    # MODEL_PATH=/models/DeepSeek-R1-0528
```

### Deliverables
- `model-resolver` Docker image (Python, ~200 lines)
- Format detection library (parse config.json, scan file extensions)
- Hardware probing (detect GPU type, VRAM via `nvidia-smi`)
- Engine image mapping table (configurable via ConfigMap)
- Helm template that reads resolver output and spawns correct engine

---

## Layer 0: GitOps & Automated Delivery

> Same as v1, with ArgoCD (CNCF Graduated) + Helm (CNCF Graduated).

### Deployment Flow

```
User: helm install kube-llmops charts/kube-llmops-stack -f values-standard.yaml
  OR
User: kubectl apply -f argocd-app.yaml   (GitOps mode)

ArgoCD Sync Waves:
  Wave 0: GPU Operator, Fluid, KEDA, OTel Collector, Cilium
  Wave 1: MinIO, PostgreSQL, Harbor, Milvus, Prometheus
  Wave 2: Model Resolver -> vLLM/llama.cpp/TEI (auto-selected), LiteLLM, Langfuse
  Wave 3: Grafana Dashboards, Alert Rules, Envoy AI Gateway (IGW)
```

### Value Presets

```yaml
# values-minimal.yaml    - 1 GPU, 1 model, basic monitoring, no vector DB
# values-standard.yaml   - Multi-model, full OTel observability, pgvector
# values-production.yaml - HA, autoscaling, Milvus, multi-tenant, full security
```

---

## Layer 1: Infrastructure & Scheduling

> Same core as v1, with one key change: **Fluid (CNCF Sandbox) replaces JuiceFS as primary cache**.

| Component | Technology | CNCF Status | What it solves |
|---|---|---|---|
| GPU Operator | NVIDIA GPU Operator | - | Auto-install drivers, device plugin |
| GPU Sharing | Time-Slicing / MIG | - | Multi-model on single GPU |
| **Model Cache** | **Fluid + Alluxio** | **CNCF Sandbox** | Distributed model weight cache, data-locality scheduling |
| Model Cache (alt) | JuiceFS + MinIO | Non-CNCF | Alternative if Fluid doesn't fit |
| Node Autoscaling | Karpenter / Cluster Autoscaler | - | Auto-provision GPU nodes |
| Node Discovery | NFD | - | Auto-label GPU type/topology |

### Model Weight Caching with Fluid

```yaml
apiVersion: data.fluid.io/v1alpha1
kind: Dataset
metadata:
  name: deepseek-r1-0528-weights
spec:
  mounts:
    - mountPoint: s3://model-store/DeepSeek-R1-0528/
      name: deepseek-r1-0528
      options:
        fs.s3a.endpoint: http://minio:9000
  placement: "Shared"     # cache shared across nodes
---
apiVersion: data.fluid.io/v1alpha1
kind: AlluxioRuntime
metadata:
  name: deepseek-r1-0528-weights
spec:
  replicas: 3             # 3 cache workers
  tieredstore:
    levels:
      - mediumtype: SSD
        path: /cache
        quota: 200Gi      # local SSD cache per node
        high: "0.95"
        low: "0.7"
```

Pods automatically get data-locality scheduling: Fluid places inference pods near cached data.

---

## Layer 2: Model Serving

> Enhanced with Model Resolver auto-selection (see above).

### Engine Matrix

| Engine | Best For | GGUF | SafeTensors | AWQ/GPTQ | Embedding | CNCF |
|---|---|---|---|---|---|---|
| **vLLM** | Production GPU inference | No | Yes | Yes | Yes | - |
| **llama.cpp** | GGUF / CPU / Edge | Yes | Partial | No | No | - |
| **SGLang** | Multi-turn / Structured output | No | Yes | Yes | No | - |
| **TEI** | Embedding & Reranking | No | Yes | No | **Yes** | - |
| **TGI** | HuggingFace ecosystem | No | Yes | Yes | No | - |

### Optional: KServe Integration (CNCF Incubating)

For teams already using KServe, kube-llmops can deploy models via KServe's InferenceService:

```yaml
# Optional: wrap vLLM in KServe InferenceService for canary/serverless features
kserve:
  enabled: false     # off by default, enable if KServe is installed
  # Provides: canary rollout, scale-to-zero (Knative), InferenceService CRD
```

---

## Layer 3: Two-Tier Gateway

> **Major change from v1**: Split into Tier 1 (AI Gateway) + Tier 2 (Inference Gateway), aligned with industry trend and CNCF ecosystem.

### Why Two Tiers

```
Single-tier (v1):       Two-tier (v2):
LiteLLM -> vLLM         LiteLLM (Tier 1) -> Envoy+IGW (Tier 2) -> vLLM

Tier 1 handles:         Tier 2 handles:
- Everything             - KV-cache-aware routing
                         - Prefix cache scheduling
                         - LoRA adapter routing
                         - Token-level load balancing
                         - Disaggregated serving (P/D split)

Tier 1 handles:
- API key management
- Multi-provider routing
- Cost tracking / budgets
- Rate limiting
- Fallback chains
```

The industry (Google GKE, llm-d, K8s SIG) is converging on **Envoy + Gateway API Inference Extension** as the Tier 2 standard. LiteLLM remains Tier 1 because no CNCF project handles API key management + multi-provider + cost tracking.

### Tier 1: LiteLLM (AI Gateway)

Unchanged from v1. Handles:
- OpenAI-compatible API for all providers
- API key management + token budgets (PostgreSQL backend)
- Per-user / per-team cost tracking
- Multi-model routing, fallback chains
- Rate limiting (RPM, TPM)

### Tier 2: Envoy AI Gateway + IGW (Inference Gateway)

```yaml
# Gateway API Inference Extension CRDs
apiVersion: inference.networking.x-k8s.io/v1alpha2
kind: InferencePool
metadata:
  name: vllm-pool
spec:
  targetPortNumber: 8000
  selector:
    matchLabels:
      app: vllm
  endpointPickerConfig:
    extensionRef:
      name: inference-gateway-epp
---
apiVersion: inference.networking.x-k8s.io/v1alpha2
kind: InferenceModel
metadata:
  name: deepseek-r1-0528
spec:
  modelName: deepseek-r1-0528
  targetRef:
    name: vllm-pool
  criticality: Critical
```

**Key capabilities from IGW**:
- **KV-cache-aware routing**: Route requests to pods that already have relevant KV cache (not round-robin)
- **Prefix-cache-aware scheduling**: Requests with similar prefixes go to the same pod
- **LoRA adapter routing**: Route by model name to correct LoRA adapter
- **Canary rollout**: A/B split between model versions via InferenceModel CRD
- **Flow control**: Priority + fairness between workloads

### Tier 2 Rollout Strategy

```
Phase 1 (MVP):     LiteLLM -> vLLM directly (no Tier 2)
Phase 2:           LiteLLM -> Envoy AI Gateway (basic) -> vLLM
Phase 3:           LiteLLM -> Envoy + IGW (full inference scheduling) -> vLLM
Phase 4 (future):  LiteLLM -> IGW + llm-d (disaggregated P/D serving)
```

---

## Layer 4: Observability (OpenTelemetry + Langfuse)

> **OpenTelemetry (CNCF Graduated) as the unified collection pipeline, Langfuse as the tracing backend. No Jaeger -- our call chain is simple, Langfuse covers it better.**

### Why no Jaeger?

Jaeger is a great general-purpose distributed tracing tool. But kube-llmops is not a general microservice platform. Our call chain is:

```
Client -> LiteLLM -> (Envoy) -> vLLM -> Response
```

3-4 hops. Deploying a full Jaeger stack (Collector + Query + Cassandra/ES) for this is overkill. Meanwhile, Langfuse:

| Capability | Jaeger | Langfuse | Verdict |
|---|---|---|---|
| Request latency trace | Yes | Yes | Tie |
| Span correlation across services | Yes | Yes (via OTel OTLP) | Tie |
| Full prompt/completion text | **No** | Yes | **Langfuse** |
| Token count per request | **No** | Yes | **Langfuse** |
| Cost per request/user/team | **No** | Yes | **Langfuse** |
| User session grouping | **No** | Yes | **Langfuse** |
| Prompt versioning & A/B test | **No** | Yes | **Langfuse** |
| Evaluation scoring (LLM-as-judge) | **No** | Yes | **Langfuse** |
| RAG pipeline decomposition | **No** | Yes | **Langfuse** |
| Extra infra to maintain | Collector + Query + Storage | Single service + PG | **Langfuse** (simpler) |

**Conclusion**: Langfuse is strictly superior for LLM tracing. Adding Jaeger would mean maintaining an extra stateful service for zero additional value. If users have an existing Jaeger cluster and want to send traces there too, OTel Collector makes that a one-line exporter config change -- but we don't ship it by default.

### Why still OpenTelemetry?

OTel is not a backend -- it's the **collection pipeline**. Even without Jaeger, OTel earns its place:

1. **Unified ingestion**: vLLM, LiteLLM, Envoy all speak OTLP natively. One protocol to collect everything.
2. **Processing**: Enrich traces/metrics with `model_name`, `user_id`, `tenant` labels before export.
3. **Fan-out**: Single pipeline exports to Prometheus (metrics) + Langfuse (traces) + Loki (logs). Add any backend later without changing instrumentation.
4. **Decoupling**: If Langfuse ever needs to be swapped, only the OTel exporter config changes. Application code stays untouched.

### Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                    Data Sources (OTel SDK built-in)                  │
│                                                                     │
│  vLLM             LiteLLM          Envoy/IGW        llama.cpp      │
│  (traces +        (traces +        (traces +        (metrics via   │
│   metrics)         metrics)         metrics +        custom export) │
│                                     access logs)                    │
└────────┬──────────────┬────────────────┬──────────────┬────────────┘
         │              │                │              │
         │      OTLP (gRPC/HTTP)         │              │
         v              v                v              v
┌─────────────────────────────────────────────────────────────────────┐
│                    OpenTelemetry Collector                           │
│                    (CNCF Graduated)                                  │
│                                                                     │
│  Receivers:    otlp, prometheus (scrape vLLM+DCGM), fluentforward  │
│  Processors:   batch, attributes (add model/user/tenant labels),   │
│                filter (drop health checks), transform              │
│  Exporters:    prometheusremotewrite, otlphttp/langfuse, loki     │
└──────┬──────────────────────────┬──────────────────────┬───────────┘
       │                          │                      │
       v                          v                      v
  Prometheus                  Langfuse                 Loki
  (Metrics store)             (Traces + LLM analytics) (Log store)
  - TTFT, ITL, throughput     - Full prompt/completion  - Engine logs
  - GPU util, VRAM, temp      - Token count & cost      - CUDA errors
  - Request rates             - User sessions           - Routing events
  - KV cache stats            - Evaluation scores       - Audit trail
       │                          │                      │
       └──────────────────────────┴──────────────────────┘
                                  │
                                  v
                             Grafana
                         (Unified dashboards)
                         - Prometheus as datasource (metrics)
                         - Loki as datasource (logs)
                         - Langfuse link-out for trace detail
```

### OTel Collector Configuration

```yaml
# otel-collector-config.yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318
  prometheus:
    config:
      scrape_configs:
        - job_name: 'vllm'
          kubernetes_sd_configs:
            - role: pod
          relabel_configs:
            - source_labels: [__meta_kubernetes_pod_label_app]
              regex: vllm
              action: keep
        - job_name: 'dcgm-exporter'
          kubernetes_sd_configs:
            - role: pod
          relabel_configs:
            - source_labels: [__meta_kubernetes_pod_label_app]
              regex: dcgm-exporter
              action: keep

processors:
  batch:
    timeout: 5s
    send_batch_size: 1024
  attributes:
    actions:
      - key: cluster.name
        value: "${CLUSTER_NAME}"
        action: upsert
  filter/health:
    traces:
      span:
        - 'attributes["http.route"] == "/health"'

exporters:
  prometheusremotewrite:
    endpoint: http://prometheus:9090/api/v1/write
    resource_to_telemetry_conversion:
      enabled: true
  otlphttp/langfuse:
    endpoint: http://langfuse:3000/api/public/otel
    headers:
      Authorization: "Basic ${LANGFUSE_AUTH}"
  loki:
    endpoint: http://loki:3100/loki/api/v1/push

service:
  pipelines:
    metrics:
      receivers: [otlp, prometheus]
      processors: [batch, attributes]
      exporters: [prometheusremotewrite]
    traces:
      receivers: [otlp]
      processors: [batch, filter/health, attributes]
      exporters: [otlphttp/langfuse]          # Langfuse is the ONLY trace backend
    logs:
      receivers: [otlp]
      processors: [batch]
      exporters: [loki]
```

### 3 Pillars of LLM Observability (not 4 -- tracing is Langfuse, not a separate pillar)

#### Pillar 1: Metrics (Prometheus, via OTel Collector)

| Category | Key Metrics | Source |
|---|---|---|
| **LLM Latency** | TTFT, ITL (Inter-Token Latency), E2E P50/P95/P99 | vLLM OTel SDK |
| **LLM Throughput** | Input/Output tokens/sec, Requests/sec | vLLM OTel SDK |
| **LLM Engine** | KV cache util, Pending requests, Batch size, Prefix cache hit | vLLM metrics |
| **GPU Hardware** | GPU util %, VRAM, Temperature, Power, ECC errors | DCGM Exporter |
| **Gateway** | Requests by model/user/status, Token consumption | LiteLLM OTel |
| **Inference Gateway** | Routing decisions, cache-hit routing, P/D split | IGW/Envoy OTel |
| **Cost** | Cost per token/request/user/day, Budget remaining | LiteLLM PostgreSQL |

#### Pillar 2: Tracing + LLM Analytics (Langfuse, via OTel OTLP)

Langfuse receives OTel traces and enriches them with LLM-specific context:

```
[Trace in Langfuse]
│
├── Generation: deepseek-r1-0528
│   ├── Input:  "Explain Kubernetes to a 5-year-old" (1,024 tokens)
│   ├── Output: "Imagine you have a bunch of toy boxes..." (512 tokens)
│   ├── Latency: TTFT=320ms, Total=4.2s
│   ├── Cost: $0.0031
│   └── Scores: relevance=0.95, fluency=0.88 (LLM-as-judge)
│
├── Retrieval (if RAG):
│   ├── Query embedding: 1.2ms
│   ├── Vector search: 8ms, top-5 retrieved
│   ├── Reranking: 45ms, top-3 selected
│   └── Context tokens: 2,048
│
└── Session: user-alice, conversation-id-xyz, turn 3/5
```

LiteLLM native integration (simplest, recommended):
```yaml
# litellm-config.yaml
litellm_settings:
  success_callback: ["langfuse"]
  failure_callback: ["langfuse"]

environment_variables:
  LANGFUSE_PUBLIC_KEY: pk-xxx
  LANGFUSE_SECRET_KEY: sk-xxx
  LANGFUSE_HOST: http://langfuse:3000
```

OTel integration (for traces from vLLM/Envoy that don't go through LiteLLM):
```yaml
# Already configured in OTel Collector above
exporters:
  otlphttp/langfuse:
    endpoint: http://langfuse:3000/api/public/otel
```

#### Pillar 3: Logging (Fluentbit -> OTel Collector -> Loki)

```
Fluentbit (CNCF Graduated)       OTel Collector          Loki
  - DaemonSet on each node         - Transform             - Store
  - Tail container logs            - Add labels            - Query via Grafana
  - Forward to OTel Collector      - Export to Loki
```

| Log Type | Content |
|---|---|
| Request Log | Request ID, model, user, input/output tokens, latency, status |
| Engine Log | vLLM/llama.cpp internal, CUDA errors, OOM events |
| Gateway Log | Routing decisions, rate limit hits, fallback triggers |
| Audit Log | API key operations, model deployments, config changes |

### Extensibility: "I already have Jaeger / Datadog / Grafana Tempo"

Because OTel Collector is the pipeline, adding any additional backend is a config change:

```yaml
# Just add an exporter -- no application code changes
exporters:
  otlphttp/langfuse:
    endpoint: http://langfuse:3000/api/public/otel   # default
  jaeger:                                              # optional, user-added
    endpoint: jaeger-collector:14250
  otlp/datadog:                                        # optional, user-added
    endpoint: https://api.datadoghq.com:4317

service:
  pipelines:
    traces:
      exporters: [otlphttp/langfuse, jaeger, otlp/datadog]  # fan-out to all
```

This is why OTel matters even without Jaeger: it's the **extensibility layer**.

### Pre-built Grafana Dashboards (3)

| # | Dashboard | Data Source | Key Panels |
|---|---|---|---|
| 1 | vLLM Model Serving Overview | Prometheus | Request rate, latency percentiles (TTFT/ITL/E2E), token throughput, KV cache utilization |
| 2 | LiteLLM AI Gateway | Prometheus + LiteLLM PG | Traffic by model, active models, error rate, token usage, cost tracking |
| 3 | GPU & Infrastructure Overview | Prometheus (DCGM) | GPU/VRAM resource usage, queue depth, TTFT, inter-token latency |

### Alert Rules (Prometheus, via OTel metrics)

```yaml
groups:
  - name: llm-serving
    rules:
      - alert: HighTTFT
        expr: histogram_quantile(0.95, rate(vllm_time_to_first_token_seconds_bucket[5m])) > 3
        for: 5m
        annotations:
          summary: "Model {{ $labels.model_name }} TTFT P95 > 3s"

      - alert: GPUMemoryPressure
        expr: DCGM_FI_DEV_FB_USED / (DCGM_FI_DEV_FB_USED + DCGM_FI_DEV_FB_FREE) > 0.95
        for: 2m
        labels:
          severity: critical

      - alert: ModelDown
        expr: up{job="vllm"} == 0
        for: 1m
        labels:
          severity: critical

      - alert: InferenceGatewayLatency
        expr: histogram_quantile(0.99, rate(envoy_http_downstream_rq_time_bucket[5m])) > 5000
        for: 3m

      - alert: BudgetExhausted
        expr: litellm_team_budget_remaining < 10
        labels:
          severity: warning
```

### Deliverables
- Helm sub-chart: `charts/observability/` (OTel Collector + Prometheus + Fluentbit + Loki + DCGM Exporter)
- Helm sub-chart: `charts/langfuse/`
- Helm sub-chart: `charts/grafana/` (with 3 pre-built dashboard JSON)
- OTel Collector config for LLM workloads
- Prometheus recording rules + alert rules
- Fluentbit DaemonSet config

---

## Layer 5: Data & Vector Infrastructure

| Component | Technology | CNCF Status | Purpose |
|---|---|---|---|
| **Vector DB (scale)** | **Milvus** | LF AI & Data | Production RAG, distributed |
| **Vector DB (lightweight)** | **pgvector** | PostgreSQL extension | Reuse LiteLLM's PG, zero extra infra |
| **Object Storage** | **MinIO** | - | Model weights, datasets, backups |
| **Model Registry** | **Harbor** | **CNCF Graduated** | OCI-based model artifact versioning |
| **Data Versioning** | **DVC** / LakeFS | - | Version training datasets alongside Git |

### Harbor as Model Registry (CNCF Graduated)

Instead of a custom model registry, use Harbor to store model artifacts as OCI images:

```bash
# Push model weights as OCI artifact
oras push harbor.example.com/models/deepseek-r1-0528:v1 \
  --artifact-type application/vnd.llmops.model.v1 \
  ./model-weights/

# Reference in Helm values
models:
  - name: deepseek-r1-0528
    source: oci://harbor.example.com/models/deepseek-r1-0528:v1
```

Benefits: versioning, access control, vulnerability scanning, replication across registries -- all built into Harbor.

---

## Layer 6: Model Development & Fine-tuning

Unchanged from v1:
- **JupyterHub** on K8s (GPU notebook environments)
- **LLaMA-Factory** as K8s Job (LoRA, QLoRA, full fine-tune)
- **MLflow** experiment tracking (reuse PostgreSQL + MinIO)
- **Label Studio** (optional, data annotation)

---

## Cross-cutting: Security & Multi-tenancy

| Component | Technology | CNCF Status |
|---|---|---|
| Auth / SSO | **Keycloak** | **CNCF Incubating** |
| API Key Management | **LiteLLM** built-in | - |
| Network Policy | **Cilium** | **CNCF Graduated** |
| Secret Management | **External Secrets Operator** | - |
| Content Safety | **LLM-Guard** | - |
| Audit | OTel traces + Loki logs | CNCF Graduated |

---

## Cross-cutting: Autoscaling & Cost

| Component | Technology | CNCF Status |
|---|---|---|
| Pod Autoscaling | **KEDA** | **CNCF Graduated** |
| Node Autoscaling | Karpenter / Cluster Autoscaler | - |
| Scale-to-Zero | KEDA ScaledObject | CNCF Graduated |
| Spot/Preemptible | Karpenter spot handler | - |

KEDA triggers for LLM workloads:
```yaml
triggers:
  - type: prometheus
    metadata:
      query: sum(vllm_num_requests_waiting{model_name="deepseek-r1-0528"})
      threshold: "50"
  - type: prometheus
    metadata:
      query: histogram_quantile(0.95, rate(vllm_time_to_first_token_seconds_bucket[5m]))
      threshold: "2.0"
```

---

## Technology Stack Summary (v2, CNCF-aligned)

| Category | v1 Choice | v2 Choice | Change Reason |
|---|---|---|---|
| Observability Pipeline | (none, direct) | **OpenTelemetry** (CNCF Graduated) | Unified pipeline, CNCF standard |
| Tracing | Tempo | **Langfuse** (via OTel OTLP) | Langfuse > Jaeger for LLM: does tracing + prompt/token/cost/session. Don't add Jaeger for a CNCF badge. |
| Log Collector | Promtail | **Fluentbit** (CNCF Graduated) | CNCF > non-CNCF |
| Model Cache | JuiceFS | **Fluid** (CNCF Sandbox) | CNCF > non-CNCF |
| Model Registry | OCI Registry | **Harbor** (CNCF Graduated) | CNCF, built-in access control |
| Gateway Tier 2 | (none) | **Envoy AI Gateway + IGW** (Envoy=CNCF Grad.) | Industry standard, KV-aware routing |
| Network | Calico | **Cilium** (CNCF Graduated) | CNCF, eBPF-based |
| Engine Selection | Manual | **Model Resolver** (auto-detect) | New feature |
| Metrics | Prometheus (direct) | **Prometheus via OTel** | Unified pipeline |
| Dashboards | Grafana | Grafana (unchanged) | No CNCF alternative |
| AI Gateway Tier 1 | LiteLLM | LiteLLM (unchanged) | No CNCF alternative |
| LLM Tracing | Langfuse | Langfuse via OTel (unchanged) | No CNCF alternative |
| Inference Engine | vLLM | vLLM + llama.cpp + TEI (auto) | Enhanced with auto-selection |

---

## Repo Structure (v2)

```
kube-llmops/
├── charts/
│   └── kube-llmops-stack/              # Umbrella Helm Chart
│       ├── Chart.yaml
│       ├── values.yaml                 # Master defaults
│       ├── values-minimal.yaml
│       ├── values-standard.yaml
│       ├── values-production.yaml
│       ├── templates/
│       │   ├── _helpers.tpl
│       │   ├── configmap-litellm.yaml
│       │   ├── configmap-otel.yaml     # OTel Collector config
│       │   └── ingress.yaml
│       └── charts/                     # Sub-charts
│           ├── vllm/
│           ├── llamacpp/               # NEW: llama.cpp for GGUF
│           ├── sglang/
│           ├── tei/                    # Embedding & reranking
│           ├── model-resolver/         # NEW: Engine auto-selection
│           ├── litellm/               # Tier 1 AI Gateway
│           ├── inference-gateway/      # NEW: Envoy AI Gateway + IGW (Tier 2)
│           ├── observability/          # OTel Collector + Prometheus + Fluentbit + Loki + DCGM
│           ├── langfuse/
│           ├── grafana/
│           ├── milvus/
│           ├── minio/
│           ├── harbor/                 # NEW: CNCF model registry
│           ├── fluid/                  # NEW: replaces juicefs
│           ├── jupyterhub/
│           ├── mlflow/
│           ├── keycloak/
│           └── keda/
│
├── images/
│   ├── model-resolver/                 # NEW: Engine auto-detection
│   │   ├── Dockerfile
│   │   ├── resolver.py
│   │   ├── format_detector.py
│   │   ├── hardware_probe.py
│   │   └── engine_map.yaml            # Format -> engine mapping rules
│   ├── model-loader/
│   │   ├── Dockerfile
│   │   └── loader.py
│   └── rag-worker/
│       ├── Dockerfile
│       └── ingest.py
│
├── dashboards/                         # Grafana dashboard JSON
│   ├── vllm-model-serving-overview.json
│   ├── litellm-ai-gateway.json
│   └── gpu-infrastructure-overview.json
│
├── alerting/
│   ├── llm-serving.yaml
│   ├── gpu-hardware.yaml
│   └── cost-budget.yaml
│
├── otel/                               # NEW: OpenTelemetry configs
│   ├── collector-config.yaml
│   ├── instrumentation.yaml            # OTel auto-instrumentation CR
│   └── sampling-policy.yaml
│
├── manifests/
│   ├── quickstart/
│   ├── kustomize/
│   │   ├── base/
│   │   └── overlays/
│   └── argocd/
│       ├── app-of-apps.yaml
│       └── applicationset.yaml
│
├── terraform/
│   ├── aws-eks/
│   ├── gcp-gke/
│   ├── azure-aks/
│   └── aliyun-ack/
│
├── examples/
│   ├── python/
│   │   ├── openai-sdk-chat.py
│   │   ├── langchain-rag.py
│   │   └── streaming.py
│   ├── curl/
│   │   └── api-examples.sh
│   └── fine-tuning/
│       ├── finetune-job.yaml
│       └── evaluate.py
│
├── scripts/
│   ├── init-cluster.sh
│   ├── install.sh
│   ├── backup.sh
│   └── health-check.sh
│
├── docs/
│   ├── getting-started.md
│   ├── architecture.md
│   ├── model-resolver-guide.md         # NEW
│   ├── observability-guide.md          # NEW (OTel setup)
│   ├── gpu-setup-guide.md
│   ├── model-serving-guide.md
│   ├── gateway-guide.md                # NEW (two-tier gateway)
│   ├── security-guide.md
│   ├── rag-guide.md
│   ├── fine-tuning-guide.md
│   ├── troubleshooting.md
│   └── faq.md
│
├── ARCHITECTURE.md
├── README.md
├── CONTRIBUTING.md
├── LICENSE                             # Apache 2.0
├── Makefile
└── .github/
    └── workflows/
        ├── lint-charts.yaml
        ├── release.yaml
        └── docs.yaml
```

---

## Roadmap (v2, updated)

### Phase 1: Foundation (MVP) -- "Deploy a model, see metrics"
- [x] Repo scaffolding, Makefile, CI, lint
- [x] **Model Resolver** init-container (format detection + engine auto-selection)
- [x] vLLM Helm sub-chart + llama.cpp Helm sub-chart
- [x] LiteLLM Helm sub-chart (Tier 1 gateway, PostgreSQL)
- [x] **OpenTelemetry Collector** + Prometheus + DCGM Exporter
- [x] Grafana + 3 dashboards (Overview, GPU Fleet, Per-Model)
- [x] Umbrella Chart + `values-minimal.yaml`
- [x] Quick start guide + README
- [x] `scripts/install.sh`

### Phase 2: Production Readiness -- "Run in production"
- [x] Multi-model deployment (vLLM + TEI + llama.cpp, auto-selected)
- [x] LiteLLM advanced (fallback, retries, rate limiting, budget control)
- [x] **Langfuse** integration (LiteLLM callback + OTel OTLP)
- [x] **Fluentbit** + Loki logging pipeline
- [x] 3 Grafana dashboards + 4 Prometheus alert rules (latency, queue, KV cache, down)
- [x] **KEDA** autoscaling (pending requests, TTFT) -- verified with KEDA operator
- [x] **Fluid** distributed model caching -- templates ready, requires Fluid operator
- [x] MinIO object storage (model uploaded + verified) + **Harbor** templates
- [x] `values-standard.yaml` + `values-production.yaml`
- [x] Keycloak SSO (Grafana OIDC verified) + NetworkPolicy templates

### Phase 3: RAG & Inference Optimization -- "Build RAG apps, optimize routing"
- [x] pgvector (PostgreSQL image switched to pgvector/pgvector:pg16)
- [x] Milvus Helm sub-chart (standalone mode)
- [x] TEI embedding/reranking serving -- chart template ready
- [x] RAG ingestion worker + example app (examples/rag/)
- [x] **Envoy AI Gateway + IGW** (Tier 2, Gateway + HTTPRoute verified on cluster)
- [x] **LoRA adapter routing** -- InferenceModel CRD templates ready (requires IGW extension)
- [x] Multi-tenancy (Namespace + ResourceQuota + NetworkPolicy per team)
- [x] `values-production.yaml`
- [x] Backup/restore automation (scripts/backup.sh + restore.sh)

### Phase 4: ML Platform -- "ML engineers love it"
- [ ] JupyterHub with GPU profiles
- [ ] LLaMA-Factory fine-tuning Job templates
- [ ] MLflow experiment tracking
- [ ] Model evaluation pipeline
- [ ] ArgoCD ApplicationSet for multi-cluster
- [ ] Terraform modules (EKS, GKE, ACK)

### Phase 5: Advanced Inference -- "State-of-the-art performance"
- [ ] **llm-d** integration (disaggregated prefill/decode serving)
- [ ] **Expert Parallelism** for MoE models (DeepSeek-R1)
- [ ] KV cache tiered offloading (GPU -> CPU -> SSD -> remote)
- [ ] Workload-variant autoscaling (SLO-aware)
- [ ] KServe integration (optional)
- [ ] AMD ROCm / Intel Gaudi support

### Phase 6: Ecosystem (Future)
- [ ] Kubernetes Operator with CRDs
- [ ] CLI tool (`kube-llmops` / `kubectl llmops`)
- [ ] Web Dashboard
- [ ] Multi-modal model serving
- [ ] Model optimization toolkit (quantization, distillation)

---

## Competitive Positioning (updated with industry research)

```
                   Full LLMOps Platform (deploy + gateway + monitor + data + finetune)
                        ^
                        |
         kube-llmops ---|--------- KubeRay ecosystem (Ray Serve + Ray Train)
                        |
                        |
   Easy to deploy <-----+-----> Performance-optimized
                        |
         KubeAI --------|-------- KServe + llm-d (CNCF standard)
         llmaz          |         KAITO (Azure-specific)
                        |
                        v
                   Inference-only
```

| Feature | KAITO | KServe+llm-d | llmaz | KubeAI | kube-llmops |
|---|---|---|---|---|---|
| Engine auto-selection | No (preset) | No | No | No | **Yes** |
| AI Gateway (key/cost) | No | No | Partial | No | **Yes (LiteLLM)** |
| KV-cache-aware routing | No | **Yes (IGW)** | **Yes (IGW)** | No | **Yes (Phase 3)** |
| OTel observability | No | Partial | No | No | **Yes (full stack)** |
| Pre-built dashboards | No | No | No | No | **Yes (3)** |
| LLM tracing (prompt) | No | No | No | No | **Yes (Langfuse)** |
| Vector DB | faiss only | No | No | No | **Yes (Milvus/pgvector)** |
| Fine-tuning workflow | Yes | No | No | No | **Yes** |
| Dev environment | No | No | No | No | **Yes (JupyterHub)** |
| CNCF alignment | Low | **High** | Medium | Low | **High** |
| Cloud-agnostic | Azure only | Yes | Yes | Yes | **Yes** |
| One-click full stack | No | No | No | No | **Yes** |

---

## License

Apache License 2.0
