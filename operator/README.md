# kube-llmops-operator

kube-llmops-operator provides Kubernetes-native CRDs for declarative LLMOps management. Instead of Helm values, define LLMPlatform, ModelDeployment, and FineTuneRun resources directly.

## Description

The kube-llmops-operator manages the full lifecycle of large-language-model operations on Kubernetes through three Custom Resource Definitions:

- **LLMPlatform** declares a complete LLMOps infrastructure stack. A single LLMPlatform resource configures the LiteLLM AI gateway (with routing, rate-limiting, and budget control), observability (Prometheus, Grafana, Langfuse), logging (Fluent Bit + Loki), a MinIO model store, PostgreSQL, Keycloak SSO, KEDA autoscaling, and optional feature modules for RAG, fine-tuning, and security. The LLMPlatform controller translates the spec into Helm values and performs an install or upgrade of the underlying `kube-llmops-stack` Helm chart, tracking the release name, revision, and per-component health in the resource status.

- **ModelDeployment** declares a single model-serving instance. You specify a HuggingFace model ID, and the controller auto-detects the appropriate inference engine (`vllm`, `tei`, or `llamacpp`) when `engine` is set to `auto`. It then creates the required PersistentVolumeClaim, Deployment, and Service, wires up GPU resources (NVIDIA, AMD, or Gaudi, including MIG devices), and registers the model with the LiteLLM gateway. For `llamacpp` the operator handles multi-shard ("split") GGUF files — the model-loader downloads every `{prefix}-NNNNN-of-NNNNN.gguf` shard, and the pod creates symlinks at startup so llama.cpp can load them by pointing at the first shard. The llama.cpp Deployment uses the `Recreate` strategy to prevent GPU device deadlock during rolling updates. The resource also supports canary deployments with traffic-weight splitting, spot/preemptible GPU scheduling, prefix caching, and per-model store overrides. Status reports the resolved engine, endpoint URL, replica readiness, and lifecycle phase.

- **FineTuneRun** declares a fine-tuning job. You choose a base model, an output name, and a method (`lora`, `qlora`, or `full`), then point at a data source (MinIO, HuggingFace, or PVC in `alpaca`, `sharegpt`, or `custom` format). The controller builds and submits an Argo Workflow that executes the training run, tracks it through data-preparation, training, evaluation, and quality-gate phases, and optionally auto-deploys the resulting model as a new ModelDeployment. Training metrics (loss, duration) and MLflow tracking information are surfaced in the resource status.

## Custom Resources

### LLMPlatform

```yaml
apiVersion: llmops.kubellmops.io/v1alpha1
kind: LLMPlatform
metadata:
  name: my-platform
spec:
  gateway:
    enabled: true
    routing: "least-busy"
    rateLimiting:
      enabled: true
    budgetControl:
      enabled: true
  observability:
    enabled: true
    grafana:
      adminPassword: "admin"
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
    endpoint: "kube-llmops-minio:9000"
    bucket: "models"
  postgresql:
    enabled: true
  keda:
    enabled: true
  ingress:
    enabled: true
    className: "nginx"
    host: "llmops.example.com"
```

### ModelDeployment

```yaml
apiVersion: llmops.kubellmops.io/v1alpha1
kind: ModelDeployment
metadata:
  name: qwen-7b
spec:
  source: "Qwen/Qwen2.5-7B-Instruct"
  engine: auto
  replicas: 2
  accelerator: nvidia
  resources:
    gpu: 1
    memory: "24Gi"
    cpu: "8"
  engineArgs:
    max-model-len: "8192"
  prefixCaching: true
  canary:
    source: "Qwen/Qwen2.5-14B-Instruct"
    weight: 20
    resources:
      gpu: 2
      memory: "32Gi"
      cpu: "8"
```

### FineTuneRun

```yaml
apiVersion: llmops.kubellmops.io/v1alpha1
kind: FineTuneRun
metadata:
  name: my-finetune
spec:
  baseModel: "Qwen/Qwen2.5-7B-Instruct"
  outputName: "qwen-7b-custom"
  method: qlora
  dataSource:
    type: minio
    path: "s3://datasets/my-data/"
    format: alpaca
  training:
    epochs: 3
    batchSize: 4
    learningRate: "2e-5"
    gradientAccumulationSteps: 4
    warmupRatio: "0.03"
    loraRank: 16
    loraAlpha: 32
    loraTarget: "q_proj,v_proj"
  resources:
    gpu: 1
    memory: "24Gi"
    cpu: "8"
  evaluation:
    enabled: true
    dataset: "tatsu-lab/alpaca_eval"
  qualityGate:
    enabled: true
    thresholds:
      minEvalLoss: "1.5"
      maxTrainLoss: "0.8"
  deploy:
    enabled: true
    canaryWeight: 10
```

## Getting Started

### Prerequisites
- go version v1.23.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To Deploy on the cluster

The recommended v0.5.0+ workflow uses the provided build script and Helm chart:

```sh
# 1. Build the operator image (embeds the umbrella chart via helm dep update)
bash operator/build.sh

# 2. Tag + push to your registry
docker tag kube-llmops/operator:latest <your-registry>/kube-llmops/operator:latest
docker push <your-registry>/kube-llmops/operator:latest

# 3. Install the operator chart (CRDs + controller + RBAC)
helm install kube-llmops-operator operator/charts/kube-llmops-operator \
  --set image.repository=<your-registry>/kube-llmops/operator

# 4. Apply a sample CR
kubectl apply -f operator/config/samples/llmplatform_full.yaml
```

**Alternative: Build and push your image with the Kubebuilder-generated targets:**

```sh
make docker-build docker-push IMG=<some-registry>/kube-llmops-operator:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don't work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/kube-llmops-operator:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Project Distribution

Following the options to release and provide this solution to the users.

### By providing a bundle with all YAML files

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/kube-llmops-operator:tag
```

**NOTE:** The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without its
dependencies.

2. Using the installer

Users can just run 'kubectl apply -f <URL for YAML BUNDLE>' to install
the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/kube-llmops-operator/<tag or branch>/dist/install.yaml
```

### By providing a Helm Chart

1. Build the chart using the optional helm plugin

```sh
kubebuilder edit --plugins=helm/v1-alpha
```

2. See that a chart was generated under 'dist/chart', and users
can obtain this solution from there.

**NOTE:** If you change the project, you need to update the Helm Chart
using the same command above to sync the latest changes. Furthermore,
if you create webhooks, you need to use the above command with
the '--force' flag and manually ensure that any custom configuration
previously added to 'dist/chart/values.yaml' or 'dist/chart/manager/manager.yaml'
is manually re-applied afterwards.

## Contributing

Contributions are welcome! To get started:

1. **Fork** the repository and clone your fork locally.
2. **Create a feature branch** from `main` (e.g. `git checkout -b feat/my-change`).
3. **Make your changes** and ensure the code compiles with `make build`.
4. **Run the tests** with `make test` and verify they pass before submitting.
5. **Commit** with a clear, descriptive commit message.
6. **Open a Pull Request** against `main` with a summary of what changed and why.

Please keep PRs focused on a single concern and include tests for new functionality.

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
