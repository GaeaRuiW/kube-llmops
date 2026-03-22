# 3. Retrieval Layer

> **Position in the RAG pipeline:** After documents have been ingested, chunked, and indexed (Layers 1–2), the **Retrieval Layer** is responsible for finding the most relevant chunks for a given user query. This is the core "R" in RAG — the quality of your retrieval directly determines the quality of the generated answer.

The Retrieval Layer encompasses six major technique families:

| # | Technique | Latency | Accuracy | Pre-compute? | Typical Top-K |
|---|-----------|---------|----------|-------------|---------------|
| 3.1 | Dense Retrieval (Vector Search) | ~10 ms | High (semantic) | Yes | 20–100 |
| 3.2 | Sparse Retrieval (Lexical Search) | ~5 ms | High (keyword) | Yes (inverted index) | 20–100 |
| 3.3 | Hybrid Retrieval | ~15 ms | Higher | Yes | 20–100 |
| 3.4 | Reranking (Cross-encoder) | ~200 ms | Highest | No | Re-score top 20–50 |
| 3.5 | Embedding Fine-tuning | N/A (training) | Domain-adapted | Yes | — |
| 3.6 | Multi-vector Retrieval (ColBERT) | ~50 ms | Very High | Partial | 20–100 |

A production RAG system typically combines multiple techniques in a **retrieval pipeline**:

```
Query → [3.2 Sparse + 3.1 Dense] → 3.3 Hybrid Fusion → 3.4 Reranking → Top-K to LLM
```

---

## 3.1 Dense Retrieval (Vector Search)

### What It Is

Dense retrieval encodes both queries and documents into fixed-dimensional dense vectors (embeddings) using neural network models, then finds the most similar documents by computing nearest-neighbor distances in the embedding space. Unlike keyword-based methods, dense retrieval captures **semantic similarity** — a query about "automobile maintenance" will match documents about "car repair" even though they share no exact keywords.

The core architecture is the **bi-encoder** (also called **dual-encoder**): two separate encoder networks (often sharing weights) independently map the query and each document to a vector. At query time, only the query needs to be encoded on-the-fly; document vectors are pre-computed and stored in a vector index. This separation enables sub-millisecond retrieval over millions of documents using Approximate Nearest Neighbor (ANN) algorithms such as HNSW, IVF, and ScaNN.

### Method Variants

#### Embedding Models

| Model | Provider | Dimensions | Max Tokens | License | Notes |
|-------|----------|-----------|------------|---------|-------|
| `all-MiniLM-L6-v2` | sentence-transformers | 384 | 256 | Apache 2.0 | Lightweight, fast, good baseline |
| `all-mpnet-base-v2` | sentence-transformers | 768 | 384 | Apache 2.0 | Higher quality than MiniLM |
| `bge-large-en-v1.5` | BAAI | 1024 | 512 | MIT | Top open-source English model |
| `bge-large-zh-v1.5` | BAAI | 1024 | 512 | MIT | Chinese-specific, strong on C-MTEB |
| `bge-m3` (BGE-M3) | BAAI | 1024 | 8192 | MIT | Multilingual, multi-granularity, multi-function |
| `e5-large-v2` | Microsoft | 1024 | 512 | MIT | Prefix-based ("query: " / "passage: ") |
| `multilingual-e5-large` | Microsoft | 1024 | 512 | MIT | 100+ languages |
| `gte-large-en-v1.5` | Alibaba DAMO | 1024 | 8192 | Apache 2.0 | Long-context, competitive |
| `nomic-embed-text-v1.5` | Nomic AI | 768 | 8192 | Apache 2.0 | Matryoshka, long-context, fully open |
| `jina-embeddings-v3` | Jina AI | 1024 | 8192 | CC-BY-NC 4.0 | Task-specific LoRA adapters |
| `embed-english-v3.0` | Cohere | 1024 | 512 | Commercial API | Input-type aware (search_document / search_query) |
| `embed-multilingual-v3.0` | Cohere | 1024 | 512 | Commercial API | 100+ languages |
| `text-embedding-3-small` | OpenAI | 1536 | 8191 | Commercial API | Matryoshka, cheaper option |
| `text-embedding-3-large` | OpenAI | 3072 | 8191 | Commercial API | Matryoshka, highest quality |
| `voyage-3` | Voyage AI | 1024 | 32000 | Commercial API | Long-context, code-aware |
| `m3e-base` | Moka AI | 768 | 512 | MIT | Chinese community model |
| `text2vec-large-chinese` | shibing624 | 1024 | 512 | Apache 2.0 | Chinese text similarity |

#### Vector Databases & Indexes

| System | Type | ANN Algorithm | Hybrid Search | License/Model |
|--------|------|--------------|---------------|---------------|
| [FAISS](https://github.com/facebookresearch/faiss) | Library | IVF, HNSW, PQ | No | MIT |
| [pgvector](https://github.com/pgvector/pgvector) | PostgreSQL extension | HNSW, IVFFlat | Yes (with tsvector) | PostgreSQL |
| [Milvus](https://github.com/milvus-io/milvus) | Distributed DB | HNSW, IVF, DiskANN | Yes (2.4+) | Apache 2.0 |
| [Qdrant](https://github.com/qdrant/qdrant) | Vector DB | HNSW | Yes (sparse vectors) | Apache 2.0 |
| [Weaviate](https://github.com/weaviate/weaviate) | Vector DB | HNSW | Yes (BM25 + vector) | BSD-3 |
| [Chroma](https://github.com/chroma-core/chroma) | Embedded DB | HNSW (hnswlib) | No (metadata filter) | Apache 2.0 |
| [Pinecone](https://www.pinecone.io/) | Managed SaaS | Proprietary | Yes (sparse-dense) | Commercial |
| [Zilliz Cloud](https://zilliz.com/) | Managed Milvus | Same as Milvus | Yes | Commercial |

#### Similarity Metrics

| Metric | Formula | When to Use |
|--------|---------|-------------|
| **Cosine Similarity** | `cos(q, d) = (q · d) / (‖q‖ · ‖d‖)` | Default for normalized embeddings; most common |
| **Inner Product (IP)** | `IP(q, d) = q · d` | When embeddings are already L2-normalized (equivalent to cosine) |
| **L2 (Euclidean)** | `L2(q, d) = ‖q - d‖²` | When magnitude matters; less common for text |

#### Multilingual & Chinese-Specific Models

For **multilingual** use cases, prefer models explicitly trained on parallel corpora:

- **BGE-M3** (`BAAI/bge-m3`): Supports 100+ languages, 8192 token context, and produces dense, sparse, and ColBERT embeddings simultaneously. Best open-source multilingual option as of 2024.
- **multilingual-e5-large** (`intfloat/multilingual-e5-large`): 100+ languages, prefix-based.
- **Cohere embed-multilingual-v3.0**: Commercial, high quality across 100+ languages.

For **Chinese-specific** applications:

- **bge-large-zh-v1.5** (`BAAI/bge-large-zh-v1.5`): SOTA on C-MTEB benchmark, MIT license.
- **m3e-base** (`moka-ai/m3e-base`): Community model trained on Chinese text pairs, good for general Chinese retrieval.
- **text2vec-large-chinese** (`shibing624/text2vec-large-chinese`): Fine-tuned for Chinese text similarity tasks.

### Key Papers

| Paper | Authors | Link | Contribution |
|-------|---------|------|-------------|
| **Dense Passage Retrieval (DPR)** | Karpukhin et al., 2020 | [arXiv:2004.04906](https://arxiv.org/abs/2004.04906) | Showed dense retrieval can outperform BM25 on open-domain QA with contrastive training |
| **Sentence-BERT (SBERT)** | Reimers & Gurevych, 2019 | [arXiv:1908.10084](https://arxiv.org/abs/1908.10084) | Siamese BERT architecture for efficient sentence embeddings; foundation of sentence-transformers |
| **BGE Embedding** | Xiao et al., 2023 | [arXiv:2309.07597](https://arxiv.org/abs/2309.07597) | Retro-MAE pre-training + contrastive fine-tuning; C-MTEB & MTEB SOTA |
| **E5: Text Embeddings by Weakly-Supervised Contrastive Pre-training** | Wang et al., 2022 | [arXiv:2212.03533](https://arxiv.org/abs/2212.03533) | Large-scale weak supervision from web data |
| **BGE-M3** | Chen et al., 2024 | [arXiv:2402.03216](https://arxiv.org/abs/2402.03216) | Multi-lingual, multi-granularity, multi-function embedding model |

### Open-Source Implementations

| Repository | Description | Link |
|-----------|-------------|------|
| sentence-transformers | Python framework for SOTA sentence embeddings | [github.com/UKPLab/sentence-transformers](https://github.com/UKPLab/sentence-transformers) |
| FlagEmbedding | BAAI's BGE model family training & inference | [github.com/FlagOpen/FlagEmbedding](https://github.com/FlagOpen/FlagEmbedding) |
| FAISS | Facebook's billion-scale similarity search library | [github.com/facebookresearch/faiss](https://github.com/facebookresearch/faiss) |
| pgvector | Vector similarity search for PostgreSQL | [github.com/pgvector/pgvector](https://github.com/pgvector/pgvector) |
| Milvus | Cloud-native vector database | [github.com/milvus-io/milvus](https://github.com/milvus-io/milvus) |
| Qdrant | High-performance vector search engine | [github.com/qdrant/qdrant](https://github.com/qdrant/qdrant) |
| Weaviate | AI-native vector database | [github.com/weaviate/weaviate](https://github.com/weaviate/weaviate) |
| Chroma | AI-native open-source embedding database | [github.com/chroma-core/chroma](https://github.com/chroma-core/chroma) |
| Hugging Face TEI | High-performance embedding inference server | [github.com/huggingface/text-embeddings-inference](https://github.com/huggingface/text-embeddings-inference) |

### Enterprise Products

| Product | Provider | Notes |
|---------|----------|-------|
| Pinecone | Pinecone | Fully managed, serverless option, hybrid sparse-dense |
| Zilliz Cloud | Zilliz | Managed Milvus, enterprise SLA |
| OpenAI Embeddings API | OpenAI | `text-embedding-3-small/large`, Matryoshka support |
| Cohere Embed | Cohere | `embed-v3`, input-type aware, multilingual |
| Voyage AI | Voyage AI | Domain-specific (code, law, finance) |
| Azure AI Search | Microsoft | Vector search with built-in embedding |
| Vertex AI Matching Engine | Google Cloud | Managed ANN service |
| Amazon Bedrock | AWS | Titan Embeddings, integrated with Knowledge Bases |

### When to Use

**Use dense retrieval when:**
- Your users ask **natural language questions** (semantic matching is crucial)
- Vocabulary mismatch is common (synonyms, paraphrases)
- You need **cross-lingual** retrieval
- Document collection is medium-sized (up to ~10M chunks for single-node vector DB)

**Avoid or supplement dense retrieval when:**
- Exact keyword matching is critical (e.g., part numbers, error codes)
- You need to match **named entities** precisely
- Your domain vocabulary is highly specialized and not in the embedding model's training data
- You have extreme latency requirements (< 1ms) — ANN adds overhead vs. inverted index

### kube-llmops Integration Path

```yaml
# Helm values for kube-llmops dense retrieval component
retrieval:
  dense:
    # Embedding model served via TEI (Text Embeddings Inference)
    embedding:
      model: BAAI/bge-large-en-v1.5
      server: text-embeddings-inference
      replicas: 2
      resources:
        requests:
          nvidia.com/gpu: 1     # GPU recommended for > 100 QPS
          memory: 4Gi
        limits:
          nvidia.com/gpu: 1
          memory: 8Gi
      # Matryoshka: reduce dimensions at query time to save storage
      truncateDimension: 512    # Optional: truncate 1024 → 512

    # Vector database (choose one)
    vectorDB:
      type: pgvector           # pgvector | milvus | qdrant | weaviate
      pgvector:
        # Co-locate with existing PostgreSQL or deploy dedicated
        existingClaim: postgres-pvc
        indexType: hnsw         # hnsw (faster query) | ivfflat (faster build)
        efConstruction: 200
        m: 16
        distanceMetric: cosine  # cosine | ip | l2
      # milvus:
      #   endpoint: milvus.vector-db.svc.cluster.local:19530
      #   collection: rag_chunks

    # Query-time parameters
    search:
      topK: 50                 # Retrieve top-50, pass to reranker
      efSearch: 128            # HNSW search parameter (accuracy vs speed)
```

**Deployment topology:**

```
┌──────────────────────────────────────────────────────────────┐
│  Kubernetes Cluster                                          │
│                                                              │
│  ┌─────────────┐    ┌──────────────────┐    ┌────────────┐  │
│  │  TEI Server  │◄───│  Retrieval API   │───►│  pgvector   │  │
│  │  (GPU pod)   │    │  (Python pod)    │    │  (StatefulSet)│ │
│  │  BGE-large   │    │  FastAPI / gRPC  │    │  HNSW index │  │
│  └─────────────┘    └──────────────────┘    └────────────┘  │
│        ▲                     ▲                               │
│        │                     │                               │
│  Encode query          User query via                        │
│  into vector           Ingress / Gateway                     │
└──────────────────────────────────────────────────────────────┘
```

---

## 3.2 Sparse Retrieval (Lexical Search)

### What It Is

Sparse retrieval matches documents to queries based on **exact or near-exact term overlap** using high-dimensional sparse vectors, where each dimension corresponds to a vocabulary term. The classic approach builds an **inverted index** — a mapping from each term to the list of documents containing it — and uses a scoring function like BM25 to rank matches by term frequency and document frequency statistics.

Despite the rise of neural methods, sparse retrieval remains indispensable. It excels at matching **proper nouns, technical terms, error codes, product IDs**, and any token where the exact string is the signal — cases where embedding models often fail because these tokens are out-of-vocabulary or semantically ambiguous. Modern **learned sparse representations** like SPLADE bridge the gap by using a neural network to expand and weight terms, combining the efficiency of inverted indexes with some semantic understanding.

### Method Variants

#### Classical Methods

| Method | Description | Scoring |
|--------|-------------|---------|
| **BM25** (Best Matching 25) | Probabilistic retrieval model based on term frequency saturation and document length normalization | `score = Σ IDF(t) · (tf(t,d) · (k1+1)) / (tf(t,d) + k1 · (1 - b + b · dl/avgdl))` |
| **TF-IDF** | Term Frequency × Inverse Document Frequency | `score = tf(t,d) · log(N / df(t))` |
| **Boolean Retrieval** | Exact match with AND/OR/NOT operators | Binary (match or no match) |

#### Learned Sparse Representations

| Method | Description | Key Idea |
|--------|-------------|----------|
| **SPLADE** | Sparse Lexical and Expansion model | Uses MLM head of BERT to produce sparse term weights; adds expansion terms not in original document |
| **SPLADE++** | Improved SPLADE with distillation | Distillation from cross-encoder + efficiency improvements |
| **DeepImpact** | Term impact scoring | Learns per-term impact scores using BERT |
| **uniCOIL** | Unified single-vector COIL | Single-dimension per-token scoring |
| **SPLADE v3** | Latest SPLADE iteration | Better efficiency-effectiveness tradeoff |

### Key Papers

| Paper | Authors | Link | Contribution |
|-------|---------|------|-------------|
| **SPLADE: Sparse Lexical and Expansion Model** | Formal et al., 2021 | [arXiv:2107.05720](https://arxiv.org/abs/2107.05720) | Learned sparse representations via BERT MLM head; competitive with dense on MSMARCO |
| **SPLADE v2: Sparse Lexical and Expansion Model for Information Retrieval** | Formal et al., 2021 | [arXiv:2109.10086](https://arxiv.org/abs/2109.10086) | Distillation and regularization improvements |
| **From Distillation to Hard Negative Sampling: Making Sparse Neural IR Models More Effective** | Formal et al., 2022 | [arXiv:2205.04733](https://arxiv.org/abs/2205.04733) | SPLADE++ with improved training |
| **The Probabilistic Relevance Framework: BM25 and Beyond** | Robertson & Zaragoza, 2009 | [Foundations and Trends in IR](https://www.staff.city.ac.uk/~sbrp622/papers/foundations_bm25_review.pdf) | Definitive reference for BM25 family |

### Open-Source Implementations

| Repository / Tool | Language | Description | Link |
|-------------------|----------|-------------|------|
| Elasticsearch | Java | Full-text search engine with BM25 | [github.com/elastic/elasticsearch](https://github.com/elastic/elasticsearch) |
| OpenSearch | Java | AWS fork of Elasticsearch | [github.com/opensearch-project/OpenSearch](https://github.com/opensearch-project/OpenSearch) |
| Tantivy | Rust | Fast full-text search library (Lucene-like) | [github.com/quickwit-oss/tantivy](https://github.com/quickwit-oss/tantivy) |
| rank-bm25 | Python | Lightweight BM25 implementation | [github.com/dorianbrown/rank_bm25](https://github.com/dorianbrown/rank_bm25) |
| bm25s | Python | Fast BM25 with Scipy sparse matrices | [github.com/xhluca/bm25s](https://github.com/xhluca/bm25s) |
| PostgreSQL tsvector | SQL | Built-in full-text search | [postgresql.org/docs](https://www.postgresql.org/docs/current/textsearch.html) |
| SPLADE (naver) | Python | Official SPLADE implementation | [github.com/naver/splade](https://github.com/naver/splade) |
| Pyserini | Python | Python wrapper for Anserini (Lucene) | [github.com/castorini/pyserini](https://github.com/castorini/pyserini) |

### Enterprise Products

| Product | Provider | Notes |
|---------|----------|-------|
| Elastic Cloud | Elastic | Managed Elasticsearch with ML features |
| Amazon OpenSearch Service | AWS | Managed OpenSearch |
| Azure AI Search | Microsoft | Full-text + semantic search |
| Algolia | Algolia | Typo-tolerant keyword search SaaS |
| Typesense | Typesense | Open-core search engine |

### When to Use

**Use sparse retrieval when:**
- Queries contain **exact terms** that must match: product names, error codes, function names, API endpoints, legal citations
- **Code search**: function/variable names are best matched lexically
- You need a **strong baseline** quickly — BM25 is hard to beat without domain-specific embeddings
- **Multilingual** scenarios where embedding models have limited coverage for the target language
- Your domain has **highly specific vocabulary** (medical codes, chemical names, part numbers)
- You need **explainability** — you can show which terms matched and why

**Use learned sparse (SPLADE) when:**
- You want the efficiency of inverted indexes **plus** query/document expansion
- You're building a system where infrastructure must stay on CPU (no GPU for inference)

### kube-llmops Integration Path

```yaml
retrieval:
  sparse:
    engine: elasticsearch      # elasticsearch | opensearch | postgresql
    elasticsearch:
      # Deploy via ECK (Elastic Cloud on Kubernetes) or connect to existing
      endpoint: http://elasticsearch.search.svc.cluster.local:9200
      index:
        name: rag_chunks
        analyzer: standard     # standard | ik_max_word (Chinese) | kuromoji (Japanese)
        similarity: BM25
        bm25:
          k1: 1.2
          b: 0.75
    # Alternative: use same PostgreSQL as vector DB
    # postgresql:
    #   useTsvector: true
    #   language: english      # english | chinese (via zhparser) | simple
    search:
      topK: 50
```

**Chinese full-text search note:** For Chinese text, use the [IK Analyzer](https://github.com/medcl/elasticsearch-analysis-ik) plugin for Elasticsearch or the [zhparser](https://github.com/amutu/zhparser) extension for PostgreSQL. Standard tokenizers break on CJK characters.

---

## 3.3 Hybrid Retrieval (Dense + Sparse Fusion)

### What It Is

Hybrid retrieval combines the results of **dense (semantic) retrieval** and **sparse (keyword) retrieval** to get the best of both worlds. Dense retrieval captures meaning and handles paraphrases; sparse retrieval captures exact terms and handles specificity. By fusing both result sets, hybrid retrieval consistently outperforms either method alone across benchmarks and real-world applications.

The key challenge is **score fusion** — dense and sparse retrievers produce scores on different scales and distributions. Simply concatenating the result lists doesn't work; you need a principled fusion strategy. The three main approaches are Reciprocal Rank Fusion (RRF), convex combination of normalized scores, and learned fusion models.

### Method Variants

#### Fusion Methods

##### 1. Reciprocal Rank Fusion (RRF)

The most popular fusion method due to its simplicity and robustness. It uses only the **rank position** of each document, not the raw scores, making it score-distribution agnostic.

```
RRF_score(d) = Σ  1 / (k + rank_i(d))
               i∈retrievers

where k = 60 (constant, typically 60)
```

**Pros:** No score normalization needed, works with any number of retrievers, robust across domains.
**Cons:** Ignores score magnitude (a document ranked #1 with 0.99 score is treated the same as one with 0.51 score).

##### 2. Convex Combination (Weighted Sum)

Normalize scores from each retriever to [0, 1], then take a weighted sum:

```
score(d) = α · norm_dense(d) + (1 - α) · norm_sparse(d)

where α ∈ [0, 1], typically α = 0.5–0.7
```

Normalization methods:
- **Min-max**: `norm(s) = (s - min) / (max - min)` within each result set
- **Z-score**: `norm(s) = (s - μ) / σ`
- **Rank-based**: Convert to percentiles

**Pros:** Tunable weight gives control; captures score magnitude.
**Cons:** Requires score normalization; optimal α varies by query.

##### 3. Learned Fusion

Train a small model (logistic regression, LambdaMART, small neural network) to combine retriever scores and features:

```
score(d) = f(dense_score, sparse_score, dense_rank, sparse_rank, query_features, ...)
```

**Pros:** Can learn query-dependent weighting (some queries are better served by keywords).
**Cons:** Requires labeled training data; adds complexity.

### Key Papers

| Paper | Authors | Link | Contribution |
|-------|---------|------|-------------|
| **Reciprocal Rank Fusion outperforms Condorcet and individual Rank Learning Methods** | Cormack, Clarke & Buettcher, 2009 | [SIGIR 2009](https://dl.acm.org/doi/10.1145/1571941.1572114) | Introduced RRF; showed it outperforms individual rankers and other fusion methods |
| **RAG-Fusion: A New Approach** | Raudaschl, 2023 | [arXiv:2402.03367](https://arxiv.org/abs/2402.03367) | Generate multiple query variants, retrieve for each, fuse with RRF |
| **Hybrid Search Re-examined** | Ma et al., 2024 | [arXiv:2407.01154](https://arxiv.org/abs/2407.01154) | Systematic evaluation of hybrid methods on BEIR benchmark |
| **BGE-M3** | Chen et al., 2024 | [arXiv:2402.03216](https://arxiv.org/abs/2402.03216) | Single model producing dense, sparse, and ColBERT vectors — native hybrid |

### Open-Source Implementations

| Tool / System | How It Does Hybrid | Link |
|---------------|-------------------|------|
| **Milvus 2.4+** | Built-in hybrid search: `AnnSearchRequest` for dense + sparse, `RRFRanker` or `WeightedRanker` | [github.com/milvus-io/milvus](https://github.com/milvus-io/milvus) |
| **Weaviate** | `hybrid` search mode with configurable `alpha` (0 = BM25, 1 = vector) | [github.com/weaviate/weaviate](https://github.com/weaviate/weaviate) |
| **Qdrant** | Sparse + dense vectors in same collection, query-time fusion | [github.com/qdrant/qdrant](https://github.com/qdrant/qdrant) |
| **pgvector + tsvector** | Same PostgreSQL instance: vector search + full-text search, application-level RRF | [github.com/pgvector/pgvector](https://github.com/pgvector/pgvector) |
| **LangChain EnsembleRetriever** | Wraps multiple retrievers, RRF fusion in Python | [github.com/langchain-ai/langchain](https://github.com/langchain-ai/langchain) |
| **LlamaIndex** | `QueryFusionRetriever` with RRF | [github.com/run-llama/llama_index](https://github.com/run-llama/llama_index) |
| **Pinecone** | Hybrid index with sparse-dense vectors in same request | [pinecone.io](https://www.pinecone.io/) |

**Example: pgvector + tsvector hybrid in SQL:**

```sql
-- Dense retrieval (pgvector)
WITH dense AS (
  SELECT id, 1 - (embedding <=> $query_embedding) AS score
  FROM chunks
  ORDER BY embedding <=> $query_embedding
  LIMIT 50
),
-- Sparse retrieval (tsvector)
sparse AS (
  SELECT id, ts_rank_cd(tsv, plainto_tsquery('english', $query_text)) AS score
  FROM chunks
  WHERE tsv @@ plainto_tsquery('english', $query_text)
  ORDER BY score DESC
  LIMIT 50
),
-- RRF fusion
ranked AS (
  SELECT
    COALESCE(d.id, s.id) AS id,
    COALESCE(1.0 / (60 + d.rank), 0) + COALESCE(1.0 / (60 + s.rank), 0) AS rrf_score
  FROM
    (SELECT id, score, ROW_NUMBER() OVER (ORDER BY score DESC) AS rank FROM dense) d
  FULL OUTER JOIN
    (SELECT id, score, ROW_NUMBER() OVER (ORDER BY score DESC) AS rank FROM sparse) s
  ON d.id = s.id
)
SELECT id, rrf_score FROM ranked ORDER BY rrf_score DESC LIMIT 20;
```

### Enterprise Products

| Product | Provider | Hybrid Approach |
|---------|----------|----------------|
| Azure AI Search | Microsoft | Semantic ranker + keyword search + vector search |
| AWS Bedrock Knowledge Bases | Amazon | Hybrid retrieval with Titan Embeddings + OpenSearch |
| Elastic Search 8.x | Elastic | kNN vector search + BM25 in same query |
| Pinecone | Pinecone | Sparse-dense hybrid vectors |
| Zilliz Cloud | Zilliz | Milvus hybrid search |
| Cohere Rerank + Embed | Cohere | Two-stage: embed for dense, rerank for fusion |

### When to Use

**Use hybrid retrieval when (almost always):**
- You want the **best out-of-the-box retrieval quality** — hybrid consistently beats pure dense or pure sparse
- Your queries mix **semantic intent** with **specific keywords** (e.g., "how to fix OOM error in pod kube-system/coredns")
- Your corpus spans multiple domains or document types
- You're building a **production system** where robustness matters more than squeezing the last 0.1% from one method

**Skip hybrid when:**
- You have extreme latency constraints and cannot afford two retrieval paths
- Your corpus is tiny (< 1000 chunks) — simple dense retrieval is sufficient
- You're prototyping and want simplicity

**Recommended starting point:** RRF with k=60, dense top-50 + sparse top-50, fused to top-20, then reranked.

### kube-llmops Integration Path

```yaml
retrieval:
  hybrid:
    enabled: true
    fusion:
      method: rrf             # rrf | convex | learned
      rrf:
        k: 60                 # RRF constant
      # convex:
      #   alpha: 0.6          # Weight for dense scores
      #   normalization: min_max

    dense:
      topK: 50
      model: BAAI/bge-large-en-v1.5
      vectorDB: pgvector

    sparse:
      topK: 50
      engine: postgresql      # Use tsvector on same PG instance as pgvector
      analyzer: english

    output:
      topK: 20                # Fused result count → sent to reranker
```

**Architecture — single PostgreSQL for both dense and sparse:**

```
┌────────────────────────────────────────────────────────────────┐
│  PostgreSQL (single instance)                                  │
│                                                                │
│  ┌─────────────────────────┐  ┌─────────────────────────────┐  │
│  │  pgvector extension     │  │  tsvector / tsquery          │  │
│  │  HNSW index on          │  │  GIN index on                │  │
│  │  embedding column       │  │  full-text column            │  │
│  └────────────┬────────────┘  └──────────────┬──────────────┘  │
│               │                               │                │
│               ▼                               ▼                │
│         Dense results                   Sparse results         │
│               │                               │                │
│               └───────────┬───────────────────┘                │
│                           ▼                                    │
│                    RRF Fusion (app-level)                       │
│                           │                                    │
│                           ▼                                    │
│                    Top-20 → Reranker                            │
└────────────────────────────────────────────────────────────────┘
```

This single-database approach minimizes operational complexity — no separate search engine to manage.

---

## 3.4 Reranking (Cross-Encoder)

### What It Is

Reranking is a **second-stage scoring** step that takes the top-K results from initial retrieval (typically 20–100 candidates from dense, sparse, or hybrid search) and re-scores each (query, document) pair using a more powerful model. While bi-encoder (embedding) models encode query and document independently, a **cross-encoder** processes them jointly — concatenating query and document as `[CLS] query [SEP] document [SEP]` and outputting a single relevance score. This joint attention allows the model to capture fine-grained interactions between query and document tokens.

The tradeoff is clear: cross-encoders are **much more accurate** than bi-encoders but **much slower** because they cannot pre-compute document representations. A bi-encoder retrieves from millions of documents in milliseconds; a cross-encoder can only re-score a few hundred pairs per second. This is why reranking is always a second stage — you first use fast retrieval to narrow down candidates, then use the expensive cross-encoder to find the best among them.

### Method Variants

#### Cross-Encoder Rerankers

| Model | Provider | Parameters | Languages | Max Tokens | License |
|-------|----------|-----------|-----------|------------|---------|
| `cross-encoder/ms-marco-MiniLM-L-6-v2` | sentence-transformers | 22M | English | 512 | Apache 2.0 |
| `cross-encoder/ms-marco-MiniLM-L-12-v2` | sentence-transformers | 33M | English | 512 | Apache 2.0 |
| `bge-reranker-v2-m3` | BAAI | 568M | Multilingual | 8192 | MIT |
| `bge-reranker-v2-gemma` | BAAI | 2.6B | Multilingual | 8192 | Gemma license |
| `jina-reranker-v2-base-multilingual` | Jina AI | 278M | 100+ languages | 1024 | CC-BY-NC 4.0 |
| `Cohere Rerank v3.5` | Cohere | — (API) | Multilingual | 4096 | Commercial |
| `FlashRank` (default model) | Prithiviraj | ~15M | English | 512 | Apache 2.0 |

#### Late Interaction Models (ColBERT)

Unlike cross-encoders, ColBERT stores **per-token vectors** for documents (pre-computable) and computes interaction scores at query time via MaxSim. This makes it faster than cross-encoders while more accurate than bi-encoders.

| Model | Provider | Notes |
|-------|----------|-------|
| `ColBERTv2` | Stanford | Original late interaction model |
| `answerai-colbert-small-v1` | Answer.AI | Lightweight ColBERT |
| `jina-colbert-v2` | Jina AI | Multilingual ColBERT |

#### LLM-based Reranking

Using an LLM to rerank by asking it to judge relevance (zero-shot or few-shot):

```
Given the query: "{query}"
Rate the relevance of this document on a scale of 1–5:
"{document}"
```

| Method | Description |
|--------|-------------|
| **Pointwise** | Score each document independently (1–5 rating) |
| **Listwise** | Give the LLM all documents, ask it to rank them |
| **Pairwise** | Compare documents in pairs, aggregate preferences |
| **RankGPT** | LLM-based listwise reranking with sliding window |

### Key Papers

| Paper | Authors | Link | Contribution |
|-------|---------|------|-------------|
| **ColBERT: Efficient and Effective Passage Search via Contextualized Late Interaction over BERT** | Khattab & Zaharia, 2020 | [arXiv:2004.12832](https://arxiv.org/abs/2004.12832) | Introduced late interaction — per-token embeddings with MaxSim scoring |
| **ColBERTv2: Effective and Efficient Retrieval via Lightweight Late Interaction** | Santhanam et al., 2021 | [arXiv:2112.01488](https://arxiv.org/abs/2112.01488) | Residual compression for ColBERT, 6x storage reduction |
| **Is ChatGPT Good at Search? Investigating Large Language Models as Re-Ranking Agents** | Sun et al., 2023 | [arXiv:2304.09542](https://arxiv.org/abs/2304.09542) | Systematic evaluation of LLM-based reranking |
| **RankGPT** | Sun et al., 2023 | [arXiv:2304.09542](https://arxiv.org/abs/2304.09542) | Permutation-based listwise reranking with GPT |
| **Passage Re-ranking with BERT** | Nogueira & Cho, 2019 | [arXiv:1901.04085](https://arxiv.org/abs/1901.04085) | First demonstration of BERT cross-encoder for reranking |

### Open-Source Implementations

| Repository | Description | Link |
|-----------|-------------|------|
| sentence-transformers (CrossEncoder) | Python cross-encoder training and inference | [github.com/UKPLab/sentence-transformers](https://github.com/UKPLab/sentence-transformers) |
| FlagEmbedding (reranker) | BAAI reranker models | [github.com/FlagOpen/FlagEmbedding](https://github.com/FlagOpen/FlagEmbedding) |
| RAGatouille | Python wrapper for ColBERTv2 | [github.com/AnswerDotAI/RAGatouille](https://github.com/AnswerDotAI/RAGatouille) |
| FlashRank | Ultra-lightweight reranking library (< 100MB) | [github.com/PrithivirajDamodaran/FlashRank](https://github.com/PrithivirajDamodaran/FlashRank) |
| Hugging Face TEI (rerank mode) | Production reranking server with batching | [github.com/huggingface/text-embeddings-inference](https://github.com/huggingface/text-embeddings-inference) |
| rerankers | Unified Python API for multiple reranking models | [github.com/AnswerDotAI/rerankers](https://github.com/AnswerDotAI/rerankers) |
| Stanford ColBERT | Reference ColBERT implementation | [github.com/stanford-futuredata/ColBERT](https://github.com/stanford-futuredata/ColBERT) |

### Enterprise Products

| Product | Provider | Notes |
|---------|----------|-------|
| Cohere Rerank API | Cohere | `rerank-v3.5`, multilingual, used by Oracle/Dell/SAP. $2/1000 queries |
| AWS Bedrock Rerank | Amazon | Integrated with Bedrock Knowledge Bases |
| Azure AI Search Semantic Ranker | Microsoft | Built-in cross-encoder reranking |
| Google Vertex AI Ranking API | Google | LLM-based reranking service |
| Jina Reranker API | Jina AI | Pay-per-use multilingual reranking |

### When to Use

**Always use reranking in production.** The accuracy uplift is substantial (typically 5–15% improvement in recall@10) and the latency cost is manageable if you limit the number of candidates.

**Use cross-encoder reranking when:**
- You have top-K candidates from retrieval and need to pick the best 5–10 for the LLM context
- Precision matters more than recall (you've already cast a wide net)
- You can afford ~100–300ms additional latency

**Use ColBERT (late interaction) when:**
- You need reranker-level quality but with faster inference
- Your corpus is large enough that cross-encoder reranking is too slow
- You want to pre-compute document token embeddings

**Use LLM-based reranking when:**
- You need the absolute highest quality and latency is not a concern
- You're doing complex queries (multi-hop, reasoning-heavy)
- Budget allows for LLM API costs per query

**Sizing guideline:**

| Candidates | Cross-encoder (GPU) | Cross-encoder (CPU) | ColBERT |
|-----------|---------------------|---------------------|---------|
| 20 | ~30 ms | ~200 ms | ~15 ms |
| 50 | ~60 ms | ~500 ms | ~30 ms |
| 100 | ~120 ms | ~1000 ms | ~50 ms |

### kube-llmops Integration Path

```yaml
retrieval:
  reranker:
    enabled: true
    model: BAAI/bge-reranker-v2-m3
    server: text-embeddings-inference   # TEI supports reranking
    replicas: 1
    resources:
      requests:
        nvidia.com/gpu: 1               # Strongly recommended for reranking
        memory: 4Gi
    candidates: 20                      # Rerank top-20 from hybrid retrieval
    returnTopK: 5                       # Return top-5 to LLM context
    maxTokens: 512                      # Truncate long passages
    batchSize: 32                       # Process 32 pairs per batch
    timeout: 500ms                      # Hard timeout for reranking
    fallback: skip                      # If reranker fails: skip | use_retrieval_order
```

**Two-stage pipeline in kube-llmops:**

```
┌───────────┐     ┌──────────────┐     ┌──────────────┐     ┌───────────┐
│   Query    │────►│  Hybrid      │────►│  Reranker    │────►│   LLM     │
│            │     │  Retrieval   │     │  (TEI)       │     │  Context  │
│            │     │  Top-50      │     │  Top-5       │     │           │
└───────────┘     └──────────────┘     └──────────────┘     └───────────┘
                   ~15 ms               ~60 ms (GPU)          ~1000 ms
```

---

## 3.5 Embedding Fine-tuning

### What It Is

Off-the-shelf embedding models are trained on general-purpose data (web text, Wikipedia, MS MARCO). While they work well for general retrieval, domain-specific applications — legal documents, medical records, codebase search, financial filings — often benefit from **fine-tuning** the embedding model on domain data. Fine-tuning adapts the model's internal representations so that documents which are relevant in *your* domain are close together in embedding space.

The standard approach uses **contrastive learning**: given a query, you provide a positive (relevant) passage and one or more negative (irrelevant) passages, and train the model to push the positive closer and negatives further away. The biggest challenge is usually **data generation** — creating high-quality (query, positive, negative) training triples. Modern approaches use LLMs to generate synthetic training data, reducing the need for manual annotation.

### Method Variants

#### Training Objectives

| Method | Description | Data Requirement |
|--------|-------------|------------------|
| **Contrastive Loss** (InfoNCE) | Maximize similarity of positive pair, minimize for negatives | (query, positive, negatives) triples |
| **Multiple Negatives Ranking Loss (MNRL)** | In-batch negatives: use other positives in batch as negatives | (query, positive) pairs only |
| **Triplet Loss** | `max(0, sim(q, neg) - sim(q, pos) + margin)` | (query, positive, negative) triples |
| **Cosine Similarity Loss** | Regress similarity to target score | (sentence_a, sentence_b, score) |
| **Distillation** | Transfer cross-encoder scores to bi-encoder | Unlabeled data + cross-encoder teacher |
| **RLHF for Embeddings** | Use human feedback to optimize retrieval quality | Human preference data |

#### Synthetic Data Generation

Use an LLM to generate training data from your corpus:

```python
# Step 1: Generate synthetic queries for each document chunk
prompt = f"""Given this document passage, generate 5 diverse questions
that this passage could answer:

Passage: {chunk_text}

Questions:"""

# Step 2: Use (synthetic_query, chunk) as positive pairs
# Step 3: Mine hard negatives using current embedding model
# Step 4: Fine-tune with contrastive loss
```

#### Matryoshka Representation Learning (MRL)

Train embeddings that are useful at **multiple dimensionalities**. A 1024-dim MRL embedding can be truncated to 256-dim with minimal quality loss, enabling flexible storage/compute tradeoffs at deployment time.

```python
from sentence_transformers import SentenceTransformer
from sentence_transformers.losses import MatryoshkaLoss, MultipleNegativesRankingLoss

model = SentenceTransformer("BAAI/bge-large-en-v1.5")
inner_loss = MultipleNegativesRankingLoss(model)
loss = MatryoshkaLoss(model, inner_loss, matryoshka_dims=[1024, 512, 256, 128, 64])
```

#### GritLM: Generative Representational Instruction Tuning

Unifies text generation and embedding in a single model, enabling the same model to serve as both the retriever and the generator.

### Key Papers

| Paper | Authors | Link | Contribution |
|-------|---------|------|-------------|
| **Matryoshka Representation Learning** | Kusupati et al., 2022 | [arXiv:2205.13147](https://arxiv.org/abs/2205.13147) | Embeddings that work at multiple dimensions via nested loss |
| **GritLM: Generative Representational Instruction Tuning** | Muennighoff et al., 2024 | [arXiv:2402.09906](https://arxiv.org/abs/2402.09906) | Unifies generation and embedding in one model |
| **Improving Text Embeddings with Large Language Models** | Wang et al., 2024 | [arXiv:2401.00368](https://arxiv.org/abs/2401.00368) | Using LLMs for synthetic training data generation |
| **ANCE: Approximate Nearest Neighbor Negative Contrastive Learning** | Xiong et al., 2020 | [arXiv:2007.00808](https://arxiv.org/abs/2007.00808) | Hard negative mining from ANN index during training |
| **GPL: Generative Pseudo Labeling** | Wang et al., 2022 | [arXiv:2112.07577](https://arxiv.org/abs/2112.07577) | Domain adaptation using generated queries + cross-encoder pseudo labels |
| **Sentence-BERT fine-tuning** | Reimers & Gurevych, 2019 | [arXiv:1908.10084](https://arxiv.org/abs/1908.10084) | Foundation for fine-tuning sentence embeddings |

### Open-Source Implementations

| Repository | Description | Link |
|-----------|-------------|------|
| sentence-transformers | Full training API for fine-tuning embedding models | [github.com/UKPLab/sentence-transformers](https://github.com/UKPLab/sentence-transformers) |
| FlagEmbedding | BAAI's training pipeline for BGE models | [github.com/FlagOpen/FlagEmbedding](https://github.com/FlagOpen/FlagEmbedding) |
| LlamaIndex Fine-tuning | Embedding fine-tuning with RAG evaluation | [github.com/run-llama/llama_index](https://github.com/run-llama/llama_index) |
| Unstructured | Data processing for generating training pairs | [github.com/Unstructured-IO/unstructured](https://github.com/Unstructured-IO/unstructured) |
| Tevatron | Dense retrieval training toolkit (DPR, ANCE, etc.) | [github.com/texttron/tevatron](https://github.com/texttron/tevatron) |

### Enterprise Products

| Product | Provider | Notes |
|---------|----------|-------|
| Cohere Embed Fine-tuning | Cohere | Fine-tune Cohere embed models on your data |
| OpenAI Fine-tuning | OpenAI | Fine-tune embeddings via API (limited) |
| Jina Fine-tuning | Jina AI | Fine-tune Jina embeddings |
| Hugging Face AutoTrain | Hugging Face | No-code embedding fine-tuning |

### When to Use

**Fine-tune embeddings when:**
- Off-the-shelf models underperform on your domain (measured by retrieval recall@K)
- You have **domain-specific vocabulary** not well-represented in pre-training data (legal, medical, scientific)
- You have at least ~1,000 query-passage pairs (or can generate synthetic ones)
- You need to improve retrieval for a **specific language** not well-covered by multilingual models
- The performance gap between your bi-encoder and a cross-encoder baseline is large (fine-tuning closes this gap)

**Don't fine-tune when:**
- Off-the-shelf models already achieve > 90% recall@20 on your eval set
- You have < 100 documents (not enough to learn from)
- Your domain is general knowledge (Wikipedia, web content)
- You're still iterating on chunking strategy — fix chunking first

**Typical improvement:** 5–20% improvement in recall@10 over base model, depending on domain specificity.

### kube-llmops Integration Path

```yaml
# Fine-tuning job definition
finetuning:
  embedding:
    enabled: true
    baseModel: BAAI/bge-large-en-v1.5
    trainingData:
      # Option 1: Provide your own training data
      source: pvc                     # pvc | s3 | gcs
      path: /data/training/pairs.jsonl
      format: triplet                  # triplet | pair | scored_pair

      # Option 2: Generate synthetic training data
      synthetic:
        enabled: true
        generator: gpt-4o-mini        # LLM for query generation
        corpus: rag_chunks             # Generate from indexed chunks
        queriesPerChunk: 3
        hardNegativeMining: true       # Use current model to find hard negatives

    training:
      loss: mnrl                       # mnrl | contrastive | triplet | matryoshka
      matryoshkaDims: [1024, 512, 256] # For MRL training
      epochs: 3
      batchSize: 32
      learningRate: 2e-5
      warmupSteps: 100
      evaluation:
        metric: ndcg@10
        evalSteps: 500

    output:
      modelName: bge-large-en-v1.5-finetuned
      registry: registry.kube-llmops.local
      # Auto-deploy fine-tuned model to TEI
      autoDeploy: true

    resources:
      requests:
        nvidia.com/gpu: 1
        memory: 16Gi
```

**Fine-tuning workflow in kube-llmops:**

```
┌──────────────────────────────────────────────────────────────────┐
│  Fine-tuning Pipeline (Kubernetes Job)                           │
│                                                                  │
│  1. Load corpus chunks from vector DB / object storage           │
│  2. Generate synthetic queries with LLM (if no labeled data)     │
│  3. Mine hard negatives from current embedding index              │
│  4. Fine-tune base model with contrastive loss                   │
│  5. Evaluate on held-out set (recall@K, NDCG)                    │
│  6. Push model to registry                                       │
│  7. Update TEI deployment with new model                         │
│  8. Re-index documents with new embeddings (background job)      │
└──────────────────────────────────────────────────────────────────┘
```

---

## 3.6 Multi-vector Retrieval (ColBERT / Late Interaction)

### What It Is

Multi-vector retrieval represents documents and queries not as single vectors, but as **sets of token-level vectors** — one embedding per token. The relevance score between a query and a document is computed as the sum of **maximum similarities** (MaxSim) between each query token vector and all document token vectors. This is known as **late interaction** because query and document tokens interact at scoring time rather than during encoding.

This architecture sits between bi-encoders (fast but coarse — one vector per text) and cross-encoders (accurate but slow — joint encoding). ColBERT, the seminal late interaction model, achieves **cross-encoder-level accuracy** while being orders of magnitude faster because document token vectors can be **pre-computed and indexed**. The tradeoff is storage: storing one vector per token requires significantly more space than one vector per document (though ColBERTv2's residual compression reduces this by 6x).

### Method Variants

#### ColBERT Architecture

```
Query:  "how to fix OOM in kubernetes"
         ↓ BERT encoder
         [v_how, v_to, v_fix, v_OOM, v_in, v_kubernetes]  ← 6 query token vectors

Document: "Kubernetes pod out-of-memory troubleshooting guide..."
           ↓ BERT encoder (pre-computed)
           [v_Kubernetes, v_pod, v_out, v_of, v_memory, ...]  ← N doc token vectors

Score = Σ      max    cos(q_i, d_j)
       i∈query  j∈doc

     = max_sim(v_how, doc_vecs) + max_sim(v_to, doc_vecs) + ...
```

#### Variants

| Model | Description | Storage | Quality |
|-------|-------------|---------|---------|
| **ColBERT v1** | Original: full per-token embeddings (128-dim) | ~100 bytes/token | High |
| **ColBERTv2** | Residual compression: centroids + residual codes | ~16 bytes/token (6x reduction) | High (same as v1) |
| **PLAID** | Performance-optimized ColBERT engine | Same as v2, faster retrieval | Same |
| **Jina ColBERT v2** | Multilingual ColBERT, 8192 token context | Variable | High, multilingual |
| **answerai-colbert-small-v1** | Compact ColBERT model | Smaller | Good |

#### ColBERT vs. Other Approaches

| Aspect | Bi-encoder | ColBERT (Late Interaction) | Cross-encoder |
|--------|-----------|---------------------------|---------------|
| Query encoding | Independent | Independent | Joint |
| Doc encoding | Independent, pre-computed | Independent, pre-computed (per-token) | Joint (query-dependent) |
| Scoring | Single dot product | MaxSim over token pairs | Full attention |
| Speed (1M docs) | ~5 ms | ~50 ms | Infeasible (must score all) |
| Speed (rerank 100) | N/A | ~15 ms | ~120 ms |
| Quality (MSMARCO) | MRR@10 ~0.33 | MRR@10 ~0.40 | MRR@10 ~0.42 |
| Storage per doc | 1 vector (768 floats) | ~100 vectors (128 floats each) | None |
| Storage (1M docs) | ~3 GB | ~30 GB (v1) / ~5 GB (v2) | 0 |

### Key Papers

| Paper | Authors | Link | Contribution |
|-------|---------|------|-------------|
| **ColBERT: Efficient and Effective Passage Search via Contextualized Late Interaction over BERT** | Khattab & Zaharia, 2020 | [arXiv:2004.12832](https://arxiv.org/abs/2004.12832) | Introduced late interaction paradigm; per-token vectors with MaxSim |
| **ColBERTv2: Effective and Efficient Retrieval via Lightweight Late Interaction** | Santhanam et al., 2021 | [arXiv:2112.01488](https://arxiv.org/abs/2112.01488) | Residual compression (6x storage reduction), cross-encoder distillation |
| **PLAID: An Efficient Engine for Late Interaction Retrieval** | Santhanam et al., 2022 | [arXiv:2205.09707](https://arxiv.org/abs/2205.09707) | Centroid-based pruning for faster ColBERT retrieval |
| **Moving Beyond Downstream Task Accuracy for Information Retrieval Benchmarking** | Khattab et al., 2023 | [arXiv:2212.01340](https://arxiv.org/abs/2212.01340) | Comprehensive evaluation of ColBERT across diverse benchmarks |

### Open-Source Implementations

| Repository | Description | Link |
|-----------|-------------|------|
| ColBERT (Stanford) | Reference implementation of ColBERT and ColBERTv2 | [github.com/stanford-futuredata/ColBERT](https://github.com/stanford-futuredata/ColBERT) |
| RAGatouille | Pythonic wrapper for ColBERT — simplest way to use it | [github.com/AnswerDotAI/RAGatouille](https://github.com/AnswerDotAI/RAGatouille) |
| Vespa | Production search engine with native ColBERT support | [github.com/vespa-engine/vespa](https://github.com/vespa-engine/vespa) |
| Jina ColBERT | Multilingual ColBERT models | [huggingface.co/jinaai/jina-colbert-v2](https://huggingface.co/jinaai/jina-colbert-v2) |

**Quick start with RAGatouille:**

```python
from ragatouille import RAGPretrainedModel

# Load pre-trained ColBERTv2
RAG = RAGPretrainedModel.from_pretrained("colbert-ir/colbertv2.0")

# Index your documents
RAG.index(
    collection=[doc1, doc2, doc3, ...],
    index_name="my_index",
    max_document_length=256,
    split_documents=True
)

# Search
results = RAG.search(query="how to fix OOM in kubernetes", k=10)
# Returns: [{"content": "...", "score": 0.85, "rank": 1}, ...]
```

**Using Vespa for production ColBERT:**

```
# Vespa schema with ColBERT support
schema doc {
  document doc {
    field text type string { ... }
    field colbert type tensor<int8>(token{}, v[16]) {
      indexing: attribute
    }
  }
  rank-profile colbert {
    inputs {
      query(qt) tensor<float>(querytoken{}, v[128])
    }
    first-phase {
      expression {
        sum(
          reduce(
            sum(
              query(qt) * unpack_bits(attribute(colbert)), v
            ), max, token
          ), querytoken
        )
      }
    }
  }
}
```

### Enterprise Products

| Product | Provider | ColBERT Support |
|---------|----------|-----------------|
| Jina AI | Jina AI | Jina ColBERT v2 models + API |
| Vespa Cloud | Yahoo/Vespa | Native ColBERT indexing and ranking |
| Relevance AI | Relevance AI | Multi-vector search support |

### When to Use

**Use ColBERT / multi-vector retrieval when:**
- You need **higher accuracy than bi-encoder** but **faster than cross-encoder**
- You can afford the **storage overhead** (5–30x more than single-vector, depending on compression)
- Your queries are **complex** (multi-aspect, long queries)
- You want to use it as a **first-stage retriever** replacing or complementing bi-encoder (ColBERT can search millions of docs directly)

**Avoid ColBERT when:**
- Storage is very constrained (e.g., edge deployment)
- You have billions of documents (storage becomes prohibitive even with compression)
- Simplicity is paramount — ColBERT adds operational complexity

**Recommended use in RAG pipeline:**

| Configuration | When |
|---------------|------|
| ColBERT as reranker only | Small-medium corpus, replace cross-encoder for faster reranking |
| ColBERT as first-stage retriever | Medium corpus (< 10M docs), highest quality retrieval |
| ColBERT + BM25 hybrid | Large corpus, combine ColBERT's semantic strength with BM25's lexical precision |

### kube-llmops Integration Path

```yaml
retrieval:
  colbert:
    enabled: true
    model: colbert-ir/colbertv2.0
    mode: reranker               # reranker | retriever | hybrid

    # As reranker (most common in RAG)
    reranker:
      candidates: 50             # Re-score top-50 from hybrid retrieval
      returnTopK: 10
      maxQueryTokens: 32
      maxDocTokens: 256

    # As retriever (higher quality, more storage)
    retriever:
      indexPath: /data/colbert/indexes
      nprobe: 10                 # PLAID parameter
      ncells: 4
      storage:
        size: 50Gi               # Plan for ~30x single-vector storage
        storageClass: fast-ssd

    resources:
      requests:
        nvidia.com/gpu: 1
        memory: 8Gi
```

---

## Summary: Choosing Your Retrieval Stack

### Decision Matrix

```
Start here:
│
├─ Do you need semantic understanding?
│  ├─ Yes → Dense Retrieval (3.1)
│  └─ No, just keyword matching → Sparse Retrieval (3.2)
│
├─ Can you run both dense + sparse?
│  └─ Yes → Hybrid Retrieval (3.3) ← RECOMMENDED DEFAULT
│
├─ Do you need high precision in top-5?
│  └─ Yes → Add Reranking (3.4) ← STRONGLY RECOMMENDED
│
├─ Are off-the-shelf models underperforming on your domain?
│  └─ Yes → Embedding Fine-tuning (3.5)
│
└─ Do you need better-than-bi-encoder quality and can afford storage?
   └─ Yes → Multi-vector / ColBERT (3.6)
```

### Recommended Production Stack

For most kube-llmops deployments, we recommend this **four-stage retrieval pipeline**:

```
┌───────────────────────────────────────────────────────────────────────┐
│                                                                       │
│  User Query                                                           │
│       │                                                               │
│       ▼                                                               │
│  ┌─────────────────┐                                                  │
│  │ Query Processing │  (Query rewriting, expansion — see Chapter 4)   │
│  └────────┬────────┘                                                  │
│           │                                                           │
│     ┌─────┴─────┐                                                     │
│     ▼           ▼                                                     │
│  ┌──────┐  ┌──────┐                                                   │
│  │Dense │  │Sparse│   Stage 1: Parallel retrieval                     │
│  │Top-50│  │Top-50│   (pgvector + tsvector on same PostgreSQL)        │
│  └──┬───┘  └──┬───┘                                                   │
│     └────┬────┘                                                       │
│          ▼                                                            │
│  ┌──────────────┐                                                     │
│  │ RRF Fusion   │     Stage 2: Hybrid fusion → Top-20                 │
│  │ (k=60)       │                                                     │
│  └──────┬───────┘                                                     │
│         ▼                                                             │
│  ┌──────────────┐                                                     │
│  │ Cross-encoder│     Stage 3: Reranking → Top-5                      │
│  │ Reranker     │     (bge-reranker-v2-m3 via TEI)                    │
│  └──────┬───────┘                                                     │
│         ▼                                                             │
│  ┌──────────────┐                                                     │
│  │ LLM Context  │     Stage 4: Pack top-5 chunks into prompt          │
│  │ Assembly     │                                                     │
│  └──────────────┘                                                     │
│                                                                       │
└───────────────────────────────────────────────────────────────────────┘
```

**Total latency budget:**

| Stage | Time (GPU) | Time (CPU) |
|-------|-----------|-----------|
| Embedding query | ~5 ms | ~30 ms |
| Dense search (pgvector HNSW) | ~10 ms | ~10 ms |
| Sparse search (tsvector) | ~5 ms | ~5 ms |
| RRF fusion | ~1 ms | ~1 ms |
| Reranking (20 candidates) | ~30 ms | ~200 ms |
| **Total retrieval** | **~50 ms** | **~250 ms** |

This is well within the typical LLM inference time (1–5 seconds), meaning retrieval is **not the bottleneck**.

---

*Next: [Chapter 4 — Query Processing & Transformation](./04-query-processing.md)*
*Previous: [Chapter 2 — Indexing & Storage](./02-indexing.md)*
