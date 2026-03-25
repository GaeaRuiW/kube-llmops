# Changelog

**English** | [中文](CHANGELOG.zh-CN.md)

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.3.0] - 2026-03-25

### Added

#### RAG Infrastructure (Full Stack)
- Dify v1.13.2 full-stack deployment (API + Web + Worker + Plugin Daemon + Redis)
- Dify Plugin Daemon with PVC persistence and embedded `.difypkg` plugin
- Automated Setup Job: admin account creation, plugin install, LLM + embedding provider config
- Single-domain path-based Ingress routing for Dify (SameSite cookie auth compatible)
- TEI embedding service with `bge-small-en-v1.5` (384 dims) auto-download
- TEI reranking service with `bge-reranker-base`, `/rerank` endpoint
- LiteLLM embedding route (`huggingface/bge-small-en` + `drop_params: true`)

#### RAG Evaluation & Quality
- Ragas CronJob with 4 metrics: faithfulness, answer_relevancy, context_precision, context_recall
- 105-sample evaluation dataset (15 docs x 9 categories with ground truth)
- Ragas metrics → Pushgateway → Prometheus → Grafana pipeline
- Quality gate Helm pre-upgrade hook (blocks deployment on quality regression)
- 5 Prometheus alert rules: FaithfulnessLow/Critical, RelevancyLow, QualityRegression, EvalStale

#### RAG Safety & Enterprise
- LLM-Guard with PromptInjection scanner (blocks direct + subtle injection)
- Presidio PII detection + anonymization (EMAIL/PERSON/URL)
- LightRAG knowledge graph with Neo4j backend
- Milvus vector database (standalone, gRPC + HTTP + monitoring)
- Multi-tenant namespace isolation (ResourceQuota + NetworkPolicy per team)

#### Observability (9 Grafana Dashboards)
- RAG Quality dashboard (4 gauges + trend + history)
- Infrastructure ROI dashboard
- SLO Overview dashboard
- Tenant Overview dashboard
- Cost & Usage dashboard
- AlertManager integration with notification channels
- 6 additional dashboards (total 9: vLLM, LiteLLM, GPU, RAG Quality, Cost, SLO, Infra ROI, Tenant, Milvus)

#### Infrastructure Hardening (27 CTO Stories)
- PodDisruptionBudget for all 14+ components
- Credential randomization + `existingSecret` support
- Prometheus Kubernetes service discovery (RBAC + `kubernetes_sd_configs`)
- PostgreSQL split architecture (operator-pg + app-pg)
- `values.schema.json` for configuration validation
- NetworkPolicy completion (PG/Redis/MinIO/LiteLLM)
- Backup CronJob for PostgreSQL
- External Secrets Operator templates (Vault backend)
- ArgoCD Application + ApplicationSet (sync waves 1-6)
- Makefile with `dev`, `lint`, `test-infra`, `bench` targets
- 6 Architecture Decision Records (ADRs)
- 3 load test scripts + performance report template
- HA production profile with replicas + remote_write

#### Testing
- Playwright E2E: Model Provider (5/5 PASS) + RAG E2E (9/9 PASS)
- Smoke Test Job: 5/5 PASS (embedding + LLM + Langfuse + trace + reranker)

### Changed
- PostgreSQL image: `postgres:16-alpine` → `pgvector/pgvector:pg16` with auto `vector` extension
- PostgreSQL init script now creates 4 databases: litellm, langfuse, dify, dify_plugin
- `.gitignore` updated: added `Chart.lock`, `screenshots/`, `test-report` patterns

### Fixed
- Dify 401 auth issue: switched from cross-domain to single-domain path-based routing (SameSite=Lax cookies)
- LiteLLM TEI embedding: `huggingface/` prefix required (not `openai/`), `drop_params: true`, no `/v1` suffix
- Helm `.tgz` cache: subchart template changes were being overridden by stale archives

## [0.2.0] - 2026-03-21

### Added

#### LLM Tracing (Langfuse v3)
- Upgraded Langfuse v2 → v3 (3.160.0) with full infrastructure stack
- ClickHouse (24.12-alpine) for OLAP trace/analytics storage
- Redis (7-alpine) for async worker queue
- S3/MinIO integration for event and media blob storage
- `ENCRYPTION_KEY` support for sensitive data encryption
- MCP (Model Context Protocol) prompts feature

#### Infrastructure Automation
- PostgreSQL `extraDatabases` auto-creation via `/docker-entrypoint-initdb.d/`
- MinIO `defaultBuckets` auto-creation on startup (mkdir before server start)
- Idempotent init scripts (safe for restarts, uses IF NOT EXISTS)

#### Keycloak SSO
- Keycloak Helm sub-chart with auto-provisioned realm, clients, roles, and users
- OIDC clients for Grafana, Langfuse, MinIO, LiteLLM
- Traefik Ingress for all services (`*.llmops.local`)

### Changed
- Langfuse image: `2.95.11` → `3.160.0`
- Parent chart now uses subchart default tags instead of `latest`
- Removed stale `.tgz` chart packages (Helm now uses directory sources)

### Fixed
- Langfuse v3 ZodError on startup (root cause: missing S3 blob storage config)
- ClickHouse single-node setup (`CLICKHOUSE_CLUSTER_ENABLED=false`)
- vLLM Blackwell GPU crash: enabled `--enforce-eager` + `--attention-backend TRITON_ATTN`
- PostgreSQL `langfuse` database not auto-created on fresh deploy

## [0.1.0] - 2026-03-19

### Added

#### Model Serving
- vLLM sub-chart with GPU support, model caching (PVC), custom CA certs
- llama.cpp sub-chart for GGUF model serving
- TEI sub-chart for embedding model serving
- Model Resolver: auto-detect model format (GGUF→llama.cpp, GPTQ/AWQ→vLLM, embedding→TEI)
- Recreate deployment strategy for GPU workloads (prevents rolling update deadlock)
- Per-model `extraEnv` and `engineArgs` support

#### AI Gateway
- LiteLLM sub-chart with PostgreSQL backend
- Auto-generated LiteLLM config from `models[]` values
- API key authentication (master key)
- Multi-model routing with simple-shuffle strategy
- OpenAI-compatible `/v1/chat/completions` endpoint

#### Observability
- Prometheus with remote write receiver
- Grafana with 3 auto-provisioned dashboards (vLLM, LiteLLM Gateway, GPU)
- OpenTelemetry Collector (Prometheus scraping + OTLP receiver)
- DCGM Exporter for NVIDIA GPU metrics (optional)
- Loki datasource auto-configured in Grafana

#### LLM Tracing
- Langfuse v2 with auto-provisioning (LANGFUSE_INIT_* env vars)
- LiteLLM → Langfuse callback (traces with model, tokens, latency, cost)
- Configurable external URL for port-forward/ingress

#### Logging
- Fluent Bit DaemonSet for container log collection
- Loki for log storage and querying
- Grafana Loki datasource for log exploration

#### Autoscaling (templates, requires KEDA operator)
- KEDA ScaledObject per vLLM model deployment
- Prometheus triggers: requests waiting, TTFT P95

#### Distributed Cache (templates, requires Fluid operator)
- MinIO for S3-compatible model storage
- Fluid Dataset + AlluxioRuntime per model

#### Model Registry (templates, requires Harbor)
- Harbor credential ConfigMap + Secret
- Integration point for OCI model sources

#### Security (templates)
- NetworkPolicy: default deny + allow rules per component
- OIDC/SSO ConfigMap for Keycloak/Dex integration
- Grafana OIDC auto-configuration

#### Infrastructure
- Umbrella Helm chart with 14 sub-charts
- 4 deployment profiles: ci, minimal, standard, production
- One-liner install script (`scripts/install.sh`)
- 3 CI workflows: lint, test, build
- Comprehensive README with credentials table

### Fixed
- LiteLLM api_base missing `/v1` suffix (broke all model routing)
- Grafana dashboard PVC path conflict
- Langfuse Next.js not binding to 0.0.0.0 (broke port-forward)
- Langfuse NEXTAUTH_URL redirect to internal K8s URL
- GPU rolling update deadlock (Recreate strategy)

### Known Issues
- DCGM Exporter may not work in WSL2 environments
- Helm SSA may not update ConfigMaps on upgrade (workaround: delete ConfigMap first)
