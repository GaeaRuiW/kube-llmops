# 1. Pre-Retrieval Query Optimization

> **Layer summary:** Everything that happens between the user pressing "Send" and
> the first vector / keyword search hitting a document store. The goal is to
> transform a raw, often ambiguous, user query into one or more retrieval-friendly
> representations that maximise recall and precision of the downstream retrieval
> stage.

---

## 1.1 Query Rewriting

### What it is

Query rewriting transforms the user's original natural-language question into a
form that is more likely to retrieve the right documents. Human questions are
frequently vague, contain pronouns without clear referents, use colloquial
phrasing, or lack the domain-specific terms that appear in the knowledge base.
A rewriter bridges this *vocabulary mismatch* gap between the query space and
the document space.

Rewriting can be as simple as a single LLM call that reformulates the question,
or as sophisticated as a small, purpose-trained model that has learned to
maximise end-to-end answer quality through reinforcement learning. In a
production RAG pipeline, query rewriting is usually the cheapest high-leverage
optimisation available -- it requires no re-indexing, no new infrastructure, and
can be toggled on and off behind a feature flag.

### Method variants

| Variant | Description |
|---|---|
| **Prompt-based rewriting** | Instruct a general-purpose LLM (via system prompt) to reformulate the user query for better retrieval. Zero-shot, no training data required. Example prompt: *"Rewrite the following question so that it is self-contained and uses precise terminology suitable for a semantic search engine."* Simplest approach; latency is one LLM call. |
| **Rewrite-Retrieve-Read (RRR)** | Proposed by Ma et al. (EMNLP 2023). The pipeline explicitly separates three stages: (1) the LLM rewrites the query, (2) the rewritten query is sent to a retriever, (3) the LLM reads the retrieved passages and generates the answer. A *trainable* variant uses reinforcement learning (RL) to optimise the rewriter: the reward signal is derived from the quality of the final reader answer, and policy-gradient methods update the rewriter's parameters. |
| **Small model rewriter** | Distil rewriting ability into a lightweight model (e.g., T5-base, FLAN-T5-small). The training loop uses the downstream LLM reader's answer quality as a reward signal via RL (REINFORCE / PPO). This dramatically reduces inference cost compared to calling a large LLM for every rewrite while retaining most of the quality gain. |
| **BEIR query generation** | Rather than rewriting at query time, generate synthetic queries from each document at index time (e.g., with docT5query). The generated queries are appended to the document or stored as separate fields, improving lexical and semantic overlap at retrieval time. Particularly effective for domain-specific corpora where real query logs are scarce. |
| **Chain-of-Thought rewriting** | Ask the LLM to reason step-by-step about what information is needed before emitting the final rewritten query. Improves handling of multi-hop questions at the cost of higher latency. |

### Key papers

| Paper | Link |
|---|---|
| Ma et al., "Query Rewriting for Retrieval-Augmented Large Language Models" (EMNLP 2023) | [arXiv:2305.14283](https://arxiv.org/abs/2305.14283) |
| Nogueira & Lin, "Document Expansion by Query Prediction" (docT5query, 2019) | [arXiv:1904.08375](https://arxiv.org/abs/1904.08375) |
| Nogueira et al., "From doc2query to docTTTTTquery" (2019) | [GitHub repo](https://github.com/castorini/docTTTTTquery) |
| Wang et al., "Query2doc: Query Expansion with Large Language Models" (EMNLP 2023) | [arXiv:2303.07678](https://arxiv.org/abs/2303.07678) |

### Open-source implementations

| Project | Notes |
|---|---|
| [LangChain `PromptTemplate` + LLMChain](https://github.com/langchain-ai/langchain) | Trivial to build a prompt-based rewriter with `ChatPromptTemplate`. |
| [LlamaIndex `QueryTransform`](https://github.com/run-llama/llama_index) | `HyDEQueryTransform`, `StepDecomposeQueryTransform` and custom transforms. |
| [docTTTTTquery](https://github.com/castorini/docTTTTTquery) | BEIR-style document expansion with T5. |
| [Haystack `PromptNode`](https://github.com/deepset-ai/haystack) | Can be wired as a query rewriting stage in a Haystack pipeline. |

### Enterprise products

| Product | Capability |
|---|---|
| **Dify** | Built-in *Query Rewrite* node in the workflow editor; uses connected LLM to reformulate the user query before retrieval. |
| **Azure AI Search** | *Semantic ranking* layer applies neural re-ranking; *query language* supports Lucene expansions. |
| **Cohere** | The Search API supports automatic *query expansion* and server-side reranking. |
| **Google Vertex AI Search** | Automatic query understanding and expansion in Enterprise Search. |

### When to use

* **Always consider it first** -- it is the lowest-cost, highest-leverage pre-retrieval optimisation.
* Particularly valuable when users write short or ambiguous queries (chatbot UX).
* Prompt-based rewriting is sufficient for most general-domain use cases.
* Switch to a trained small-model rewriter when (a) latency of an extra LLM call is unacceptable, or (b) you have enough user traffic to generate training signal.
* BEIR query generation is preferable when queries are predictable but documents are long-tail technical content.

### kube-llmops integration path

```
User Query
    |
    v
[QueryRewriteStep]  <-- CRD: RAGPipeline.spec.preRetrieval.queryRewrite
    |                    fields: provider (openai | local-t5 | ollama)
    |                            prompt_template (ConfigMap ref)
    |                            max_rewrites (int, default 1)
    v
Rewritten Query --> Retriever
```

* Implement as a **sidecar container** or **init-step** in the RAG Pipeline CRD.
  The `QueryRewriteStep` calls the configured LLM (via the existing
  `LLMConnection` resource) with the specified prompt template.
* For the trained small-model variant, deploy a T5 model as a separate
  `InferenceService` (KServe / Seldon) and point the CRD at its endpoint.
* Expose Prometheus metrics: `rag_query_rewrite_latency_seconds`,
  `rag_query_rewrite_token_count`.

---

## 1.2 HyDE (Hypothetical Document Embeddings)

### What it is

HyDE flips the traditional retrieve-then-read paradigm on its head. Instead of
embedding the raw user query and searching for similar document chunks, HyDE
first asks the LLM to generate a *hypothetical answer document* -- a passage
that *would* answer the question if it existed in the corpus. This fabricated
passage is then embedded with the same document encoder, and the resulting vector
is used for nearest-neighbour search against the real document index.

The intuition is simple but powerful: a short query like "What is RLHF?" and a
150-word explanatory paragraph about RLHF live in very different regions of
embedding space. By generating the hypothetical paragraph, HyDE moves the query
representation into the *document neighbourhood* of the embedding space, which
typically leads to higher recall. The generated text need not be factually
correct -- it only needs to be topically relevant enough to land near the right
cluster of document vectors.

### Method variants

| Variant | Description |
|---|---|
| **Vanilla HyDE** | Generate a single hypothetical document, embed it, use that single vector for retrieval. Original method from Gao et al. |
| **Multi-HyDE** | Generate *N* hypothetical documents (e.g., N=5), embed each independently, then either (a) average the vectors, or (b) issue N separate searches and fuse the result lists (e.g., via Reciprocal Rank Fusion). Increases recall at the cost of N times the generation latency (parallelisable). |
| **HyDE with reranking** | Combine HyDE retrieval with a cross-encoder reranker to prune false positives introduced by the noisy hypothetical generation. |
| **Domain-conditioned HyDE** | Condition the hypothetical document generation on domain-specific instructions (e.g., "Write a Kubernetes troubleshooting guide paragraph that answers...") to improve embedding alignment for vertical corpora. |

### Key papers

| Paper | Link |
|---|---|
| Gao et al., "Precise Zero-Shot Dense Retrieval without Relevance Labels" (HyDE, 2022) | [arXiv:2212.10496](https://arxiv.org/abs/2212.10496) |
| Chuang et al., "Expand, Rerank, and Retrieve: Query Reranking with Multi-Hypothetical Document Embeddings" (2023) | [arXiv:2305.13264](https://arxiv.org/abs/2305.13264) |

### Open-source implementations

| Project | Notes |
|---|---|
| [LlamaIndex `HyDEQueryTransform`](https://docs.llamaindex.ai/en/stable/examples/query_transformations/HyDEQueryTransformDemo/) | Drop-in query transform; wraps any LLM + embedding model. |
| [LangChain `HypotheticalDocumentEmbedder`](https://python.langchain.com/docs/integrations/text_embedding/hypothetical_document_embedder/) | Chains an LLM generation step with an embedding model; returns an `Embeddings` object usable with any vector store. |
| [Haystack](https://github.com/deepset-ai/haystack) | Can be assembled manually via `PromptNode` -> embed -> retriever. |

### Enterprise products

| Product | Capability |
|---|---|
| **LlamaCloud** | HyDE toggle available in managed retrieval pipelines. |
| **Dify** | Achievable through a custom code node that calls LLM + embedding before the retrieval node. |
| **Zilliz Cloud Pipelines** | Supports pluggable query transformations including HyDE-style generation. |

### When to use

* Best suited for **short, keyword-style queries** against a corpus of long,
  explanatory documents (knowledge bases, documentation, medical literature).
* Adds one full LLM generation call of latency -- avoid for latency-sensitive
  chat applications unless you can cache or pre-generate.
* Less beneficial when queries are already long and descriptive, or when the
  embedding model has been fine-tuned on in-domain (query, document) pairs.
* **Caution:** If the LLM hallucinates a hypothetical document about the wrong
  topic, recall drops sharply. Pair with a reranker to mitigate.

### kube-llmops integration path

```
User Query
    |
    v
[HyDEStep]         <-- CRD: RAGPipeline.spec.preRetrieval.hyde
    |                    fields: llm_provider, prompt_template,
    |                            num_hypothetical_docs (int, default 1),
    |                            embedding_model (ref to EmbeddingService)
    v
Hypothetical Doc Vector(s)
    |
    v
Vector Store Search
```

* Runs as a step in the pipeline controller. The step calls the LLM to generate
  `num_hypothetical_docs` passages, embeds them via the same
  `EmbeddingService` used at index time, and forwards the vectors to the
  retriever.
* For Multi-HyDE with fusion, the controller fans out N vector searches in
  parallel and applies RRF before passing results downstream.
* Prometheus metrics: `rag_hyde_generation_latency_seconds`,
  `rag_hyde_doc_count`.

---

## 1.3 Multi-Query Expansion

### What it is

Multi-query expansion generates multiple reformulations of the original user
query -- each capturing a different facet, phrasing, or level of specificity --
and issues all of them as independent retrieval requests. The retrieved result
sets are then merged, typically using Reciprocal Rank Fusion (RRF), to produce a
single, more comprehensive ranking.

The motivation is that a single query phrasing may only match a subset of the
relevant documents. For example, a user asking "How do I scale a deployment in
Kubernetes?" might also benefit from searches for "kubectl scale command",
"HorizontalPodAutoscaler configuration", and "replica count update in K8s
manifest". By expanding the query surface, multi-query expansion improves recall
without requiring changes to the index or the embedding model.

### Method variants

| Variant | Description |
|---|---|
| **LLM-generated reformulations** | Prompt the LLM: "Generate 3-5 different versions of this question that capture different aspects." Each reformulation is sent to the retriever independently. |
| **RAG-Fusion** | Popularised by Raudaschl (2023). Combines LLM-generated multi-queries with Reciprocal Rank Fusion (RRF) for merging. The RRF score for document *d* across *k* rankings is: `RRF(d) = sum_{r in rankings} 1 / (k + rank_r(d))`. |
| **Query fan-out with filters** | Generate variants that include different metadata filters (e.g., date ranges, document types) to widen coverage across structured dimensions. |
| **Cross-lingual expansion** | Translate the query into multiple languages and search multilingual or language-specific indexes. Useful for multilingual knowledge bases. |

### Key papers

| Paper | Link |
|---|---|
| Raudaschl, "Forget RAG, the Future is RAG-Fusion" (2023) | [Blog / GitHub](https://github.com/Raudaschl/rag-fusion) |
| Cormack et al., "Reciprocal Rank Fusion outperforms Condorcet and individual Rank Learning Methods" (SIGIR 2009) | [Paper](https://dl.acm.org/doi/10.1145/1571941.1572114) |
| Dhuliawala et al., "Chain-of-Verification Reduces Hallucination in LLMs" (Meta, 2023) | [arXiv:2309.11495](https://arxiv.org/abs/2309.11495) |

### Open-source implementations

| Project | Notes |
|---|---|
| [LangChain `MultiQueryRetriever`](https://python.langchain.com/docs/how_to/MultiQueryRetriever/) | Generates multiple query variants via LLM, retrieves for each, deduplicates results. |
| [RAG-Fusion reference implementation](https://github.com/Raudaschl/rag-fusion) | Minimal Python implementation demonstrating multi-query + RRF. |
| [LlamaIndex `QueryFusionRetriever`](https://github.com/run-llama/llama_index) | Built-in multi-query generation with configurable fusion strategies. |
| [Haystack `MultihopEmbeddingRetriever`](https://github.com/deepset-ai/haystack) | Can be composed for multi-query workflows. |

### Enterprise products

| Product | Capability |
|---|---|
| **Dify** | Supports parallel retrieval branches in the workflow editor; each branch can carry a different query variant. |
| **AWS Bedrock Knowledge Bases** | Automatic *query decomposition* -- breaks complex queries into sub-queries and merges retrieved results. |
| **Azure AI Search** | Supports multiple search requests in a single API call; client-side RRF is straightforward. |
| **Cohere Rerank** | While primarily a reranker, Cohere's Search API can accept multiple queries and fuse internally. |

### When to use

* When users ask **broad or multi-faceted questions** that a single query
  phrasing cannot fully represent.
* When recall is more important than latency (each additional query adds a
  retrieval round-trip, though they can run in parallel).
* Especially effective combined with a **reranker** downstream to prune the
  enlarged result set back to a manageable context window.
* Avoid if the corpus is small and a single query already achieves near-perfect
  recall.

### kube-llmops integration path

```
User Query
    |
    v
[MultiQueryExpansionStep]  <-- CRD: RAGPipeline.spec.preRetrieval.multiQuery
    |                           fields: num_queries (int, default 3),
    |                                   fusion_strategy (rrf | union | interleave),
    |                                   rrf_k (int, default 60)
    |--- Query Variant 1 ---> Retriever ---> Results 1 --|
    |--- Query Variant 2 ---> Retriever ---> Results 2 --|-- [FusionStep] --> Merged Results
    |--- Query Variant 3 ---> Retriever ---> Results 3 --|
```

* The pipeline controller spawns parallel retrieval goroutines (or async tasks)
  for each query variant.
* `FusionStep` implements RRF (or configurable strategy) and emits a single
  ranked list.
* Prometheus metrics: `rag_multiquery_variant_count`,
  `rag_multiquery_fusion_latency_seconds`,
  `rag_multiquery_unique_docs_retrieved`.

---

## 1.4 Sub-Question Decomposition

### What it is

Sub-question decomposition tackles *complex, multi-hop* questions that cannot be
answered by a single retrieval pass. The strategy is to break the user's
question into a sequence of simpler, self-contained sub-questions, retrieve and
answer each one independently, and then synthesise the sub-answers into a final
comprehensive response.

For example, the question "How does the memory usage of vLLM compare to TGI
when serving Llama-2-70B on A100s?" requires at least two independent
information needs: (1) memory usage of vLLM for Llama-2-70B on A100, and (2)
memory usage of TGI for the same setup. A single retrieval query is unlikely to
surface both pieces of information from a documentation corpus. By decomposing,
the system can target each information need precisely.

### Method variants

| Variant | Description |
|---|---|
| **LLM-based decomposition** | Prompt the LLM: "Break this complex question into simpler sub-questions that can each be answered independently." The most common approach. |
| **Least-to-Most prompting** | Proposed by Zhou et al. (2022). Decompose the problem from simplest to most complex; solve each in order, feeding prior sub-answers as context. |
| **IRCoT (Interleaving Retrieval with Chain-of-Thought)** | Proposed by Trivedi et al. (2022). Interleaves retrieval and CoT reasoning steps: generate a CoT step, retrieve evidence for it, generate the next step, retrieve again, etc. |
| **Plan-and-Solve** | Generate an explicit plan of sub-questions, then execute them in dependency order (some sub-questions may depend on answers to prior ones). |
| **Tree-of-Questions** | Decompose into a tree structure where some sub-questions can be answered in parallel (independent branches) while others must be sequential (dependent branches). |

### Key papers

| Paper | Link |
|---|---|
| Trivedi et al., "Interleaving Retrieval with Chain-of-Thought Reasoning for Knowledge-Intensive Multi-Step Questions" (IRCoT, 2022) | [arXiv:2212.10509](https://arxiv.org/abs/2212.10509) |
| Zhou et al., "Least-to-Most Prompting Enables Complex Reasoning in Large Language Models" (2022) | [arXiv:2205.10625](https://arxiv.org/abs/2205.10625) |
| Press et al., "Measuring and Narrowing the Compositionality Gap in Language Models" (Self-Ask, 2022) | [arXiv:2210.03350](https://arxiv.org/abs/2210.03350) |
| Khattab et al., "Demonstrate-Search-Predict: Composing retrieval and language models for knowledge-intensive NLP" (DSP, 2022) | [arXiv:2212.14024](https://arxiv.org/abs/2212.14024) |

### Open-source implementations

| Project | Notes |
|---|---|
| [LlamaIndex `SubQuestionQueryEngine`](https://docs.llamaindex.ai/en/stable/examples/query_engine/sub_question_query_engine/) | Decomposes a query into sub-questions, routes each to a relevant query engine (can target different indexes), synthesises answers. |
| [LangChain question decomposition](https://python.langchain.com/docs/tutorials/rag/#query-decomposition) | Chains for decomposing questions and answering sub-parts; can be combined with `create_retrieval_chain`. |
| [DSPy `ChainOfThought` + `Retrieve`](https://github.com/stanfordnlp/dspy) | Programmatic framework for composing retrieval-interleaved reasoning chains. |
| [Haystack Agents](https://github.com/deepset-ai/haystack) | Agent-based pipelines can implement iterative decomposition and retrieval. |

### Enterprise products

| Product | Capability |
|---|---|
| **AWS Bedrock Knowledge Bases** | Automatic query decomposition for complex questions. |
| **Google Vertex AI Search** | Follow-up question understanding with automatic decomposition. |
| **LlamaCloud** | Managed sub-question decomposition via hosted LlamaIndex pipelines. |

### When to use

* Essential for **multi-hop questions** -- questions that require combining
  information from 2+ distinct sources (comparisons, timelines, causal chains).
* Also valuable for **compound questions** that contain multiple independent
  information needs joined by "and" / "also".
* Adds significant latency (sequential LLM calls for decomposition, then N
  parallel retrievals, then synthesis). Reserve for complex-query flows.
* Consider pairing with **query routing** (Section 1.5) so each sub-question
  can be routed to the most appropriate data source.

### kube-llmops integration path

```
User Query
    |
    v
[DecomposeStep]         <-- CRD: RAGPipeline.spec.preRetrieval.decompose
    |                        fields: strategy (llm | least-to-most | ircot),
    |                                max_sub_questions (int, default 4),
    |                                allow_sequential (bool, default true)
    |
    |--- Sub-Q 1 ---> Retriever A ---> Answer 1 --|
    |--- Sub-Q 2 ---> Retriever B ---> Answer 2 --|-- [SynthesisStep] --> Final Answer
    |--- Sub-Q 3 ---> Retriever A ---> Answer 3 --|
```

* The `DecomposeStep` calls the LLM to produce a plan (list of sub-questions
  with optional dependency graph).
* The controller schedules independent sub-questions in parallel and dependent
  ones sequentially.
* Each sub-question can optionally be routed via the `QueryRouter` (see 1.5).
* Prometheus metrics: `rag_decompose_sub_question_count`,
  `rag_decompose_total_latency_seconds`.

---

## 1.5 Query Routing

### What it is

Query routing is the decision layer that determines *where* a query (or
sub-query) should be sent for retrieval. In real-world RAG systems, data lives
in multiple backends: vector databases for semantic search, keyword indexes
(Elasticsearch / OpenSearch) for exact-match and BM25, graph databases for
relational queries, SQL databases for structured data, and even external APIs
for real-time information. A one-size-fits-all retriever rarely works across all
query types.

A query router classifies the incoming query and dispatches it to the most
appropriate backend(s). The classification can be performed by an LLM (function
calling / tool selection), a lightweight classifier (logistic regression, small
BERT), or a rule-based system (regex patterns, keyword detection). Sophisticated
routers can fan out to multiple backends simultaneously and merge results.

### Method variants

| Variant | Description |
|---|---|
| **LLM-based routing** | Present the LLM with descriptions of available retrieval backends (as "tools" or "choices") and let it decide which to invoke. Leverages function-calling or structured output capabilities. Most flexible but highest latency. |
| **Embedding similarity routing** | Embed the query and compare against pre-computed representative embeddings for each backend/index. Route to the backend whose representative embedding is closest. Fast and deterministic. |
| **Classifier-based routing** | Train a lightweight classifier (logistic regression, SVM, small BERT) on labelled query-to-backend pairs. Low latency, high accuracy when training data is available. |
| **Rule-based routing** | Use regex patterns, keyword lists, or query structure (e.g., presence of SQL keywords, date patterns) to route deterministically. Zero ML overhead; good for clearly separable query types. |
| **Hybrid fan-out** | Route to multiple backends in parallel and fuse results (similar to multi-query expansion but across heterogeneous data sources). Maximises recall at the cost of higher resource usage. |

### Key papers

| Paper | Link |
|---|---|
| Asai et al., "Self-RAG: Learning to Retrieve, Generate, and Critique through Self-Reflection" (2023) | [arXiv:2310.11511](https://arxiv.org/abs/2310.11511) |
| Shu et al., "RewriteRAG: Learning to Adapt Retrieval-Augmented Large Language Models through Query Rewriting" (2024) | [arXiv:2305.14283](https://arxiv.org/abs/2305.14283) |
| Patil et al., "Gorilla: Large Language Model Connected with Massive APIs" (2023) | [arXiv:2305.15334](https://arxiv.org/abs/2305.15334) |

### Open-source implementations

| Project | Notes |
|---|---|
| [LangChain `RouterChain`](https://python.langchain.com/docs/how_to/routing/) | LLM-based or embedding-based routing across multiple retrieval chains. Supports `MultiPromptChain` and custom routers. |
| [LlamaIndex `RouterQueryEngine`](https://docs.llamaindex.ai/en/stable/examples/query_engine/RouterQueryEngine/) | Routes to different query engines based on LLM-selected or embedding-selected "selector". Supports `LLMSingleSelector`, `LLMMultiSelector`, `PydanticSingleSelector`. |
| [Haystack `ConditionalRouter`](https://github.com/deepset-ai/haystack) | Routes between pipeline branches based on conditions (Jinja2 templates). |
| [DSPy `TypedPredictor` routing](https://github.com/stanfordnlp/dspy) | Can build a classifier module that routes to different retrieval modules. |

### Enterprise products

| Product | Capability |
|---|---|
| **Dify** | Visual workflow editor with *conditional branches* that route based on LLM classification, keyword matching, or variable conditions. |
| **Azure AI Search** | Multi-index search; applications can query multiple indexes and merge results. Skills pipeline supports routing. |
| **AWS Bedrock Knowledge Bases** | Supports multiple data sources (S3, Confluence, SharePoint, web, custom) with automatic routing. |
| **Google Vertex AI Search** | Blended search across unstructured, structured, and website data stores. |

### When to use

* **Mandatory** when your RAG system has more than one retrieval backend (which
  is most production systems).
* Even with a single vector store, routing is useful to decide between
  *semantic search* vs. *keyword search* vs. *metadata-filtered search* modes.
* LLM-based routing is best for diverse, unpredictable queries; classifier-based
  is better for high-throughput, latency-sensitive scenarios.
* Combine with sub-question decomposition: decompose first, then route each
  sub-question to the right backend.

### kube-llmops integration path

```
User Query (or Sub-Question)
    |
    v
[QueryRouterStep]       <-- CRD: RAGPipeline.spec.preRetrieval.router
    |                        fields: strategy (llm | embedding | classifier | rules),
    |                                backends:
    |                                  - name: vector-store
    |                                    type: qdrant
    |                                    description: "Semantic search over docs"
    |                                  - name: keyword-index
    |                                    type: elasticsearch
    |                                    description: "BM25 keyword search"
    |                                  - name: graph-db
    |                                    type: neo4j
    |                                    description: "Entity relationship queries"
    |
    |--- [vector-store]   ---> Qdrant retrieval
    |--- [keyword-index]  ---> Elasticsearch retrieval
    |--- [graph-db]       ---> Neo4j Cypher query
    |
    v
[ResultMergeStep] (optional, for fan-out)
```

* The `QueryRouterStep` is backed by a Kubernetes `Deployment` that runs the
  routing logic. For LLM-based routing, it calls the `LLMConnection` with
  function-calling. For classifier-based, it loads a model from a `ConfigMap` or
  `PVC`.
* Each backend is registered as a `RetrievalBackend` CRD instance, allowing
  the router to discover available backends dynamically.
* Prometheus metrics: `rag_router_decisions_total{backend="..."}`,
  `rag_router_latency_seconds`.

---

## 1.6 Step-back Prompting

### What it is

Step-back prompting is a technique developed by Google DeepMind that improves RAG
performance on questions requiring deep reasoning or domain expertise. Instead
of directly trying to answer (or retrieve for) the specific question, the system
first "steps back" to ask a more general, higher-level question. The answer to
this broader question provides background knowledge and context that helps the
LLM reason about the original specific question more accurately.

For example, given the question "What happens to the pressure of an ideal gas
if the temperature is increased by a factor of 2 and the volume is increased by
a factor of 8?", the step-back question would be "What is the ideal gas law and
how do its variables relate to each other?" The retrieved background knowledge
about PV = nRT then makes the specific calculation straightforward. In a RAG
context, this means issuing *two* retrieval queries: one for the step-back
(general) question and one for the original (specific) question, then providing
both sets of retrieved documents to the reader LLM.

### Method variants

| Variant | Description |
|---|---|
| **Vanilla step-back** | Generate one step-back question, retrieve for both the step-back and the original, concatenate context, and answer. Original method from Zheng et al. |
| **Multi-level step-back** | Generate step-back questions at multiple levels of abstraction (e.g., concept-level, principle-level, domain-level). Provides layered context from general to specific. |
| **Step-back + CoT** | Combine step-back prompting with chain-of-thought: first retrieve background knowledge via step-back, then reason step-by-step using that knowledge to answer the original question. |
| **Conditional step-back** | Only apply step-back when the original query is classified as requiring deep reasoning or domain knowledge. Saves latency for simple factual lookups. |

### Key papers

| Paper | Link |
|---|---|
| Zheng et al., "Take a Step Back: Evoking Reasoning via Abstraction in Large Language Models" (Google DeepMind, 2023) | [arXiv:2310.06117](https://arxiv.org/abs/2310.06117) |

### Open-source implementations

| Project | Notes |
|---|---|
| [LangChain step-back prompting example](https://python.langchain.com/docs/tutorials/rag/) | Documented pattern using `ChatPromptTemplate` with a step-back generation chain followed by retrieval. |
| [LlamaIndex `StepDecomposeQueryTransform`](https://github.com/run-llama/llama_index) | Query transform that can be configured for step-back style abstraction. |
| [DSPy](https://github.com/stanfordnlp/dspy) | Composable modules make it straightforward to implement step-back as a `ChainOfThought` -> `Retrieve` -> `ChainOfThought` pipeline. |

### Enterprise products

| Product | Capability |
|---|---|
| **Google Vertex AI Search** | Internal use of step-back-style reasoning in Google's search stack. |
| **Dify** | Implementable via workflow: an LLM node generates the step-back question, two parallel retrieval nodes fetch context, and a final LLM node synthesises. |

### When to use

* Most beneficial for **domain-expert questions** in science, law, medicine, and
  engineering where background principles are needed to reason about specifics.
* Also effective for **time-sensitive or highly specific** questions where
  direct retrieval often fails (e.g., "What was the GDP growth rate of Vietnam
  in Q3 2023?" -- step-back: "What are the recent economic trends in Vietnam?").
* Adds latency of one additional LLM call (step-back generation) and one
  additional retrieval round-trip. Can be parallelised with the original query
  retrieval.
* **Skip** for simple factual questions ("What is the capital of France?") or
  when the user's query is already at the right level of abstraction.

### kube-llmops integration path

```
User Query
    |
    v
[StepBackStep]          <-- CRD: RAGPipeline.spec.preRetrieval.stepBack
    |                        fields: enabled (bool),
    |                                conditional (bool, default false),
    |                                complexity_threshold (float, for conditional mode)
    |
    |--- Step-back Question ---> Retriever ---> Background Context --|
    |--- Original Question  ---> Retriever ---> Specific Context   --|
    |                                                                 |
    v                                                                 v
                    [ReaderLLM receives both contexts]
```

* When `conditional` is true, the controller first classifies query complexity
  (via a lightweight LLM call or a trained classifier). If the complexity score
  exceeds `complexity_threshold`, the step-back path is activated; otherwise,
  only the original query goes through.
* Both retrieval calls (step-back and original) run in parallel to minimise
  added latency.
* Prometheus metrics: `rag_stepback_activated_total`,
  `rag_stepback_skipped_total`, `rag_stepback_latency_seconds`.

---

## Summary: Choosing the Right Pre-Retrieval Strategy

| Technique | Latency Cost | Implementation Complexity | Best For |
|---|---|---|---|
| **Query Rewriting** | +1 LLM call | Low | Ambiguous / short queries |
| **HyDE** | +1 LLM call + 1 embedding | Medium | Short queries against long-doc corpora |
| **Multi-Query Expansion** | +1 LLM call + N retrievals | Medium | Broad / multi-faceted questions |
| **Sub-Question Decomposition** | +1 LLM call + N*(retrieval+LLM) | High | Multi-hop / comparison questions |
| **Query Routing** | +1 classification | Medium | Multi-backend architectures |
| **Step-back Prompting** | +1 LLM call + 1 retrieval | Low-Medium | Domain-expert / reasoning questions |

> **Recommendation for kube-llmops users:** Start with **Query Rewriting** (nearly
> free and universally beneficial), add **Query Routing** as soon as you have
> more than one retrieval backend, and layer in **HyDE** or **Multi-Query
> Expansion** based on your query profile. Reserve **Sub-Question Decomposition**
> and **Step-back Prompting** for pipelines that handle complex, multi-hop
> queries where answer quality justifies the added latency.

---

*Next section: [02 - Retrieval & Indexing](./02-retrieval-indexing.md)*
