# RAG 企业级生产方案：技术栈叠加与组合策略

> 单个技术是原料，生产系统是菜谱。本文档讲"怎么做菜"——各技术层如何叠加组合，各方案优缺点对比，以及企业当前主流选择。

---

## 一、核心认知：生产级 RAG 是分层防御架构

企业级 RAG 不是"选一个最好的技术"，而是像网络安全一样做**分层防御**——每层解决一类问题，层层兜底：

```
用户原始 query: "我们公司去年Q3华东区的营收增长率是多少？"
  │
  ▼ [Layer 1: Intent + Safety] 识别意图 + 输入安全过滤
  │  → 判定为：知识库查询 (非闲聊/非注入攻击)
  ▼ [Layer 2: Query 改写] 多策略叠加
  │  → 训练好的 rewriter: "2023年第三季度华东区营收同比增长率"
  │  → Multi-Query 扩展: + "2023 Q3 华东区收入变化" + "东部区域三季度业绩"
  │  → 兜底: 原始 query 也参与检索
  ▼ [Layer 3: 检索] 混合检索 + 融合
  │  → Dense (语义相似): 找到年报段落
  │  → Sparse (BM25 关键词): 精确匹配"Q3""华东"
  │  → RRF 融合 → Top 20
  ▼ [Layer 4: Rerank + 过滤]
  │  → Cross-encoder 重排序 → Top 5
  │  → 元数据过滤: 只保留 2023 年的文档
  │  → 去重
  ▼ [Layer 5: 生成]
  │  → Faithful prompt: "仅基于以下上下文回答，引用来源"
  │  → 流式输出
  ▼ [Layer 6: 输出安全]
  │  → PII 脱敏检查
  │  → 幻觉检测（如果置信度低，标注"无法确认"）
  ▼
最终回答: "根据2023年度报告[1]，华东区Q3营收同比增长12.3%。"
```

**每一层都有"主力方案 + 兜底方案"的设计模式。**

---

## 二、技术叠加详解：以 Query Rewriting 为例

### 2.1 为什么需要叠加？

单独任何一种 query 改写方法都有明显弱点：

| 方法 | 优点 | 缺点 | 失败场景 |
|------|------|------|---------|
| **System prompt 指导** | 零成本、零延迟、无需额外模型 | 效果弱，LLM 经常忽略改写指令 | 复杂多跳问题、模糊指代 |
| **LLM 直接改写** | 理解力强、能处理模糊语义 | 增加一次 LLM 调用（+200-500ms）、可能改写过度丢失原意 | 已经很精确的 query 被画蛇添足 |
| **小模型 rewriter (T5)** | 快速（<50ms）、可离线训练、确定性强 | 需要训练数据、领域迁移差 | 没见过的 query 类型 |
| **Multi-Query 扩展** | 覆盖面广、能捕获不同语义角度 | 检索量翻倍（成本↑）、低质量变体引入噪声 | 简单查询被不必要地复杂化 |
| **HyDE** | 显著提升召回率（特别是开放域问题） | 延迟高（需要生成完整假设文档）、LLM 生成的假设可能偏离实际 | 事实性查询（"XX价格多少"）不需要假设 |
| **Sub-Question 分解** | 多跳推理必备 | 简单问题被过度分解、每个子问题都要单独检索 | 单跳事实问题 |

### 2.2 企业主流叠加模式

**模式 A：轻量级（推荐起步方案，延迟 +100-300ms）**

```
用户 query
  │
  ├─→ [原始 query] ──────────────────→ ┐
  │                                     │
  └─→ [LLM prompt 改写] ─→ [改写后 query] → 合并去重 → 检索
                                        │
      system prompt 示例:                │
      "将用户问题改写为更适合             │
       向量检索的形式，保留               │
       所有关键实体和时间。"              │
```

- **核心思路**：原始 query + LLM 改写后的 query **同时参与检索**，结果合并
- **兜底机制**：如果 LLM 改写失败或超时，原始 query 保底
- **使用者**：Dify 默认方案、大多数初创企业
- **论文支撑**：Rewrite-Retrieve-Read (Ma et al., [arXiv:2305.14283](https://arxiv.org/abs/2305.14283))

**模式 B：中等复杂（大多数企业生产环境，延迟 +300-800ms）**

```
用户 query
  │
  ▼
[意图分类] ─── 闲聊 ─→ 直接回答（不检索）
  │
  ├── 简单事实查询 ─→ [原始 query + BM25 关键词改写]
  │
  ├── 复杂分析查询 ─→ [LLM 改写 + Multi-Query 3 变体]
  │
  └── 多跳推理查询 ─→ [Sub-Question 分解 → 逐个检索 → 合并]
```

- **核心思路**：先分类再决定用什么改写策略，不浪费算力
- **意图分类**：用 LLM 或小分类模型（<10ms）
- **兜底机制**：分类不确定时退回"中等复杂"路径
- **使用者**：字节跳动 Coze、AWS Bedrock Knowledge Bases、Azure AI Search
- **论文支撑**：Query Routing (RAG Survey Section III-C3)、Adaptive Retrieval (Section V-C)

**模式 C：完整方案（金融/医疗等高准确率场景，延迟 +500-1500ms）**

```
用户 query
  │
  ▼
[输入安全] → prompt injection 检测 + PII 脱敏
  │
  ▼
[意图 + 复杂度判断]
  │
  ├── L1 简单 → [原始 query]
  │
  ├── L2 中等 → [LLM 改写] + [HyDE 假设文档]
  │                  │              │
  │                  ▼              ▼
  │              query 检索     假设文档检索
  │                  │              │
  │                  └──── RRF 融合 ─┘
  │
  └── L3 复杂 → [Sub-Question 分解]
                    │
                    ├── Q1 → [LLM 改写 + Multi-Query] → 检索1
                    ├── Q2 → [LLM 改写 + Multi-Query] → 检索2
                    └── Q3 → [LLM 改写 + Multi-Query] → 检索3
                                                         │
                                                    合并 + Rerank
```

- **核心思路**：按查询复杂度分级处理，高复杂度查询投入更多计算资源
- **每层都有 fallback**：HyDE 超时 → 回退到纯 LLM 改写；分解失败 → 回退到单 query
- **使用者**：微软 Copilot、Google AI Search（内部方案类似）
- **论文支撑**：Self-RAG ([arXiv:2310.11511](https://arxiv.org/abs/2310.11511))、FLARE ([arXiv:2305.06983](https://arxiv.org/abs/2305.06983))

### 2.3 各企业实际选择

| 企业/产品 | Query 改写方案 | 为什么这样选 |
|-----------|---------------|-------------|
| **Dify** (开源 RAG 平台) | Prompt-based + 可选 Multi-Query | 面向 low-code 用户，需要简单配置 |
| **RAGFlow** (InfoQ) | 内置多策略：关键词提取 + 语义改写 + 问题分解 | 面向企业私有化，注重文档理解 |
| **AWS Bedrock KB** | 自动 query 分解 + 混合检索 | 全托管，用户无需感知细节 |
| **Azure AI Search** | Semantic query + 自动扩展 + Reranker | 深度集成 Azure 生态 |
| **Cohere** | Query expansion API + Rerank | 专注检索质量，API 驱动 |
| **Google Vertex AI** | 自动多变体 + Grounding | 深度集成 Google Search 作为 fallback |
| **Perplexity** | Multi-Query + Web search + 多引擎融合 | 搜索引擎级别的召回率需求 |
| **金融企业（自建）** | T5 rewriter + LLM fallback + 领域词典 | 需要可审计、确定性强、延迟低 |

---

## 三、完整生产 RAG Pipeline 的技术栈选择

### 3.1 推荐起步方案（3 周可落地）

```
┌─────────────────────────────────────────────────────────┐
│  Pre-Retrieval                                           │
│  LLM prompt 改写 (system prompt, 零成本)                  │
│  原始 query 同时保留 (兜底)                                │
├─────────────────────────────────────────────────────────┤
│  Indexing                                                │
│  Dify 内置文档解析 + 递归 chunking (500 token, 50 overlap) │
│  pgvector 存储 (复用现有 PostgreSQL)                       │
├─────────────────────────────────────────────────────────┤
│  Retrieval                                               │
│  Dense only (pgvector cosine, top-20)                    │
│  无 reranking                                             │
├─────────────────────────────────────────────────────────┤
│  Generation                                              │
│  Faithful prompt (rag-strict 模板)                        │
│  流式输出 (LiteLLM streaming)                             │
├─────────────────────────────────────────────────────────┤
│  Evaluation                                              │
│  手动抽检 + Langfuse trace 查看                            │
└─────────────────────────────────────────────────────────┘

优点: 3 周内可上线，复用全部现有组件
缺点: 检索质量一般（无混合检索、无 rerank），无自动化评估
适用: POC 验证、内部知识库 demo
```

### 3.2 推荐生产方案（2-3 个月落地）

```
┌─────────────────────────────────────────────────────────┐
│  Pre-Retrieval                                           │
│  意图分类 (LLM, <50ms)                                    │
│  LLM query 改写 + Multi-Query 3 变体 (+200ms)             │
│  原始 query 保底                                          │
├─────────────────────────────────────────────────────────┤
│  Indexing                                                │
│  Unstructured.io (文档解析, 20+ 格式)                      │
│  语义 chunking (embedding 相似度断点, 200-500 token)       │
│  元数据: 来源、日期、部门、页码                              │
│  pgvector (向量) + tsvector (全文)                         │
├─────────────────────────────────────────────────────────┤
│  Retrieval                                               │
│  Hybrid: Dense (pgvector, top-30) + Sparse (BM25, top-30) │
│  RRF 融合 → top-20                                        │
│  Cross-encoder reranking (bge-reranker, top-5) (+100ms)  │
│  元数据过滤 (日期、部门)                                    │
├─────────────────────────────────────────────────────────┤
│  Generation                                              │
│  Faithful + Citation prompt (标注来源段落)                  │
│  流式输出                                                  │
│  LLM-Guard 输出扫描 (PII 检查, <20ms)                     │
├─────────────────────────────────────────────────────────┤
│  Evaluation (离线)                                        │
│  Ragas (Faithfulness + Context Precision + Relevance)     │
│  每日 CronJob → Prometheus → Grafana 告警                  │
│  回归门控: 质量下降 >5% 阻断部署                             │
└─────────────────────────────────────────────────────────┘

优点: 检索质量显著提升（hybrid + rerank），有自动化质量监控
缺点: 需要额外组件（Unstructured、Reranker），延迟增加 300-500ms
适用: 正式生产环境、企业知识库、客服系统
```

### 3.3 推荐企业级方案（6+ 个月，全能力覆盖）

```
┌─────────────────────────────────────────────────────────┐
│  Input Safety                                            │
│  LLM-Guard (prompt injection + PII + toxicity)           │
│  Presidio (PII 脱敏)                                     │
├─────────────────────────────────────────────────────────┤
│  Pre-Retrieval                                           │
│  意图分类 → 路由 (知识库/SQL/图谱/Web搜索)                  │
│  复杂度评估 → 选择改写策略                                  │
│  L1: 原始 query                                          │
│  L2: LLM 改写 + Multi-Query + HyDE                       │
│  L3: Sub-Question 分解 → 各子问题独立检索                    │
├─────────────────────────────────────────────────────────┤
│  Indexing                                                │
│  Unstructured.io + MinerU (中文文档)                       │
│  多策略 chunking: 结构感知 + 语义分割                       │
│  多粒度索引: 文档摘要层 + 段落层 + 句子层 (RAPTOR)          │
│  知识图谱: LightRAG (实体关系抽取 + 图检索)                 │
│  pgvector + Milvus (大规模) + tsvector                     │
├─────────────────────────────────────────────────────────┤
│  Retrieval                                               │
│  Stage 1: Hybrid retrieval (dense + sparse + graph)       │
│  Stage 2: RRF 融合 → top-50                               │
│  Stage 3: Cross-encoder rerank → top-10                   │
│  Stage 4: Context compression (LLMLingua) → 保留核心部分   │
│  元数据 RBAC: 按用户角色过滤可见文档                         │
├─────────────────────────────────────────────────────────┤
│  Generation                                              │
│  Faithful + Citation + 结构化输出                           │
│  Self-verification (CoVe): LLM 检查自己的回答               │
│  流式输出 + 思维链控制 (Qwen3.5 enable_thinking)           │
├─────────────────────────────────────────────────────────┤
│  Output Safety                                           │
│  NeMo Guardrails / LLM-Guard (topical + safety + PII)    │
│  Hallucination 检测 (Vectara HHEM 模型)                    │
│  不确定性标注 ("此信息未经验证，请核实")                      │
├─────────────────────────────────────────────────────────┤
│  Evaluation (全自动)                                      │
│  Ragas 8 指标 + DeepEval + HHEM                           │
│  Automated test generation (Ragas TestsetGenerator)       │
│  A/B testing: 不同 prompt/模型/检索策略对比                  │
│  回归门控: CI/CD pipeline，质量下降自动阻断                   │
│  Grafana 实时 dashboard: 检索延迟、质量评分趋势、幻觉率      │
└─────────────────────────────────────────────────────────┘
```

---

## 四、各层技术优缺点详细对比

### 4.1 检索层选型对比

| 维度 | Dense Only | BM25 Only | Hybrid (Dense+BM25) | + Reranking | + ColBERT |
|------|-----------|-----------|---------------------|-------------|-----------|
| **精确关键词** | ❌ 差（语义漂移） | ✅ 优 | ✅ 优 | ✅ 优 | ✅ 优 |
| **语义理解** | ✅ 优 | ❌ 差 | ✅ 优 | ✅✅ 更优 | ✅✅ 更优 |
| **延迟** | ~10ms | ~5ms | ~15ms | +50-200ms | +30-100ms |
| **召回率** | 中 | 中 | 高 | 高 | 最高 |
| **准确率** | 中 | 中 | 中高 | 高 | 最高 |
| **额外组件** | 向量 DB | 全文索引 | 两者 | + reranker 模型 | + ColBERT 索引 |
| **企业主流** | 起步 | 传统搜索 | **大多数生产环境** | **金融/医疗** | 前沿研究 |
| **代表用户** | 小团队 POC | Elasticsearch | Dify, AWS, Azure | Cohere 客户 | Jina, Vespa |

**结论**：绝大多数企业在 **Hybrid + Reranking** 这一档。pure dense 或 pure sparse 只适合 POC。

### 4.2 Embedding 模型选型对比

| 模型 | 维度 | 语言 | MTEB 排名 | 许可 | 推荐场景 |
|------|------|------|----------|------|---------|
| `text-embedding-3-large` (OpenAI) | 3072 | 多语言 | #1-3 | API only | 不限私有化的场景 |
| `bge-m3` (BAAI) | 1024 | 100+语言 | Top 10 | MIT | **私有化首选（多语言）** |
| `bge-large-zh-v1.5` (BAAI) | 1024 | 中文 | 中文 #1 | MIT | **中文企业首选** |
| `all-MiniLM-L6-v2` | 384 | 英文 | 中等 | Apache 2.0 | 轻量/低延迟/demo |
| `e5-mistral-7b-instruct` (Microsoft) | 4096 | 多语言 | Top 5 | MIT | 高精度（但需 GPU） |
| `nomic-embed-text-v1.5` | 768 | 英文 | Top 15 | Apache 2.0 | 长文本（8192 token） |
| `jina-embeddings-v3` (Jina) | 1024 | 多语言 | Top 10 | Apache 2.0 | 多任务（检索+分类） |
| `voyage-3` (Voyage AI) | 1024 | 多语言 | Top 5 | API only | 代码+文本混合检索 |

**企业主流**：
- 私有化中文：`bge-large-zh-v1.5` 或 `bge-m3`
- 私有化多语言：`bge-m3`
- 不限私有化：OpenAI `text-embedding-3-large`
- kube-llmops 建议：**TEI 部署 bge-m3**（通过 LiteLLM 路由）

### 4.3 Reranker 选型对比

| Reranker | 延迟 (10 doc) | 精度 | 语言 | 部署 | 许可 | 成本 |
|----------|-------------|------|------|------|------|------|
| **Cohere Rerank v4** | ~100ms | 最高 | 100+ | API | 商业 | $0.002/查询 |
| **bge-reranker-v2-m3** (BAAI) | ~80ms | 高 | 100+ | 本地 | MIT | GPU 算力 |
| **cross-encoder/ms-marco-MiniLM-L-6-v2** | ~20ms | 中 | 英文 | 本地 | Apache 2.0 | CPU 可跑 |
| **Jina Reranker v2** | ~50ms | 高 | 多语言 | 本地+API | Apache 2.0 | GPU 算力 |
| **FlashRank** | ~5ms | 中等 | 英文 | 本地 | MIT | CPU 可跑 |
| **RankGPT** (LLM-based) | ~2000ms | 极高 | 任意 | LLM API | - | LLM 调用费 |

**企业主流**：
- 私有化：`bge-reranker-v2-m3`（精度高 + MIT 开源）
- 不限私有化：Cohere Rerank API（最简单）
- 低延迟优先：FlashRank 或 MiniLM（CPU 可跑）
- kube-llmops 建议：**TEI 部署 bge-reranker-v2-m3**

### 4.4 评估框架选型对比

| 框架 | 核心指标数 | LLM 依赖 | Langfuse 集成 | CI/CD 支持 | 许可 |
|------|----------|---------|--------------|-----------|------|
| **Ragas** | 8 | 是（评估 LLM） | ✅ 原生 | ✅ pytest | Apache 2.0 |
| **DeepEval** | 15+ | 是 | ❌ | ✅ pytest | Apache 2.0 |
| **TruLens** | 8 | 是 | ❌ | ⚠️ 部分 | MIT |
| **ARES** | 3 | 是 | ❌ | ✅ | MIT |
| **Vectara HHEM** | 1 (幻觉) | **否**（小模型） | ❌ | ✅ | Apache 2.0 |

**企业主流**：**Ragas + HHEM 组合**
- Ragas 做 Faithfulness/Relevance/Precision 等全面评估
- HHEM 做轻量级幻觉检测（不需要 LLM，成本低 100x）
- 两者结合 = 全面 + 经济

### 4.5 Guardrails 选型对比

| 框架 | 输入防护 | 输出防护 | PII | 延迟 | 部署 | 许可 |
|------|---------|---------|-----|------|------|------|
| **NeMo Guardrails** | ✅ 完整 | ✅ 完整 | ⚠️ 基础 | ~200ms | 本地 | Apache 2.0 |
| **LLM-Guard** | ✅ 11 scanner | ✅ 17 scanner | ✅ 完整 | ~50ms | 本地 API | MIT |
| **Guardrails AI** | ❌ 弱 | ✅ 完整 | ⚠️ 基础 | ~100ms | 本地 | Apache 2.0 |
| **Presidio** | ❌ 仅 PII | ❌ 仅 PII | ✅✅ 最佳 | ~10ms | 本地 | MIT |
| **Azure Content Safety** | ✅ | ✅ | ✅ | ~50ms | 云 API | 商业 |
| **AWS Bedrock Guardrails** | ✅ | ✅ | ✅ | ~100ms | 云 API | 商业 |

**企业主流组合**：
- 私有化：**LLM-Guard（输入/输出扫描）+ Presidio（PII 脱敏）**
- 高安全：**NeMo Guardrails（策略引擎）+ LLM-Guard（扫描器）+ Presidio（PII）**
- kube-llmops 建议：LLM-Guard 作为 LiteLLM 中间件 + Presidio sidecar

---

## 五、kube-llmops 推荐路线图

基于以上分析，kube-llmops 应该按以下顺序实现：

### Phase 1（起步方案，3 周）
```
[已有] LiteLLM + vLLM + pgvector + Langfuse
[新增] TEI embedding (bge-m3) → LiteLLM embedding route
[新增] Dify embedding 接 LiteLLM（不走 HuggingFace 直连）
[新增] Prompt-based query 改写（rag-strict 模板升级）
结果: 端到端 RAG 能跑通
```

### Phase 2（生产方案，2 月）
```
[新增] Hybrid retrieval: pgvector + tsvector 在同一 PG 内
[新增] TEI reranking (bge-reranker-v2-m3)
[新增] Ragas 评估 CronJob → Prometheus → Grafana
[新增] LLM-Guard 作为 LiteLLM 中间件
[升级] RAG trace spans: embed → retrieve → rerank → generate
结果: 检索质量+评估+安全 到位
```

### Phase 3（企业方案，3 月+）
```
[新增] LightRAG 知识图谱（可选子 chart）
[新增] Multi-Query + Query Routing
[新增] Context compression (LLMLingua)
[新增] Presidio PII 脱敏
[新增] 多租户 RBAC（metadata filter + Keycloak org）
[新增] 回归测试门控（CI/CD quality gate）
结果: 企业级全能力
```

---

## 六、关键决策点总结

| 决策 | 选 A | 选 B | 我们的选择 | 理由 |
|------|------|------|----------|------|
| Embedding 模型 | OpenAI API | **bge-m3 本地** | B | 私有化 + 多语言 + MIT |
| 向量 DB | Milvus | **pgvector** | B (起步) | 零额外组件，复用 PG |
| 全文检索 | Elasticsearch | **PostgreSQL tsvector** | B | 零额外组件 |
| Reranker | Cohere API | **bge-reranker 本地** | B | 私有化 + MIT |
| 文档解析 | 自建 | **Unstructured.io** | B | Apache 2.0，20+ 格式 |
| RAG 评估 | 自写关键词匹配 | **Ragas** | B | 8 指标 + Langfuse 集成 |
| Guardrails | 自写规则 | **LLM-Guard** | B | MIT，28 扫描器 |
| 知识图谱 | Microsoft GraphRAG | **LightRAG** | B | 更轻量，适合起步 |
| RAG 平台 | 自建 | **Dify** | B | 134K stars，UI + workflow |

**核心原则**：不自建任何已有成熟开源方案的功能。我们的价值是**集成和运维**，不是重复造轮子。
