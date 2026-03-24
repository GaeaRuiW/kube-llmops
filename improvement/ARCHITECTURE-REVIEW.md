# kube-llmops Deep Architecture Review

> **Review by**: Senior System Architect (15Y+ experience in distributed systems, AI Infra, DevOps)
> **Date**: 2026-03-24
> **Scope**: Full codebase — Helm charts, CI/CD, observability, security, networking, testing
> **Version reviewed**: v0.2.0 (commit 06130ea)

---

## Executive Summary

kube-llmops is an ambitious Kubernetes-native LLMOps umbrella Helm chart that attempts to deploy a full-stack LLM infrastructure with a single `helm install`. The project demonstrates strong architectural vision (CNCF alignment, two-tier gateway, model auto-detection) and covers an impressive breadth of concerns (serving, gateway, observability, RAG, security, evaluation).

However, beneath the well-polished documentation lies a set of **structural risks** that would become **critical blockers** as the project scales toward production use. This review identifies 5 fatal-class issues, 8 high-severity concerns, and provides concrete refactoring recommendations for each.

**Overall Architecture Score: 6.5 / 10**
- Vision & documentation: 9/10
- Implementation maturity: 5/10
- Production readiness: 4/10
- Extensibility & maintainability: 6/10
- Security posture: 4/10

---

## Table of Contents

1. [Architecture Design & Rationality](#1-architecture-design--rationality)
2. [Technology Stack & Ecosystem](#2-technology-stack--ecosystem)
3. [Scalability & High Availability](#3-scalability--high-availability)
4. [Module Decoupling & Maintainability](#4-module-decoupling--maintainability)
5. [Security & Deployment Operations](#5-security--deployment-operations)
6. [Fatal Issues Summary](#6-fatal-issues-summary)
7. [Refactoring Roadmap](#7-refactoring-roadmap)
8. [Appendix: File-Level Findings](#appendix-file-level-findings)

---

## 1. Architecture Design & Rationality

### 1.1 Pattern Analysis: Umbrella Helm Chart

**Current pattern**: Single umbrella chart (`kube-llmops-stack`) containing 15 local subcharts via `file://` dependencies.

**Verdict**: The umbrella chart pattern is **correct for the use case** — it provides a one-command deployment experience and enables coordinated upgrades. This is the same pattern used by Rancher, GitLab, and Airflow Helm charts. No over-engineering here.

**However, the implementation has critical flaws:**

#### FATAL-01: Monolithic PostgreSQL — Single Point of Failure & Blast Radius

The entire platform shares **one PostgreSQL instance** (deployed by the `litellm` subchart) across 4 databases:

```
litellm-pg:5432
  ├── litellm      (API keys, spend tracking, rate limits)
  ├── langfuse     (trace metadata)
  ├── dify         (RAG workflows, knowledge bases)
  └── dify_plugin  (plugin daemon state)
```

**Why this is fatal:**

1. **Blast radius**: A single `ALTER TABLE` lock in the `dify` database can cascade latency into `litellm` API key validation, causing **all inference requests to timeout**. PostgreSQL connection pool exhaustion in one database starves all others.

2. **Upgrade coupling**: Dify 1.x → 2.x migration may require destructive schema changes. With a shared instance, you cannot upgrade Dify without risking LiteLLM and Langfuse downtime.

3. **Resource contention**: Langfuse's ClickHouse handles OLAP workloads, but its PostgreSQL metadata queries still compete with LiteLLM's high-frequency API key lookups on the same instance.

4. **Backup/restore granularity**: You cannot independently backup/restore Dify's data without affecting LiteLLM's operational state.

**Recommendation**:
```
# Option A (Recommended): Separate StatefulSets per logical group
litellm-pg:5432    → litellm DB only
langfuse-pg:5432   → langfuse DB only  
dify-pg:5432       → dify + dify_plugin DBs

# Option B (Minimal): At least separate operator from application DBs
operator-pg:5432   → litellm (hot path, low latency required)
app-pg:5432        → langfuse, dify, dify_plugin (tolerant of higher latency)
```

Each subchart should declare its own PostgreSQL dependency with independent lifecycle. Use Helm dependency conditions to allow users to point to an external database if they already have one.

#### FATAL-02: Hardcoded Service Names Break Multi-Instance Deployment

Prometheus scrape targets contain hardcoded service names:

```yaml
# charts/observability/templates/prometheus.yaml
scrape_configs:
  - job_name: vllm
    static_configs:
      - targets: ['vllm-qwen2-5-0-5b:8000']   # HARDCODED model name
```

This means:
- Adding a second model requires editing the Prometheus ConfigMap
- Multi-release deployment (e.g., staging + production in same cluster) will have name collisions
- The scrape config doesn't use `{{ .Release.Name }}` prefix

Similarly, the OTel Collector uses Kubernetes SD with label selectors like `kube_llmops_engine=vllm`, which is better but still doesn't namespace by release.

**Recommendation**: Replace all static targets with Kubernetes service discovery:
```yaml
scrape_configs:
  - job_name: vllm
    kubernetes_sd_configs:
      - role: pod
        namespaces:
          names: ["{{ .Release.Namespace }}"]
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_label_app_kubernetes_io_part_of]
        regex: "{{ include \"kube-llmops.fullname\" . }}"
        action: keep
      - source_labels: [__meta_kubernetes_pod_label_app_kubernetes_io_component]
        regex: vllm
        action: keep
```

#### HIGH-01: Architecture Doc vs Implementation Gap

ARCHITECTURE.md describes a **two-tier gateway** (LiteLLM → Envoy AI Gateway + IGW → vLLM) and components like SGLang, Harbor model registry, JupyterHub, MLflow, LLaMA-Factory, and Kustomize overlays. The actual implementation has:

- No SGLang subchart (only `llamacpp`)
- Envoy Gateway templates exist but are effectively dead code (requires external CRDs)
- No JupyterHub, MLflow, LLaMA-Factory, or Kustomize overlays
- Harbor subchart is empty/placeholder
- No ArgoCD ApplicationSet

The documentation creates expectations the codebase cannot deliver. Users trying to follow ARCHITECTURE.md will be confused when features don't exist.

**Recommendation**: Either (a) clearly mark unimplemented features as "Planned" in ARCHITECTURE.md with target versions, or (b) split into `ARCHITECTURE.md` (current state) and `ARCHITECTURE-VISION.md` (future state). The current doc mixes both without distinction.

### 1.2 Deployment Profile Strategy

The 4-profile system (`ci`, `minimal`, `standard`, `production`) is **well-designed**:

| Profile | GPU | Components | Use Case |
|---------|-----|------------|----------|
| ci | 0 | LiteLLM + Prometheus | CI testing |
| minimal | 1 | Full stack | Dev |
| standard | 4-8 | Multi-model | Team |
| production | 16+ | HA + security | Enterprise |

This progressive complexity approach is excellent and follows the "smart defaults, full override" principle correctly.

**Minor issue**: `values-single-node.yaml` exists but is not listed in Chart.yaml or documentation as an official profile. It appears to be a duplicate of `values-minimal.yaml` with slight differences. Consolidate or document.

### 1.3 Model Resolver — Good Design, Incomplete Integration

The Model Resolver (format auto-detection → engine selection) is architecturally sound:

```
User specifies model → init-container detects format → selects vLLM/llama.cpp/TEI
```

The resolver has good unit test coverage (28 tests). However, the resolver image is never actually used as an init-container in the vLLM deployment template. The `model-loader` init-container in `charts/vllm/templates/deployment.yaml` downloads models but doesn't invoke the resolver. The engine is determined at Helm template time, not at runtime.

**Impact**: The "auto-detection" is actually manual — users must set `engine: vllm` or `engine: tei` in values.yaml. The resolver code exists but is not wired in.

**Recommendation**: Complete the integration or remove the auto-detection claim from documentation.

---

## 2. Technology Stack & Ecosystem

### 2.1 Technology Choices Assessment

| Component | Choice | Assessment |
|-----------|--------|------------|
| Inference Engine | vLLM v0.9.2 | Excellent — industry standard, active development |
| Embeddings | TEI | Good — HuggingFace's dedicated embedding server |
| AI Gateway | LiteLLM v1.82.3 | Good — wide model support, but rapid version churn creates upgrade risk |
| Tracing | Langfuse v3.161.0 | Good — purpose-built for LLM tracing, correct choice over Jaeger |
| Metrics | Prometheus v3.9.1 | Solid — industry standard |
| Dashboards | Grafana 12.4.1 | Standard |
| Log Collection | Fluent Bit 4.2.3.1 | CNCF Graduated, correct choice |
| Log Storage | Loki 3.6.7 | Good — Grafana ecosystem integration |
| RAG Platform | Dify 1.13.2 | Risky — see below |
| SSO | Keycloak 26.5.6 | CNCF Incubating, solid choice |
| Object Storage | MinIO | Standard for self-hosted S3 |
| Orchestration | Kubernetes + Helm | Correct |

#### HIGH-02: Dify as RAG Platform — High Coupling, Breaking Change Risk

Dify is a rapidly evolving platform with frequent breaking changes. The kube-llmops Dify subchart has deep coupling:

1. **5 separate deployments** (API, Worker, Web, Plugin Daemon, Redis) — almost a sub-platform within the platform
2. **Plugin daemon** requires its own database, Redis, and PVC with offline `uv sync` workaround
3. **HttpOnly cookie** constraint forces path-based routing complexity in the ingress template (232-line ingress.yaml with Dify-specific special cases)
4. **Setup job** with hardcoded API calls for admin account creation, plugin installation, and model provider configuration

When Dify releases a breaking change (e.g., API path changes, plugin system rewrite, auth mechanism change), the setup job, ingress rules, and environment variables all need synchronized updates. This happened between Dify 0.x → 1.x and will likely happen again.

**Recommendation**:
- Abstract Dify behind an interface — define a "RAG Provider" contract with configurable backend (Dify, Langflow, custom)
- Move Dify-specific ingress rules into the Dify subchart instead of the parent ingress template
- Add version pinning with explicit Dify API compatibility matrix in documentation
- Consider making Dify an **optional addon** rather than a core component

#### HIGH-03: LiteLLM Version Pinning Risk

LiteLLM (`main-v1.82.3-stable`) releases multiple times per week with frequent breaking changes in config format, provider behavior, and database schema. The current chart pins to a specific version (good), but:

1. No database migration strategy documented for LiteLLM upgrades
2. LiteLLM's PostgreSQL schema may change between versions — shared database makes this dangerous
3. The `litellm_config` ConfigMap format is tightly coupled to LiteLLM's internal config parser

**Recommendation**: Pin LiteLLM version in Chart.yaml `appVersion`, document upgrade procedure, and test config compatibility in CI across at least 2 LiteLLM versions.

### 2.2 Missing Technology Debt

#### No External Secrets Operator Integration

All secrets (PostgreSQL passwords, API keys, OIDC client secrets) are stored in `values.yaml` in plaintext. The ARCHITECTURE.md mentions "External Secrets Operator" but there is zero implementation. For any production use, this is a blocker.

#### No Service Mesh

Inter-pod communication is unencrypted. The architecture mentions Cilium (CNCF Graduated) for network policy but doesn't leverage Cilium's mTLS capabilities or any service mesh. For environments handling sensitive prompts, this is a compliance concern.

---

## 3. Scalability & High Availability

### 3.1 Bottleneck Analysis

#### FATAL-03: Single-Replica Stateful Components Without HA Story

| Component | Production Replicas | HA Support | Impact of Failure |
|-----------|-------------------|------------|-------------------|
| PostgreSQL | **1** | None | **Total platform outage** — LiteLLM, Langfuse, Dify all down |
| Keycloak | **1** | None | No new logins, SSO token refresh fails |
| Prometheus | **1** | None | Alerting blind, dashboards empty |
| Grafana | **1** | None | Dashboard access lost |
| Loki | **1** | None | Log ingestion stops |
| ClickHouse | **1** | None | Langfuse trace ingestion stops |
| MinIO | **1** (StatefulSet) | None | Blob storage unavailable |

Even in `values-production.yaml`, **none of these components have HA**. LiteLLM and Langfuse are scaled to 2 replicas (good), but their shared PostgreSQL remains a single replica.

**This means a single PostgreSQL pod restart causes a complete platform outage.**

**Recommendation (phased)**:

**Phase 1** (Minimal HA):
- PostgreSQL: Use a Helm subchart that supports primary-replica (e.g., Bitnami PostgreSQL with `replication.enabled=true`, or CloudNativePG operator)
- Add PodDisruptionBudgets for all Deployments with ≥2 replicas

**Phase 2** (Full HA):
- Prometheus: Switch to Thanos or VictoriaMetrics for HA metrics
- Loki: Use Loki's `simple-scalable` deployment mode
- MinIO: Enable distributed mode (minimum 4 nodes)
- ClickHouse: Use ClickHouse Keeper for replication

**Phase 3** (External-first):
- Provide values overrides to point at external managed databases (RDS, Cloud SQL, etc.)
- Document and test external database configuration

#### FATAL-04: No PodDisruptionBudget Anywhere

Zero PDBs in the entire codebase. During a Kubernetes node drain (rolling update, spot instance reclamation), **all replicas of a component can be evicted simultaneously**.

For a production platform, this is unacceptable. A `kubectl drain` on a node running the PostgreSQL pod will cause a complete outage with no protection.

**Recommendation**: Add PDBs for every component:
```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: {{ include "litellm.fullname" . }}
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: litellm
```

### 3.2 GPU Scheduling Gaps

#### HIGH-04: No GPU Topology Awareness

The vLLM deployment uses a simple `nvidia.com/gpu: N` resource request. For multi-GPU models using tensor parallelism, this doesn't guarantee NVLink-connected GPUs. Two GPUs on different PCIe buses will have dramatically different inter-GPU bandwidth.

**Recommendation**:
- Use NVIDIA's GPU topology-aware scheduling (node feature discovery + TopologyManager)
- Document GPU topology requirements for tensor-parallel models
- Add nodeSelector/affinity rules for GPU topology

#### HIGH-05: Model Loading Cold Start Not Addressed

vLLM model loading for large models (70B+) takes 5-15 minutes. The current probes allow up to ~14 minutes (liveness: 240s initial + 20 failures × 30s). But:

1. KEDA autoscaling can trigger a new vLLM replica that won't serve traffic for 10+ minutes
2. No preemptive model loading strategy (models load from HuggingFace Hub or S3 on every pod start)
3. The Fluid/Alluxio cache is template-only — requires external Fluid operator installation with no documentation on setup

**Recommendation**:
- Implement model warm-up by pre-pulling model weights to node-local storage (DaemonSet or NodeLocal PV)
- Add readiness gate so KEDA doesn't count starting pods as ready
- Document Fluid operator installation and provide a self-contained caching alternative (e.g., hostPath with model pre-pull job)

### 3.3 Scaling Ceiling Analysis

| Component | Current Scaling | Ceiling | Bottleneck |
|-----------|----------------|---------|------------|
| vLLM | KEDA (replicas) | GPU count | Node provisioning (no Karpenter) |
| LiteLLM | Manual replicas | PostgreSQL connections | Shared PG instance |
| Prometheus | None (single) | ~2M active series | No remote storage, no sharding |
| Langfuse | Manual replicas | ClickHouse IOPS | Single ClickHouse |
| Loki | None (single) | ~10MB/s ingestion | Single instance, filesystem storage |

For a team of 50+ engineers sending continuous prompts, the single-instance bottlenecks will surface within weeks.

---

## 4. Module Decoupling & Maintainability

### 4.1 Dependency Graph Analysis

```
Parent Chart (ingress.yaml)
  ├── Knows about: LiteLLM, Grafana, Langfuse, Keycloak, Dify, Prometheus, MinIO
  │   (7 services hardcoded in parent ingress template)
  │
  ├── litellm/
  │   ├── Owns: PostgreSQL (shared by langfuse, dify, dify_plugin)
  │   ├── Knows about: vLLM service names, TEI service names, Langfuse URL
  │   └── ConfigMap: Hardcodes model names in litellm_config
  │
  ├── observability/
  │   ├── Prometheus: Hardcodes vLLM model service name
  │   ├── Grafana: Knows about Keycloak OIDC URLs, Prometheus URL, Loki URL
  │   └── OTel: Kubernetes SD with hardcoded label selectors
  │
  ├── langfuse/
  │   ├── Depends on: litellm-pg (PostgreSQL), MinIO (S3)
  │   └── Knows about: Keycloak OIDC URLs
  │
  ├── dify/
  │   ├── Depends on: litellm-pg (PostgreSQL), its own Redis, MinIO
  │   ├── Setup job: Knows LiteLLM API endpoint, model names
  │   └── Plugin daemon: Separate database, PVC, Redis dependency
  │
  ├── keycloak/
  │   ├── Realm ConfigMap: Hardcodes client names for grafana, langfuse, minio, litellm
  │   └── No dependency on other charts
  │
  └── security/
      ├── NetworkPolicy: Hardcodes service names (litellm, otel-collector, grafana)
      └── LLM-Guard: Standalone
```

#### HIGH-06: Cross-Chart Dependency Spaghetti

The dependency graph reveals tight coupling:

1. **Parent chart knows too much**: The 232-line `ingress.yaml` in the parent chart has service-specific routing rules for 7 subcharts. Each new subchart requires modifying the parent ingress.

2. **litellm owns shared infrastructure**: PostgreSQL is deployed by the `litellm` subchart but used by `langfuse` and `dify`. This creates an implicit ordering dependency — `litellm` must deploy before `langfuse` or `dify` can start.

3. **Circular knowledge**: `observability` knows about `vllm` service names; `vllm` doesn't know about `observability`. `keycloak` creates OIDC clients for `grafana` and `langfuse`; those charts reference `keycloak` URLs. This circular reference makes independent testing impossible.

**Recommendation**: Introduce a **shared infrastructure layer**:

```
Layer 0: Infrastructure (independent, deploy first)
  ├── postgresql/    (single chart, creates all DBs)
  ├── redis/         (single chart, shared Redis)
  ├── minio/         (object storage)
  └── keycloak/      (SSO provider)

Layer 1: Core Services (depend only on Layer 0)
  ├── vllm/          (model serving)
  ├── tei/           (embeddings)
  └── litellm/       (gateway, depends on postgresql)

Layer 2: Application Services (depend on Layers 0-1)
  ├── langfuse/      (depends on postgresql, minio)
  ├── dify/          (depends on postgresql, redis, litellm)
  └── observability/ (depends on nothing, scrapes via SD)

Layer 3: Platform Services (depend on Layers 0-2)
  ├── rag-eval/      (depends on litellm, langfuse)
  └── security/      (depends on nothing, overlays NetworkPolicy)
```

Each layer should be independently deployable and testable.

### 4.2 Template Quality & Helm Best Practices

#### HIGH-07: Inconsistent Helm Label Standards

Some subcharts use `app.kubernetes.io/name` labels consistently, others use custom labels like `app: vllm` or `app.kubernetes.io/part-of: kube-llmops`. This inconsistency breaks NetworkPolicy selectors and Prometheus service discovery.

Example inconsistencies found:
- vLLM: Uses `app.kubernetes.io/name: vllm`, `app.kubernetes.io/component: {{ $model.name }}`
- LiteLLM: Uses `app.kubernetes.io/name: litellm`
- Prometheus scrape: Matches on `kube_llmops_engine: vllm` (custom label)
- NetworkPolicy: Matches on `app.kubernetes.io/name: litellm` and `app.kubernetes.io/name: otel-collector`

**Recommendation**: Define a label standard in `_helpers.tpl` and enforce it across all subcharts:
```yaml
# Standard labels for ALL resources
app.kubernetes.io/name: {{ .Chart.Name }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion }}
app.kubernetes.io/component: <component-specific>
app.kubernetes.io/part-of: kube-llmops
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ include "kube-llmops.chart" . }}
```

### 4.3 Developer Onboarding Cost

**Positive aspects**:
- AGENTS.md provides an excellent AI-assistant knowledge base
- CONTRIBUTING.md covers development setup
- 4 deployment profiles reduce configuration burden
- Makefile provides standard targets

**Negative aspects**:
- `.tgz` cache gotcha (documented but still a trap — developers will forget to run `helm dependency update` after editing subcharts)
- No local development environment (e.g., Tilt, Skaffold, or DevSpace)
- E2E tests require a GPU node (no CPU-only E2E path for Dify/RAG tests)
- No architectural decision records (ADRs) — the "why" behind choices is scattered across ARCHITECTURE.md and commit messages

**Recommendation**:
- Add a `make dev` target that uses Tilt or Skaffold for hot-reload development
- Create `docs/adr/` directory with ADR templates for major decisions
- Add a pre-commit hook that runs `helm dependency update` when subchart templates change

---

## 5. Security & Deployment Operations

### 5.1 Security Assessment

#### FATAL-05: Hardcoded Credentials Throughout

The codebase contains **plaintext default credentials** in multiple value files:

| Credential | Location | Default Value |
|------------|----------|---------------|
| PostgreSQL password | `values-*.yaml` | `llmops-pg-dev-pw` |
| LiteLLM master key | `values-*.yaml` | `sk-kube-llmops-dev` / `sk-kube-llmops-default` |
| Keycloak admin password | `values-*.yaml` | `admin123!` |
| Grafana admin password | `values-*.yaml` | `admin123!` |
| Langfuse secrets | `values-*.yaml` | Various hardcoded values |
| MinIO credentials | `values-*.yaml` | `minioadmin/minioadmin` |
| LLM-Guard token | `values.yaml` | `llm-guard-kube-llmops` |
| Dify secret key | `values-*.yaml` | Hardcoded |
| Langfuse encryption key | `values-*.yaml` | Hardcoded |

**Why this is fatal**: Users will deploy with defaults. The `helm install` documentation doesn't prominently warn about changing credentials. Once deployed, these credentials are in ConfigMaps and Secrets that persist across upgrades. A credential rotation requires coordinated changes across 5+ components.

**Recommendation (immediate)**:
1. **Generate random defaults** at install time using Helm's `randAlphaNum` function:
   ```yaml
   password: {{ .Values.postgresql.password | default (randAlphaNum 24) }}
   ```
2. Add a `NOTES.txt` warning that prominently displays all default credentials and instructions to change them
3. Add a `--set` override example in the quickstart documentation
4. Implement a "security audit" Helm hook that checks for default credentials and warns/blocks

**Recommendation (medium-term)**:
5. Integrate External Secrets Operator (ESO) with a reference implementation for AWS Secrets Manager and HashiCorp Vault
6. Add `existingSecret` fields to all subcharts so users can pre-create Kubernetes Secrets

#### HIGH-08: No Egress NetworkPolicy

The `security` subchart implements ingress-only NetworkPolicies. Pods can freely connect to the internet. This means:

1. A compromised LLM-Guard pod can exfiltrate data
2. vLLM pods can be used for cryptomining if compromised
3. No protection against supply chain attacks in init containers that download from the internet

**Recommendation**: Add default-deny egress policies with explicit allow rules:
```yaml
# Allow vLLM to reach HuggingFace Hub (model download) and internal services only
egress:
  - to:
    - podSelector:
        matchLabels:
          app.kubernetes.io/part-of: kube-llmops
  - to:
    - ipBlock:
        cidr: 0.0.0.0/0
    ports:
      - port: 443  # HTTPS only for model downloads
```

### 5.2 CI/CD Assessment

**Strengths**:
- 7 GitHub Actions workflows covering lint, test, build, E2E, release
- Trivy vulnerability scanning on Docker images (CRITICAL severity)
- TruffleHog secret scanning
- Multi-profile template rendering tests (6 profiles)
- 28 unit tests for model resolver
- Comprehensive E2E test suite (30+ assertions)

**Critical Gaps**:

| Gap | Severity | Impact |
|-----|----------|--------|
| `chart-install-test` uses `continue-on-error: true` | HIGH | CI passes even if cluster install fails |
| No Helm schema validation (`values.schema.json`) | HIGH | Invalid values only caught at deploy time |
| No SBOM generation | MEDIUM | Supply chain visibility gap |
| No image signing (cosign/sigstore) | MEDIUM | Image provenance unverified |
| No database migration testing | HIGH | LiteLLM/Langfuse upgrades may break schema |
| License check is warning-only | MEDIUM | AGPL dependencies can slip through |
| No performance/load testing | MEDIUM | Scalability claims unverified |

#### CI Pipeline Recommendation

```
Current:
  PR → Lint + Build + Test (best-effort) → Merge

Recommended:
  PR → Lint → Template Render (all 6 profiles)
     → Schema Validation (values.schema.json)
     → Unit Tests (pytest)
     → kind Cluster Install (REQUIRED, not best-effort)
     → Smoke Test (health checks)
     → E2E (Playwright, weekly or on-demand)

  Main → Build + Trivy + SBOM → Push (GHCR)
       → Sign (cosign)

  Tag → Release (chart + images + SBOM + signature)
```

### 5.3 Deployment Operations Gaps

#### No Backup/Restore Procedure

Despite ARCHITECTURE.md mentioning backup/restore scripts, no `scripts/backup.sh` or `scripts/restore.sh` exists. For a platform with 4+ databases, this is critical.

**Recommendation**: Implement automated backup:
```bash
# Minimum viable backup
kubectl exec litellm-pg-0 -- pg_dumpall > backup-$(date +%Y%m%d).sql

# Better: Use a Kubernetes-native backup solution
# - Velero (CNCF project) for full cluster backup
# - pgBackRest for PostgreSQL-specific backup
```

#### No Upgrade Runbook

The project lacks a documented upgrade procedure. Questions like:
- What happens when upgrading from v0.1.0 to v0.2.0?
- Are there database migrations?
- What's the rollback procedure?
- Are there breaking changes in values.yaml between versions?

These are unanswered. The Quality Gate (pre-upgrade hook) is a good start but only checks RAG quality metrics, not infrastructure health.

**Recommendation**: Create `docs/upgrade-guide.md` with version-specific migration notes and add a pre-upgrade Job that validates database schema compatibility.

---

## 6. Fatal Issues Summary

| ID | Issue | Impact | Effort to Fix |
|----|-------|--------|---------------|
| **FATAL-01** | Monolithic shared PostgreSQL | Single point of failure, upgrade coupling, total outage risk | High (refactor DB layer) |
| **FATAL-02** | Hardcoded service names in Prometheus/OTel | Breaks multi-model, multi-release deployment | Medium (template refactor) |
| **FATAL-03** | No HA for any stateful component | Any pod restart = platform outage | High (add replication) |
| **FATAL-04** | No PodDisruptionBudget | Node drain = uncontrolled outage | Low (add PDB templates) |
| **FATAL-05** | Plaintext hardcoded credentials | Security breach risk, credential rotation impossible | Medium (generate random defaults + ESO) |

**High-severity issues**: 8 total (detailed above)

---

## 7. Refactoring Roadmap

### Phase 1: Critical Fixes (1-2 weeks)

**Goal**: Make the platform survivable in production.

| Task | Files Affected | Priority |
|------|---------------|----------|
| Add PodDisruptionBudgets for all Deployments | All subchart templates | P0 |
| Replace hardcoded Prometheus targets with Kubernetes SD | `observability/templates/prometheus.yaml` | P0 |
| Generate random default credentials with `randAlphaNum` | All `values.yaml` files | P0 |
| Add `existingSecret` support to all subcharts | All subchart `values.yaml` and templates | P0 |
| Standardize Helm labels across all subcharts | All `_helpers.tpl` and templates | P1 |
| Make `chart-install-test` required (remove `continue-on-error`) | `.github/workflows/test.yaml` | P1 |

### Phase 2: Architecture Improvements (2-4 weeks)

**Goal**: Decouple components and enable independent lifecycle.

| Task | Files Affected | Priority |
|------|---------------|----------|
| Split PostgreSQL into separate instances per logical group | `litellm/`, `langfuse/`, `dify/` subcharts | P0 |
| Move Dify-specific ingress rules into Dify subchart | `templates/ingress.yaml`, `dify/templates/` | P1 |
| Add `values.schema.json` for validation | `charts/kube-llmops-stack/` | P1 |
| Implement layered dependency ordering (see Section 4.1) | `Chart.yaml`, all subcharts | P1 |
| Add egress NetworkPolicies | `security/templates/` | P2 |
| Create upgrade runbook and migration framework | `docs/upgrade-guide.md` | P2 |

### Phase 3: Production Hardening (1-2 months)

**Goal**: Enterprise-grade HA and operations.

| Task | Files Affected | Priority |
|------|---------------|----------|
| PostgreSQL HA (CloudNativePG or Bitnami replication) | New subchart or external dependency | P0 |
| External Secrets Operator integration | New templates, documentation | P1 |
| Backup/restore automation (Velero or pg_dump CronJob) | New templates, scripts | P1 |
| Prometheus HA (VictoriaMetrics or Thanos sidecar) | `observability/` subchart | P2 |
| Add inter-pod mTLS (Cilium or service mesh) | Infrastructure layer | P2 |
| Performance/load testing suite | `tests/load/` | P2 |
| SBOM generation + image signing in CI | `.github/workflows/` | P2 |

### Phase 4: Ecosystem Maturity (2-3 months)

**Goal**: Developer experience and ecosystem integration.

| Task | Files Affected | Priority |
|------|---------------|----------|
| Tilt/Skaffold dev environment | `Tiltfile` or `skaffold.yaml` | P1 |
| Complete Model Resolver integration | `vllm/templates/deployment.yaml` | P1 |
| ArgoCD ApplicationSet for GitOps | `manifests/argocd/` | P2 |
| Multi-cluster support documentation | `docs/` | P2 |
| Terraform modules for cloud providers | `terraform/` | P3 |
| Kubernetes Operator (long-term) | New Go project | P3 |

---

## Appendix: File-Level Findings

### Critical Path Files (Review Priority)

| File | Lines | Risk | Finding |
|------|-------|------|---------|
| `charts/kube-llmops-stack/templates/ingress.yaml` | 232 | HIGH | Monolithic, knows about 7 subcharts, Dify special cases |
| `charts/litellm/templates/postgresql.yaml` | ~150 | FATAL | Shared PG with init script creating 4 databases |
| `charts/observability/templates/prometheus.yaml` | 299 | FATAL | Hardcoded vLLM target, no service discovery |
| `charts/dify/templates/` | ~500+ | HIGH | 5 deployments, complex setup job, offline PVC workaround |
| `charts/keycloak/templates/realm-configmap.yaml` | 63 | MEDIUM | Hardcoded client names, default passwords |
| `charts/security/templates/network-policies.yaml` | 103 | HIGH | Ingress-only, no egress, hardcoded service names |

### Positive Highlights

| File | Finding |
|------|---------|
| `images/model-resolver/` | Well-structured Python code with 28 tests, good separation of concerns |
| `charts/rag-eval/` | Quality gate concept is excellent — pre-upgrade RAG quality validation |
| `charts/observability/dashboards/` | 4 purpose-built dashboards with LLM-specific metrics |
| `AGENTS.md` | Outstanding AI-assistant knowledge base, captures critical gotchas |
| `Makefile` | Clean, comprehensive build targets |
| `tests/e2e/test_full_e2e.py` | 30+ assertions covering 9 test categories — impressive breadth |

---

## Final Assessment

kube-llmops is a **visionary project** that correctly identifies the need for a batteries-included Kubernetes LLMOps platform. The architecture document, CNCF alignment, and technology choices demonstrate deep understanding of the AI infrastructure landscape.

However, the implementation has **not yet caught up with the vision**. The five fatal issues identified — shared PostgreSQL, hardcoded service names, no HA, no PDBs, and plaintext credentials — would each independently cause production incidents. Together, they make the platform unsuitable for production workloads without significant remediation.

The good news: most fixes are well-scoped and can be implemented incrementally. The project's modular subchart structure makes it possible to refactor one component at a time without a rewrite. **The architecture is fundamentally sound — it's the implementation details that need hardening.**

**Priority action**: Start with Phase 1 (PDBs, service discovery, credential generation) — these are high-impact, low-effort fixes that immediately improve production readiness.

---

*This review is based on static analysis of the codebase and documentation. A live deployment review with load testing would likely surface additional operational concerns.*
