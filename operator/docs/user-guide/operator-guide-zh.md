# kube-llmops Operator 用户指南

> **版本**：v1alpha1 · **API Group**：`llmops.kubellmops.io`
>
> 本文档面向集群管理员和 MLOps 工程师，涵盖 kube-llmops Operator 的安装、日常操作与运维全流程。

---

## 目录

- [1. 快速开始](#1-快速开始)
- [2. 核心概念](#2-核心概念)
- [3. 模型管理](#3-模型管理)
- [4. 平台配置](#4-平台配置)
- [5. 模型微调](#5-模型微调)
- [6. 运维指南](#6-运维指南)
- [7. 迁移指南](#7-迁移指南)
- [8. API 参考](#8-api-参考)

---

## 1. 快速开始

本章节将带你从零开始，在 Kubernetes 集群中完成 Operator 安装、平台部署和模型上线的完整流程。

### 1.1 前置条件

| 组件 | 最低版本 |
|------|----------|
| Kubernetes | v1.26+ |
| Helm | v3.12+ |
| kubectl | v1.26+ |
| GPU 驱动（可选） | NVIDIA Driver 535+、NVIDIA Device Plugin |

### 1.2 安装 Operator

使用 Helm 一键安装 Operator 及其 CRD：

```bash
helm install kube-llmops-operator charts/kube-llmops-operator/
```

验证 Operator Pod 是否正常运行：

```bash
kubectl get pods -l app.kubernetes.io/name=kube-llmops-operator
```

预期输出：

```
NAME                                       READY   STATUS    RESTARTS   AGE
kube-llmops-operator-6d8f7b4c9f-xk2rp     1/1     Running   0          45s
```

确认三个 CRD 已注册：

```bash
kubectl get crd | grep llmops
```

预期输出：

```
finetuneruns.llmops.kubellmops.io       2026-01-15T10:00:00Z
llmplatforms.llmops.kubellmops.io       2026-01-15T10:00:00Z
modeldeployments.llmops.kubellmops.io   2026-01-15T10:00:00Z
```

### 1.3 部署平台基础设施

创建一个最小化的 LLMPlatform，启用 AI 网关、可观测性和模型缓存：

```bash
kubectl apply -f config/samples/llmplatform_minimal.yaml
```

该配置文件内容如下：

```yaml
apiVersion: llmops.kubellmops.io/v1alpha1
kind: LLMPlatform
metadata:
  name: kube-llmops
spec:
  gateway:
    enabled: true
  observability:
    enabled: true
  modelStore:
    enabled: true
    endpoint: kube-llmops-minio:9000
    bucket: models
    accessKey: minioadmin
    secretKey: minioadmin
    image: kube-llmops/model-loader:latest
  postgresql:
    enabled: true
```

查看平台状态：

```bash
kubectl get lp
```

预期输出：

```
NAME          PHASE   GATEWAY   GRAFANA   AGE
kube-llmops   Ready   Ready     Ready     2m
```

### 1.4 部署第一个模型

部署一个 vLLM 推理模型：

```bash
kubectl apply -f config/samples/modeldeployment_vllm.yaml
```

查看模型部署状态：

```bash
kubectl get md
```

预期输出（部署中）：

```
NAME           ENGINE   REPLICAS   PHASE       AGE
gemma-4-26b    vllm     0          Deploying   30s
```

等待模型下载并启动完成后：

```
NAME           ENGINE   REPLICAS   PHASE   AGE
gemma-4-26b    vllm     1          Ready   5m
```

查看详细信息（包含 Endpoint）：

```bash
kubectl get md -o wide
```

预期输出：

```
NAME           ENGINE   REPLICAS   PHASE   ENDPOINT                                                    AGE
gemma-4-26b    vllm     1          Ready   http://gemma-4-26b.default.svc.cluster.local:8000           5m
```

### 1.5 验证模型服务

通过集群内部地址调用模型 API：

```bash
kubectl run curl-test --rm -it --image=curlimages/curl -- \
  curl -s http://gemma-4-26b.default.svc.cluster.local:8000/v1/models | jq .
```

如果已启用 AI 网关，也可以通过 LiteLLM 统一入口访问：

```bash
kubectl run curl-test --rm -it --image=curlimages/curl -- \
  curl -s http://kube-llmops-litellm:4000/v1/chat/completions \
    -H "Content-Type: application/json" \
    -d '{
      "model": "gemma-4-26b",
      "messages": [{"role": "user", "content": "你好"}]
    }'
```

### 1.6 清理资源

```bash
kubectl delete md gemma-4-26b
kubectl delete lp kube-llmops
helm uninstall kube-llmops-operator
```

---

## 2. 核心概念

### 2.1 CRD 概述

kube-llmops Operator 通过三个自定义资源定义（CRD）来管理 LLM 平台的完整生命周期：

| CRD | 简称 | 用途 | API Group |
|-----|------|------|-----------|
| **ModelDeployment** | `md` | 管理模型推理服务的部署与生命周期 | `llmops.kubellmops.io/v1alpha1` |
| **LLMPlatform** | `lp`、`llmplatforms` | 管理平台基础设施（网关、监控、存储等） | `llmops.kubellmops.io/v1alpha1` |
| **FineTuneRun** | `ftr` | 管理模型微调任务的全流程 | `llmops.kubellmops.io/v1alpha1` |

它们之间的关系：

```
LLMPlatform (平台基础设施)
├── AI Gateway (LiteLLM)
├── 可观测性 (Prometheus + Grafana + Langfuse)
├── 模型存储 (MinIO)
├── 数据库 (PostgreSQL)
└── ...

ModelDeployment (模型推理)          FineTuneRun (模型微调)
├── 自动检测引擎                    ├── 创建 Argo Workflow
├── 创建 PVC / Deployment / Service ├── 训练 → 评估 → 质量门控
├── 注册到 AI Gateway               └── 自动部署微调后的模型
└── 状态同步
```

### 2.2 引擎自动检测机制

创建 ModelDeployment 时，可以将 `engine` 字段设为 `auto`（默认值），Operator 会根据模型名称自动推断最合适的推理引擎：

| 检测规则 | 匹配条件 | 推断引擎 |
|----------|----------|----------|
| GGUF 量化模型 | 模型名称中包含 `gguf`（不区分大小写，匹配 `*GGUF*` / `*gguf*`） | `llamacpp` |
| 嵌入模型 | 包含 `bge-`、`e5-`、`gte-`、`minilm`、`jina-embed`、`nomic-embed`、`all-mpnet`、`embedding` 等关键词 | `tei` |
| 重排序模型 | 包含 `rerank` | `tei` |
| 通用 LLM | 不匹配上述任何规则 | `vllm` |

优先级顺序：**显式指定** > **自动检测** > **默认 vllm**

> **注意**：GGUF 检测使用 `strings.ToLower` + `strings.Contains`，同时匹配两个子串：
> `"gguf"`（标准拼写）和 `"guff"`（部分 HF 社区仓库的常见 typo，如
> `nohurry/gemma-4-26B-A4B-it-heretic-GUFF`）。两者都解析为 `llamacpp`。

示例：

```yaml
# 自动检测为 vllm（通用 LLM，不含特殊关键词）
source: Qwen/Qwen2.5-7B-Instruct

# 自动检测为 tei（包含 "bge-"）
source: BAAI/bge-small-en-v1.5

# 自动检测为 tei（包含 "rerank"）
source: BAAI/bge-reranker-base

# 自动检测为 llamacpp（包含 "GGUF"）
source: TheBloke/Llama-2-7B-GGUF

# 同样自动检测为 llamacpp（v0.5.0+ 容错 "GUFF" typo）
source: nohurry/gemma-4-26B-A4B-it-heretic-GUFF
# engine: llamacpp  # 显式设置是可选的
```

### 2.3 协调循环

Operator 采用 Kubernetes 控制器的标准协调（Reconcile）模式，持续将集群的实际状态收敛到用户声明的期望状态。

**ModelDeployment 控制器的协调流程：**

1. 读取 ModelDeployment CR
2. 处理删除逻辑（从网关注销模型、移除 Finalizer）
3. 添加 Finalizer（确保删除时能执行清理）
4. 解析推理引擎（自动检测或使用用户指定值）
5. 查找同命名空间下的 LLMPlatform（获取模型存储配置）
6. 创建或更新 PVC（模型缓存卷）
7. 创建或更新 Deployment（推理服务 Pod）
8. 创建 Service（集群内访问入口）
9. 同步状态（Phase、ReadyReplicas、Endpoint）
10. 当状态为 Ready 时，自动注册到 AI Gateway

**LLMPlatform 控制器的协调流程：**

1. 读取 LLMPlatform CR
2. 将 Spec 转换为 Helm values
3. 检查 Helm Release 是否存在
4. 不存在则 Install，已存在则 Upgrade
5. 更新状态（Phase、HelmRelease、HelmRevision）

**FineTuneRun 控制器的协调流程：**

1. 读取 FineTuneRun CR
2. 如果已有 Argo Workflow → 同步 Workflow 状态
3. 否则创建 Argo Workflow（六步 DAG 流水线）
4. 每 30 秒轮询 Workflow 状态直到终态

### 2.4 状态阶段

#### ModelDeployment 状态阶段

```
Pending → Downloading → Deploying → Ready
                                  ↘ Degraded
                                  ↘ Failed
```

| 阶段 | 说明 |
|------|------|
| `Pending` | CR 已创建，等待处理 |
| `Downloading` | 模型文件正在从 HuggingFace 或 MinIO 下载 |
| `Deploying` | Deployment 已创建，Pod 正在启动中，尚无就绪副本 |
| `Ready` | 所有期望副本均已就绪，模型已注册到网关 |
| `Degraded` | 部分副本就绪，但未达到期望数量 |
| `Failed` | 部署失败 |

#### LLMPlatform 状态阶段

```
Pending → Installing → Ready
                    ↘ Failed
Ready → Upgrading → Ready
                 ↘ Failed
```

| 阶段 | 说明 |
|------|------|
| `Pending` | CR 已创建，等待处理 |
| `Installing` | Helm Release 正在安装 |
| `Ready` | 所有组件部署成功 |
| `Upgrading` | 配置变更触发 Helm Release 升级 |
| `Degraded` | 部分组件异常 |
| `Failed` | 安装或升级失败 |

#### FineTuneRun 状态阶段

```
Pending → DataPreparing → Training → Evaluating → QualityGate → Deploying → Succeeded
                                                                           ↘ Failed
```

| 阶段 | 说明 |
|------|------|
| `Pending` | Argo Workflow 已提交，等待调度 |
| `DataPreparing` | 正在下载基础模型和训练数据 |
| `Training` | 模型训练进行中 |
| `Evaluating` | 训练后评估阶段 |
| `QualityGate` | 质量门控检查 |
| `Deploying` | 通过质量检查后自动部署模型 |
| `Succeeded` | 微调流程全部完成 |
| `Failed` | 任意阶段失败 |

---

## 3. 模型管理

### 3.1 部署 vLLM 模型

vLLM 是默认的推理引擎，适用于绝大多数因果语言模型（Causal LM），支持 PagedAttention、连续批处理等高性能特性。

```yaml
# config/samples/modeldeployment_vllm.yaml
apiVersion: llmops.kubellmops.io/v1alpha1
kind: ModelDeployment
metadata:
  name: qwen-7b
spec:
  source: Qwen/Qwen2.5-7B-Instruct
  replicas: 1
  resources:
    gpu: 1
    memory: 24Gi
    cpu: "4"
  engineArgs:
    --gpu-memory-utilization: "0.93"
    --max-model-len: "8192"
    --dtype: "half"
    --enforce-eager: ""
```

```bash
kubectl apply -f config/samples/modeldeployment_vllm.yaml
```

**字段说明：**

- `source`：HuggingFace 模型 ID 或路径
- `replicas`：推理服务副本数
- `resources.gpu`：请求的 GPU 数量
- `resources.memory`：内存限制
- `resources.cpu`：CPU 限制
- `engineArgs`：传递给 vLLM 的额外命令行参数（键值对形式）

> **提示**：`engineArgs` 中值为空字符串 `""` 的参数（如 `--enforce-eager`）会作为无值标志传入引擎。

Operator 会自动创建以下 Kubernetes 资源：

| 资源 | 名称 | 说明 |
|------|------|------|
| PersistentVolumeClaim | `qwen-7b-cache` | 50Gi 模型缓存卷 |
| Deployment | `qwen-7b` | 推理服务 Pod（含 model-loader init 容器） |
| Service | `qwen-7b` | ClusterIP 服务，端口 8000 |

### 3.2 部署 TEI 嵌入模型

Text Embeddings Inference（TEI）是 HuggingFace 推出的高性能嵌入推理引擎。当模型名称包含嵌入模型特征关键词时，引擎会自动检测为 `tei`。

```yaml
# config/samples/modeldeployment_tei_embedding.yaml
apiVersion: llmops.kubellmops.io/v1alpha1
kind: ModelDeployment
metadata:
  name: bge-small-en
spec:
  source: BAAI/bge-small-en-v1.5
  replicas: 1
  resources:
    gpu: 0
    cpu: "0.5"
    memory: 256Mi
```

```bash
kubectl apply -f config/samples/modeldeployment_tei_embedding.yaml
```

> **注意**：`gpu: 0` 表示仅使用 CPU 推理。TEI 对小型嵌入模型的 CPU 推理性能非常出色，可省去 GPU 开销。

TEI 引擎使用端口 8080，PVC 默认分配 10Gi（小于 vLLM 的 50Gi，因为嵌入模型通常较小）。

查看自动检测的引擎：

```bash
kubectl get md bge-small-en
```

```
NAME           ENGINE   REPLICAS   PHASE   AGE
bge-small-en   tei      1          Ready   2m
```

### 3.3 部署 TEI 重排序模型

重排序模型同样使用 TEI 引擎，模型名称中的 `rerank` 关键词会触发自动检测。

```yaml
# config/samples/modeldeployment_tei_reranker.yaml
apiVersion: llmops.kubellmops.io/v1alpha1
kind: ModelDeployment
metadata:
  name: bge-reranker-base
spec:
  source: BAAI/bge-reranker-base
  replicas: 1
  resources:
    gpu: 0
    cpu: "1"
    memory: 1Gi
```

```bash
kubectl apply -f config/samples/modeldeployment_tei_reranker.yaml
```

### 3.4 部署 llama.cpp GGUF 模型

对于 GGUF 格式的量化模型，使用 llama.cpp 引擎。当模型名称包含 `gguf` 时会自动检测，也可以显式指定 `engine: llamacpp`。

```yaml
# config/samples/modeldeployment_llamacpp.yaml
apiVersion: llmops.kubellmops.io/v1alpha1
kind: ModelDeployment
metadata:
  name: llama-gguf
spec:
  source: TheBloke/Llama-2-7B-GGUF
  engine: llamacpp
  replicas: 1
  resources:
    gpu: 1
    memory: 16Gi
```

```bash
kubectl apply -f config/samples/modeldeployment_llamacpp.yaml
```

> **提示**：即使模型名称已包含 `GGUF`，显式设置 `engine: llamacpp` 也是推荐的做法——明确优于隐晦。

llama.cpp 引擎的 PVC 默认分配 30Gi，服务端口为 8080。v0.5.0 使用 `ghcr.io/ggml-org/llama.cpp:server-cuda-b8672` 镜像。
Deployment 采用 `Recreate` 策略（而非 `RollingUpdate`）——因为 GPU 设备无法被新旧两个 Pod 同时占用，
滚动更新会导致新 Pod 永远无法分配到 GPU 而陷入死锁。

#### 分片（split）GGUF 模型支持

大型 GGUF 模型通常以 `{prefix}-NNNNN-of-NNNNN.gguf`（如 `model-00001-of-00009.gguf`）命名的多个分片发布。
Operator 会透明处理这种情况：

1. `model-loader` init 容器会从 HuggingFace **下载所有匹配的分片**（并同步到 MinIO 供后续 Pod 复用）。
2. Pod 启动钩子为每个分片创建符号链接，保证 llama.cpp 可以按约定命名加载所有分片。
3. llama.cpp 启动时 `--model` 参数指向**第一个**分片，server 会自动发现并加载其余分片。

使用 `allowPatterns` 可以只下载某一种量化（避免下载仓库中几十 GB 的其他量化版本）：

```yaml
apiVersion: llmops.kubellmops.io/v1alpha1
kind: ModelDeployment
metadata:
  name: gemma-4-26b
spec:
  source: nohurry/gemma-4-26B-A4B-it-heretic-GUFF
  engine: llamacpp           # 显式设置（可选）：自动检测对 "GUFF" typo 也生效
  replicas: 1
  resources:
    gpu: 1
    memory: 20Gi
    cpu: "4"
  allowPatterns: "*q4_k_m*"  # 仅下载 q4_k_m 分片（总计约 16.87GB）
  engineArgs:
    --jinja: ""              # 启用 Jinja 聊天模板
    --ctx-size: "163840"     # 160K 上下文（模型本身支持 256K，受 24GB 显存限制）
```

这就是 v0.5.0 默认的 LLM 配置，可在 RTX 3090（24GB 显存）单卡上运行。

### 3.5 副本扩缩容

#### 水平扩容

修改 `replicas` 字段即可触发 Deployment 的滚动更新：

```bash
kubectl patch md gemma-4-26b --type merge -p '{"spec":{"replicas":3}}'
```

验证扩容结果：

```bash
kubectl get md gemma-4-26b -w
```

```
NAME           ENGINE   REPLICAS   PHASE      AGE
gemma-4-26b    vllm     1          Degraded   5m
gemma-4-26b    vllm     2          Degraded   6m
gemma-4-26b    vllm     3          Ready      8m
```

#### 缩容到零

将副本设为 0 可释放 GPU 资源，同时保留 PVC 中的模型缓存：

```bash
kubectl patch md gemma-4-26b --type merge -p '{"spec":{"replicas":0}}'
```

#### 使用 kubectl scale

```bash
kubectl scale md gemma-4-26b --replicas=2
```

### 3.6 自定义引擎参数

通过 `engineArgs` 传递引擎特定的命令行参数。这些参数会按字典序排列后追加到引擎默认参数之后。

**vLLM 常用参数：**

```yaml
spec:
  engineArgs:
    --gpu-memory-utilization: "0.95"    # GPU 显存利用率
    --max-model-len: "32768"            # 最大上下文长度
    --dtype: "half"                     # 计算精度
    --enforce-eager: ""                 # 关闭 CUDA Graph，降低显存占用
    --quantization: "awq"              # 量化方法
    --tensor-parallel-size: "2"         # 张量并行（需多 GPU）
```

**启用前缀缓存**（vLLM 独有）：

```yaml
spec:
  prefixCaching: true   # 开启自动前缀缓存，提升相似请求的推理速度
```

**TEI 常用参数：**

```yaml
spec:
  engineArgs:
    --max-batch-tokens: "16384"
    --max-concurrent-requests: "512"
```

**llama.cpp 常用参数：**

```yaml
spec:
  engineArgs:
    --ctx-size: "4096"           # 上下文长度
    --n-gpu-layers: "35"         # 卸载到 GPU 的层数
    --threads: "8"               # CPU 推理线程数
```

### 3.7 使用 MIG 设备

NVIDIA Multi-Instance GPU（MIG）允许将一块物理 GPU 划分为多个独立的 GPU 实例。通过 `migDevice` 字段指定 MIG 设备资源名：

```yaml
apiVersion: llmops.kubellmops.io/v1alpha1
kind: ModelDeployment
metadata:
  name: small-model-mig
spec:
  source: Qwen/Qwen2.5-1.5B-Instruct
  resources:
    gpu: 0          # 当使用 MIG 时，将 gpu 设为 0
    memory: 8Gi
    cpu: "2"
  migDevice: "nvidia.com/mig-3g.20gb"   # MIG 3g.20gb profile
```

`migDevice` 字段直接映射为 Pod 的资源请求，常见的 MIG Profile 如下：

| Profile | 显存 | 适用场景 |
|---------|------|----------|
| `nvidia.com/mig-1g.5gb` | 5GB | 小型嵌入模型 |
| `nvidia.com/mig-2g.10gb` | 10GB | 中型嵌入模型、重排序模型 |
| `nvidia.com/mig-3g.20gb` | 20GB | 小型 LLM（1.5B-7B 量化） |
| `nvidia.com/mig-7g.40gb` | 40GB | 中型 LLM |

### 3.8 AMD / Gaudi GPU 支持

通过 `accelerator` 字段指定 GPU 厂商，Operator 会自动选择对应的 Kubernetes 设备插件资源名称：

| accelerator 值 | 设备资源名 |
|----------------|------------|
| `nvidia`（默认） | `nvidia.com/gpu` |
| `amd` | `amd.com/gpu` |
| `gaudi` | `habana.ai/gaudi` |

**AMD GPU 示例：**

```yaml
apiVersion: llmops.kubellmops.io/v1alpha1
kind: ModelDeployment
metadata:
  name: model-amd
spec:
  source: meta-llama/Llama-3.1-8B-Instruct
  accelerator: amd
  resources:
    gpu: 1
    memory: 32Gi
    cpu: "8"
```

**Intel Gaudi 示例：**

```yaml
apiVersion: llmops.kubellmops.io/v1alpha1
kind: ModelDeployment
metadata:
  name: model-gaudi
spec:
  source: meta-llama/Llama-3.1-8B-Instruct
  accelerator: gaudi
  resources:
    gpu: 1
    memory: 32Gi
    cpu: "8"
```

### 3.9 Spot 实例调度

对于成本敏感的工作负载，可以启用 Spot/抢占式实例调度：

```yaml
spec:
  spot:
    enabled: true
    provider: aws    # 可选：aws, gcp, azure, karpenter
```

### 3.10 金丝雀部署

通过 `canary` 配置实现 A/B 流量分割，在新旧模型间渐进切换：

```yaml
spec:
  source: Qwen/Qwen2.5-7B-Instruct
  canary:
    source: Qwen/Qwen2.5-14B-Instruct     # 金丝雀模型（更大版本）
    weight: 20                                     # 20% 流量导向金丝雀
    resources:
      gpu: 1
      memory: 24Gi
```

---

## 4. 平台配置

### 4.1 最小化配置

最小化配置仅启用核心组件——AI 网关、可观测性和模型存储：

```yaml
# config/samples/llmplatform_minimal.yaml
apiVersion: llmops.kubellmops.io/v1alpha1
kind: LLMPlatform
metadata:
  name: kube-llmops
spec:
  gateway:
    enabled: true
  observability:
    enabled: true
  modelStore:
    enabled: true
    endpoint: kube-llmops-minio:9000
    bucket: models
    accessKey: minioadmin
    secretKey: minioadmin
    image: kube-llmops/model-loader:latest
  postgresql:
    enabled: true
```

```bash
kubectl apply -f config/samples/llmplatform_minimal.yaml
```

此配置会部署以下组件：

| 组件 | 说明 |
|------|------|
| LiteLLM | OpenAI 兼容的 AI 网关，统一管理多模型路由 |
| Prometheus + Grafana | 监控与可视化 |
| MinIO | 模型文件缓存存储 |
| PostgreSQL | LiteLLM 和 Langfuse 的共享数据库 |

### 4.2 完整配置

完整配置启用所有功能模块，适用于生产环境：

```yaml
# config/samples/llmplatform_full.yaml
apiVersion: llmops.kubellmops.io/v1alpha1
kind: LLMPlatform
metadata:
  name: kube-llmops
spec:
  gateway:
    enabled: true
    routing: latency-based-routing           # 基于延迟的智能路由
  observability:
    enabled: true
    grafana:
      adminPassword: admin                   # Grafana 管理员密码
    langfuse:
      enabled: true                          # LLM 可观测性平台
  logging:
    enabled: true                            # Fluent Bit + Loki 日志采集
  modules:
    rag:
      enabled: true                          # RAG 模块（Dify + Milvus）
    finetune:
      enabled: true                          # 微调模块（Argo + MLflow）
    security:
      enabled: false                         # 安全模块（Keycloak SSO）
  modelStore:
    enabled: true
    endpoint: kube-llmops-minio:9000
    bucket: models
    accessKey: minioadmin
    secretKey: minioadmin
    hfTransferConcurrency: 32                # 并行下载线程数
    image: kube-llmops/model-loader:latest
  postgresql:
    enabled: true
  nodePort:
    enabled: true
    host: "172.29.193.187"                   # NodePort 绑定的主机 IP
```

```bash
kubectl apply -f config/samples/llmplatform_full.yaml
```

### 4.3 网关路由策略

AI 网关（LiteLLM）支持以下路由策略：

| 策略 | 说明 |
|------|------|
| `simple-shuffle` | 简单随机路由 |
| `least-busy` | 选择当前负载最低的后端 |
| `latency-based-routing` | 基于实际延迟的智能路由 |
| `usage-based-routing` | 基于使用量的路由分配 |

```yaml
spec:
  gateway:
    enabled: true
    routing: latency-based-routing
```

### 4.4 NodePort 访问配置

在没有 LoadBalancer 的环境中（如裸机集群、本地开发），使用 NodePort 暴露服务：

```yaml
spec:
  nodePort:
    enabled: true
    host: "192.168.1.100"    # 节点 IP 地址
```

启用后，各组件的 NodePort 端口信息可通过 LLMPlatform 状态查看：

```bash
kubectl get lp kube-llmops -o jsonpath='{.status.components}' | jq .
```

### 4.5 Ingress 配置

对于有 Ingress Controller 的集群，推荐使用 Ingress 暴露服务：

```yaml
spec:
  ingress:
    enabled: true
    className: nginx                         # Ingress Controller 类型
    host: llmops.example.com                 # 域名
```

### 4.6 启用 / 禁用功能模块

通过 `modules` 字段按需开关各功能模块：

```yaml
spec:
  modules:
    rag:
      enabled: true       # RAG 管道（Dify + Milvus 向量库）
    finetune:
      enabled: true       # 模型微调（Argo Workflows + MLflow）
    security:
      enabled: false      # SSO 认证（Keycloak）
```

**模块依赖关系：**

- `finetune` 模块需要 `modelStore` 和 `postgresql` 启用
- `rag` 模块需要 `postgresql` 启用
- `security` 模块启用后会部署 Keycloak，并为 Grafana 配置 OIDC

### 4.7 HuggingFace Token 配置

部署 Gated 模型（如 Llama 3 系列）需要提供 HuggingFace API Token：

```yaml
spec:
  hfToken: "hf_xxxxxxxxxxxxxxxxxxxxxxxxxxxx"
```

> **安全建议**：生产环境中建议使用 Kubernetes Secret 而非明文 Token。后续版本将支持 `secretRef` 引用方式。

### 4.8 修改已有配置

LLMPlatform 采用声明式管理。修改 YAML 后重新 apply，Operator 会自动检测变更并触发 Helm Release 升级：

```bash
# 编辑配置
kubectl edit lp kube-llmops

# 或修改 YAML 后重新 apply
kubectl apply -f config/samples/llmplatform_full.yaml
```

查看升级过程：

```bash
kubectl get lp kube-llmops -w
```

```
NAME          PHASE       GATEWAY   GRAFANA   AGE
kube-llmops   Upgrading   Ready     Ready     1h
kube-llmops   Ready       Ready     Ready     1h
```

---

## 5. 模型微调

### 5.1 创建 LoRA 微调任务

FineTuneRun 管理从数据准备到模型部署的完整微调流水线。以下是一个 LoRA 微调示例：

```yaml
# config/samples/finetunerun_lora.yaml
apiVersion: llmops.kubellmops.io/v1alpha1
kind: FineTuneRun
metadata:
  name: gemma-lora-v1
spec:
  baseModel: google/gemma-2-9b     # 微调使用未量化的基础权重
  outputName: gemma-4-lora-v1
  method: lora
  dataSource:
    type: minio
    path: "s3://datasets/my-data/"
    format: alpaca
  training:
    epochs: 3
    batchSize: 4
    learningRate: "2e-4"
    loraRank: 16
    loraAlpha: 32
    loraTarget: "all"
  resources:
    gpu: 1
    memory: 24Gi
    cpu: "4"
  evaluation:
    enabled: true
  qualityGate:
    enabled: true
    thresholds:
      minEvalLoss: "0.8"
      maxTrainLoss: "0.5"
  deploy:
    enabled: false
    canaryWeight: 20
```

```bash
kubectl apply -f config/samples/finetunerun_lora.yaml
```

查看微调任务状态：

```bash
kubectl get ftr
```

```
NAME            BASE MODEL             METHOD   PHASE      AGE
gemma-lora-v1   google/gemma-2-9b     lora     Training   5m
```

查看详细信息（含训练指标）：

```bash
kubectl get ftr -o wide
```

```
NAME            BASE MODEL             METHOD   PHASE       LOSS    DURATION   AGE
gemma-lora-v1   google/gemma-2-9b     lora     Succeeded   0.42    2h15m      3h
```

Operator 会创建一个六步 Argo Workflow DAG：

```
prepare-data → finetune → merge-upload → evaluate → quality-gate → deploy
```

查看关联的 Argo Workflow：

```bash
kubectl get ftr gemma-lora-v1 -o jsonpath='{.status.argoWorkflow}'
# 输出：gemma-lora-v1-gemma-4-
```

### 5.2 微调方法

| 方法 | 字段值 | 说明 | 显存需求 |
|------|--------|------|----------|
| **LoRA** | `lora` | 低秩适配，仅训练少量参数 | 较低 |
| **QLoRA** | `qlora` | 量化 + LoRA，进一步降低显存 | 最低 |
| **全量微调** | `full` | 训练全部模型参数 | 最高 |

**QLoRA 示例：**

```yaml
spec:
  method: qlora
  training:
    epochs: 3
    batchSize: 2              # QLoRA 可用更小 batch size
    learningRate: "2e-4"
    loraRank: 8
    loraAlpha: 16
    loraTarget: "all"
```

**全量微调示例：**

```yaml
spec:
  method: full
  training:
    epochs: 2
    batchSize: 1
    learningRate: "5e-5"
    gradientAccumulationSteps: 8
    warmupRatio: "0.1"
  resources:
    gpu: 4                    # 全量微调通常需要多 GPU
    memory: 96Gi
    cpu: "16"
```

### 5.3 训练超参数

| 字段 | 说明 | 默认值 |
|------|------|--------|
| `epochs` | 训练轮次 | — |
| `batchSize` | 批大小 | — |
| `learningRate` | 学习率 | — |
| `gradientAccumulationSteps` | 梯度累积步数 | — |
| `warmupRatio` | 学习率预热比例 | — |
| `loraRank` | LoRA 秩 | — |
| `loraAlpha` | LoRA Alpha 系数 | — |
| `loraTarget` | LoRA 目标模块（`"all"` 或指定层） | — |

### 5.4 数据源配置

支持三种训练数据来源：

**MinIO（对象存储）：**

```yaml
spec:
  dataSource:
    type: minio
    path: "s3://datasets/my-data/"
    format: alpaca
```

**HuggingFace Hub：**

```yaml
spec:
  dataSource:
    type: huggingface
    path: "tatsu-lab/alpaca"
    format: alpaca
```

**PVC（已有持久卷）：**

```yaml
spec:
  dataSource:
    type: pvc
    path: "training-data-pvc"
    format: sharegpt
```

数据格式支持：

| 格式 | 说明 |
|------|------|
| `alpaca` | Alpaca 格式（instruction / input / output） |
| `sharegpt` | ShareGPT 多轮对话格式 |
| `custom` | 自定义格式 |

### 5.5 质量门控和评估

启用评估和质量门控后，Operator 会在训练完成后自动评估模型，并根据阈值判断是否通过：

```yaml
spec:
  evaluation:
    enabled: true
    dataset: "eval-dataset"           # 评估数据集（可选）
  qualityGate:
    enabled: true
    thresholds:
      minEvalLoss: "0.8"             # 评估损失不超过此值
      maxTrainLoss: "0.5"            # 训练损失不超过此值
```

> **注意**：启用 `qualityGate` 时必须同时启用 `evaluation`，否则 Webhook 验证会拒绝该 CR。

查看质量门控结果：

```bash
kubectl get ftr gemma-lora-v1 -o jsonpath='{.status.qualityGate}' | jq .
```

```json
{
  "passed": true,
  "message": "All quality thresholds met"
}
```

### 5.6 微调后自动部署

将 `deploy.enabled` 设为 `true`，Operator 会在质量门控通过后自动创建 ModelDeployment：

```yaml
spec:
  deploy:
    enabled: true
    canaryWeight: 20    # 以 20% 流量金丝雀方式部署
```

若 `canaryWeight` 大于 0，新模型会以金丝雀方式接入，可渐进调整流量比例后全量切换。若设为 0，则直接全量部署。

查看自动创建的 ModelDeployment：

```bash
kubectl get md
```

```
NAME              ENGINE   REPLICAS   PHASE   AGE
gemma-4-26b       vllm     1          Ready   1d
gemma-4-lora-v1   vllm     1          Ready   10m
```

---

## 6. 运维指南

### 6.1 监控：Printer Columns

Operator 为每个 CRD 配置了 printer columns，使用标准 `kubectl get` 即可快速了解资源状态。

**ModelDeployment：**

```bash
kubectl get md
```

| 列名 | 来源 | 说明 |
|------|------|------|
| Engine | `.status.engine` | 实际使用的推理引擎 |
| Replicas | `.status.readyReplicas` | 就绪副本数 |
| Phase | `.status.phase` | 当前阶段 |
| Age | `.metadata.creationTimestamp` | 创建时间 |

使用 `-o wide` 可展示高优先级列（priority=1）：

```bash
kubectl get md -o wide
```

额外列：

| 列名 | 来源 | 说明 |
|------|------|------|
| Endpoint | `.status.endpoint` | 集群内服务地址 |

**LLMPlatform：**

```bash
kubectl get lp
```

| 列名 | 来源 | 说明 |
|------|------|------|
| Phase | `.status.phase` | 平台整体阶段 |
| Gateway | `.status.components.gateway.phase` | 网关组件状态 |
| Grafana | `.status.components.grafana.phase` | Grafana 组件状态 |
| Age | `.metadata.creationTimestamp` | 创建时间 |

**FineTuneRun：**

```bash
kubectl get ftr
```

| 列名 | 来源 | 说明 |
|------|------|------|
| Base Model | `.spec.baseModel` | 基础模型 |
| Method | `.spec.method` | 微调方法 |
| Phase | `.status.phase` | 当前阶段 |
| Age | `.metadata.creationTimestamp` | 创建时间 |

使用 `-o wide` 额外列：

| 列名 | 来源 | 说明 |
|------|------|------|
| Loss | `.status.metrics.trainLoss` | 训练损失 |
| Duration | `.status.metrics.trainingDuration` | 训练时长 |

### 6.2 状态条件（Conditions）

每个 CRD 都在 `.status.conditions` 中维护结构化的状态条件，遵循 Kubernetes 标准 Condition 格式。

查看 ModelDeployment 的 Conditions：

```bash
kubectl get md gemma-4-26b -o jsonpath='{.status.conditions}' | jq .
```

```json
[
  {
    "type": "Ready",
    "status": "True",
    "reason": "AllReplicasReady",
    "message": "All replicas are ready",
    "lastTransitionTime": "2026-01-15T12:00:00Z",
    "observedGeneration": 1
  }
]
```

查看 LLMPlatform 的 Conditions：

```bash
kubectl get lp kube-llmops -o jsonpath='{.status.conditions}' | jq .
```

```json
[
  {
    "type": "HelmRelease",
    "status": "True",
    "reason": "InstallSucceeded",
    "message": "Helm release installed successfully",
    "lastTransitionTime": "2026-01-15T10:30:00Z",
    "observedGeneration": 1
  }
]
```

查看 FineTuneRun 的 Conditions：

```bash
kubectl get ftr gemma-lora-v1 -o jsonpath='{.status.conditions}' | jq .
```

```json
[
  {
    "type": "WorkflowReady",
    "status": "True",
    "reason": "ArgoPhaseSucceeded",
    "message": "Argo Workflow phase: Succeeded",
    "lastTransitionTime": "2026-01-15T14:15:00Z",
    "observedGeneration": 1
  }
]
```

**Condition 类型汇总：**

| CRD | Condition Type | Reason（成功） | Reason（失败） |
|-----|----------------|----------------|----------------|
| ModelDeployment | `Ready` | `AllReplicasReady` | `ReplicasNotReady` |
| LLMPlatform | `HelmRelease` | `InstallSucceeded` / `UpgradeSucceeded` | `InstallFailed` / `UpgradeFailed` |
| FineTuneRun | `WorkflowReady` | `ArgoPhaseSucceeded` | `ArgoCRDMissing` / `ArgoPhaseError` / `WorkflowNotFound` |

### 6.3 常见问题排查

#### ModelDeployment 一直处于 Deploying 状态

**检查 Pod 状态：**

```bash
kubectl get pods -l kube-llmops/model=gemma-4-26b
```

**检查 Init 容器日志（模型下载）：**

```bash
kubectl logs -l kube-llmops/model=gemma-4-26b -c model-loader
```

**检查推理服务容器日志：**

```bash
kubectl logs -l kube-llmops/model=gemma-4-26b -c model-server
```

**常见原因：**

| 现象 | 原因 | 解决方案 |
|------|------|----------|
| Init 容器 CrashLoopBackOff | MinIO 连接失败 | 确认 LLMPlatform 中 modelStore 配置正确 |
| Init 容器 OOMKilled | 模型过大，loader 内存不足 | 增加模型加载器内存限制 |
| Pod Pending | 无可用 GPU 节点 | 检查 `kubectl describe node` 中的 GPU 资源 |
| model-server CrashLoop | 显存不足 | 降低 `--gpu-memory-utilization` 或 `--max-model-len` |
| model-server 启动超时 | 模型加载慢 | 增加 readinessProbe 的 initialDelaySeconds |

#### LLMPlatform 状态为 Failed

查看错误原因：

```bash
kubectl get lp kube-llmops -o jsonpath='{.status.conditions[0].message}'
```

```bash
# 查看 Helm Release 状态
helm list -A
helm status kube-llmops
```

#### FineTuneRun 状态为 Failed

```bash
# 检查 Condition
kubectl get ftr gemma-lora-v1 -o jsonpath='{.status.conditions}' | jq .

# 如果是 ArgoCRDMissing，需要先安装 Argo Workflows
kubectl apply -n argo -f https://github.com/argoproj/argo-workflows/releases/latest/download/quick-start-minimal.yaml

# 查看 Argo Workflow 日志
WORKFLOW=$(kubectl get ftr gemma-lora-v1 -o jsonpath='{.status.argoWorkflow}')
argo logs $WORKFLOW
```

#### 查看 Operator 自身日志

```bash
kubectl logs -l app.kubernetes.io/name=kube-llmops-operator --tail=100
```

### 6.4 Operator 升级

升级 Operator 版本：

```bash
# 1. 更新 Helm Chart
helm upgrade kube-llmops-operator charts/kube-llmops-operator/

# 2. 验证 CRD 更新
kubectl get crd modeldeployments.llmops.kubellmops.io -o jsonpath='{.metadata.resourceVersion}'

# 3. 确认 Operator Pod 重启完成
kubectl rollout status deployment kube-llmops-operator
```

> **注意**：CRD 升级遵循 Kubernetes 的 CRD 版本策略。Helm 不会自动删除旧 CRD 字段，确保向后兼容。

### 6.5 备份与恢复

**备份所有 CR：**

```bash
# 导出所有 CRD 实例
kubectl get lp -o yaml > backup-llmplatforms.yaml
kubectl get md -o yaml > backup-modeldeployments.yaml
kubectl get ftr -o yaml > backup-finetuneruns.yaml
```

**恢复：**

```bash
# 清除 resourceVersion 和 uid 等元数据后重新 apply
cat backup-modeldeployments.yaml | \
  yq 'del(.items[].metadata.resourceVersion, .items[].metadata.uid, .items[].metadata.creationTimestamp, .items[].status)' | \
  kubectl apply -f -
```

**PVC 备份**（模型缓存数据）：

模型缓存 PVC 的命名规则为 `<modeldeployment-name>-cache`。如果底层存储支持快照：

```bash
kubectl get pvc -l app.kubernetes.io/part-of=kube-llmops
```

---

## 7. 迁移指南

如果你之前通过 Helm Chart 直接部署 kube-llmops（而非使用 Operator），本章介绍如何迁移到 Operator 管理的 CR 模式。

### 7.1 迁移概述

迁移工具 `cmd/migrate/` 会读取现有 Helm Release 的 values，自动生成对应的 LLMPlatform 和 ModelDeployment CR。

**迁移流程：**

```
旧 Helm Release → 迁移工具读取 values → 生成 CRs → 审查 → 卸载旧 Release → Apply CRs
```

### 7.2 使用迁移工具

**步骤 1：运行迁移工具**

```bash
# 语法：go run ./cmd/migrate/ <helm-release-name> [namespace]
go run ./cmd/migrate/ kube-llmops default
```

预期输出：

```
Generated: generated/llmplatform.yaml
Generated: generated/modeldeployment_gemma_4_26b.yaml
Generated: generated/modeldeployment_bge_small_en.yaml

Review the generated CRs, then:
  helm uninstall kube-llmops -n default
  kubectl apply -f generated/
```

工具会在当前目录创建 `generated/` 文件夹，包含所有生成的 CR 文件。

**步骤 2：审查生成的 CR**

```bash
# 查看生成的平台配置
cat generated/llmplatform.yaml

# 查看生成的模型部署
cat generated/modeldeployment_gemma_4_26b.yaml
```

重点检查以下内容：

- `modelStore` 的 endpoint、accessKey、secretKey 是否正确
- 各个模型的 `source`、`replicas`、`resources` 是否匹配
- `engineArgs` 是否完整迁移
- `modules` 的启用状态是否与原来一致

**步骤 3：安装 Operator**

```bash
helm install kube-llmops-operator charts/kube-llmops-operator/
```

**步骤 4：卸载旧 Helm Release**

> ⚠️ **注意**：卸载旧 Release 会删除其管理的 Kubernetes 资源。建议先备份重要的 PVC 和 ConfigMap。

```bash
helm uninstall kube-llmops -n default
```

**步骤 5：应用生成的 CR**

```bash
kubectl apply -f generated/
```

**步骤 6：验证迁移结果**

```bash
kubectl get lp
kubectl get md
```

### 7.3 迁移工具的映射逻辑

迁移工具按以下规则将 Helm values 转换为 CR 字段：

| Helm Values 路径 | CR 字段 |
|-------------------|---------|
| `litellm.enabled` | `LLMPlatform.spec.gateway.enabled` |
| `observability.enabled` | `LLMPlatform.spec.observability.enabled` |
| `global.modules.*` | `LLMPlatform.spec.modules.*` |
| `global.modelStore.*` | `LLMPlatform.spec.modelStore.*` |
| `global.nodePort.*` | `LLMPlatform.spec.nodePort.*` |
| `global.models[*].name` | `ModelDeployment.metadata.name` |
| `global.models[*].source` | `ModelDeployment.spec.source` |
| `global.models[*].replicas` | `ModelDeployment.spec.replicas` |
| `global.models[*].resources` | `ModelDeployment.spec.resources` |
| `global.models[*].engineArgs` | `ModelDeployment.spec.engineArgs` |
| `global.models[*].engine` | `ModelDeployment.spec.engine` |

### 7.4 手动迁移

如果自动迁移工具不能满足需求，可以手动创建 CR：

```bash
# 1. 导出现有 Helm values
helm get values kube-llmops -n default -o yaml > old-values.yaml

# 2. 参照 old-values.yaml 手动编写 LLMPlatform 和 ModelDeployment CR

# 3. 按前述步骤 3-6 完成切换
```

---

## 8. API 参考

### 8.1 ModelDeployment

**全称**：`modeldeployments.llmops.kubellmops.io` · **简称**：`md`

#### Spec 字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `source` | `string` | ✅ | — | HuggingFace 模型 ID 或路径（最少 1 个字符） |
| `engine` | `string` | — | `"auto"` | 推理引擎，可选值：`auto`、`vllm`、`tei`、`llamacpp` |
| `replicas` | `int32` | — | `1` | 推理服务副本数（≥ 0） |
| `resources` | `ModelResources` | — | — | 计算资源配置 |
| `resources.gpu` | `int32` | — | `1` | GPU 数量（0 表示仅 CPU） |
| `resources.memory` | `string` | — | `"16Gi"` | 内存限制 |
| `resources.cpu` | `string` | — | `"4"` | CPU 限制 |
| `accelerator` | `string` | — | `"nvidia"` | GPU 厂商，可选值：`nvidia`、`amd`、`gaudi` |
| `migDevice` | `string` | — | — | NVIDIA MIG 设备资源名（设置后覆盖 gpu 字段） |
| `engineArgs` | `map[string]string` | — | — | 引擎额外命令行参数 |
| `prefixCaching` | `bool` | — | `false` | 启用 vLLM 自动前缀缓存 |
| `spot` | `SpotConfig` | — | — | Spot 实例调度配置 |
| `spot.enabled` | `bool` | — | `false` | 是否启用 Spot 调度 |
| `spot.provider` | `string` | — | — | 云平台，可选值：`aws`、`gcp`、`azure`、`karpenter` |
| `canary` | `CanaryConfig` | — | — | 金丝雀部署配置 |
| `canary.source` | `string` | ✅* | — | 金丝雀模型的 HuggingFace ID |
| `canary.weight` | `int32` | ✅* | — | 金丝雀流量权重（0-100） |
| `canary.resources` | `ModelResources` | — | — | 金丝雀部署的计算资源 |
| `modelStore` | `ModelStoreOverride` | — | — | 覆盖平台级别的模型存储配置 |
| `modelStore.endpoint` | `string` | — | — | MinIO Endpoint |
| `modelStore.bucket` | `string` | — | — | MinIO Bucket 名称 |

*\* canary 配置本身可选，但如果设置了 canary，则 source 和 weight 为必填。*

#### Status 字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `phase` | `string` | 生命周期阶段：`Pending`、`Downloading`、`Deploying`、`Ready`、`Degraded`、`Failed` |
| `engine` | `string` | 自动检测或用户指定后的实际引擎 |
| `endpoint` | `string` | 集群内服务地址（格式：`http://<name>.<ns>.svc.cluster.local:<port>`） |
| `readyReplicas` | `int32` | 就绪副本数 |
| `totalReplicas` | `int32` | 期望副本数 |
| `modelSize` | `string` | 模型文件大小 |
| `conditions` | `[]Condition` | 标准 Kubernetes Conditions |
| `canary` | `CanaryStatus` | 金丝雀部署状态 |
| `canary.phase` | `string` | 金丝雀部署阶段 |
| `canary.endpoint` | `string` | 金丝雀服务地址 |
| `canary.readyReplicas` | `int32` | 金丝雀就绪副本数 |

#### Webhook 验证规则

- `source` 不能为空
- `engine` 必须是 `auto`、`vllm`、`tei`、`llamacpp` 之一（或空字符串）
- `replicas` 必须 ≥ 0
- `resources.gpu` 必须 ≥ 0
- `accelerator` 必须是 `nvidia`、`amd`、`gaudi` 之一（或空字符串）
- 如果设置了 `canary`：`canary.source` 不能为空，`canary.weight` 必须在 0-100 之间

---

### 8.2 LLMPlatform

**全称**：`llmplatforms.llmops.kubellmops.io` · **简称**：`lp`、`llmplatforms`

#### Spec 字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `gateway` | `GatewaySpec` | — | — | AI 网关（LiteLLM）配置 |
| `gateway.enabled` | `bool` | — | `false` | 是否启用网关 |
| `gateway.routing` | `string` | — | — | 路由策略：`simple-shuffle`、`least-busy`、`latency-based-routing`、`usage-based-routing` |
| `gateway.image.repository` | `string` | — | — | 自定义镜像仓库 |
| `gateway.image.tag` | `string` | — | — | 自定义镜像 Tag |
| `gateway.rateLimiting.enabled` | `bool` | — | `false` | 启用请求限流 |
| `gateway.budgetControl.enabled` | `bool` | — | `false` | 启用预算控制 |
| `observability` | `ObservabilitySpec` | — | — | 可观测性配置 |
| `observability.enabled` | `bool` | — | `false` | 启用 Prometheus + Grafana |
| `observability.grafana.adminPassword` | `string` | — | — | Grafana 管理员密码 |
| `observability.grafana.oidc.enabled` | `bool` | — | `false` | 启用 Grafana OIDC 登录 |
| `observability.langfuse.enabled` | `bool` | — | `false` | 启用 Langfuse LLM 可观测性 |
| `logging` | `LoggingSpec` | — | — | 日志配置 |
| `logging.enabled` | `bool` | — | `false` | 启用 Fluent Bit + Loki |
| `modules` | `ModulesSpec` | — | — | 功能模块开关 |
| `modules.rag.enabled` | `bool` | — | `false` | 启用 RAG 模块 |
| `modules.finetune.enabled` | `bool` | — | `false` | 启用微调模块 |
| `modules.security.enabled` | `bool` | — | `false` | 启用安全模块 |
| `modelStore` | `ModelStoreSpec` | — | — | 模型存储（MinIO）配置 |
| `modelStore.enabled` | `bool` | — | `false` | 启用模型存储 |
| `modelStore.endpoint` | `string` | 条件必填 | — | MinIO Endpoint（启用时必填） |
| `modelStore.bucket` | `string` | 条件必填 | — | MinIO Bucket 名称（启用时必填） |
| `modelStore.accessKey` | `string` | — | — | MinIO Access Key |
| `modelStore.secretKey` | `string` | — | — | MinIO Secret Key |
| `modelStore.hfTransferConcurrency` | `int32` | — | `4` | HuggingFace 模型并行下载线程数 |
| `modelStore.image` | `string` | — | — | model-loader 镜像地址 |
| `hfToken` | `string` | — | — | HuggingFace API Token（用于 Gated 模型） |
| `keycloak` | `KeycloakSpec` | — | — | SSO 配置 |
| `keycloak.enabled` | `bool` | — | `false` | 启用 Keycloak |
| `postgresql` | `PostgreSQLSpec` | — | — | 数据库配置 |
| `postgresql.enabled` | `bool` | — | `false` | 启用 PostgreSQL |
| `keda` | `KEDASpec` | — | — | 自动伸缩配置 |
| `keda.enabled` | `bool` | — | `false` | 启用 KEDA |
| `nodePort` | `NodePortSpec` | — | — | NodePort 访问配置 |
| `nodePort.enabled` | `bool` | — | `false` | 启用 NodePort |
| `nodePort.host` | `string` | — | — | 节点 IP 地址 |
| `ingress` | `IngressSpec` | — | — | Ingress 配置 |
| `ingress.enabled` | `bool` | — | `false` | 启用 Ingress |
| `ingress.className` | `string` | — | — | Ingress Class 名称 |
| `ingress.host` | `string` | — | — | 域名 |

#### Status 字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `phase` | `string` | 生命周期阶段：`Pending`、`Installing`、`Ready`、`Upgrading`、`Degraded`、`Failed` |
| `helmRelease` | `string` | 关联的 Helm Release 名称 |
| `helmRevision` | `int` | Helm Release 修订版本号 |
| `components` | `ComponentStatuses` | 各组件健康状态 |
| `components.gateway` | `ComponentStatus` | 网关组件状态 |
| `components.grafana` | `ComponentStatus` | Grafana 组件状态 |
| `components.prometheus` | `ComponentStatus` | Prometheus 组件状态 |
| `components.langfuse` | `ComponentStatus` | Langfuse 组件状态 |
| `components.minio` | `ComponentStatus` | MinIO 组件状态 |
| `components.postgresql` | `ComponentStatus` | PostgreSQL 组件状态 |
| `components.dify` | `ComponentStatus` | Dify 组件状态 |
| `components.milvus` | `ComponentStatus` | Milvus 组件状态 |
| `conditions` | `[]Condition` | 标准 Kubernetes Conditions |

每个 `ComponentStatus` 包含：

| 字段 | 类型 | 说明 |
|------|------|------|
| `phase` | `string` | 组件阶段 |
| `endpoint` | `string` | 组件访问地址 |
| `nodePort` | `int32` | NodePort 端口号（如启用） |

#### Webhook 验证规则

- 当 `modelStore.enabled` 为 `true` 时，`modelStore.endpoint` 和 `modelStore.bucket` 必填
- `gateway.routing` 必须是 `simple-shuffle`、`least-busy`、`latency-based-routing`、`usage-based-routing` 之一（或空字符串）

---

### 8.3 FineTuneRun

**全称**：`finetuneruns.llmops.kubellmops.io` · **简称**：`ftr`

#### Spec 字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `baseModel` | `string` | ✅ | — | 基础模型的 HuggingFace ID（最少 1 个字符） |
| `outputName` | `string` | ✅ | — | 微调后模型制品的名称（最少 1 个字符） |
| `method` | `string` | — | `"lora"` | 微调方法：`lora`、`qlora`、`full` |
| `dataSource` | `DataSourceSpec` | ✅ | — | 训练数据来源 |
| `dataSource.type` | `string` | ✅ | — | 数据源类型：`minio`、`huggingface`、`pvc` |
| `dataSource.path` | `string` | 条件必填 | — | 数据路径（type 为 minio 时必填） |
| `dataSource.format` | `string` | — | `"alpaca"` | 数据格式：`alpaca`、`sharegpt`、`custom` |
| `training` | `TrainingSpec` | — | — | 训练超参数 |
| `training.epochs` | `int32` | — | — | 训练轮次（≥ 0） |
| `training.batchSize` | `int32` | — | — | 批大小（≥ 0） |
| `training.learningRate` | `string` | — | — | 学习率 |
| `training.gradientAccumulationSteps` | `int32` | — | — | 梯度累积步数 |
| `training.warmupRatio` | `string` | — | — | 学习率预热比例 |
| `training.loraRank` | `int32` | — | — | LoRA 秩 |
| `training.loraAlpha` | `int32` | — | — | LoRA Alpha |
| `training.loraTarget` | `string` | — | — | LoRA 目标模块 |
| `resources` | `ModelResources` | — | — | 训练容器资源配置 |
| `resources.gpu` | `int32` | — | `1` | GPU 数量 |
| `resources.memory` | `string` | — | `"16Gi"` | 内存限制 |
| `resources.cpu` | `string` | — | `"4"` | CPU 限制 |
| `evaluation` | `EvaluationSpec` | — | — | 训练后评估配置 |
| `evaluation.enabled` | `bool` | — | `false` | 启用评估 |
| `evaluation.dataset` | `string` | — | — | 评估数据集 |
| `qualityGate` | `QualityGateSpec` | — | — | 质量门控配置 |
| `qualityGate.enabled` | `bool` | — | `false` | 启用质量门控 |
| `qualityGate.thresholds` | `QualityThresholds` | — | — | 阈值配置 |
| `qualityGate.thresholds.minEvalLoss` | `string` | — | — | 评估损失上限 |
| `qualityGate.thresholds.maxTrainLoss` | `string` | — | — | 训练损失上限 |
| `deploy` | `DeploySpec` | — | — | 自动部署配置 |
| `deploy.enabled` | `bool` | — | `false` | 通过质量门控后自动部署 |
| `deploy.canaryWeight` | `int32` | — | `0` | 金丝雀流量权重（0 为全量部署） |

#### Status 字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `phase` | `string` | 生命周期阶段：`Pending`、`DataPreparing`、`Training`、`Evaluating`、`QualityGate`、`Deploying`、`Succeeded`、`Failed` |
| `argoWorkflow` | `string` | 关联的 Argo Workflow 名称 |
| `startTime` | `Time` | 任务开始时间 |
| `completionTime` | `Time` | 任务完成时间 |
| `metrics` | `TrainingMetrics` | 训练指标 |
| `metrics.trainLoss` | `string` | 训练损失 |
| `metrics.evalLoss` | `string` | 评估损失 |
| `metrics.trainingDuration` | `string` | 训练时长 |
| `mlflow` | `MLflowStatus` | MLflow 追踪信息 |
| `mlflow.runId` | `string` | MLflow Run ID |
| `mlflow.experimentName` | `string` | MLflow 实验名称 |
| `mlflow.artifactUri` | `string` | 模型制品 URI |
| `qualityGate` | `QualityGateStatus` | 质量门控结果 |
| `qualityGate.passed` | `bool` | 是否通过 |
| `qualityGate.message` | `string` | 结果说明 |
| `outputModel` | `OutputModelStatus` | 产出模型信息 |
| `outputModel.source` | `string` | 模型来源 |
| `outputModel.modelDeployment` | `string` | 自动创建的 ModelDeployment 名称 |
| `conditions` | `[]Condition` | 标准 Kubernetes Conditions |

#### Webhook 验证规则

- `baseModel` 不能为空
- `outputName` 不能为空
- `method` 必须是 `lora`、`qlora`、`full` 之一（或空字符串）
- `dataSource.type` 必须是 `minio`、`huggingface`、`pvc` 之一
- 当 `dataSource.type` 为 `minio` 时，`dataSource.path` 不能为空
- `training.epochs` 必须 ≥ 0
- `training.batchSize` 必须 ≥ 0
- 当 `qualityGate.enabled` 为 `true` 时，`evaluation.enabled` 也必须为 `true`

---

### 8.4 资源简称速查

| CRD | kubectl 简称 | 示例 |
|-----|-------------|------|
| ModelDeployment | `md` | `kubectl get md` |
| LLMPlatform | `lp` | `kubectl get lp` |
| FineTuneRun | `ftr` | `kubectl get ftr` |

### 8.5 引擎默认配置

| 引擎 | 镜像 | 服务端口 | PVC 大小 |
|------|------|----------|----------|
| `vllm` | `vllm/vllm-openai:latest` | 8000 | 50Gi |
| `tei` | `ghcr.io/huggingface/text-embeddings-inference:cpu-1.6` | 8080 | 10Gi |
| `llamacpp` | `ghcr.io/ggml-org/llama.cpp:server` | 8080 | 30Gi |

### 8.6 GPU 设备资源映射

| accelerator | Kubernetes 资源名 |
|-------------|-------------------|
| `nvidia` | `nvidia.com/gpu` |
| `amd` | `amd.com/gpu` |
| `gaudi` | `habana.ai/gaudi` |
| 自定义 MIG | 由 `migDevice` 字段直接指定 |
