# Changelog

**English** | [中文](CHANGELOG.zh-CN.md)

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
- Umbrella Helm chart with 12 sub-charts
- 4 deployment profiles: ci, minimal, standard, production
- One-liner install script (`scripts/install.sh`)
- 3 CI workflows: lint, test, build
- Comprehensive README with credentials table

### Fixed
- LiteLLM api_base missing `/v1` suffix (broke all model routing)
- Langfuse v3 breaking change (pinned to v2.95.11)
- Grafana dashboard PVC path conflict
- Langfuse Next.js not binding to 0.0.0.0 (broke port-forward)
- Langfuse NEXTAUTH_URL redirect to internal K8s URL
- GPU rolling update deadlock (Recreate strategy)

### Known Issues
- DCGM Exporter may not work in WSL2 environments
- FlashAttention 2 hangs on Blackwell (SM 12.0) GPUs — use `--attention-backend TRITON_ATTN`
- Helm SSA may not update ConfigMaps on upgrade (workaround: delete ConfigMap first)
