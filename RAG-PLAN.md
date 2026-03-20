# kube-llmops RAG Infrastructure Plan

**English** | [中文](RAG-PLAN.zh-CN.md)

> AI Infra perspective: we don't build a RAG app — we provide the infrastructure
> to **deploy, manage, test, observe, and continuously improve** any RAG system on Kubernetes.

---

## Core Philosophy

1. **We are infra, not application** — RAG app is user's choice (Dify, n8n, LazyLLM, LangChain...), we provide everything below it
2. **If it doesn't work out of the box, it doesn't count** — every feature must be `helm install` → works, not "template ready"
3. **Quality is a first-class citizen** — eval pipeline, hallucination detection, regression testing are not optional add-ons, they are core features
4. **CI/CD for AI** — prompt changes, data updates, model swaps all go through a validation pipeline before reaching production

```
┌──────────────────────────────────────────────────────────────────┐
│  RAG Application Layer (user's choice, we provide templates)     │
│  Dify / n8n / LazyLLM / LangChain / LlamaIndex / Coze          │
├──────────────────────────────────────────────────────────────────┤
│  CI/CD & Quality Gate                                            │
│  ┌─────────────┐ ┌──────────────┐ ┌───────────────────────────┐ │
│  │ Prompt CI/CD│ │ Data Pipeline│ │ RAG Eval Pipeline         │ │
│  │ Git→Langfuse│ │ Ingest→VecDB │ │ Hallucination Detection   │ │
│  │ A/B Deploy  │ │ Versioning   │ │ Regression Test on Update │ │
│  └─────────────┘ └──────────────┘ └───────────────────────────┘ │
├──────────────────────────────────────────────────────────────────┤
│  Core Infrastructure                                             │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌────────────────────┐ │
│  │ Vector DB│ │ Embedding│ │ LLM GW   │ │ Observability      │ │
│  │ pgvector │ │ TEI      │ │ LiteLLM  │ │ Langfuse (traces)  │ │
│  │ Milvus   │ │ (+ LiteLLM│ │          │ │ Prometheus (metrics)│ │
│  │          │ │  routing) │ │          │ │ Grafana (dashboard) │ │
│  └──────────┘ └──────────┘ └──────────┘ └────────────────────┘ │
│  ┌──────────────────────────────────────────────────────────────┐│
│  │ Model Serving: vLLM / llama.cpp / TEI                       ││
│  └──────────────────────────────────────────────────────────────┘│
└──────────────────────────────────────────────────────────────────┘
```

---

## 7 Pillars

### Pillar 1: Vector Database Infrastructure

| Item | Status | What "done" means |
|---|---|---|
| pgvector enabled | **Done** | `CREATE EXTENSION vector` works, 0.8.2 verified |
| Milvus standalone chart | **Done** | `helm install --set milvus.enabled=true` → Milvus running |
| Init script: auto-create collections | **Done** | On first deploy, create default collection with proper index |
| Data versioning | **Done** | Each ingestion batch tagged with version ID in metadata |
| Grafana dashboard: vector DB metrics | **Done** | Query latency, index size, row count, connection pool |

### Pillar 2: Embedding Service

| Item | Status | What "done" means |
|---|---|---|
| TEI chart | **Done** | Template exists |
| LiteLLM as embedding gateway | **Done** | `POST /v1/embeddings` routes to TEI, same auth + tracing |
| Embedding model presets | **Done** | values.yaml: `embedding.model: bge-large-zh-v1.5` → TEI deploys it |
| Embedding version tracking | **Done** | Langfuse metadata records embedding model + version per request |

### Pillar 3: Prompt Management & Versioning

Langfuse v2 has native prompt management. We wire it, not rebuild it.

| Item | Status | What "done" means |
|---|---|---|
| Langfuse prompt management | **Done** | UI: create prompt → version → deploy, already works |
| RAG prompt templates | **Done** | Ship 3-5 battle-tested RAG system prompts in Langfuse via init |
| Prompt CI/CD | **Done** | GitHub Action: on prompt file change → validate → push to Langfuse API |
| Prompt A/B metrics in Grafana | **Done** | Dashboard panel: response quality by prompt version |

### Pillar 4: RAG Evaluation & Quality (Key Differentiator)

This is what separates "toy" from "production". No other K8s LLM platform does this.

| Item | Status | What "done" means |
|---|---|---|
| Eval dataset schema | **Done** | PostgreSQL table: `eval_dataset(question, expected_answer, context, tags)` |
| Eval runner (CronJob/Job) | **Done** | K8s Job: load dataset → query RAG → score → push to Langfuse + Prometheus |
| Faithfulness scorer | **Done** | Does the answer only use info from retrieved context? Score 0-1 |
| Relevance scorer | **Done** | Is the retrieved context relevant to the question? Score 0-1 |
| Hallucination detector | **Done** | Claims in answer not supported by context → flagged |
| Regression gate | **Done** | On data update: auto-run eval, block deploy if quality drops >5% |
| Grafana quality dashboard | **Done** | Faithfulness/relevance/hallucination trends over time |
| Prometheus alerts | **Done** | `rag_hallucination_rate > 0.1` → alert |

**Eval tools considered:**
- [Ragas](https://github.com/explodinggradients/ragas) — most mature RAG eval framework
- [DeepEval](https://github.com/confident-ai/deepeval) — alternative with more metrics
- LLM-as-judge via LiteLLM (use a model to evaluate another model's output)

### Pillar 5: CI/CD for RAG (AI-native CI/CD)

Traditional CI/CD tests code. RAG CI/CD tests **data + prompts + models**.

| Item | Status | What "done" means |
|---|---|---|
| Prompt change pipeline | **Done** | Git push prompt → CI runs eval → pass → deploy to Langfuse |
| Data update pipeline | **Done** | New docs ingested → CI runs regression eval → pass → serve |
| Model swap pipeline | **Done** | Switch vLLM model → CI verifies RAG quality maintained → rollout |
| GitHub Actions workflow | **Done** | `.github/workflows/rag-eval.yaml` |
| Quality gate in Helm upgrade | **Done** | Pre-upgrade hook: run eval, abort if fail |

**CI/CD flow:**
```
Developer pushes:
  prompt change   → rag-eval.yaml → eval suite → pass? → Langfuse deploy
  new documents   → rag-eval.yaml → eval suite → pass? → vector DB update
  model change    → rag-eval.yaml → eval suite → pass? → helm upgrade
  
  Any failure → block deploy + alert + Langfuse annotation
```

### Pillar 6: RAG Observability

| Item | Status | What "done" means |
|---|---|---|
| Langfuse traces LLM calls | **Done** | Every LiteLLM request traced |
| RAG trace structure | **Done** | Trace spans: embed → retrieve → generate (not just generate) |
| Grafana RAG dashboard | **Done** | Retrieval latency, embedding throughput, quality score trend |
| End-to-end latency breakdown | **Done** | "Where did this 3s request spend its time?" visible in Langfuse |
| Prometheus RAG metrics | **Done** | Custom metrics: retrieval_latency, embedding_latency, quality_score |

### Pillar 7: RAG App Templates

Templates that ACTUALLY WORK — not "template ready, requires X".

| Platform | Type | Priority | What "done" means |
|---|---|---|---|
| **Dify** | Full RAG platform + UI | P0 | `helm install --set dify.enabled=true` → Dify UI works, pre-wired to LiteLLM + pgvector |
| **LazyLLM** | Chinese LLM app framework | P1 | Example project + K8s deployment, connected to our infra |
| **n8n** | Workflow automation | P2 | `--set n8n.enabled=true` → n8n with LiteLLM node pre-configured |
| **LangChain** | Python framework | P2 | Working example: ingest docs → query → answer, using our endpoints |
| **LlamaIndex** | Python framework | P2 | Working example, same as LangChain |

"Done" criteria for each template:
- [ ] `helm install` one command, everything runs
- [ ] Send a document → get it back via RAG query → within 5 minutes of install
- [ ] Traces visible in Langfuse
- [ ] Metrics visible in Grafana
- [ ] No manual steps, no hidden requirements, no SSL cert surprises

---

## Implementation Order

### Phase 3a: Make RAG Actually Work (immediate)
1. Dify sub-chart (real deployment, pre-wired)
2. RAG Grafana dashboard
3. Embed endpoint through LiteLLM

### Phase 3b: Quality & CI/CD (next)
4. Eval dataset schema + runner Job
5. Ragas integration for faithfulness/relevance scoring
6. GitHub Actions rag-eval.yaml workflow
7. Quality gate Grafana dashboard

### Phase 3c: Ecosystem (later)
8. LazyLLM template
9. n8n sub-chart
10. LangChain/LlamaIndex working examples
11. Prompt CI/CD pipeline
12. Data versioning

---

## Anti-patterns to Avoid

| Don't | Do |
|---|---|
| "Template ready, requires X operator" | Ship it working or don't ship it |
| Custom Python RAG app as demo | Integrate real tools people already use |
| Eval as optional afterthought | Eval pipeline runs on every deploy |
| Manual prompt management | GitOps: prompts in Git → CI validates → Langfuse deploys |
| "Works on my cluster" | Test on fresh cluster, document every prereq |
