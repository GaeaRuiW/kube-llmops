# ADR-0004: Dify as RAG Platform

## Status

Accepted

## Context

Building a production RAG pipeline from scratch requires implementing document
ingestion, chunking, prompt orchestration, conversation memory, and a user
interface. Maintaining these components in-house diverts effort from model
tuning and evaluation.

## Decision

We adopt Dify as the RAG application layer. Dify provides a visual workflow
editor, built-in knowledge-base management, and an end-user chat interface that
connects to LiteLLM via the OpenAI-compatible API.

## Consequences

- Rapid prototyping: non-engineers can build and iterate on RAG workflows.
- Vendor coupling: upgrading Dify requires tracking its release cadence.
- Custom logic (e.g., re-ranking, hybrid search) must fit Dify's plugin model.
