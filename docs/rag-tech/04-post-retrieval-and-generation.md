# Part A: Post-Retrieval Processing / Part B: Generation Layer / Part C: Advanced RAG Patterns

> **Series**: RAG Technology Encyclopedia for kube-llmops
> **Scope**: Sections 4 -- 6 covering everything that happens *after* candidate documents leave the retriever and *before* (or while) the final answer reaches the user, plus the advanced orchestration patterns that tie the full pipeline together.

---

## Table of Contents

- [Part A -- Post-Retrieval Processing](#part-a----post-retrieval-processing)
  - [4.1 Reranking (Cross-Reference)](#41-reranking-cross-reference)
  - [4.2 Context Compression & Selection](#42-context-compression--selection)
  - [4.3 Lost-in-the-Middle Mitigation](#43-lost-in-the-middle-mitigation)
  - [4.4 Deduplication & Filtering](#44-deduplication--filtering)
- [Part B -- Generation Layer](#part-b----generation-layer)
  - [5.1 Faithful Generation (Grounded Generation)](#51-faithful-generation-grounded-generation)
  - [5.2 Citation & Source Attribution](#52-citation--source-attribution)
  - [5.3 Streaming Output](#53-streaming-output)
  - [5.4 Structured Output](#54-structured-output)
- [Part C -- Advanced RAG Patterns](#part-c----advanced-rag-patterns)
  - [6.1 Self-RAG (Self-Reflective RAG)](#61-self-rag-self-reflective-rag)
  - [6.2 Corrective RAG (CRAG)](#62-corrective-rag-crag)
  - [6.3 Agentic RAG](#63-agentic-rag)
  - [6.4 Iterative Retrieval](#64-iterative-retrieval)
  - [6.5 Recursive Retrieval (Multi-hop)](#65-recursive-retrieval-multi-hop)
  - [6.6 Multi-modal RAG](#66-multi-modal-rag)

---

# Part A -- Post-Retrieval Processing

Post-retrieval processing sits between the retriever and the generator. Its job is to *refine, compress, reorder, and deduplicate* the candidate document set so that the generator receives the highest-quality, most token-efficient context possible.

---

## 4.1 Reranking (Cross-Reference)

> **Full coverage**: See [03-retrieval.md -- Section 3.6 Reranking](./03-retrieval.md#36-reranking) for an in-depth treatment of cross-encoder rerankers, ColBERT-style late interaction, and LLM-based listwise reranking.

Reranking is listed here for completeness because it is the single most impactful post-retrieval step. In brief, a reranker takes the top-*k* candidates from a first-stage retriever (sparse, dense, or hybrid) and re-scores them with a more expressive model -- typically a cross-encoder that jointly attends to query and document. The reranked list is then truncated to top-*n* (where *n* << *k*) and passed downstream.

**Quick pointers for kube-llmops users**:

| Aspect | Recommendation |
|---|---|
| Self-hosted reranker | Deploy `BAAI/bge-reranker-v2-m3` via TEI sidecar (Helm value `reranker.enabled: true`) |
| API reranker | Cohere Rerank v3 or Jina Reranker v2 via LiteLLM proxy |
| LLM-based reranker | Use GPT-4o / Claude with a listwise prompt for < 20 candidates |

---

## 4.2 Context Compression & Selection

### What It Is

Even after reranking, retrieved chunks often contain large stretches of text that are irrelevant to the query. Feeding all of it into the LLM wastes precious context-window tokens, increases latency, raises cost, and -- counter-intuitively -- can *hurt* answer quality because the model has to sift through noise. Context compression and selection techniques address this by removing or summarizing the irrelevant portions of retrieved passages *before* they enter the generation prompt.

There are two broad families: **extractive** methods that select or delete tokens/sentences from the original text, and **abstractive** methods that rewrite passages into shorter, query-focused summaries. A third, increasingly popular family uses **information-theoretic signals** (entropy, self-information) to decide which tokens carry the most meaning and can be safely dropped.

### Method Variants

| Method | Type | Core Idea | Compression Ratio |
|---|---|---|---|
| **LLMLingua** | Extractive (token-level) | Use a small LM to compute per-token perplexity; drop low-information tokens | 2--5x |
| **LongLLMLingua** | Extractive (token-level) | Extension of LLMLingua optimised for long-context RAG; adds question-aware coarse-to-fine compression | 2--8x |
| **LLMLingua-2** | Extractive (token-level) | Data-distillation approach; trains a small classifier to predict which tokens to keep | 3--6x |
| **RECOMP -- Extractive** | Extractive (sentence-level) | Trains a sentence selector to pick relevant sentences from each passage | Variable |
| **RECOMP -- Abstractive** | Abstractive | Trains a small seq2seq model to produce a query-focused summary of each passage | Variable |
| **Selective Context** | Extractive (token-level) | Uses self-information (negative log probability from a causal LM) to filter low-value tokens | 2--5x |
| **LangChain ContextualCompressionRetriever** | Extractive / Abstractive | Wraps any retriever; uses an LLM or embeddings-based filter to extract relevant portions | Depends on LLM |
| **LlamaIndex SentenceEmbeddingOptimizer** | Extractive (sentence-level) | Keeps only sentences whose embedding is above a cosine-similarity threshold with the query | Variable |

### Key Papers

| Paper | Authors | Year | Link |
|---|---|---|---|
| LLMLingua: Compressing Prompts for Accelerated Inference of Large Language Models | Jiang et al. (Microsoft) | 2023 | [arXiv:2310.05736](https://arxiv.org/abs/2310.05736) |
| LongLLMLingua: Accelerating and Enhancing LLMs in Long Context Scenarios via Prompt Compression | Jiang et al. (Microsoft) | 2023 | [arXiv:2310.06839](https://arxiv.org/abs/2310.06839) |
| LLMLingua-2: Data Distillation for Efficient and Faithful Task-Agnostic Prompt Compression | Pan et al. (Microsoft) | 2024 | [arXiv:2403.12968](https://arxiv.org/abs/2403.12968) |
| RECOMP: Improving Retrieval-Augmented LMs with Compression and Selective Augmentation | Xu et al. | 2023 | [arXiv:2310.04408](https://arxiv.org/abs/2310.04408) |
| Selective Context | Li et al. | 2023 | [arXiv:2310.06201](https://arxiv.org/abs/2310.06201) |

### Open-Source Implementations

| Project | Link | Notes |
|---|---|---|
| **LLMLingua / LongLLMLingua / LLMLingua-2** | [github.com/microsoft/LLMLingua](https://github.com/microsoft/LLMLingua) | pip install `llmlingua`; provides `PromptCompressor` class |
| **RECOMP** | [github.com/carriex/recomp](https://github.com/carriex/recomp) | Training & inference code for extractive and abstractive compressors |
| **LangChain ContextualCompressionRetriever** | [python.langchain.com/docs/how_to/contextual_compression](https://python.langchain.com/docs/how_to/contextual_compression/) | Part of `langchain` core; works with any base retriever |
| **LlamaIndex** | [docs.llamaindex.ai](https://docs.llamaindex.ai/) | `SentenceEmbeddingOptimizer` post-processor built-in |

### Enterprise Products

Context compression is not yet widely commercialised as a standalone product. It is primarily an open-source / research-driven technique. Some platforms embed it implicitly:

- **Azure AI Search** -- semantic ranker trims snippets to the most relevant captions.
- **Cohere RAG** -- the `chat` endpoint internally compresses retrieved documents before generation.
- **Google Vertex AI Search** -- extractive answers and extractive segments act as a form of compression.

### When to Use

- Your retrieved chunks are long (> 512 tokens each) and contain mixed relevant/irrelevant content.
- You are budget-constrained and want to reduce token usage (LLMLingua can cut cost by 2--5x).
- You are hitting context-window limits even after reranking and top-*n* truncation.
- You need to feed many documents (> 10) into the prompt simultaneously.

**When to skip**: If your chunking strategy already produces short, focused chunks (< 256 tokens), compression adds latency for marginal gain.

### kube-llmops Integration Path

```yaml
# values.yaml -- enable context compression sidecar
postRetrieval:
  compression:
    enabled: true
    method: llmlingua          # llmlingua | recomp | langchain-contextual
    model: NousResearch/Llama-2-7b-hf   # small LM for perplexity scoring
    targetRatio: 0.3           # keep 30% of tokens
    gpu: true                  # GPU recommended for speed
```

1. **Deploy a compression sidecar** next to the RAG orchestrator pod. The sidecar loads a small causal LM (e.g., Llama-2-7B or Phi-2) and exposes a gRPC/REST endpoint.
2. After the retriever + reranker stage, the orchestrator sends the retrieved chunks + query to the compression sidecar.
3. The sidecar returns compressed text which is injected into the generation prompt.
4. Monitor compression ratio and answer-quality metrics via the kube-llmops evaluation dashboard.

---

## 4.3 Lost-in-the-Middle Mitigation

### What It Is

A landmark 2023 study by Liu et al. demonstrated that large language models exhibit a strong **U-shaped attention pattern** when processing long contexts: they attend heavily to information at the *beginning* and *end* of the input but largely ignore information in the *middle*. This means that if the most relevant retrieved document happens to land in the middle of your prompt, the LLM may effectively overlook it, producing a worse answer than if that document were positioned first or last.

Lost-in-the-middle is not a bug in a specific model -- it was observed across GPT-3.5 Turbo, Claude-1.3, MPT-30B-Instruct, and LongChat-13B-16K, among others. Newer models (GPT-4 Turbo, Claude 3, Gemini 1.5 Pro) have partially mitigated this through improved training on long contexts, but the effect has not been fully eliminated. For production RAG systems, explicit mitigation is still advisable.

### Method Variants

| Strategy | Description | Complexity |
|---|---|---|
| **Relevance-first ordering** | Place the highest-relevance documents at the very beginning of the context window | Trivial |
| **Ends-first ordering** | Place the most relevant document first, the second-most-relevant last, and lower-relevance documents in the middle | Low |
| **LangChain LongContextReorder** | Built-in transformer that implements ends-first reordering automatically | Low |
| **Chunked summarisation** | Summarise middle documents into shorter form so the LLM processes them faster | Medium |
| **Use better models** | Switch to models with superior long-context handling (Gemini 1.5 Pro 1M, Claude 3.5 Sonnet 200K, GPT-4 Turbo 128K) | N/A (model choice) |
| **Reduce context length** | Combine with compression (Section 4.2) to keep total context short enough that "middle" is small | Medium |

### Key Papers

| Paper | Authors | Year | Link |
|---|---|---|---|
| Lost in the Middle: How Language Models Use Long Contexts | Liu et al. (Stanford, UC Berkeley) | 2023 | [arXiv:2307.03172](https://arxiv.org/abs/2307.03172) |

### Open-Source Implementations

| Project | Link | Notes |
|---|---|---|
| **LangChain LongContextReorder** | [python.langchain.com/docs/how_to/long_context_reorder](https://python.langchain.com/docs/how_to/long_context_reorder/) | `LongContextReorder` document transformer; drop-in usage |
| **LlamaIndex** | Built-in node post-processors | `SentenceTransformerRerank` + custom ordering logic |

### Enterprise Products

No standalone enterprise product addresses this specifically. It is handled at the orchestration layer:

- **RAGFlow** -- internal document ordering heuristics.
- **Azure AI Search** -- semantic ranker returns results in relevance order; the application layer controls prompt construction.

### When to Use

- You are feeding **5+ documents** into the LLM context simultaneously.
- Your model's context window is large (32K+) and you are filling a significant portion of it.
- You observe that the LLM sometimes ignores clearly relevant retrieved passages (symptom: answer quality drops when more documents are added).

**When to skip**: If you only pass 1--3 short chunks, positional effects are negligible.

### kube-llmops Integration Path

```yaml
# values.yaml -- document ordering strategy
postRetrieval:
  ordering:
    strategy: ends-first        # relevance-first | ends-first | none
```

Implementation is straightforward: after reranking, the orchestrator applies the chosen ordering strategy to the list of document chunks before constructing the prompt. This is a pure Python transformation with zero external dependencies. The `ends-first` strategy interleaves documents: positions 0, 2, 4, ... get decreasing relevance from the top, and positions 1, 3, 5, ... get decreasing relevance from the bottom.

---

## 4.4 Deduplication & Filtering

### What It Is

When a retriever searches across multiple indices, multiple chunk overlaps (e.g., overlapping sliding-window chunks), or multiple query reformulations, the returned candidate set often contains **duplicate or near-duplicate passages**. Feeding duplicates into the LLM wastes context tokens and can bias the model toward over-representing certain information. Deduplication and filtering remove these redundancies and apply quality gates (e.g., minimum relevance score thresholds) before the context reaches the generator.

Deduplication operates at two granularities: **exact deduplication** (identical text or identical chunk IDs) and **near-duplicate detection** (passages that are substantially similar but not character-for-character identical, which commonly arises from overlapping chunking windows).

### Method Variants

| Method | Type | How It Works |
|---|---|---|
| **Exact ID dedup** | Exact | Remove chunks with the same document ID or chunk hash | 
| **MinHash + LSH** | Near-duplicate | Compute MinHash signatures of n-gram shingles; use Locality-Sensitive Hashing to group near-duplicates | 
| **SimHash** | Near-duplicate | Compute a single fingerprint per chunk; Hamming distance below threshold = duplicate |
| **Cosine similarity threshold** | Semantic | Compute pairwise cosine similarity of chunk embeddings; remove chunks with similarity > threshold (e.g., 0.95) |
| **MMR (Maximal Marginal Relevance)** | Diversity filter | Iteratively select chunks that are relevant to the query but dissimilar to already-selected chunks |
| **Relevance score threshold** | Quality filter | Drop any chunk whose retriever/reranker score falls below a minimum threshold |
| **Source diversity filter** | Metadata filter | Ensure no single source document contributes more than *k* chunks to the final set |

### Key Papers

| Paper / Resource | Link |
|---|---|
| Maximal Marginal Relevance (Carbonell & Goldstein, 1998) | [dl.acm.org/doi/10.1145/290941.291025](https://dl.acm.org/doi/10.1145/290941.291025) |
| MinHash and LSH (Broder, 1997) | Classic IR literature |

### Open-Source Implementations

| Project | Link | Notes |
|---|---|---|
| **datasketch** | [github.com/ekzhu/datasketch](https://github.com/ekzhu/datasketch) | Python library for MinHash, LSH, HyperLogLog |
| **LangChain EmbeddingsRedundantFilter** | [LangChain docs](https://python.langchain.com/docs/how_to/contextual_compression/#embeddingsfilter) | Filters near-duplicates by embedding similarity |
| **LlamaIndex** | Built-in `SimilarityPostprocessor` | Removes nodes below a similarity threshold |

### Enterprise Products

Deduplication is typically a built-in feature rather than a standalone product:

- **Pinecone** -- deduplication by vector ID at the index level.
- **Weaviate** -- supports `autocut` to automatically truncate results at score drops.
- **Azure AI Search** -- `distinct` parameter removes duplicate documents by field.

### When to Use

- Always. Deduplication is cheap and should be a default step in any retrieval pipeline.
- Especially important when using **overlapping chunks**, **multi-index retrieval**, or **query expansion** (which multiplies candidates).

### kube-llmops Integration Path

```yaml
# values.yaml -- deduplication & filtering
postRetrieval:
  dedup:
    enabled: true
    method: cosine              # exact | minhash | cosine | mmr
    similarityThreshold: 0.95   # for cosine method
  filtering:
    minScore: 0.5               # drop chunks with reranker score < 0.5
    maxChunksPerSource: 3       # source diversity limit
```

Implementation lives in the orchestrator's post-retrieval pipeline as a lightweight Python module. For `cosine` mode, chunk embeddings (already computed during retrieval) are reused -- no extra model calls needed. For `mmr`, the standard MMR formula is applied with a configurable lambda parameter to trade off relevance vs. diversity.

---

# Part B -- Generation Layer

The generation layer is where the LLM synthesises an answer from the user query plus the refined context. Getting generation right means ensuring **faithfulness** (no hallucination), **attribution** (cite your sources), **low latency** (stream tokens), and **structural compliance** (output the format the application expects).

---

## 5.1 Faithful Generation (Grounded Generation)

### What It Is

Faithful (or grounded) generation is the discipline of constraining the LLM to base its answer **exclusively on the retrieved context**, suppressing its parametric knowledge where it conflicts with or goes beyond the provided evidence. This is the central promise of RAG -- and also its hardest guarantee to keep. Even with relevant context in the prompt, LLMs can hallucinate facts, blend retrieved information with memorised information, or confidently fabricate details that appear nowhere in the source material.

Achieving faithfulness requires a multi-layered approach: careful **prompt engineering** (explicit instructions to stay grounded), **self-verification loops** (the model checks its own claims), **corrective retrieval** (detect when retrieval failed and retry), and **post-generation validation** (automated fact-checking against the context). No single technique is sufficient; production systems typically combine several.

### Method Variants

| Method | Category | Description |
|---|---|---|
| **System prompt grounding** | Prompt engineering | Instruct the model: "Answer ONLY based on the provided context. If the context does not contain the answer, say 'I don't know.'" |
| **Chain-of-Verification (CoVe)** | Self-verification | LLM generates a draft answer, then generates verification questions about its own claims, answers them independently, and revises the draft |
| **Corrective RAG (CRAG)** | Corrective retrieval | After retrieval, a lightweight evaluator scores document relevance; if low, the system triggers web search or falls back to "no answer" |
| **Attributable generation** | Inline grounding | Force the LLM to output `[cite:chunk_id]` markers for every factual claim; post-processing verifies each citation actually supports the claim |
| **NLI-based verification** | Post-generation | Run a Natural Language Inference model (e.g., DeBERTa-v3-large-mnli) to check if each generated sentence is entailed by the context |
| **Knowledge-grounded decoding** | Decoding-time | Modify token probabilities during generation to prefer tokens that appear in the context (experimental) |

### Key Papers

| Paper | Authors | Year | Link |
|---|---|---|---|
| Chain-of-Verification Reduces Hallucination in LLMs | Dhuliawala et al. (Meta) | 2023 | [arXiv:2309.11495](https://arxiv.org/abs/2309.11495) |
| Corrective Retrieval Augmented Generation (CRAG) | Yan et al. | 2024 | [arXiv:2401.15884](https://arxiv.org/abs/2401.15884) |
| FActScore: Fine-grained Atomic Evaluation of Factual Precision | Min et al. | 2023 | [arXiv:2305.14251](https://arxiv.org/abs/2305.14251) |
| RARR: Researching and Revising What Language Models Say | Gao et al. (Google) | 2023 | [arXiv:2210.08726](https://arxiv.org/abs/2210.08726) |

### Open-Source Implementations

| Project | Link | Notes |
|---|---|---|
| **LangGraph CRAG template** | [github.com/langchain-ai/langgraph](https://github.com/langchain-ai/langgraph) | Full corrective RAG workflow with relevance grading |
| **FActScore** | [github.com/shmsw25/FActScore](https://github.com/shmsw25/FActScore) | Atomic fact-checking evaluation framework |
| **RARR** | [github.com/google-research/rarr](https://github.com/google-research/rarr) | Research-and-revise pipeline from Google |
| **kube-llmops `rag-strict` template** | `charts/rag-pipeline/templates/prompts/rag-strict.yaml` | Built-in strict grounding prompt template |

### Enterprise Products

| Product | Faithfulness Feature |
|---|---|
| **Azure AI Search + OpenAI On Your Data** | Built-in grounding with citations; `strictness` parameter controls how closely the answer must match retrieved content |
| **Google Vertex AI Search** | Grounding scores per response; configurable grounding threshold |
| **Cohere RAG** | `chat` endpoint returns `citations` array mapping each answer span to source documents |
| **RAGFlow** | Every sentence in the output is grounded to a specific chunk with visual source attribution |
| **Vectara** | Hallucination Evaluation Model (HEM) provides a factual consistency score with every response |

### When to Use

- **Always** in any RAG system where factual accuracy matters (enterprise Q&A, legal, medical, financial).
- Especially critical when the LLM's parametric knowledge may be outdated or incorrect for your domain.
- Combine prompt-level grounding (cheap, always-on) with at least one verification method (CoVe or NLI) for high-stakes applications.

### kube-llmops Integration Path

```yaml
# values.yaml -- faithful generation configuration
generation:
  grounding:
    promptTemplate: rag-strict   # uses the built-in strict grounding prompt
    selfVerification:
      enabled: true
      method: cove               # cove | nli | none
      nliModel: cross-encoder/nli-deberta-v3-large  # if method=nli
    crag:
      enabled: true
      relevanceThreshold: 0.7    # CRAG triggers web search if max relevance < 0.7
      webSearchFallback: tavily  # tavily | serper | none
```

The orchestrator constructs the generation prompt using the `rag-strict` template, which includes explicit grounding instructions. If `selfVerification` is enabled, the orchestrator runs a second LLM call (or NLI model call) to verify the draft answer. If `crag` is enabled, retrieval quality is assessed before generation begins.

---

## 5.2 Citation & Source Attribution

### What It Is

Citation and source attribution is the practice of annotating each factual claim in the LLM's response with a reference to the specific source document(s) that support it. In a well-implemented citation system, users see inline markers like `[1]`, `[2]` that link to the original passages, enabling them to **verify** any claim with a single click. This transforms the LLM from a black-box oracle into a transparent research assistant.

There are two main paradigms: **inline citation**, where the LLM is instructed to produce citations as it generates (e.g., "The revenue grew 15% [1]"), and **post-hoc citation**, where the answer is generated first and a separate step matches each sentence to its supporting source(s) using NLI or semantic similarity. Inline citation is simpler to implement but less reliable; post-hoc citation is more robust but adds latency.

### Method Variants

| Method | Description | Reliability |
|---|---|---|
| **Inline citation (prompt-based)** | Instruct the LLM: "Cite sources using [1], [2] etc." The model generates citations as part of its output | Medium -- LLMs sometimes cite wrong source numbers or hallucinate citations |
| **Post-hoc citation (NLI-based)** | Generate answer without citations, then for each sentence, run NLI against each source to find supporting passages | High -- but adds latency |
| **Post-hoc citation (embedding-based)** | Generate answer, embed each sentence, compute similarity against source chunks | Medium-High |
| **Hybrid** | Inline citation during generation + post-hoc verification to correct wrong citations | Highest |
| **Structured citation** | Force the LLM to output a structured format (JSON) with claim-source pairs; render in the UI | High (with structured output constraints) |

### Key Papers

| Paper | Authors | Year | Link |
|---|---|---|---|
| ALCE: Enabling Large Language Models to Generate Text with Citations | Gao et al. | 2023 | [arXiv:2305.14627](https://arxiv.org/abs/2305.14627) |
| HAGRID: A Human-LLM Collaborative Dataset for Generative Information-Seeking with Attribution | Kamalloo et al. | 2023 | [arXiv:2307.16883](https://arxiv.org/abs/2307.16883) |
| Attributed Question Answering | Bohnet et al. (Google) | 2022 | [arXiv:2212.08037](https://arxiv.org/abs/2212.08037) |

### Open-Source Implementations

| Project | Link | Notes |
|---|---|---|
| **ALCE** | [github.com/princeton-nlp/ALCE](https://github.com/princeton-nlp/ALCE) | Benchmark + code for citation generation and evaluation |
| **RAGFlow** | [github.com/infiniflow/ragflow](https://github.com/infiniflow/ragflow) | Every answer sentence has clickable source attribution out-of-the-box |
| **LangChain** | Prompt templates + output parsers | Build custom citation chains with `ChatPromptTemplate` and `PydanticOutputParser` |
| **LlamaIndex CitationQueryEngine** | [docs.llamaindex.ai](https://docs.llamaindex.ai/) | Built-in citation engine that annotates responses with source nodes |

### Enterprise Products

| Product | Citation Capability |
|---|---|
| **Perplexity.ai** | Industry-leading inline citations with source previews; every answer is fully attributed |
| **Bing Chat / Microsoft Copilot** | Inline numbered citations linking to web sources |
| **Google SGE / AI Overviews** | Source cards alongside generated answers |
| **RAGFlow** | Sentence-level source attribution with chunk highlighting |
| **Cohere RAG** | API returns structured `citations` array with start/end character offsets and source document IDs |
| **Vectara** | Citations with grounding scores |

### When to Use

- **Always** for user-facing RAG applications. Users need to trust and verify answers.
- **Especially critical** in regulated industries (legal, medical, financial) where traceability is a compliance requirement.
- For internal/API-only use, structured citation (JSON with sources) is preferred over inline text citations.

### kube-llmops Integration Path

```yaml
# values.yaml -- citation configuration
generation:
  citation:
    enabled: true
    method: hybrid              # inline | posthoc-nli | posthoc-embedding | hybrid
    format: numbered            # numbered ([1], [2]) | footnote | structured-json
    verification:
      enabled: true             # verify inline citations with NLI post-hoc
      model: cross-encoder/nli-deberta-v3-large
```

The kube-llmops RAG orchestrator:
1. Numbers each context chunk (`[1]`, `[2]`, ...) in the prompt.
2. Instructs the LLM to cite using these numbers.
3. (If `verification.enabled`) Post-processes the output: for each `[N]` citation, checks via NLI that the preceding sentence is entailed by chunk N. Incorrect citations are flagged or corrected.
4. The API response includes both the annotated text and a structured `sources` array mapping each citation number to its chunk metadata (document title, page, URL).

---

## 5.3 Streaming Output

### What It Is

Streaming output delivers generated tokens to the client **as they are produced**, rather than waiting for the entire response to be complete. This dramatically improves perceived latency -- the user sees the first token within 100--500ms instead of waiting 5--30 seconds for the full answer. Streaming is implemented via **Server-Sent Events (SSE)** over HTTP or **WebSocket** connections, following the pattern established by the OpenAI Chat Completions API (`stream: true`).

In a RAG system, streaming interacts with post-generation steps (citation verification, faithfulness checking) in interesting ways: you may need to stream tokens to the user while simultaneously buffering the full response for verification. A common pattern is to stream optimistically and append a "verification badge" or corrections at the end.

### Method Variants

| Method | Protocol | Description |
|---|---|---|
| **SSE (Server-Sent Events)** | HTTP/1.1 or HTTP/2 | Unidirectional server-to-client stream; `text/event-stream` content type; simplest to implement |
| **WebSocket** | WS/WSS | Bidirectional; useful when the client needs to send signals (stop, edit) during generation |
| **gRPC streaming** | HTTP/2 | Server-streaming RPC; used by vLLM's gRPC API and some internal microservice architectures |
| **Chunked transfer encoding** | HTTP/1.1 | Lower-level HTTP chunking; less semantic than SSE but works everywhere |

### Key Papers

Streaming is an engineering practice rather than a research topic. No specific papers are dedicated to it, but it is a core feature of all modern LLM serving frameworks.

### Open-Source Implementations

| Project | Link | Streaming Support |
|---|---|---|
| **vLLM** | [github.com/vllm-project/vllm](https://github.com/vllm-project/vllm) | SSE streaming via OpenAI-compatible API; gRPC streaming |
| **LiteLLM** | [github.com/BerriAI/litellm](https://github.com/BerriAI/litellm) | Unified streaming interface across 100+ LLM providers; `stream=True` parameter |
| **TGI (Text Generation Inference)** | [github.com/huggingface/text-generation-inference](https://github.com/huggingface/text-generation-inference) | SSE streaming built-in |
| **Ollama** | [github.com/ollama/ollama](https://github.com/ollama/ollama) | Streaming by default in API responses |
| **LangChain** | [python.langchain.com](https://python.langchain.com/) | `.stream()` and `.astream()` methods on all LLM/Chain objects |

### Enterprise Products

| Product | Streaming |
|---|---|
| **OpenAI API** | `stream: true` parameter; SSE format |
| **Azure OpenAI** | Same SSE streaming as OpenAI |
| **Anthropic API** | SSE streaming with `stream: true`; also supports `MessageStream` helper |
| **Google Gemini API** | `stream=True` in `generate_content`; SSE for REST |
| **AWS Bedrock** | `InvokeModelWithResponseStream` API |
| **Cohere** | `stream=True` in chat endpoint |

All major LLM APIs support streaming. It is table-stakes for production deployments.

### When to Use

- **Always** for user-facing applications. There is no reason not to stream in production.
- For API-only / batch use cases, non-streaming may simplify downstream processing.
- When combining with citation verification, use a "stream + verify" pattern: stream tokens immediately, buffer the full response, verify citations after completion, and send a final verification event.

### kube-llmops Integration Path

```yaml
# values.yaml -- streaming is enabled by default
generation:
  streaming:
    enabled: true               # default: true
    protocol: sse               # sse | websocket
    heartbeatIntervalMs: 15000  # keep-alive for long-running generations
```

kube-llmops already supports streaming through the LiteLLM proxy. The RAG orchestrator streams tokens from the LLM backend (vLLM, TGI, or cloud API) through LiteLLM to the client. The `heartbeatIntervalMs` setting sends empty SSE comments to prevent proxy/load-balancer timeouts during the retrieval phase (before tokens start flowing).

---

## 5.4 Structured Output

### What It Is

Structured output forces the LLM to produce responses that conform to a specific format or schema -- most commonly **JSON**, but also XML, YAML, Markdown tables, or arbitrary grammars. This is essential for RAG systems that feed LLM output into downstream programmatic pipelines (e.g., populating a database, rendering a structured UI component, or triggering an API call). Without structured output guarantees, post-processing code must handle arbitrary free-text, leading to brittle parsing and frequent failures.

Modern approaches to structured output range from **prompt-based** (instruct the LLM to output JSON and hope for the best) to **constrained decoding** (mathematically guarantee that every generated token sequence is valid according to a context-free grammar or JSON schema). Constrained decoding is strictly superior for reliability but requires server-side support.

### Method Variants

| Method | Guarantee Level | Description |
|---|---|---|
| **Prompt-based** | Low | Include "Output valid JSON" in the prompt; no enforcement |
| **JSON mode (OpenAI)** | Medium | `response_format: { type: "json_object" }`; guarantees valid JSON but not schema compliance |
| **Structured Outputs (OpenAI)** | High | `response_format: { type: "json_schema", json_schema: {...} }`; guarantees JSON schema compliance |
| **Function calling / Tool use** | High | Define functions with typed parameters; the LLM produces arguments conforming to the schema |
| **Outlines (constrained decoding)** | Very High | Uses finite-state machines to mask invalid tokens at each generation step; works with any open model |
| **vLLM guided decoding** | Very High | Built-in support for Outlines and lm-format-enforcer backends; schema-constrained generation at serving layer |
| **LMQL** | High | Query language for LLMs with constraints expressed as Python-like assertions |
| **Guidance (Microsoft)** | High | Template language that interleaves text with constrained generation blocks |
| **Instructor** | High | Python library that wraps LLM calls with Pydantic model validation and automatic retries |

### Key Papers

| Paper | Authors | Year | Link |
|---|---|---|---|
| Efficient Guided Generation for Large Language Models (Outlines) | Willard & Louf | 2023 | [arXiv:2307.09702](https://arxiv.org/abs/2307.09702) |
| LMQL: Programming Large Language Models | Beurer-Kellner et al. | 2023 | [arXiv:2212.06094](https://arxiv.org/abs/2212.06094) |

### Open-Source Implementations

| Project | Link | Notes |
|---|---|---|
| **Outlines** | [github.com/outlines-dev/outlines](https://github.com/outlines-dev/outlines) | Industry-standard constrained decoding; regex, JSON schema, CFG support |
| **vLLM** | [github.com/vllm-project/vllm](https://github.com/vllm-project/vllm) | `--guided-decoding-backend outlines` flag; schema enforcement at serving time |
| **Instructor** | [github.com/jxnl/instructor](https://github.com/jxnl/instructor) | Pydantic-based structured output for OpenAI, Anthropic, Mistral, and more |
| **Guidance** | [github.com/guidance-ai/guidance](https://github.com/guidance-ai/guidance) | Microsoft's template-based constrained generation |
| **LMQL** | [github.com/eth-sri/lmql](https://github.com/eth-sri/lmql) | Query language with constraints |
| **LiteLLM** | [github.com/BerriAI/litellm](https://github.com/BerriAI/litellm) | `response_format` parameter unified across providers |

### Enterprise Products

| Product | Structured Output Support |
|---|---|
| **OpenAI** | JSON mode, Structured Outputs (JSON Schema), function calling |
| **Anthropic** | Tool use (function calling); JSON mode via system prompt |
| **Google Gemini** | `response_mime_type: "application/json"` + `response_schema` |
| **Azure OpenAI** | Same as OpenAI |
| **AWS Bedrock** | Function calling (Converse API); model-dependent JSON mode |

### When to Use

- Your RAG pipeline output feeds into a **programmatic consumer** (API, database, UI component).
- You need **deterministic parsing** -- no regex hacks or "try to parse, retry on failure" loops.
- You want to extract **structured data** from documents (e.g., extract all dates, names, amounts into a table).
- For self-hosted models, always use **Outlines via vLLM** for maximum reliability.
- For API-based models, use **Instructor** (for Pydantic validation) or the provider's native structured output feature.

### kube-llmops Integration Path

```yaml
# values.yaml -- structured output
generation:
  structuredOutput:
    enabled: false              # enable per-endpoint
    backend: outlines           # outlines (vLLM) | instructor | native
    schema: null                # JSON Schema object or reference to ConfigMap
```

When structured output is enabled for a specific endpoint:
1. If `backend: outlines` -- the vLLM serving layer is configured with `--guided-decoding-backend outlines`, and the orchestrator passes the JSON schema in the `guided_json` parameter of the completion request.
2. If `backend: instructor` -- the orchestrator wraps the LiteLLM call with Instructor's `patch()` function and a Pydantic model derived from the schema.
3. If `backend: native` -- the orchestrator passes `response_format` to the cloud LLM provider via LiteLLM.

---

# Part C -- Advanced RAG Patterns

These patterns go beyond the linear retrieve-then-generate pipeline. They introduce **feedback loops**, **agent-based decision-making**, **iterative refinement**, and **multi-modal understanding** to handle complex, real-world information needs.

---

## 6.1 Self-RAG (Self-Reflective RAG)

### What It Is

Self-RAG is a framework that trains a single LLM to **adaptively retrieve, generate, and critique** its own output through special reflection tokens. Unlike standard RAG, which always retrieves for every query, Self-RAG lets the model decide *whether* retrieval is needed at all. When it does retrieve, it evaluates the relevance of each retrieved passage, checks whether its generated output is supported by the evidence, and scores the overall usefulness of its response.

The key innovation is the introduction of four special tokens that the model generates inline: `[Retrieve]` (should I retrieve?), `[IsRel]` (is this passage relevant?), `[IsSup]` (is my generation supported by this passage?), and `[IsUse]` (is this response useful overall?). These reflection tokens are trained via a critic model and then distilled into the generator, creating a single model that handles the entire RAG pipeline with built-in quality control.

### Method Variants

| Variant | Description |
|---|---|
| **Self-RAG (original)** | Full training pipeline: train critic model on reflection tokens, then train generator with critic supervision |
| **Self-RAG (inference-only adaptation)** | Use a standard LLM with carefully crafted prompts to simulate the reflection tokens without fine-tuning |
| **LangGraph Self-RAG** | Graph-based implementation using separate LLM calls for generation, relevance grading, hallucination checking, and answer quality assessment |

### Key Papers

| Paper | Authors | Year | Link |
|---|---|---|---|
| Self-RAG: Learning to Retrieve, Generate, and Critique through Self-Reflection | Asai et al. (University of Washington) | 2023 | [arXiv:2310.11511](https://arxiv.org/abs/2310.11511) |

### Open-Source Implementations

| Project | Link | Notes |
|---|---|---|
| **Self-RAG (original)** | [github.com/AkariAsai/self-rag](https://github.com/AkariAsai/self-rag) | Training and inference code; fine-tuned Llama-2 models on HuggingFace |
| **LangGraph Self-RAG** | [github.com/langchain-ai/langgraph](https://github.com/langchain-ai/langgraph) | Tutorial notebook implementing Self-RAG as a state graph |

### Enterprise Products

Self-RAG is primarily a research technique. No major enterprise product explicitly markets "Self-RAG" as a feature, but the underlying ideas (adaptive retrieval, self-verification) appear in:

- **Google Vertex AI Search** -- confidence-based retrieval decisions.
- **Cohere RAG** -- internal relevance assessment before generation.

### When to Use

- You have a **mix of query types**: some need retrieval (factual), some don't (creative, conversational).
- You want the model to **self-assess** rather than relying on external evaluation.
- You can **fine-tune** a model (for the full Self-RAG approach) or are willing to accept higher latency from multi-step prompting (for the LangGraph adaptation).

**When to skip**: If all queries in your application need retrieval (e.g., enterprise knowledge base Q&A), the adaptive retrieval aspect adds complexity without benefit.

### kube-llmops Integration Path

```yaml
# values.yaml -- Self-RAG pattern
advancedPatterns:
  selfRag:
    enabled: false              # experimental
    implementation: langgraph   # langgraph | native (requires fine-tuned model)
    reflectionModel: null       # model for reflection steps (defaults to main generation model)
    retrievalThreshold: 0.5     # confidence threshold for triggering retrieval
```

When enabled via LangGraph, the orchestrator runs a state machine:
1. **Route**: Determine if retrieval is needed based on query analysis.
2. **Retrieve**: If needed, run the standard retrieval + reranking pipeline.
3. **Grade**: Evaluate relevance of each retrieved document.
4. **Generate**: Produce answer from relevant documents.
5. **Hallucination check**: Verify generation is grounded in context.
6. **Quality check**: Assess if the answer addresses the question.
7. **Loop or return**: If checks fail, retry with different documents or query reformulation.

---

## 6.2 Corrective RAG (CRAG)

### What It Is

Corrective RAG (CRAG) addresses a fundamental weakness of standard RAG: **what happens when retrieval fails?** In standard RAG, if the retriever returns irrelevant documents, the LLM either hallucinates an answer from its parametric memory or produces a confused response that blends irrelevant context with fabricated details. CRAG adds a **retrieval evaluator** that assesses the quality of retrieved documents and takes corrective action when quality is low.

The CRAG pipeline works as follows: after retrieval, a lightweight evaluator (which can be an LLM, a cross-encoder, or a trained classifier) assigns a confidence score to each retrieved document. Based on the aggregate confidence, one of three actions is triggered: **Correct** (confidence is high -- proceed with standard generation), **Ambiguous** (confidence is medium -- supplement retrieved documents with web search results), or **Incorrect** (confidence is low -- discard retrieved documents entirely and fall back to web search or decline to answer).

### Method Variants

| Variant | Evaluator | Corrective Action |
|---|---|---|
| **CRAG (original paper)** | Trained retrieval evaluator (T5-based) | Correct / Incorrect / Ambiguous → decompose-then-search |
| **LangGraph CRAG** | LLM-as-judge (GPT-4, Claude) | Binary relevant/not-relevant per document; web search fallback |
| **Simple CRAG** | Reranker score threshold | If max reranker score < threshold → trigger web search |
| **CRAG + Knowledge Refinement** | Same as original | Additionally extracts key knowledge strips from relevant docs, filters irrelevant strips |

### Key Papers

| Paper | Authors | Year | Link |
|---|---|---|---|
| Corrective Retrieval Augmented Generation | Yan et al. | 2024 | [arXiv:2401.15884](https://arxiv.org/abs/2401.15884) |

### Open-Source Implementations

| Project | Link | Notes |
|---|---|---|
| **LangGraph CRAG template** | [github.com/langchain-ai/langgraph](https://github.com/langchain-ai/langgraph) | Reference implementation with Tavily web search fallback |
| **CRAG (original)** | Paper code (check paper appendix) | Training and evaluation code |
| **Tavily** | [github.com/tavily-ai/tavily-python](https://github.com/tavily-ai/tavily-python) | Search API commonly used as the web search fallback in CRAG |

### Enterprise Products

| Product | CRAG-like Feature |
|---|---|
| **Perplexity.ai** | Automatically searches the web for supplementary information |
| **Google Vertex AI Search** | Blends enterprise data with web results based on confidence |
| **You.com** | RAG with dynamic web search augmentation |

### When to Use

- Your knowledge base **may not have answers** to all user questions.
- You want to **gracefully handle retrieval failures** rather than hallucinating.
- You have access to a **web search API** (Tavily, Serper, Google Custom Search) as a fallback.
- You want to improve user trust by declining to answer when context is insufficient rather than fabricating.

### kube-llmops Integration Path

```yaml
# values.yaml -- CRAG pattern
advancedPatterns:
  crag:
    enabled: true
    evaluator:
      method: llm-judge          # llm-judge | reranker-threshold | trained-classifier
      model: gpt-4o-mini         # for llm-judge
      threshold: 0.7             # reranker score threshold (for reranker-threshold method)
    corrective:
      webSearch:
        enabled: true
        provider: tavily          # tavily | serper | google-custom-search
        apiKeySecret: tavily-api-key
      noAnswerFallback: true     # if web search also fails, respond with "I don't know"
```

The CRAG pattern integrates as a decision node in the kube-llmops orchestrator graph:
1. After retrieval + reranking, the evaluator scores each document.
2. If all scores < threshold → trigger web search via the configured provider.
3. Web search results are embedded, chunked, and merged with original results.
4. Generation proceeds with the augmented context.
5. If web search also yields poor results and `noAnswerFallback` is true → return a "no answer" response.

---

## 6.3 Agentic RAG

### What It Is

Agentic RAG elevates the retrieval-augmented generation paradigm from a fixed pipeline to a **dynamic, tool-using agent**. Instead of a rigid sequence (retrieve → process → generate), an agentic RAG system uses an LLM as a reasoning engine that can autonomously decide **what actions to take**: query a vector database, search the web, call an API, run a SQL query, decompose a complex question into sub-questions, or even decide that no external information is needed. The agent loops through a reason-act-observe cycle until it has gathered enough information to produce a final answer.

This approach is powered by the **ReAct** (Reasoning + Acting) paradigm and its successors. The LLM is given a set of tools (retriever, web search, calculator, code interpreter, etc.) and uses chain-of-thought reasoning to decide which tool to invoke at each step. Agentic RAG excels at complex questions that require multi-step reasoning, comparison across multiple sources, or dynamic information gathering.

### Method Variants

| Variant | Description |
|---|---|
| **ReAct Agent** | Single agent with tools; reasons step-by-step (Thought → Action → Observation loop) |
| **Multi-agent** | Multiple specialised agents (researcher, summariser, critic) collaborate to answer the query |
| **Router Agent** | Lightweight agent that routes queries to different specialised RAG pipelines based on query analysis |
| **Plan-and-Execute** | Agent first creates a multi-step plan, then executes each step; separates planning from execution |
| **Tool-augmented RAG** | Standard RAG pipeline where the LLM can optionally invoke tools (calculator, code interpreter) during generation |

### Key Papers

| Paper | Authors | Year | Link |
|---|---|---|---|
| ReAct: Synergizing Reasoning and Acting in Language Models | Yao et al. (Princeton) | 2022 | [arXiv:2210.03629](https://arxiv.org/abs/2210.03629) |
| Toolformer: Language Models Can Teach Themselves to Use Tools | Schick et al. (Meta) | 2023 | [arXiv:2302.04761](https://arxiv.org/abs/2302.04761) |
| HuggingGPT: Solving AI Tasks with ChatGPT and its Friends in Hugging Face | Shen et al. | 2023 | [arXiv:2303.17580](https://arxiv.org/abs/2303.17580) |
| Voyager: An Open-Ended Embodied Agent with LLMs | Wang et al. (NVIDIA) | 2023 | [arXiv:2305.16291](https://arxiv.org/abs/2305.16291) |

### Open-Source Implementations

| Project | Link | Notes |
|---|---|---|
| **LangGraph** | [github.com/langchain-ai/langgraph](https://github.com/langchain-ai/langgraph) | Full agent loop with state persistence, human-in-the-loop, streaming; the de facto standard for agentic RAG |
| **LlamaIndex Agents** | [docs.llamaindex.ai](https://docs.llamaindex.ai/) | `ReActAgent`, `OpenAIAgent` with tool abstractions |
| **CrewAI** | [github.com/crewAIInc/crewAI](https://github.com/crewAIInc/crewAI) | Multi-agent framework with role-based agent design |
| **AutoGen** | [github.com/microsoft/autogen](https://github.com/microsoft/autogen) | Microsoft's multi-agent conversation framework |
| **Dify** | [github.com/langgenius/dify](https://github.com/langgenius/dify) | Visual workflow builder for agentic RAG; no-code agent configuration |
| **Haystack Agents** | [github.com/deepset-ai/haystack](https://github.com/deepset-ai/haystack) | Agent components in the Haystack pipeline |

### Enterprise Products

| Product | Agentic Capability |
|---|---|
| **AWS Bedrock Agents** | Fully managed agents with tool use, knowledge base integration, and action groups |
| **Azure AI Agent Service** | Managed agent runtime with tools, code interpreter, file search |
| **Google Vertex AI Agent Builder** | Visual agent builder with grounding, tools, and data stores |
| **OpenAI Assistants API** | Agents with code interpreter, file search, and function calling |
| **Dify Cloud** | Managed agentic workflow platform |
| **LangSmith + LangGraph Cloud** | Managed deployment and monitoring for LangGraph agents |

### When to Use

- Questions require **multiple reasoning steps** or **dynamic tool selection**.
- Your RAG system needs to query **multiple heterogeneous data sources** (vector DB + SQL DB + API + web).
- Users ask **complex comparative questions** ("Compare Q3 and Q4 revenue across all product lines").
- You need **adaptive behaviour** -- different queries need different processing strategies.

**When to skip**: For simple factual Q&A from a single knowledge base, a standard RAG pipeline is faster, cheaper, and more predictable. Agents add latency (multiple LLM calls) and cost.

### kube-llmops Integration Path

```yaml
# values.yaml -- agentic RAG
advancedPatterns:
  agenticRag:
    enabled: false
    framework: langgraph        # langgraph | llamaindex | custom
    tools:
      - name: vector-search
        type: retriever
        config:
          index: default
      - name: web-search
        type: tavily
        apiKeySecret: tavily-api-key
      - name: sql-query
        type: sql
        connectionSecret: postgres-connection
      - name: calculator
        type: builtin
    maxIterations: 10           # safety limit on agent loops
    timeout: 60s                # max total execution time
```

The kube-llmops agentic RAG deployment creates a LangGraph-based agent as a Kubernetes Deployment with:
1. **Tool pods**: Each tool runs as a sidecar or separate service (vector search reuses the existing retriever; SQL query runs a text-to-SQL service).
2. **State persistence**: Agent conversation state is stored in Redis (for short-term) or PostgreSQL (for long-running multi-turn agents).
3. **Observability**: Every agent step (thought, action, observation) is logged to the LangSmith-compatible tracing endpoint.
4. **Safety rails**: `maxIterations` and `timeout` prevent infinite loops and runaway costs.

---

## 6.4 Iterative Retrieval

### What It Is

Iterative retrieval recognises that a single round of retrieval often fails to gather all the information needed for a comprehensive answer, especially for complex or multi-faceted questions. Instead of one retrieve-then-generate pass, iterative retrieval performs **multiple rounds** of retrieval and generation, where each round's output informs the next round's query. The LLM generates partial answers, identifies information gaps, formulates new retrieval queries to fill those gaps, and progressively refines its response.

The two most influential approaches are **ITER-RETGEN**, which alternates between retrieval-augmented generation and generation-augmented retrieval (using the LLM's own output as a query for the next retrieval round), and **FLARE** (Forward-Looking Active REtrieval), which generates the answer sentence by sentence, monitors token-level confidence, and proactively retrieves when confidence drops below a threshold.

### Method Variants

| Method | Strategy | Trigger for New Retrieval |
|---|---|---|
| **ITER-RETGEN** | Alternate retrieval and generation; use previous generation as context for next retrieval | Fixed number of iterations (typically 2--3) |
| **FLARE** | Generate sentence by sentence; check per-token probability | Retrieve when any token probability falls below threshold |
| **Active Retrieval** | Model explicitly outputs a `[SEARCH]` token when it needs more information | Model-initiated (requires training or prompting) |
| **Retrieve-Read-Retrieve-Read** | Fixed multi-round pipeline; first round answers the surface question, second round answers follow-up questions identified by the model | Fixed 2-round pipeline |

### Key Papers

| Paper | Authors | Year | Link |
|---|---|---|---|
| ITER-RETGEN: Enhancing Retrieval-Augmented Large Language Models with Iterative Retrieval-Generation Synergy | Shao et al. | 2023 | [arXiv:2305.15294](https://arxiv.org/abs/2305.15294) |
| Active Retrieval Augmented Generation (FLARE) | Jiang et al. (CMU) | 2023 | [arXiv:2305.06983](https://arxiv.org/abs/2305.06983) |

### Open-Source Implementations

| Project | Link | Notes |
|---|---|---|
| **FLARE** | [github.com/jzbjyb/FLARE](https://github.com/jzbjyb/FLARE) | Original implementation from CMU |
| **LangChain FLARE** | [python.langchain.com](https://python.langchain.com/) | `FlareChain` in LangChain experimental; wraps the FLARE logic as a chain |
| **LlamaIndex** | [docs.llamaindex.ai](https://docs.llamaindex.ai/) | `FLAREInstructQueryEngine` built-in |

### Enterprise Products

Iterative retrieval is not commonly exposed as a named feature in enterprise products but is used internally:

- **Perplexity.ai** -- performs multiple search rounds to build comprehensive answers for complex queries.
- **Google Gemini Deep Research** -- iteratively searches and synthesises information across multiple rounds.

### When to Use

- Questions are **complex, multi-faceted**, or require synthesising information from multiple topics.
- A single retrieval round yields partial or insufficient information.
- You are building **research-style** or **deep-dive** features (e.g., "Write a comprehensive analysis of...").
- You can tolerate **higher latency** (multiple retrieval + generation rounds add significant time).

**When to skip**: For simple factual lookups or when latency is critical (< 3 seconds total).

### kube-llmops Integration Path

```yaml
# values.yaml -- iterative retrieval
advancedPatterns:
  iterativeRetrieval:
    enabled: false
    method: flare               # flare | iter-retgen | fixed-rounds
    maxRounds: 3                # maximum retrieval rounds
    flare:
      confidenceThreshold: 0.5  # trigger retrieval when token probability < threshold
    iterRetgen:
      rounds: 2                 # fixed number of generate-retrieve cycles
```

The orchestrator implements iterative retrieval as a loop in the generation pipeline:
1. **FLARE mode**: Generate token by token (or sentence by sentence) via streaming. Monitor logprobs from the LLM. When a low-confidence span is detected, pause generation, formulate a retrieval query from the low-confidence span, retrieve, inject new context, and resume generation.
2. **ITER-RETGEN mode**: Generate a full draft answer, use it as a retrieval query (concatenated with the original question), retrieve new documents, and regenerate with the augmented context. Repeat for `rounds` iterations.
3. The final output is the last round's generation.

---

## 6.5 Recursive Retrieval (Multi-hop)

### What It Is

Recursive (or multi-hop) retrieval tackles questions that **cannot be answered from a single retrieval step** because they require chaining facts across multiple documents. For example, "What is the GDP of the country where the inventor of the telephone was born?" requires first identifying the inventor (Alexander Graham Bell), then his birth country (Scotland/UK), then the UK's GDP. Each retrieval step depends on the *reasoning* from the previous step.

Unlike iterative retrieval (which refines the same query), recursive retrieval involves **distinct retrieval queries at each hop**, with each query informed by the chain of reasoning so far. Two major paradigms exist: **interleaved retrieval and reasoning** (IRCoT), which interleaves chain-of-thought reasoning steps with retrieval calls, and **hierarchical retrieval** (RAPTOR), which pre-builds a tree of document summaries at indexing time and retrieves at different levels of abstraction.

### Method Variants

| Method | Approach | When Retrieval Happens |
|---|---|---|
| **IRCoT** | Interleave chain-of-thought reasoning with retrieval; after each reasoning step, retrieve relevant documents for the next step | After each CoT step |
| **RAPTOR** | At indexing time, recursively cluster and summarise documents into a tree; at query time, retrieve from the appropriate tree level | At query time (tree traversal) |
| **ReAct multi-hop** | Agent uses search tool multiple times, chaining observations | Agent-decided |
| **LlamaIndex RecursiveRetriever** | Follow references from chunk metadata to retrieve parent documents or related chunks | Metadata-driven |
| **Sub-question decomposition** | Decompose complex question into sub-questions, answer each independently, then synthesise | Pre-retrieval decomposition |

### Key Papers

| Paper | Authors | Year | Link |
|---|---|---|---|
| Interleaving Retrieval with Chain-of-Thought Reasoning (IRCoT) | Trivedi et al. | 2022 | [arXiv:2212.10509](https://arxiv.org/abs/2212.10509) |
| RAPTOR: Recursive Abstractive Processing for Tree-Organized Retrieval | Sarthi et al. (Stanford) | 2024 | [arXiv:2401.18059](https://arxiv.org/abs/2401.18059) |
| Demonstrate-Search-Predict (DSP) | Khattab et al. (Stanford) | 2022 | [arXiv:2212.14024](https://arxiv.org/abs/2212.14024) |
| Multi-hop Question Answering via Reasoning Chains | Trivedi et al. | 2021 | [arXiv:2106.00855](https://arxiv.org/abs/2106.00855) |

### Open-Source Implementations

| Project | Link | Notes |
|---|---|---|
| **IRCoT** | [github.com/stonybrooknlp/ircot](https://github.com/stonybrooknlp/ircot) | Original IRCoT implementation |
| **RAPTOR** | [github.com/parthsarthi03/raptor](https://github.com/parthsarthi03/raptor) | Original RAPTOR implementation; tree construction and retrieval |
| **LlamaIndex RecursiveRetriever** | [docs.llamaindex.ai](https://docs.llamaindex.ai/) | Built-in recursive retrieval over hierarchical node structures |
| **LangChain multi-hop** | [python.langchain.com](https://python.langchain.com/) | Custom chains with sequential retrieval steps |
| **DSPy** | [github.com/stanfordnlp/dspy](https://github.com/stanfordnlp/dspy) | Declarative framework for multi-hop retrieval pipelines with automatic optimisation |

### Enterprise Products

| Product | Multi-hop Feature |
|---|---|
| **Perplexity.ai** | Internal multi-hop reasoning for complex queries |
| **Google Vertex AI Search** | Follow-up search and multi-turn retrieval |
| **AWS Bedrock Knowledge Bases** | Metadata-based retrieval chaining |

### When to Use

- Users ask **complex, compositional questions** that require reasoning across multiple facts.
- Your knowledge base contains **interconnected information** (e.g., entity relationships, cross-references).
- You are building a **research assistant** or **analytical tool** where depth of reasoning matters.
- **RAPTOR specifically**: Your corpus is large and hierarchical (textbooks, documentation, legal codes) and you want to retrieve at different levels of granularity.

**When to skip**: For simple, single-fact questions. Multi-hop adds significant latency and complexity.

### kube-llmops Integration Path

```yaml
# values.yaml -- recursive retrieval
advancedPatterns:
  recursiveRetrieval:
    enabled: false
    method: ircot               # ircot | raptor | sub-question | react-multihop
    maxHops: 3                  # maximum reasoning/retrieval hops
    raptor:
      enabled: false
      clusteringModel: text-embedding-3-small
      summarisationModel: gpt-4o-mini
      treeLevels: 3             # depth of the summary tree
```

For **IRCoT** and **ReAct multi-hop**, the orchestrator runs a LangGraph state machine where each node is a reason-then-retrieve step. For **RAPTOR**, an indexing job (CronJob or one-shot Job) builds the summary tree at ingest time and stores it in the vector database with level metadata. At query time, the retriever traverses the tree from the root (most abstract) down to leaves (most specific) based on relevance.

---

## 6.6 Multi-modal RAG

### What It Is

Multi-modal RAG extends the retrieval-augmented generation paradigm beyond text to incorporate **images, tables, charts, audio, and video**. Real-world knowledge bases are rarely text-only: technical documentation contains diagrams, financial reports contain charts and tables, medical records contain scans, and customer support data includes screenshots. A multi-modal RAG system can index, retrieve, and reason over all these modalities, producing richer and more accurate answers.

There are three main architectural approaches: **(1) Extract-and-index**, where non-text content is converted to text (OCR for images, table serialisation for tables, ASR for audio) and then processed through a standard text RAG pipeline; **(2) Multi-modal embedding**, where images, text, and other modalities are embedded into a shared vector space (e.g., CLIP) and retrieved together; and **(3) Native multi-modal generation**, where a vision-language model (GPT-4V, Gemini, Claude 3) directly processes images alongside text in the generation prompt.

### Method Variants

| Method | Modalities | Approach |
|---|---|---|
| **OCR + Text RAG** | Images with text (scans, PDFs, screenshots) | Extract text via OCR (Tesseract, PaddleOCR, Azure Document Intelligence), index as text |
| **Table extraction + serialisation** | Tables (PDFs, HTML) | Extract table structure, serialise as Markdown or HTML, index as text |
| **CLIP-based retrieval** | Images + text | Embed images and text into shared CLIP space; retrieve by cross-modal similarity |
| **Jina CLIP** | Images + text | `jina-clip-v2` embeds text and images into the same space; 1024-dim vectors |
| **ColPali** | Document pages as images | Use vision-language model to create multi-vector representations of document page images; no OCR needed |
| **Multi-modal LLM generation** | Images + text | Pass retrieved images directly to GPT-4V/Gemini/Claude as image inputs alongside text context |
| **Audio RAG** | Audio + text | Transcribe audio (Whisper), index transcriptions, optionally store audio embeddings |
| **Video RAG** | Video + text | Extract keyframes + transcription; index frames as images and transcript as text |

### Key Papers

| Paper | Authors | Year | Link |
|---|---|---|---|
| MuRAG: Multimodal Retrieval-Augmented Generator | Chen et al. (Google) | 2022 | [arXiv:2210.02928](https://arxiv.org/abs/2210.02928) |
| Learning Transferable Visual Models From Natural Language Supervision (CLIP) | Radford et al. (OpenAI) | 2021 | [arXiv:2103.00020](https://arxiv.org/abs/2103.00020) |
| ColPali: Efficient Document Retrieval with Vision Language Models | Faysse et al. | 2024 | [arXiv:2407.01449](https://arxiv.org/abs/2407.01449) |
| UniIR: Training and Benchmarking Universal Multimodal Information Retrieval | Wei et al. | 2023 | [arXiv:2311.17136](https://arxiv.org/abs/2311.17136) |

### Open-Source Implementations

| Project | Link | Notes |
|---|---|---|
| **RAGFlow** | [github.com/infiniflow/ragflow](https://github.com/infiniflow/ragflow) | Built-in image understanding, OCR, table extraction for RAG |
| **LlamaIndex Multi-modal** | [docs.llamaindex.ai](https://docs.llamaindex.ai/) | `MultiModalVectorStoreIndex`, `MultiModalLLM` classes |
| **Unstructured** | [github.com/Unstructured-IO/unstructured](https://github.com/Unstructured-IO/unstructured) | Extract text, tables, images from any document format (PDF, DOCX, PPTX, HTML) |
| **ColPali** | [github.com/illuin-tech/colpali](https://github.com/illuin-tech/colpali) | Vision-language retrieval without OCR |
| **OpenCLIP** | [github.com/mlfoundations/open_clip](https://github.com/mlfoundations/open_clip) | Open-source CLIP models for multi-modal embeddings |
| **Jina CLIP v2** | [huggingface.co/jinaai/jina-clip-v2](https://huggingface.co/jinaai/jina-clip-v2) | Unified text-image embedding model |
| **PaddleOCR** | [github.com/PaddlePaddle/PaddleOCR](https://github.com/PaddlePaddle/PaddleOCR) | State-of-the-art multilingual OCR |
| **Whisper** | [github.com/openai/whisper](https://github.com/openai/whisper) | Speech-to-text for audio RAG |

### Enterprise Products

| Product | Multi-modal Capability |
|---|---|
| **AWS Bedrock Knowledge Bases** | Multi-modal document parsing; image understanding via Claude 3 |
| **Google Vertex AI Search** | Multi-modal embeddings; image + text search; document AI for extraction |
| **Azure AI Document Intelligence** | Industry-leading document extraction (OCR, tables, forms, layout analysis) |
| **RAGFlow** | Native image, table, and chart understanding in RAG pipeline |
| **Unstructured (commercial)** | Managed document processing API for all file types |
| **LlamaParse (LlamaCloud)** | Cloud parsing service with multi-modal document understanding |

### When to Use

- Your knowledge base contains **PDFs with images/tables**, **presentations**, **scanned documents**, or **screenshots**.
- Users ask questions that require understanding **charts, diagrams, or visual content**.
- You need to build a **document Q&A system** for rich media documents.
- **ColPali specifically**: You want to skip OCR entirely and retrieve based on visual similarity of full document pages.

**When to skip**: If your corpus is pure text (e.g., API docs, text-only articles), multi-modal adds unnecessary complexity.

### kube-llmops Integration Path

```yaml
# values.yaml -- multi-modal RAG
advancedPatterns:
  multiModal:
    enabled: false
    ingestion:
      ocr:
        enabled: true
        engine: paddleocr        # paddleocr | tesseract | azure-doc-intelligence
      tableExtraction:
        enabled: true
        engine: unstructured     # unstructured | llamaparse | azure-doc-intelligence
      imageEmbedding:
        enabled: false
        model: jinaai/jina-clip-v2
    retrieval:
      imageSearch:
        enabled: false
        index: clip-images       # separate vector index for image embeddings
    generation:
      visionModel: gpt-4o       # model with vision capability for image-in-context generation
      maxImages: 5               # max images to include in generation context
```

The multi-modal pipeline adds components at each stage:
1. **Ingestion**: An Unstructured-powered preprocessing Job extracts text, tables, and images from uploaded documents. Tables are serialised as Markdown. Images are optionally embedded with Jina CLIP v2 and stored in a separate vector index.
2. **Retrieval**: Text chunks and (optionally) image embeddings are searched together. Retrieved images are passed as base64-encoded inputs to the vision-capable LLM.
3. **Generation**: The orchestrator constructs a multi-modal prompt with text context and inline images, sent to GPT-4o, Gemini, or Claude 3.5 Sonnet via LiteLLM.

---

## Quick Reference: Pattern Selection Guide

| Pattern | Best For | Latency Impact | Complexity |
|---|---|---|---|
| **Context Compression** | Reducing cost/tokens, handling long chunks | Low (+100--500ms) | Low |
| **Lost-in-the-Middle** | Many documents in context | Negligible | Very Low |
| **Deduplication** | Multi-index or overlapping-chunk retrieval | Negligible | Very Low |
| **Faithful Generation** | All production RAG systems | Low--Medium | Low--Medium |
| **Citation** | User-facing applications | Low | Low--Medium |
| **Streaming** | All user-facing applications | Negative (reduces perceived latency) | Low |
| **Structured Output** | Programmatic consumers | Negligible | Low |
| **Self-RAG** | Mixed query types (some need retrieval, some don't) | High (+2--5 LLM calls) | High |
| **CRAG** | Unreliable retrieval, open-domain questions | Medium (+1--2 LLM calls) | Medium |
| **Agentic RAG** | Complex, multi-source, multi-step questions | High (+3--10 LLM calls) | High |
| **Iterative Retrieval** | Research-style deep-dive questions | High (2--3x pipeline time) | Medium |
| **Recursive Retrieval** | Multi-hop reasoning, compositional questions | High (2--5x pipeline time) | High |
| **Multi-modal RAG** | Documents with images, tables, charts | Medium (image processing) | Medium--High |

---

## Recommended Reading Order

1. [01-overview.md](./01-overview.md) -- RAG fundamentals and architecture
2. [02-indexing.md](./02-indexing.md) -- Document processing, chunking, and embedding
3. [03-retrieval.md](./03-retrieval.md) -- Retrieval strategies, reranking, and hybrid search
4. **04-post-retrieval-and-generation.md** (this document) -- Post-retrieval processing, generation, and advanced patterns
5. [05-evaluation.md](./05-evaluation.md) -- RAG evaluation metrics and frameworks

---

*Last updated: 2025-01*
*Maintained by the kube-llmops team*
