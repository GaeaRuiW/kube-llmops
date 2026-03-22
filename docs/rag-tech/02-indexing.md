# 2. Indexing & Knowledge Organization

**English** | [Table of Contents](./README.md)

> **Layer summary:** Before a RAG system can retrieve anything, raw documents must be transformed into a searchable knowledge base. This chapter covers the full pipeline: parsing diverse file formats into clean text, splitting that text into retrieval-friendly chunks, enriching chunks with metadata, building hierarchical and graph-based indexes, and implementing parent-child retrieval strategies. Together these techniques determine the **quality ceiling** of every downstream retrieval and generation step.

---

## Table of Contents

- [2.1 Document Parsing](#21-document-parsing)
- [2.2 Chunking Strategies](#22-chunking-strategies)
- [2.3 Metadata Enrichment](#23-metadata-enrichment)
- [2.4 Hierarchical Indexing](#24-hierarchical-indexing)
- [2.5 Knowledge Graph RAG (GraphRAG)](#25-knowledge-graph-rag-graphrag)
- [2.6 Parent Document Retriever](#26-parent-document-retriever)

---

## 2.1 Document Parsing

### What It Is

Document parsing is the foundational step of any RAG pipeline: extracting clean, structured text from heterogeneous source formats such as PDF, Word (.docx), PowerPoint (.pptx), Excel (.xlsx), HTML, images, and scanned documents. The challenge is deceptively hard -- PDFs lack semantic structure (they are essentially instructions for drawing glyphs on a page), tables span multiple pages, images contain embedded text, and real-world documents mix all of these together in unpredictable layouts.

A poor parsing stage silently poisons every downstream component. Garbled table data, lost headings, merged paragraphs, and OCR errors all degrade chunking quality, embedding fidelity, and ultimately the accuracy of generated answers. Production RAG systems therefore invest heavily in robust, format-aware parsing pipelines that preserve document structure (headings, lists, tables, captions) alongside raw text.

### Method Variants

| Approach | Description | Strengths | Limitations |
|---|---|---|---|
| **Rule-based extraction** | Use format-specific libraries (python-docx, openpyxl, BeautifulSoup) to traverse the document object model | Fast, deterministic, no ML dependency | Brittle with non-standard layouts; each format needs its own code |
| **PDF-native parsing** | Parse the PDF instruction stream to extract text runs and positions (PyMuPDF, pdfplumber, pdfminer) | Lightweight, pure Python | Cannot handle scanned PDFs; poor table extraction; loses reading order in multi-column layouts |
| **OCR-based parsing** | Convert pages to images, then apply OCR (Tesseract, PaddleOCR, EasyOCR) | Works on scanned docs and images | Slower; accuracy depends on image quality and language; no structural understanding |
| **Layout-aware deep learning** | Use vision models to detect document regions (titles, paragraphs, tables, figures) then extract text per region. Models: LayoutLM, LayoutLMv2, DiT, YOLO-based detectors | Preserves document structure; handles complex layouts | Requires GPU; model training/fine-tuning needed for specialized domains |
| **Multi-modal / VLM parsing** | Use vision-language models (GPT-4V, Gemini, Qwen-VL) to directly "read" document pages as images | Highest accuracy on complex layouts; understands context | Expensive at scale; latency; API dependency |
| **Unified ETL pipeline** | Orchestrate multiple extractors behind a single API: detect format, choose optimal extractor, post-process | Production-ready; handles any format | More complex to deploy; multiple dependencies |

### Key Papers

| Paper | Year | Link | Contribution |
|---|---|---|---|
| **LayoutLM: Pre-training of Text and Layout for Document AI** (Xu et al.) | 2020 | [arXiv:1912.13318](https://arxiv.org/abs/1912.13318) | Joint pre-training of text + 2D positional layout for document understanding |
| **LayoutLMv3: Pre-training for Document AI with Unified Text and Image Masking** (Huang et al.) | 2022 | [arXiv:2204.08387](https://arxiv.org/abs/2204.08387) | Unified multi-modal architecture for document AI tasks |
| **DiT: Self-supervised Pre-training for Document Image Transformer** (Li et al.) | 2022 | [arXiv:2203.02378](https://arxiv.org/abs/2203.02378) | Self-supervised vision transformer for document layout analysis |
| **Nougat: Neural Optical Understanding for Academic Documents** (Blecher et al., Meta) | 2023 | [arXiv:2308.13418](https://arxiv.org/abs/2308.13418) | End-to-end transformer for academic PDF to Markdown conversion |
| **Table Transformer (TATR)** (Smock et al.) | 2022 | [arXiv:2110.00061](https://arxiv.org/abs/2110.00061) | DETR-based table detection and structure recognition |

### Open Source Implementations

| Project | License | GitHub | Description |
|---|---|---|---|
| **Unstructured** | Apache 2.0 | [Unstructured-IO/unstructured](https://github.com/Unstructured-IO/unstructured) | Most popular open-source document ETL. Supports 20+ formats (PDF, DOCX, PPTX, XLSX, HTML, EML, images, etc.). Provides `partition_*` functions per format plus a universal `partition()` auto-router. Has an API server mode for microservice deployment. Actively maintained with ~10k GitHub stars. |
| **RAGFlow DeepDoc** | Apache 2.0 | [infiniflow/ragflow](https://github.com/infiniflow/ragflow) | Deep document understanding engine integrated into the RAGFlow RAG platform. Features layout detection (text, table, figure, header/footer recognition), table structure extraction, OCR via PaddleOCR, and intelligent merge of OCR results with PDF-native text. Particularly strong on Chinese documents. |
| **Apache Tika** | Apache 2.0 | [apache/tika](https://github.com/apache/tika) | Java-based, the most mature document parser. Supports 1000+ MIME types. Battle-tested in enterprise search (Solr, Elasticsearch). Can run as a REST server (`tika-server`). Text extraction is reliable but output is flat (less structural information than newer tools). |
| **Docling** | MIT | [DS4SD/docling](https://github.com/DS4SD/docling) | By IBM Research. Uses AI models for document layout analysis and TableFormer for table structure recognition. Excels at scientific papers and technical documents. Outputs structured JSON with reading order, table cells, and figure references. Growing community (~5k stars). |
| **MinerU** | AGPL-3.0 | [opendatalab/MinerU](https://github.com/opendatalab/MinerU) | By Shanghai AI Lab. High-quality document parsing with strong support for Chinese and multilingual documents. Features PDF-to-Markdown conversion, layout detection, formula recognition, and table extraction. Integrated with the OpenDataLab ecosystem. |
| **PyMuPDF (fitz)** | AGPL-3.0 | [pymupdf/PyMuPDF](https://github.com/pymupdf/PyMuPDF) | Fast Python binding for MuPDF. Good for text extraction, page rendering, and metadata. Lightweight and fast but limited structural understanding. |
| **pdfplumber** | MIT | [jsvine/pdfplumber](https://github.com/jsvine/pdfplumber) | Built on pdfminer.six. Good for table extraction from native PDFs using visual line detection. Simple API. |
| **Marker** | GPL-3.0 | [VikParuchuri/marker](https://github.com/VikParuchuri/marker) | Converts PDF to Markdown using a pipeline of deep learning models (layout detection, OCR, table recognition). High accuracy on academic and technical documents. |
| **Nougat** | MIT | [facebookresearch/nougat](https://github.com/facebookresearch/nougat) | Meta's end-to-end transformer that converts academic PDF pages directly to Markdown. Excellent for LaTeX-heavy scientific papers. |

### Enterprise Products

| Product | Vendor | Key Features |
|---|---|---|
| **Amazon Textract** | AWS | OCR + forms + tables + queries. AnalyzeDocument API detects layout, tables, key-value pairs. Pay per page. Integrated with S3 and Lambda. |
| **Azure Document Intelligence** (formerly Form Recognizer) | Microsoft | Pre-built models (invoices, receipts, ID cards) + custom models. Layout API extracts text, tables, structure with bounding boxes. SDK for Python/JS/C#/.NET. |
| **Google Document AI** | GCP | Specialized processors for different document types. OCR, form parsing, document classification. AutoML for custom extractors. |
| **Unstructured API** (SaaS) | Unstructured.io | Hosted version of the open-source library with higher accuracy models, faster processing, and no infrastructure management. |
| **Reducto** | Reducto | API-first document parsing with focus on accuracy. Strong table extraction and layout understanding. |

### When to Use

- **Quick prototype / simple PDFs:** Use PyMuPDF or pdfplumber. Fast, lightweight, zero ML dependencies.
- **Production multi-format pipeline:** Use Unstructured. Broadest format coverage, active community, API server mode.
- **Scientific / academic papers:** Use Docling or Nougat. Both understand LaTeX, equations, and complex figure/table layouts.
- **Chinese-heavy documents:** Use MinerU or RAGFlow DeepDoc. Purpose-built for CJK text and Chinese document layouts.
- **Scanned documents / images:** Use cloud APIs (Textract, Azure Document Intelligence) or Marker. OCR accuracy and table handling are critical here.
- **Maximum accuracy on complex layouts:** Combine layout detection (DiT/LayoutLM) + OCR + LLM post-processing. Most expensive but most accurate.

### kube-llmops Integration Path

```
Recommended deployment in kube-llmops:

1. Deploy Unstructured API server as a Kubernetes Deployment:
   - Image: quay.io/unstructured-io/unstructured-api
   - Mount document storage via PV or connect to MinIO (S3-compatible)
   - Expose via ClusterIP Service for internal pipeline access
   - Set resource requests/limits (CPU-heavy; GPU optional for hi-res strategy)

2. For OCR workloads, deploy PaddleOCR or Tesseract as a sidecar or
   separate service, referenced by the parsing pipeline.

3. Pipeline integration:
   - CronJob or Argo Workflow triggers parsing on new documents in MinIO
   - Parsed output (structured JSON/elements) stored back to MinIO
   - Downstream chunking service consumes parsed output

4. Helm values example:
   documentParsing:
     enabled: true
     engine: unstructured   # or "docling", "tika"
     replicas: 2
     resources:
       requests: { cpu: "2", memory: "4Gi" }
       limits:   { cpu: "4", memory: "8Gi" }
     storage:
       type: minio
       bucket: raw-documents
```

---

## 2.2 Chunking Strategies

### What It Is

Chunking is the process of splitting parsed documents into smaller, retrieval-appropriate segments (chunks). It is one of the most impactful yet underappreciated decisions in RAG system design. The chunk size and splitting strategy directly control the trade-off between **retrieval precision** (small chunks match specific queries) and **context completeness** (large chunks provide sufficient information for generation).

Too-small chunks lose context and fragment meaning; too-large chunks dilute relevance signals and waste the LLM's context window on irrelevant text. The optimal strategy depends on document type, query patterns, embedding model capabilities (most models perform best at 256-512 tokens), and the downstream LLM's context length. Modern RAG systems increasingly combine multiple chunking strategies or use adaptive approaches.

### Method Variants

#### 2.2.1 Fixed-Size Chunking

Split text by a fixed character or token count with a configurable overlap window.

- **How it works:** Walk through the text, cut every N characters/tokens, and optionally overlap the last M characters/tokens with the next chunk to preserve cross-boundary context.
- **Typical settings:** 512-1024 tokens per chunk, 50-200 token overlap.
- **Pros:** Dead simple, deterministic, fast, works on any text.
- **Cons:** Splits mid-sentence, mid-paragraph, even mid-word (if character-based). Ignores semantic boundaries entirely.
- **Best for:** Quick prototypes, when document structure is unknown or absent.

#### 2.2.2 Recursive Text Splitting

LangChain's default and most widely used strategy. Attempts to split by the largest semantic unit first, falling back to smaller units.

- **How it works:** Define a hierarchy of separators (e.g., `["\n\n", "\n", ". ", " ", ""]`). Try splitting by double-newline (paragraph). If any resulting chunk exceeds the target size, recursively split it by single newline (line), then by sentence, then by space, and finally by character.
- **Pros:** Respects paragraph and sentence boundaries as much as possible while guaranteeing chunks stay under the size limit.
- **Cons:** Heuristic-based; separators must be tuned for different languages and formats.
- **Best for:** General-purpose text, most RAG applications.

#### 2.2.3 Semantic Chunking

Use embedding similarity to detect natural semantic breakpoints in the text.

- **How it works:** Split text into sentences. Compute embeddings for each sentence (or sliding window of sentences). Calculate cosine similarity between consecutive sentence embeddings. Where similarity drops below a threshold (or drops significantly relative to neighbors), insert a chunk boundary.
- **Variants:**
  - *Percentile-based:* Break when similarity falls below the Nth percentile of all consecutive similarities.
  - *Standard deviation-based:* Break when similarity is more than K standard deviations below the mean.
  - *Interquartile range:* Break when similarity falls below Q1 - 1.5 * IQR.
  - *Gradient-based:* Look at the rate of change in similarity rather than absolute values.
- **Key reference:** Greg Kamradt's "5 Levels of Text Splitting" ([YouTube](https://www.youtube.com/watch?v=8OJC21T2SL4), [Notebook](https://github.com/FullStackRetrieval-com/RetrievalTutorials/blob/main/tutorials/LevelsOfTextSplitting/5_Levels_Of_Text_Splitting.ipynb))
- **Pros:** Produces semantically coherent chunks; chunk boundaries align with topic shifts.
- **Cons:** Requires an embedding model call per sentence (slower, more expensive); results vary by embedding model; non-deterministic.
- **Best for:** Long-form narrative documents, articles, books where topic boundaries are implicit.

#### 2.2.4 Document-Structure-Aware Chunking

Parse the document's structural elements (headings, sections, lists, tables) first, then chunk within those structural units.

- **How it works:** Use the document parser to identify headings (H1, H2, H3...), sections, lists, tables, and code blocks. Create chunk boundaries at structural transitions. If a section exceeds the target size, apply recursive splitting within that section.
- **Variants:**
  - *Markdown-based:* Split by markdown headings (`#`, `##`, `###`). LangChain's `MarkdownHeaderTextSplitter`.
  - *HTML-based:* Split by HTML header tags. LangChain's `HTMLHeaderTextSplitter`, `HTMLSectionSplitter`.
  - *Code-based:* Split by functions, classes, methods. LangChain's `Language`-aware splitters for Python, JS, Java, etc.
  - *LaTeX-based:* Split by `\section`, `\subsection`, etc.
- **Pros:** Chunks respect the author's intended organization; metadata (section title, heading hierarchy) comes for free.
- **Cons:** Requires reliable document structure parsing; not all documents have clear structure.
- **Best for:** Technical documentation, legal documents, structured reports, codebases.

#### 2.2.5 Agentic Chunking (LLM-Based)

Use an LLM to decide where to split and optionally generate a summary or proposition for each chunk.

- **How it works:** Present text to an LLM with a prompt like "Identify natural topic boundaries" or "Split this into self-contained propositions." The LLM outputs chunk boundaries or reformulated propositions.
- **Variants:**
  - *Proposition-based:* Decompose text into atomic propositions (single facts). Paper: "Dense X Retrieval" (Chen et al., arXiv:2312.06648).
  - *Topic-based:* LLM identifies topic shifts and splits accordingly.
  - *Summary-augmented:* Each chunk gets an LLM-generated summary prepended for better embedding.
- **Pros:** Highest semantic quality; can normalize and deduplicate information.
- **Cons:** Extremely expensive (LLM call per chunk); slow; non-deterministic; hard to scale.
- **Best for:** High-value, low-volume knowledge bases where quality justifies the cost.

#### 2.2.6 Late Chunking

Embed the full document first with a long-context embedding model, then chunk the embedding sequence.

- **How it works:** Pass the entire document through a long-context embedding model (e.g., jina-embeddings-v2 with 8192 token context). The model produces contextualized token embeddings for the full document. Then apply pooling over spans (chunks) of the token embedding sequence to produce chunk embeddings.
- **Key reference:** Jina AI's "Late Chunking" blog post and implementation.
- **Pros:** Each chunk embedding is contextualized by the full document; no information loss at boundaries.
- **Cons:** Requires long-context embedding model; more complex pipeline.
- **Best for:** When chunks need full-document context for disambiguation.

### Key Papers

| Paper | Year | Link | Contribution |
|---|---|---|---|
| **Dense X Retrieval: What Retrieval Granularity Should We Use?** (Chen et al.) | 2023 | [arXiv:2312.06648](https://arxiv.org/abs/2312.06648) | Proposes proposition-based retrieval: decompose text into atomic propositions for finer-grained retrieval |
| **Five Levels of Chunking** (Greg Kamradt) | 2023 | [Notebook](https://github.com/FullStackRetrieval-com/RetrievalTutorials) | Practical taxonomy: character, recursive, document-based, semantic, agentic |
| **Late Chunking** (Jina AI) | 2024 | [Blog](https://jina.ai/news/late-chunking-in-long-context-embedding-models/) | Contextual chunk embeddings from long-context models |
| **An Evaluation of Chunking Strategies for RAG** (Nair et al.) | 2024 | [arXiv:2406.15547](https://arxiv.org/abs/2406.15547) | Systematic comparison of chunking strategies across multiple RAG benchmarks |

### Open Source Implementations

| Project | License | GitHub | Description |
|---|---|---|---|
| **LangChain Text Splitters** | MIT | [langchain-ai/langchain](https://github.com/langchain-ai/langchain) | `RecursiveCharacterTextSplitter`, `MarkdownHeaderTextSplitter`, `HTMLHeaderTextSplitter`, `HTMLSectionSplitter`, `LatexTextSplitter`, `PythonCodeTextSplitter`, `TokenTextSplitter`, `SemanticChunker`, and more. The most comprehensive collection. Also available as standalone `langchain-text-splitters` package. |
| **Chonkie** | MIT | [bhavnicksm/chonkie](https://github.com/bhavnicksm/chonkie) | Lightweight, blazing-fast chunking library. Supports token, word, sentence, semantic, and SDPM (Semantic Double-Pass Merge) chunking. No heavy dependencies. Designed as an alternative to LangChain splitters when you only need chunking. |
| **LlamaIndex Node Parsers** | MIT | [run-llama/llama_index](https://github.com/run-llama/llama_index) | `SentenceSplitter`, `SemanticSplitterNodeParser`, `HierarchicalNodeParser`, `MarkdownNodeParser`, `HTMLNodeParser`, `CodeSplitter`, `TokenTextSplitter`. Integrates with LlamaIndex's node/index abstraction. |
| **Semantic Text Splitter** | MIT | [benbrandt/text-splitter](https://github.com/benbrandt/text-splitter) | Rust-based (with Python bindings) semantic text splitter. Fast and supports multiple tokenizers (tiktoken, Hugging Face). |
| **Unstructured Chunking** | Apache 2.0 | [Unstructured-IO/unstructured](https://github.com/Unstructured-IO/unstructured) | `chunk_by_title()` function that groups elements by section title, respecting structural boundaries from the parser. |

### Enterprise Products

| Product | Chunking Capabilities |
|---|---|
| **Dify** | 4 built-in strategies: automatic, custom (fixed-size with overlap), paragraph-based, and QA-extraction mode (LLM generates Q&A pairs per chunk). UI for adjusting chunk size and overlap. |
| **RAGFlow** | Template-based chunking with format-specific templates (General, Q&A, Table, Paper, Book, Laws, Presentation, Picture, One, Knowledge Graph). Each template applies domain-optimized parsing and chunking rules. |
| **AWS Bedrock Knowledge Bases** | Automatic chunking (default), fixed-size chunking (configurable), and no chunking modes. Integrated with S3 data sources and OpenSearch Serverless / Pinecone / Redis. |
| **LlamaCloud** | Managed parsing and chunking with LlamaParse (proprietary parser). Automatic structure-aware chunking. Connected to LlamaIndex framework. |
| **Cohere** | Built-in document chunking in their Embed + Rerank pipeline. |

### When to Use

| Scenario | Recommended Strategy | Chunk Size Guidance |
|---|---|---|
| Quick prototype | Fixed-size or Recursive | 512 tokens, 100 overlap |
| General documents (mixed formats) | Recursive text splitting | 500-1000 tokens, 100-200 overlap |
| Technical docs / wikis | Document-structure-aware | Varies by section; cap at 1000 tokens |
| Legal / regulatory | Document-structure-aware + metadata | Per-section, preserving clause numbers |
| Narrative / articles | Semantic chunking | 300-800 tokens, let similarity decide |
| Code repositories | Language-aware (function/class level) | Per-function or per-class |
| High-value enterprise KB | Agentic / proposition-based | 1-3 sentences per proposition |
| FAQ / Q&A datasets | QA-pair extraction (Dify-style) | One Q&A pair per chunk |

**Golden rule:** Always evaluate chunking quality empirically on your actual queries. Use metrics like retrieval recall@k and end-to-end answer accuracy to compare strategies.

### kube-llmops Integration Path

```
Chunking as a configurable pipeline stage in kube-llmops:

1. Chunking runs as a processing step between document parsing and embedding.
   Implemented as a Kubernetes Job or pipeline step in Argo Workflows.

2. Configuration via Helm values:
   chunking:
     strategy: recursive        # Options: fixed, recursive, semantic, structural, agentic
     chunkSize: 512             # Target tokens per chunk
     chunkOverlap: 100          # Overlap tokens
     embeddingModel: ""         # Required only for semantic chunking
     language: "en"             # Affects sentence boundary detection
     # Strategy-specific settings:
     semantic:
       breakpointThreshold: percentile  # percentile | stddev | iqr | gradient
       bufferSize: 1
     structural:
       headingLevels: [1, 2, 3]
       respectTables: true

3. The chunking service reads parsed elements from MinIO, applies the
   configured strategy, and writes chunks (with metadata) back to MinIO
   for the embedding stage.

4. For semantic chunking, the embedding model service (already deployed
   for retrieval) is reused via ClusterIP for sentence embeddings.
```

---

## 2.3 Metadata Enrichment

### What It Is

Metadata enrichment is the process of attaching structured attributes to each chunk beyond its raw text content. These attributes -- source file name, page number, section title, creation date, author, document type, language, custom tags, and more -- transform a flat collection of text chunks into a richly annotated, filterable knowledge base.

In retrieval, metadata enables **filtered search** (sometimes called "pre-filtering" or "post-filtering" depending on the vector database): instead of searching the entire corpus, the retriever can narrow the search space to only chunks matching specific metadata criteria. For example, "find relevant chunks about deployment, but only from documents published in 2024, authored by the platform team, and tagged as 'production-ready'." This dramatically improves both precision and performance.

### Method Variants

#### 2.3.1 Extraction-Based Metadata

Automatically extract metadata from the document itself during parsing.

| Metadata Field | Source | Extraction Method |
|---|---|---|
| File name, path, format | File system | Trivial -- read from file properties |
| Page number, line number | Parser | Tracked during parsing (e.g., `element.metadata.page_number` in Unstructured) |
| Title, headings | Document structure | Parse heading elements; use as `section_title` metadata |
| Author, creation date | Document properties | Read from DOCX/PDF metadata fields (e.g., `pdf.metadata`, `python-docx` core properties) |
| Language | Text content | Detect with `langdetect`, `fasttext`, or `lingua-py` |
| Table/figure indicator | Parser | Flag chunks that contain or were extracted from tables or figures |
| URL / source link | Crawler | Attach the source URL if documents were web-crawled |

#### 2.3.2 LLM-Generated Metadata

Use an LLM to generate richer metadata that cannot be extracted mechanically.

- **Topic / category classification:** "Classify this chunk into one of: [architecture, deployment, monitoring, security]."
- **Summary:** Generate a one-line summary of the chunk.
- **Hypothetical questions:** "What questions would this chunk answer?" (Used in HyDE-like approaches.)
- **Entity extraction:** Extract named entities (people, organizations, technologies, dates) and attach as metadata.
- **Sentiment / tone:** Relevant for customer feedback or review data.
- **Key reference:** LlamaIndex's `MetadataExtractor` and `SummaryExtractor`, `KeywordExtractor`, `QuestionsAnsweredExtractor`, `EntityExtractor`.

#### 2.3.3 User-Defined / Business Metadata

Metadata injected from external systems, not derivable from the document text.

- **Access control tags:** Department, classification level, project code.
- **Workflow status:** Draft, reviewed, approved, deprecated.
- **Custom taxonomies:** Product line, customer segment, geographic region.
- **Version / revision:** Document version, Git commit hash for code-derived docs.

### Key Papers

| Paper | Year | Link | Contribution |
|---|---|---|---|
| **Self-RAG: Learning to Retrieve, Generate, and Critique** (Asai et al.) | 2023 | [arXiv:2310.11511](https://arxiv.org/abs/2310.11511) | Metadata-like "reflection tokens" for self-assessment of retrieval relevance |
| **Precise Zero-Shot Dense Retrieval without Relevance Labels (HyDE)** (Gao et al.) | 2022 | [arXiv:2212.10496](https://arxiv.org/abs/2212.10496) | Generating hypothetical documents as a form of query-time metadata enrichment |

### Open Source Implementations

| Project | Feature | Details |
|---|---|---|
| **LlamaIndex** | `MetadataExtractor` pipeline | Plug-in extractors: `TitleExtractor`, `SummaryExtractor`, `QuestionsAnsweredExtractor`, `KeywordExtractor`, `EntityExtractor`. Runs at index time. |
| **LangChain** | Document metadata | `Document` objects carry a `metadata` dict populated during loading. Splitters propagate metadata. `SelfQueryRetriever` translates natural language filters into metadata queries. |
| **Unstructured** | Element-level metadata | Every parsed element carries metadata: `filename`, `page_number`, `coordinates`, `text_as_html`, `languages`, `parent_id`, etc. Metadata flows through chunking. |
| **Haystack** | `Document` metadata + filters | Haystack's `Document` dataclass supports arbitrary metadata. All retrievers accept `filters` parameter for metadata-based pre-filtering. |

### Enterprise Products

All major vector databases support metadata filtering:

| Vector Database | Metadata Filtering |
|---|---|
| **Pinecone** | Rich metadata filtering with `$eq`, `$ne`, `$gt`, `$lt`, `$in`, `$nin` operators. Integrated with namespaces for coarse partitioning. |
| **Weaviate** | `where` filters with boolean operators. Supports filtering by any property type. |
| **Qdrant** | Payload-based filtering with full boolean logic, range queries, geo filters, nested object filters. Filterable indices for performance. |
| **Milvus / Zilliz** | Scalar field filtering alongside vector search. Supports creating scalar indexes for fast filtering. |
| **ChromaDB** | `where` and `where_document` filters using `$eq`, `$ne`, `$gt`, `$lt`, `$in`, `$nin`. |
| **pgvector (PostgreSQL)** | Full SQL `WHERE` clauses alongside `<=>` vector similarity. Most flexible filtering via SQL. |

### When to Use

- **Always.** There is no scenario where metadata should be skipped. At minimum, attach `source`, `page_number`, and `chunk_index`.
- **Filtered retrieval is mandatory** when the corpus contains documents from multiple sources, time periods, authors, or access levels.
- **LLM-generated metadata** (summaries, hypothetical questions) improves retrieval quality but adds cost. Use for high-value, stable knowledge bases (not rapidly changing data).
- **Standardize your metadata schema** early. Define a project-wide schema and enforce it in the ingestion pipeline. Inconsistent metadata makes filtering unreliable.

### kube-llmops Integration Path

```
Metadata enrichment in kube-llmops:

1. Define a metadata schema in the Helm values:
   metadata:
     schema:
       required: [source, page_number, chunk_index, ingestion_timestamp]
       optional: [author, section_title, language, document_type, version]
       custom: []   # User-defined fields
     llmEnrichment:
       enabled: false          # Enable LLM-based metadata extraction
       extractors: [summary, keywords, questions]
       model: "gpt-4o-mini"   # Use a cheap/fast model for metadata
       batchSize: 50

2. The metadata enrichment step runs as a post-chunking processor in the
   ingestion pipeline (Argo Workflows step).

3. Extraction-based metadata is populated during the parsing and chunking
   stages (zero additional cost).

4. LLM-generated metadata is batched and processed asynchronously via
   the LLM gateway (already deployed in kube-llmops).

5. All metadata is stored alongside vectors in the vector database,
   with appropriate scalar indexes created for frequently-filtered fields.
```

---

## 2.4 Hierarchical Indexing

### What It Is

Hierarchical indexing organizes chunks into a multi-level tree structure rather than a flat collection. The core idea is to create summaries at progressively higher levels of abstraction: individual chunks at the leaves, section summaries at intermediate nodes, and document-level summaries at the root. At retrieval time, the system can first identify relevant documents or sections via their summaries (cheap, coarse-grained search), then drill down into specific chunks within those sections (precise, fine-grained search).

This approach addresses a fundamental limitation of flat vector search: when a query requires information synthesized across multiple chunks or sections, flat retrieval often returns a fragmented, disconnected set of chunks. Hierarchical indexing provides the "zoomed-out" context that connects individual facts into a coherent narrative. It is particularly effective for long documents, multi-document corpora, and questions that require understanding the overall structure or theme of a document.

### Method Variants

#### 2.4.1 Summary-Based Hierarchy

- **Level 0 (leaves):** Original text chunks with embeddings.
- **Level 1:** Section/subsection summaries. Generated by an LLM summarizing all chunks within a section.
- **Level 2:** Document-level summaries. Generated by summarizing all section summaries.
- **Retrieval:** Query is matched against all levels. High-level matches identify relevant documents/sections; leaf-level matches provide specific passages.

#### 2.4.2 RAPTOR (Recursive Abstractive Processing for Tree-Organized Retrieval)

The most well-known hierarchical indexing method. Builds a tree bottom-up using clustering and summarization.

- **How it works:**
  1. Start with leaf-level text chunks.
  2. Embed all chunks and cluster them (using Gaussian Mixture Models or K-Means).
  3. Summarize each cluster using an LLM to produce a new "parent" node.
  4. Embed the parent summaries and repeat clustering + summarization.
  5. Continue until a single root summary or a desired tree depth is reached.
- **Retrieval strategies:**
  - *Tree traversal:* Start at the root, compare query to children, recurse into most relevant branches (top-down).
  - *Collapsed tree:* Flatten all nodes (all levels) into a single index and retrieve from anywhere in the tree.
- **Key advantage:** No predefined document structure needed; the tree emerges from semantic clustering.

#### 2.4.3 Sliding Window + Merge Hierarchy

- Create overlapping windows of increasing size: sentence-level, paragraph-level, section-level.
- Retrieve at the finest grain, then merge overlapping or adjacent matches into larger context windows.
- LlamaIndex's `AutoMergingRetriever` implements this: if enough leaf chunks under a parent are retrieved, the parent node is returned instead.

#### 2.4.4 Multi-Granularity Indexing

- Create multiple separate indexes at different granularities (e.g., sentence-level, paragraph-level, document-level).
- Query all indexes in parallel and fuse results.
- More flexible but requires maintaining multiple indexes.

### Key Papers

| Paper | Year | Link | Contribution |
|---|---|---|---|
| **RAPTOR: Recursive Abstractive Processing for Tree-Organized Retrieval** (Sarthi et al.) | 2024 | [arXiv:2401.18059](https://arxiv.org/abs/2401.18059) | Introduces recursive clustering + summarization to build retrieval trees. Shows significant improvements on multi-step reasoning tasks (NarrativeQA, QASPER, QuALITY). |
| **From Local to Global: A Graph RAG Approach** (Edge et al., Microsoft) | 2024 | [arXiv:2404.16130](https://arxiv.org/abs/2404.16130) | Related: uses community summaries at multiple granularities for hierarchical retrieval over knowledge graphs. |
| **Walking Down the Memory Maze: Beyond Context Limit through Interactive Reading** (Chen et al.) | 2023 | [arXiv:2310.05029](https://arxiv.org/abs/2310.05029) | Hierarchical memory structure for reading long documents |

### Open Source Implementations

| Project | Component | Details |
|---|---|---|
| **LlamaIndex RAPTOR Pack** | `RaptorPack` | Official LlamaIndex implementation of RAPTOR. Uses GMM clustering + LLM summarization. Supports both tree traversal and collapsed tree retrieval. [llama-index-packs-raptor](https://github.com/run-llama/llama_index/tree/main/llama-index-packs/llama-index-packs-raptor) |
| **LlamaIndex TreeIndex** | `TreeIndex` | Built-in tree-structured index. Builds a tree of summaries over documents. Supports `TreeSelectLeafRetriever` (top-down traversal) and `TreeAllLeafRetriever`. |
| **LlamaIndex AutoMergingRetriever** | `AutoMergingRetriever` | Works with `HierarchicalNodeParser` to create parent-child chunk relationships. If a threshold fraction of a parent's children are retrieved, the parent node is returned. |
| **LangChain MultiVector Retriever** | `MultiVectorRetriever` | Store multiple representations (summary, questions, full doc) per document. Retrieve by any representation but return the full document. Can be used for hierarchical retrieval. |

### Enterprise Products

| Product | Hierarchical Indexing Feature |
|---|---|
| **LlamaCloud** | Managed RAPTOR-style hierarchical indexing. Automatic tree construction over uploaded documents. |
| **Pinecone** | Namespaces can be used for manual multi-level organization. No built-in hierarchy. |
| **Cohere** | Multi-step retrieval with reranking can approximate hierarchical benefits (coarse retrieval + fine rerank). |

### When to Use

- **Long documents (50+ pages):** Hierarchical indexing prevents losing the forest for the trees. Summaries provide the "big picture" that flat chunk retrieval misses.
- **Multi-document QA:** When answers require synthesizing information across multiple documents, document-level summaries help identify relevant sources.
- **Multi-hop reasoning:** Questions like "How does the company's 2023 revenue strategy differ from 2022?" require understanding both documents at a high level before finding specific comparisons.
- **Cost trade-off:** Building the hierarchy requires LLM calls (for summarization). This is a one-time index-time cost that pays off at query time through better retrieval quality.
- **NOT needed for:** Simple factoid lookup, FAQ-style Q&A, or small corpora (< 100 chunks) where flat retrieval works well.

### kube-llmops Integration Path

```
Hierarchical indexing in kube-llmops:

1. The hierarchy builder runs as an index-time batch Job/Argo step:
   - Input: chunks + embeddings from MinIO
   - Process: cluster (GMM/K-Means), summarize (via LLM gateway), embed summaries
   - Recurse until tree depth reached
   - Output: tree structure stored in vector DB (all levels) + tree metadata in PostgreSQL

2. Helm values:
   hierarchicalIndex:
     enabled: false            # Off by default (significant LLM cost)
     method: raptor            # Options: raptor, summary_hierarchy, auto_merging
     maxTreeDepth: 3
     clusteringMethod: gmm     # gmm | kmeans
     summarizationModel: "gpt-4o-mini"
     retrievalStrategy: collapsed_tree  # collapsed_tree | tree_traversal
     # auto_merging specific:
     mergeThreshold: 0.5       # Fraction of children that triggers parent merge
     chunkSizes: [2048, 512, 128]  # Parent > child > leaf sizes

3. At query time, the retriever queries all tree levels in the vector DB
   and applies the configured retrieval strategy (collapsed or traversal).
```

---

## 2.5 Knowledge Graph RAG (GraphRAG)

### What It Is

Knowledge Graph RAG (GraphRAG) enhances traditional vector-based retrieval by first extracting **entities** (people, organizations, concepts, technologies) and **relationships** (works-at, caused-by, part-of, related-to) from documents to build a knowledge graph, then using **graph traversal** and **community detection** alongside or instead of vector similarity for retrieval. This addresses a critical weakness of pure vector search: it struggles with multi-hop reasoning ("Who is the CEO of the company that acquired the maker of GPT?") and questions about relationships between entities.

The approach was popularized by Microsoft Research's GraphRAG paper (2024), which demonstrated that graph-based retrieval with community summaries significantly outperforms naive RAG on questions requiring global understanding of a corpus. Since then, multiple lightweight alternatives have emerged. GraphRAG is particularly powerful for datasets with rich entity relationships -- organizational knowledge, scientific literature, legal corpora, and codebases -- where understanding the connections between concepts is as important as understanding the concepts themselves.

### Method Variants

#### 2.5.1 Microsoft GraphRAG (Local + Global)

The most comprehensive GraphRAG approach, featuring two retrieval modes:

- **Graph construction:**
  1. Extract entities and relationships from each chunk using an LLM (entity extraction prompt).
  2. Build a knowledge graph from extracted triples (entity, relationship, entity).
  3. Run community detection (Leiden algorithm) to identify clusters of related entities.
  4. Generate LLM summaries for each community at multiple hierarchical levels.

- **Local search:** For specific, entity-centric queries. Starts from query-relevant entities, traverses their neighborhood in the graph, collects associated text chunks and community summaries, and generates an answer.
- **Global search:** For broad, thematic queries ("What are the main themes in this dataset?"). Aggregates community summaries using map-reduce: each community summary is scored for relevance, top summaries are combined, and a final answer is generated.

#### 2.5.2 LightRAG

A simpler, faster alternative to Microsoft GraphRAG.

- **Key simplifications:**
  - Uses a simpler entity/relationship extraction process.
  - Integrates both vector similarity and graph structure in a single retrieval step.
  - No community detection or multi-level summaries.
  - Supports incremental graph updates (add new documents without rebuilding).
- **Retrieval modes:** Naive (vector only), Local (entity neighborhood), Global (high-level keywords), Hybrid (combination).
- **Advantage:** Much faster indexing and lower LLM cost than Microsoft GraphRAG. Suitable for dynamic corpora.

#### 2.5.3 Graph + Vector Hybrid

Combine traditional vector retrieval with graph-based retrieval:

1. Vector search retrieves top-K chunks by embedding similarity.
2. Entity extraction on the query identifies key entities.
3. Graph traversal expands context by finding related entities and their associated chunks.
4. Merge and rerank results from both sources.

Implemented in: Neo4j GraphRAG for Python, LlamaIndex KnowledgeGraphIndex.

#### 2.5.4 Property Graph RAG

Use a property graph database (Neo4j, Amazon Neptune) as the primary knowledge store:

- Nodes represent entities with properties (name, type, description, embedding).
- Edges represent typed relationships with properties (weight, source, timestamp).
- Retrieval combines Cypher queries (structured) with vector similarity (unstructured).
- LlamaIndex `PropertyGraphIndex` implements this pattern.

#### 2.5.5 Ontology-Driven GraphRAG

Start with a predefined ontology (schema) that defines entity types and relationship types:

- Domain experts define the ontology (e.g., for medical: Patient, Disease, Treatment, Symptom, with relationships like has_symptom, treats, causes).
- LLM extraction is constrained to the ontology, improving precision.
- Retrieval can use SPARQL or Cypher with schema-aware traversal.

### Key Papers

| Paper | Year | Link | Contribution |
|---|---|---|---|
| **From Local to Global: A Graph RAG Approach to Query-Focused Summarization** (Edge et al., Microsoft Research) | 2024 | [arXiv:2404.16130](https://arxiv.org/abs/2404.16130) | Introduces community detection + hierarchical summarization for graph-based RAG. Local search for entity-centric queries, global search for thematic queries. State-of-the-art on corpus-level comprehension tasks. |
| **LightRAG: Simple and Fast Retrieval-Augmented Generation** (Guo et al., HKU) | 2024 | [arXiv:2410.05779](https://arxiv.org/abs/2410.05779) | Lightweight GraphRAG with dual-level retrieval (low-level entities + high-level concepts), incremental updates, simpler than Microsoft GraphRAG with competitive performance. |
| **Graph Retrieval-Augmented Generation: A Survey** (Peng et al.) | 2024 | [arXiv:2408.08921](https://arxiv.org/abs/2408.08921) | Comprehensive survey of Graph + RAG methods, taxonomy, and benchmarks. |
| **KnowledGPT: Enhancing LLMs with Retrieval and Storage Access on Knowledge Bases** (Wang et al.) | 2023 | [arXiv:2308.11761](https://arxiv.org/abs/2308.11761) | Framework for LLM interaction with knowledge graphs for retrieval |
| **G-Retriever: Retrieval-Augmented Generation for Textual Graph Understanding** (He et al.) | 2024 | [arXiv:2402.07630](https://arxiv.org/abs/2402.07630) | GNN-based retrieval over textual graphs combined with LLM generation |

### Open Source Implementations

| Project | License | GitHub | Description |
|---|---|---|---|
| **Microsoft GraphRAG** | MIT | [microsoft/graphrag](https://github.com/microsoft/graphrag) | Official implementation of the Microsoft Research paper. Python-based, uses LLM for entity extraction and community summarization. Supports local + global search. Configurable via YAML. ~20k stars. Significant LLM cost for indexing. |
| **LightRAG** | MIT | [HKUDS/LightRAG](https://github.com/HKUDS/LightRAG) | Simpler and faster GraphRAG. Supports multiple LLM backends (OpenAI, Ollama, etc.) and storage backends. Incremental graph updates. ~10k stars and growing fast. |
| **nano-graphrag** | MIT | [gusye1234/nano-graphrag](https://github.com/gusye1234/nano-graphrag) | Minimal (~800 lines) reimplementation of Microsoft GraphRAG. Easy to understand, customize, and extend. Great for learning and prototyping. |
| **neo4j-graphrag-python** | Apache 2.0 | [neo4j/neo4j-graphrag-python](https://github.com/neo4j/neo4j-graphrag-python) | Official Neo4j Python library for GraphRAG. Combines Cypher graph queries with vector similarity search over Neo4j. Entity extraction via LLM, storage in Neo4j, retrieval with graph context. |
| **LlamaIndex KnowledgeGraphIndex** | MIT | [run-llama/llama_index](https://github.com/run-llama/llama_index) | `KnowledgeGraphIndex` for triple extraction and graph-based retrieval. `PropertyGraphIndex` for richer property graphs. Supports Neo4j, Nebula Graph, and in-memory graph backends. |
| **LangChain + Neo4j** | MIT | [langchain-ai/langchain](https://github.com/langchain-ai/langchain) | `GraphCypherQAChain` generates Cypher queries from natural language. `Neo4jVector` combines vector search with graph context. Extensive Neo4j integration documentation. |
| **GraphRAG-SDK** | MIT | [FalkorDB/GraphRAG-SDK](https://github.com/FalkorDB/GraphRAG-SDK) | GraphRAG implementation using FalkorDB (Redis-compatible graph database). Ontology support, multi-agent orchestration. |

### Enterprise Products

| Product | Vendor | GraphRAG Features |
|---|---|---|
| **Neo4j AuraDB** | Neo4j | Managed graph database with vector search (since v5.11). Cypher + vector hybrid queries. GenAI integrations via neo4j-graphrag-python. |
| **Amazon Neptune Analytics** | AWS | Graph analytics + vector search on Neptune. Supports openCypher and Gremlin. Integrated with Bedrock for LLM-powered graph queries. |
| **Azure Cosmos DB (Gremlin API)** | Microsoft | Graph database with global distribution. Can be combined with Azure OpenAI for GraphRAG workloads. |
| **TigerGraph** | TigerGraph | Distributed graph database with built-in graph analytics. CoPilot feature for natural language to graph queries. |
| **Diffbot** | Diffbot | Auto-generates knowledge graphs from web content. Natural Language Graph Intelligence API for entity extraction and linking. |

### When to Use

| Scenario | Recommendation |
|---|---|
| **Multi-hop questions** ("What products are made by companies that John advises?") | Strong fit. Graph traversal naturally handles multi-hop. |
| **Entity-relationship queries** ("What is the relationship between X and Y?") | Perfect fit. This is what graphs are designed for. |
| **Corpus-level summarization** ("What are the main themes across all documents?") | Microsoft GraphRAG's global search excels here. |
| **Rapidly changing corpus** | Use LightRAG (supports incremental updates) rather than Microsoft GraphRAG (requires full reindex). |
| **Simple factoid lookup** | Overkill. Standard vector RAG is sufficient and much cheaper. |
| **Cost-sensitive deployment** | GraphRAG indexing requires many LLM calls (entity extraction for every chunk). Budget accordingly. LightRAG is cheaper than Microsoft GraphRAG. |
| **Documents with strong entity connections** (organizational docs, legal, medical, scientific) | High value. The graph structure captures connections that flat retrieval misses. |
| **Documents with few entities** (narratives, opinion pieces, creative writing) | Low value. Sparse graphs don't add much over vector retrieval. |

### kube-llmops Integration Path

```
GraphRAG in kube-llmops:

1. Graph database deployment (choose one):
   a. Neo4j Community (Helm chart: neo4j/neo4j) as a StatefulSet
   b. FalkorDB (Redis-compatible, lighter weight)
   c. In-memory graph (for small corpora, via LightRAG's built-in store)

2. GraphRAG indexing pipeline (Argo Workflow):
   Step 1: Document parsing + chunking (existing pipeline)
   Step 2: Entity/relationship extraction (LLM calls via gateway)
   Step 3: Graph construction (write to Neo4j/FalkorDB)
   Step 4: Community detection + summarization (for Microsoft GraphRAG mode)
   Step 5: Vector embedding of entities and summaries

3. Helm values:
   graphrag:
     enabled: false              # Significant infrastructure + LLM cost
     engine: lightrag            # Options: microsoft_graphrag, lightrag, neo4j_hybrid
     graphDatabase:
       type: neo4j               # neo4j | falkordb | in_memory
       persistence:
         size: 50Gi
         storageClass: standard
     entityExtraction:
       model: "gpt-4o-mini"      # Balance cost vs. quality
       batchSize: 100
     retrieval:
       mode: hybrid              # local | global | hybrid
       topK: 20
       graphDepth: 2             # How many hops to traverse

4. Query-time integration:
   - The retriever service queries both vector DB and graph DB
   - Results are merged and reranked before passing to the LLM
   - Graph context (entity relationships, community summaries) is
     formatted and injected into the LLM prompt alongside retrieved chunks
```

---

## 2.6 Parent Document Retriever

### What It Is

The Parent Document Retriever pattern addresses a core tension in RAG: small chunks produce more precise embedding matches (higher retrieval accuracy), but the retrieved context may be too narrow for the LLM to generate a complete, grounded answer. The solution is to **index small chunks for matching** but **return their larger parent document or section for context**.

Concretely, the system maintains two stores: (1) a vector store containing small chunk embeddings for retrieval, and (2) a document store containing the full parent documents (or larger sections). When a query matches a small chunk, the system looks up the chunk's parent ID and returns the full parent text to the LLM. This gives the LLM sufficient context without sacrificing retrieval precision. The pattern can be extended to multiple levels: match on sentence-level chunks, but return paragraph-level or section-level context, or even the entire document.

### Method Variants

#### 2.6.1 Simple Parent-Child

- **Index:** Split documents into large parent chunks (e.g., 2000 tokens), then split each parent into smaller child chunks (e.g., 400 tokens).
- **Store:** Child chunks are embedded and stored in the vector database with a `parent_id` metadata field. Parent chunks are stored in a separate document store (or the same DB with a flag).
- **Retrieve:** Search the vector store for matching child chunks. For each match, look up the parent by `parent_id`. Return the parent (deduplicated).
- **Implementation:** LangChain `ParentDocumentRetriever`.

#### 2.6.2 Auto-Merging Retriever

- **Index:** Create a hierarchy of chunk sizes (e.g., 2048 > 512 > 128 tokens) using `HierarchicalNodeParser`.
- **Store:** Only leaf (smallest) chunks are embedded in the vector store. All levels are stored in a document store with parent-child references.
- **Retrieve:** Search for matching leaf chunks. If a threshold fraction (e.g., > 50%) of a parent's children are among the results, "merge" them by returning the parent instead. This prevents returning many small, overlapping chunks when the answer spans a larger section.
- **Implementation:** LlamaIndex `AutoMergingRetriever`.

#### 2.6.3 Multi-Representation Indexing

- **Index:** For each parent document/section, generate multiple representations: the raw text, an LLM-generated summary, hypothetical questions the section answers, and keyword extractions.
- **Store:** Embed all representations in the vector store, each pointing to the same parent.
- **Retrieve:** Query matches against any representation, but the parent is always returned.
- **Implementation:** LangChain `MultiVectorRetriever`.

#### 2.6.4 Sentence Window Retrieval

- **Index:** Split into individual sentences. Each sentence is embedded.
- **Store:** Along with each sentence embedding, store a configurable "window" of surrounding sentences (e.g., 3 sentences before and after).
- **Retrieve:** Match on the single sentence embedding, but return the full window for context.
- **Implementation:** LlamaIndex `MetadataReplacementPostProcessor` with `SentenceWindowNodeParser`.

### Key Papers

| Paper | Year | Link | Contribution |
|---|---|---|---|
| **Dense X Retrieval: What Retrieval Granularity Should We Use?** (Chen et al.) | 2023 | [arXiv:2312.06648](https://arxiv.org/abs/2312.06648) | Demonstrates that retrieval granularity significantly impacts RAG quality. Proposes proposition-level retrieval (finest grain) with document-level return. |
| **RAPTOR** (Sarthi et al.) | 2024 | [arXiv:2401.18059](https://arxiv.org/abs/2401.18059) | Related: hierarchical tree with auto-merging for multi-level context retrieval |
| **In-Context Retrieval-Augmented Language Models** (Ram et al.) | 2023 | [arXiv:2302.00083](https://arxiv.org/abs/2302.00083) | Studies the effect of retrieved passage length and granularity on LLM generation quality |

### Open Source Implementations

| Project | Component | Details |
|---|---|---|
| **LangChain** | `ParentDocumentRetriever` | Splits documents into parent and child chunks. Child embeddings are stored in a vector store; parents in a `ByteStore` or `InMemoryStore`. Configurable parent/child splitters. Simple and effective. [Documentation](https://python.langchain.com/docs/how_to/parent_document_retriever/) |
| **LangChain** | `MultiVectorRetriever` | Generalizes parent retrieval: store summaries, hypothetical questions, or any derived representation in the vector store, all pointing to the original document. [Documentation](https://python.langchain.com/docs/how_to/multi_vector/) |
| **LlamaIndex** | `AutoMergingRetriever` | Works with `HierarchicalNodeParser` to create multi-level chunk hierarchies. Automatically merges child nodes into parent when retrieval density exceeds a threshold. |
| **LlamaIndex** | `SentenceWindowNodeParser` + `MetadataReplacementPostProcessor` | Index single sentences but retrieve a configurable window of surrounding sentences. Post-processor replaces the sentence with its window in the final response context. |
| **Haystack** | Custom pipeline | No built-in parent retriever, but easily implemented with Haystack's pipeline abstraction: store parent docs in a `DocumentStore`, child chunks in another, and join results. |

### Enterprise Products

| Product | Parent Retrieval Support |
|---|---|
| **LlamaCloud** | Built-in multi-level chunk management with automatic parent retrieval. |
| **Dify** | Supports parent-child chunk configuration in the knowledge base settings. |
| **Cohere** | Reranking can be combined with larger document retrieval for a similar effect: retrieve many small chunks, rerank, then expand context around top results. |
| **Pinecone** | No native parent retrieval, but metadata (`parent_id`) + secondary lookup pattern is well-documented. |

### When to Use

| Scenario | Variant | Why |
|---|---|---|
| **General RAG with medium-length docs** | Simple Parent-Child | Easy to implement, significant quality boost over flat retrieval. Start here. |
| **Long documents with dense information** | Auto-Merging | Prevents returning too many small overlapping chunks; automatically provides section-level context. |
| **Diverse query patterns** (specific + broad) | Multi-Representation | Summaries catch broad queries; raw text catches specific ones; all return the same parent. |
| **Conversational / QA over precise content** | Sentence Window | Best precision for factoid questions; surrounding window provides disambiguation context. |
| **Very small corpus** (< 50 docs) | Not needed. | Just index the full documents or large chunks. |

**Rule of thumb:** If you're using chunks smaller than 500 tokens for retrieval, you almost certainly benefit from a parent document strategy. The smaller the retrieval chunk, the more important the parent context becomes.

### kube-llmops Integration Path

```
Parent document retriever in kube-llmops:

1. The ingestion pipeline produces two outputs:
   a. Small child chunks → embedded and indexed in the vector database
   b. Large parent chunks → stored in the document store (MinIO or PostgreSQL)
   Child-to-parent mapping is maintained via parent_id metadata on each child chunk.

2. The retrieval service implements the lookup:
   a. Query → vector search on child chunks → top-K child matches
   b. Deduplicate parent_ids
   c. Fetch parent chunks from document store
   d. (Optional) Auto-merge if enough children share a parent

3. Helm values:
   parentRetriever:
     enabled: true
     strategy: parent_child       # Options: parent_child, auto_merging, multi_vector, sentence_window
     parentChunkSize: 2000        # Tokens
     childChunkSize: 400          # Tokens
     # auto_merging specific:
     mergeThreshold: 0.5
     hierarchyLevels: [2048, 512, 128]
     # sentence_window specific:
     windowSize: 5                # Sentences before and after
     # multi_vector specific:
     representations: [summary, questions]  # Generate summaries + hypothetical questions
     representationModel: "gpt-4o-mini"

4. Document store backend:
   documentStore:
     type: minio                  # minio | postgresql | redis
     bucket: parent-documents     # For MinIO
     # PostgreSQL option uses the existing kube-llmops PostgreSQL instance
```

---

## Summary: Choosing Your Indexing Strategy

The following decision matrix helps choose the right combination of techniques for your use case:

| Corpus Characteristic | Recommended Techniques | Complexity | LLM Cost |
|---|---|---|---|
| Small (< 100 docs), simple formats | Recursive chunking + metadata + flat vector index | Low | None |
| Medium (100-10K docs), mixed formats | Unstructured parsing + recursive chunking + metadata + parent retriever | Medium | None |
| Large (10K+ docs), needs precision | Structure-aware chunking + metadata + hierarchical indexing + parent retriever | Medium-High | Medium (summaries) |
| Entity-rich (org docs, legal, medical) | All above + GraphRAG (LightRAG for cost, Microsoft GraphRAG for quality) | High | High (entity extraction + summaries) |
| Multi-language, scanned docs | Cloud parsing (Textract/Azure) + semantic chunking + metadata (language filter) | Medium | Low-Medium |

**Start simple, measure, and add complexity only when retrieval quality demands it.** Every technique in this chapter has a cost -- in infrastructure, LLM API spend, latency, and maintenance burden. The best RAG systems are those that use the minimum complexity needed to meet their quality targets.

---

*Next: [03 - Embedding & Vector Search](./03-embedding.md)* | *Previous: [01 - Data Ingestion](./01-ingestion.md)*
