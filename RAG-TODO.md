# RAG 未完成事项 & 差距分析

**English** | [中文](RAG-TODO.zh-CN.md)

> 基于 LazyLLM 企业级 RAG 方案 + 我们的 RAG-PLAN.md + 实际集群状态的交叉对比。
> 分三类：已完成、未完成（可做）、差距（需要新设计）。

---

## 一、LazyLLM 企业级 RAG 提出的 6 大能力 vs kube-llmops 现状

| LazyLLM 能力 | 描述 | kube-llmops 现状 | 差距 |
|---|---|---|---|
| **1. 权限隔离** | 按部门/标签隔离知识库访问 | Keycloak SSO + NetworkPolicy 多租户模板 | **缺**：知识库级别的 RBAC（Dify 开源版不支持，需要在网关层做） |
| **2. 标签权限** | 文档级别 permission_level 过滤 | pgvector 有 metadata jsonb 字段 | **缺**：检索时 metadata filter 的集成示例 |
| **3. 算法共享** | 同一 embedding/LLM 算法服务多个知识库 | LiteLLM 统一网关 ✓ | **已有**：LiteLLM 天然支持多知识库共用同一模型 |
| **4. 召回解耦** | 知识库与 RAG 召回服务分离，多对多 | Dify 的知识库管理 ✓ | **部分**：Dify 支持，但不如 LazyLLM 灵活 |
| **5. 对话管理** | 历史对话、多用户并发、流式输出 | LiteLLM + Dify 都支持 ✓ | **已有** |
| **6. 内容安全** | 敏感词过滤、全链路加密、私有化部署 | 私有化部署 ✓，NetworkPolicy ✓ | **缺**：敏感词过滤、内容审核 |

---

## 二、RAG 应用平台集成 — 未完成清单

### 已完成
- [x] **Dify** — sub-chart 部署，接了 pgvector + MinIO + LiteLLM（但 embedding 断了）

### 未完成

| # | 平台 | 类型 | 优先级 | 工作量 | 描述 |
|---|---|---|---|---|---|
| R1 | **Dify embedding 修复** | Bug fix | P0 | 小 | 当前 Dify 无法使用 embedding（企业 SSL 阻断模型下载），需提供离线 embedding 方案或 API 配置文档 |
| R2 | **LazyLLM** | Python 框架 | P1 | 中 | K8s deployment 模板 + 配置连接 LiteLLM + pgvector 的示例项目 |
| R3 | **n8n** | Workflow | P2 | 中 | Helm sub-chart，预配置 LiteLLM HTTP 节点 |
| R4 | **Coze (open source)** | Agent 平台 | P2 | 中 | 如果开源版可用，提供集成模板 |
| R5 | **LangChain 示例** | 代码框架 | P2 | 小 | Python 示例：ingest → pgvector → query → LiteLLM → answer |
| R6 | **LlamaIndex 示例** | 代码框架 | P2 | 小 | 同上 |

---

## 三、RAG 基础设施 — 未完成清单

### Embedding 服务

| # | 项 | 状态 | 说明 |
|---|---|---|---|
| E1 | LiteLLM embedding routing 模板 | ✅ Done | `embeddingModels[]` 配置 |
| E2 | 本地 embedding 服务部署 | ❌ 未完成 | TEI/infinity 因 SSL 失败。需要：a) CA cert 注入方案，b) 离线模型加载方案，c) 外部 API fallback |
| E3 | Embedding API 集成（OpenAI/Cohere/etc） | ✅ 模板有 | `embeddingModels[].apiBase` 指向外部 API |
| E4 | Embedding 版本追踪 | ❌ 未完成 | Langfuse metadata 记录 embedding model + version |

### 向量数据库

| # | 项 | 状态 | 说明 |
|---|---|---|---|
| V1 | pgvector | ✅ Done | 0.8.2 已启用 |
| V2 | Milvus | ✅ 模板有 | Helm chart 存在，未部署验证 |
| V3 | 集合初始化脚本 | ❌ 未完成 | 首次部署自动创建 collection + index |
| V4 | 数据版本标签 | ❌ 未完成 | ingestion batch 带 version metadata |
| V5 | 向量 DB 监控 | ❌ 未完成 | Grafana dashboard: query latency, index size |

### 质量评估

| # | 项 | 状态 | 说明 |
|---|---|---|---|
| Q1 | Eval 脚本 | ✅ Done | `rag-eval.sh` + eval dataset |
| Q2 | K8s Job 模板 | ✅ Done | `k8s-eval-job.yaml` |
| Q3 | LLM-as-judge scoring | ❌ 未完成 | 用 LLM 评估回答的 faithfulness/relevance（prompt 模板 `rag-eval-judge` 已有，runner 未实现） |
| Q4 | Ragas/DeepEval 集成 | ❌ 未完成 | 专业 RAG eval 框架集成 |
| Q5 | 回归测试门控 | ❌ 未完成 | 数据更新 → 自动 eval → 质量下降则阻断部署 |

### Prompt 管理

| # | 项 | 状态 | 说明 |
|---|---|---|---|
| P1 | Prompt 模板 | ✅ Done | 5 个模板 (default/strict/conversational/chinese/judge) |
| P2 | Prompt → Langfuse 同步 | ✅ Done | `sync-prompts.sh` |
| P3 | Prompt CI/CD | ✅ Done | `prompt-sync.yaml` workflow |
| P4 | Prompt A/B 测试指标 | ❌ 未完成 | Grafana panel: 按 prompt version 对比质量 |

### CI/CD

| # | 项 | 状态 | 说明 |
|---|---|---|---|
| C1 | RAG eval workflow | ✅ Done | `rag-eval.yaml` |
| C2 | Prompt sync workflow | ✅ Done | `prompt-sync.yaml` |
| C3 | 数据更新触发 eval | ❌ 未完成 | 新文档 ingest → 自动跑 eval |
| C4 | 模型切换触发 eval | ❌ 未完成 | Helm upgrade model → pre-hook eval |

### 可观测性

| # | 项 | 状态 | 说明 |
|---|---|---|---|
| O1 | RAG Grafana dashboard | ✅ Done | `rag-quality.json` |
| O2 | RAG Prometheus alerts | ✅ Done | 6 条 rules |
| O3 | RAG trace 结构 (embed→retrieve→generate spans) | ❌ 未完成 | 当前只有 LLM generation span，缺 embedding + retrieval 独立 span |
| O4 | E2E latency breakdown | ❌ 未完成 | Langfuse 里看 "这 3s 花在哪了" |

### 企业级安全（来自 LazyLLM 文章的启发）

| # | 项 | 状态 | 说明 |
|---|---|---|---|
| S1 | 知识库级别 RBAC | ❌ 未完成 | 不同团队只能访问自己的知识库 |
| S2 | 敏感词过滤 | ❌ 全新 | 问答过程中自动检测+阻断敏感内容输出 |
| S3 | 文档级权限标签 | ❌ 全新 | 上传文档时标记 permission_level，检索时过滤 |
| S4 | 全链路加密 | ❌ 全新 | 文档上传/存储/传输/生成全程加密 |

---

## 四、优先级排序

### P0 — 不做完 RAG 就不能用
| # | 说明 |
|---|---|
| R1 | Dify embedding 修复（或提供替代方案） |
| E2 | 本地 embedding 服务可用 |

### P1 — 核心差异化
| # | 说明 |
|---|---|
| R2 | LazyLLM 集成模板 |
| Q3 | LLM-as-judge scoring |
| O3 | RAG trace 结构 (embed/retrieve/generate spans) |
| S1 | 知识库级别 RBAC |
| S2 | 敏感词过滤 |

### P2 — 生态完善
| # | 说明 |
|---|---|
| R3-R6 | n8n, Coze, LangChain, LlamaIndex 模板 |
| V3-V5 | 向量 DB 初始化、版本、监控 |
| Q4-Q5 | Ragas 集成、回归门控 |
| C3-C4 | 数据/模型更新触发 eval |
| P4 | Prompt A/B 指标 |
| S3-S4 | 文档权限标签、全链路加密 |
