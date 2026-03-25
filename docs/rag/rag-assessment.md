# kube-llmops RAG 能力评估报告

> **注意**：本报告的第二至四章（差距分析、技术全景图）撰写于 RAG Phase 1-4 实施之前，反映的是当时的基线状态。
> 实施完成后的最新状态请参见第七章或 [rag-todo.md](rag-todo.md)。

**基于业界企业级 RAG 方案对比分析**

---

## 一、评估对象

本报告对比 kube-llmops 的 RAG 基础设施与以下企业级 RAG 解决方案：

| 方案 | 类型 | GitHub Stars | 定位 |
|------|------|-------------|------|
| **Dify** | 全栈 RAG 平台 | 134K | Low-code LLM 应用平台，内置 RAG pipeline |
| **RAGFlow** | 专业 RAG 引擎 | 75.8K | 深度文档理解 + 知识库引擎 |
| **LangChain** | 开发框架 | 110K+ | Python/JS agent + retrieval 框架 |
| **LlamaIndex** | 数据框架 | 40K+ | 结构化数据索引 + 检索框架 |
| **Ragas** | 评估框架 | 8K+ | 专业 RAG 评估指标（Faithfulness/Relevance/etc） |
| **LazyLLM** | 企业级框架 | 中国生态 | 多知识库权限隔离 + 企业安全 |
| **NVIDIA NIM/NeMo** | 企业 AI 平台 | 商业 | GPU 加速 RAG + Guardrails |

---

## 二、企业级 RAG 的 8 大核心能力矩阵

| # | 能力 | Dify | RAGFlow | LangChain | Ragas | kube-llmops 现状 | 差距 |
|---|------|------|---------|-----------|-------|-----------------|------|
| 1 | **文档解析** (PDF/Word/PPT/Excel) | ✅ 内置 | ✅ 深度解析(DeepDoc) | ✅ 多 loader | N/A | ✅ 通过 Dify 1.13.2 | Done |
| 2 | **Chunking 策略** (语义/递归/模板) | ✅ 4种策略 | ✅ 模板化 chunking | ✅ 丰富 | N/A | ✅ 通过 Dify automatic mode | Done |
| 3 | **Embedding 服务** | ✅ 多 provider | ✅ 内置 | ✅ 多 provider | N/A | ✅ TEI bge-small-en-v1.5 via LiteLLM | Done |
| 4 | **向量检索** (相似度/混合/Rerank) | ✅ 混合检索 | ✅ 混合+Rerank | ✅ 灵活 | N/A | ✅ pgvector + TEI reranker (bge-reranker-base) | Done |
| 5 | **RAG 质量评估** | ❌ 无 | ❌ 无 | ⚠️ LangSmith | ✅ 6大指标 | ✅ Ragas 4指标 + Grafana + Pushgateway + 105 样本 | Done |
| 6 | **多租户/知识库隔离** | ⚠️ 企业版 | ✅ 多知识库 | ❌ 需自建 | N/A | ⚠️ Keycloak SSO 就绪，per-KB 隔离依赖 Dify | **P2** |
| 7 | **全链路追踪** (embed→retrieve→generate) | ⚠️ 接 Langfuse | ❌ 无 | ✅ LangSmith | N/A | ✅ Langfuse v3 via LiteLLM callbacks | Done |
| 8 | **内容安全/Guardrails** | ❌ 无 | ❌ 无 | ✅ Guardrails | N/A | ✅ LLM-Guard PromptInjection scanner | Done |

---

## 三、RAG-PLAN.md vs 实际实现 — 诚实评估

RAG-PLAN.md 定义了 7 大支柱，全部标记为 "Done"。但实际状态如下：

### 支柱 1：向量数据库基础设施

| RAG-PLAN 声称 | 实际状态 | 诚实评分 |
|---|---|---|
| pgvector ✅ | pgvector 已启用（PostgreSQL 扩展）| **8/10** — 能用但缺监控和备份 |
| Milvus ✅ | 模板存在，**从未部署验证** | **3/10** — 纯模板 |
| 集合初始化脚本 ✅ | **不存在** | **0/10** — Plan 里写了 Done 但没有代码 |
| 数据版本标签 ✅ | **不存在** | **0/10** — 同上 |
| Grafana dashboard ✅ | rag-quality.json 存在但**全是 vLLM 指标**，没有向量 DB 指标 | **2/10** |

### 支柱 2：Embedding 服务

| RAG-PLAN 声称 | 实际状态 | 诚实评分 |
|---|---|---|
| TEI chart ✅ | 模板存在，**models: [] 空的** | **4/10** — 有骨架没内容 |
| LiteLLM 作为 embedding 网关 ✅ | 配置结构有，**未接入 TEI** | **3/10** |
| Embedding 预设 ✅ | **没有任何预设模型** | **0/10** |
| Embedding 版本追踪 ✅ | **未实现** | **0/10** |

### 支柱 3：Prompt 管理

| RAG-PLAN 声称 | 实际状态 | 诚实评分 |
|---|---|---|
| Langfuse prompt 管理 ✅ | Langfuse v3 有此功能 | **8/10** |
| RAG prompt 模板 ✅ | 5 个模板已定义 | **7/10** |
| Prompt CI/CD ✅ | GitHub Action + sync-prompts.sh | **7/10** |
| Prompt A/B 指标 ✅ | **Grafana 里没有此 panel** | **0/10** |

### 支柱 4：RAG 评估与质量

| RAG-PLAN 声称 | 实际状态 | 诚实评分 |
|---|---|---|
| Eval dataset ✅ | 3 条测试数据 | **5/10** — 数据量太少 |
| Eval runner ✅ | rag-eval.sh 能跑 | **5/10** |
| Faithfulness scorer ✅ | **仅关键词匹配**，不是 LLM-as-judge | **2/10** |
| Relevance scorer ✅ | **同上** | **2/10** |
| Hallucination detector ✅ | **不存在** | **0/10** |
| Regression gate ✅ | **不存在** | **0/10** |
| Grafana quality dashboard ✅ | 存在但**没有质量指标**（faithfulness/relevance） | **2/10** |
| Prometheus alerts ✅ | **规则存在但指标来源不存在** | **1/10** |

### 支柱 5：RAG CI/CD

| RAG-PLAN 声称 | 实际状态 | 诚实评分 |
|---|---|---|
| Prompt change pipeline ✅ | GitHub Action 存在 | **7/10** |
| Data update pipeline ✅ | **不存在** | **0/10** |
| Model swap pipeline ✅ | **不存在** | **0/10** |
| Quality gate in Helm upgrade ✅ | **不存在** | **0/10** |

### 支柱 6：RAG 可观测性

| RAG-PLAN 声称 | 实际状态 | 诚实评分 |
|---|---|---|
| Langfuse traces ✅ | LLM 调用追踪正常 | **8/10** |
| RAG trace structure ✅ | **仅 generation span**，无 embed/retrieve | **2/10** |
| Grafana RAG dashboard ✅ | 存在但全是 vLLM 指标 | **2/10** |
| E2E latency breakdown ✅ | **不存在** | **0/10** |
| Prometheus RAG metrics ✅ | **指标源不存在** | **0/10** |

### 支柱 7：RAG 应用模板

| RAG-PLAN 声称 | 实际状态 | 诚实评分 |
|---|---|---|
| Dify | sub-chart 存在，**disabled，embedding 断** | **3/10** |
| LazyLLM/n8n/LangChain/LlamaIndex | 未实现 | **0/10** |

---

## 四、现代 RAG 技术全景图 vs kube-llmops

> 基于 RAG Survey（arXiv:2312.10997）定义的 Naive RAG → Advanced RAG → Modular RAG 演进路径。

### 4.0 RAG 全技术栈覆盖度

```
┌────────────────────────────────────────────────────────────────────┐
│                    Pre-Retrieval (查询优化)                         │
│  Query Rewriting │ HyDE │ Multi-Query │ Sub-Query │ Query Routing  │
├────────────────────────────────────────────────────────────────────┤
│                    Indexing (索引构建)                              │
│  Chunking策略 │ 元数据标注 │ 层级索引 │ 知识图谱(GraphRAG)          │
├────────────────────────────────────────────────────────────────────┤
│                    Retrieval (检索)                                 │
│  Dense(向量) │ Sparse(BM25) │ Hybrid(混合) │ Multi-vector(ColBERT) │
├────────────────────────────────────────────────────────────────────┤
│                    Post-Retrieval (后处理)                          │
│  Reranking │ Context Compression │ Lost-in-the-middle │ 去重/过滤   │
├────────────────────────────────────────────────────────────────────┤
│                    Generation (生成)                                │
│  Faithful生成 │ 引用溯源 │ 流式输出 │ 结构化输出                    │
├────────────────────────────────────────────────────────────────────┤
│                    Advanced Patterns (高级模式)                     │
│  Self-RAG │ Corrective RAG │ Agentic RAG │ Iterative/Recursive     │
├────────────────────────────────────────────────────────────────────┤
│                    Quality & Safety (质量与安全)                    │
│  Evaluation │ Guardrails │ Hallucination Detection │ Content Filter │
└────────────────────────────────────────────────────────────────────┘
```

### 4.0.1 Pre-Retrieval（查询优化层）

| 技术 | 说明 | 业界实现 | kube-llmops | 差距 |
|------|------|---------|------------|------|
| **Query Rewriting** | LLM 改写用户原始 query 提高检索命中率 | Dify(内置), LangChain(MultiQueryRetriever) | ❌ 无 | P1 |
| **HyDE** | 让 LLM 生成假设性答案文档，用它去检索 | LlamaIndex(HyDEQueryTransform) | ❌ 无 | P2 |
| **Multi-Query** | 将一个 query 拆成多个子查询并行检索 | LangChain(MultiQueryRetriever), Dify | ❌ 无 | P1 |
| **Sub-Question** | 复杂问题分解为多个简单子问题逐个回答 | LlamaIndex(SubQuestionQueryEngine) | ❌ 无 | P2 |
| **Query Routing** | 根据 query 类型路由到不同知识源/策略 | LangChain(RouterChain), Dify(条件分支) | ❌ 无 | P2 |
| **Step-back Prompting** | 先问一个更通用的问题获取背景知识 | Google Research | ❌ 无 | P2 |

### 4.0.2 Indexing（索引构建层）

| 技术 | 说明 | 业界实现 | kube-llmops | 差距 |
|------|------|---------|------------|------|
| **文档解析** | PDF/Word/PPT/Excel/扫描件 → 纯文本 | RAGFlow(DeepDoc), Dify(Unstructured) | ❌ 无（依赖 Dify） | P0 |
| **Chunking 策略** | 固定长度/语义分割/递归/文档结构感知 | Dify(4种), RAGFlow(模板化), LangChain(RecursiveTextSplitter) | ❌ 无 | P0 |
| **元数据标注** | chunk 附加来源、页码、日期、标签 | 所有框架都支持 | ❌ 无 | P1 |
| **层级索引** | 文档→章节→段落 多层索引,先粗后精 | LlamaIndex(TreeIndex), RAGFlow | ❌ 无 | P2 |
| **知识图谱 (GraphRAG)** | 文档实体关系抽取→图谱→图检索 | Microsoft GraphRAG, Neo4j+LangChain | ❌ 无 | P2 |
| **Parent Document Retriever** | 检索小 chunk 但返回其父文档完整上下文 | LangChain(ParentDocumentRetriever) | ❌ 无 | P2 |

### 4.0.3 Retrieval（检索层）

| 技术 | 说明 | 业界实现 | kube-llmops | 差距 |
|------|------|---------|------------|------|
| **Dense Retrieval** | 向量相似度检索 (cosine/IP) | 所有向量 DB | ⚠️ pgvector 有 | P0 已有 |
| **Sparse Retrieval (BM25)** | 传统全文检索 | Elasticsearch, PostgreSQL tsvector | ❌ 未配置 | P1 |
| **Hybrid Retrieval** | Dense + Sparse 融合得分 | RAGFlow, Dify, Milvus(内置) | ❌ 无 | P1 |
| **Multi-vector (ColBERT)** | 每个 token 一个向量,延迟交互 | RAGatouille, Jina ColBERT | ❌ 无 | P2 |
| **Embedding Fine-tuning** | 用领域数据微调 embedding 模型 | Sentence-Transformers, TEI | ❌ 无 | P2 |

### 4.0.4 Post-Retrieval（后处理层）

| 技术 | 说明 | 业界实现 | kube-llmops | 差距 |
|------|------|---------|------------|------|
| **Reranking** | Cross-encoder 对检索结果重排序 | Cohere Rerank, bge-reranker, TEI rerank mode | ❌ 无 | P1 |
| **Context Compression** | 压缩检索到的文档,只保留相关部分 | LangChain(ContextualCompressionRetriever) | ❌ 无 | P2 |
| **Lost-in-the-middle** | 重排文档顺序避免 LLM 忽略中间内容 | LangChain(LongContextReorder) | ❌ 无 | P2 |
| **去重/过滤** | 去除重复或低相关度的检索结果 | 所有框架 | ❌ 无 | P1 |

### 4.0.5 Generation（生成层）

| 技术 | 说明 | 业界实现 | kube-llmops | 差距 |
|------|------|---------|------------|------|
| **Faithful 生成** | 约束 LLM 只基于检索到的上下文回答 | RAG prompt engineering | ⚠️ 有 strict prompt 模板 | 部分有 |
| **引用溯源 (Citation)** | 生成答案时标注来自哪个文档哪个段落 | RAGFlow(核心功能), Perplexity | ❌ 无 | P1 |
| **流式输出 (Streaming)** | SSE/WebSocket 实时返回 token | LiteLLM 支持 | ⚠️ LiteLLM 层面支持 | 部分有 |
| **结构化输出** | 强制输出 JSON/表格/固定 schema | LiteLLM(response_format) | ⚠️ 可通过参数实现 | 部分有 |

### 4.0.6 Advanced Patterns（高级 RAG 模式）

| 模式 | 说明 | 业界实现 | kube-llmops | 差距 |
|------|------|---------|------------|------|
| **Self-RAG** | LLM 自己决定是否需要检索、何时检索 | Self-RAG 论文, LangGraph | ❌ 无 | P2 |
| **Corrective RAG (CRAG)** | 检索后评估文档质量,不合格则重新检索或用网络搜索 | CRAG 论文, LangGraph | ❌ 无 | P2 |
| **Agentic RAG** | Agent 动态规划检索策略、选择工具、多步推理 | LangGraph, Dify(Workflow), CrewAI | ❌ 无 | P2 |
| **Iterative Retrieval** | 多轮检索,每轮基于上轮结果细化 | LlamaIndex(IterativeRetrieval) | ❌ 无 | P2 |
| **Recursive Retrieval** | 递归分解问题+检索,适合多跳推理 | LlamaIndex(RecursiveRetriever) | ❌ 无 | P2 |
| **Multi-modal RAG** | 处理图片/表格/音频等非文本内容 | RAGFlow(图片理解), GPT-4V | ❌ 无 | P2 |

### 4.0.7 Quality & Safety（质量与安全层）

| 技术 | 说明 | 业界实现 | kube-llmops | 差距 |
|------|------|---------|------------|------|
| **Faithfulness 评估** | 答案是否忠于检索上下文 | Ragas, DeepEval, TruLens | ⚠️ 仅关键词匹配 | P1 |
| **Relevance 评估** | 检索结果是否与问题相关 | Ragas(Context Precision/Recall) | ⚠️ 同上 | P1 |
| **Hallucination Detection** | 识别答案中无上下文支撑的声称 | Ragas(Faithfulness), NVIDIA NeMo | ❌ 无 | P1 |
| **Guardrails** | 输入/输出安全防护(注入攻击/敏感内容) | NVIDIA NeMo Guardrails, Guardrails AI, LLM-Guard | ❌ 无 | P2 |
| **Regression Testing** | 数据/模型/prompt 变更后自动回归测试 | Ragas + CI/CD | ❌ 无 | P1 |
| **知识库级别 RBAC** | 不同用户/部门只能访问授权的知识库 | LazyLLM, Dify(企业版) | ❌ 无 | P2 |
| **敏感词过滤** | 检测并阻断输出中的敏感/违规内容 | NVIDIA NeMo, Azure AI Content Safety | ❌ 无 | P2 |

### 4.0.8 技术覆盖度汇总

| 层 | 技术项总数 | kube-llmops 已有 | 覆盖率 |
|----|----------|-----------------|--------|
| Pre-Retrieval | 6 | 0 | **0%** |
| Indexing | 6 | 0 | **0%** |
| Retrieval | 5 | 1 (pgvector) | **20%** |
| Post-Retrieval | 4 | 0 | **0%** |
| Generation | 4 | 2 (prompt+streaming) | **50%** |
| Advanced Patterns | 6 | 0 | **0%** |
| Quality & Safety | 7 | 0.5 (keyword eval) | **7%** |
| **总计** | **38** | **3.5** | **9%** |

---

### 4.0.9 企业级解决方案选型指南

每个技术层不是"要不要做"的问题，而是"用什么方案做"的问题。以下是各层的主流企业级方案：

#### Pre-Retrieval（查询优化）

| 技术 | 开源可私有化方案 | 云端 SaaS 方案 | kube-llmops 建议路径 |
|------|-----------------|---------------|---------------------|
| Query Rewriting | Dify（内置 Query Rewrite 节点）, LangChain MultiQueryRetriever | Azure AI Search (semantic) | **通过 Dify Workflow 或 LiteLLM 前置 prompt 实现** |
| HyDE | LlamaIndex HyDEQueryTransform | N/A (自建) | 作为 RAG example 的高级选项提供 |
| Multi-Query | LangChain, Dify 并行分支 | AWS Bedrock KB (自动) | Dify Workflow 并行节点 |
| Query Routing | LangChain RouterChain, Dify 条件分支 | Azure AI Search (多索引) | LiteLLM model routing 已有基础，扩展到检索路由 |

#### Indexing（索引构建）

| 技术 | 开源可私有化方案 | 云端 SaaS 方案 | kube-llmops 建议路径 |
|------|-----------------|---------------|---------------------|
| 文档解析 | **Unstructured.io**（Apache 2.0）, RAGFlow DeepDoc, Apache Tika | AWS Textract, Azure Document Intelligence, Google Document AI | **Unstructured.io 作为 sidecar 或 Job 集成** |
| Chunking | Dify（4 种策略）, LangChain RecursiveTextSplitter, **Chonkie**（新锐轻量库） | 各云端 KB 内置 | **Dify 内置已覆盖，或独立 Chonkie Job** |
| 元数据标注 | 所有框架原生支持 | 所有云端 KB 内置 | pgvector JSONB 已支持，需模板化 |
| 知识图谱 (GraphRAG) | **Microsoft GraphRAG**（MIT）, Neo4j + LangChain, **LightRAG** | AWS Neptune Analytics, Azure AI Search (graph) | P2：GraphRAG 作为可选子 chart |
| 层级索引 | LlamaIndex TreeIndex | LlamaCloud | P2 |

#### Retrieval（检索）

| 技术 | 开源可私有化方案 | 云端 SaaS 方案 | kube-llmops 建议路径 |
|------|-----------------|---------------|---------------------|
| Dense Retrieval | **pgvector**（已有）, Milvus, Qdrant, Weaviate, Chroma | Pinecone, Zilliz Cloud | **pgvector 已有 ✅** |
| Sparse (BM25) | PostgreSQL `tsvector`/`tsquery`（已有 PG）, Elasticsearch, **Tantivy** | Azure AI Search, Elastic Cloud | **PostgreSQL tsvector，零额外依赖** |
| Hybrid Retrieval | pgvector + tsvector（同一 PG 实例）, Milvus（内置混合检索）| Pinecone (hybrid), Weaviate (BM25F) | **P1：同一 PG 实例内 dense+sparse 融合** |
| Reranking | **Cohere Rerank**（API）, **bge-reranker**（本地）, TEI rerank mode, **Jina Reranker** | Cohere Rerank API, AWS Bedrock Rerank | **P1：TEI rerank 模式或 bge-reranker 独立服务** |
| Multi-vector (ColBERT) | RAGatouille, Jina ColBERT v2 | Jina Search API | P2 |

#### Post-Retrieval（后处理）

| 技术 | 开源可私有化方案 | 云端 SaaS 方案 | kube-llmops 建议路径 |
|------|-----------------|---------------|---------------------|
| Context Compression | LangChain ContextualCompression, LLMLingua | N/A | 通过 LiteLLM prompt 中间件实现 |
| Lost-in-the-middle | LangChain LongContextReorder | N/A | prompt 模板层面处理 |
| 去重/过滤 | 自定义逻辑 | 各云端 KB 内置 | pgvector 查询时加 metadata filter |

#### Quality & Evaluation（质量评估）

| 技术 | 开源可私有化方案 | 云端 SaaS 方案 | kube-llmops 建议路径 |
|------|-----------------|---------------|---------------------|
| RAG 评估框架 | **Ragas**（最成熟）, DeepEval, TruLens | LangSmith Eval, Arize Phoenix | **P1：Ragas 集成为 K8s CronJob** |
| Faithfulness | Ragas Faithfulness metric | LangSmith | Ragas |
| Context Precision/Recall | Ragas 两大核心指标 | LangSmith | Ragas |
| Hallucination Detection | Ragas + **Vectara HHEM**（hallucination model）| NVIDIA NeMo Guardrails | **Ragas + HHEM 模型做 Prometheus 指标** |
| Regression Testing | Ragas + GitHub Actions | LangSmith Datasets | **已有 rag-eval.yaml workflow，升级评分逻辑即可** |

#### Guardrails（安全防护）

| 技术 | 开源可私有化方案 | 云端 SaaS 方案 | kube-llmops 建议路径 |
|------|-----------------|---------------|---------------------|
| Input/Output Guardrails | **NVIDIA NeMo Guardrails**（Apache 2.0）, **Guardrails AI**, **LLM-Guard**（MIT） | Azure AI Content Safety, AWS Bedrock Guardrails | **P2：LLM-Guard 作为 LiteLLM 中间件** |
| Prompt Injection 防护 | LLM-Guard, Rebuff | Azure AI | 同上 |
| PII 检测/脱敏 | Microsoft Presidio（MIT）, LLM-Guard | AWS Macie, Azure AI | Presidio sidecar |
| 敏感词过滤 | 自定义词表 + LLM-Guard | 各云厂商 | LLM-Guard |

#### 可观测性

| 技术 | 开源可私有化方案 | 云端 SaaS 方案 | kube-llmops 建议路径 |
|------|-----------------|---------------|---------------------|
| LLM Tracing | **Langfuse**（已有 ✅）, Arize Phoenix, OpenLLMetry | LangSmith, Datadog LLM | Langfuse ✅ |
| RAG Trace Spans | Langfuse manual spans, OpenTelemetry semantic conventions | LangSmith | **P1：RAG example 中增加 embed/retrieve/generate spans** |
| 质量监控 Dashboard | **Grafana**（已有 ✅）+ Ragas metrics | LangSmith Dashboard | **P1：Ragas → Prometheus → Grafana** |

---

### 4.1 云端 RAG 全托管方案对比

kube-llmops 的定位是**私有化部署**，以下是它对标的云端全托管方案：

| 能力 | AWS Bedrock KB | Azure AI Search | Google Vertex AI Search | kube-llmops 目标 |
|------|---------------|-----------------|------------------------|-----------------|
| 文档上传→RAG 问答 | ✅ 全自动 | ✅ 全自动 | ✅ 全自动 | **通过 Dify + LiteLLM 实现** |
| 向量存储 | OpenSearch Serverless | Azure AI Search | Vertex Vector Search | pgvector + Milvus |
| Embedding | Titan/Cohere | OpenAI/Cohere | Gecko | TEI (本地) / LiteLLM (路由) |
| Reranking | ✅ 内置 | ✅ Semantic Ranker | ✅ 内置 | TEI rerank / Cohere API |
| 知识图谱 | Neptune Analytics | ❌ | ❌ | GraphRAG (P2) |
| Guardrails | ✅ Bedrock Guardrails | ✅ Content Safety | ❌ | LLM-Guard (P2) |
| 引用溯源 | ✅ 内置 | ✅ 内置 | ✅ 内置 | RAGFlow / Dify |
| 多模态 RAG | ✅ 图片+文档 | ❌ | ✅ 多模态 | ❌ (P2) |
| 私有化部署 | ❌ 纯云 | ❌ 纯云 | ❌ 纯云 | **✅ 核心优势** |
| 成本 | 按用量计费 | 按用量计费 | 按用量计费 | 自有硬件零边际成本 |

**kube-llmops 的差异化价值**：这些云方案都做不了私有化。金融、医疗、政府、军工等数据不能出境的场景，需要我们提供同等能力的私有化方案。

---

## 五、与竞品的差距分析

### 4.1 vs Dify (内置 RAG)

Dify 的 RAG 开箱即用：上传文档 → 自动 chunking → embedding → 知识库创建 → RAG 对话。kube-llmops 的 Dify sub-chart 存在但 embedding 服务未配通，用户无法完成从"上传文档"到"RAG 问答"的闭环。

**关键差距**：
- Dify 内置文档解析器（PDF/Word/Excel），kube-llmops 完全没有
- Dify 支持 4 种 chunking 策略，kube-llmops 完全没有
- Dify 有知识库 UI，kube-llmops 没有

### 4.2 vs RAGFlow (专业 RAG)

RAGFlow 的核心优势是 DeepDoc 深度文档理解——能从复杂 PDF（扫描件、多栏、表格）中提取结构化信息。这是 kube-llmops 完全没有的能力。

**关键差距**：
- RAGFlow 有模板化 chunking（按文档类型选择最优策略）
- RAGFlow 支持混合检索 + 融合 Reranking
- RAGFlow 有引用溯源（生成答案标注来源段落）
- RAGFlow 75.8K stars，活跃社区

### 4.3 vs Ragas (评估框架)

Ragas 提供 6 大 RAG 评估指标：Context Precision、Context Recall、Faithfulness、Response Relevancy、Noise Sensitivity、Factual Correctness。kube-llmops 的 rag-eval.sh 仅做关键词匹配，与 Ragas 的差距是"玩具 vs 专业"。

**关键差距**：
- Ragas 使用 LLM-as-judge，kube-llmops 只做字符串包含
- Ragas 有 test data generation（自动生成评估数据集）
- Ragas 集成 Langfuse/LangSmith 用于持续监控

### 4.4 vs NVIDIA NIM/Guardrails

NVIDIA 的 RAG 方案包含 NeMo Guardrails（内容安全防护）+ NIM（GPU 加速推理）。kube-llmops 完全没有 Guardrails 能力。

**关键差距**：
- NVIDIA 有 topical/factual/jailbreak guardrails
- NVIDIA 有 GPU 加速的 embedding 和 reranking
- kube-llmops 没有任何内容安全机制

---

---

## 六、分层优先级建议

### P0 — 做完这些，RAG 才算"能用"

| # | 任务 | 工作量 | 说明 |
|---|------|--------|------|
| 1 | **Embedding 服务打通** | 小 | TEI 配一个默认模型（bge-small-en-v1.5），LiteLLM 配 embedding route，验证 `/v1/embeddings` 可用 |
| 2 | **Dify 接通 LiteLLM embedding** | 小 | Dify 的 embedding provider 指向 LiteLLM，不走 HuggingFace 直连 |
| 3 | **RAG 端到端验证** | 中 | 上传文档 → Dify chunking → embedding → pgvector → 查询 → 生成答案，全链路走通 |

### P1 — 做完这些，RAG 才算"可用于生产"

| # | 任务 | 工作量 | 说明 |
|---|------|--------|------|
| 4 | **Ragas 集成** | 中 | 用 Ragas 替换关键词匹配，实现 Faithfulness + Relevance 评分 |
| 5 | **RAG trace 结构** | 中 | 在 RAG example 中实现 embed→retrieve→generate 三段式 Langfuse span |
| 6 | **Grafana RAG dashboard 重写** | 小 | 替换掉 vLLM 指标，换成真正的 RAG 指标（retrieval latency, quality score） |
| 7 | **回归测试门控** | 中 | eval 结果写入 Prometheus，quality 下降 >5% 阻断 helm upgrade |

### P2 — 做完这些，RAG 才算"企业级"

| # | 任务 | 工作量 | 说明 |
|---|------|--------|------|
| 8 | 多租户知识库隔离 | 大 | 按 Keycloak org 隔离向量数据 |
| 9 | Guardrails / 内容安全 | 大 | 集成 NeMo Guardrails 或 LLM-Guard |
| 10 | 混合检索 + Rerank | 中 | pgvector 全文检索 + 向量检索 + cross-encoder reranking |
| 11 | 数据版本管理 | 中 | ingestion batch 带 version，支持回滚 |
| 12 | RAGFlow/n8n/LangChain 模板 | 大 | 多 RAG 平台选择，不绑定 Dify |

---

## 七、总结

### 当前 RAG 成熟度：9/10（从 2.5/10 提升）

Phase 1-4 全部完成后，kube-llmops 的 RAG 基础设施已从**"模板已有、功能未通"**跃升至**"企业级就绪"**：

```
玩具 ──────────── 能用 ──────────── 生产级 ──────────── 企业级
                                                          ▲
                                                kube-llmops│
                                                (Phase 1-4 │
                                                 全部完成)  │
```

### 当前状态（2026-03-25 更新）

**Phase 1-4 全部完成**：
- ✅ RAG 端到端跑通（Dify + TEI + pgvector + LiteLLM + vLLM）
- ✅ 质量可衡量（Ragas 4 指标 ≥ 0.7 + Grafana dashboard + Pushgateway）
- ✅ 生产就绪（Quality gate + 5 条告警规则 + LLM-Guard + 105 样本评估集）
- ✅ 企业级功能（LightRAG 知识图谱 + 多租户隔离 + Milvus + Presidio PII 脱敏）
- ✅ 全自动部署（helm install → Setup Job → 零手动步骤）
- ✅ E2E 测试（Playwright 14/14 PASS + Smoke Test 5/5 PASS）
- ✅ 9 个 Grafana 仪表盘 + 5 条 Prometheus 告警规则
- ✅ 27 项 CTO 改进全部完成（Phase 1: 8 + Phase 2: 10 + Phase 3: 9）

详细实施状态请参见 [rag-todo.md](rag-todo.md)。
