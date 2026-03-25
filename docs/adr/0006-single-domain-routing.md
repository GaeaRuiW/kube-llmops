# ADR-0006: Single-Domain Path-Based Routing for Dify

## Status

Accepted

## Context

Dify exposes a web UI, a public-facing API, and an internal console on
different ports. Allocating separate subdomains for each increases certificate
management overhead and complicates CORS configuration for single-page
applications.

## Decision

We expose all Dify services under one domain using path-based Ingress rules:
`/` for the web app, `/v1` for the service API, and `/console/api` for the
management console. A single TLS certificate covers the entire domain.

## Consequences

- One wildcard or SAN cert simplifies TLS renewal and secret management.
- Path conflicts require careful ordering of Ingress rules by specificity.
- Future addition of non-Dify paths (e.g., `/grafana`) is straightforward.
