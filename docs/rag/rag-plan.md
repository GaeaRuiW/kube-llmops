# kube-llmops RAG Infrastructure Plan

**English** | [中文](RAG-PLAN.zh-CN.md)

> We don't build a RAG app — we provide the Kubernetes infrastructure
> for any RAG system to deploy, run, evaluate, and improve.

---

## Positioning

kube-llmops is the **private-deployment equivalent of AWS Bedrock Knowledge Bases**.

We provide infrastructure services that RAG applications (Dify, RAGFlow, LangChain, custom) consume:

```
┌──────────────────────────────────────────────────────┐
│  RAG Application (user's choice — NOT our scope)      │
│  Dify │ RAGFlow │ LangChain │ n8n │ custom           │
├──────────────────────────────────────────────────────┤
│  kube-llmops RAG Infrastructure                       │
│                                                       │
│  Embedding     Retrieval    Reranking    LLM Gateway  │
│  Service       Backend      Service      (LiteLLM)   │
│  (TEI)         (pgvector)   (TEI)                     │
│                                                       │
│  Evaluation    Guardrails   Observability  Storage    │
│  (Ragas)       (LLM-Guard)  (Langfuse)    (MinIO)    │
└──────────────────────────────────────────────────────┘
```

**What we build**: Helm charts, K8s Jobs/CronJobs, Prometheus exporters, Grafana dashboards.
**What we don't build**: Document parsers, chunking logic, RAG UIs, agent workflows.

---

## Infrastructure Components

### 1. Embedding Service

| Item | Status | Detail |
|------|--------|--------|
| TEI sub-chart | ✅ Working | `charts/tei/` deployed with bge-small-en-v1.5 (384 dims) |
| Default embedding model | ✅ Configured | `BAAI/bge-small-en-v1.5` auto-downloaded from HuggingFace |
| LiteLLM embedding route | ✅ Working | `huggingface/bge-small-en` + `drop_params: true` |
| Health check | ✅ Working | readinessProbe on TEI |
| Model preloading | ✅ Auto-download | TEI downloads from HF on first start |

### 2. Reranking Service

| Item | Status | Detail |
|------|--------|--------|
| TEI rerank mode | ❌ Not implemented | TEI supports `--model bge-reranker-v2-m3` but no chart |
| Rerank API endpoint | ❌ Not implemented | Need `/rerank` endpoint |
| LiteLLM integration | ❌ Not implemented | Route rerank requests through LiteLLM |

### 3. Vector & Retrieval Backend

| Item | Status | Detail |
|------|--------|--------|
| pgvector extension | ✅ Auto-enabled | pgvector/pgvector:pg16, init script runs `CREATE EXTENSION IF NOT EXISTS vector` |
| tsvector full-text | ❌ Not configured | PostgreSQL has it but no index/function setup |
| Hybrid search function | ❌ Not implemented | Need SQL function for dense+sparse+RRF |
| Milvus chart | ✅ Template exists | Not tested in cluster |
| pg_trgm extension | ❌ Not enabled | Needed for fuzzy text matching |

### 4. RAG Application Platform

| Item | Status | Detail |
|------|--------|--------|
| Dify sub-chart | ✅ Full stack | API + Web + Worker + PluginDaemon + Redis, v1.13.2 |
| Dify → LiteLLM embedding | ✅ Auto-configured | Setup Job installs OpenAI-API-compatible plugin + credentials |
| Dify → LiteLLM LLM | ✅ Auto-configured | Setup Job configures qwen2-5-0-5b as LLM provider |
| Dify → pgvector | ✅ Configured | Vector store set to pgvector |
| Plugin Daemon | ✅ Working | .difypkg embedded in Secret, PVC for persistence |
| Single-domain routing | ✅ Working | path-based Ingress, SameSite cookie auth works |
| End-to-end: upload → answer | ✅ Working | Playwright 9/9 PASS: KB → upload → index → chat → verify |

### 5. RAG Evaluation (Differentiator)

| Item | Status | Detail |
|------|--------|--------|
| Eval script | ✅ Ragas-based | `ragas-eval.py` with LLM-as-judge faithfulness + embedding similarity |
| Eval dataset | ✅ 105 samples | 15 docs × 9 categories with ground truth |
| K8s eval Job | ✅ CronJob | Runs daily at 2:00 AM, pushes to Pushgateway |
| Ragas integration | ✅ Working | 4 metrics: faithfulness, answer_relevancy, context_precision, context_recall |
| Ragas CronJob | ✅ Working | `kube-llmops-ragas-eval` CronJob with Prometheus export |
| Ragas metrics → Prometheus | ✅ Working | Via Pushgateway, scraped by Prometheus |
| Grafana RAG dashboard | ✅ Working | 6 panels: 4 gauges + trend + history |
| Prometheus alert rules | ✅ 5 rules | FaithfulnessLow/Critical, RelevancyLow, QualityRegression, EvalStale |
| Quality gate (Helm hook) | ✅ Working | pre-upgrade hook blocks on quality regression |

### 6. RAG Observability

| Item | Status | Detail |
|------|--------|--------|
| Langfuse LLM traces | ✅ Working | Every LiteLLM request traced via callbacks |
| RAG trace spans | ✅ Via Langfuse | LiteLLM callbacks trace embed + generate spans |
| E2E latency breakdown | ✅ Via Langfuse | Trace timeline shows per-step latency |
| Retrieval metrics | ⚠️ Partial | Via Ragas context_precision/recall metrics |
| Embedding metrics | ✅ Via Langfuse | Embedding calls traced with latency |

### 7. RAG Safety

| Item | Status | Detail |
|------|--------|--------|
| LLM-Guard sidecar | ✅ Working | PromptInjection scanner, 4/4 tests pass |
| Prompt injection defense | ✅ Working | Blocks direct + subtle injection (score=1.0) |
| PII detection | ❌ Not implemented | Presidio sidecar (Phase 4) |
| Content filtering | ✅ Partial | NoRefusal output scanner enabled |

### 8. RAG Testing

| Item | Status | Detail |
|------|--------|--------|
| Smoke Test Job (L1+L2) | ✅ 5/5 PASS | embedding + LLM + langfuse + trace + reranker |
| Eval dataset (105 samples) | ✅ Created | 15 docs, 9 categories, ground truth |
| Playwright E2E tests | ✅ 14/14 PASS | Model provider (5/5) + RAG E2E (9/9) |
| Regression detection | ✅ Working | Prometheus alert + quality gate Helm hook |

### 9. Prompt Management

| Item | Status | Detail |
|------|--------|--------|
| Langfuse prompt management | ✅ Working | Langfuse v3 native feature |
| RAG prompt templates | ✅ Created | 5 templates in examples/prompts/ |
| Prompt sync script | ✅ Working | sync-prompts.sh + GitHub Action |
| Prompt A/B metrics | ❌ Not implemented | Grafana panel by prompt version |

---

## Implementation Phases

### Phase 1: RAG Works (2-3 weeks)

**Goal**: `helm install` → upload document → RAG answer. Zero manual steps.

| # | Task | Depends on | Acceptance |
|---|------|-----------|------------|
| 1 | TEI default model (bge-m3) | - | `/v1/embeddings` returns 1024-dim vector |
| 2 | LiteLLM embedding route | #1 | Embedding through LiteLLM proxy |
| 3 | Dify embedding → LiteLLM | #2 | Dify uses LiteLLM for embedding |
| 4 | End-to-end validation | #3 | Upload PDF → ask question → get answer with context |
| 5 | Smoke Test Job (L1) | #1,#2 | All 8 steps PASS |

**Exit criteria**: Smoke Test Job passes. Dify RAG e2e demo works.

### Phase 2: RAG Quality Measurable (3-4 weeks)

**Goal**: Automated quality evaluation with Grafana visibility.

| # | Task | Depends on | Acceptance |
|---|------|-----------|------------|
| 6 | TEI reranking service | - | `/rerank` endpoint returns reordered results |
| 7 | Hybrid retrieval (pgvector+tsvector) | - | SQL returns dense+sparse scores |
| 8 | Eval dataset (35 samples) | Phase 1 | 6 categories, human-verified ground truth |
| 9 | Ragas CronJob | #8 | 5 metrics computed daily, pushed to Prometheus |
| 10 | Grafana RAG dashboard | #9 | 6 panels with real RAG data |
| 11 | RAG trace spans | Phase 1 | embed→retrieve→rerank→generate in Langfuse |
| 12 | Smoke Test Job (L2) | #6,#7 | Rerank + hybrid steps PASS |

**Exit criteria**: Ragas Faithfulness ≥ 0.7, Answer Relevancy ≥ 0.7. Dashboard has data.

### Phase 3: RAG Production Ready (3-4 weeks)

**Goal**: Safety, quality gates, production-grade monitoring.

| # | Task | Depends on | Acceptance |
|---|------|-----------|------------|
| 13 | LLM-Guard sidecar | - | Prompt injection blocked, PII detected |
| 14 | Quality gate (Helm hook) | Phase 2 | helm upgrade blocked on quality regression |
| 15 | Regression detection | #9 | Alert fires when score drops >5% |
| 16 | Ragas production thresholds | #9 | Faithfulness ≥ 0.85, Hallucination ≤ 0.15 |
| 17 | Eval dataset expansion (100+) | #8 | Ragas TestsetGenerator + real queries from Langfuse |

**Exit criteria**: LLM-Guard blocks test attack. Quality gate blocks bad upgrade.

### Phase 4: Enterprise Features (on demand)

| # | Task | Detail |
|---|------|--------|
| 18 | LightRAG knowledge graph | Optional sub-chart |
| 19 | Multi-tenant RBAC | pgvector metadata filter + Keycloak org |
| 20 | Milvus production-ready | Verify chart, add monitoring |
| 21 | Presidio PII anonymization | Sidecar deployment |

---

## What We Don't Build

| Capability | Why not | Who does it |
|-----------|---------|-------------|
| Document parsing | Application layer | Dify, RAGFlow, Unstructured.io |
| Chunking strategies | Application layer | Dify (4 strategies), LangChain |
| Query rewriting / HyDE | Application layer | Dify Workflow, LangChain |
| RAG conversation UI | Application layer | Dify, RAGFlow, custom |
| Agent / workflow orchestration | Application layer | Dify, LangGraph, n8n |
| Citation / source attribution | Application layer | RAGFlow, application |

We provide the **infrastructure** these features need:
- Parsing needs S3 → we provide MinIO
- Chunking needs embedding → we provide TEI
- Query rewriting needs retrieval → we provide pgvector + tsvector
- UI needs LLM → we provide LiteLLM + vLLM
- Citations need traces → we provide Langfuse

---

## Related Documents

- [RAG Development Direction](RAG-DIRECTION.md) — Strategic positioning and architecture
- [RAG Test Plan](RAG-TEST-PLAN.md) — Test data design, acceptance criteria, Ragas integration
- [RAG Technology Encyclopedia](docs/rag-tech/README.md) — 47 techniques with papers and implementations
- [RAG Capability Assessment](RAG-ASSESSMENT.md) — Current state vs enterprise solutions
