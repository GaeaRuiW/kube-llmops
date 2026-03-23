# RAG Infrastructure — Status Tracker

> Tracks actual implementation status of all RAG infrastructure components.
> Updated per Phase completion. See [RAG-PLAN.md](RAG-PLAN.md) for full plan.
>
> **Last updated**: 2026-03-23

---

## Status Summary

| Phase | Target | Status | Key Blocker |
|-------|--------|--------|------------|
| **Phase 1** | RAG 能跑通 | 🟡 In progress | 端到端 RAG 测试 |
| **Phase 2** | 质量可衡量 | ❌ Not started | 依赖 Phase 1 |
| **Phase 3** | 可上生产 | ❌ Not started | 依赖 Phase 2 |
| **Phase 4** | 企业级 | ❌ Not started | 按需 |

---

## Phase 1: RAG 能跑通

| # | 任务 | 状态 | 验收标准 |
|---|------|------|---------|
| 1 | TEI 配默认 embedding 模型 (bge-small-en-v1.5) | ✅ Done | `/v1/embeddings` 返回 384 维向量 |
| 2 | LiteLLM 配 embedding route 到 TEI | ✅ Done | `huggingface/bge-small-en` + `drop_params: true` |
| 3 | Dify embedding 指向 LiteLLM | ✅ Done | Setup Job 自动配置 OpenAI-API-compatible provider |
| 4 | 端到端验证：上传文档 → RAG 回答 | 🟡 Testing | Playwright E2E 测试中 |
| 5 | Smoke Test Job (L1) | 🟡 Needs fix | rag-eval sub-chart 需适配新环境 |

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
- [ ] Dify 上传文档 → 提问 → 得到基于文档的回答
- [ ] Smoke Test Job 通过

---

## Phase 2: 质量可衡量

| # | 任务 | 状态 | 验收标准 |
|---|------|------|---------|
| 6 | TEI reranking 服务 | ❌ | `/rerank` 返回重排序结果 |
| 7 | Hybrid 检索 (pgvector + tsvector) | ❌ | SQL 返回 dense + sparse 双分数 |
| 8 | 评估数据集 (35 样本) | ❌ | 6 类问题，人工验证的标准答案 |
| 9 | Ragas CronJob | ❌ | 5 个指标每日计算，推 Prometheus |
| 10 | Grafana RAG dashboard | ❌ | 6 个 panel 全有数据 |
| 11 | RAG trace spans | ❌ | Langfuse 显示 embed→retrieve→rerank→generate |
| 12 | Smoke Test Job (L2) | ❌ | 包含 rerank + hybrid 验证 |

**Phase 2 Exit Criteria**:
- [ ] Ragas Faithfulness ≥ 0.7
- [ ] Ragas Answer Relevancy ≥ 0.7
- [ ] Grafana RAG Quality dashboard 6 panel 有数据
- [ ] Langfuse 中看到 4 段式 RAG trace span

---

## Phase 3: 可上生产

| # | 任务 | 状态 | 验收标准 |
|---|------|------|---------|
| 13 | LLM-Guard sidecar | ❌ | prompt injection 被拦截 |
| 14 | Quality gate (Helm hook) | ❌ | 质量不达标时 helm upgrade 被阻断 |
| 15 | 回归检测 + 告警 | ❌ | 质量下降 >5% 时 Prometheus 告警 |
| 16 | Ragas 生产阈值 | ❌ | Faithfulness ≥ 0.85, Hallucination ≤ 0.15 |
| 17 | 评估数据集扩展 (100+) | ❌ | 加入 Langfuse 导出的真实 query |

**Phase 3 Exit Criteria**:
- [ ] LLM-Guard 阻断测试攻击
- [ ] Quality gate 在质量低时阻断 helm upgrade
- [ ] 回归告警触发

---

## Phase 4: 企业级（按需）

| # | 任务 | 状态 | 说明 |
|---|------|------|------|
| 18 | LightRAG 知识图谱 | ❌ | 可选子 chart |
| 19 | 多租户隔离 | ❌ | pgvector metadata filter + Keycloak org |
| 20 | Milvus 生产就绪 | ❌ | 验证 + 监控 |
| 21 | Presidio PII 脱敏 | ❌ | sidecar 部署 |

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
