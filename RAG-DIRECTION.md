# kube-llmops RAG 模块开发建议

## 定位澄清

kube-llmops 是 **Infra/Stack**，不是 RAG 应用。我们的用户是平台工程师，不是 AI 应用开发者。

这意味着：
- **我们不做文档解析、chunking、query 改写**——这些是 Dify/RAGFlow/LangChain 的事
- **我们做的是**：让这些 RAG 应用跑在 K8s 上时，embedding 有服务、向量有库、检索有加速、质量有评估、安全有防护、出了问题能看到
- 类比：我们是 AWS Bedrock 的私有化版——不是做 RAG 应用，而是提供 RAG 应用需要的一切基础设施

```
┌────────────────────────────────────────────────────────┐
│  RAG 应用层（用户自选，我们不碰）                         │
│  Dify │ RAGFlow │ LangChain │ n8n │ 自建              │
├────────────────────────────────────────────────────────┤
│  ▼ kube-llmops 的边界 ▼                                │
│                                                        │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ │
│  │ Embedding│ │ Retrieval│ │ Reranking│ │ LLM 推理  │ │
│  │ Service  │ │ Backend  │ │ Service  │ │ Gateway  │ │
│  │ (TEI)    │ │(pgvector)│ │ (TEI)    │ │(LiteLLM) │ │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘ │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ │
│  │ RAG Eval │ │Guardrails│ │Observabil│ │ Storage  │ │
│  │ (Ragas)  │ │(LLM-Guard│ │(Langfuse)│ │ (MinIO)  │ │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘ │
└────────────────────────────────────────────────────────┘
```

---

## 一、开发原则

### 1. "helm install 能用" 是唯一标准

不是"模板有了"，不是"文档里写了怎么配"。是用户执行 `helm install` 之后，**不看文档、不改配置，RAG 基础设施就已经在跑了**。

具体含义：
- TEI 部署后，`/v1/embeddings` 接口能返回向量——不需要用户手动下载模型
- pgvector 里有 `tsvector` 全文索引——不需要用户手动建
- Ragas 评估 CronJob 在跑——不需要用户写 Python 脚本
- LLM-Guard 在拦截——不需要用户配规则

### 2. 不造轮子，只做集成和胶水

RAG 技术栈里每一层都有成熟开源方案（详见 `docs/rag-tech/`）。我们的价值是：
- 把 Ragas（Python 库）变成 K8s CronJob + Prometheus exporter + Grafana dashboard
- 把 LLM-Guard（Python 库）变成 LiteLLM 的 sidecar/middleware
- 把 TEI（Docker 镜像）变成 Helm sub-chart + LiteLLM 自动注册 + 健康检查 + 自动扩缩

**我们不写 RAG 算法，我们写 K8s manifests + Helm charts + Prometheus rules。**

### 3. 渐进式开启，不强制全家桶

用户可能只想要 embedding 服务，不想要 Guardrails。所有 RAG 组件都是 `enabled: false` 默认关闭，通过 values 开启：

```yaml
rag:
  embedding:
    enabled: true
    model: BAAI/bge-m3
  reranking:
    enabled: false
  eval:
    enabled: false
  guardrails:
    enabled: false
```

---

## 二、建议做什么（按优先级）

### Tier 1：RAG 基础服务层（让 RAG 能跑起来）

这是让 kube-llmops 从 "LLM 推理平台" 升级为 "LLMOps 平台" 的最小可行 RAG 支持。

#### 1.1 Embedding Service（TEI 打通）

**现状**：TEI sub-chart 模板存在但 `models: []`，没有默认模型，与 LiteLLM 未集成。

**目标**：`helm install` 后 `/v1/embeddings` 可用。

**做什么**：
- TEI 配一个开箱即用的默认模型（`BAAI/bge-m3`，MIT 许可，多语言）
- LiteLLM configmap 自动注册 TEI 为 embedding provider
- 健康检查：TEI 启动完成（模型加载）后 readiness probe 才通过
- 关键验证：`curl /v1/embeddings -d '{"input":"hello","model":"bge-m3"}'` 返回向量

**不做什么**：不做 embedding fine-tuning、不做多模型切换 UI、不做模型管理。

#### 1.2 Reranking Service（TEI rerank 模式）

**现状**：无。

**目标**：通过 LiteLLM 或独立 endpoint 提供 rerank API。

**做什么**：
- TEI 支持 rerank 模式（`--model BAAI/bge-reranker-v2-m3`）
- 独立 Service + Deployment，与 embedding TEI 分开
- 提供 `/rerank` endpoint

**不做什么**：不做 ColBERT、不做多 reranker 路由。

#### 1.3 Hybrid Retrieval Backend（pgvector + tsvector）

**现状**：pgvector 扩展已启用，但没有 tsvector 全文索引。

**目标**：同一 PostgreSQL 实例提供 dense + sparse 检索能力。

**做什么**：
- 在 PostgreSQL initdb 脚本中启用 `pg_trgm` 扩展
- 提供 SQL 示例/函数：hybrid search with RRF
- 文档：教 RAG 应用（Dify/LangChain）如何调用 hybrid search

**不做什么**：不做检索 API 服务（这是应用层的事），不做 Elasticsearch。

#### 1.4 Dify 打通

**现状**：sub-chart 存在但 disabled，embedding 服务断了。

**目标**：`dify.enabled: true` 后，上传文档 → RAG 对话可以跑通。

**做什么**：
- Dify embedding provider 配置指向 LiteLLM（而非 HuggingFace 直连）
- Dify LLM provider 配置指向 LiteLLM
- 验证端到端：上传 PDF → chunking → embedding → pgvector → 查询 → 答案

**不做什么**：不做 Dify 的二次开发，不维护 Dify 版本升级。

---

### Tier 2：RAG 质量层（让 RAG 质量可衡量）

这是 kube-llmops 的**差异化核心**——Dify/RAGFlow 都不做评估和持续质量监控，我们做。

#### 2.1 Ragas 评估 CronJob

**现状**：rag-eval.sh 做关键词匹配，不是真正的 RAG 评估。

**目标**：每日自动评估 RAG 质量，结果可视化在 Grafana。

**做什么**：
- K8s CronJob：拉取 eval dataset → 调用 RAG pipeline → Ragas 评分 → 结果推 Prometheus
- 3 个核心指标：Faithfulness、Context Precision、Answer Relevancy
- Grafana dashboard：质量趋势图 + 告警（质量下降 >5%）
- Eval LLM 使用 LiteLLM（复用已部署的 LLM，不额外引入外部 API 依赖）

**不做什么**：不做自己的评估算法，不做评估 UI。

#### 2.2 RAG Trace Spans

**现状**：Langfuse 只有 LLM generation 一个 span。

**目标**：在 Langfuse 里看到完整 RAG 链路：embed → retrieve → rerank → generate。

**做什么**：
- 提供 Python SDK wrapper / OpenTelemetry instrumentation 示例
- 在 RAG example app 中集成
- 文档：教 Dify/LangChain 用户如何接入

**不做什么**：不改 Langfuse 源码，不做自己的 tracing 后端。

#### 2.3 Grafana RAG Dashboard（真正的 RAG 指标）

**现状**：rag-quality.json 全是 vLLM 推理指标，没有一个 RAG 指标。

**目标**：替换为真正的 RAG 运维指标。

**做什么**：
- Ragas 评分趋势（Faithfulness / Relevance / Precision）
- 检索延迟分位数（P50/P95/P99）
- Embedding 吞吐量（requests/sec, tokens/sec）
- 幻觉率趋势
- 回归告警（质量下降自动通知）

**不做什么**：不做 RAG 应用层的业务指标。

---

### Tier 3：RAG 安全层（让 RAG 可以上生产）

#### 3.1 LLM-Guard Sidecar

**现状**：无任何输入/输出安全检查。

**目标**：每个经过 LiteLLM 的请求自动过安全扫描。

**做什么**：
- LLM-Guard 作为 LiteLLM 的 sidecar 或前置 middleware
- 默认开启：prompt injection 检测 + PII 检测
- 可选开启：toxicity、ban topics
- Prometheus 指标：blocked_requests_total, scan_latency

**不做什么**：不自建安全扫描器，不做规则引擎。

#### 3.2 回归测试质量门控

**现状**：无。

**目标**：数据更新/模型切换时，自动跑 eval，质量不达标阻断部署。

**做什么**：
- Helm pre-upgrade hook：跑 Ragas eval
- 质量阈值可配（`eval.minFaithfulness: 0.8`）
- 不达标：hook 返回失败，helm upgrade 自动回滚
- GitHub Action：PR 里改了 prompt/model → 自动触发 eval

**不做什么**：不做自己的 CI/CD 系统。

---

### Tier 4：RAG 高级能力（锦上添花）

这些不阻塞任何用户，但有了会更好：

| 能力 | 做什么 | 不做什么 |
|------|--------|---------|
| **知识图谱** | LightRAG 作为可选 sub-chart | 不自建图数据库 |
| **多租户隔离** | pgvector 按 Keycloak org 做 metadata filter | 不做应用层 RBAC |
| **Milvus** | 验证 + 修复现有 chart，大规模场景使用 | 不维护 Milvus 本身 |
| **PII 脱敏** | Presidio sidecar | 不自建 NLP 模型 |

---

## 三、明确不做什么

以下能力属于 RAG 应用层，**不是 kube-llmops 的范围**：

| 能力 | 为什么不做 | 谁做 |
|------|----------|------|
| 文档解析 (PDF/Word) | 这是 RAG 应用的核心功能 | Dify、RAGFlow、Unstructured |
| Chunking 策略 | 不同应用有不同的 chunking 需求 | Dify（4种）、LangChain |
| Query 改写 / HyDE / Multi-Query | 应用层 prompt 工程 | Dify Workflow、LangChain |
| RAG 对话 UI | 我们是 infra，不做前端 | Dify、RAGFlow、自建 |
| Agent / Workflow 编排 | 这是应用层的编排逻辑 | Dify、LangGraph、n8n |
| 引用溯源 (Citation) | 应用层 prompt 工程 + 后处理 | RAGFlow、应用自建 |

**但是**，我们为这些应用层能力提供**基础设施支撑**：
- 文档解析需要 S3 存储 → 我们提供 MinIO
- Chunking 后需要 embedding → 我们提供 TEI
- Query 改写后需要检索 → 我们提供 pgvector + tsvector
- Agent 需要 LLM → 我们提供 LiteLLM + vLLM
- Citation 需要 trace → 我们提供 Langfuse

---

## 四、交付形态

每个 RAG 能力的交付标准：

| 层 | 交付物 | 验证方法 |
|----|--------|---------|
| Embedding | TEI sub-chart + LiteLLM config | `curl /v1/embeddings` 返回向量 |
| Reranking | TEI rerank sub-chart | `curl /rerank` 返回排序结果 |
| Hybrid Retrieval | PG initdb 脚本 + SQL 示例 | SQL 查询返回 hybrid 结果 |
| Eval | CronJob + Prometheus exporter + Grafana panel | Grafana 看到 Faithfulness 趋势 |
| Guardrails | LLM-Guard sidecar + Prometheus metrics | 注入攻击被拦截 + Grafana 看到 blocked 指标 |
| Quality Gate | Helm pre-upgrade hook | 质量下降时 helm upgrade 失败回滚 |
| Trace | Python SDK 示例 + docs | Langfuse 看到 embed→retrieve→generate spans |
| Dashboard | rag-quality.json 替换 | Grafana 6 panel 全有数据 |

---

## 五、排期建议

| Phase | 内容 | 时间 | 效果 |
|-------|------|------|------|
| **P1** | Embedding + Dify 打通 + 端到端验证 | 2-3 周 | RAG 从 0 到 1（能跑通） |
| **P2** | Reranking + Hybrid + Ragas eval + Dashboard | 3-4 周 | RAG 质量可衡量 |
| **P3** | LLM-Guard + Quality Gate + Trace Spans | 3-4 周 | RAG 可上生产 |
| **P4** | LightRAG + 多租户 + Milvus | 按需 | 企业级高级能力 |

P1 做完，README 可以改成：
```
- **RAG Infrastructure** -- Embedding (TEI) + Vector DB (pgvector) + Reranking + Dify RAG platform
```

P2 做完，这就是市面上唯一一个 **"带 RAG 质量评估的 K8s LLMOps 平台"**。
