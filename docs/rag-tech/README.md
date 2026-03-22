# RAG 技术全景百科

> 基于 RAG Survey（Gao et al., arXiv:2312.10997）的 Naive RAG → Advanced RAG → Modular RAG 演进框架，
> 覆盖 38 项核心技术的方法论、论文出处、开源实现与企业级方案。

---

## 文档索引

| # | 层 | 文件 | 技术数 | 行数 |
|---|---|------|-------|------|
| 1 | **Pre-Retrieval（查询优化）** | [01-pre-retrieval.md](01-pre-retrieval.md) | 6 | 581 |
| 2 | **Indexing（索引构建）** | [02-indexing.md](02-indexing.md) | 6 | 776 |
| 3 | **Retrieval（检索）** | [03-retrieval.md](03-retrieval.md) | 6 | 1065 |
| 4 | **Post-Retrieval + Generation + Advanced Patterns** | [04-post-retrieval-and-generation.md](04-post-retrieval-and-generation.md) | 16 | 1059 |
| 5 | **Quality & Safety（质量评估与安全防护）** | [05-quality-and-safety.md](05-quality-and-safety.md) | 13 | 1636 |
| | **合计** | | **47** | **5117** |

---

## 技术全景图

```
用户查询
  │
  ▼
┌─────────────────────────────────────────────────┐
│  Layer 1: Pre-Retrieval (查询优化)               │  01-pre-retrieval.md
│  Query Rewriting │ HyDE │ Multi-Query │          │
│  Sub-Question │ Query Routing │ Step-back        │
└─────────────────────┬───────────────────────────┘
                      ▼
┌─────────────────────────────────────────────────┐
│  Layer 2: Indexing (索引构建)                     │  02-indexing.md
│  Document Parsing │ Chunking │ Metadata │        │
│  Hierarchical Index │ GraphRAG │ Parent Doc       │
└─────────────────────┬───────────────────────────┘
                      ▼
┌─────────────────────────────────────────────────┐
│  Layer 3: Retrieval (检索)                       │  03-retrieval.md
│  Dense │ Sparse(BM25) │ Hybrid │ Reranking │     │
│  Embedding Fine-tune │ Multi-vector(ColBERT)     │
└─────────────────────┬───────────────────────────┘
                      ▼
┌─────────────────────────────────────────────────┐
│  Layer 4: Post-Retrieval (后处理)                │  04-post-retrieval-
│  Context Compression │ Lost-in-Middle │ 去重      │  and-generation.md
├─────────────────────────────────────────────────┤
│  Layer 5: Generation (生成)                      │
│  Faithful │ Citation │ Streaming │ Structured     │
├─────────────────────────────────────────────────┤
│  Layer 6: Advanced Patterns (高级模式)            │
│  Self-RAG │ CRAG │ Agentic RAG │ Iterative │     │
│  Recursive │ Multi-modal                         │
└─────────────────────┬───────────────────────────┘
                      ▼
┌─────────────────────────────────────────────────┐
│  Layer 7: Quality & Safety (质量与安全)           │  05-quality-and-safety.md
│  Ragas │ DeepEval │ TruLens │ HHEM │ LLM-Judge │ │
│  NeMo Guardrails │ LLM-Guard │ Presidio │ RBAC   │
└─────────────────────────────────────────────────┘
```

---

## 每篇文档的结构

每个技术点统一包含 7 个维度：

1. **What it is** — 原理说明
2. **Method variants** — 所有已知的方法变体
3. **Key papers** — arXiv 论文链接
4. **Open source implementations** — GitHub 仓库
5. **Enterprise products** — 商业方案
6. **When to use** — 使用场景指导
7. **kube-llmops integration path** — 集成路径建议

---

## 核心论文引用

| 论文 | arXiv | 贡献 |
|------|-------|------|
| RAG Survey | [2312.10997](https://arxiv.org/abs/2312.10997) | RAG 全景综述（Naive/Advanced/Modular） |
| Rewrite-Retrieve-Read | [2305.14283](https://arxiv.org/abs/2305.14283) | Query Rewriting 框架 |
| HyDE | [2212.10496](https://arxiv.org/abs/2212.10496) | 假设性文档嵌入 |
| IRCoT | [2212.10509](https://arxiv.org/abs/2212.10509) | 交替检索+思维链 |
| Step-back Prompting | [2310.06117](https://arxiv.org/abs/2310.06117) | 回退提问策略 |
| RAPTOR | [2401.18059](https://arxiv.org/abs/2401.18059) | 递归摘要树检索 |
| GraphRAG | [2404.16130](https://arxiv.org/abs/2404.16130) | 微软知识图谱 RAG |
| LightRAG | [2410.05779](https://arxiv.org/abs/2410.05779) | 轻量级图 RAG |
| DPR | [2004.04906](https://arxiv.org/abs/2004.04906) | 稠密段落检索 |
| ColBERT | [2004.12832](https://arxiv.org/abs/2004.12832) | 延迟交互多向量检索 |
| SPLADE | [2107.05720](https://arxiv.org/abs/2107.05720) | 学习型稀疏检索 |
| LLMLingua | [2310.05736](https://arxiv.org/abs/2310.05736) | 上下文压缩 |
| Lost in the Middle | [2307.03172](https://arxiv.org/abs/2307.03172) | 长上下文注意力偏差 |
| Self-RAG | [2310.11511](https://arxiv.org/abs/2310.11511) | 自反思检索增强 |
| CRAG | [2401.15884](https://arxiv.org/abs/2401.15884) | 纠错式 RAG |
| FLARE | [2305.06983](https://arxiv.org/abs/2305.06983) | 前瞻性主动检索 |
| Ragas | [2309.15217](https://arxiv.org/abs/2309.15217) | RAG 评估框架 |
| NeMo Guardrails | [2310.10501](https://arxiv.org/abs/2310.10501) | LLM 安全防护框架 |
| Llama Guard | [2312.06674](https://arxiv.org/abs/2312.06674) | 内容安全分类模型 |
| ALCE | [2305.14627](https://arxiv.org/abs/2305.14627) | 引用溯源生成 |
| Outlines | [2307.09702](https://arxiv.org/abs/2307.09702) | 结构化输出引导 |

---

## 相关文档

- [RAG 能力评估报告](../../RAG-ASSESSMENT.md) — kube-llmops RAG 现状 vs 企业级方案对比
- [RAG 实施计划](../../RAG-PLAN.md) — 7 大支柱实施路线
- [RAG 待办清单](../../RAG-TODO.md) — 未完成项跟踪
