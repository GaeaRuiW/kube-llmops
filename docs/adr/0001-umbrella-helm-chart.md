# ADR-0001: Umbrella Helm Chart

## Status

Accepted

## Context

Deploying the full LLMOps stack (PostgreSQL, Keycloak, LiteLLM, vLLM, Dify,
observability) requires coordinating multiple Helm releases with shared
configuration such as domain names, TLS secrets, and database credentials.
Managing individual `helm install` commands is error-prone and difficult to
reproduce across environments.

## Decision

We package all components as sub-charts of a single umbrella Helm chart. Shared
values are defined once at the top level and passed down via global values and
explicit sub-chart overrides.

## Consequences

- Atomic upgrades: a single `helm upgrade` updates every component together.
- Version coupling: bumping one sub-chart requires a full chart release.
- Operators can still disable any sub-chart via `<component>.enabled: false`.
