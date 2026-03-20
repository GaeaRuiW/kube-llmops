# kube-llmops RAG Infrastructure Plan

**English** | [中文](RAG-PLAN.zh-CN.md)

> AI Infra perspective: we don't build a RAG app — we provide the infrastructure
> to **deploy, manage, observe, and validate** any RAG system on Kubernetes.

---

## Core Philosophy

The RAG application layer (Dify, n8n, LangChain, LlamaIndex...) is the user's choice.
kube-llmops provides everything **below** the app:

```
┌─────────────────────────────────────────────────┐
│  RAG Application (user's choice)                │
│  Dify / n8n / Coze / LangChain / LlamaIndex    │
├─────────────────────────────────────────────────┤
│  kube-llmops RAG Infrastructure                 │
│                                                 │
│  ┌──────────┐ ┌──────────┐ ┌────────────────┐  │
│  │ Vector DB│ │ Embedding│ │ Prompt Registry │  │
│  │ pgvector │ │ TEI      │ │ Langfuse       │  │
│  │ Milvus   │ │          │ │                │  │
│  └──────────┘ └──────────┘ └────────────────┘  │
│  ┌──────────┐ ┌──────────┐ ┌────────────────┐  │
│  │ LLM GW   │ │ Eval     │ │ Observability  │  │
│  │ LiteLLM  │ │ Pipeline │ │ Grafana+Langfuse│ │
│  └──────────┘ └──────────┘ └────────────────┘  │
│  ┌──────────────────────────────────────────┐   │
│  │ Model Serving: vLLM / llama.cpp / TEI    │   │
│  └──────────────────────────────────────────┘   │
└─────────────────────────────────────────────────┘
```

---

## What We Provide (6 pillars)

### Pillar 1: Vector Database Infrastructure

**Already done:**
- [x] pgvector (PostgreSQL with vector extension)
- [x] Milvus sub-chart (standalone mode)

**To add:**
- [ ] Collection management API / init scripts
- [ ] Data versioning: tag vector collections with version labels
- [ ] Migration tooling: pgvector ↔ Milvus data export/import
- [ ] Grafana dashboard: vector DB performance (query latency, index size, memory)

### Pillar 2: Embedding Service

**Already done:**
- [x] TEI (Text Embeddings Inference) sub-chart

**To add:**
- [ ] Pre-configured embedding models (all-MiniLM-L6-v2, bge-large-zh, etc.)
- [ ] Embedding version tracking: which model version created which vectors
- [ ] A/B embedding comparison pipeline
- [ ] LiteLLM routing for embeddings (same gateway for both LLM and embedding)

### Pillar 3: Prompt Management & Versioning

**Langfuse already supports this natively:**
- Prompt Registry: create, version, deploy prompts via Langfuse UI/API
- Prompt A/B testing: route % of traffic to prompt v1 vs v2
- Prompt performance tracking: Langfuse scores per prompt version

**To add:**
- [ ] Prompt template examples for RAG (system prompt with context injection)
- [ ] Prompt CI/CD: GitOps workflow for prompt updates (store in Git, deploy via API)
- [ ] Grafana dashboard: prompt version performance comparison

### Pillar 4: RAG Evaluation & Quality

This is the **highest-value differentiator** — most platforms skip this.

**To add:**
- [ ] Eval dataset management: ground truth Q&A pairs stored in PostgreSQL
- [ ] Automated eval pipeline (CronJob):
  - Run queries against RAG system
  - Compare answers to ground truth
  - Score: faithfulness, relevance, hallucination rate
  - Push metrics to Prometheus, traces to Langfuse
- [ ] Hallucination detection:
  - Cross-reference LLM answer with retrieved context
  - Flag answers that contain claims not in the source documents
  - Langfuse annotation for human review
- [ ] Regression testing on data updates:
  - When knowledge base is updated, re-run eval suite
  - Alert if quality drops below threshold
- [ ] Grafana dashboard: RAG quality metrics over time

### Pillar 5: RAG Observability

**Already done:**
- [x] Langfuse traces LLM calls (prompt, tokens, latency, cost)
- [x] Prometheus metrics for vLLM
- [x] Grafana dashboards

**To add:**
- [ ] RAG-specific Langfuse trace structure:
  - Trace: full RAG request
    - Span: embedding (model, latency, input length)
    - Span: retrieval (query, top-k, vector DB latency, result count)
    - Span: generation (prompt version, model, tokens, latency)
    - Score: user feedback, auto-eval score
- [ ] Grafana RAG dashboard:
  - Retrieval latency (p50/p95)
  - Embedding throughput
  - Cache hit rate
  - Answer quality score trend
  - Hallucination rate trend
- [ ] Prometheus alerts:
  - RAG retrieval latency > threshold
  - Hallucination rate > threshold
  - Vector DB connection errors

### Pillar 6: RAG App Templates

We provide Helm values + integration configs for mainstream RAG platforms.
Users choose one, all pre-wired to use our infra.

| Platform | Type | Deployment | LiteLLM Integration |
|---|---|---|---|
| **Dify** | Full RAG platform + UI | Helm sub-chart | OpenAI-compatible provider |
| **n8n** | Workflow automation | Helm sub-chart | HTTP node → LiteLLM API |
| **LangChain** | Python framework | Example code + K8s Job | `ChatOpenAI(base_url=litellm)` |
| **LlamaIndex** | Python framework | Example code + K8s Job | `OpenAI(api_base=litellm)` |

Each template includes:
- Pre-configured connection to pgvector/Milvus
- LiteLLM as the LLM + embedding backend
- Langfuse tracing enabled
- Grafana datasource wired

---

## Implementation Priority

| Phase | Items | Value |
|---|---|---|
| **3a (now)** | Dify sub-chart, prompt template examples, RAG Grafana dashboard | Users can deploy RAG platform immediately |
| **3b (next)** | Eval pipeline CronJob, hallucination detection, quality dashboard | Differentiation — no other platform does this |
| **3c (later)** | n8n/LangChain/LlamaIndex templates, data versioning, embedding A/B | Ecosystem completeness |

---

## Key Design Decisions

1. **We don't build a RAG app** — we provide infra + templates for any RAG app
2. **Langfuse is the prompt registry** — no need for a separate prompt management tool
3. **Eval is a first-class citizen** — automated quality testing differentiates us from every other K8s LLM platform
4. **Vector DB is pluggable** — pgvector for simple, Milvus for scale, same Helm interface
5. **Everything is observable** — every RAG step (embed → retrieve → generate) traced in Langfuse, metrics in Prometheus, logs in Loki
