# kube-llmops Operator User Guide

> **Version:** 1.0.0 &nbsp;|&nbsp; **API Group:** `llmops.kubellmops.io/v1alpha1` &nbsp;|&nbsp; **License:** Apache 2.0

---

## Table of Contents

1. [Quick Start](#1-quick-start)
2. [Core Concepts](#2-core-concepts)
3. [Model Management](#3-model-management)
4. [Platform Setup](#4-platform-setup)
5. [Fine-Tuning](#5-fine-tuning)
6. [Operations](#6-operations)
7. [Migration Guide](#7-migration-guide)
8. [API Reference](#8-api-reference)

---

## 1. Quick Start

This chapter walks you through installing the kube-llmops operator, deploying the platform infrastructure, and serving your first model --- all in under five minutes.

### Prerequisites

| Requirement | Minimum Version |
|---|---|
| Kubernetes cluster | v1.24+ |
| kubectl | v1.24+ |
| Helm | v3.12+ |
| GPU driver (NVIDIA, AMD, or Gaudi) | Latest stable |

### Step 1 --- Install the Operator

The operator is distributed as a Helm chart. It installs the CRDs, RBAC, and the operator Deployment in one command:

```bash
helm install kube-llmops-operator charts/kube-llmops-operator/
```

Verify the operator pod is running:

```bash
kubectl get pods -l app.kubernetes.io/name=kube-llmops-operator
```

Expected output:

```
NAME                                       READY   STATUS    RESTARTS   AGE
kube-llmops-operator-7b9f4c6d8a-x2k9p     1/1     Running   0          42s
```

### Step 2 --- Deploy the Platform

Apply the minimal platform manifest. This enables the LiteLLM AI gateway, observability (Prometheus + Grafana), and the MinIO model store:

```bash
kubectl apply -f config/samples/llmplatform_minimal.yaml
```

The manifest contains:

```yaml
apiVersion: llmops.kubellmops.io/v1alpha1
kind: LLMPlatform
metadata:
  name: kube-llmops
spec:
  gateway:
    enabled: true
  observability:
    enabled: true
  modelStore:
    enabled: true
    endpoint: kube-llmops-minio:9000
    bucket: models
    accessKey: minioadmin
    secretKey: minioadmin
    image: kube-llmops/model-loader:latest
  postgresql:
    enabled: true
```

Check the platform status:

```bash
kubectl get lp
```

Expected output:

```
NAME          PHASE    GATEWAY   GRAFANA   AGE
kube-llmops   Ready    Ready     Ready     2m
```

### Step 3 --- Deploy a Model

Deploy a vLLM-based model:

```bash
kubectl apply -f config/samples/modeldeployment_vllm.yaml
```

The manifest contains:

```yaml
apiVersion: llmops.kubellmops.io/v1alpha1
kind: ModelDeployment
metadata:
  name: gemma-4-26b
spec:
  source: cyankiwi/gemma-4-26B-A4B-it-AWQ-4bit
  replicas: 1
  resources:
    gpu: 1
    memory: 24Gi
    cpu: "4"
  engineArgs:
    --gpu-memory-utilization: "0.93"
    --max-model-len: "8192"
    --dtype: "half"
    --enforce-eager: ""
```

### Step 4 --- Check Status

List all model deployments:

```bash
kubectl get md
```

Expected output:

```
NAME          ENGINE   REPLICAS   PHASE       AGE
gemma-4-26b   vllm     1          Deploying   30s
```

After the model downloads and the pod becomes healthy:

```
NAME          ENGINE   REPLICAS   PHASE   AGE
gemma-4-26b   vllm     1          Ready   5m
```

Get detailed status:

```bash
kubectl get md gemma-4-26b -o wide
```

Expected output (with the priority=1 `Endpoint` column):

```
NAME          ENGINE   REPLICAS   PHASE   ENDPOINT                                              AGE
gemma-4-26b   vllm     1          Ready   http://gemma-4-26b.default.svc.cluster.local:8000     5m
```

### Step 5 --- Send a Request

Test the model through the in-cluster endpoint:

```bash
kubectl run curl-test --rm -it --image=curlimages/curl -- \
  curl -s http://gemma-4-26b.default.svc.cluster.local:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemma-4-26b",
    "messages": [{"role": "user", "content": "Hello!"}],
    "max_tokens": 64
  }'
```

### Clean Up

```bash
kubectl delete md gemma-4-26b
kubectl delete lp kube-llmops
helm uninstall kube-llmops-operator
```

---

## 2. Core Concepts

### 2.1 Custom Resource Definitions (CRDs)

The operator introduces three CRDs, each with a convenient short name:

| CRD | Kind | Short Name | Purpose |
|---|---|---|---|
| `modeldeployments.llmops.kubellmops.io` | `ModelDeployment` | **md** | Deploy and serve individual LLM, embedding, or reranker models |
| `llmplatforms.llmops.kubellmops.io` | `LLMPlatform` | **lp** | Manage shared platform infrastructure (gateway, observability, model store) |
| `finetuneruns.llmops.kubellmops.io` | `FineTuneRun` | **ftr** | Execute fine-tuning jobs with evaluation and auto-deployment |

Use the short names with kubectl for convenience:

```bash
kubectl get md          # List ModelDeployments
kubectl get lp          # List LLMPlatforms
kubectl get ftr         # List FineTuneRuns
```

### 2.2 Engine Auto-Detection

When you set `engine: auto` (the default), the operator automatically selects the best inference engine based on the model source name. The resolution logic follows this priority:

1. **Explicit engine** --- If you set `engine` to `vllm`, `tei`, or `llamacpp`, that value is used directly.
2. **GGUF heuristic** --- If the source name contains `gguf` (case-insensitive), the engine resolves to `llamacpp`.
3. **Embedding / Reranker heuristic** --- If the source name matches known embedding patterns (`/bge-`, `/e5-`, `/gte-`, `minilm`, `/jina-embed`, `jina-embeddings`, `/nomic-embed`, `/all-mpnet`, `embedding`) or reranker patterns (`rerank`), the engine resolves to `tei`.
4. **Default fallback** --- Everything else resolves to `vllm`.

Examples:

| Source | Resolved Engine | Reason |
|---|---|---|
| `Qwen/Qwen2.5-7B-Instruct` | `vllm` | Default fallback |
| `BAAI/bge-small-en-v1.5` | `tei` | Matches `/bge-` pattern |
| `BAAI/bge-reranker-base` | `tei` | Matches `rerank` pattern |
| `TheBloke/Llama-2-7B-GGUF` | `llamacpp` | Matches `gguf` pattern |

The resolved engine is written to `status.engine` so you can always verify the decision:

```bash
kubectl get md -o custom-columns=NAME:.metadata.name,ENGINE:.status.engine
```

### 2.3 Reconciliation Loop

Each controller runs a standard Kubernetes reconciliation loop, watching for changes to its primary resource and any owned child resources.

**ModelDeployment Controller:**

```
Watch ModelDeployment CR
  ├── Handle deletion (remove finalizer, deregister from gateway)
  ├── Add finalizer if missing
  ├── Resolve engine (auto-detection)
  ├── Find LLMPlatform in same namespace
  ├── Ensure PVC (create if not exists)
  ├── Ensure Deployment (create or update)
  ├── Ensure Service (create if not exists)
  ├── Update status (phase, readyReplicas, endpoint)
  └── Register model with LiteLLM gateway (when Ready)
```

The ModelDeployment controller also watches owned Deployments, Services, and PVCs. When a child resource changes (e.g., a pod becomes ready), the controller requeues the parent CR.

**LLMPlatform Controller:**

```
Watch LLMPlatform CR
  ├── Translate CR spec to Helm values
  ├── Check if Helm release exists
  │   ├── NO  → Install Helm release
  │   └── YES → Upgrade Helm release
  └── Update status (phase, helmRelease, helmRevision)
```

**FineTuneRun Controller:**

```
Watch FineTuneRun CR
  ├── Skip if terminal (Succeeded / Failed)
  ├── If Argo Workflow exists → sync workflow status
  │   └── Map Argo phase → FineTuneRun phase
  ├── If no workflow → create Argo Workflow (6-step DAG)
  └── Requeue every 30s if non-terminal
```

### 2.4 Status Phases

#### ModelDeployment Phases

| Phase | Description |
|---|---|
| `Pending` | CR accepted, waiting for resources to be created |
| `Downloading` | Model data is being downloaded to the PVC |
| `Deploying` | Deployment created, pods starting but not yet ready |
| `Ready` | All desired replicas are ready; model registered with gateway |
| `Degraded` | Some replicas are ready, but fewer than desired |
| `Failed` | An unrecoverable error occurred |

#### LLMPlatform Phases

| Phase | Description |
|---|---|
| `Pending` | CR accepted, processing not started |
| `Installing` | Helm release is being installed for the first time |
| `Ready` | Helm release installed/upgraded successfully |
| `Upgrading` | Existing Helm release is being upgraded |
| `Degraded` | Helm release exists but some components are unhealthy |
| `Failed` | Helm install or upgrade failed |

#### FineTuneRun Phases

| Phase | Description |
|---|---|
| `Pending` | Argo Workflow has been created, waiting to start |
| `DataPreparing` | Training data is being downloaded and prepared |
| `Training` | Model fine-tuning is in progress (Argo Workflow running) |
| `Evaluating` | Post-training evaluation is running |
| `QualityGate` | Quality gate checks are being evaluated |
| `Deploying` | Fine-tuned model is being deployed |
| `Succeeded` | Run completed successfully |
| `Failed` | Run failed (Argo Workflow errored, quality gate failed, etc.) |

---

## 3. Model Management

### 3.1 Deploying vLLM Models

vLLM is the default engine for LLM text generation models. It provides high-throughput serving with continuous batching, PagedAttention, and OpenAI-compatible APIs.

**Basic deployment:**

```yaml
apiVersion: llmops.kubellmops.io/v1alpha1
kind: ModelDeployment
metadata:
  name: gemma-4-26b
spec:
  source: cyankiwi/gemma-4-26B-A4B-it-AWQ-4bit
  replicas: 1
  resources:
    gpu: 1
    memory: 24Gi
    cpu: "4"
  engineArgs:
    --gpu-memory-utilization: "0.93"
    --max-model-len: "8192"
    --dtype: "half"
    --enforce-eager: ""
```

**Key points:**
- The engine auto-detects as `vllm` because the source name doesn't match GGUF or embedding patterns.
- The operator creates a `/dev/shm` emptyDir volume (8Gi) for vLLM's shared memory requirements.
- The PVC size for vLLM defaults to 50Gi.
- The model is served on port **8000**.
- The model is accessible at `http://<name>.<namespace>.svc.cluster.local:8000`.

**Common vLLM engine arguments:**

| Argument | Example Value | Description |
|---|---|---|
| `--gpu-memory-utilization` | `"0.93"` | Fraction of GPU memory to use |
| `--max-model-len` | `"8192"` | Maximum sequence length |
| `--dtype` | `"half"` | Data type (`auto`, `half`, `float16`, `bfloat16`) |
| `--enforce-eager` | `""` | Disable CUDA graph (value is empty string for flag-only args) |
| `--tensor-parallel-size` | `"2"` | Number of GPUs for tensor parallelism |
| `--quantization` | `"awq"` | Quantization method |
| `--enable-prefix-caching` | N/A | Use `prefixCaching: true` in spec instead |

> **Tip:** For flag-only arguments (no value), set the value to an empty string `""`.

### 3.2 Deploying TEI Embedding Models

Text Embeddings Inference (TEI) is auto-detected for models whose names match embedding patterns (`/bge-`, `/e5-`, `/gte-`, `minilm`, `jina-embed`, etc.).

```yaml
apiVersion: llmops.kubellmops.io/v1alpha1
kind: ModelDeployment
metadata:
  name: bge-small-en
spec:
  source: BAAI/bge-small-en-v1.5
  replicas: 1
  resources:
    gpu: 0
    cpu: "0.5"
    memory: 256Mi
```

**Key points:**
- The engine auto-detects as `tei` because the source matches `/bge-`.
- With `gpu: 0`, the CPU-optimized TEI image (`ghcr.io/huggingface/text-embeddings-inference:cpu-1.6`) is used.
- The model is served on port **8080**.
- The PVC size for TEI defaults to 10Gi.

### 3.3 Deploying TEI Reranker Models

Reranker models are also served by TEI. The operator detects them when the source name contains `rerank`:

```yaml
apiVersion: llmops.kubellmops.io/v1alpha1
kind: ModelDeployment
metadata:
  name: bge-reranker-base
spec:
  source: BAAI/bge-reranker-base
  replicas: 1
  resources:
    gpu: 0
    cpu: "1"
    memory: 1Gi
```

**Key points:**
- Auto-detects as `tei` via the `rerank` pattern.
- Exposes the `/rerank` endpoint on port **8080**.

### 3.4 Deploying llama.cpp GGUF Models

For GGUF-quantized models, the operator uses the llama.cpp server:

```yaml
apiVersion: llmops.kubellmops.io/v1alpha1
kind: ModelDeployment
metadata:
  name: llama-gguf
spec:
  source: TheBloke/Llama-2-7B-GGUF
  engine: llamacpp
  replicas: 1
  resources:
    gpu: 1
    memory: 16Gi
```

**Key points:**
- If the source contains `gguf`, the engine auto-detects as `llamacpp`. You can also set it explicitly.
- Uses the `ghcr.io/ggml-org/llama.cpp:server` image.
- The model is served on port **8080**.
- The PVC size for llama.cpp defaults to 30Gi.

### 3.5 Scaling Replicas

Scale a model by patching the `replicas` field:

```bash
# Scale up to 3 replicas
kubectl patch md gemma-4-26b --type=merge -p '{"spec": {"replicas": 3}}'

# Scale down to 0 (stop serving but keep the PVC)
kubectl patch md gemma-4-26b --type=merge -p '{"spec": {"replicas": 0}}'
```

Verify scaling:

```bash
kubectl get md gemma-4-26b
```

```
NAME          ENGINE   REPLICAS   PHASE      AGE
gemma-4-26b   vllm     2/3        Degraded   10m
```

When all replicas are ready:

```
NAME          ENGINE   REPLICAS   PHASE   AGE
gemma-4-26b   vllm     3          Ready   12m
```

### 3.6 Custom Engine Arguments

Pass arbitrary CLI arguments to any engine through `engineArgs`. These are appended to the engine's default arguments in alphabetical key order:

```yaml
spec:
  engineArgs:
    --gpu-memory-utilization: "0.95"
    --max-model-len: "16384"
    --tensor-parallel-size: "2"
    --enable-chunked-prefill: ""
```

The operator generates arguments as: `--key value`. For boolean flags, use an empty string `""` as the value.

### 3.7 Using NVIDIA MIG Devices

To schedule on a specific MIG (Multi-Instance GPU) partition, use the `migDevice` field:

```yaml
spec:
  source: BAAI/bge-small-en-v1.5
  migDevice: nvidia.com/mig-1g.5gb
  resources:
    gpu: 1
    memory: 8Gi
```

When `migDevice` is set, it overrides the default GPU resource name (`nvidia.com/gpu`). The value is used directly as the Kubernetes resource name in pod requests and limits.

Common MIG profiles:

| MIG Profile | Resource Name |
|---|---|
| 1g.5gb | `nvidia.com/mig-1g.5gb` |
| 2g.10gb | `nvidia.com/mig-2g.10gb` |
| 3g.20gb | `nvidia.com/mig-3g.20gb` |
| 4g.20gb | `nvidia.com/mig-4g.20gb` |
| 7g.40gb | `nvidia.com/mig-7g.40gb` |

### 3.8 AMD and Intel Gaudi GPU Support

The `accelerator` field selects which device plugin resource to request:

| Accelerator | Kubernetes Resource | Use Case |
|---|---|---|
| `nvidia` (default) | `nvidia.com/gpu` | NVIDIA GPUs |
| `amd` | `amd.com/gpu` | AMD Instinct GPUs |
| `gaudi` | `habana.ai/gaudi` | Intel Gaudi accelerators |

**Example --- AMD GPU:**

```yaml
spec:
  source: meta-llama/Llama-3-8B-Instruct
  accelerator: amd
  resources:
    gpu: 1
    memory: 32Gi
```

**Example --- Intel Gaudi:**

```yaml
spec:
  source: meta-llama/Llama-3-8B-Instruct
  accelerator: gaudi
  resources:
    gpu: 1
    memory: 32Gi
```

### 3.9 Prefix Caching (vLLM)

Enable vLLM's automatic prefix caching for workloads with many repeated prompt prefixes:

```yaml
spec:
  source: Qwen/Qwen2.5-7B-Instruct
  prefixCaching: true
```

This adds `--enable-prefix-caching` to the vLLM arguments.

### 3.10 Spot / Preemptible GPU Scheduling

Run models on cheaper spot instances:

```yaml
spec:
  source: Qwen/Qwen2.5-7B-Instruct
  spot:
    enabled: true
    provider: karpenter
```

Supported providers: `aws`, `gcp`, `azure`, `karpenter`. The operator adds the appropriate tolerations for the cloud provider's spot/preemptible taints.

### 3.11 Canary Deployments

Deploy a new model version alongside the current one with traffic splitting:

```yaml
spec:
  source: Qwen/Qwen2.5-7B-Instruct
  canary:
    source: Qwen/Qwen2.5-14B-Instruct
    weight: 20
    resources:
      gpu: 1
      memory: 32Gi
```

This routes 20% of traffic to the canary model and 80% to the primary. Canary status is tracked separately in `status.canary`.

### 3.12 Model Store Override

Override the platform-level MinIO settings for a specific model:

```yaml
spec:
  source: my-org/private-model
  modelStore:
    endpoint: custom-minio:9000
    bucket: private-models
```

---

## 4. Platform Setup

The `LLMPlatform` custom resource manages the shared infrastructure stack through a Helm bridge. The operator translates the CR spec into Helm values and installs or upgrades the underlying `kube-llmops-stack` chart.

### 4.1 Minimal Setup

The minimal setup enables the three essential components: gateway, observability, and model store.

```yaml
# config/samples/llmplatform_minimal.yaml
apiVersion: llmops.kubellmops.io/v1alpha1
kind: LLMPlatform
metadata:
  name: kube-llmops
spec:
  gateway:
    enabled: true
  observability:
    enabled: true
  modelStore:
    enabled: true
    endpoint: kube-llmops-minio:9000
    bucket: models
    accessKey: minioadmin
    secretKey: minioadmin
    image: kube-llmops/model-loader:latest
  postgresql:
    enabled: true
```

```bash
kubectl apply -f config/samples/llmplatform_minimal.yaml
```

Monitor the installation:

```bash
kubectl get lp -w
```

```
NAME          PHASE        GATEWAY   GRAFANA   AGE
kube-llmops   Installing                       5s
kube-llmops   Ready        Ready     Ready     90s
```

### 4.2 Full Setup with All Modules

The full setup enables all platform features: gateway with latency-based routing, observability with Langfuse, logging, RAG, fine-tuning, and NodePort access.

```yaml
# config/samples/llmplatform_full.yaml
apiVersion: llmops.kubellmops.io/v1alpha1
kind: LLMPlatform
metadata:
  name: kube-llmops
spec:
  gateway:
    enabled: true
    routing: latency-based-routing
  observability:
    enabled: true
    grafana:
      adminPassword: admin
    langfuse:
      enabled: true
  logging:
    enabled: true
  modules:
    rag:
      enabled: true
    finetune:
      enabled: true
    security:
      enabled: false
  modelStore:
    enabled: true
    endpoint: kube-llmops-minio:9000
    bucket: models
    accessKey: minioadmin
    secretKey: minioadmin
    hfTransferConcurrency: 32
    image: kube-llmops/model-loader:latest
  postgresql:
    enabled: true
  nodePort:
    enabled: true
    host: "172.29.193.187"
```

```bash
kubectl apply -f config/samples/llmplatform_full.yaml
```

### 4.3 Gateway Configuration

The LiteLLM AI gateway provides a unified OpenAI-compatible API and routes requests to your models.

**Routing strategies:**

| Strategy | Description |
|---|---|
| `simple-shuffle` | Random load balancing |
| `least-busy` | Route to the replica with fewest in-flight requests |
| `latency-based-routing` | Route to the lowest-latency backend |
| `usage-based-routing` | Route based on usage/budget constraints |

```yaml
spec:
  gateway:
    enabled: true
    routing: latency-based-routing
    rateLimiting:
      enabled: true
    budgetControl:
      enabled: true
    image:
      repository: ghcr.io/berriai/litellm
      tag: main-v1.67.0
```

When a ModelDeployment reaches the `Ready` phase, the operator automatically registers it with the LiteLLM gateway. When a ModelDeployment is deleted, the operator deregisters it.

### 4.4 NodePort Access Configuration

Expose platform services on node ports for environments without a load balancer:

```yaml
spec:
  nodePort:
    enabled: true
    host: "192.168.1.100"
```

The `host` field specifies the node IP used to construct external access URLs.

### 4.5 Ingress Configuration

For clusters with an ingress controller:

```yaml
spec:
  ingress:
    enabled: true
    className: nginx
    host: llm.example.com
```

This configures Ingress resources for the gateway, Grafana, and other platform endpoints under the specified host.

### 4.6 Enabling and Disabling Modules

Toggle feature modules independently:

```yaml
spec:
  modules:
    rag:
      enabled: true        # Deploys Dify + Milvus for RAG workflows
    finetune:
      enabled: true        # Deploys Argo Workflows + MLflow for fine-tuning
    security:
      enabled: false       # Deploys Keycloak + LLM Guard for security
```

**Module components:**

| Module | Components Deployed |
|---|---|
| `rag` | Dify, Milvus (vector database) |
| `finetune` | Argo Workflows, MLflow, LLaMA-Factory |
| `security` | Keycloak (SSO), LLM Guard |

### 4.7 Observability Configuration

```yaml
spec:
  observability:
    enabled: true
    grafana:
      adminPassword: my-secure-password
      oidc:
        enabled: true       # Enable Keycloak SSO for Grafana
    langfuse:
      enabled: true         # Deploy Langfuse for LLM tracing
```

### 4.8 Logging Configuration

```yaml
spec:
  logging:
    enabled: true           # Deploys Fluent Bit + Loki
```

### 4.9 Updating the Platform

To change any platform setting, simply edit the CR and re-apply. The operator detects that the Helm release already exists and performs an upgrade:

```bash
# Enable the finetune module on an existing platform
kubectl patch lp kube-llmops --type=merge -p '{"spec": {"modules": {"finetune": {"enabled": true}}}}'
```

Monitor the upgrade:

```bash
kubectl get lp -w
```

```
NAME          PHASE       GATEWAY   GRAFANA   AGE
kube-llmops   Upgrading   Ready     Ready     1d
kube-llmops   Ready       Ready     Ready     1d
```

The Helm revision is tracked in the status:

```bash
kubectl get lp kube-llmops -o jsonpath='{.status.helmRevision}'
```

---

## 5. Fine-Tuning

The `FineTuneRun` CRD manages end-to-end fine-tuning pipelines. Each run creates a six-step Argo Workflow DAG:

```
prepare-data → finetune → merge-upload → evaluate → quality-gate → deploy
```

### 5.1 Prerequisites

Fine-tuning requires the `finetune` module to be enabled on your LLMPlatform:

```yaml
# In your LLMPlatform CR
spec:
  modules:
    finetune:
      enabled: true
```

This deploys:
- **Argo Workflows** --- DAG orchestration engine
- **MLflow** --- Experiment tracking and metric logging
- **LLaMA-Factory** --- Training framework

### 5.2 Creating a LoRA Fine-Tune Run

```yaml
# config/samples/finetunerun_lora.yaml
apiVersion: llmops.kubellmops.io/v1alpha1
kind: FineTuneRun
metadata:
  name: gemma-lora-v1
spec:
  baseModel: cyankiwi/gemma-4-26B-A4B-it-AWQ-4bit
  outputName: gemma-4-lora-v1
  method: lora
  dataSource:
    type: minio
    path: "s3://datasets/my-data/"
    format: alpaca
  training:
    epochs: 3
    batchSize: 4
    learningRate: "2e-4"
    loraRank: 16
    loraAlpha: 32
    loraTarget: "all"
  resources:
    gpu: 1
    memory: 24Gi
    cpu: "4"
  evaluation:
    enabled: true
  qualityGate:
    enabled: true
    thresholds:
      minEvalLoss: "0.8"
      maxTrainLoss: "0.5"
  deploy:
    enabled: false
    canaryWeight: 20
```

```bash
kubectl apply -f config/samples/finetunerun_lora.yaml
```

Monitor progress:

```bash
kubectl get ftr
```

```
NAME            BASE MODEL                                METHOD   PHASE      AGE
gemma-lora-v1   cyankiwi/gemma-4-26B-A4B-it-AWQ-4bit     lora     Training   5m
```

Get detailed info with priority columns:

```bash
kubectl get ftr -o wide
```

```
NAME            BASE MODEL                                METHOD   PHASE       LOSS    DURATION   AGE
gemma-lora-v1   cyankiwi/gemma-4-26B-A4B-it-AWQ-4bit     lora     Succeeded   0.42    2h15m      3h
```

### 5.3 Fine-Tuning Methods

#### LoRA (Low-Rank Adaptation)

The most memory-efficient method. Trains small adapter layers without modifying the base model:

```yaml
spec:
  method: lora
  training:
    loraRank: 16        # Rank of the low-rank matrices (higher = more capacity)
    loraAlpha: 32       # Scaling factor (typically 2x loraRank)
    loraTarget: "all"   # Target modules (e.g., "all", "q_proj,v_proj")
```

#### QLoRA (Quantized LoRA)

Combines 4-bit quantization with LoRA for even lower memory usage:

```yaml
spec:
  method: qlora
  training:
    loraRank: 16
    loraAlpha: 32
    loraTarget: "all"
```

#### Full Fine-Tuning

Updates all model weights. Requires significantly more GPU memory:

```yaml
spec:
  method: full
  resources:
    gpu: 4
    memory: 96Gi
    cpu: "16"
```

### 5.4 Data Sources

#### MinIO

Load training data from the platform's MinIO instance:

```yaml
spec:
  dataSource:
    type: minio
    path: "s3://datasets/my-data/"
    format: alpaca
```

#### HuggingFace

Load a public dataset from HuggingFace Hub:

```yaml
spec:
  dataSource:
    type: huggingface
    path: "tatsu-lab/alpaca"
    format: alpaca
```

#### PVC (Persistent Volume Claim)

Load data from an existing PVC:

```yaml
spec:
  dataSource:
    type: pvc
    path: "my-training-data-pvc"
    format: sharegpt
```

**Supported data formats:**

| Format | Description |
|---|---|
| `alpaca` | Instruction-following format (`instruction`, `input`, `output`) |
| `sharegpt` | Multi-turn conversation format (`conversations` array) |
| `custom` | User-defined format with custom preprocessing |

### 5.5 Quality Gates and Evaluation

Enable post-training evaluation and quality gates to automatically validate model quality:

```yaml
spec:
  evaluation:
    enabled: true
    dataset: "eval-data"    # Optional: specific evaluation dataset
  qualityGate:
    enabled: true
    thresholds:
      minEvalLoss: "0.8"    # Maximum acceptable evaluation loss
      maxTrainLoss: "0.5"   # Maximum acceptable training loss
```

> **Note:** `evaluation.enabled` must be `true` when `qualityGate.enabled` is `true`. The webhook rejects CRs that violate this constraint.

The quality gate results are available in the status:

```bash
kubectl get ftr gemma-lora-v1 -o jsonpath='{.status.qualityGate}'
```

```json
{"passed": true, "message": "All thresholds met"}
```

### 5.6 Auto-Deployment After Fine-Tuning

Automatically deploy the fine-tuned model when the quality gate passes:

```yaml
spec:
  deploy:
    enabled: true
    canaryWeight: 20    # Route 20% of traffic to the new model
```

When `deploy.enabled` is `true` and the quality gate passes, the `deploy` step in the Argo Workflow creates a new ModelDeployment CR (or updates a canary config on an existing one).

The output model information is tracked in the status:

```bash
kubectl get ftr gemma-lora-v1 -o jsonpath='{.status.outputModel}'
```

```json
{"source": "gemma-4-lora-v1", "modelDeployment": "gemma-4-lora-v1"}
```

### 5.7 MLflow Integration

Each FineTuneRun automatically logs metrics to MLflow. The tracking info is available in the status:

```bash
kubectl get ftr gemma-lora-v1 -o jsonpath='{.status.mlflow}'
```

```json
{"runId": "abc123", "experimentName": "gemma-4-lora-v1", "artifactUri": "s3://mlflow/..."}
```

### 5.8 Viewing the Argo Workflow

The name of the generated Argo Workflow is stored in `status.argoWorkflow`:

```bash
# Get the workflow name
kubectl get ftr gemma-lora-v1 -o jsonpath='{.status.argoWorkflow}'

# View the workflow
argo get $(kubectl get ftr gemma-lora-v1 -o jsonpath='{.status.argoWorkflow}')
```

---

## 6. Operations

### 6.1 Monitoring with Printer Columns

The CRDs include printer columns for at-a-glance monitoring:

**ModelDeployment columns:**

```bash
kubectl get md
```

```
NAME                ENGINE    REPLICAS   PHASE       AGE
gemma-4-26b         vllm      1          Ready       2h
bge-small-en        tei       1          Ready       1h
bge-reranker-base   tei       1          Ready       1h
llama-gguf          llamacpp  1          Deploying   5m
```

**LLMPlatform columns:**

```bash
kubectl get lp
```

```
NAME          PHASE   GATEWAY   GRAFANA   AGE
kube-llmops   Ready   Ready     Ready     3d
```

**FineTuneRun columns:**

```bash
kubectl get ftr
```

```
NAME            BASE MODEL                                METHOD   PHASE       AGE
gemma-lora-v1   cyankiwi/gemma-4-26B-A4B-it-AWQ-4bit     lora     Succeeded   5h
```

### 6.2 Status Conditions

All three CRDs use standard Kubernetes conditions for detailed status reporting.

**ModelDeployment conditions:**

```bash
kubectl get md gemma-4-26b -o jsonpath='{.status.conditions}' | jq .
```

```json
[
  {
    "type": "Ready",
    "status": "True",
    "reason": "AllReplicasReady",
    "message": "All replicas are ready",
    "lastTransitionTime": "2026-01-15T10:30:00Z",
    "observedGeneration": 2
  }
]
```

| Condition Type | Status | Reason | Meaning |
|---|---|---|---|
| `Ready` | `True` | `AllReplicasReady` | All desired replicas are running |
| `Ready` | `False` | `ReplicasNotReady` | Some replicas not yet ready (`N/M replicas ready`) |

**LLMPlatform conditions:**

| Condition Type | Status | Reason | Meaning |
|---|---|---|---|
| `HelmRelease` | `True` | `InstallSucceeded` | Initial Helm install succeeded |
| `HelmRelease` | `True` | `UpgradeSucceeded` | Helm upgrade succeeded |
| `HelmRelease` | `False` | `Installing` | Helm install in progress |
| `HelmRelease` | `False` | `Upgrading` | Helm upgrade in progress |
| `HelmRelease` | `False` | `InstallFailed` | Helm install failed (check message) |
| `HelmRelease` | `False` | `UpgradeFailed` | Helm upgrade failed (check message) |

**FineTuneRun conditions:**

| Condition Type | Status | Reason | Meaning |
|---|---|---|---|
| `WorkflowReady` | `True` | `ArgoPhaseSucceeded` | Argo Workflow completed successfully |
| `WorkflowReady` | `False` | `WorkflowCreated` | Argo Workflow has been created |
| `WorkflowReady` | `False` | `ArgoPhaseRunning` | Argo Workflow is running |
| `WorkflowReady` | `False` | `ArgoPhaseError` | Argo Workflow encountered an error |
| `WorkflowReady` | `False` | `ArgoCRDMissing` | Argo Workflows CRD is not installed |
| `WorkflowReady` | `False` | `WorkflowNotFound` | Argo Workflow was deleted externally |

### 6.3 Troubleshooting Common Issues

#### Model stuck in "Deploying" phase

**Symptoms:** `kubectl get md` shows `Phase: Deploying` for an extended time.

**Diagnosis:**

```bash
# Check the deployment
kubectl describe deployment gemma-4-26b

# Check the pods
kubectl get pods -l kube-llmops/model=gemma-4-26b

# Check init container logs (model download)
kubectl logs <pod-name> -c model-loader

# Check main container logs (engine startup)
kubectl logs <pod-name> -c model-server
```

**Common causes:**
- **Insufficient GPU memory:** The model is too large for the allocated GPU. Increase `resources.gpu` or use a quantized model.
- **Model download failure:** Check MinIO connectivity, credentials, or HuggingFace access. Check the `model-loader` init container logs.
- **GPU scheduling failure:** No nodes have available GPUs. Check `kubectl describe pod` for scheduling events.
- **OOM Kill:** Check `kubectl describe pod` for OOMKilled status. Increase `resources.memory`.

#### LLMPlatform stuck in "Installing" or "Failed"

**Diagnosis:**

```bash
# Check conditions
kubectl get lp kube-llmops -o jsonpath='{.status.conditions}' | jq .

# Check the operator logs
kubectl logs -l app.kubernetes.io/name=kube-llmops-operator

# Check Helm release status
helm list -A
helm status kube-llmops
```

**Common causes:**
- **Missing model store configuration:** `modelStore.endpoint` and `modelStore.bucket` are required when model store is enabled.
- **Chart not found:** Verify `chartPath` in the operator configuration.
- **PVC provisioning failure:** Ensure a storage class is configured.

#### FineTuneRun immediately "Failed" with ArgoCRDMissing

**Cause:** Argo Workflows CRD is not installed in the cluster.

**Fix:** Enable the finetune module in your LLMPlatform:

```bash
kubectl patch lp kube-llmops --type=merge -p '{"spec": {"modules": {"finetune": {"enabled": true}}}}'
```

Or install Argo Workflows manually:

```bash
kubectl apply -n argo -f https://github.com/argoproj/argo-workflows/releases/latest/download/install.yaml
```

#### Gateway not registering models

**Symptoms:** Model is `Ready` but not accessible through the LiteLLM gateway.

**Diagnosis:**

```bash
# Check operator logs for gateway errors
kubectl logs -l app.kubernetes.io/name=kube-llmops-operator | grep gateway

# Check gateway health
kubectl exec -it deploy/kube-llmops-litellm -- curl localhost:4000/health
```

**Common causes:**
- Gateway pod is not ready.
- Gateway master key mismatch.

### 6.4 Useful kubectl Commands

```bash
# Watch all resources in real-time
kubectl get md,lp,ftr -w

# Get YAML for a specific model deployment
kubectl get md gemma-4-26b -o yaml

# Describe (includes events)
kubectl describe md gemma-4-26b

# List all operator-managed resources
kubectl get deployments,services,pvc -l app.kubernetes.io/part-of=kube-llmops

# Check model endpoints
kubectl get md -o custom-columns=NAME:.metadata.name,ENGINE:.status.engine,ENDPOINT:.status.endpoint

# Get all model endpoints as JSON
kubectl get md -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.endpoint}{"\n"}{end}'
```

### 6.5 Upgrading the Operator

To upgrade the operator to a new version:

```bash
# Update CRDs first
kubectl apply -f charts/kube-llmops-operator/crds/

# Upgrade the operator Helm release
helm upgrade kube-llmops-operator charts/kube-llmops-operator/
```

Existing CRs will be re-reconciled by the new operator version. The upgrade is non-disruptive --- running models continue to serve traffic.

### 6.6 Backup and Restore

**Backing up CRs:**

```bash
# Export all ModelDeployments
kubectl get md -o yaml > backup-modeldeployments.yaml

# Export all LLMPlatforms
kubectl get lp -o yaml > backup-llmplatforms.yaml

# Export all FineTuneRuns
kubectl get ftr -o yaml > backup-finetuneruns.yaml

# Export everything
kubectl get md,lp,ftr -o yaml > backup-all.yaml
```

**Restoring CRs:**

Before restoring, remove status and metadata fields that are server-managed:

```bash
# Clean up and re-apply (remove resourceVersion, uid, status, etc.)
kubectl apply -f backup-modeldeployments.yaml
```

> **Tip:** Model data is stored in PVCs. As long as PVCs are preserved, models will not need to be re-downloaded after restoring CRs.

### 6.7 Uninstalling

```bash
# 1. Delete all CRs (this cleans up child resources)
kubectl delete md --all
kubectl delete ftr --all
kubectl delete lp --all

# 2. Uninstall the operator
helm uninstall kube-llmops-operator

# 3. (Optional) Remove CRDs
kubectl delete -f charts/kube-llmops-operator/crds/
```

---

## 7. Migration Guide

If you are running the kube-llmops Helm chart directly (without the operator), this guide helps you migrate to the operator-managed approach.

### 7.1 Why Migrate?

| Helm-only approach | Operator-managed approach |
|---|---|
| One monolithic `values.yaml` | Declarative CRs per model |
| Manual `helm upgrade` for changes | Automatic reconciliation |
| No status visibility | Rich status with phases, conditions, endpoints |
| No auto-registration with gateway | Automatic gateway registration/deregistration |
| No fine-tuning orchestration | Built-in FineTuneRun pipeline |

### 7.2 Using the Migration Tool

The operator ships with a migration tool that reads your existing Helm release values and generates the equivalent CRs:

```bash
cd operator/
go run ./cmd/migrate/ kube-llmops default
```

**Arguments:**

| Positional Arg | Description | Default |
|---|---|---|
| 1st | Helm release name (e.g., `kube-llmops`) | Required |
| 2nd | Namespace | `default` |

**What the tool does:**

1. Runs `helm get values <release> -n <namespace> -o json` to extract current values.
2. Generates a `generated/llmplatform.yaml` from the platform-level configuration (litellm, observability, modules, modelStore, nodePort).
3. Generates a `generated/modeldeployment_<name>.yaml` for each model in `global.models`.
4. Prints instructions for cutover.

**Example output:**

```
Generated: generated/llmplatform.yaml
Generated: generated/modeldeployment_gemma_4_26b.yaml
Generated: generated/modeldeployment_bge_small_en.yaml

Review the generated CRs, then:
  helm uninstall kube-llmops -n default
  kubectl apply -f generated/
```

### 7.3 Review Generated CRs

Always review the generated CRs before applying:

```bash
# Review the platform CR
cat generated/llmplatform.yaml

# Review model CRs
cat generated/modeldeployment_*.yaml
```

Verify that:
- Model sources are correct.
- Resource requests (GPU, memory, CPU) are appropriate.
- Engine args are preserved.
- Module toggles match your current configuration.

### 7.4 Cutover Procedure

> **Warning:** This procedure involves a brief period where models are being redeployed. Plan for downtime or execute during a maintenance window.

**Step 1 --- Install the operator** (does not affect the running Helm release):

```bash
helm install kube-llmops-operator charts/kube-llmops-operator/
```

**Step 2 --- Uninstall the old Helm release:**

```bash
helm uninstall kube-llmops -n default
```

> **Tip:** If your PVCs use `Retain` reclaim policy, model data is preserved and won't need re-downloading.

**Step 3 --- Apply the generated CRs:**

```bash
kubectl apply -f generated/
```

**Step 4 --- Verify:**

```bash
kubectl get lp
kubectl get md
```

All models should transition from `Deploying` to `Ready` as pods come up.

### 7.5 Rollback

If the migration fails, you can rollback:

```bash
# Delete operator CRs
kubectl delete -f generated/

# Re-install original Helm chart
helm install kube-llmops charts/kube-llmops-stack/ -f your-original-values.yaml

# (Optional) Uninstall operator
helm uninstall kube-llmops-operator
```

---

## 8. API Reference

### 8.1 ModelDeployment

**API Version:** `llmops.kubellmops.io/v1alpha1`  
**Kind:** `ModelDeployment`  
**Short Name:** `md`

#### Spec Fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `source` | `string` | **Yes** | | HuggingFace model ID or path (e.g., `Qwen/Qwen2.5-7B-Instruct`). Min length: 1. |
| `engine` | `string` | No | `auto` | Inference engine. Enum: `auto`, `vllm`, `tei`, `llamacpp`. |
| `replicas` | `int32` | No | `1` | Desired number of model serving pods. Min: 0. |
| `resources` | `ModelResources` | No | | Compute resources for the model serving container. |
| `resources.gpu` | `int32` | No | `1` | Number of GPUs to request (0 for CPU-only). Min: 0. |
| `resources.memory` | `string` | No | `16Gi` | Memory limit (e.g., `16Gi`, `32Gi`). |
| `resources.cpu` | `string` | No | `4` | CPU limit (e.g., `4`, `500m`). |
| `accelerator` | `string` | No | `nvidia` | GPU vendor for scheduling. Enum: `nvidia`, `amd`, `gaudi`. |
| `migDevice` | `string` | No | | NVIDIA MIG device resource name (e.g., `nvidia.com/mig-1g.5gb`). Overrides GPU resource. |
| `engineArgs` | `map[string]string` | No | | Extra CLI arguments passed to the inference engine. Keys are flag names, values are flag values. |
| `prefixCaching` | `bool` | No | `false` | Enable vLLM automatic prefix caching. |
| `spot` | `SpotConfig` | No | | Spot/preemptible GPU scheduling configuration. |
| `spot.enabled` | `bool` | No | `false` | Enable spot instance scheduling. |
| `spot.provider` | `string` | No | | Cloud provider. Enum: `aws`, `gcp`, `azure`, `karpenter`. |
| `canary` | `CanaryConfig` | No | | Canary deployment configuration. |
| `canary.source` | `string` | **Yes** (if canary) | | HuggingFace model ID for the canary model. |
| `canary.weight` | `int32` | **Yes** (if canary) | | Traffic percentage to route to canary (0-100). |
| `canary.resources` | `ModelResources` | No | | Resources for the canary deployment. |
| `modelStore` | `ModelStoreOverride` | No | | Per-model override of platform model store settings. |
| `modelStore.endpoint` | `string` | No | | MinIO endpoint override. |
| `modelStore.bucket` | `string` | No | | MinIO bucket override. |

#### Status Fields

| Field | Type | Description |
|---|---|---|
| `phase` | `string` | High-level lifecycle phase. Enum: `Pending`, `Downloading`, `Deploying`, `Ready`, `Degraded`, `Failed`. |
| `engine` | `string` | Resolved engine after auto-detection (e.g., `vllm`, `tei`, `llamacpp`). |
| `endpoint` | `string` | In-cluster URL for the model API (e.g., `http://gemma-4-26b.default.svc.cluster.local:8000`). |
| `readyReplicas` | `int32` | Number of ready model serving pods. |
| `totalReplicas` | `int32` | Desired number of replicas. |
| `modelSize` | `string` | Discovered model size after download. |
| `conditions` | `[]Condition` | Standard Kubernetes conditions (`Ready`). |
| `canary` | `CanaryStatus` | Status of the canary deployment. |
| `canary.phase` | `string` | Canary deployment phase. |
| `canary.endpoint` | `string` | In-cluster URL for the canary model API. |
| `canary.readyReplicas` | `int32` | Number of ready canary pods. |

#### Printer Columns

| Column | JSONPath | Priority |
|---|---|---|
| Engine | `.status.engine` | 0 (always shown) |
| Replicas | `.status.readyReplicas` | 0 (always shown) |
| Phase | `.status.phase` | 0 (always shown) |
| Endpoint | `.status.endpoint` | 1 (shown with `-o wide`) |
| Age | `.metadata.creationTimestamp` | 0 (always shown) |

#### Validation Rules

- `source` is required and must be non-empty.
- `engine` must be one of: `auto`, `vllm`, `tei`, `llamacpp` (or empty, which defaults to `auto`).
- `replicas` must be >= 0.
- `resources.gpu` must be >= 0.
- `accelerator` must be one of: `nvidia`, `amd`, `gaudi` (or empty, which defaults to `nvidia`).
- If `canary` is specified, `canary.source` is required and `canary.weight` must be 0-100.

#### Child Resources Created

| Resource | Name Pattern | Condition |
|---|---|---|
| `PersistentVolumeClaim` | `<name>-cache` | Always |
| `Deployment` | `<name>` | Always |
| `Service` | `<name>` (ClusterIP) | Always |

---

### 8.2 LLMPlatform

**API Version:** `llmops.kubellmops.io/v1alpha1`  
**Kind:** `LLMPlatform`  
**Short Names:** `lp`, `llmplatforms`

#### Spec Fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `gateway` | `GatewaySpec` | No | | LiteLLM AI gateway configuration. |
| `gateway.enabled` | `bool` | No | `false` | Enable the LiteLLM gateway. |
| `gateway.routing` | `string` | No | | Routing strategy. Enum: `simple-shuffle`, `least-busy`, `latency-based-routing`, `usage-based-routing`. |
| `gateway.image.repository` | `string` | No | | Override gateway container image repository. |
| `gateway.image.tag` | `string` | No | | Override gateway container image tag. |
| `gateway.rateLimiting.enabled` | `bool` | No | `false` | Enable rate limiting. |
| `gateway.budgetControl.enabled` | `bool` | No | `false` | Enable budget control. |
| `observability` | `ObservabilitySpec` | No | | Monitoring and tracing configuration. |
| `observability.enabled` | `bool` | No | `false` | Enable Prometheus + Grafana. |
| `observability.grafana.adminPassword` | `string` | No | | Grafana admin password. |
| `observability.grafana.oidc.enabled` | `bool` | No | `false` | Enable Keycloak SSO for Grafana. |
| `observability.langfuse.enabled` | `bool` | No | `false` | Enable Langfuse LLM tracing. |
| `logging` | `LoggingSpec` | No | | Fluent Bit + Loki logging configuration. |
| `logging.enabled` | `bool` | No | `false` | Enable log aggregation. |
| `modules` | `ModulesSpec` | No | | Feature module toggles. |
| `modules.rag.enabled` | `bool` | No | `false` | Enable RAG module (Dify + Milvus). |
| `modules.finetune.enabled` | `bool` | No | `false` | Enable fine-tuning module (Argo + MLflow). |
| `modules.security.enabled` | `bool` | No | `false` | Enable security module (Keycloak + LLM Guard). |
| `modelStore` | `ModelStoreSpec` | No | | MinIO model cache configuration. |
| `modelStore.enabled` | `bool` | No | `false` | Enable the MinIO model store. |
| `modelStore.endpoint` | `string` | Conditional | | MinIO endpoint (required when enabled). |
| `modelStore.bucket` | `string` | Conditional | | MinIO bucket name (required when enabled). |
| `modelStore.accessKey` | `string` | No | | MinIO access key. |
| `modelStore.secretKey` | `string` | No | | MinIO secret key. |
| `modelStore.hfTransferConcurrency` | `int32` | No | `4` | HuggingFace download concurrency. |
| `modelStore.image` | `string` | No | | Model loader init container image. |
| `hfToken` | `string` | No | | HuggingFace API token for gated model access. |
| `keycloak` | `KeycloakSpec` | No | | SSO configuration. |
| `keycloak.enabled` | `bool` | No | `false` | Enable Keycloak. |
| `postgresql` | `PostgreSQLSpec` | No | | Shared database configuration. |
| `postgresql.enabled` | `bool` | No | `false` | Enable PostgreSQL. |
| `keda` | `KEDASpec` | No | | Autoscaling configuration. |
| `keda.enabled` | `bool` | No | `false` | Enable KEDA autoscaler. |
| `nodePort` | `NodePortSpec` | No | | NodePort access configuration. |
| `nodePort.enabled` | `bool` | No | `false` | Enable NodePort access. |
| `nodePort.host` | `string` | No | | Node IP for constructing external URLs. |
| `ingress` | `IngressSpec` | No | | Ingress access configuration. |
| `ingress.enabled` | `bool` | No | `false` | Enable Ingress resources. |
| `ingress.className` | `string` | No | | Ingress class name (e.g., `nginx`). |
| `ingress.host` | `string` | No | | Ingress host (e.g., `llm.example.com`). |

#### Status Fields

| Field | Type | Description |
|---|---|---|
| `phase` | `string` | High-level lifecycle phase. Enum: `Pending`, `Installing`, `Ready`, `Upgrading`, `Degraded`, `Failed`. |
| `helmRelease` | `string` | Name of the managed Helm release. |
| `helmRevision` | `int` | Helm release revision number. |
| `components` | `ComponentStatuses` | Per-component health status. |
| `components.gateway` | `ComponentStatus` | Gateway component status (phase, endpoint, nodePort). |
| `components.grafana` | `ComponentStatus` | Grafana component status. |
| `components.prometheus` | `ComponentStatus` | Prometheus component status. |
| `components.langfuse` | `ComponentStatus` | Langfuse component status. |
| `components.minio` | `ComponentStatus` | MinIO component status. |
| `components.postgresql` | `ComponentStatus` | PostgreSQL component status. |
| `components.dify` | `ComponentStatus` | Dify component status. |
| `components.milvus` | `ComponentStatus` | Milvus component status. |
| `conditions` | `[]Condition` | Standard Kubernetes conditions (`HelmRelease`). |

#### Printer Columns

| Column | JSONPath | Priority |
|---|---|---|
| Phase | `.status.phase` | 0 (always shown) |
| Gateway | `.status.components.gateway.phase` | 0 (always shown) |
| Grafana | `.status.components.grafana.phase` | 0 (always shown) |
| Age | `.metadata.creationTimestamp` | 0 (always shown) |

#### Validation Rules

- When `modelStore.enabled` is `true`, `modelStore.endpoint` and `modelStore.bucket` are required.
- `gateway.routing` must be one of: `simple-shuffle`, `least-busy`, `latency-based-routing`, `usage-based-routing` (or empty).

---

### 8.3 FineTuneRun

**API Version:** `llmops.kubellmops.io/v1alpha1`  
**Kind:** `FineTuneRun`  
**Short Name:** `ftr`

#### Spec Fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `baseModel` | `string` | **Yes** | | HuggingFace model ID to fine-tune. Min length: 1. |
| `outputName` | `string` | **Yes** | | Name for the fine-tuned model artifact. Min length: 1. |
| `method` | `string` | No | `lora` | Fine-tuning method. Enum: `lora`, `qlora`, `full`. |
| `dataSource` | `DataSourceSpec` | **Yes** | | Training data source configuration. |
| `dataSource.type` | `string` | **Yes** | | Data source type. Enum: `minio`, `huggingface`, `pvc`. |
| `dataSource.path` | `string` | Conditional | | Data location. Required when `type` is `minio`. |
| `dataSource.format` | `string` | No | `alpaca` | Data format. Enum: `alpaca`, `sharegpt`, `custom`. |
| `training` | `TrainingSpec` | No | | Training hyperparameters. |
| `training.epochs` | `int32` | No | `0` | Number of training epochs. Must be >= 0. |
| `training.batchSize` | `int32` | No | `0` | Batch size per device. Must be >= 0. |
| `training.learningRate` | `string` | No | | Learning rate (e.g., `"2e-4"`). |
| `training.gradientAccumulationSteps` | `int32` | No | | Number of gradient accumulation steps. |
| `training.warmupRatio` | `string` | No | | Warmup ratio (e.g., `"0.1"`). |
| `training.loraRank` | `int32` | No | | LoRA rank (e.g., `16`). |
| `training.loraAlpha` | `int32` | No | | LoRA alpha scaling factor (e.g., `32`). |
| `training.loraTarget` | `string` | No | | LoRA target modules (e.g., `"all"`, `"q_proj,v_proj"`). |
| `resources` | `ModelResources` | No | | Compute resources for the training container. |
| `resources.gpu` | `int32` | No | `1` | Number of GPUs. |
| `resources.memory` | `string` | No | `16Gi` | Memory limit. |
| `resources.cpu` | `string` | No | `4` | CPU limit. |
| `evaluation` | `EvaluationSpec` | No | | Post-training evaluation configuration. |
| `evaluation.enabled` | `bool` | No | `false` | Enable post-training evaluation. |
| `evaluation.dataset` | `string` | No | | Specific evaluation dataset name. |
| `qualityGate` | `QualityGateSpec` | No | | Quality gate configuration. |
| `qualityGate.enabled` | `bool` | No | `false` | Enable quality gate. Requires `evaluation.enabled: true`. |
| `qualityGate.thresholds.minEvalLoss` | `string` | No | | Maximum acceptable evaluation loss (e.g., `"0.8"`). |
| `qualityGate.thresholds.maxTrainLoss` | `string` | No | | Maximum acceptable training loss (e.g., `"0.5"`). |
| `deploy` | `DeploySpec` | No | | Auto-deployment configuration. |
| `deploy.enabled` | `bool` | No | `false` | Enable auto-deployment after quality gate passes. |
| `deploy.canaryWeight` | `int32` | No | `0` | Canary traffic weight for the new model (0-100). |

#### Status Fields

| Field | Type | Description |
|---|---|---|
| `phase` | `string` | High-level lifecycle phase. Enum: `Pending`, `DataPreparing`, `Training`, `Evaluating`, `QualityGate`, `Deploying`, `Succeeded`, `Failed`. |
| `argoWorkflow` | `string` | Name of the created Argo Workflow. |
| `startTime` | `Time` | When the run started. |
| `completionTime` | `Time` | When the run completed. |
| `metrics` | `TrainingMetrics` | Observed training metrics. |
| `metrics.trainLoss` | `string` | Final training loss. |
| `metrics.evalLoss` | `string` | Final evaluation loss. |
| `metrics.trainingDuration` | `string` | Total training duration. |
| `mlflow` | `MLflowStatus` | MLflow tracking information. |
| `mlflow.runId` | `string` | MLflow run ID. |
| `mlflow.experimentName` | `string` | MLflow experiment name. |
| `mlflow.artifactUri` | `string` | MLflow artifact URI. |
| `qualityGate` | `QualityGateStatus` | Quality gate evaluation results. |
| `qualityGate.passed` | `bool` | Whether the quality gate passed. |
| `qualityGate.message` | `string` | Quality gate result message. |
| `outputModel` | `OutputModelStatus` | Produced model artifact information. |
| `outputModel.source` | `string` | Source path/ID of the fine-tuned model. |
| `outputModel.modelDeployment` | `string` | Name of the auto-created ModelDeployment. |
| `conditions` | `[]Condition` | Standard Kubernetes conditions (`WorkflowReady`). |

#### Printer Columns

| Column | JSONPath | Priority |
|---|---|---|
| Base Model | `.spec.baseModel` | 0 (always shown) |
| Method | `.spec.method` | 0 (always shown) |
| Phase | `.status.phase` | 0 (always shown) |
| Loss | `.status.metrics.trainLoss` | 1 (shown with `-o wide`) |
| Duration | `.status.metrics.trainingDuration` | 1 (shown with `-o wide`) |
| Age | `.metadata.creationTimestamp` | 0 (always shown) |

#### Validation Rules

- `baseModel` is required and must be non-empty.
- `outputName` is required and must be non-empty.
- `method` must be one of: `lora`, `qlora`, `full` (or empty, which defaults to `lora`).
- `dataSource.type` must be one of: `minio`, `huggingface`, `pvc`.
- `dataSource.path` is required when `dataSource.type` is `minio`.
- `training.epochs` must be >= 0.
- `training.batchSize` must be >= 0.
- `qualityGate.enabled: true` requires `evaluation.enabled: true`.

#### Argo Workflow DAG

The operator creates a six-step DAG workflow:

| Step | Image | Description |
|---|---|---|
| `prepare-data` | `kube-llmops/model-loader:latest` | Download base model and training data |
| `finetune` | `hiyouga/llamafactory:latest` | Run the fine-tuning job with LLaMA-Factory |
| `merge-upload` | `kube-llmops/model-loader:latest` | Merge LoRA adapters and upload to model store |
| `evaluate` | `python:3.13-slim` | Run evaluation on the fine-tuned model |
| `quality-gate` | `python:3.13-slim` | Evaluate metrics against thresholds |
| `deploy` | `bitnami/kubectl:latest` | Create/update ModelDeployment CR |

---

## Appendix: Engine Defaults

| Engine | Default Image | Port | PVC Size | Shared Memory |
|---|---|---|---|---|
| `vllm` | `vllm/vllm-openai:latest` | 8000 | 50Gi | 8Gi (`/dev/shm`) |
| `tei` | `ghcr.io/huggingface/text-embeddings-inference:cpu-1.6` | 8080 | 10Gi | None |
| `llamacpp` | `ghcr.io/ggml-org/llama.cpp:server` | 8080 | 30Gi | None |

## Appendix: Accelerator Resource Mapping

| Accelerator | Kubernetes Resource Name |
|---|---|
| `nvidia` | `nvidia.com/gpu` |
| `amd` | `amd.com/gpu` |
| `gaudi` | `habana.ai/gaudi` |

## Appendix: Labels Applied to Child Resources

All resources created by the operator carry the following labels:

| Label | Example Value | Description |
|---|---|---|
| `app.kubernetes.io/name` | `vllm` | Engine name |
| `app.kubernetes.io/instance` | `gemma-4-26b` | ModelDeployment name |
| `app.kubernetes.io/part-of` | `kube-llmops` | Platform identifier |
| `app.kubernetes.io/component` | `model-serving` | Component type (on Deployments) |
| `kube-llmops/model` | `gemma-4-26b` | Model identifier |
| `kube-llmops/engine` | `vllm` | Resolved engine |
