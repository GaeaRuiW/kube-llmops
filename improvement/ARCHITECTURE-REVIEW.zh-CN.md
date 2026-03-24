# kube-llmops 深度架构评审报告

> **评审人**: 首席系统架构师 (15年以上分布式系统、AI Infra、DevOps 经验)
> **日期**: 2026-03-24
> **范围**: 全代码库 — Helm Charts、CI/CD、可观测性、安全、网络、测试
> **评审版本**: v0.2.0 (commit 06130ea)

---

## 总体评价

kube-llmops 是一个野心勃勃的 Kubernetes 原生 LLMOps 伞状 Helm Chart 项目，试图通过一条 `helm install` 命令部署完整的 LLM 基础设施。项目展现出很强的架构愿景（CNCF 对齐、双层网关、模型自动检测），覆盖了令人印象深刻的关注面（推理服务、网关、可观测性、RAG、安全、评估）。

然而，在精心打磨的文档背后，隐藏着一系列**结构性风险**，这些风险会在项目迈向生产使用时成为**致命阻塞**。本次评审识别出 5 个致命级问题、8 个高危隐患，并为每项提供具体的重构建议。

**架构总评分: 6.5 / 10**
- 愿景与文档: 9/10
- 实现成熟度: 5/10
- 生产就绪度: 4/10
- 可扩展性与可维护性: 6/10
- 安全态势: 4/10

---

## 目录

1. [架构设计与合理性](#1-架构设计与合理性)
2. [技术栈与生态](#2-技术栈与生态)
3. [扩展性与高可用](#3-扩展性与高可用)
4. [模块解耦与维护性](#4-模块解耦与维护性)
5. [安全性与部署运维](#5-安全性与部署运维)
6. [致命问题总表](#6-致命问题总表)
7. [重构路线图](#7-重构路线图)
8. [附录：文件级发现](#附录文件级发现)

---

## 1. 架构设计与合理性

### 1.1 模式分析：伞状 Helm Chart

**当前模式**: 单一伞状 Chart (`kube-llmops-stack`) 包含 15 个通过 `file://` 依赖引入的本地子 Chart。

**结论**: 伞状 Chart 模式**完全契合此用例** — 它提供了一键部署体验，并支持协调式升级。这与 Rancher、GitLab、Airflow 的 Helm Chart 使用的是同一模式。此处不存在过度设计。

**然而，实现层面存在致命缺陷：**

#### FATAL-01：单体 PostgreSQL — 单点故障与爆炸半径

整个平台共享**一个 PostgreSQL 实例**（由 `litellm` 子 Chart 部署），承载 4 个数据库：

```
litellm-pg:5432
  ├── litellm      (API 密钥、开支追踪、限流)
  ├── langfuse     (追踪元数据)
  ├── dify         (RAG 工作流、知识库)
  └── dify_plugin  (插件守护进程状态)
```

**为什么这是致命问题：**

1. **爆炸半径**: `dify` 数据库中一条 `ALTER TABLE` 产生的锁可以级联到 `litellm` 的 API 密钥校验，导致**所有推理请求超时**。某个数据库的连接池耗尽会饿死所有其他数据库。

2. **升级耦合**: Dify 1.x → 2.x 迁移可能需要破坏性 Schema 变更。共享实例下，你无法在升级 Dify 的同时不冒 LiteLLM 和 Langfuse 宕机的风险。

3. **资源竞争**: Langfuse 的 ClickHouse 处理 OLAP 负载，但其 PostgreSQL 元数据查询仍与 LiteLLM 的高频 API 密钥查找在同一实例上竞争。

4. **备份/恢复粒度**: 你无法独立备份/恢复 Dify 的数据而不影响 LiteLLM 的运行状态。

**建议：**
```
# 方案 A（推荐）：按逻辑分组拆分独立 StatefulSet
litellm-pg:5432    → 仅 litellm 数据库
langfuse-pg:5432   → 仅 langfuse 数据库
dify-pg:5432       → dify + dify_plugin 数据库

# 方案 B（最低要求）：至少将核心链路与应用数据库分离
operator-pg:5432   → litellm（热路径，要求低延迟）
app-pg:5432        → langfuse, dify, dify_plugin（可容忍较高延迟）
```

每个子 Chart 应声明自己的 PostgreSQL 依赖并拥有独立生命周期。使用 Helm dependency conditions 允许用户指向外部已有数据库。

#### FATAL-02：硬编码服务名导致多实例部署失败

Prometheus 抓取目标包含硬编码的服务名：

```yaml
# charts/observability/templates/prometheus.yaml
scrape_configs:
  - job_name: vllm
    static_configs:
      - targets: ['vllm-qwen2-5-0-5b:8000']   # 硬编码模型名
```

这意味着：
- 添加第二个模型需要手动编辑 Prometheus ConfigMap
- 同一集群内的多 Release 部署（如 staging + production）会产生名称冲突
- 抓取配置未使用 `{{ .Release.Name }}` 前缀

OTel Collector 类似地使用 Kubernetes SD 匹配 `kube_llmops_engine=vllm` 标签，虽然比静态配置好，但仍未按 Release 进行命名空间隔离。

**建议**: 用 Kubernetes 服务发现替换所有静态目标：
```yaml
scrape_configs:
  - job_name: vllm
    kubernetes_sd_configs:
      - role: pod
        namespaces:
          names: ["{{ .Release.Namespace }}"]
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_label_app_kubernetes_io_part_of]
        regex: "{{ include \"kube-llmops.fullname\" . }}"
        action: keep
      - source_labels: [__meta_kubernetes_pod_label_app_kubernetes_io_component]
        regex: vllm
        action: keep
```

#### HIGH-01：架构文档与实际实现严重脱节

ARCHITECTURE.md 描述了**双层网关**（LiteLLM → Envoy AI Gateway + IGW → vLLM）以及 SGLang、Harbor 模型仓库、JupyterHub、MLflow、LLaMA-Factory、Kustomize overlays 等组件。但实际实现中：

- 无 SGLang 子 Chart（仅有 `llamacpp`）
- Envoy Gateway 模板存在但实质上是死代码（需要外部 CRD）
- 无 JupyterHub、MLflow、LLaMA-Factory 或 Kustomize overlays
- Harbor 子 Chart 是空壳/占位符
- 无 ArgoCD ApplicationSet

文档制造了代码库无法兑现的预期。用户按照 ARCHITECTURE.md 操作时会因功能不存在而困惑。

**建议**: (a) 在 ARCHITECTURE.md 中明确标注未实现功能为 "Planned" 并注明目标版本，或 (b) 拆分为 `ARCHITECTURE.md`（当前状态）和 `ARCHITECTURE-VISION.md`（未来愿景）。当前文档混淆了两者。

### 1.2 部署 Profile 策略

4 个 Profile 体系（`ci`、`minimal`、`standard`、`production`）**设计精良**：

| Profile | GPU | 组件 | 适用场景 |
|---------|-----|------|----------|
| ci | 0 | LiteLLM + Prometheus | CI 测试 |
| minimal | 1 | 全栈 | 开发 |
| standard | 4-8 | 多模型 | 团队 |
| production | 16+ | HA + 安全 | 企业 |

这种渐进式复杂度方案非常优秀，正确遵循了"智能默认、完全可覆盖"原则。

**小问题**: `values-single-node.yaml` 存在但未在 Chart.yaml 或文档中作为正式 Profile 列出。它与 `values-minimal.yaml` 看起来是重复的，仅有微小差异。建议合并或明确文档化。

### 1.3 Model Resolver — 设计优秀但集成未完成

Model Resolver（格式自动检测 → 引擎选择）架构设计合理：

```
用户指定模型 → init-container 检测格式 → 选择 vLLM/llama.cpp/TEI
```

Resolver 有良好的单测覆盖率（28 个测试）。然而，resolver 镜像**从未**作为 init-container 实际用于 vLLM 部署模板中。`charts/vllm/templates/deployment.yaml` 中的 `model-loader` init-container 下载模型但不调用 resolver。引擎选择在 Helm 模板渲染阶段确定，而非运行时。

**影响**: 所谓的"自动检测"实际上是手动的 — 用户必须在 values.yaml 中设置 `engine: vllm` 或 `engine: tei`。Resolver 代码存在但未接入管线。

**建议**: 完成集成或从文档中移除自动检测的声明。

---

## 2. 技术栈与生态

### 2.1 技术选型评估

| 组件 | 选型 | 评估 |
|------|------|------|
| 推理引擎 | vLLM v0.9.2 | 优秀 — 业界标准，活跃开发 |
| 向量嵌入 | TEI | 良好 — HuggingFace 专用嵌入服务器 |
| AI 网关 | LiteLLM v1.82.3 | 良好 — 广泛模型支持，但版本迭代过快导致升级风险 |
| 追踪 | Langfuse v3.161.0 | 良好 — 为 LLM 追踪量身定制，优于 Jaeger |
| 指标 | Prometheus v3.9.1 | 扎实 — 业界标准 |
| 仪表盘 | Grafana 12.4.1 | 标准 |
| 日志采集 | Fluent Bit 4.2.3.1 | CNCF 毕业项目，正确选择 |
| 日志存储 | Loki 3.6.7 | 良好 — Grafana 生态集成 |
| RAG 平台 | Dify 1.13.2 | **高风险** — 见下文 |
| SSO | Keycloak 26.5.6 | CNCF 孵化项目，扎实选择 |
| 对象存储 | MinIO | 自建 S3 的标准选型 |
| 编排 | Kubernetes + Helm | 正确 |

#### HIGH-02：Dify 作为 RAG 平台 — 高耦合、破坏性变更风险

Dify 是一个快速迭代的平台，频繁发布破坏性变更。kube-llmops 的 Dify 子 Chart 存在深度耦合：

1. **5 个独立 Deployment**（API、Worker、Web、Plugin Daemon、Redis）— 几乎是平台中的子平台
2. **Plugin Daemon** 需要独立的数据库、Redis 和 PVC，还有离线 `uv sync` 的变通方案
3. **HttpOnly Cookie** 约束迫使 Ingress 模板增加了路径路由的复杂性（232 行 ingress.yaml 中有 Dify 特定的特殊处理）
4. **Setup Job** 硬编码了管理员账户创建、插件安装和模型提供者配置的 API 调用

当 Dify 发布破坏性变更（如 API 路径变更、插件系统重写、认证机制变更）时，setup job、ingress 规则和环境变量都需要同步更新。这在 Dify 0.x → 1.x 升级时已经发生过，未来大概率还会发生。

**建议：**
- 将 Dify 抽象为接口 — 定义"RAG Provider"契约，支持可配置后端（Dify、Langflow、自定义）
- 将 Dify 特定的 Ingress 规则移入 Dify 子 Chart 而非父级 Ingress 模板
- 添加版本锁定并在文档中明确 Dify API 兼容性矩阵
- 考虑将 Dify 定位为**可选插件**而非核心组件

#### HIGH-03：LiteLLM 版本锁定风险

LiteLLM（`main-v1.82.3-stable`）每周多次发布，配置格式、Provider 行为和数据库 Schema 经常发生破坏性变更。当前 Chart 锁定了特定版本（正确），但：

1. 未记录 LiteLLM 升级的数据库迁移策略
2. LiteLLM 的 PostgreSQL Schema 可能跨版本变化 — 共享数据库使这变得更加危险
3. `litellm_config` ConfigMap 格式与 LiteLLM 内部配置解析器紧密耦合

**建议**: 在 Chart.yaml 的 `appVersion` 中锁定 LiteLLM 版本，记录升级流程，在 CI 中测试至少 2 个 LiteLLM 版本的配置兼容性。

### 2.2 欠缺的技术项

#### 缺失 External Secrets Operator 集成

所有密钥（PostgreSQL 密码、API 密钥、OIDC 客户端密钥）以明文存储在 `values.yaml` 中。ARCHITECTURE.md 提到了"External Secrets Operator"但零实现。对于任何生产用途，这都是阻塞项。

#### 缺失 Service Mesh

Pod 间通信未加密。架构文档提到 Cilium（CNCF 毕业项目）用于网络策略，但未利用其 mTLS 能力或任何 Service Mesh。对于处理敏感提示词的环境，这是合规隐患。

---

## 3. 扩展性与高可用

### 3.1 瓶颈分析

#### FATAL-03：所有有状态组件均为单副本，无高可用方案

| 组件 | 生产副本数 | HA 支持 | 故障影响 |
|------|-----------|---------|----------|
| PostgreSQL | **1** | 无 | **整个平台瘫痪** — LiteLLM、Langfuse、Dify 全部宕机 |
| Keycloak | **1** | 无 | 无法新登录，SSO Token 刷新失败 |
| Prometheus | **1** | 无 | 告警失明，仪表盘空白 |
| Grafana | **1** | 无 | 无法访问仪表盘 |
| Loki | **1** | 无 | 日志摄入停止 |
| ClickHouse | **1** | 无 | Langfuse 追踪数据摄入停止 |
| MinIO | **1** (StatefulSet) | 无 | Blob 存储不可用 |

即使在 `values-production.yaml` 中，**以上组件均无 HA 配置**。LiteLLM 和 Langfuse 扩展到了 2 副本（正确），但它们共享的 PostgreSQL 仍然是单副本。

**这意味着一次 PostgreSQL Pod 重启就会导致整个平台瘫痪。**

**建议（分阶段）：**

**第 1 阶段**（最小化 HA）：
- PostgreSQL: 使用支持主从复制的 Helm 子 Chart（如 Bitnami PostgreSQL 的 `replication.enabled=true`，或 CloudNativePG Operator）
- 为所有副本数 ≥2 的 Deployment 添加 PodDisruptionBudget

**第 2 阶段**（完整 HA）：
- Prometheus: 切换到 Thanos 或 VictoriaMetrics 实现 HA 指标存储
- Loki: 使用 Loki 的 `simple-scalable` 部署模式
- MinIO: 启用分布式模式（最少 4 节点）
- ClickHouse: 使用 ClickHouse Keeper 实现复制

**第 3 阶段**（外部优先）：
- 提供 values 覆盖以指向外部托管数据库（RDS、Cloud SQL 等）
- 记录并测试外部数据库配置

#### FATAL-04：全局缺失 PodDisruptionBudget

整个代码库中 PDB 数量为零。在 Kubernetes 节点排空（滚动更新、Spot 实例回收）期间，**一个组件的所有副本可能被同时驱逐**。

对于生产平台，这是不可接受的。对运行 PostgreSQL Pod 的节点执行 `kubectl drain` 将导致完全中断且无任何保护。

**建议**: 为每个组件添加 PDB：
```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: {{ include "litellm.fullname" . }}
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: litellm
```

### 3.2 GPU 调度缺陷

#### HIGH-04：缺失 GPU 拓扑感知

vLLM 部署使用简单的 `nvidia.com/gpu: N` 资源请求。对于使用张量并行的多 GPU 模型，这无法保证分配到 NVLink 互联的 GPU。跨 PCIe 总线的两块 GPU 的互联带宽差距巨大。

**建议：**
- 使用 NVIDIA 的 GPU 拓扑感知调度（Node Feature Discovery + TopologyManager）
- 为张量并行模型记录 GPU 拓扑要求
- 添加 nodeSelector/affinity 规则以匹配 GPU 拓扑

#### HIGH-05：模型加载冷启动问题未解决

vLLM 加载大模型（70B+）需要 5-15 分钟。当前探针允许最多约 14 分钟（liveness: 240s 初始延迟 + 20 次失败 x 30s）。但：

1. KEDA 自动扩容可能触发一个新 vLLM 副本，该副本 10+ 分钟内无法服务流量
2. 无预加载模型策略（每次 Pod 启动都从 HuggingFace Hub 或 S3 下载模型）
3. Fluid/Alluxio 缓存仅有模板 — 需要外部安装 Fluid Operator 且无安装文档

**建议：**
- 通过将模型权重预拉取到节点本地存储来实现模型预热（DaemonSet 或 NodeLocal PV）
- 添加 readiness gate 确保 KEDA 不将启动中的 Pod 计为就绪
- 记录 Fluid Operator 安装流程并提供自包含的缓存替代方案（如 hostPath + 模型预拉取 Job）

### 3.3 扩展上限分析

| 组件 | 当前扩展方式 | 上限 | 瓶颈 |
|------|-------------|------|------|
| vLLM | KEDA（副本） | GPU 数量 | 节点供给（无 Karpenter） |
| LiteLLM | 手动副本 | PostgreSQL 连接数 | 共享 PG 实例 |
| Prometheus | 无（单实例） | ~200 万活跃时序 | 无远端存储、无分片 |
| Langfuse | 手动副本 | ClickHouse IOPS | 单 ClickHouse |
| Loki | 无（单实例） | ~10MB/s 摄入 | 单实例、文件系统存储 |

对于 50+ 工程师持续发送提示词的团队，单实例瓶颈会在数周内暴露。

---

## 4. 模块解耦与维护性

### 4.1 依赖图分析

```
父 Chart (ingress.yaml)
  ├── 知道：LiteLLM, Grafana, Langfuse, Keycloak, Dify, Prometheus, MinIO
  │   (7 个服务硬编码在父级 Ingress 模板中)
  │
  ├── litellm/
  │   ├── 拥有：PostgreSQL（被 langfuse, dify, dify_plugin 共享）
  │   ├── 知道：vLLM 服务名, TEI 服务名, Langfuse URL
  │   └── ConfigMap：硬编码模型名到 litellm_config
  │
  ├── observability/
  │   ├── Prometheus：硬编码 vLLM 模型服务名
  │   ├── Grafana：知道 Keycloak OIDC URL、Prometheus URL、Loki URL
  │   └── OTel：Kubernetes SD 使用硬编码标签选择器
  │
  ├── langfuse/
  │   ├── 依赖：litellm-pg (PostgreSQL), MinIO (S3)
  │   └── 知道：Keycloak OIDC URL
  │
  ├── dify/
  │   ├── 依赖：litellm-pg (PostgreSQL), 自有 Redis, MinIO
  │   ├── Setup Job：知道 LiteLLM API 端点、模型名
  │   └── Plugin Daemon：独立数据库、PVC、Redis 依赖
  │
  ├── keycloak/
  │   ├── Realm ConfigMap：硬编码 grafana, langfuse, minio, litellm 的客户端名
  │   └── 无其他 Chart 依赖
  │
  └── security/
      ├── NetworkPolicy：硬编码服务名 (litellm, otel-collector, grafana)
      └── LLM-Guard：独立
```

#### HIGH-06：跨 Chart 依赖混乱

依赖图揭示了紧耦合：

1. **父 Chart 知道太多**: 父 Chart 中 232 行的 `ingress.yaml` 包含 7 个子 Chart 的服务特定路由规则。每添加一个新子 Chart 都需要修改父级 Ingress。

2. **litellm 持有共享基础设施**: PostgreSQL 由 `litellm` 子 Chart 部署但被 `langfuse` 和 `dify` 使用。这造成了隐式的部署顺序依赖 — `litellm` 必须先于 `langfuse` 或 `dify` 部署。

3. **循环知识**: `observability` 知道 `vllm` 服务名；`vllm` 不知道 `observability`。`keycloak` 为 `grafana` 和 `langfuse` 创建 OIDC 客户端；这些 Chart 反向引用 `keycloak` URL。这种循环引用使独立测试成为不可能。

**建议**: 引入**分层基础设施架构**：

```
Layer 0: 基础设施（独立，优先部署）
  ├── postgresql/    (单独 Chart，创建所有数据库)
  ├── redis/         (单独 Chart，共享 Redis)
  ├── minio/         (对象存储)
  └── keycloak/      (SSO 提供者)

Layer 1: 核心服务（仅依赖 Layer 0）
  ├── vllm/          (模型推理)
  ├── tei/           (向量嵌入)
  └── litellm/       (网关，依赖 postgresql)

Layer 2: 应用服务（依赖 Layer 0-1）
  ├── langfuse/      (依赖 postgresql, minio)
  ├── dify/          (依赖 postgresql, redis, litellm)
  └── observability/ (无依赖，通过 SD 抓取)

Layer 3: 平台服务（依赖 Layer 0-2）
  ├── rag-eval/      (依赖 litellm, langfuse)
  └── security/      (无依赖，叠加 NetworkPolicy)
```

每一层应可独立部署和测试。

### 4.2 模板质量与 Helm 最佳实践

#### HIGH-07：Helm 标签标准不统一

部分子 Chart 一致使用 `app.kubernetes.io/name` 标签，其他则使用自定义标签如 `app: vllm` 或 `app.kubernetes.io/part-of: kube-llmops`。这种不一致会导致 NetworkPolicy 选择器和 Prometheus 服务发现失效。

发现的不一致示例：
- vLLM: 使用 `app.kubernetes.io/name: vllm`, `app.kubernetes.io/component: {{ $model.name }}`
- LiteLLM: 使用 `app.kubernetes.io/name: litellm`
- Prometheus 抓取: 匹配 `kube_llmops_engine: vllm`（自定义标签）
- NetworkPolicy: 匹配 `app.kubernetes.io/name: litellm` 和 `app.kubernetes.io/name: otel-collector`

**建议**: 在 `_helpers.tpl` 中定义标签标准并在所有子 Chart 中强制执行：
```yaml
# 所有资源的标准标签
app.kubernetes.io/name: {{ .Chart.Name }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion }}
app.kubernetes.io/component: <组件特定>
app.kubernetes.io/part-of: kube-llmops
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ include "kube-llmops.chart" . }}
```

### 4.3 开发者入门成本

**正面：**
- AGENTS.md 提供了优秀的 AI 助手知识库
- CONTRIBUTING.md 覆盖了开发环境搭建
- 4 个部署 Profile 降低了配置负担
- Makefile 提供了标准化的构建目标

**负面：**
- `.tgz` 缓存陷阱（虽已文档化但仍是坑 — 开发者会忘记在编辑子 Chart 后运行 `helm dependency update`）
- 无本地开发环境（如 Tilt、Skaffold 或 DevSpace）
- E2E 测试需要 GPU 节点（Dify/RAG 测试无纯 CPU 路径）
- 无架构决策记录（ADR）— 选型的"为什么"散落在 ARCHITECTURE.md 和 commit message 中

**建议：**
- 添加 `make dev` 目标，使用 Tilt 或 Skaffold 实现热重载开发
- 创建 `docs/adr/` 目录，为重大决策提供 ADR 模板
- 添加 pre-commit hook，在子 Chart 模板变更时自动运行 `helm dependency update`

---

## 5. 安全性与部署运维

### 5.1 安全评估

#### FATAL-05：全局硬编码明文凭据

代码库在多个 values 文件中包含**明文默认凭据**：

| 凭据 | 位置 | 默认值 |
|------|------|--------|
| PostgreSQL 密码 | `values-*.yaml` | `llmops-pg-dev-pw` |
| LiteLLM 主密钥 | `values-*.yaml` | `sk-kube-llmops-dev` / `sk-kube-llmops-default` |
| Keycloak 管理员密码 | `values-*.yaml` | `admin123!` |
| Grafana 管理员密码 | `values-*.yaml` | `admin123!` |
| Langfuse 密钥 | `values-*.yaml` | 各种硬编码值 |
| MinIO 凭据 | `values-*.yaml` | `minioadmin/minioadmin` |
| LLM-Guard Token | `values.yaml` | `llm-guard-kube-llmops` |
| Dify 密钥 | `values-*.yaml` | 硬编码 |
| Langfuse 加密密钥 | `values-*.yaml` | 硬编码 |

**为什么这是致命问题**: 用户会使用默认值部署。`helm install` 文档未醒目提示更改凭据。一旦部署，这些凭据存在于 ConfigMap 和 Secret 中并跨升级持久化。凭据轮换需要跨 5+ 组件的协调变更。

**建议（立即）：**
1. **安装时生成随机默认值**，使用 Helm 的 `randAlphaNum` 函数：
   ```yaml
   password: {{ .Values.postgresql.password | default (randAlphaNum 24) }}
   ```
2. 在 `NOTES.txt` 中添加醒目警告，显示所有默认凭据及修改说明
3. 在快速开始文档中添加 `--set` 覆盖示例
4. 实现"安全审计" Helm Hook，检查默认凭据并发出警告/阻断

**建议（中期）：**
5. 集成 External Secrets Operator (ESO)，提供 AWS Secrets Manager 和 HashiCorp Vault 的参考实现
6. 在所有子 Chart 中添加 `existingSecret` 字段，允许用户预创建 Kubernetes Secret

#### HIGH-08：缺失出站 NetworkPolicy

`security` 子 Chart 仅实现了入站 NetworkPolicy。Pod 可以自由连接外网。这意味着：

1. 被攻陷的 LLM-Guard Pod 可以外泄数据
2. 被攻陷的 vLLM Pod 可被用于挖矿
3. 对从互联网下载资源的 init container 中的供应链攻击无防护

**建议**: 添加默认拒绝出站策略并配置显式放行规则：
```yaml
# 允许 vLLM 仅访问 HuggingFace Hub（模型下载）和内部服务
egress:
  - to:
    - podSelector:
        matchLabels:
          app.kubernetes.io/part-of: kube-llmops
  - to:
    - ipBlock:
        cidr: 0.0.0.0/0
    ports:
      - port: 443  # 仅 HTTPS 用于模型下载
```

### 5.2 CI/CD 评估

**优势：**
- 7 个 GitHub Actions 工作流覆盖 lint、test、build、E2E、release
- Trivy 漏洞扫描（CRITICAL 级别）
- TruffleHog 密钥扫描
- 多 Profile 模板渲染测试（6 个 Profile）
- Model Resolver 28 个单元测试
- 全面的 E2E 测试套件（30+ 断言）

**关键缺口：**

| 缺口 | 严重性 | 影响 |
|------|--------|------|
| `chart-install-test` 使用 `continue-on-error: true` | 高 | 集群安装失败 CI 仍通过 |
| 无 Helm Schema 验证 (`values.schema.json`) | 高 | 无效值仅在部署时才被捕获 |
| 无 SBOM 生成 | 中 | 供应链可见性缺失 |
| 无镜像签名 (cosign/sigstore) | 中 | 镜像来源未验证 |
| 无数据库迁移测试 | 高 | LiteLLM/Langfuse 升级可能破坏 Schema |
| 许可证检查仅为警告 | 中 | AGPL 依赖可能混入 |
| 无性能/压力测试 | 中 | 扩展性声明未验证 |

#### CI 管线改进建议

```
当前：
  PR → Lint + Build + Test（尽力而为）→ 合并

建议：
  PR → Lint → 模板渲染（全部 6 个 Profile）
     → Schema 验证 (values.schema.json)
     → 单元测试 (pytest)
     → kind 集群安装（必须通过，非尽力而为）
     → 冒烟测试（健康检查）
     → E2E (Playwright, 每周或按需)

  Main → Build + Trivy + SBOM → 推送 (GHCR)
       → 签名 (cosign)

  Tag → Release (chart + 镜像 + SBOM + 签名)
```

### 5.3 部署运维缺口

#### 缺失备份/恢复流程

尽管 ARCHITECTURE.md 提到了备份/恢复脚本，但 `scripts/backup.sh` 和 `scripts/restore.sh` 实际上不存在。对于拥有 4+ 数据库的平台，这是关键缺失。

**建议**: 实现自动化备份：
```bash
# 最小可行备份
kubectl exec litellm-pg-0 -- pg_dumpall > backup-$(date +%Y%m%d).sql

# 更优方案：使用 Kubernetes 原生备份方案
# - Velero（CNCF 项目）全集群备份
# - pgBackRest 用于 PostgreSQL 专项备份
```

#### 缺失升级手册

项目缺少文档化的升级流程。以下问题未得到解答：
- 从 v0.1.0 升级到 v0.2.0 会发生什么？
- 是否有数据库迁移？
- 回滚流程是什么？
- values.yaml 在版本间是否有破坏性变更？

Quality Gate（pre-upgrade hook）是良好的开端，但仅检查 RAG 质量指标，不检查基础设施健康状态。

**建议**: 创建 `docs/upgrade-guide.md`，包含版本特定的迁移说明，并添加 pre-upgrade Job 验证数据库 Schema 兼容性。

---

## 6. 致命问题总表

| 编号 | 问题 | 影响 | 修复成本 |
|------|------|------|----------|
| **FATAL-01** | 单体共享 PostgreSQL | 单点故障、升级耦合、全平台瘫痪风险 | 高（重构数据库层） |
| **FATAL-02** | Prometheus/OTel 硬编码服务名 | 多模型、多 Release 部署失败 | 中（模板重构） |
| **FATAL-03** | 所有有状态组件无 HA | 任何 Pod 重启 = 平台瘫痪 | 高（添加复制） |
| **FATAL-04** | 全局无 PodDisruptionBudget | 节点排空 = 不可控中断 | 低（添加 PDB 模板） |
| **FATAL-05** | 明文硬编码凭据 | 安全漏洞风险、凭据轮换不可行 | 中（随机生成默认值 + ESO） |

**高危问题**: 共 8 项（详见上文各节）

---

## 7. 重构路线图

### 第 1 阶段：紧急修复（1-2 周）

**目标**: 使平台在生产环境中可存活。

| 任务 | 涉及文件 | 优先级 |
|------|---------|--------|
| 为所有 Deployment 添加 PodDisruptionBudget | 所有子 Chart 模板 | P0 |
| 用 Kubernetes SD 替换硬编码 Prometheus 目标 | `observability/templates/prometheus.yaml` | P0 |
| 使用 `randAlphaNum` 生成随机默认凭据 | 所有 `values.yaml` 文件 | P0 |
| 在所有子 Chart 中添加 `existingSecret` 支持 | 所有子 Chart `values.yaml` 和模板 | P0 |
| 统一所有子 Chart 的 Helm 标签标准 | 所有 `_helpers.tpl` 和模板 | P1 |
| 将 `chart-install-test` 设为必须通过（移除 `continue-on-error`） | `.github/workflows/test.yaml` | P1 |

### 第 2 阶段：架构改进（2-4 周）

**目标**: 解耦组件，实现独立生命周期。

| 任务 | 涉及文件 | 优先级 |
|------|---------|--------|
| 按逻辑分组拆分 PostgreSQL 为独立实例 | `litellm/`, `langfuse/`, `dify/` 子 Chart | P0 |
| 将 Dify 特定 Ingress 规则移入 Dify 子 Chart | `templates/ingress.yaml`, `dify/templates/` | P1 |
| 添加 `values.schema.json` 进行校验 | `charts/kube-llmops-stack/` | P1 |
| 实现分层依赖排序（见第 4.1 节） | `Chart.yaml`、所有子 Chart | P1 |
| 添加出站 NetworkPolicy | `security/templates/` | P2 |
| 创建升级手册和迁移框架 | `docs/upgrade-guide.md` | P2 |

### 第 3 阶段：生产强化（1-2 个月）

**目标**: 企业级 HA 和运维能力。

| 任务 | 涉及文件 | 优先级 |
|------|---------|--------|
| PostgreSQL HA（CloudNativePG 或 Bitnami 复制） | 新子 Chart 或外部依赖 | P0 |
| External Secrets Operator 集成 | 新模板、文档 | P1 |
| 备份/恢复自动化（Velero 或 pg_dump CronJob） | 新模板、脚本 | P1 |
| Prometheus HA（VictoriaMetrics 或 Thanos Sidecar） | `observability/` 子 Chart | P2 |
| 添加 Pod 间 mTLS（Cilium 或 Service Mesh） | 基础设施层 | P2 |
| 性能/压力测试套件 | `tests/load/` | P2 |
| CI 中添加 SBOM 生成 + 镜像签名 | `.github/workflows/` | P2 |

### 第 4 阶段：生态成熟（2-3 个月）

**目标**: 开发者体验与生态集成。

| 任务 | 涉及文件 | 优先级 |
|------|---------|--------|
| Tilt/Skaffold 本地开发环境 | `Tiltfile` 或 `skaffold.yaml` | P1 |
| 完成 Model Resolver 集成 | `vllm/templates/deployment.yaml` | P1 |
| ArgoCD ApplicationSet 用于 GitOps | `manifests/argocd/` | P2 |
| 多集群支持文档 | `docs/` | P2 |
| 云厂商 Terraform 模块 | `terraform/` | P3 |
| Kubernetes Operator（长期） | 新 Go 项目 | P3 |

---

## 附录：文件级发现

### 关键路径文件（审查优先级）

| 文件 | 行数 | 风险 | 发现 |
|------|------|------|------|
| `charts/kube-llmops-stack/templates/ingress.yaml` | 232 | 高 | 单体化，知道 7 个子 Chart，Dify 特殊处理 |
| `charts/litellm/templates/postgresql.yaml` | ~150 | 致命 | 共享 PG，init 脚本创建 4 个数据库 |
| `charts/observability/templates/prometheus.yaml` | 299 | 致命 | 硬编码 vLLM 目标，无服务发现 |
| `charts/dify/templates/` | 500+ | 高 | 5 个 Deployment，复杂 setup job，离线 PVC 变通方案 |
| `charts/keycloak/templates/realm-configmap.yaml` | 63 | 中 | 硬编码客户端名、默认密码 |
| `charts/security/templates/network-policies.yaml` | 103 | 高 | 仅入站、无出站、硬编码服务名 |

### 正面亮点

| 文件 | 发现 |
|------|------|
| `images/model-resolver/` | 结构良好的 Python 代码，28 个测试，关注点分离优秀 |
| `charts/rag-eval/` | Quality Gate 概念出色 — pre-upgrade RAG 质量验证 |
| `charts/observability/dashboards/` | 4 个为 LLM 特定指标量身定制的仪表盘 |
| `AGENTS.md` | 出色的 AI 助手知识库，捕获了关键陷阱 |
| `Makefile` | 清晰、全面的构建目标 |
| `tests/e2e/test_full_e2e.py` | 30+ 断言覆盖 9 个测试类别 — 覆盖面令人印象深刻 |

---

## 最终评定

kube-llmops 是一个**具有远见的项目**，正确识别了对开箱即用 Kubernetes LLMOps 平台的需求。架构文档、CNCF 对齐度和技术选型展现了对 AI 基础设施领域的深刻理解。

然而，实现**尚未追上愿景**。识别出的五个致命问题 — 共享 PostgreSQL、硬编码服务名、无 HA、无 PDB、明文凭据 — 每一个都能独立导致生产事故。叠加在一起，它们使该平台在不经过重大整改的情况下不适合生产负载。

好消息是：大多数修复范围明确，可以渐进式实施。项目的模块化子 Chart 结构使得逐个组件重构成为可能，无需整体重写。**架构根基是正确的 — 需要强化的是实现细节。**

**优先行动**: 从第 1 阶段开始（PDB、服务发现、凭据生成）— 这些是高影响、低成本的修复，能立即提升生产就绪度。

---

*本评审基于代码库和文档的静态分析。结合压力测试的线上部署评审可能会暴露更多运维层面的问题。*
