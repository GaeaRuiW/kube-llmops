# kube-llmops Operator Architecture

> **API Group:** `llmops.kubellmops.io/v1alpha1`
> **Source:** `operator/` directory
> **Built with:** controller-runtime v0.20, Helm SDK v3, Argo Workflows (unstructured)

---

## Table of Contents

1. [Design Philosophy](#1-design-philosophy)
2. [CRD API Reference](#2-crd-api-reference)
3. [Controller Internals](#3-controller-internals)
4. [Engine Auto-Detection](#4-engine-auto-detection)
5. [Model Lifecycle](#5-model-lifecycle)
6. [LiteLLM Integration](#6-litellm-integration)
7. [Helm SDK Bridge](#7-helm-sdk-bridge)
8. [FineTuneRun Pipeline](#8-finetunerun-pipeline)
9. [RBAC Model](#9-rbac-model)
10. [Failure Modes](#10-failure-modes)
11. [Operator Observability](#11-operator-observability)
12. [Security](#12-security)

---

## 1. Design Philosophy

The kube-llmops operator is built on the principle that LLM infrastructure management
should be **Kubernetes-native and declarative**. Platform engineers express their desired
state through Custom Resources, and the operator continuously reconciles the cluster to
match. This eliminates imperative scripts, manual `helm install` commands, and ad-hoc
`kubectl apply` workflows that typically plague ML platform operations.

### Hybrid Reconciliation Strategy

The operator employs three distinct reconciliation strategies, each optimized for
the resource type it manages:

```
+-------------------+-----------------------------+----------------------------+
|       CRD         |   Reconciliation Strategy   |        Rationale           |
+-------------------+-----------------------------+----------------------------+
| ModelDeployment   | Direct K8s API (client-go)  | Speed, fine-grained        |
|                   |                             | control over Deployments,  |
|                   |                             | Services, and PVCs         |
+-------------------+-----------------------------+----------------------------+
| LLMPlatform       | Helm SDK bridge             | Feature parity with the    |
|                   | (helm.sh/helm/v3)           | umbrella chart; reuse of   |
|                   |                             | 20+ subchart templates     |
+-------------------+-----------------------------+----------------------------+
| FineTuneRun       | Argo Workflow CRs           | DAG orchestration for      |
|                   | (unstructured)              | multi-step pipelines;      |
|                   |                             | retry/timeout built-in     |
+-------------------+-----------------------------+----------------------------+
```

**ModelDeployment** creates Kubernetes resources directly through the API because model
serving requires sub-second reconciliation response times, deterministic resource
construction (Deployment, Service, PVC), and owner-reference-based garbage collection.
Direct API manipulation gives the controller full visibility into each child resource's
lifecycle without the overhead of rendering Helm templates.

**LLMPlatform** delegates to the Helm SDK because the platform stack spans 20+ components
(LiteLLM, Grafana, Prometheus, Loki, Langfuse, MinIO, Keycloak, PostgreSQL, Argo,
MLflow, Dify, Milvus, KEDA, Fluent Bit) with complex inter-dependency wiring. Rewriting
all those Helm templates as direct API calls would be prohibitively expensive and
fragile. The Helm SDK bridge translates the CR spec into a values map and executes
`install` or `upgrade` through the same codepath as `helm install`.

**FineTuneRun** creates Argo Workflow custom resources because fine-tuning is an inherently
multi-step, long-running process (data preparation, training, merge, evaluation, quality
gating, deployment) that benefits from Argo's DAG execution engine, retry policies, and
artifact passing. The operator uses `unstructured.Unstructured` to avoid importing Argo's
Go types as a compile-time dependency.

### Why Three CRDs Instead of One

A single monolithic CRD would conflate three fundamentally different concerns: real-time
model serving (scale-to-zero latency matters), platform infrastructure lifecycle (Helm
release versioning), and batch training pipelines (DAG state machines). Separating them
yields independent status tracking, distinct RBAC scoping, and the ability to evolve each
API surface without breaking the others. A `FineTuneRun` can reference a `ModelDeployment`
for auto-deploy without coupling their reconciliation loops.

---

## 2. CRD API Reference

All CRDs live under `api/v1alpha1/` and belong to the `llmops.kubellmops.io` API group.

### ModelDeployment

**Short name:** `md`
**Scope:** Namespaced
**Source:** `api/v1alpha1/modeldeployment_types.go`

```yaml
apiVersion: llmops.kubellmops.io/v1alpha1
kind: ModelDeployment
metadata:
  name: gemma-4-26b
spec:
  source: nohurry/gemma-4-26B-A4B-it-heretic-GUFF   # Required. HuggingFace model ID
  engine: llamacpp        # auto | vllm | tei | llamacpp (default: auto)
                          # Optional here: auto-detect is typo-tolerant and matches "GUFF" as llamacpp
  replicas: 1             # Desired pod count (default: 1, min: 0)
  resources:
    gpu: 1                # GPU count (default: 1, min: 0)
    memory: 20Gi          # Memory limit (default: 16Gi; q4_k_m ~16.87GB)
    cpu: "4"              # CPU limit (default: 4)
  accelerator: nvidia     # nvidia | amd | gaudi (default: nvidia)
  migDevice: ""           # NVIDIA MIG resource name override
  engineArgs:             # Extra CLI flags passed to inference engine
    --jinja: ""           # llama.cpp: enable Jinja chat templates
    --ctx-size: "163840"  # llama.cpp: 160K context (model supports 256K; limited by 24GB VRAM)
  allowPatterns: "*q4_k_m*"   # Selective shard download (split GGUF)
  prefixCaching: false    # Enable vLLM prefix caching (no-op for llamacpp)
  spot:                   # Spot/preemptible scheduling
    enabled: false
    provider: aws         # aws | gcp | azure | karpenter
  canary:                 # A/B traffic splitting
    source: org/model-v2
    weight: 20            # 0-100 percent to canary
    resources:
      gpu: 1
  modelStore:             # Per-model MinIO override
    endpoint: ""
    bucket: ""
```

**Status fields:**

| Field            | Type             | Description                                      |
|------------------|------------------|--------------------------------------------------|
| `phase`          | `string`         | `Pending`, `Downloading`, `Deploying`, `Ready`, `Degraded`, `Failed` |
| `engine`         | `string`         | Resolved engine after auto-detection              |
| `endpoint`       | `string`         | In-cluster URL (e.g., `http://md.ns.svc:8000`)   |
| `readyReplicas`  | `int32`          | Current ready pod count                           |
| `totalReplicas`  | `int32`          | Desired pod count from Deployment                 |
| `modelSize`      | `string`         | Discovered model size after download              |
| `conditions`     | `[]Condition`    | Standard Kubernetes conditions (`Ready`)          |
| `canary`         | `CanaryStatus`   | Canary phase, endpoint, readyReplicas             |

### LLMPlatform

**Short name:** `lp`, `llmplatforms`
**Scope:** Namespaced
**Source:** `api/v1alpha1/llmplatform_types.go`

```yaml
apiVersion: llmops.kubellmops.io/v1alpha1
kind: LLMPlatform
metadata:
  name: kube-llmops
spec:
  gateway:
    enabled: true
    routing: latency-based-routing
    image: { repository: "", tag: "" }
    rateLimiting: { enabled: false }
    budgetControl: { enabled: false }
  observability:
    enabled: true
    grafana: { adminPassword: admin, oidc: { enabled: false } }
    langfuse: { enabled: true }
  logging:
    enabled: true
  modules:
    rag: { enabled: true }
    finetune: { enabled: true }
    security: { enabled: false }
  modelStore:
    enabled: true
    endpoint: kube-llmops-minio:9000
    bucket: models
    accessKey: minioadmin
    secretKey: minioadmin
    hfTransferConcurrency: 32
    image: kube-llmops/model-loader:latest
  hfToken: ""
  keycloak: { enabled: false }
  postgresql: { enabled: true }
  keda: { enabled: false }
  nodePort: { enabled: true, host: "172.29.193.187" }
  ingress: { enabled: false, className: nginx, host: llm.example.com }
```

**Status fields:**

| Field           | Type                | Description                                 |
|-----------------|---------------------|---------------------------------------------|
| `phase`         | `string`            | `Pending`, `Installing`, `Ready`, `Upgrading`, `Degraded`, `Failed` |
| `helmRelease`   | `string`            | Managed Helm release name                   |
| `helmRevision`  | `int`               | Helm release revision number                |
| `components`    | `ComponentStatuses` | Per-component health (gateway, grafana, etc.) |
| `conditions`    | `[]Condition`       | `HelmRelease` condition                     |

### FineTuneRun

**Short name:** `ftr`
**Scope:** Namespaced
**Source:** `api/v1alpha1/finetunerun_types.go`

```yaml
apiVersion: llmops.kubellmops.io/v1alpha1
kind: FineTuneRun
metadata:
  name: gemma-lora-v1
spec:
  baseModel: google/gemma-2-9b     # Fine-tuning needs non-quantized weights
  outputName: gemma-4-lora-v1
  method: lora                  # lora | qlora | full (default: lora)
  dataSource:
    type: minio                 # minio | huggingface | pvc
    path: "s3://datasets/my-data/"
    format: alpaca              # alpaca | sharegpt | custom
  training:
    epochs: 3
    batchSize: 4
    learningRate: "2e-4"
    gradientAccumulationSteps: 1
    warmupRatio: "0.1"
    loraRank: 16
    loraAlpha: 32
    loraTarget: "all"
  resources:
    gpu: 1
    memory: 24Gi
    cpu: "4"
  evaluation:
    enabled: true
    dataset: ""
  qualityGate:
    enabled: true
    thresholds:
      minEvalLoss: "0.8"
      maxTrainLoss: "0.5"
  deploy:
    enabled: false
    canaryWeight: 20
```

**Status fields:**

| Field             | Type               | Description                                     |
|-------------------|--------------------|-------------------------------------------------|
| `phase`           | `string`           | `Pending`, `DataPreparing`, `Training`, `Evaluating`, `QualityGate`, `Deploying`, `Succeeded`, `Failed` |
| `argoWorkflow`    | `string`           | Name of the created Argo Workflow                |
| `startTime`       | `*Time`            | When the pipeline started                        |
| `completionTime`  | `*Time`            | When the pipeline completed                      |
| `metrics`         | `TrainingMetrics`  | `trainLoss`, `evalLoss`, `trainingDuration`      |
| `mlflow`          | `MLflowStatus`     | `runId`, `experimentName`, `artifactUri`         |
| `qualityGate`     | `QualityGateStatus`| `passed` (bool), `message`                      |
| `outputModel`     | `OutputModelStatus`| `source`, `modelDeployment` reference            |
| `conditions`      | `[]Condition`      | `WorkflowReady` condition                        |

---

## 3. Controller Internals

The operator runs three independent controllers within a single manager process,
registered in `cmd/main.go`. Each controller has its own work queue and reconciliation
loop, sharing the same informer cache.

```
┌──────────────────────────────────────────────────────────────────────┐
│                        ctrl.Manager                                  │
│  ┌─────────────────┐  ┌──────────────────┐  ┌────────────────────┐  │
│  │ ModelDeployment  │  │  LLMPlatform     │  │  FineTuneRun       │  │
│  │  Reconciler      │  │   Reconciler     │  │   Reconciler       │  │
│  │                  │  │                  │  │                    │  │
│  │ Owns:            │  │ Uses:            │  │ Uses:              │  │
│  │  - Deployment    │  │  - HelmClient    │  │  - unstructured    │  │
│  │  - Service       │  │    (SDKClient)   │  │    Argo Workflows  │  │
│  │  - PVC           │  │                  │  │                    │  │
│  │                  │  │                  │  │                    │  │
│  │ Injects:         │  │ Injects:         │  │ Injects:           │  │
│  │  - GatewayClient │  │  - ChartPath     │  │  - ReleaseName     │  │
│  └─────────────────┘  └──────────────────┘  └────────────────────┘  │
│                                                                      │
│  Health: :8081/healthz, :8081/readyz                                 │
│  Metrics: :8080 (controller-runtime)                                 │
│  Leader election: 61f31610.kubellmops.io                             │
└──────────────────────────────────────────────────────────────────────┘
```

### ModelDeploymentReconciler

**Source:** `internal/controller/modeldeployment_controller.go`

The reconciliation loop follows a strict 10-step sequence:

1. **Fetch** the `ModelDeployment` CR; return if not found (deleted externally).
2. **Handle deletion** — if `DeletionTimestamp` is set, deregister the model from the
   gateway via `GatewayClient.DeregisterModel()`, remove the finalizer
   `llmops.kubellmops.io/finalizer`, and update the resource.
3. **Add finalizer** if missing — ensures cleanup runs before the CR is garbage-collected.
4. **Resolve engine** — call `engine.ResolveEngine(source, engine)` to determine the
   actual inference engine (see [Section 4](#4-engine-auto-detection)).
5. **Find LLMPlatform** — list `LLMPlatform` resources in the same namespace to obtain
   model store configuration for the init container.
6. **Ensure PVC** — call `builder.BuildPVC()`, GET the existing PVC or CREATE it. The
   PVC is named `{md.Name}-cache` and sized per engine (50Gi vllm, 30Gi llamacpp, 10Gi tei).
7. **Ensure Deployment** — call `builder.BuildDeployment()`, GET or CREATE. On update,
   the existing Deployment's `.spec` and `.labels` are overwritten in-place.
8. **Ensure Service** — call `builder.BuildService()`, GET or CREATE a `ClusterIP` service.
9. **Update status** — read the Deployment's `ReadyReplicas`, compute the phase
   (`Ready`/`Degraded`/`Deploying`), set the `Ready` condition, and write the endpoint URL.
10. **Register with gateway** — when phase is `Ready`, call `GatewayClient.RegisterModel()`
    with the `openai/{name}` model prefix.

Owner references are set on all child resources (PVC, Deployment, Service) via
`controllerutil.SetControllerReference()`, enabling automatic garbage collection when the
`ModelDeployment` is deleted. The controller watches owned `Deployment`, `Service`, and
`PVC` resources so that external changes (e.g., pod eviction) trigger re-reconciliation.

```go
func (r *ModelDeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&v1alpha1.ModelDeployment{}).
        Owns(&appsv1.Deployment{}).
        Owns(&corev1.Service{}).
        Owns(&corev1.PersistentVolumeClaim{}).
        Named("modeldeployment").
        Complete(r)
}
```

### LLMPlatformReconciler

**Source:** `internal/controller/llmplatform_controller.go`

This reconciler is simpler because it delegates all resource creation to Helm:

1. **Fetch** the `LLMPlatform` CR.
2. **Translate** spec to Helm values via `helmbridge.TranslateValues()`.
3. **Check** if a Helm release already exists via `HelmClient.GetRelease()`.
4. **Install or Upgrade** — if no release exists, call `Install()`; otherwise call
   `Upgrade()`. Phase transitions: `Pending` → `Installing` → `Ready` (or `Failed`).
5. **Update status** with the release name, revision number, and `HelmRelease` condition.

The controller does not set owner references on Helm-managed resources (Helm handles its
own lifecycle). It watches only the `LLMPlatform` CR itself.

### FineTuneRunReconciler

**Source:** `internal/controller/finetunerun_controller.go`

The reconciler has two operational modes depending on whether an Argo Workflow already
exists:

**Mode 1 — Create Workflow:** Build an Argo Workflow via `builder.BuildArgoWorkflow()`,
set the owner reference, create it. Record the workflow name in `status.argoWorkflow`.

**Mode 2 — Sync Status:** If `status.argoWorkflow` is set, GET the workflow via
unstructured client, read `status.phase`, and map it to the FineTuneRun phase. Requeue
every 30 seconds until the workflow reaches a terminal state (`Succeeded` or `Failed`).

---

## 4. Engine Auto-Detection

The engine resolver in `internal/engine/resolver.go` determines which inference engine
to use for a `ModelDeployment` based on the model source name. The algorithm follows a
strict priority chain with no ambiguity.

### Resolution Algorithm

```
                    ┌─────────────────────┐
                    │ spec.engine set and  │
                    │ != "auto" and != ""? │
                    └──────┬──────────────┘
                           │
                    Yes    │    No
                    ▼      │    ▼
              ┌──────────┐ │ ┌───────────────────┐
              │ Return    │ │ │ Lowercase source   │
              │ explicit  │ │ │ Contains "gguf"?   │
              └──────────┘ │ └──────┬────────────┘
                           │        │
                           │ Yes    │    No
                           │ ▼      │    ▼
                           │ Return │ ┌─────────────────────┐
                           │llamacpp│ │ isEmbeddingOrRerank? │
                           │        │ └──────┬──────────────┘
                           │        │        │
                           │        │ Yes    │    No
                           │        │ ▼      │    ▼
                           │        │ Return │  Return
                           │        │  tei   │  vllm
                           │        │        │ (default)
                           └────────┴────────┘
```

**Priority:** `explicit engine` > `GGUF pattern → llamacpp` > `embedding/reranker → tei` > `vllm (default)`

The `GGUF` check uses `strings.ToLower` and matches two substrings: `"gguf"` (the
standard spelling) and `"guff"` (a common typo found in community HF repos, e.g.
`nohurry/gemma-4-26B-A4B-it-heretic-GUFF`). Both resolve to `llamacpp`.

### Pattern Matching for Embedding and Reranker Models

The `isEmbeddingOrReranker()` function first checks for `"rerank"` in the lowercased
source string. If not matched, it delegates to `isEmbedding()` which checks against a
curated pattern list covering the most widely-deployed embedding model families:

```go
// internal/engine/resolver.go
func isEmbedding(s string) bool {
    patterns := []string{
        "/bge-", "/e5-", "/gte-",          // BAAI BGE, IntFloat E5, GTE
        "minilm",                           // sentence-transformers MiniLM
        "/jina-embed", "jina-embeddings",   // Jina AI
        "/nomic-embed", "nomic-embed",      // Nomic
        "/all-mpnet",                       // sentence-transformers MPNet
        "embedding",                        // Catch-all keyword
    }
    for _, p := range patterns {
        if strings.Contains(s, p) {
            return true
        }
    }
    return false
}
```

### Resolution Examples

| Source                              | Explicit Engine | Resolved | Reason                     |
|-------------------------------------|-----------------|----------|----------------------------|
| `Qwen/Qwen2.5-7B-Instruct`        | `auto`          | `vllm`   | No pattern match, default  |
| `TheBloke/Llama-2-7B-GGUF`         | (empty)         | `llamacpp` | Contains "gguf"          |
| `BAAI/bge-small-en-v1.5`           | (empty)         | `tei`    | Matches `/bge-`            |
| `BAAI/bge-reranker-base`           | (empty)         | `tei`    | Contains "rerank"          |
| `intfloat/e5-large-v2`             | (empty)         | `tei`    | Matches `/e5-`             |
| `nomic-ai/nomic-embed-text`        | (empty)         | `tei`    | Matches "nomic-embed"      |
| `anything`                          | `llamacpp`      | `llamacpp` | Explicit always wins     |
| `nohurry/gemma-4-...-heretic-GUFF` | (empty)         | `llamacpp` | Matches `guff` typo pattern (v0.5.0+) |

A companion function `ResolveModelType()` classifies models as `"llm"`, `"embedding"`,
or `"reranker"` using the same pattern set, used by the gateway integration to select
the correct LiteLLM model prefix.

---

## 5. Model Lifecycle

A `ModelDeployment` progresses through a deterministic lifecycle from creation to ready
state. Each phase corresponds to observable Kubernetes resource states.

### Lifecycle State Machine

```
  ┌──────────┐     ┌───────────────┐     ┌───────────────┐
  │  Create   │────▶│ Resolve Engine│────▶│  Create PVC   │
  │    MD     │     │  (auto-detect)│     │ {name}-cache  │
  └──────────┘     └───────────────┘     └──────┬────────┘
                                                 │
                                                 ▼
  ┌──────────────────────────────────────────────────────────┐
  │                    Create Deployment                      │
  │  ┌──────────────────┐    ┌───────────────────────────┐   │
  │  │  Init Container   │───▶│    Main Container          │   │
  │  │  (model-loader)   │    │    (vllm/tei/llamacpp)     │   │
  │  │                    │    │                             │   │
  │  │  Downloads model   │    │  Serves inference on       │   │
  │  │  from HF → MinIO   │    │  /models/{slug}            │   │
  │  │  → PVC /models/    │    │                             │   │
  │  └──────────────────┘    └───────────────────────────┘   │
  └──────────────────────────────────────────────────────────┘
                                                 │
                                                 ▼
                                     ┌───────────────────┐
                                     │  Create Service    │
                                     │  (ClusterIP)       │
                                     └───────┬───────────┘
                                              │
                                              ▼
                                     ┌───────────────────┐
                                     │ Register Gateway   │
                                     │ (LiteLLM /model/   │
                                     │  new)              │
                                     └───────┬───────────┘
                                              │
                                              ▼
                                     ┌───────────────────┐
                                     │     Ready          │
                                     │ (all replicas up)  │
                                     └───────────────────┘
```

### Phase Transitions

| From         | To           | Trigger                                              |
|--------------|--------------|------------------------------------------------------|
| (none)       | `Deploying`  | CR created, Deployment created but no ready pods     |
| `Deploying`  | `Ready`      | `readyReplicas >= desired` and `desired > 0`         |
| `Deploying`  | `Degraded`   | `readyReplicas > 0` but `< desired`                 |
| `Ready`      | `Degraded`   | Pod eviction or node failure reduces ready count     |
| `Degraded`   | `Ready`      | Replacement pods reach ready state                   |
| Any          | (deleted)    | CR deleted, finalizer deregisters from gateway, owner references trigger GC |

### Resource Construction Details

**PVC:** Named `{md.Name}-cache`, sized by engine (vllm: 50Gi, llamacpp: 30Gi, tei: 10Gi),
`ReadWriteOnce` access mode. Mounted at `/models` in both init and main containers.

**Deployment:** Uses `Recreate` strategy (GPU workloads cannot share devices during
rolling update — a new pod would deadlock trying to allocate a GPU already held by
the old pod). Termination grace period is 90 seconds. Service links are disabled for
DNS performance. For vLLM, an additional `dshm` volume (8Gi `emptyDir` on `Memory`
medium) is mounted at `/dev/shm` for shared memory IPC. For llama.cpp with multi-shard
("split") GGUF models (files named `{prefix}-NNNNN-of-NNNNN.gguf`), a pod startup hook
creates symlinks so llama.cpp can load the full model by pointing `--model` at the first
shard; the model-loader init container downloads all matching shards from HuggingFace or
MinIO.

**Service:** `ClusterIP` type. Port is engine-dependent: `8000` for vLLM, `8080` for TEI
and llama.cpp. The endpoint URL follows the pattern:
`http://{name}.{namespace}.svc.cluster.local:{port}`.

### Init Container (model-loader)

When an `LLMPlatform` with `modelStore.enabled: true` exists in the same namespace, the
Deployment includes a `model-loader` init container that downloads model weights before
the inference engine starts. The init container receives environment variables for MinIO
connectivity and HuggingFace transfer concurrency:

```yaml
env:
  - name: MODEL_SOURCE
    value: "nohurry/gemma-4-26B-A4B-it-heretic-GUFF"
  - name: MODEL_SLUG
    value: "nohurry--gemma-4-26B-A4B-it-heretic-GUFF"
  - name: MINIO_ENDPOINT
    value: "kube-llmops-minio:9000"
  - name: MINIO_BUCKET
    value: "models"
  - name: HF_TRANSFER_CONCURRENCY
    value: "32"
```

Per-model overrides via `spec.modelStore.endpoint` and `spec.modelStore.bucket` take
precedence over the platform-level settings.

---

## 6. LiteLLM Integration

The operator integrates with the LiteLLM AI Gateway to provide a unified OpenAI-compatible
API endpoint for all deployed models. The gateway client handles model registration,
deregistration, and health checking.

### Gateway Client Interface

**Source:** `internal/gateway/client.go`

```go
type Client interface {
    RegisterModel(ctx context.Context, model GatewayModel) error
    DeregisterModel(ctx context.Context, modelID string) error
    HealthCheck(ctx context.Context) error
}
```

The interface has two implementations:

| Implementation | Purpose                                  | Timeout |
|----------------|------------------------------------------|---------|
| `HTTPClient`   | Production HTTP client for LiteLLM API   | 10s     |
| `NoopClient`   | Test/development no-op (default in main) | N/A     |

The `HTTPClient` communicates with LiteLLM's admin API over HTTP, authenticating via a
`masterKey` bearer token when configured.

### Model Registration Flow

```
ModelDeployment Ready
        │
        ▼
┌───────────────────────┐      POST /model/new
│  GatewayClient.       │─────────────────────────▶  LiteLLM
│  RegisterModel()      │                             Gateway
│                       │◀─────────────────────────
│  model_name: md.Name  │      200 OK / 4xx error
│  model: openai/{name} │      (non-fatal, logged)
│  api_base: endpoint   │
└───────────────────────┘
```

When a `ModelDeployment` reaches the `Ready` phase, the reconciler constructs a
`GatewayModel` and registers it:

```go
model := gateway.GatewayModel{
    ModelName: md.Name,
    LiteLLMParams: gateway.LiteLLMParams{
        Model:   "openai/" + md.Name,
        APIBase: md.Status.Endpoint,
    },
}
```

The `openai/` prefix tells LiteLLM to use its OpenAI-compatible provider, which is
appropriate for vLLM and llama.cpp models that expose an OpenAI-compatible API. For TEI
embedding/reranker models, the same prefix works because LiteLLM routes based on the
model type from its internal configuration.

### Model Deregistration

Deregistration occurs during the finalizer cleanup when a `ModelDeployment` is deleted:

```go
// During deletion handling
if r.GatewayClient != nil {
    if err := r.GatewayClient.DeregisterModel(ctx, md.Name); err != nil {
        log.Error(err, "failed to deregister model from gateway")
        // Best-effort: finalizer is still removed
    }
}
```

Deregistration calls `POST /model/delete` with `{"id": modelID}`. Failures are logged
but do not block deletion — the finalizer is removed regardless. This prevents a downed
gateway from blocking resource cleanup.

### Health Checking

The `HealthCheck()` method calls `GET /health` on the gateway. This is used for
proactive health monitoring and can be integrated into readiness gates for dependent
services.

```go
func (c *HTTPClient) HealthCheck(ctx context.Context) error {
    req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
    resp, err := c.httpClient.Do(req)
    if resp.StatusCode >= 400 {
        return fmt.Errorf("gateway unhealthy: HTTP %d", resp.StatusCode)
    }
    return nil
}
```

---

## 7. Helm SDK Bridge

The Helm SDK bridge translates `LLMPlatform` CR specs into Helm release operations,
enabling the operator to manage the full kube-llmops stack through a single CR while
maintaining feature parity with the umbrella Helm chart.

### Architecture

```
┌─────────────┐     TranslateValues()     ┌────────────┐     Helm SDK     ┌──────────────┐
│ LLMPlatform │─────────────────────────▶│   values    │───────────────▶│  Helm Release │
│    CR       │     (spec → map)          │   map       │  Install() /   │  kube-llmops  │
│             │                           │             │  Upgrade()     │  -stack        │
└─────────────┘                           └────────────┘                └──────────────┘
```

### TranslateValues

**Source:** `internal/helmbridge/values.go`

The `TranslateValues()` function performs a direct mapping from the `LLMPlatformSpec`
struct to a nested `map[string]interface{}` that matches the umbrella chart's
`values.yaml` schema. The translation preserves the chart's section structure:

```go
func TranslateValues(platform *v1alpha1.LLMPlatform) map[string]interface{} {
    vals := map[string]interface{}{}

    // Global section — modules, modelStore, hfToken, nodePort
    global := map[string]interface{}{}
    global["modules"] = map[string]interface{}{
        "rag":      map[string]interface{}{"enabled": platform.Spec.Modules.RAG.Enabled},
        "finetune": map[string]interface{}{"enabled": platform.Spec.Modules.Finetune.Enabled},
        "security": map[string]interface{}{"enabled": platform.Spec.Modules.Security.Enabled},
    }
    vals["global"] = global

    // Top-level sections — litellm, observability, logging, keycloak, etc.
    vals["litellm"] = map[string]interface{}{
        "enabled":         platform.Spec.Gateway.Enabled,
        "routingStrategy": platform.Spec.Gateway.Routing,
    }
    // ... (postgresql, keda, fluid, ingress, etc.)
    return vals
}
```

### SDKClient

**Source:** `internal/helmbridge/client.go`

The `SDKClient` wraps Helm v3's Go SDK (`helm.sh/helm/v3/pkg/action`) to provide four
operations:

```go
type HelmClient interface {
    Install(name, namespace, chartPath string, values map[string]interface{}) (*release.Release, error)
    Upgrade(name, namespace, chartPath string, values map[string]interface{}) (*release.Release, error)
    GetRelease(name, namespace string) (*release.Release, error)
    Uninstall(name, namespace string) error
}
```

Each operation creates a fresh `action.Configuration` scoped to the target namespace,
using `"secret"` as the storage backend (Helm stores release state in Kubernetes Secrets).
Key design decisions:

- **`CreateNamespace: false`** — the operator does not create namespaces; they must exist.
- **`Wait: false`** — the operator does not block on pod readiness; it relies on
  subsequent reconciliation loops to observe convergence.
- Chart is loaded from disk via `loader.Load(chartPath)`. The default chart path is
  `charts/kube-llmops-stack`, configurable via the `ChartPath` field on the reconciler.

### Chart Path Configuration

The chart path is injected at controller setup time:

```go
&controller.LLMPlatformReconciler{
    Client:     mgr.GetClient(),
    Scheme:     mgr.GetScheme(),
    HelmClient: &helmbridge.SDKClient{},
    ChartPath:  "charts/kube-llmops-stack", // Configurable
}
```

### Mock Client

A `MockHelmClient` is provided for unit testing. It records the last values map passed to
`Install()` or `Upgrade()` and returns configurable errors, enabling tests to verify
value translation without an actual Helm installation:

```go
type MockHelmClient struct {
    LastValues map[string]interface{}
    InstallErr error
    UpgradeErr error
}
```

---

## 8. FineTuneRun Pipeline

The `FineTuneRun` controller orchestrates multi-step fine-tuning pipelines by generating
Argo Workflow DAGs. Each pipeline consists of six stages that execute sequentially,
with the entire workflow managed as a child resource of the `FineTuneRun` CR.

### DAG Structure

```
┌──────────────────────────────────────────────────────────────────────┐
│                        Argo Workflow DAG                              │
│                                                                      │
│  ┌──────────────┐     ┌──────────────┐     ┌──────────────────┐     │
│  │ prepare-data  │────▶│   finetune   │────▶│  merge-upload    │     │
│  │               │     │              │     │                  │     │
│  │ model-loader  │     │ llamafactory │     │  model-loader    │     │
│  │ image         │     │ image        │     │  image           │     │
│  │               │     │              │     │                  │     │
│  │ Download base │     │ Run training │     │ Merge LoRA +     │     │
│  │ model + data  │     │ with MLflow  │     │ upload to MinIO  │     │
│  └──────────────┘     └──────────────┘     └────────┬─────────┘     │
│                                                      │               │
│  ┌──────────────┐     ┌──────────────┐     ┌────────▼─────────┐     │
│  │    deploy     │◀────│ quality-gate │◀────│    evaluate      │     │
│  │               │     │              │     │                  │     │
│  │ kubectl       │     │ python:3.13  │     │  python:3.13     │     │
│  │ image         │     │              │     │                  │     │
│  │               │     │ Check loss   │     │ Run eval suite   │     │
│  │ Create MD CR  │     │ thresholds   │     │ on test split    │     │
│  └──────────────┘     └──────────────┘     └──────────────────┘     │
│                                                                      │
│  Volume: workspace (10Gi PVC, shared across all steps)               │
│  Deadline: 21600s (6 hours)                                          │
│  ServiceAccount: {releaseName}-finetune                              │
└──────────────────────────────────────────────────────────────────────┘
```

### Workflow Construction

**Source:** `internal/builder/workflow.go`

The `BuildArgoWorkflow()` function constructs an `unstructured.Unstructured` Argo
Workflow to avoid importing Argo's Go types as a compile-time dependency. The workflow
name is derived from `{ftr.Name}-{outputName[:8]}`:

```go
wf := builder.BuildArgoWorkflow(ftr, releaseName)
// Sets: apiVersion: argoproj.io/v1alpha1, kind: Workflow
```

### Stage Details

| Stage          | Image                              | Purpose                                |
|----------------|------------------------------------|----------------------------------------|
| `prepare-data` | `kube-llmops/model-loader:latest`  | Download base model + training data from MinIO/HF |
| `finetune`     | `hiyouga/llamafactory:latest`      | Execute LLaMA-Factory training with MLflow tracking |
| `merge-upload` | `kube-llmops/model-loader:latest`  | Merge LoRA adapters, upload artifacts to MinIO |
| `evaluate`     | `python:3.13-slim`                 | Run evaluation on held-out test split  |
| `quality-gate` | `python:3.13-slim`                 | Compare metrics against configured thresholds |
| `deploy`       | `bitnami/kubectl:latest`           | Create/update a `ModelDeployment` CR for the fine-tuned model |

### LLaMA-Factory Integration

The `finetune` step uses the LLaMA-Factory image (`hiyouga/llamafactory`) with the
following environment configuration:

```yaml
env:
  - name: MLFLOW_TRACKING_URI
    value: "http://{releaseName}-mlflow:5000"
  - name: MLFLOW_EXPERIMENT_NAME
    value: "{outputName}"
```

GPU resources from `spec.resources` are applied to the finetune container:

```go
if res := buildGPUResources(ftr.Spec.Resources); len(res) > 0 {
    finetuneContainer["resources"] = res
}
```

### Phase Synchronization

The `FineTuneRunReconciler` maps Argo Workflow phases to `FineTuneRun` phases:

```
┌─────────────────┐     ┌───────────────────┐
│  Argo Phase      │────▶│ FineTuneRun Phase  │
├─────────────────┤     ├───────────────────┤
│  ""  (pending)   │────▶│  Pending           │
│  Running         │────▶│  Training          │
│  Succeeded       │────▶│  Succeeded         │
│  Failed          │────▶│  Failed            │
│  Error           │────▶│  Failed            │
└─────────────────┘     └───────────────────┘
```

The controller requeues every 30 seconds for non-terminal workflows. Completion time is
recorded when the workflow reaches `Succeeded` or `Failed`. If the Argo Workflow is
externally deleted while the `FineTuneRun` is still running, the phase transitions to
`Failed` with a `WorkflowNotFound` condition.

---

## 9. RBAC Model

The operator follows the principle of least privilege with a tiered RBAC model. The
ClusterRole grants exactly the permissions each controller requires, no more.

### Operator ClusterRole

**Source:** `config/rbac/role.yaml`, `charts/kube-llmops-operator/templates/rbac.yaml`

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: manager-role
rules:
  # CRD access — full lifecycle management
  - apiGroups: ["llmops.kubellmops.io"]
    resources:
      - modeldeployments
      - llmplatforms
      - finetuneruns
    verbs: [get, list, watch, create, update, patch, delete]

  # Status subresource — controllers must update status
  - apiGroups: ["llmops.kubellmops.io"]
    resources:
      - modeldeployments/status
      - llmplatforms/status
      - finetuneruns/status
    verbs: [get, update, patch]

  # Finalizer subresource — controllers must manage finalizers
  - apiGroups: ["llmops.kubellmops.io"]
    resources:
      - modeldeployments/finalizers
      - llmplatforms/finalizers
      - finetuneruns/finalizers
    verbs: [update]

  # K8s resources — ModelDeployment controller creates these
  - apiGroups: ["apps"]
    resources: [deployments]
    verbs: [get, list, watch, create, update, patch, delete]

  - apiGroups: [""]
    resources: [services, persistentvolumeclaims, configmaps, secrets]
    verbs: [get, list, watch, create, update, patch, delete]

  # Argo Workflows — FineTuneRun controller creates these
  - apiGroups: ["argoproj.io"]
    resources: [workflows, workflowtemplates]
    verbs: [get, list, watch, create, update, patch, delete]
```

### Permission Breakdown by Controller

```
┌───────────────────────────┬──────────────────────────────────────────┐
│ Controller                 │ Required Permissions                     │
├───────────────────────────┼──────────────────────────────────────────┤
│ ModelDeploymentReconciler  │ modeldeployments/* (CRUD+status+finalizer)│
│                            │ llmplatforms (get, list, watch)          │
│                            │ deployments (CRUD)                       │
│                            │ services (CRUD)                          │
│                            │ persistentvolumeclaims (CRUD)            │
├───────────────────────────┼──────────────────────────────────────────┤
│ LLMPlatformReconciler      │ llmplatforms/* (CRUD+status+finalizer)   │
│                            │ (Helm SDK handles K8s resources via      │
│                            │  its own service account)                │
├───────────────────────────┼──────────────────────────────────────────┤
│ FineTuneRunReconciler      │ finetuneruns/* (CRUD+status+finalizer)   │
│                            │ workflows (CRUD)                         │
│                            │ workflowtemplates (CRUD)                 │
└───────────────────────────┴──────────────────────────────────────────┘
```

### User-Facing Roles

The operator ships three role tiers per CRD for RBAC delegation to end users:

| Role                         | Permissions            | Use Case                |
|------------------------------|------------------------|-------------------------|
| `modeldeployment-admin-role` | Full CRUD + status     | Platform engineers      |
| `modeldeployment-editor-role`| Create, update, delete | ML engineers            |
| `modeldeployment-viewer-role`| Get, list, watch       | Read-only dashboards    |

The same pattern applies for `llmplatform-*` and `finetunerun-*` roles.

### Leader Election

The operator uses lease-based leader election with ID `61f31610.kubellmops.io`. This
requires additional RBAC for `leases` and `configmaps` in the operator's namespace,
provided by the `leader-election-role` in `config/rbac/leader_election_role.yaml`.

### ServiceAccount

Each operator instance runs under a dedicated ServiceAccount. The Helm chart creates this
with a configurable name via `serviceAccount.name` in `values.yaml`, bound to the
ClusterRole via a `ClusterRoleBinding`.

---

## 10. Failure Modes

The operator is designed with graceful degradation principles. No single component
failure should cascade into cluster-wide disruption.

### Argo CRD Missing

**Trigger:** `FineTuneRunReconciler` attempts to create a Workflow, but the
`argoproj.io/v1alpha1` CRD is not installed in the cluster.

**Detection:** The `isNoCRDError()` function catches three error types:

```go
func isNoCRDError(err error) bool {
    if apierrors.IsNotFound(err) {
        return true
    }
    if apimeta.IsNoMatchError(err) {
        return true      // "no matches for kind" from discovery
    }
    if meta, ok := err.(*apierrors.StatusError); ok {
        return meta.ErrStatus.Code == 404
    }
    return false
}
```

**Behavior:** The `FineTuneRun` transitions to `Failed` with a `WorkflowReady` condition
set to `ArgoCRDMissing`. The controller does **not** requeue, preventing an infinite
retry loop. Other controllers (`ModelDeployment`, `LLMPlatform`) continue operating
normally. The operator logs the error but remains healthy.

**Recovery:** Install the Argo Workflows CRD, then delete and recreate the `FineTuneRun`.

### Helm Install/Upgrade Failure

**Trigger:** `LLMPlatformReconciler` calls `HelmClient.Install()` or `Upgrade()` and
receives an error (chart not found, invalid values, template rendering failure).

**Behavior:**

```
Phase: Installing/Upgrading → Failed
Condition: HelmRelease = False
  Reason: InstallFailed / UpgradeFailed
  Message: "Helm install failed: {error details}"
```

The error is returned to the controller-runtime work queue, which will retry with
exponential backoff. The operator remains healthy for other CRs.

**Recovery:** Fix the CR spec or chart issue. The next reconciliation loop will attempt
the install/upgrade again automatically.

### Gateway Unreachable

**Trigger:** `ModelDeploymentReconciler` calls `GatewayClient.RegisterModel()` or
`DeregisterModel()` and the LiteLLM gateway is down or unreachable.

**Behavior:** Gateway errors are **non-fatal**. The error is logged but does not prevent
the `ModelDeployment` from reaching `Ready` phase:

```go
if err := r.GatewayClient.RegisterModel(ctx, model); err != nil {
    log.Error(err, "failed to register model with gateway")
    // No return — the model is still Ready for direct access
}
```

The model is accessible via its Service endpoint even without gateway registration.
Registration will succeed on the next reconciliation when the gateway recovers.

**Recovery:** Automatic on next reconciliation loop when the gateway becomes available.

### Workflow Deleted Externally

**Trigger:** An Argo Workflow referenced by `status.argoWorkflow` is manually deleted
or garbage-collected while the `FineTuneRun` is in a non-terminal state.

**Behavior:** Phase transitions to `Failed` with condition `WorkflowNotFound`:

```go
if apierrors.IsNotFound(err) {
    ftr.Status.Phase = "Failed"
    setCondition(&ftr.Status.Conditions, metav1.Condition{
        Reason:  "WorkflowNotFound",
        Message: "Argo Workflow was deleted",
    })
}
```

**Recovery:** Delete the `FineTuneRun` and create a new one.

### Pod Scheduling Failures

**Trigger:** Model serving pods cannot be scheduled (insufficient GPU, node taints,
resource quota exceeded).

**Behavior:** The `ModelDeployment` remains in `Deploying` phase with `readyReplicas: 0`.
The `Ready` condition shows `ReplicasNotReady` with `0/{desired} replicas ready`.

**Recovery:** Add GPU nodes, adjust resource requests, or configure spot scheduling.

---

## 11. Operator Observability

The operator exposes metrics, health probes, and structured status conditions following
Kubernetes operator best practices.

### Metrics Endpoint

**Address:** `:8080` (configurable via `--metrics-bind-address`)
**Source:** controller-runtime built-in metrics server

The operator exposes standard controller-runtime metrics including:

```
┌────────────────────────────────────────────────────────────────────┐
│                    Metrics (controller-runtime)                     │
├────────────────────────────────────────────────────────────────────┤
│ controller_runtime_reconcile_total{controller, result}             │
│ controller_runtime_reconcile_errors_total{controller}              │
│ controller_runtime_reconcile_time_seconds{controller}              │
│ workqueue_depth{name}                                              │
│ workqueue_adds_total{name}                                         │
│ workqueue_queue_duration_seconds{name}                              │
│ workqueue_work_duration_seconds{name}                              │
│ workqueue_retries_total{name}                                      │
│ rest_client_requests_total{code, host, method}                     │
│ rest_client_request_duration_seconds_bucket{host, method}          │
└────────────────────────────────────────────────────────────────────┘
```

Controller names in metrics labels correspond to the `Named()` values set during
controller setup: `modeldeployment`, `llmplatform`, `finetunerun`.

Secure metrics are enabled by default (`--metrics-secure=true`) with TLS certificate
support via `--metrics-cert-path`. When secure metrics are enabled, the
`FilterProvider` is configured with `filters.WithAuthenticationAndAuthorization` to
enforce RBAC on the metrics endpoint. A dedicated `metrics-reader` ClusterRole and
`metrics-auth-role` are provided in `config/rbac/`.

### Health Probes

**Address:** `:8081` (configurable via `--health-probe-bind-address`)

| Endpoint     | Check       | Purpose                                           |
|--------------|-------------|---------------------------------------------------|
| `/healthz`   | `Ping`      | Liveness — is the operator process alive?         |
| `/readyz`    | `Ping`      | Readiness — is the operator ready to reconcile?   |

Both probes use the simple `healthz.Ping` checker. The probes are registered in
`cmd/main.go`:

```go
mgr.AddHealthzCheck("healthz", healthz.Ping)
mgr.AddReadyzCheck("readyz", healthz.Ping)
```

These endpoints are used by the operator's own Deployment in the Helm chart to configure
`livenessProbe` and `readinessProbe` on the manager container.

### Status Conditions

Each CRD maintains structured conditions following the Kubernetes conditions convention
(`metav1.Condition`). The `setCondition()` helper function performs upsert semantics:

```go
func setCondition(conditions *[]metav1.Condition, cond metav1.Condition) {
    for i, existing := range *conditions {
        if existing.Type == cond.Type {
            (*conditions)[i] = cond
            return
        }
    }
    *conditions = append(*conditions, cond)
}
```

**ModelDeployment conditions:**

| Type    | Status | Reason             | When                              |
|---------|--------|--------------------|-----------------------------------|
| `Ready` | True   | `AllReplicasReady` | All desired replicas are ready    |
| `Ready` | False  | `ReplicasNotReady` | Fewer than desired replicas ready |

**LLMPlatform conditions:**

| Type          | Status | Reason             | When                          |
|---------------|--------|--------------------|-------------------------------|
| `HelmRelease` | True   | `InstallSucceeded` | Helm install completed        |
| `HelmRelease` | True   | `UpgradeSucceeded` | Helm upgrade completed        |
| `HelmRelease` | False  | `Installing`       | Install in progress           |
| `HelmRelease` | False  | `Upgrading`        | Upgrade in progress           |
| `HelmRelease` | False  | `InstallFailed`    | Helm install error            |
| `HelmRelease` | False  | `UpgradeFailed`    | Helm upgrade error            |

**FineTuneRun conditions:**

| Type            | Status | Reason               | When                           |
|-----------------|--------|-----------------------|--------------------------------|
| `WorkflowReady` | True   | `ArgoPhaseSucceeded` | Workflow completed successfully |
| `WorkflowReady` | False  | `WorkflowCreated`    | Workflow just created          |
| `WorkflowReady` | False  | `ArgoCRDMissing`     | Argo CRD not installed         |
| `WorkflowReady` | False  | `WorkflowNotFound`   | Workflow externally deleted     |
| `WorkflowReady` | False  | `ArgoPhaseRunning`   | Workflow executing             |
| `WorkflowReady` | False  | `ArgoPhaseFailed`    | Workflow failed                |

### Prometheus ServiceMonitor

A `ServiceMonitor` resource is provided in `config/prometheus/monitor.yaml` for
Prometheus Operator-based scraping of the metrics endpoint.

---

## 12. Security

The operator follows defense-in-depth principles to minimize the attack surface of
LLM infrastructure management.

### Secret Management

The `LLMPlatformSpec` includes fields for `hfToken`, `modelStore.accessKey`, and
`modelStore.secretKey`. In the current v1alpha1 API, these are plaintext strings for
development convenience. **For production deployments, these must be replaced with
`secretKeyRef` references** to Kubernetes Secrets:

```yaml
# Development (current v1alpha1 — NOT for production)
spec:
  hfToken: "hf_abc123..."
  modelStore:
    accessKey: minioadmin
    secretKey: minioadmin

# Recommended production pattern (future API evolution)
spec:
  hfTokenSecretRef:
    name: hf-credentials
    key: token
  modelStore:
    accessKeySecretRef:
      name: minio-credentials
      key: access-key
```

The gateway client's `masterKey` is injected as a constructor argument to the
`HTTPClient`, not stored in any CR spec. In the default configuration, the `NoopClient`
is used, which carries no credentials.

### RBAC Least-Privilege

The operator's ClusterRole is scoped to the minimum set of API groups and resources
required by each controller. Notable restrictions:

- **No access to Nodes, Namespaces, or cluster-scoped settings** — the operator cannot
  modify cluster topology.
- **No access to Secrets directly from CRD controllers** — the Helm SDK accesses Secrets
  for release storage through its own action configuration, but the controller code itself
  does not read arbitrary Secrets.
- **Status and finalizer subresources are separated** — controllers can update status
  without needing full resource update permissions.

### Namespace Isolation

Each `ModelDeployment`, `LLMPlatform`, and `FineTuneRun` operates within its declared
namespace. The `ModelDeploymentReconciler` looks up `LLMPlatform` resources only in the
same namespace as the `ModelDeployment`:

```go
r.List(ctx, platformList, client.InNamespace(md.Namespace))
```

This prevents cross-namespace information leakage. Multi-tenant deployments can use
separate namespaces with namespace-scoped `RoleBindings` delegated from the operator's
`ClusterRole`.

### Network Security

The operator's Deployment does not expose any externally-accessible ports. The metrics
endpoint (`:8080`) and health probe endpoint (`:8081`) are cluster-internal only:

- **Metrics:** Protected by TLS and RBAC authentication when `--metrics-secure=true`
  (default). The `metrics-auth-role` ClusterRole gates access.
- **Health probes:** Unauthenticated but internal-only (`/healthz`, `/readyz`).
- **HTTP/2 disabled by default** to mitigate CVE-2023-44487 (Rapid Reset) and
  CVE-2023-39325 (Stream Cancellation):

```go
disableHTTP2 := func(c *tls.Config) {
    c.NextProtos = []string{"http/1.1"}
}
```

A `NetworkPolicy` resource is provided in `config/network-policy/allow-metrics-traffic.yaml`
to restrict metrics endpoint access to authorized Prometheus instances.

### Container Security

Model serving Deployments created by the operator follow these security practices:

- **Service links disabled** (`enableServiceLinks: false`) — prevents DNS-based
  environment variable injection attacks.
- **No privileged containers** — inference engines run as non-root where supported.
- **Toleration-based GPU scheduling** — pods are only scheduled on nodes with the
  appropriate GPU taint, preventing accidental scheduling on control-plane nodes.
- **Termination grace period** (90s) allows models to complete in-flight requests
  during graceful shutdown.

### Webhook Validation

The operator includes a validating webhook for `ModelDeployment` resources
(`api/v1alpha1/modeldeployment_webhook.go`) that enforces:

- `source` must be non-empty.
- `engine` must be one of: `auto`, `vllm`, `tei`, `llamacpp`.
- `replicas` must be `>= 0`.
- `gpu` must be `>= 0`.
- `accelerator` must be one of: `nvidia`, `amd`, `gaudi`.
- Canary `source` must be non-empty and `weight` must be `0-100`.

This prevents invalid configurations from reaching the reconciler, reducing error
handling complexity in the controller code.
