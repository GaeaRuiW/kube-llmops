[English](ARCHITECTURE.md) | **中文**

# kube-llmops - 架构设计 (v2)

> **Kubernetes 原生 LLMOps 平台**
> 一条命令即可在 Kubernetes 上部署、管理、监控和优化你的整个 LLM 基础设施。

---

## 核心设计原则

| # | 原则 | 含义 |
|---|---|---|
| 1 | **最优方案优先，CNCF 优选** | 为特定需求选择最佳工具。当多个方案同样优秀时，优先选择 CNCF 项目（毕业级 > 孵化级 > 沙箱级）。绝不为了挂 CNCF 标签而增加不必要的复杂度。 |
| 2 | **不重复造轮子，做好集成** | vLLM、LiteLLM、OpenTelemetry、Envoy……都已经过生产验证。我们的价值在于**粘合层 + 合理默认配置 + 一键体验**。 |
| 3 | **智能默认，完全可覆盖** | 模型格式自动检测自动选择引擎。3 种部署预设（`minimal`/`standard`/`production`）。一切均可手动覆盖。 |
| 4 | **IaC + GitOps 原生** | 一切声明式。ArgoCD Sync Waves 处理部署顺序。 |
| 5 | **面向 LLM，而非通用** | 基于 Token 的计量、GPU 调度、模型权重缓存、TTFT 监控、Prefix-Cache 感知路由。 |

---

## 架构总览

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

## CNCF 对齐全景图

每项技术选型及其 CNCF 状态：

| 组件 | 技术 | CNCF 状态 | 替代方案 |
|---|---|---|---|
| **编排** | Kubernetes | **毕业级** | - |
| **服务代理 / AI Gateway 第二层** | Envoy (AI Gateway + IGW) | **毕业级** | - |
| **可观测性管线** | OpenTelemetry | **毕业级** | - |
| **指标** | Prometheus | **毕业级** | - |
| **链路追踪（LLM 专用）** | Langfuse（通过 OTel OTLP） | 社区开源 | 没有 CNCF 工具能处理 Prompt/Token/成本追踪 |
| **日志采集** | Fluentbit | **毕业级**（Fluentd 项目） | Promtail（非 CNCF） |
| **Pod 自动扩缩** | KEDA | **毕业级** | 自定义指标 HPA |
| **GitOps** | Argo CD | **毕业级** | Flux（CNCF 毕业级） |
| **包管理** | Helm | **毕业级** | - |
| **容器镜像 / 模型仓库** | Harbor | **毕业级** | Docker Registry |
| **网络策略** | Cilium | **毕业级** | Calico |
| **ML Serving（可选）** | KServe | **孵化级** | 原始 Deployment |
| **认证 / SSO** | Keycloak | **孵化级** | Dex |
| **数据缓存** | Fluid | **沙箱级** | JuiceFS（非 CNCF） |
| **向量数据库** | Milvus | LF AI & Data 基金会 | pgvector、Qdrant |
| **AI Gateway 第一层** | LiteLLM | 社区开源 | 无 CNCF 等价物 |
| **推理引擎** | vLLM / llama.cpp / TEI | 社区开源 | 无 CNCF 等价物 |
| **仪表盘** | Grafana | 社区开源 (AGPL) | 无 CNCF 等价物 |
| **对象存储** | MinIO | 社区开源 | SeaweedFS |
| **日志存储** | Loki | 社区开源 (AGPL) | OpenSearch |
| **推理调度** | llm-d | 社区开源（K8s SIG 关联） | - |

**决策规则**：最适合的工具胜出。当两个工具同样优秀时，选 CNCF 的。例如：Jaeger（CNCF）vs Langfuse —— Langfuse 胜出，因为它在我们的使用场景下涵盖了 Jaeger 的全部功能，还额外提供了 Prompt/Token/成本/会话追踪。我们不会仅仅为了挂 CNCF 标签而引入 Jaeger。

---

## 新增：Model Resolver（引擎自动选择）

> **核心理念：用户只需指定模型，平台自动选择最优推理引擎。**

### 问题

用户不应该需要知道 AWQ 模型在 vLLM 上运行最佳、GGUF 模型需要 llama.cpp。平台应该自动判断。

### 检测算法

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

### 格式 -> 引擎映射规则

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

### 硬件感知逻辑

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

### 用户接口

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

### 实现方式

Model Resolver 是一个在推理引擎启动前运行的 **init-container**：

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

### 交付件
- `model-resolver` Docker 镜像（Python，约 200 行代码）
- 格式检测库（解析 config.json、扫描文件扩展名）
- 硬件探测（通过 `nvidia-smi` 检测 GPU 类型、VRAM）
- 引擎镜像映射表（可通过 ConfigMap 配置）
- Helm 模板读取 resolver 输出并启动对应引擎

---

## 第 0 层：GitOps 与自动化交付

> 与 v1 相同，使用 ArgoCD（CNCF 毕业级）+ Helm（CNCF 毕业级）。

### 部署流程

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

### 预设值文件

```yaml
# values-minimal.yaml    - 1 GPU, 1 model, basic monitoring, no vector DB
# values-standard.yaml   - Multi-model, full OTel observability, pgvector
# values-production.yaml - HA, autoscaling, Milvus, multi-tenant, full security
```

---

## 第 1 层：基础设施与调度

> 核心与 v1 相同，关键变更：**Fluid（CNCF 沙箱级）替代 JuiceFS 作为主要缓存方案**。

| 组件 | 技术 | CNCF 状态 | 解决的问题 |
|---|---|---|---|
| GPU Operator | NVIDIA GPU Operator | - | 自动安装驱动、设备插件 |
| GPU 共享 | Time-Slicing / MIG | - | 单 GPU 多模型 |
| **模型缓存** | **Fluid + Alluxio** | **CNCF 沙箱级** | 分布式模型权重缓存、数据局部性调度 |
| 模型缓存（备选） | JuiceFS + MinIO | 非 CNCF | Fluid 不适用时的替代方案 |
| 节点自动扩缩 | Karpenter / Cluster Autoscaler | - | 自动扩展 GPU 节点 |
| 节点发现 | NFD | - | 自动标记 GPU 类型/拓扑 |

### 使用 Fluid 进行模型权重缓存

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

Pod 自动获得数据局部性调度：Fluid 会将推理 Pod 调度到已缓存数据的节点附近。

---

## 第 2 层：模型服务

> 增强了 Model Resolver 自动选择引擎功能（详见上文）。

### 引擎矩阵

| 引擎 | 最适场景 | GGUF | SafeTensors | AWQ/GPTQ | Embedding | CNCF |
|---|---|---|---|---|---|---|
| **vLLM** | 生产级 GPU 推理 | 否 | 是 | 是 | 是 | - |
| **llama.cpp** | GGUF / CPU / 边缘设备 | 是 | 部分 | 否 | 否 | - |
| **SGLang** | 多轮对话 / 结构化输出 | 否 | 是 | 是 | 否 | - |
| **TEI** | Embedding 与 Reranking | 否 | 是 | 否 | **是** | - |
| **TGI** | HuggingFace 生态 | 否 | 是 | 是 | 否 | - |

### 可选：KServe 集成（CNCF 孵化级）

对于已经在使用 KServe 的团队，kube-llmops 可以通过 KServe 的 InferenceService 部署模型：

```yaml
# Optional: wrap vLLM in KServe InferenceService for canary/serverless features
kserve:
  enabled: false     # off by default, enable if KServe is installed
  # Provides: canary rollout, scale-to-zero (Knative), InferenceService CRD
```

---

## 第 3 层：双层网关

> **v1 的重大变更**：拆分为第一层（AI Gateway）+ 第二层（Inference Gateway），与行业趋势和 CNCF 生态对齐。

### 为何采用双层架构

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

业界（Google GKE、llm-d、K8s SIG）正在向 **Envoy + Gateway API Inference Extension** 作为第二层标准方向收敛。LiteLLM 仍然作为第一层，因为没有任何 CNCF 项目能够处理 API 密钥管理 + 多供应商路由 + 成本追踪。

### 第一层：LiteLLM（AI Gateway）

与 v1 保持不变。负责：
- 面向所有供应商的 OpenAI 兼容 API
- API 密钥管理 + Token 预算（PostgreSQL 后端）
- 按用户/按团队的成本追踪
- 多模型路由、故障回退链
- 速率限制（RPM、TPM）

### 第二层：Envoy AI Gateway + IGW（Inference Gateway）

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

**来自 IGW 的核心能力**：
- **KV cache 感知路由**：将请求路由到已拥有相关 KV cache 的 Pod（而非轮询）
- **Prefix cache 感知调度**：具有相似前缀的请求被调度到同一 Pod
- **LoRA 适配器路由**：按模型名称路由到正确的 LoRA 适配器
- **金丝雀发布**：通过 InferenceModel CRD 在模型版本间进行 A/B 分流
- **流量控制**：工作负载间的优先级 + 公平性

### 第二层上线策略

```
Phase 1 (MVP):     LiteLLM -> vLLM directly (no Tier 2)
Phase 2:           LiteLLM -> Envoy AI Gateway (basic) -> vLLM
Phase 3:           LiteLLM -> Envoy + IGW (full inference scheduling) -> vLLM
Phase 4 (future):  LiteLLM -> IGW + llm-d (disaggregated P/D serving)
```

---

## 第 4 层：可观测性（OpenTelemetry + Langfuse）

> **OpenTelemetry（CNCF 毕业级）作为统一采集管线，Langfuse 作为追踪后端。不使用 Jaeger —— 我们的调用链很简单，Langfuse 覆盖得更好。**

### 为何不用 Jaeger？

Jaeger 是一个优秀的通用分布式追踪工具。但 kube-llmops 不是通用微服务平台。我们的调用链是：

```
Client -> LiteLLM -> (Envoy) -> vLLM -> Response
```

仅 3-4 跳。为此部署一个完整的 Jaeger 栈（Collector + Query + Cassandra/ES）实属过度。而 Langfuse：

| 能力 | Jaeger | Langfuse | 结论 |
|---|---|---|---|
| 请求延迟追踪 | 是 | 是 | 平手 |
| 跨服务 Span 关联 | 是 | 是（通过 OTel OTLP） | 平手 |
| 完整的 Prompt/Completion 文本 | **否** | 是 | **Langfuse** |
| 每个请求的 Token 计数 | **否** | 是 | **Langfuse** |
| 每个请求/用户/团队的成本 | **否** | 是 | **Langfuse** |
| 用户会话分组 | **否** | 是 | **Langfuse** |
| Prompt 版本管理与 A/B 测试 | **否** | 是 | **Langfuse** |
| 评估评分（LLM-as-judge） | **否** | 是 | **Langfuse** |
| RAG 管线分解 | **否** | 是 | **Langfuse** |
| 需要额外维护的基础设施 | Collector + Query + Storage | 单服务 + PG | **Langfuse**（更简单） |

**结论**：Langfuse 在 LLM 追踪方面严格优于 Jaeger。引入 Jaeger 意味着需要额外维护一个有状态服务，却毫无附加价值。如果用户已有 Jaeger 集群并希望同时发送追踪数据，OTel Collector 只需一行 exporter 配置即可实现 —— 但我们默认不附带它。

### 为何仍然需要 OpenTelemetry？

OTel 不是后端 —— 它是**采集管线**。即使没有 Jaeger，OTel 也值得使用：

1. **统一采集**：vLLM、LiteLLM、Envoy 都原生支持 OTLP。一种协议采集一切。
2. **数据处理**：在导出前为追踪/指标添加 `model_name`、`user_id`、`tenant` 标签。
3. **扇出分发**：单一管线同时导出到 Prometheus（指标）+ Langfuse（追踪）+ Loki（日志）。后续添加任何后端都无需修改探针代码。
4. **解耦**：如果日后需要替换 Langfuse，只需修改 OTel exporter 配置。应用代码无需改动。

### 架构

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

### OTel Collector 配置

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

### LLM 可观测性三大支柱（不是四个 —— 追踪由 Langfuse 承担，不单独作为一个支柱）

#### 支柱 1：指标（Prometheus，通过 OTel Collector）

| 分类 | 关键指标 | 数据来源 |
|---|---|---|
| **LLM 延迟** | TTFT、ITL（Inter-Token Latency）、端到端 P50/P95/P99 | vLLM OTel SDK |
| **LLM 吞吐量** | 输入/输出 tokens/sec、请求/sec | vLLM OTel SDK |
| **LLM 引擎** | KV cache 利用率、等待请求数、批大小、Prefix cache 命中率 | vLLM metrics |
| **GPU 硬件** | GPU 利用率 %、VRAM、温度、功耗、ECC 错误 | DCGM Exporter |
| **网关** | 按模型/用户/状态分组的请求数、Token 消耗量 | LiteLLM OTel |
| **Inference Gateway** | 路由决策、cache 命中路由、P/D 拆分 | IGW/Envoy OTel |
| **成本** | 每 Token/请求/用户/日成本、预算余额 | LiteLLM PostgreSQL |

#### 支柱 2：追踪 + LLM 分析（Langfuse，通过 OTel OTLP）

Langfuse 接收 OTel 追踪数据并添加 LLM 专属上下文：

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

LiteLLM 原生集成（最简单、推荐方式）：
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

OTel 集成（用于未经过 LiteLLM 的 vLLM/Envoy 追踪数据）：
```yaml
# Already configured in OTel Collector above
exporters:
  otlphttp/langfuse:
    endpoint: http://langfuse:3000/api/public/otel
```

#### 支柱 3：日志（Fluentbit -> OTel Collector -> Loki）

```
Fluentbit (CNCF Graduated)       OTel Collector          Loki
  - DaemonSet on each node         - Transform             - Store
  - Tail container logs            - Add labels            - Query via Grafana
  - Forward to OTel Collector      - Export to Loki
```

| 日志类型 | 内容 |
|---|---|
| 请求日志 | Request ID、模型、用户、输入/输出 Token 数、延迟、状态 |
| 引擎日志 | vLLM/llama.cpp 内部日志、CUDA 错误、OOM 事件 |
| 网关日志 | 路由决策、速率限制触发、故障回退触发 |
| 审计日志 | API 密钥操作、模型部署、配置变更 |

### 可扩展性："我已经有 Jaeger / Datadog / Grafana Tempo 了"

由于 OTel Collector 是采集管线，添加任何额外后端只需一行配置：

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

这就是即使没有 Jaeger，OTel 仍然重要的原因：它是**可扩展层**。

### 预置 Grafana 仪表盘（3 个）

| # | 仪表盘 | 数据源 | 核心面板 |
|---|---|---|---|
| 1 | vLLM 模型服务总览 | Prometheus | 请求速率、延迟百分位（TTFT/ITL/E2E）、Token 吞吐量、KV cache 利用率 |
| 2 | LiteLLM AI Gateway | Prometheus + LiteLLM PG | 按模型分组的流量、活跃模型数、错误率、Token 用量、成本追踪 |
| 3 | GPU 与基础设施总览 | Prometheus (DCGM) | GPU/VRAM 资源使用、队列深度、TTFT、Inter-Token 延迟 |

### 告警规则（Prometheus，通过 OTel 指标）

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

### 交付件
- Helm 子 Chart：`charts/observability/`（OTel Collector + Prometheus + Fluentbit + Loki + DCGM Exporter）
- Helm 子 Chart：`charts/langfuse/`
- Helm 子 Chart：`charts/grafana/`（含 3 个预置仪表盘 JSON）
- 面向 LLM 工作负载的 OTel Collector 配置
- Prometheus 记录规则 + 告警规则
- Fluentbit DaemonSet 配置

---

## 第 5 层：数据与向量基础设施

| 组件 | 技术 | CNCF 状态 | 用途 |
|---|---|---|---|
| **向量数据库（规模化）** | **Milvus** | LF AI & Data | 生产级 RAG、分布式 |
| **向量数据库（轻量级）** | **pgvector** | PostgreSQL 扩展 | 复用 LiteLLM 的 PG，零额外基础设施 |
| **对象存储** | **MinIO** | - | 模型权重、数据集、备份 |
| **模型仓库** | **Harbor** | **CNCF 毕业级** | 基于 OCI 的模型制品版本管理 |
| **数据版本控制** | **DVC** / LakeFS | - | 将训练数据集与 Git 一起版本化 |

### Harbor 作为模型仓库（CNCF 毕业级）

不使用自定义模型仓库，而是使用 Harbor 将模型制品存储为 OCI 镜像：

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

优势：版本管理、访问控制、漏洞扫描、跨镜像仓库复制 —— 所有功能 Harbor 原生内置。

---

## 第 6 层：模型开发与微调

与 v1 保持不变：
- **JupyterHub** 部署在 K8s 上（GPU Notebook 环境）
- **LLaMA-Factory** 作为 K8s Job（LoRA、QLoRA、全量微调）
- **MLflow** 实验追踪（复用 PostgreSQL + MinIO）
- **Label Studio**（可选，数据标注）

---

## 横切关注点：安全与多租户

| 组件 | 技术 | CNCF 状态 |
|---|---|---|
| 认证 / SSO | **Keycloak** | **CNCF 孵化级** |
| API 密钥管理 | **LiteLLM** 内置 | - |
| 网络策略 | **Cilium** | **CNCF 毕业级** |
| 密钥管理 | **External Secrets Operator** | - |
| 内容安全 | **LLM-Guard** | - |
| 审计 | OTel 追踪 + Loki 日志 | CNCF 毕业级 |

---

## 横切关注点：自动扩缩与成本

| 组件 | 技术 | CNCF 状态 |
|---|---|---|
| Pod 自动扩缩 | **KEDA** | **CNCF 毕业级** |
| 节点自动扩缩 | Karpenter / Cluster Autoscaler | - |
| 缩容到零 | KEDA ScaledObject | CNCF 毕业级 |
| 竞价/抢占实例 | Karpenter spot handler | - |

面向 LLM 工作负载的 KEDA 触发器：
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

## 技术栈总结（v2，CNCF 对齐）

| 分类 | v1 选择 | v2 选择 | 变更原因 |
|---|---|---|---|
| 可观测性管线 | （无，直连） | **OpenTelemetry**（CNCF 毕业级） | 统一管线，CNCF 标准 |
| 追踪 | Tempo | **Langfuse**（通过 OTel OTLP） | Langfuse 在 LLM 场景优于 Jaeger：兼具追踪 + Prompt/Token/成本/会话追踪。不为 CNCF 标签引入 Jaeger。 |
| 日志采集器 | Promtail | **Fluentbit**（CNCF 毕业级） | CNCF > 非 CNCF |
| 模型缓存 | JuiceFS | **Fluid**（CNCF 沙箱级） | CNCF > 非 CNCF |
| 模型仓库 | OCI Registry | **Harbor**（CNCF 毕业级） | CNCF，内置访问控制 |
| 网关第二层 | （无） | **Envoy AI Gateway + IGW**（Envoy=CNCF 毕业级） | 行业标准，KV 感知路由 |
| 网络 | Calico | **Cilium**（CNCF 毕业级） | CNCF，基于 eBPF |
| 引擎选择 | 手动 | **Model Resolver**（自动检测） | 新功能 |
| 指标 | Prometheus（直连） | **Prometheus 通过 OTel** | 统一管线 |
| 仪表盘 | Grafana | Grafana（不变） | 无 CNCF 替代方案 |
| AI Gateway 第一层 | LiteLLM | LiteLLM（不变） | 无 CNCF 替代方案 |
| LLM 追踪 | Langfuse | Langfuse 通过 OTel（不变） | 无 CNCF 替代方案 |
| 推理引擎 | vLLM | vLLM + llama.cpp + TEI（自动） | 增强自动选择 |

---

## 仓库结构（v2）

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

## 路线图（v2，已更新）

### 第一阶段：基础建设（MVP）—— "部署一个模型，看到指标"
- [x] 仓库脚手架、Makefile、CI、lint
- [x] **Model Resolver** init-container（格式检测 + 引擎自动选择）
- [x] vLLM Helm 子 Chart + llama.cpp Helm 子 Chart
- [x] LiteLLM Helm 子 Chart（第一层网关，PostgreSQL）
- [x] **OpenTelemetry Collector** + Prometheus + DCGM Exporter
- [x] Grafana + 3 个仪表盘（总览、GPU 集群、单模型）
- [x] Umbrella Chart + `values-minimal.yaml`
- [x] 快速入门指南 + README
- [x] `scripts/install.sh`

### 第二阶段：生产就绪 —— "在生产环境运行"
- [x] 多模型部署（vLLM + TEI + llama.cpp，自动选择）
- [x] LiteLLM 高级功能（故障回退、重试、速率限制、预算控制）
- [x] **Langfuse** 集成（LiteLLM 回调 + OTel OTLP）
- [x] **Fluentbit** + Loki 日志管线
- [x] 3 个 Grafana 仪表盘 + 4 条 Prometheus 告警规则（延迟、队列、KV 缓存、宕机）
- [x] **KEDA** 自动扩缩（等待请求数、TTFT）—— 已通过 KEDA operator 验证
- [x] **Fluid** 分布式模型缓存 —— 模板已就绪，需预装 Fluid operator
- [x] MinIO 对象存储（模型已上传并验证）+ **Harbor** 模板
- [x] `values-standard.yaml` + `values-production.yaml`
- [x] Keycloak SSO（Grafana OIDC 已验证）+ NetworkPolicy 模板

### 第三阶段：RAG 与推理优化 —— "构建 RAG 应用，优化路由"
- [ ] pgvector（复用 LiteLLM PG）
- [ ] Milvus Helm 子 Chart（单机 + 集群）
- [ ] TEI embedding/reranking 服务
- [ ] RAG 数据摄取 Worker + 示例应用
- [ ] **Envoy AI Gateway + IGW**（第二层，KV cache 感知路由）
- [ ] **LoRA 适配器路由**（通过 IGW InferenceModel CRD）
- [ ] 多租户（LiteLLM Teams + K8s Namespace + ResourceQuota）
- [ ] `values-production.yaml`
- [ ] 备份/恢复自动化

### 第四阶段：ML 平台 —— "ML 工程师的最爱"
- [ ] 带 GPU 配置的 JupyterHub
- [ ] LLaMA-Factory 微调 Job 模板
- [ ] MLflow 实验追踪
- [ ] 模型评估管线
- [ ] ArgoCD ApplicationSet 多集群支持
- [ ] Terraform 模块（EKS、GKE、ACK）

### 第五阶段：高级推理 —— "顶尖性能"
- [ ] **llm-d** 集成（Prefill/Decode 分离式推理）
- [ ] MoE 模型的**专家并行**（DeepSeek-R1）
- [ ] KV cache 分级卸载（GPU -> CPU -> SSD -> 远端）
- [ ] 工作负载感知自动扩缩（SLO 感知）
- [ ] KServe 集成（可选）
- [ ] AMD ROCm / Intel Gaudi 支持

### 第六阶段：生态建设（未来）
- [ ] Kubernetes Operator 与 CRD
- [ ] CLI 工具（`kube-llmops` / `kubectl llmops`）
- [ ] Web 管理界面
- [ ] 多模态模型服务
- [ ] 模型优化工具箱（量化、蒸馏）

---

## 竞争定位（基于行业调研更新）

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

| 功能 | KAITO | KServe+llm-d | llmaz | KubeAI | kube-llmops |
|---|---|---|---|---|---|
| 引擎自动选择 | 否（预设） | 否 | 否 | 否 | **是** |
| AI Gateway（密钥/成本） | 否 | 否 | 部分 | 否 | **是（LiteLLM）** |
| KV cache 感知路由 | 否 | **是（IGW）** | **是（IGW）** | 否 | **是（第三阶段）** |
| OTel 可观测性 | 否 | 部分 | 否 | 否 | **是（全栈）** |
| 预置仪表盘 | 否 | 否 | 否 | 否 | **是（3 个）** |
| LLM 追踪（Prompt） | 否 | 否 | 否 | 否 | **是（Langfuse）** |
| 向量数据库 | 仅 faiss | 否 | 否 | 否 | **是（Milvus/pgvector）** |
| 微调工作流 | 是 | 否 | 否 | 否 | **是** |
| 开发环境 | 否 | 否 | 否 | 否 | **是（JupyterHub）** |
| CNCF 对齐度 | 低 | **高** | 中 | 低 | **高** |
| 云厂商无关 | 仅 Azure | 是 | 是 | 是 | **是** |
| 一键全栈部署 | 否 | 否 | 否 | 否 | **是** |

---

## 许可证

Apache License 2.0
