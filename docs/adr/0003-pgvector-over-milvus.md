# ADR-0003: pgvector over Milvus as Default Vector Store

## Status

Accepted

## Context

The RAG pipeline requires a vector store for embedding retrieval. Milvus offers
high-throughput ANN search but needs its own etcd, MinIO, and multiple
microservices. Many teams already run PostgreSQL for application state and
prefer minimizing operational surface area.

## Decision

We default to pgvector backed by the shared PostgreSQL instance. Milvus remains
available as an opt-in sub-chart (`milvus.enabled: true`) for teams that need
billion-scale vector search or dedicated GPU-accelerated indexing.

## Consequences

- Fewer pods and simpler backups; vector data lives alongside relational data.
- pgvector IVFFlat/HNSW indices are adequate up to ~10 M vectors.
- Teams exceeding that threshold must switch to Milvus and re-index.
