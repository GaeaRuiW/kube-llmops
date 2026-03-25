# kube-llmops：承诺 vs 实现 — Infra 能力差距清单

> **审计日期**: 2026-03-24（Phase 4 commit c1e91d4 后更新）
> **审计范围**: README.md, ARCHITECTURE.md, PLAN.md, RAG-PLAN.md, RAG-TODO.md, values.yaml, 全部 Helm 模板代码
> **审计方法**: 文档声明逐条 ↔ 代码 `grep` + 模板文件逐行核对

---

## 一句话结论

文档中声明了约 **150+ 项 Infra 能力**，其中约 **75 项已实现且可用**，约 **30 项有模板但功能不完整或需要外部依赖**，约 **45 项仅存在于文档描述中**。

> **Phase 4 更新**：commit c1e91d4 新增了 LightRAG 知识图谱、Presidio PII 脱敏、Milvus 生产化（含 Grafana Dashboard）、多租户 Namespace 隔离，消除了 5 项此前标注为"未实现"的差距。

---

## 分类说明

| 标签 | 含义 |
|------|------|
| ✅ **已实现** | 代码完整、默认启用或可一键启用、有测试覆盖 |
| ⚠️ **部分实现** | 模板/代码存在但：功能不完整、需要外部 Operator、未经集成测试、或默认禁用且文档不充分 |
| ❌ **未实现** | 文档中描述了但代码中不存在（仅 `.gitkeep`、注释、或 ARCHITECTURE.md 愿景描述）|
| 🗓️ **明确标注为 Roadmap** | README Roadmap 中标为未来版本，读者不会误解为已实现 |

---

## 第一类：核心承诺 — 读者读完 README 会认为"现在就能用"

> 来源：README.md 功能列表、Features 对比表、Quick Start

| # | 承诺 | 来源 | 实际状态 | 差距说明 |
|---|------|------|---------|---------|
| 1 | **推理引擎自动选择**（GPTQ→vLLM, GGUF→llama.cpp） | README L14, L147 | ⚠️ 部分 | Model Resolver 代码存在（28 个单测），但 **未集成到 Helm init-container**。用户必须在 values.yaml 手动指定 `engine: vllm`。README 对比表写的是 "Yes" |
| 2 | **KEDA 自动扩缩**（队列深度 + TTFT） | README L18, L152 | ⚠️ 部分 | ScaledObject 模板存在，但：①需要预装 KEDA Operator（无安装文档）②默认 disabled ③未经实际负载验证 |
| 3 | **GPU 监控 (DCGM)** | README L16, L151 | ⚠️ 部分 | DaemonSet + Dashboard 存在且功能完整，但 **WSL2 环境不可用**（Known Issue 已标注）。需要宿主机预装 NVIDIA 驱动 + DCGM |
| 4 | **全栈一键部署** | README L22-24 | ⚠️ 部分 | `helm repo add` URL 写的是 "v0.1.0 发布后可用"——但 v0.2.0 已发布。暗示 **Helm Repo 可能尚未就绪**，用户只能用本地 chart 目录安装 |
| 5 | **LiteLLM 速率限制 / 预算控制** | README L15 | ⚠️ 部分 | LiteLLM 本身支持，但平台层 **无 Grafana 成本/用量 Dashboard**，无法可视化各团队消耗 |

---

## 第二类：ARCHITECTURE.md 描述为架构组成但代码不存在

> ARCHITECTURE.md 是 1085 行的详细技术设计文档。读者很容易将其中的描述理解为"已实现"或"即将实现"，但许多内容实际是愿景。

| # | 承诺 | ARCHITECTURE.md 位置 | 实际状态 | 差距说明 |
|---|------|---------------------|---------|---------|
| 6 | **SGLang 推理引擎** | L241, L357-365 | ❌ 未实现 | values.yaml 注释中列为 `engine: sglang` 选项，但无 subchart、无模板、无 engine_map 映射 |
| 7 | **双层网关 Tier-2**（Envoy AI Gateway + IGW） | L380-450 | ⚠️ 部分 | 模板文件存在 `envoy-gateway.yaml`，但：①需外部 Envoy Gateway Controller ②InferenceModel CRD 是 placeholder ③默认 disabled ④无安装文档 |
| 8 | **KV-cache-aware 路由 / 前缀缓存调度** | L416-450 | ❌ 未实现 | 仅在 ARCHITECTURE.md 中描述了 InferencePool/InferenceModel CRD 概念，代码中是注释占位符 |
| 9 | **ArgoCD Sync Waves** | L278-294 | ❌ 未实现 | `manifests/argocd/` 目录仅有 `.gitkeep`，无 Application / ApplicationSet 清单 |
| 10 | **JupyterHub GPU 笔记本环境** | L796-803 | ❌ 未实现 | 零代码 |
| 11 | **LLaMA-Factory 微调 Job 模板** | L796-803 | ❌ 未实现 | 零代码 |
| 12 | **MLflow 实验追踪** | L796-803 | ❌ 未实现 | 零代码 |
| 13 | **Label Studio 数据标注** | L796-803 | ❌ 未实现 | 零代码 |
| 14 | **DVC / LakeFS 数据版本管理** | L766-793 | ❌ 未实现 | 零代码 |
| 15 | **llm-d 解耦式推理**（Prefill/Decode 分离） | L1032-1037 | ❌ 未实现 | 零代码。README Roadmap 标为 v0.5.0 |
| 16 | **Expert Parallelism（MoE 模型）** | L1032-1037 | ❌ 未实现 | 零代码 |
| 17 | **KV cache 分层卸载**（GPU→CPU→SSD→远程） | L1032-1037 | ❌ 未实现 | 零代码 |
| 18 | **AMD ROCm / Intel Gaudi 支持** | L1032-1037 | ❌ 未实现 | 零代码 |
| 19 | **KServe 集成** | L1032-1037 | ❌ 未实现 | 零代码 |
| 20 | **GPU Time-Slicing / MIG 共享** | L310-318 | ❌ 未实现 | 零代码。需 NVIDIA GPU Operator 支持 |
| 21 | **Karpenter / Cluster Autoscaler 节点扩缩** | L310-318 | ❌ 未实现 | 零代码 |
| 22 | **NFD（Node Feature Discovery）GPU 拓扑标签** | L310-318 | ❌ 未实现 | 零代码 |
| 23 | **External Secrets Operator** | L810-815 | ❌ 未实现 | 零代码。全部密码硬编码在 values.yaml 中，无 `existingSecret` 字段 |
| 24 | **Cilium mTLS / Service Mesh** | L810-815 | ❌ 未实现 | 零代码。仅有基础 NetworkPolicy（4 个服务，无 egress 规则） |
| 25 | **结构化日志解析**（Request/Engine/Gateway/Audit 四类日志） | L676-691 | ⚠️ 部分 | Fluent Bit + Loki 基础管道存在，但 **无针对 vLLM/LiteLLM 的日志解析规则**，无 Request ID / Token 级结构化字段 |
| 26 | **Scale-to-zero** | L828-839 | ❌ 未实现 | KEDA 模板的 `minReplicaCount` 默认为 1，无 scale-to-zero 配置 |

---

## 第三类：生产级 Infra 工程能力缺失

> 这些不是文档中显式声明的"功能"，而是一个自称"生产级"的 Infra 平台应当具备的基础能力。

| # | 能力 | 实际状态 | 影响 |
|---|------|---------|------|
| 27 | **PodDisruptionBudget** | ❌ 全部缺失 | `kubectl drain` 可同时驱逐所有副本 → 全平台中断 |
| 28 | **AlertManager + 通知渠道** | ❌ 缺失 | 11 条 Prometheus 告警规则已定义，但无 AlertManager 部署 → 告警触发后无人收到通知 |
| 29 | **有状态组件 HA** | ❌ 全部单副本 | PostgreSQL / Keycloak / Prometheus / Grafana / Loki / ClickHouse / MinIO 均为单副本，含 production profile |
| 30 | **升级指南** | ❌ 缺失 | 无 `docs/upgrade-guide.md`，v0.1→v0.2 无迁移说明、无 DB schema 兼容性说明 |
| 31 | **values.schema.json** | ❌ 缺失 | values 填错只有部署时才会报错，无静态校验 |
| 32 | **备份 CronJob** | ⚠️ 仅脚本 | `scripts/backup.sh` 存在，但无 K8s CronJob 模板，无保留策略 |
| 33 | **GPU 拓扑感知调度** | ❌ 缺失 | 多卡 Tensor Parallelism 场景下，无法保证分配到 NVLink 互连的 GPU |
| 34 | **Egress NetworkPolicy** | ❌ 缺失 | 仅有 Ingress 规则，Pod 可任意访问外网 |
| 35 | **NetworkPolicy 覆盖不全** | ⚠️ 部分 | 仅覆盖 vLLM / LiteLLM / Prometheus / Grafana 4 个服务，Langfuse / Dify / Keycloak / MinIO / TEI / PostgreSQL / ClickHouse / Redis 均无策略 |
| 36 | **OTel Traces 持久化** | ⚠️ 部分 | OTel Collector 收到 Traces 后仅输出到 debug exporter，**不写入 Jaeger/Tempo** |
| 37 | **Langfuse OIDC 集成** | ⚠️ 部分 | Grafana / MinIO / LiteLLM 均已对接 Keycloak OIDC，**Langfuse 未对接**（values 中 `oidc.enabled: false`，模板中无 AUTH env vars）|

---

## 第四类：PLAN.md 中声明的文档 / 运维工具

> PLAN.md 列出了约 15 篇文档和若干运维脚本，大部分未创建。

| # | 承诺的文档 / 工具 | 实际状态 |
|---|------------------|---------|
| 38 | `docs/upgrade-guide.md` | ❌ 不存在 |
| 39 | `docs/troubleshooting.md`（独立排障指南） | ❌ 不存在（getting-started.md 末尾有少量排障内容）|
| 40 | `docs/faq.md` | ❌ 不存在 |
| 41 | `docs/gpu-setup-guide.md` | ❌ 不存在 |
| 42 | `docs/model-serving-guide.md` | ❌ 不存在 |
| 43 | `docs/gateway-guide.md` | ❌ 不存在 |
| 44 | `docs/security-guide.md` | ❌ 不存在 |
| 45 | `docs/rag-guide.md` | ❌ 不存在 |
| 46 | `docs/fine-tuning-guide.md` | ❌ 不存在 |
| 47 | `docs/model-resolver-guide.md` | ❌ 不存在 |
| 48 | `docs/observability-guide.md` | ❌ 不存在 |
| 49 | `docs/operations-guide.md`（故障模式分析） | ❌ 不存在 |
| 50 | `docs/compatibility.md`（兼容性矩阵） | ❌ 不存在 |
| 51 | `docs/sizing-guide.md`（资源推荐） | ❌ 不存在 |
| 52 | `docs/slo.md`（SLO 框架） | ❌ 不存在 |
| 53 | `scripts/benchmark.sh`（性能基准） | ❌ 不存在 |
| 54 | `docs/migration/`（从竞品迁移指南） | ❌ 不存在 |
| 55 | 备份 CronJob 模板（PG dump / MinIO sync / Milvus snapshot） | ❌ 不存在 |
| 56 | Grafana 成本/团队用量 Dashboard | ❌ 不存在 |
| 57 | Prompt A/B 测试指标 Grafana Panel | ❌ 不存在 |

---

## 第五类：RAG-ASSESSMENT.md 中自查发现的差距（Phase 4 后更新）

> **重要上下文**：RAG-ASSESSMENT.md 是一份渐进式文档。Phase 4 (commit c1e91d4) 完成了全部 4 项企业级功能。以下标注 Phase 4 后的最新状态。

| # | 能力 | Phase 4 前状态 | Phase 4 后状态 | 说明 |
|---|------|--------------|--------------|------|
| 58 | **Milvus 生产验证** | ⚠️ 纯模板 | ✅ 已实现 | etcd 修复 + MinIO 存储对接 + Prometheus scrape + Grafana Dashboard（6 panel：Collections/Entities/Latency/Memory/SearchQPS/InsertQPS）|
| 59 | **多租户知识库隔离** | ⚠️ 部分 | ✅ 已实现 | 2 个 team namespace (team-alpha/team-beta) + ResourceQuota (GPU/CPU/Memory/Pods) + NetworkPolicy 隔离 |
| 60 | **PII 脱敏（Presidio）** | ❌ 未实现 | ✅ 已实现 | Analyzer (EMAIL/PERSON/URL 检测) + Anonymizer (文本脱敏)，双 Deployment + Service + 健康检查 |
| 61 | **知识图谱（LightRAG）** | ❌ 未实现 | ✅ 已实现 | Neo4j v5-community (7687/7474) + LightRAG API (9621)，OpenAI binding → LiteLLM，APOC 插件，ConfigMap 配置 |
| 62 | **Hybrid Retrieval（tsvector + pgvector）** | ⏭️ 跳过 | ⏭️ 跳过 | 因 Dify 内置混合检索而跳过，未在平台层实现 |
| 63 | **Prompt A/B 指标** | ❌ 未实现 | ❌ 未实现 | Grafana 中无按 prompt 版本对比的 Panel |
| 64 | **Data 更新 Pipeline** | ❌ 未实现 | ❌ 未实现 | 无自动化的知识库数据更新流水线 |
| 65 | **Model 热换 Pipeline** | ❌ 未实现 | ❌ 未实现 | 换模型后无自动化回归测试流程 |

---

## 汇总统计

| 分类 | 数量 | 说明 |
|------|------|------|
| ✅ 已实现且可用 | **~75** | 推理(vLLM/TEI/llamacpp) + 网关(LiteLLM) + 监控(Prometheus/Grafana/**5** Dashboard) + 追踪(Langfuse v3) + 日志(Fluent Bit/Loki) + SSO(Keycloak) + RAG(Dify/pgvector/Ragas/质量门控) + 安全(LLM-Guard/NetworkPolicy/**Presidio PII**) + 存储(MinIO) + 自动化部署(Setup Job) + E2E 测试(31 tests) + **LightRAG 知识图谱** + **Milvus 向量数据库(含监控)** + **多租户隔离** |
| ⚠️ 部分实现 | **~30** | 有模板但需外部 Operator(KEDA/Envoy/Fluid)、功能不完整(Model Resolver 未集成)、覆盖不全(NetworkPolicy/OIDC) |
| ❌ 未实现 | **~45** | ARCHITECTURE.md 愿景(SGLang/JupyterHub/MLflow/llm-d/ArgoCD)、生产工程能力(PDB/HA/ESO/mTLS)、文档(15 篇)、Dashboard(成本/A-B) |

> **Phase 4 变化**：✅ 从 ~70 → ~75（+5），❌ 从 ~50 → ~45（-5）。LightRAG / Presidio / Milvus 生产化 / 多租户 4 项从"未实现"移入"已实现"。

---

## 建议：怎么修

### 方案 A：修文档（低成本，立即可做）

ARCHITECTURE.md 应区分"当前实现"和"未来愿景"。建议：

1. **ARCHITECTURE.md** 中每个组件加状态标签：`[IMPLEMENTED]` / `[TEMPLATE-ONLY]` / `[PLANNED v0.x]`
2. **README.md** Features 对比表中，KEDA / Engine Auto-Selection 标注为 `Partial` 而非 `Yes`
3. **PLAN.md** 中未实现的文档从 checklist 中移除或标为 `[ ]`
4. 根目录的 8 个 RAG-*.md 文件合并收敛为 1-2 个，消除自相矛盾

### 方案 B：修代码（按优先级排列）

| 优先级 | 项目 | 工作量 | 理由 |
|--------|------|--------|------|
| **P0** | PodDisruptionBudget（全组件） | 半天 | 零成本高回报，防止 node drain 导致全平台宕机 |
| **P0** | `existingSecret` 字段 + `randAlphaNum` 默认密码 | 1 天 | 消除安全硬伤 |
| **P0** | Prometheus 硬编码 target → Kubernetes SD | 1 天 | 当前实现无法支持多模型部署 |
| **P1** | AlertManager 子 chart + Slack/Webhook 配置 | 1 天 | 告警有规则无通知 = 等于没有告警 |
| **P1** | Model Resolver 集成到 vLLM init-container | 2 天 | README 核心卖点，当前是空头承诺 |
| **P1** | values.schema.json | 2 天 | 大幅降低用户配置出错率 |
| **P1** | Upgrade Guide (docs/upgrade-guide.md) | 1 天 | v0.1→v0.2 已发生，用户无迁移路径 |
| **P2** | Grafana 成本/团队用量 Dashboard | 2 天 | README Use Case 承诺但未兑现 |
| **P2** | NetworkPolicy 扩展到全部服务 + Egress 规则 | 2 天 | 当前仅覆盖 4/15 服务 |
| **P2** | Langfuse OIDC 集成 | 半天 | 其他 3 个服务都接了，Langfuse 是唯一缺口 |
| **P2** | 备份 CronJob 模板 | 1 天 | 脚本已有，包装成 K8s CronJob 即可 |
| **P3** | PostgreSQL 分离（至少拆为 2 实例） | 3 天 | 消除全平台 DB 单点故障 |
| **P3** | KEDA / Envoy / Fluid 的前置安装文档 | 1 天 | 标注外部依赖，降低用户困惑 |

---

*本文档基于代码静态审计。标注为"未实现"的项目可能在后续版本中已补充，以实际代码为准。*
