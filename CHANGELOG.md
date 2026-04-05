# Changelog

**English** | [中文](CHANGELOG.zh-CN.md)

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.5.0] - 2026-04-05

### Added
- Latency-based routing (default strategy, replacing simple-shuffle)
- Prefix caching flag per model (`prefixCaching: true`)
- Multi-trigger KEDA autoscaling (queue depth + TTFT P95 + TPOT P95)
- SLO alert rules (TTFTSLOBreach, TTFTSLOCritical, TPOTSLOBreach)
- Scale-to-zero with LiteLLM fallback for cold start
- Spot/preemptible GPU tolerations (AWS, GCP, Azure, Karpenter)
- Graceful drain (terminationGracePeriodSeconds: 90 + preStop hook)
- MIG GPU device support (nvidia.com/mig-*)
- Canary model deployment with weight-based traffic splitting
- llm-d disaggregated serving (experimental, prefill/decode split)
- Multi-accelerator support (nvidia, amd, gaudi)
- 6 new documentation pages (routing, large models, speculative, kserve, llm-d, canary)
- SLO dashboard panels (TTFT/TPOT vs threshold, HPA replica count)
- Cost dashboard panels (GPU idle rate, scale-to-zero events)
- Canary dashboard panels (latency comparison, traffic weight)
- Prefix cache hit rate panel in vLLM dashboard

### Changed
- Default routing strategy: simple-shuffle -> latency-based-routing
- GPU resource names use helper function (supports nvidia/amd/gaudi)
- DCGM exporter conditional on nvidia accelerator

## [0.4.0] - 2026-04-04

### Added

#### Fine-tuning Pipeline (Argo Workflows + LLaMA-Factory)
- `finetune` subchart with Argo Workflows DAG: prepare-data → finetune → merge-upload → evaluate → quality-gate → deploy
- LLaMA-Factory integration for LoRA, QLoRA, and Full fine-tuning
- Data sources: MinIO (s3://), HuggingFace datasets, PVC mount
- Quality gate step with configurable metric thresholds (eval_loss, accuracy, bleu, rouge)
- Canary deployment via LiteLLM weight routing (configurable canary percentage)
- Human approval via webhook notifications (Slack/DingTalk/generic)
- ConfigMap-based training config generation from Helm values
- RBAC: ServiceAccount + ClusterRole for Argo workflow execution
- PodDisruptionBudget for MLflow
- Sample training data (`examples/finetune/sample-data.json`) in Alpaca format

#### MLflow Experiment Tracking
- MLflow Deployment with PostgreSQL backend + MinIO artifact store
- Reuses existing PostgreSQL (database: `mlflow`) and MinIO infrastructure
- Exposed via NodePort :30505 when `global.nodePort.enabled=true`
- Integrated into fine-tuning workflow for metric logging and model registry

#### JupyterHub (Interactive ML Development)
- JupyterHub subchart with KubeSpawner for GPU notebook environments
- 3 GPU profiles: cpu (default), gpu-small (1 GPU, 8Gi), gpu-large (2 GPUs, 16Gi)
- Keycloak OIDC SSO integration (auto-configured when keycloak.enabled)
- PodDisruptionBudget for hub availability
- NodePort :30888 when `global.nodePort.enabled=true`
- Enabled by default in `values-production.yaml`

#### Terraform Modules (Infrastructure as Code)
- `terraform/aws-eks/` — EKS cluster with GPU node group (g5.xlarge), EBS CSI, GP3 storage
- `terraform/gcp-gke/` — GKE Standard cluster with T4 GPU node pool, Workload Identity
- `terraform/azure-aks/` — AKS cluster with NC6s_v3 GPU pool, Azure CNI, Premium SSD
- All modules: NVIDIA GPU Operator, optional KEDA, kube-llmops Helm release
- Consistent GPU taint (`nvidia.com/gpu=present:NoSchedule`) across all clouds
- README per module with prerequisites, cost estimates, and teardown instructions

#### Grafana Dashboard
- Fine-tuning Pipeline dashboard (`finetune-overview`): job status, training loss, GPU utilization, step progress
- Total dashboards: 10 → 11

#### Model Loader Performance
- hf-transfer concurrency raised from 8 to 32 (`HF_TRANSFER_CONCURRENCY` env var)
- Configurable via `global.modelStore.hfTransferConcurrency` in values
- Applied to model-preload Job, model-loader init-containers, and finetune workflow steps

### Changed
- `_helpers.tpl`: `modelLoaderEnv` helper now includes `HF_TRANSFER_CONCURRENCY`
- `model-loader` Dockerfile: `ENV HF_TRANSFER_CONCURRENCY=32`
- `values-single-node.yaml`: added `global.modelStore.hfTransferConcurrency: 32`
- README install commands: use local chart path instead of Helm repo (GitHub Pages not yet published)
- Dashboard count in feature comparison table: 10 → 11
- KEDA feature description: removed inaccurate TTFT claim (only queue depth is implemented)

### Fixed
- README version banner was stale (v0.3.0 → v0.4.0)
- README Helm repo install commands referenced unpublished GitHub Pages repo
- ARCHITECTURE.md Phase 4 checkboxes now reflect implemented state
- README.zh-CN.md synced with all English README changes

## [0.3.1] - 2026-03-29

### Added

#### Engine Auto-Detection
- `resolveEngine` Helm template: auto-selects vllm/tei/llamacpp from model source name
- `resolveModelType` template: auto-detects embedding/reranker/llm for LiteLLM routing
- `global.models` unified list: define all models in one place, no per-subchart duplication
- `engine:` field is now optional (backward compatible when set explicitly)

#### Unified Model Distribution
- Pre-built `model-loader` Docker image (`images/model-loader/Dockerfile`)
- Model-loader init-container for all 3 engines (vllm, tei, llamacpp)
- MinIO-first download: check MinIO cache → fallback HuggingFace → upload back to MinIO
- `hf-transfer` multi-threaded downloads (Rust-based, 3-5x faster)
- Model-preload Helm Job (post-install/post-upgrade hook, batch populate MinIO)
- `global.hfToken` for gated models (single token for all engines)
- Configurable parallel workers and per-file concurrency
- Retry with resume for interrupted downloads

#### NodePort Access
- `global.nodePort.enabled=true` exposes all services on fixed ports (30400-30909)
- NodePort SSO: OIDC URLs auto-computed from `global.nodePort.host`
- 7 services exposed: LiteLLM, Grafana, Langfuse, Dify, Keycloak, Prometheus, MinIO

#### System Monitoring
- Node Exporter DaemonSet (`quay.io/prometheus/node-exporter:v1.9.0`)
- Kube State Metrics Deployment (`rancher/mirrored-kube-state-metrics:v2.15.0`)
- System Overview Grafana dashboard: CPU, memory, disk, network, pod count, resource table
- Total dashboards: 9 → 10

#### Developer Experience
- `examples/curl/` — API usage examples (chat, embedding, rerank, health, traces)
- `examples/python/` — Python SDK examples (chat, streaming, Langfuse tracing)
- Prompt A/B Quality Comparison panel in RAG Quality dashboard
- `/kube-llmops` Devin skill with 9 subcommands

### Changed
- `values-single-node.yaml`: models defined in `global.models` (was per-subchart)
- `values-single-node.yaml`: `global.modelStore` config for MinIO endpoint
- All subchart templates: read `global.models` with fallback to `.Values.models`
- Prometheus: replaced custom cAdvisor scrape with node-exporter + kube-state-metrics
- Prometheus RBAC: scoped to pod/node/service/endpoint (removed nodes/proxy)

### Fixed
- Helm NOTES.txt nil pointer when dify/rag-eval not in values
- vLLM drop-cache init-container ordering (after model-loader)
- xet protocol stall on arm64 with large models
- Prompt A/B panel: switched from barchart to bargauge (instant queries)

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
