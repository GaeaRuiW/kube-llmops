# ADR-0002: LiteLLM as API Gateway

## Status

Accepted

## Context

Applications need a unified OpenAI-compatible endpoint regardless of which
inference backend is running. Exposing vLLM directly couples clients to a
specific backend and prevents features like key management, rate limiting, and
model aliasing without custom code.

## Decision

We deploy LiteLLM as the sole ingress point for LLM traffic. All clients,
including Dify, hit `/v1/chat/completions` on LiteLLM, which proxies to one or
more vLLM instances registered in its model configuration.

## Consequences

- Teams can swap or add backends (e.g., Ollama, TGI) without client changes.
- Adds a network hop and ~2-5 ms of proxy latency per request.
- LiteLLM becomes a single point of failure; horizontal replicas are recommended.
