# RAG Infrastructure — Status Tracker

> Tracks actual implementation status of all RAG infrastructure components.
> Updated per Phase completion. See [RAG-PLAN.md](RAG-PLAN.md) for full plan.

---

## Status Summary

| Phase | Target | Status | Key Blocker |
|-------|--------|--------|------------|
| **Phase 1** | RAG 能跑通 | ❌ Not started | TEI embedding 未配默认模型 |
| **Phase 2** | 质量可衡量 | ❌ Not started | 依赖 Phase 1 |
| **Phase 3** | 可上生产 | ❌ Not started | 依赖 Phase 2 |
| **Phase 4** | 企业级 | ❌ Not started | 按需 |

---

## Phase 1: RAG 能跑通

| # | 任务 | 状态 | 验收标准 |
|---|------|------|---------|
| 1 | TEI 配默认 embedding 模型 (bge-m3) | ❌ | `/v1/embeddings` 返回 1024 维向量 |
| 2 | LiteLLM 配 embedding route 到 TEI | ❌ | 通过 LiteLLM 代理调用 embedding |
| 3 | Dify embedding 指向 LiteLLM | ❌ | Dify 不再直连 HuggingFace |
| 4 | 端到端验证：上传文档 → RAG 回答 | ❌ | 回答包含文档中的信息 |
| 5 | Smoke Test Job (L1) | ❌ | 8 个 step 全部 PASS |

**Phase 1 Exit Criteria**:
- [ ] Smoke Test Job 通过
- [ ] Dify 上传 PDF → 提问 → 得到基于文档的回答

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

以下基础设施已完成，是 RAG 开发的前置条件：

| 组件 | 状态 | 说明 |
|------|------|------|
| pgvector 扩展 | ✅ | PostgreSQL 17 with vector extension |
| LiteLLM 网关 | ✅ | OpenAI 兼容 API，embedding route 结构已有 |
| Langfuse v3 追踪 | ✅ | ClickHouse + Redis + Worker，trace 链路通 |
| MinIO 存储 | ✅ | S3 兼容，langfuse bucket 自动创建 |
| Prometheus + Grafana | ✅ | 指标 + 告警 + dashboard 框架 |
| Keycloak SSO | ✅ | OIDC 认证，多租户基础 |
| TEI 模板 | ✅ | sub-chart 骨架有，缺默认模型 |
| Dify 模板 | ✅ | sub-chart 骨架有，embedding 断 |
| Milvus 模板 | ✅ | sub-chart 骨架有，未验证 |
| Prompt 模板 | ✅ | 5 个 RAG prompt + sync 脚本 |
| Eval 脚本骨架 | ✅ | rag-eval.sh + k8s-eval-job.yaml（仅关键词匹配） |

---

## Related Documents

| 文档 | 内容 |
|------|------|
| [RAG-PLAN.md](RAG-PLAN.md) | 架构设计 + 完整任务列表 |
| [RAG-DIRECTION.md](RAG-DIRECTION.md) | 开发方向建议 + Smoke Test 设计 |
| [RAG-TEST-PLAN.md](RAG-TEST-PLAN.md) | 测试数据设计 + 验收标准 + Ragas 集成 |
| [RAG-ASSESSMENT.md](RAG-ASSESSMENT.md) | 现状评估 + 竞品对比 |
| [docs/rag-tech/](docs/rag-tech/README.md) | 47 项 RAG 技术百科（论文 + 开源 + 企业方案） |
