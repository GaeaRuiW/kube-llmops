# Phase 6: Kubernetes Operator — Design Spec

> Declarative LLM infrastructure management via Custom Resource Definitions.

**Date:** 2026-04-05
**Status:** Approved
**Scope:** kube-llmops v1.0.0 — Phase 6 Sub-project 1 (Operator + CRDs)
**Checkpoint:** `kubectl apply -f model.yaml` deploys a model end-to-end; `kubectl get modeldeployments` shows live status.

---

## 1. Overview

Add a Kubernetes Operator to kube-llmops so that users can manage LLM infrastructure declaratively through Custom Resources instead of Helm values files. The Operator provides a Kubernetes-native experience: deploy a model with `kubectl apply`, scale with `kubectl patch`, monitor with `kubectl get`.

**Key decisions:**

- **Framework:** Go + Kubebuilder (industry standard — used by KubeAI, KServe, KubeRay, Seldon, NVIDIA GPU Operator)
- **CRD granularity:** 3 CRDs — `ModelDeployment` (per-model), `LLMPlatform` (cluster infra), `FineTuneRun` (training jobs)
- **Helm coexistence:** Both Helm and Operator are first-class paths with full feature parity. Mutually exclusive per cluster.
- **Hybrid reconciliation:** ModelDeployment manages K8s resources directly (speed + rich status); LLMPlatform uses Helm SDK internally (automatic feature parity for 20+ subcharts)
- **Status:** Rich subresource on all CRDs — phase, component health, endpoints, conditions

---

## 2. CRD Definitions

### 2.1 ModelDeployment

The primary user-facing CRD. One CR per model. High-frequency operations (create, scale, update, delete).

```yaml
apiVersion: llmops.kubellmops.io/v1alpha1
kind: ModelDeployment
metadata:
  name: gemma-4-26b
  namespace: default
spec:
  # Required
  source: cyankiwi/gemma-4-26B-A4B-it-AWQ-4bit   # HuggingFace model ID or path
  
  # Engine selection (default: auto)
  engine: auto              # auto | vllm | tei | llamacpp
  # Auto-detection rules:
  #   *GGUF*           → llamacpp
  #   *rerank*, bge-*, e5-*, *embedding* → tei
  #   everything else  → vllm
  
  # Scaling
  replicas: 1
  
  # Resources
  resources:
    gpu: 1                  # nvidia.com/gpu count (0 for CPU-only)
    memory: 24Gi
    cpu: "4"                # optional, defaults based on engine
  
  # Accelerator (default: nvidia)
  accelerator: nvidia       # nvidia | amd | gaudi
  migDevice: ""             # e.g. "nvidia.com/mig-3g.24gb" — overrides gpu field
  
  # Engine-specific arguments (passed directly to engine CLI)
  engineArgs:
    --gpu-memory-utilization: "0.93"
    --max-model-len: "8192"
    --dtype: "half"
    --enforce-eager: ""
  
  # Performance
  prefixCaching: false      # enable vLLM automatic prefix caching
  
  # Spot/preemptible GPU
  spot:
    enabled: false
    provider: ""            # aws | gcp | azure | karpenter (auto-sets tolerations)
  
  # Canary deployment (optional)
  canary:
    source: other/model-v2  # canary model source
    weight: 20              # percentage of traffic to canary (0-100)
    resources:
      gpu: 1
      memory: 24Gi
  
  # Model store override (optional — defaults from LLMPlatform)
  modelStore:
    endpoint: ""            # override LLMPlatform modelStore
    bucket: ""
status:
  phase: Ready              # Pending | Downloading | Deploying | Ready | Degraded | Failed
  engine: vllm              # resolved engine (after auto-detection)
  endpoint: "http://gemma-4-26b:8000/v1"
  readyReplicas: 1
  totalReplicas: 1
  modelSize: "14.2 GB"      # discovered after download
  conditions:
    - type: Available
      status: "True"
      lastTransitionTime: "2026-04-05T12:00:00Z"
    - type: ModelLoaded
      status: "True"
      message: "Model loaded from MinIO cache in 0.8s"
    - type: GatewayRegistered
      status: "True"
      message: "Registered with LiteLLM gateway"
  canary:
    phase: Ready
    endpoint: "http://gemma-4-26b-canary:8000/v1"
    readyReplicas: 1
```

**Printer columns** (`kubectl get modeldeployments`):

```
NAME           ENGINE   REPLICAS   PHASE   ENDPOINT                          AGE
gemma-4-26b    vllm     1/1        Ready   http://gemma-4-26b:8000/v1        5m
bge-small-en   tei      1/1        Ready   http://bge-small-en:8080/v1       5m
```

### 2.2 LLMPlatform

Cluster-level infrastructure. One instance per cluster. Low-frequency changes (initial setup, module toggles).

```yaml
apiVersion: llmops.kubellmops.io/v1alpha1
kind: LLMPlatform
metadata:
  name: kube-llmops
spec:
  # AI Gateway (LiteLLM)
  gateway:
    enabled: true
    routing: latency-based-routing    # simple-shuffle | latency-based-routing
    image:
      tag: "v1.63.14.dev2"
    rateLimiting:
      enabled: false
    budgetControl:
      enabled: false
  
  # Observability (Prometheus + Grafana + Langfuse)
  observability:
    enabled: true
    grafana:
      adminPassword: "admin"
      oidc:
        enabled: false
    langfuse:
      enabled: true
  
  # Logging (Fluent Bit + Loki)
  logging:
    enabled: true
  
  # Module switches
  modules:
    rag:
      enabled: true           # Dify + Milvus + LightRAG + rag-eval
    finetune:
      enabled: false          # LLaMA-Factory + Argo + MLflow + JupyterHub
    security:
      enabled: false          # LLM-Guard + NetworkPolicy + multi-tenancy
  
  # Model store (MinIO)
  modelStore:
    enabled: true
    endpoint: "kube-llmops-minio:9000"
    bucket: "models"
    accessKey: "minioadmin"
    secretKey: "minioadmin"
    hfTransferConcurrency: 32
    image: "kube-llmops/model-loader:latest"
  
  # HuggingFace token (for gated models)
  hfToken: ""
  
  # SSO (Keycloak)
  keycloak:
    enabled: false
  
  # Database (PostgreSQL)
  postgresql:
    enabled: true
  
  # Autoscaling (KEDA)
  keda:
    enabled: false
  
  # Access
  nodePort:
    enabled: true
    host: "172.29.193.187"
  ingress:
    enabled: false
    className: "traefik"
    host: "llmops.local"
status:
  phase: Ready                # Pending | Installing | Ready | Upgrading | Degraded | Failed
  helmRelease: "kube-llmops"
  helmRevision: 3
  components:
    gateway:
      phase: Ready
      endpoint: "http://kube-llmops-litellm:4000"
      nodePort: 30400
    grafana:
      phase: Ready
      endpoint: "http://kube-llmops-grafana:3000"
      nodePort: 30300
    prometheus:
      phase: Ready
      endpoint: "http://kube-llmops-prometheus:9090"
    langfuse:
      phase: Ready
      endpoint: "http://kube-llmops-langfuse:3000"
      nodePort: 30301
    minio:
      phase: Ready
      endpoint: "http://kube-llmops-minio:9000"
    postgresql:
      phase: Ready
    dify:
      phase: Ready
      endpoint: "http://kube-llmops-dify-web:3000"
      nodePort: 30500
    milvus:
      phase: Ready
  conditions:
    - type: Available
      status: "True"
    - type: HelmReleaseReady
      status: "True"
      message: "Helm release kube-llmops revision 3 deployed"
```

**Printer columns** (`kubectl get llmplatforms`):

```
NAME           PHASE   GATEWAY   GRAFANA   MODULES          AGE
kube-llmops    Ready   Ready     Ready     rag              10m
```

### 2.3 FineTuneRun

On-demand training jobs. One CR per fine-tuning task. Job-like lifecycle (create → run → complete/fail).

```yaml
apiVersion: llmops.kubellmops.io/v1alpha1
kind: FineTuneRun
metadata:
  name: gemma-lora-v1
spec:
  # Model
  baseModel: cyankiwi/gemma-4-26B-A4B-it-AWQ-4bit
  outputName: gemma-4-lora-v1
  
  # Method
  method: lora                  # lora | qlora | full
  
  # Data
  dataSource:
    type: minio                 # minio | huggingface | pvc
    path: "s3://datasets/my-data/"
    format: alpaca              # alpaca | sharegpt
  
  # Training hyperparameters
  training:
    epochs: 3
    batchSize: 4
    learningRate: 2e-4
    gradientAccumulationSteps: 4
    warmupRatio: 0.1
    loraRank: 16                # LoRA/QLoRA only
    loraAlpha: 32               # LoRA/QLoRA only
    loraTarget: "all"           # target modules
  
  # Resources
  resources:
    gpu: 1
    memory: 24Gi
  
  # Evaluation
  evaluation:
    enabled: true
    dataset: ""                 # defaults to validation split of training data
  
  # Quality gate
  qualityGate:
    enabled: true
    thresholds:
      minEvalLoss: 0.8
      maxTrainLoss: 0.5
  
  # Deployment (after quality gate passes)
  deploy:
    enabled: false              # auto-deploy as ModelDeployment after success
    canaryWeight: 20            # initial canary traffic percentage
status:
  phase: Succeeded              # Pending | DataPreparing | Training | Evaluating | QualityGate | Deploying | Succeeded | Failed
  argoWorkflow: "gemma-lora-v1-abc12"
  startTime: "2026-04-05T14:00:00Z"
  completionTime: "2026-04-05T16:30:00Z"
  metrics:
    trainLoss: 0.42
    evalLoss: 0.51
    trainingDuration: "2h30m"
  mlflow:
    runId: "abc123def456"
    experimentName: "gemma-lora-v1"
    artifactUri: "s3://mlflow/artifacts/abc123def456"
  qualityGate:
    passed: true
    message: "All thresholds met"
  outputModel:
    source: "s3://models/gemma-4-lora-v1/"
    modelDeployment: "gemma-4-lora-v1"   # if deploy.enabled=true
  conditions:
    - type: DataReady
      status: "True"
    - type: TrainingComplete
      status: "True"
    - type: QualityGatePassed
      status: "True"
```

**Printer columns** (`kubectl get finetuneruns`):

```
NAME             BASE MODEL                                METHOD   PHASE       LOSS    DURATION   AGE
gemma-lora-v1    cyankiwi/gemma-4-26B-A4B-it-AWQ-4bit     lora     Succeeded   0.42    2h30m      3h
```

---

## 3. Operator Architecture

### 3.1 Controller Structure

Single operator binary with three controllers:

```
kube-llmops-operator (single binary)
│
├── ModelDeployment Controller
│   ├── watches: ModelDeployment CRs
│   ├── manages: Deployment, Service, PVC, InitContainer (model-loader)
│   ├── calls: LiteLLM API (register/deregister model)
│   └── reads: LLMPlatform (modelStore config, gateway endpoint)
│
├── LLMPlatform Controller
│   ├── watches: LLMPlatform CR (singleton per cluster)
│   ├── manages: Helm Release via Helm SDK
│   ├── renders: charts/kube-llmops-stack with CR spec as values
│   └── watches: deployed resources for component health
│
└── FineTuneRun Controller
    ├── watches: FineTuneRun CRs
    ├── manages: Argo Workflow (WorkflowTemplate + Workflow)
    ├── reads: LLMPlatform (MLflow endpoint, MinIO config)
    └── optionally creates: ModelDeployment (if deploy.enabled=true)
```

### 3.2 Hybrid Reconciliation Strategy

The three controllers use different reconciliation strategies optimized for their use case:

**ModelDeployment — Direct Resource Management**

The operator creates and manages Kubernetes resources (Deployments, Services, PVCs) directly using the controller-runtime client. No Helm involved.

Rationale:
- Models are the high-frequency resource (create/delete/scale constantly)
- Direct management provides instant status feedback with no Helm release cycle overhead
- Engine auto-detection logic is compact (~30 lines of Go) and easy to maintain
- The operator registers/deregisters models with LiteLLM via its REST API
- OwnerReferences ensure garbage collection on CR deletion

**LLMPlatform — Helm SDK Bridge**

The operator uses the Helm SDK (`helm.sh/helm/v3/pkg/action`) to manage the `kube-llmops-stack` umbrella chart programmatically. The CR spec is translated to Helm values.

Rationale:
- The umbrella chart has 20+ subcharts with complex inter-dependencies
- Reimplementing all subchart logic in Go would be enormous and fragile
- Helm SDK gives automatic feature parity — any new subchart feature is immediately available
- The operator embeds the chart (or references a chart repository)
- Upgrades are idempotent: changing the CR spec triggers `helm upgrade`

**FineTuneRun — Argo Workflow Creation**

The operator creates Argo Workflow CRs directly. Each FineTuneRun maps to one Argo Workflow with the standard DAG (prepare-data → finetune → merge-upload → evaluate → quality-gate → deploy).

Rationale:
- Argo Workflows handles the actual pipeline orchestration
- The operator translates the simplified FineTuneRun spec into the full Argo Workflow spec
- Status is aggregated from Argo Workflow status + MLflow metrics
- If `deploy.enabled=true` and quality gate passes, the operator creates a ModelDeployment CR

### 3.3 ModelDeployment Reconciliation Flow

```
┌─ User applies ModelDeployment CR ──────────────────────────────┐
│                                                                 │
│  1. Resolve engine (auto-detect from spec.source)               │
│  2. Look up LLMPlatform CR for modelStore + gateway config      │
│  3. Create/update PVC for model cache                           │
│  4. Create/update Deployment:                                   │
│       initContainer: model-loader                               │
│         - Check MinIO → fallback HuggingFace → upload MinIO     │
│       container: engine image (vllm/tei/llamacpp)               │
│         - Mount model PVC                                       │
│         - Apply engineArgs                                      │
│         - Set resource limits (GPU, memory, CPU)                │
│         - Add accelerator tolerations if needed                 │
│  5. Create/update Service (ClusterIP)                           │
│  6. Wait for Deployment rollout                                 │
│  7. Register model with LiteLLM gateway (POST /model/new)      │
│  8. If canary defined: create canary Deployment + Service       │
│     + configure LiteLLM weight routing                          │
│  9. Update CR status: phase=Ready, endpoint, engine, replicas   │
└─────────────────────────────────────────────────────────────────┘

┌─ User deletes ModelDeployment CR ──────────────────────────────┐
│                                                                 │
│  1. Finalizer triggers                                          │
│  2. Deregister from LiteLLM gateway (POST /model/delete)        │
│  3. Remove finalizer → Kubernetes GC deletes owned resources    │
└─────────────────────────────────────────────────────────────────┘
```

### 3.4 LLMPlatform Reconciliation Flow

```
┌─ User applies LLMPlatform CR ──────────────────────────────────┐
│                                                                 │
│  1. Translate CR spec to Helm values map:                       │
│       spec.gateway       → litellm.* values                    │
│       spec.observability → observability.* values               │
│       spec.modules       → global.modules.* values              │
│       spec.modelStore    → global.modelStore.* + fluid.* values │
│       spec.nodePort      → global.nodePort.* values             │
│       (etc.)                                                    │
│  2. Check if Helm release "kube-llmops" exists:                 │
│       - No  → helm install                                      │
│       - Yes → helm upgrade (if values changed)                  │
│  3. Watch key resources for component health:                   │
│       - Deployments (ready replicas)                            │
│       - Services (endpoints exist)                              │
│       - StatefulSets (PostgreSQL, Milvus)                       │
│  4. Aggregate component status into CR status                   │
│  5. Set phase: Ready if all enabled components healthy          │
│                Degraded if some components unhealthy             │
│                Failed if Helm release failed                    │
└─────────────────────────────────────────────────────────────────┘
```

### 3.5 Engine Auto-Detection (Go)

Ported from the existing Helm `_helpers.tpl` resolveEngine logic:

```go
func ResolveEngine(source string, explicit string) string {
    if explicit != "" && explicit != "auto" {
        return explicit
    }
    s := strings.ToLower(source)
    if strings.Contains(s, "gguf") {
        return "llamacpp"
    }
    if strings.Contains(s, "rerank") ||
       strings.HasPrefix(s, "baai/bge-") ||
       strings.HasPrefix(s, "intfloat/e5-") ||
       strings.Contains(s, "embedding") {
        return "tei"
    }
    return "vllm"
}
```

### 3.6 LiteLLM Gateway Integration

ModelDeployment controller communicates with LiteLLM via its REST API:

| Operation | API Call | When |
|-----------|----------|------|
| Register model | `POST /model/new` | Deployment reaches Ready state |
| Deregister model | `POST /model/delete` | CR deleted (finalizer) |
| Health check | `GET /health` | Periodic reconciliation |
| Update routing weights | `POST /model/update` | Canary weight changed |

The gateway endpoint is discovered from the LLMPlatform CR status.

---

## 4. Helm Coexistence

### 4.1 Two Installation Paths

Users choose ONE path per cluster:

**Path A: Helm (existing)**
```bash
helm install kube-llmops charts/kube-llmops-stack \
  -f values-single-node.yaml \
  --set global.nodePort.enabled=true
```
- Models defined in `global.models[]`
- Day-2: edit values, `helm upgrade`
- No operator needed

**Path B: Operator (new)**
```bash
# Step 1: Install operator
helm install kube-llmops-operator charts/kube-llmops-operator

# Step 2: Apply platform config
kubectl apply -f platform.yaml

# Step 3: Deploy models
kubectl apply -f models/
```
- Models as individual CRs
- Day-2: `kubectl apply/edit/delete`
- GitOps: ArgoCD syncs CRs from git

### 4.2 Feature Parity

Both paths support the full feature set:

| Feature | Helm Path | Operator Path |
|---------|-----------|---------------|
| Deploy model | `global.models[]` in values | `ModelDeployment` CR |
| Engine auto-detect | `_helpers.tpl` resolveEngine | `engine.ResolveEngine()` Go |
| Model store (MinIO) | `global.modelStore` values | `LLMPlatform.spec.modelStore` |
| Module switches | `global.modules` values | `LLMPlatform.spec.modules` |
| Canary deployment | `canary:` in model entry | `ModelDeployment.spec.canary` |
| Scale-to-zero | `keda:` values | KEDA ScaledObject (via LLMPlatform) |
| Fine-tuning | `finetune:` values | `FineTuneRun` CR |
| Observability | `observability:` values | `LLMPlatform.spec.observability` |
| SSO | `keycloak:` values | `LLMPlatform.spec.keycloak` |
| NodePort | `global.nodePort` values | `LLMPlatform.spec.nodePort` |

### 4.3 Migration Path (Helm → Operator)

A migration tool (`operator/cmd/migrate/`) converts existing Helm releases to CRs:

1. Read current Helm release values: `helm get values kube-llmops`
2. Generate `LLMPlatform` CR from infrastructure values
3. Generate `ModelDeployment` CRs from `global.models[]` entries
4. User reviews generated CRs
5. `helm uninstall kube-llmops` (removes Helm-managed resources)
6. `kubectl apply -f generated/` (operator takes over)

---

## 5. Validation Webhooks

Validating admission webhooks catch errors at `kubectl apply` time:

### ModelDeployment

| Rule | Error Message |
|------|---------------|
| `spec.source` required | `source is required` |
| `spec.engine` must be valid | `engine must be one of: auto, vllm, tei, llamacpp` |
| `spec.replicas` >= 0 | `replicas must be non-negative` |
| `spec.resources.gpu` >= 0 | `gpu count must be non-negative` |
| TEI model with gpu > 0 and no explicit engine | warning: `embedding/reranker models typically run on CPU` |
| `spec.canary.weight` 0-100 | `canary weight must be between 0 and 100` |
| `spec.accelerator` must be valid | `accelerator must be one of: nvidia, amd, gaudi` |

### LLMPlatform

| Rule | Error Message |
|------|---------------|
| At most 1 LLMPlatform per namespace | `only one LLMPlatform allowed per namespace` |
| `spec.modelStore.endpoint` required if enabled | `modelStore endpoint is required` |
| Conflicting module + component override | warning: `dify.enabled=false overrides modules.rag.enabled=true` |

### FineTuneRun

| Rule | Error Message |
|------|---------------|
| `spec.baseModel` required | `baseModel is required` |
| `spec.outputName` required | `outputName is required` |
| `spec.method` must be valid | `method must be one of: lora, qlora, full` |
| `spec.dataSource.path` required for minio type | `dataSource.path is required when type is minio` |

---

## 6. Project Structure

```
operator/
├── Dockerfile
├── Makefile                            # kubebuilder standard targets
├── PROJECT                             # kubebuilder project metadata
├── go.mod / go.sum
│
├── cmd/
│   └── main.go                         # entrypoint, registers all controllers
│
├── api/
│   └── v1alpha1/
│       ├── modeldeployment_types.go    # ModelDeployment CRD types
│       ├── llmplatform_types.go        # LLMPlatform CRD types
│       ├── finetunerun_types.go        # FineTuneRun CRD types
│       ├── groupversion_info.go        # API group registration
│       └── zz_generated.deepcopy.go    # generated by controller-gen
│
├── internal/
│   ├── controller/
│   │   ├── modeldeployment_controller.go       # direct resource management
│   │   ├── modeldeployment_controller_test.go  # envtest tests
│   │   ├── llmplatform_controller.go           # Helm SDK bridge
│   │   ├── llmplatform_controller_test.go
│   │   ├── finetunerun_controller.go           # Argo Workflow creation
│   │   └── finetunerun_controller_test.go
│   │
│   ├── engine/
│   │   ├── resolver.go                 # auto-detect engine from source name
│   │   └── resolver_test.go
│   │
│   ├── helm/
│   │   ├── client.go                   # Helm SDK wrapper (install/upgrade/uninstall)
│   │   ├── values.go                   # CR spec → Helm values translation
│   │   └── values_test.go
│   │
│   ├── litellm/
│   │   ├── client.go                   # LiteLLM REST API client
│   │   └── client_test.go
│   │
│   └── builder/
│       ├── deployment.go               # Build engine Deployment spec
│       ├── service.go                  # Build Service spec
│       ├── pvc.go                      # Build PVC spec
│       └── workflow.go                 # Build Argo Workflow spec
│
├── config/
│   ├── crd/
│   │   └── bases/                      # generated CRD manifests (controller-gen)
│   │       ├── llmops.kubellmops.io_modeldeployments.yaml
│   │       ├── llmops.kubellmops.io_llmplatforms.yaml
│   │       └── llmops.kubellmops.io_finetuneruns.yaml
│   ├── manager/
│   │   └── manager.yaml                # operator Deployment manifest
│   ├── rbac/
│   │   ├── role.yaml                   # generated ClusterRole
│   │   ├── role_binding.yaml
│   │   └── service_account.yaml
│   ├── webhook/
│   │   ├── manifests.yaml              # ValidatingWebhookConfiguration
│   │   └── service.yaml
│   └── samples/
│       ├── modeldeployment_vllm.yaml
│       ├── modeldeployment_tei_embedding.yaml
│       ├── modeldeployment_tei_reranker.yaml
│       ├── modeldeployment_llamacpp.yaml
│       ├── llmplatform_minimal.yaml
│       ├── llmplatform_full.yaml
│       └── finetunerun_lora.yaml
│
├── test/
│   └── e2e/
│       ├── setup_test.go               # kind cluster setup
│       ├── modeldeployment_test.go     # E2E: apply CR → verify Deployment
│       ├── llmplatform_test.go         # E2E: apply CR → verify Helm release
│       └── finetunerun_test.go         # E2E: apply CR → verify Workflow
│
└── charts/
    └── kube-llmops-operator/
        ├── Chart.yaml
        ├── values.yaml
        ├── templates/
        │   ├── deployment.yaml         # operator pod
        │   ├── serviceaccount.yaml
        │   ├── rbac.yaml               # ClusterRole + ClusterRoleBinding
        │   └── webhook.yaml            # optional: ValidatingWebhookConfiguration
        └── crds/
            ├── llmops.kubellmops.io_modeldeployments.yaml
            ├── llmops.kubellmops.io_llmplatforms.yaml
            └── llmops.kubellmops.io_finetuneruns.yaml
```

---

## 7. Testing Strategy

| Layer | Tool | Scope | Count (est.) |
|-------|------|-------|-------------|
| Unit tests | Go `testing` | Engine resolver, Helm values translation, builder functions, LiteLLM client | ~40 |
| Integration tests | envtest (kubebuilder) | Controller reconciliation with fake K8s API server — verify resources created, status updated, finalizers work | ~30 |
| E2E tests | kind + Go test | Deploy operator to kind cluster, apply sample CRs, verify end-to-end flow | ~15 |
| Helm regression | existing pytest | Ensure `charts/kube-llmops-stack` renders correctly (93 existing tests) | 93 |
| Webhook tests | envtest | Admission webhook rejects invalid CRs, accepts valid ones | ~15 |

**Total estimated: ~100 new tests + 93 existing Helm tests.**

---

## 8. Documentation Deliverables

### 8.1 Architecture Document

**File:** `docs/architecture/operator.md`

Detailed technical reference covering:

| Section | Content |
|---------|---------|
| Design Philosophy | Why hybrid approach (native + Helm SDK); comparison with KubeAI, KServe, KubeRay |
| CRD API Reference | Every field in all 3 CRDs — types, defaults, validation rules, examples |
| Controller Internals | Reconciliation loops, state machines, error handling, retry backoff policy |
| Engine Auto-Detection | Algorithm, override mechanism, engine image mapping |
| Model Lifecycle | Full state diagram: Pending → Downloading → Deploying → Ready → Degraded → Failed |
| LiteLLM Integration | Register/deregister flow, health check, weight routing, error recovery |
| Helm SDK Bridge | CR spec → Helm values translation rules, upgrade strategy, rollback |
| FineTuneRun Pipeline | CR → Argo Workflow mapping, MLflow tracking integration, quality gate |
| RBAC Model | Operator service account permissions, per-namespace model deployment |
| Failure Modes | Model download failure, gateway unreachable, Helm upgrade failure, Argo timeout |
| Operator Observability | Operator metrics (reconcile_duration_seconds, queue_depth), Grafana dashboard |
| Security | Webhook TLS, RBAC least-privilege, Secret handling for HF tokens and MinIO credentials |

### 8.2 User Manual (English)

**File:** `docs/user-guide/operator-guide-en.md`

| Chapter | Content |
|---------|---------|
| 1. Quick Start | Prerequisites, install operator, deploy first model, verify |
| 2. Core Concepts | ModelDeployment, LLMPlatform, FineTuneRun — what they are and how they relate |
| 3. Model Management | Deploy, scale, update, canary, delete, engine selection, GPU config (NVIDIA/AMD/Gaudi/MIG) |
| 4. Platform Setup | Minimal setup, enable RAG/finetune/security modules, NodePort/Ingress, SSO |
| 5. Fine-tuning | Create run, monitor progress, quality gate, deploy fine-tuned model |
| 6. Operations | Monitor operator health, troubleshoot, backup/restore, upgrade operator |
| 7. Migration Guide | Helm → Operator migration steps, side-by-side comparison table |
| 8. API Reference | Complete CRD field reference with examples for every field |

### 8.3 User Manual (Chinese)

**File:** `docs/user-guide/operator-guide-zh.md`

Identical structure, natively written in Chinese (not machine-translated):

| 章节 | 内容 |
|------|------|
| 1. 快速开始 | 前提条件、安装 Operator、部署第一个模型、验证 |
| 2. 核心概念 | ModelDeployment、LLMPlatform、FineTuneRun 详解及关系 |
| 3. 模型管理 | 部署、扩缩容、更新、金丝雀部署、删除、引擎选择、GPU 配置 |
| 4. 平台配置 | 最小化部署、启用模块（RAG/微调/安全）、访问配置、单点登录 |
| 5. 模型微调 | 创建微调任务、监控进度、质量门与审批、部署微调模型 |
| 6. 运维指南 | 监控 Operator 状态、常见问题排查、备份恢复、升级 Operator |
| 7. 迁移指南 | 从 Helm 迁移到 Operator、Helm vs Operator 对照表 |
| 8. API 参考 | 完整 CRD 字段参考与示例 |

---

## 9. Scope Summary

### In Scope (v1.0.0)

| Deliverable | Description |
|-------------|-------------|
| Operator binary | Go + Kubebuilder, 3 controllers, single binary |
| 3 CRDs | ModelDeployment, LLMPlatform, FineTuneRun (`v1alpha1`) |
| Validation webhooks | Admission control for all 3 CRDs |
| Operator Helm chart | `charts/kube-llmops-operator/` for installation |
| Architecture doc | `docs/architecture/operator.md` |
| User manual (EN) | `docs/user-guide/operator-guide-en.md` |
| User manual (ZH) | `docs/user-guide/operator-guide-zh.md` |
| Sample CRs | 7 example YAMLs for all CRDs |
| Migration tool | `operator/cmd/migrate/` — Helm release → CRs |
| Tests | ~100 new (unit + envtest + E2E + webhook) |

### Out of Scope (future)

- OLM bundle / OperatorHub listing
- Multi-cluster federation
- CLI tool (`kubectl llmops`) — Phase 6 sub-project 2
- Web Dashboard — Phase 6 sub-project 3
- CRD version upgrade (`v1alpha1` → `v1beta1` → `v1`)

---

## 10. API Group and Versioning

- **API Group:** `llmops.kubellmops.io`
- **Initial Version:** `v1alpha1` (signals: API may change without notice)
- **Graduation path:** `v1alpha1` → `v1beta1` (stable fields, conversion webhooks) → `v1` (GA)
- **Short names:** `md` for ModelDeployment, `lp` for LLMPlatform, `ftr` for FineTuneRun
  - `kubectl get md` = `kubectl get modeldeployments`
