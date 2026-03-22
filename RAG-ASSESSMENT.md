# kube-llmops RAG 能力评估报告

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
| 1 | **文档解析** (PDF/Word/PPT/Excel) | ✅ 内置 | ✅ 深度解析(DeepDoc) | ✅ 多 loader | N/A | ❌ 无（依赖 Dify） | **P0** |
| 2 | **Chunking 策略** (语义/递归/模板) | ✅ 4种策略 | ✅ 模板化 chunking | ✅ 丰富 | N/A | ❌ 无 | **P0** |
| 3 | **Embedding 服务** | ✅ 多 provider | ✅ 内置 | ✅ 多 provider | N/A | ⚠️ TEI 模板有但未集成 | **P0** |
| 4 | **向量检索** (相似度/混合/Rerank) | ✅ 混合检索 | ✅ 混合+Rerank | ✅ 灵活 | N/A | ⚠️ pgvector 仅相似度 | **P1** |
| 5 | **RAG 质量评估** | ❌ 无 | ❌ 无 | ⚠️ LangSmith | ✅ 6大指标 | ⚠️ 脚本有但仅关键词匹配 | **P1** |
| 6 | **多租户/知识库隔离** | ⚠️ 企业版 | ✅ 多知识库 | ❌ 需自建 | N/A | ❌ 无 | **P1** |
| 7 | **全链路追踪** (embed→retrieve→generate) | ⚠️ 接 Langfuse | ❌ 无 | ✅ LangSmith | N/A | ⚠️ 仅 LLM generation span | **P1** |
| 8 | **内容安全/Guardrails** | ❌ 无 | ❌ 无 | ✅ Guardrails | N/A | ❌ 无 | **P2** |

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

## 四、与竞品的差距分析

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

## 五、RAG-PLAN vs RAG-TODO 不一致

RAG-PLAN.md 将几乎所有项目标为 **Done**，但 RAG-TODO.md 诚实地列出了大量未完成项。两份文档互相矛盾。

| 项目 | RAG-PLAN 状态 | RAG-TODO 状态 | 实际 |
|------|-------------|-------------|------|
| 本地 embedding 部署 | Done | ❌ 未完成 | ❌ |
| Embedding 版本追踪 | Done | ❌ 未完成 | ❌ |
| 集合初始化脚本 | Done | ❌ 未完成 | ❌ |
| 数据版本标签 | Done | ❌ 未完成 | ❌ |
| 向量 DB 监控 | Done | ❌ 未完成 | ❌ |
| LLM-as-judge | Done | ❌ 未完成 | ❌ |
| Ragas/DeepEval 集成 | Done | ❌ 未完成 | ❌ |
| 回归测试门控 | Done | ❌ 未完成 | ❌ |
| RAG trace 结构 | Done | ❌ 未完成 | ❌ |
| E2E latency breakdown | Done | ❌ 未完成 | ❌ |
| Prompt A/B 指标 | Done | ❌ 未完成 | ❌ |

**结论**：RAG-PLAN.md 需要修正为实际状态。

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

### 当前 RAG 成熟度：2.5/10

kube-llmops 的 RAG 基础设施处于**"模板已有、功能未通"**的阶段：

- **有的**：pgvector 扩展、Dify 子 chart 骨架、TEI 模板、eval 脚本、prompt 模板
- **没通的**：embedding 未接入、Dify 不能用、评估只做关键词匹配、dashboard 显示错误指标
- **完全没有的**：文档解析、chunking、reranking、guardrails、多租户、数据版本

### 对标定位

```
玩具 ──────────── 能用 ──────────── 生产级 ──────────── 企业级
  ▲                                                      ▲
  │ kube-llmops                                    Dify / RAGFlow
  │ 当前状态                                        (开箱即用)
```

### 核心问题

**RAG-PLAN.md 的 "Done" 标记严重失实**。大量标记为 Done 的功能实际上不存在或不能工作。建议：

1. 修正 RAG-PLAN.md，将实际未完成的项标为 TODO
2. 先做 P0（3 项），让 RAG 端到端跑通
3. 再做 P1（4 项），让评估和监控到位
4. P2 是锦上添花，不阻塞
