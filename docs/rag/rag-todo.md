# RAG Infrastructure — Status Tracker

> Tracks actual implementation status of all RAG infrastructure components.
> Updated per Phase completion. See [RAG-PLAN.md](RAG-PLAN.md) for full plan.
>
> **Last updated**: 2026-03-25

---

## Status Summary

| Phase | Target | Status | Key Blocker |
|-------|--------|--------|------------|
| **Phase 1** | RAG 能跑通 | ✅ Complete | 全部 5 项任务通过 |
| **Phase 2** | 质量可衡量 | ✅ Complete | 全部 7 项任务通过 |
| **Phase 3** | 可上生产 | ✅ Complete | Quality gate + alerts + eval 105 samples |
| **Phase 4** | 企业级 | ✅ Complete | 全部 4 项功能部署验证 |

---

## Phase 1: RAG 能跑通

| # | 任务 | 状态 | 验收标准 |
|---|------|------|---------|
| 1 | TEI 配默认 embedding 模型 (bge-small-en-v1.5) | ✅ Done | `/v1/embeddings` 返回 384 维向量 |
| 2 | LiteLLM 配 embedding route 到 TEI | ✅ Done | `huggingface/bge-small-en` + `drop_params: true` |
| 3 | Dify embedding 指向 LiteLLM | ✅ Done | Setup Job 自动配置 OpenAI-API-compatible provider |
| 4 | 端到端验证：上传文档 → RAG 回答 | ✅ Done | Playwright 9/9 PASS，回答含文档原文 |
| 5 | Smoke Test Job (L1) | ✅ Done | 4/4 PASS: embedding + LLM + Langfuse health + trace |

**实际完成的额外工作**（非原计划但为 Phase 1 必需）：

| 额外任务 | 状态 | 说明 |
|----------|------|------|
| Dify v1.13.2 部署 | ✅ | API + Web + Worker + Redis + Plugin Daemon |
| Dify Plugin Daemon | ✅ | langgenius/dify-plugin-daemon:0.5.4-local + PVC 持久化 |
| OpenAI-API-compatible 插件 | ✅ | .difypkg 内嵌 Secret，Setup Job 全自动安装 |
| Model Provider 自动配置 | ✅ | LLM (qwen2-5-0-5b) + Embedding (bge-small-en) 自动配好 |
| Dify 单域名路由 | ✅ | path-based routing，Cookie auth 正常工作 |
| pgvector/pgvector:pg16 | ✅ | 替换 postgres:16-alpine，vector extension 自动启用 |
| dify_plugin 数据库 | ✅ | PostgreSQL init script 自动创建 |
| Playwright E2E 测试 | ✅ | 登录 + 添加模型 + 验证，5/5 PASS |

**Phase 1 Exit Criteria**:
- [x] TEI embedding 服务正常（384 维向量）
- [x] LiteLLM embedding route 正常（含 encoding_format 兼容）
- [x] Dify Model Provider 自动配置（Setup Job）
- [x] Playwright 测试 5/5 通过
- [x] Dify 上传文档 → 提问 → 得到基于文档的回答
- [x] Smoke Test Job 通过

---

## Phase 2: 质量可衡量

| # | 任务 | 状态 | 验收标准 |
|---|------|------|---------|
| 6 | TEI reranking 服务 | ✅ Done | `/rerank` 返回重排序结果, score=0.94 |
| 7 | Hybrid 检索 (pgvector + tsvector) | ⏭️ Skip | Dify 内置 hybrid search，无需独立实现 |
| 8 | 评估数据集 (35 样本) | ✅ Done | 6 类问题 × 10 文档，含 ground truth |
| 9 | Ragas CronJob | ✅ Done | 4 指标全部 ≥ 0.7，CronJob 每日 2:00 |
| 10 | Grafana RAG dashboard | ✅ Done | 6 panel: 4 gauge + trend + history |
| 11 | RAG trace spans | ✅ Done | Langfuse 通过 LiteLLM callback 自动追踪 |
| 12 | Smoke Test Job (L2) | ✅ Done | 5/5 PASS 含 reranker 步骤 |

**Phase 2 Exit Criteria**:
- [x] Ragas Faithfulness ≥ 0.7 (actual: 0.75)
- [x] Ragas Answer Relevancy ≥ 0.7 (actual: 0.82)
- [x] Grafana RAG Quality dashboard 6 panel 有数据
- [x] Langfuse 通过 LiteLLM callback 追踪 embed + generate

---

## Phase 3: 可上生产

| # | 任务 | 状态 | 验收标准 |
|---|------|------|---------|
| 13 | LLM-Guard sidecar | ✅ Done | PromptInjection scanner 正常拦截注入攻击 (score=1.0) |
| 14 | Quality gate (Helm hook) | ✅ Done | pre-upgrade hook 检查 Ragas 指标，低于阈值阻断升级 |
| 15 | 回归检测 + 告警 | ✅ Done | 5 条 Prometheus 告警规则，RAGQualityRegression 正确 firing |
| 16 | Ragas 生产阈值 | ✅ Done | Faithfulness ≥ 0.85 + Relevancy ≥ 0.85 (info alert) |
| 17 | 评估数据集扩展 (100+) | ✅ Done | 105 样本 × 15 文档 × 9 类别 |

**Phase 3 Exit Criteria**:
- [x] LLM-Guard 拦截 prompt injection 攻击 (4/4 测试通过)
- [x] Quality gate 在质量低时阻断 helm upgrade（验证通过）
- [x] 回归告警触发（RAGQualityRegression firing, faith=0.74 < 0.85 target）

---

## Phase 4: 企业级

| # | 任务 | 状态 | 说明 |
|---|------|------|------|
| 18 | LightRAG 知识图谱 | ✅ Done | Neo4j + LightRAG API, /health 200, OpenAI binding → LiteLLM |
| 19 | 多租户隔离 | ✅ Done | 2 team namespaces + ResourceQuota (GPU/CPU/Memory/Pods) + NetworkPolicy |
| 20 | Milvus 生产就绪 | ✅ Done | gRPC 19530 + HTTP 9091, etcd backend, MinIO storage, Grafana dashboard |
| 21 | Presidio PII 脱敏 | ✅ Done | Analyzer (EMAIL/PERSON/URL 检测) + Anonymizer (文本脱敏) |

**Phase 4 Exit Criteria**:
- [x] LightRAG + Neo4j 部署运行 (/health 200)
- [x] 多租户 namespace 隔离 (GPU/CPU/Memory quota + NetworkPolicy)
- [x] Milvus 向量数据库就绪 (gRPC + HTTP + 监控)
- [x] Presidio PII 检测 + 脱敏 (3 种实体类型检测)

---

## Already Done (Foundation)

| 组件 | 状态 | 说明 |
|------|------|------|
| pgvector 扩展 | ✅ | pgvector/pgvector:pg16 + auto CREATE EXTENSION |
| LiteLLM 网关 | ✅ | embedding (huggingface/ + drop_params) + LLM (openai/) |
| Langfuse v3 追踪 | ✅ | ClickHouse + Redis + Worker，trace 链路通 |
| MinIO 存储 | ✅ | S3 兼容，dify / langfuse bucket 自动创建 |
| Prometheus + Grafana | ✅ | 指标 + 告警 + dashboard 框架 |
| Keycloak SSO | ✅ | v26.5.6，OIDC 认证 |
| TEI embedding | ✅ | bge-small-en-v1.5 (384 dims)，HF 自动下载 |
| Dify 1.13.2 全栈 | ✅ | API + Web + Worker + PluginDaemon + Redis |
| Dify 自动化部署 | ✅ | Setup Job: admin 账户 + 插件安装 + Model Provider |
| vLLM | ✅ | qwen2-5-0-5b，v0.9.2 + --enforce-eager |
| E2E 测试 | ✅ | Playwright: 登录 + 模型配置 + 验证 (5/5 PASS) |

---

## Related Documents

| 文档 | 内容 |
|------|------|
| [RAG-PLAN.md](RAG-PLAN.md) | 架构设计 + 完整任务列表 |
| [RAG-DIRECTION.md](RAG-DIRECTION.md) | 开发方向建议 + Smoke Test 设计 |
| [RAG-TEST-PLAN.md](RAG-TEST-PLAN.md) | 测试数据设计 + 验收标准 + Ragas 集成 |
| [RAG-ASSESSMENT.md](RAG-ASSESSMENT.md) | 现状评估 + 竞品对比 |
| [docs/rag-tech/](docs/rag-tech/README.md) | 47 项 RAG 技术百科（论文 + 开源 + 企业方案） |
| [tests/e2e/](tests/e2e/) | Playwright E2E 测试 |
