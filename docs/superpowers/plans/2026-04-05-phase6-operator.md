# Phase 6: Kubernetes Operator — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Kubernetes Operator that manages LLM infrastructure declaratively via 3 CRDs (`ModelDeployment`, `LLMPlatform`, `FineTuneRun`).

**Architecture:** Go operator built with Kubebuilder v4. Hybrid reconciliation: ModelDeployment creates K8s resources directly for speed, LLMPlatform uses Helm SDK internally for automatic feature parity with the existing umbrella chart, FineTuneRun creates Argo Workflow CRs.

**Tech Stack:** Go 1.22+, Kubebuilder v4, controller-runtime v0.19+, Helm SDK v3, envtest, Ginkgo v2

**Spec:** `docs/superpowers/specs/2026-04-05-phase6-operator-design.md`

---

## Prerequisites

```bash
# Install Go 1.22+
go version  # must be >= 1.22

# Install Kubebuilder v4
go install sigs.k8s.io/kubebuilder/cmd@latest
kubebuilder version

# Install controller-gen (for CRD generation)
go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest

# Install setup-envtest (for integration tests)
go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

# Install kind (for E2E tests)
go install sigs.k8s.io/kind@latest
```

---

## File Structure

All new files live under `operator/` within the existing `kube-llmops` repo.

```
operator/
├── Dockerfile
├── Makefile                                    # kubebuilder targets
├── PROJECT                                     # kubebuilder metadata
├── go.mod
├── go.sum
├── cmd/
│   └── main.go                                 # entrypoint
├── api/
│   └── v1alpha1/
│       ├── modeldeployment_types.go            # Task 2
│       ├── llmplatform_types.go                # Task 3
│       ├── finetunerun_types.go                # Task 4
│       ├── modeldeployment_webhook.go          # Task 15
│       ├── llmplatform_webhook.go              # Task 15
│       ├── finetunerun_webhook.go              # Task 15
│       ├── groupversion_info.go                # Task 1 (scaffold)
│       └── zz_generated.deepcopy.go            # generated
├── internal/
│   ├── engine/
│   │   ├── resolver.go                         # Task 5
│   │   └── resolver_test.go                    # Task 5
│   ├── builder/
│   │   ├── deployment.go                       # Task 6
│   │   ├── deployment_test.go                  # Task 6
│   │   ├── service.go                          # Task 7
│   │   ├── pvc.go                              # Task 7
│   │   ├── service_pvc_test.go                 # Task 7
│   │   ├── workflow.go                         # Task 13
│   │   └── workflow_test.go                    # Task 13
│   ├── gateway/
│   │   ├── client.go                           # Task 8
│   │   └── client_test.go                      # Task 8
│   ├── helmbridge/
│   │   ├── values.go                           # Task 10
│   │   ├── values_test.go                      # Task 10
│   │   └── client.go                           # Task 11
│   └── controller/
│       ├── modeldeployment_controller.go       # Task 9
│       ├── modeldeployment_controller_test.go  # Task 9
│       ├── llmplatform_controller.go           # Task 12
│       ├── llmplatform_controller_test.go      # Task 12
│       ├── finetunerun_controller.go           # Task 14
│       ├── finetunerun_controller_test.go      # Task 14
│       └── suite_test.go                       # Task 9 (shared envtest setup)
├── config/
│   ├── crd/bases/                              # generated
│   ├── manager/manager.yaml                    # scaffold
│   ├── rbac/                                   # scaffold
│   └── samples/                                # Task 17
│       ├── modeldeployment_vllm.yaml
│       ├── modeldeployment_tei_embedding.yaml
│       ├── modeldeployment_tei_reranker.yaml
│       ├── modeldeployment_llamacpp.yaml
│       ├── llmplatform_minimal.yaml
│       ├── llmplatform_full.yaml
│       └── finetunerun_lora.yaml
├── charts/
│   └── kube-llmops-operator/                   # Task 16
│       ├── Chart.yaml
│       ├── values.yaml
│       └── templates/
│           ├── deployment.yaml
│           ├── serviceaccount.yaml
│           └── rbac.yaml
└── docs/
    ├── architecture/
    │   └── operator.md                         # Task 19
    └── user-guide/
        ├── operator-guide-en.md                # Task 20
        └── operator-guide-zh.md                # Task 21
```

### Task Dependencies

```
Task 1 (scaffold) ──→ Task 2,3,4 (types) ──→ Task 5 (engine)  ──→ Task 6,7,8 (builders+client) ──→ Task 9 (MD ctrl)
                                                                                                  ──→ Task 10,11 (helm) ──→ Task 12 (LP ctrl)
                                                                                                  ──→ Task 13 (wf builder) ──→ Task 14 (FTR ctrl)
                                              Tasks 9,12,14 ──→ Task 15 (webhooks) ──→ Task 16 (chart) ──→ Task 17 (samples)
                                              Task 15 ──→ Task 18 (migration)
                                              Task 17 ──→ Task 19,20,21 (docs — parallelizable)
```

---

## Task 1: Kubebuilder Project Scaffold

**Files:**
- Create: `operator/` (entire scaffold)

- [ ] **Step 1: Create operator directory and initialize project**

```bash
cd /home/rui/kube-llmops
mkdir -p operator && cd operator
kubebuilder init --domain kubellmops.io --repo github.com/kube-llmops/operator --project-name kube-llmops-operator
```

Expected: `PROJECT`, `Makefile`, `Dockerfile`, `cmd/main.go`, `go.mod`, `config/` created.

- [ ] **Step 2: Create ModelDeployment API**

```bash
cd /home/rui/kube-llmops/operator
kubebuilder create api --group llmops --version v1alpha1 --kind ModelDeployment --resource --controller
```

Expected: `api/v1alpha1/modeldeployment_types.go`, `internal/controller/modeldeployment_controller.go` created. Press `y` for both prompts.

- [ ] **Step 3: Create LLMPlatform API**

```bash
cd /home/rui/kube-llmops/operator
kubebuilder create api --group llmops --version v1alpha1 --kind LLMPlatform --resource --controller
```

- [ ] **Step 4: Create FineTuneRun API**

```bash
cd /home/rui/kube-llmops/operator
kubebuilder create api --group llmops --version v1alpha1 --kind FineTuneRun --resource --controller
```

- [ ] **Step 5: Verify project compiles**

```bash
cd /home/rui/kube-llmops/operator
go build ./...
```

Expected: exit 0, no errors.

- [ ] **Step 6: Commit**

```bash
cd /home/rui/kube-llmops
git add operator/
git commit -m "feat(operator): kubebuilder scaffold with 3 CRDs"
```

---

## Task 2: ModelDeployment CRD Types

**Files:**
- Modify: `operator/api/v1alpha1/modeldeployment_types.go`

- [ ] **Step 1: Replace scaffolded types with full ModelDeployment spec/status**

Replace the entire contents of `operator/api/v1alpha1/modeldeployment_types.go` with:

```go
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ModelDeploymentSpec defines the desired state of a model deployment.
type ModelDeploymentSpec struct {
	// Source is the HuggingFace model ID or path (e.g. "Qwen/Qwen2.5-7B-Instruct").
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Source string `json:"source"`

	// Engine selects the inference engine. "auto" detects from source name.
	// +kubebuilder:validation:Enum=auto;vllm;tei;llamacpp
	// +kubebuilder:default="auto"
	// +optional
	Engine string `json:"engine,omitempty"`

	// Replicas is the desired number of model serving pods.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=1
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Resources specifies compute resources for the model serving container.
	// +optional
	Resources ModelResources `json:"resources,omitempty"`

	// Accelerator selects the GPU vendor for scheduling.
	// +kubebuilder:validation:Enum=nvidia;amd;gaudi
	// +kubebuilder:default="nvidia"
	// +optional
	Accelerator string `json:"accelerator,omitempty"`

	// MIGDevice specifies an NVIDIA MIG device resource name, overriding the gpu field.
	// +optional
	MIGDevice string `json:"migDevice,omitempty"`

	// EngineArgs are extra CLI arguments passed to the inference engine.
	// Keys are flag names (e.g. "--max-model-len"), values are flag values.
	// +optional
	EngineArgs map[string]string `json:"engineArgs,omitempty"`

	// PrefixCaching enables vLLM automatic prefix caching.
	// +optional
	PrefixCaching bool `json:"prefixCaching,omitempty"`

	// Spot configures spot/preemptible GPU scheduling.
	// +optional
	Spot *SpotConfig `json:"spot,omitempty"`

	// Canary configures a canary deployment for A/B traffic splitting.
	// +optional
	Canary *CanaryConfig `json:"canary,omitempty"`

	// ModelStore overrides the platform-level model store configuration.
	// If unset, the operator reads from the LLMPlatform CR in the same namespace.
	// +optional
	ModelStore *ModelStoreOverride `json:"modelStore,omitempty"`
}

// ModelResources defines compute resources for model serving.
type ModelResources struct {
	// GPU is the number of GPUs to request (0 for CPU-only).
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=1
	// +optional
	GPU int32 `json:"gpu,omitempty"`

	// Memory is the memory limit (e.g. "16Gi").
	// +kubebuilder:default="16Gi"
	// +optional
	Memory string `json:"memory,omitempty"`

	// CPU is the CPU limit (e.g. "4").
	// +kubebuilder:default="4"
	// +optional
	CPU string `json:"cpu,omitempty"`
}

// SpotConfig configures spot/preemptible GPU scheduling.
type SpotConfig struct {
	// Enabled enables spot instance scheduling.
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Provider selects the cloud provider for spot tolerations.
	// +kubebuilder:validation:Enum=aws;gcp;azure;karpenter
	// +optional
	Provider string `json:"provider,omitempty"`
}

// CanaryConfig configures canary deployment with traffic splitting.
type CanaryConfig struct {
	// Source is the HuggingFace model ID for the canary model.
	// +kubebuilder:validation:Required
	Source string `json:"source"`

	// Weight is the percentage of traffic routed to the canary (0-100).
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	Weight int32 `json:"weight"`

	// Resources for the canary deployment.
	// +optional
	Resources ModelResources `json:"resources,omitempty"`
}

// ModelStoreOverride allows per-model override of model store settings.
type ModelStoreOverride struct {
	// Endpoint is the MinIO endpoint (e.g. "kube-llmops-minio:9000").
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// Bucket is the MinIO bucket name.
	// +optional
	Bucket string `json:"bucket,omitempty"`
}

// ModelDeploymentStatus defines the observed state of ModelDeployment.
type ModelDeploymentStatus struct {
	// Phase is the high-level lifecycle phase.
	// +kubebuilder:validation:Enum=Pending;Downloading;Deploying;Ready;Degraded;Failed
	// +optional
	Phase string `json:"phase,omitempty"`

	// Engine is the resolved engine after auto-detection.
	// +optional
	Engine string `json:"engine,omitempty"`

	// Endpoint is the in-cluster URL for the model's API.
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// ReadyReplicas is the number of ready model serving pods.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// TotalReplicas is the desired number of replicas.
	// +optional
	TotalReplicas int32 `json:"totalReplicas,omitempty"`

	// ModelSize is the discovered model size after download.
	// +optional
	ModelSize string `json:"modelSize,omitempty"`

	// Conditions represent the latest observations of the resource's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Canary holds the status of the canary deployment.
	// +optional
	Canary *CanaryStatus `json:"canary,omitempty"`
}

// CanaryStatus holds the observed state of the canary deployment.
type CanaryStatus struct {
	// Phase is the canary deployment phase.
	// +optional
	Phase string `json:"phase,omitempty"`

	// Endpoint is the in-cluster URL for the canary model API.
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// ReadyReplicas is the number of ready canary pods.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=md
// +kubebuilder:printcolumn:name="Engine",type=string,JSONPath=`.status.engine`
// +kubebuilder:printcolumn:name="Replicas",type=string,JSONPath=`.status.readyReplicas`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Endpoint",type=string,JSONPath=`.status.endpoint`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ModelDeployment is the Schema for the modeldeployments API.
type ModelDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ModelDeploymentSpec   `json:"spec,omitempty"`
	Status ModelDeploymentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ModelDeploymentList contains a list of ModelDeployment.
type ModelDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ModelDeployment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ModelDeployment{}, &ModelDeploymentList{})
}
```

- [ ] **Step 2: Generate deepcopy and CRD manifests**

```bash
cd /home/rui/kube-llmops/operator
make generate
make manifests
```

Expected: `zz_generated.deepcopy.go` updated, `config/crd/bases/llmops.kubellmops.io_modeldeployments.yaml` generated.

- [ ] **Step 3: Verify CRD YAML has correct printer columns**

```bash
cd /home/rui/kube-llmops/operator
grep -A 5 "printcolumn" config/crd/bases/llmops.kubellmops.io_modeldeployments.yaml | head -20
```

Expected: columns for Engine, Replicas, Phase, Endpoint, Age visible.

- [ ] **Step 4: Verify compilation**

```bash
cd /home/rui/kube-llmops/operator
go build ./...
```

Expected: exit 0.

- [ ] **Step 5: Commit**

```bash
cd /home/rui/kube-llmops
git add operator/api/v1alpha1/modeldeployment_types.go operator/api/v1alpha1/zz_generated.deepcopy.go operator/config/crd/
git commit -m "feat(operator): ModelDeployment CRD types with full spec/status"
```

---

## Task 3: LLMPlatform CRD Types

**Files:**
- Modify: `operator/api/v1alpha1/llmplatform_types.go`

- [ ] **Step 1: Replace scaffolded types with full LLMPlatform spec/status**

Replace the entire contents of `operator/api/v1alpha1/llmplatform_types.go` with:

```go
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LLMPlatformSpec defines the desired state of the platform infrastructure.
type LLMPlatformSpec struct {
	// Gateway configures the LiteLLM AI Gateway.
	// +optional
	Gateway GatewaySpec `json:"gateway,omitempty"`

	// Observability configures Prometheus + Grafana + Langfuse.
	// +optional
	Observability ObservabilitySpec `json:"observability,omitempty"`

	// Logging configures Fluent Bit + Loki.
	// +optional
	Logging LoggingSpec `json:"logging,omitempty"`

	// Modules toggles high-level feature modules.
	// +optional
	Modules ModulesSpec `json:"modules,omitempty"`

	// ModelStore configures the MinIO model cache.
	// +optional
	ModelStore ModelStoreSpec `json:"modelStore,omitempty"`

	// HFToken is a HuggingFace API token for gated models.
	// +optional
	HFToken string `json:"hfToken,omitempty"`

	// Keycloak configures SSO.
	// +optional
	Keycloak KeycloakSpec `json:"keycloak,omitempty"`

	// PostgreSQL configures the shared database.
	// +optional
	PostgreSQL PostgreSQLSpec `json:"postgresql,omitempty"`

	// KEDA configures autoscaling.
	// +optional
	KEDA KEDASpec `json:"keda,omitempty"`

	// NodePort configures NodePort access to services.
	// +optional
	NodePort NodePortSpec `json:"nodePort,omitempty"`

	// Ingress configures ingress-based access.
	// +optional
	Ingress IngressSpec `json:"ingress,omitempty"`
}

// GatewaySpec configures LiteLLM.
type GatewaySpec struct {
	Enabled        bool             `json:"enabled,omitempty"`
	Routing        string           `json:"routing,omitempty"`        // simple-shuffle | latency-based-routing
	Image          ImageSpec        `json:"image,omitempty"`
	RateLimiting   EnabledToggle    `json:"rateLimiting,omitempty"`
	BudgetControl  EnabledToggle    `json:"budgetControl,omitempty"`
}

// ObservabilitySpec configures monitoring and tracing.
type ObservabilitySpec struct {
	Enabled  bool        `json:"enabled,omitempty"`
	Grafana  GrafanaSpec `json:"grafana,omitempty"`
	Langfuse EnabledToggle `json:"langfuse,omitempty"`
}

// GrafanaSpec configures Grafana.
type GrafanaSpec struct {
	AdminPassword string        `json:"adminPassword,omitempty"`
	OIDC          EnabledToggle `json:"oidc,omitempty"`
}

// LoggingSpec configures Fluent Bit + Loki.
type LoggingSpec struct {
	Enabled bool `json:"enabled,omitempty"`
}

// ModulesSpec toggles feature modules.
type ModulesSpec struct {
	RAG      EnabledToggle `json:"rag,omitempty"`
	Finetune EnabledToggle `json:"finetune,omitempty"`
	Security EnabledToggle `json:"security,omitempty"`
}

// ModelStoreSpec configures MinIO model caching.
type ModelStoreSpec struct {
	Enabled              bool   `json:"enabled,omitempty"`
	Endpoint             string `json:"endpoint,omitempty"`
	Bucket               string `json:"bucket,omitempty"`
	AccessKey            string `json:"accessKey,omitempty"`
	SecretKey            string `json:"secretKey,omitempty"`
	HFTransferConcurrency int32 `json:"hfTransferConcurrency,omitempty"`
	Image                string `json:"image,omitempty"`
}

// KeycloakSpec configures SSO.
type KeycloakSpec struct {
	Enabled bool `json:"enabled,omitempty"`
}

// PostgreSQLSpec configures the database.
type PostgreSQLSpec struct {
	Enabled bool `json:"enabled,omitempty"`
}

// KEDASpec configures autoscaling.
type KEDASpec struct {
	Enabled bool `json:"enabled,omitempty"`
}

// NodePortSpec configures NodePort access.
type NodePortSpec struct {
	Enabled bool   `json:"enabled,omitempty"`
	Host    string `json:"host,omitempty"`
}

// IngressSpec configures ingress access.
type IngressSpec struct {
	Enabled   bool   `json:"enabled,omitempty"`
	ClassName string `json:"className,omitempty"`
	Host      string `json:"host,omitempty"`
}

// ImageSpec holds container image configuration.
type ImageSpec struct {
	Repository string `json:"repository,omitempty"`
	Tag        string `json:"tag,omitempty"`
}

// EnabledToggle is a simple enabled/disabled toggle.
type EnabledToggle struct {
	Enabled bool `json:"enabled,omitempty"`
}

// LLMPlatformStatus defines the observed state of LLMPlatform.
type LLMPlatformStatus struct {
	// Phase is the high-level lifecycle phase.
	// +kubebuilder:validation:Enum=Pending;Installing;Ready;Upgrading;Degraded;Failed
	// +optional
	Phase string `json:"phase,omitempty"`

	// HelmRelease is the name of the managed Helm release.
	// +optional
	HelmRelease string `json:"helmRelease,omitempty"`

	// HelmRevision is the Helm release revision number.
	// +optional
	HelmRevision int `json:"helmRevision,omitempty"`

	// Components holds per-component health status.
	// +optional
	Components ComponentStatuses `json:"components,omitempty"`

	// Conditions represent the latest observations.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// ComponentStatuses holds status for each platform component.
type ComponentStatuses struct {
	Gateway    *ComponentStatus `json:"gateway,omitempty"`
	Grafana    *ComponentStatus `json:"grafana,omitempty"`
	Prometheus *ComponentStatus `json:"prometheus,omitempty"`
	Langfuse   *ComponentStatus `json:"langfuse,omitempty"`
	MinIO      *ComponentStatus `json:"minio,omitempty"`
	PostgreSQL *ComponentStatus `json:"postgresql,omitempty"`
	Dify       *ComponentStatus `json:"dify,omitempty"`
	Milvus     *ComponentStatus `json:"milvus,omitempty"`
}

// ComponentStatus holds the health of a single platform component.
type ComponentStatus struct {
	Phase    string `json:"phase,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
	NodePort int32  `json:"nodePort,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=lp;llmplatforms
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Gateway",type=string,JSONPath=`.status.components.gateway.phase`
// +kubebuilder:printcolumn:name="Grafana",type=string,JSONPath=`.status.components.grafana.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// LLMPlatform is the Schema for the llmplatforms API.
type LLMPlatform struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LLMPlatformSpec   `json:"spec,omitempty"`
	Status LLMPlatformStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LLMPlatformList contains a list of LLMPlatform.
type LLMPlatformList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LLMPlatform `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LLMPlatform{}, &LLMPlatformList{})
}
```

- [ ] **Step 2: Generate deepcopy and CRD manifests**

```bash
cd /home/rui/kube-llmops/operator
make generate && make manifests
```

- [ ] **Step 3: Verify compilation**

```bash
cd /home/rui/kube-llmops/operator
go build ./...
```

Expected: exit 0.

- [ ] **Step 4: Commit**

```bash
cd /home/rui/kube-llmops
git add operator/api/ operator/config/crd/
git commit -m "feat(operator): LLMPlatform CRD types with full spec/status"
```

---

## Task 4: FineTuneRun CRD Types

**Files:**
- Modify: `operator/api/v1alpha1/finetunerun_types.go`

- [ ] **Step 1: Replace scaffolded types with full FineTuneRun spec/status**

Replace the entire contents of `operator/api/v1alpha1/finetunerun_types.go` with:

```go
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FineTuneRunSpec defines the desired state of a fine-tuning job.
type FineTuneRunSpec struct {
	// BaseModel is the HuggingFace model ID to fine-tune.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	BaseModel string `json:"baseModel"`

	// OutputName is the name for the fine-tuned model artifact.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	OutputName string `json:"outputName"`

	// Method is the fine-tuning method.
	// +kubebuilder:validation:Enum=lora;qlora;full
	// +kubebuilder:default="lora"
	Method string `json:"method,omitempty"`

	// DataSource configures where training data comes from.
	DataSource DataSourceSpec `json:"dataSource"`

	// Training holds hyperparameters.
	// +optional
	Training TrainingSpec `json:"training,omitempty"`

	// Resources for the training container.
	// +optional
	Resources ModelResources `json:"resources,omitempty"`

	// Evaluation configures post-training evaluation.
	// +optional
	Evaluation EvaluationSpec `json:"evaluation,omitempty"`

	// QualityGate configures pass/fail thresholds.
	// +optional
	QualityGate QualityGateSpec `json:"qualityGate,omitempty"`

	// Deploy configures auto-deployment after quality gate passes.
	// +optional
	Deploy DeploySpec `json:"deploy,omitempty"`
}

// DataSourceSpec defines where training data comes from.
type DataSourceSpec struct {
	// Type is the data source type.
	// +kubebuilder:validation:Enum=minio;huggingface;pvc
	Type string `json:"type"`

	// Path is the data location (e.g. "s3://datasets/my-data/" or "tatsu-lab/alpaca").
	// +optional
	Path string `json:"path,omitempty"`

	// Format is the data format.
	// +kubebuilder:validation:Enum=alpaca;sharegpt;custom
	// +kubebuilder:default="alpaca"
	// +optional
	Format string `json:"format,omitempty"`
}

// TrainingSpec holds training hyperparameters.
type TrainingSpec struct {
	Epochs                    int32   `json:"epochs,omitempty"`
	BatchSize                 int32   `json:"batchSize,omitempty"`
	LearningRate              string  `json:"learningRate,omitempty"`
	GradientAccumulationSteps int32   `json:"gradientAccumulationSteps,omitempty"`
	WarmupRatio               string  `json:"warmupRatio,omitempty"`
	LoraRank                  int32   `json:"loraRank,omitempty"`
	LoraAlpha                 int32   `json:"loraAlpha,omitempty"`
	LoraTarget                string  `json:"loraTarget,omitempty"`
}

// EvaluationSpec configures post-training evaluation.
type EvaluationSpec struct {
	Enabled bool   `json:"enabled,omitempty"`
	Dataset string `json:"dataset,omitempty"`
}

// QualityGateSpec configures pass/fail thresholds.
type QualityGateSpec struct {
	Enabled    bool                `json:"enabled,omitempty"`
	Thresholds QualityThresholds   `json:"thresholds,omitempty"`
}

// QualityThresholds defines metric thresholds for the quality gate.
type QualityThresholds struct {
	MinEvalLoss  string `json:"minEvalLoss,omitempty"`
	MaxTrainLoss string `json:"maxTrainLoss,omitempty"`
}

// DeploySpec configures auto-deployment of fine-tuned models.
type DeploySpec struct {
	Enabled      bool  `json:"enabled,omitempty"`
	CanaryWeight int32 `json:"canaryWeight,omitempty"`
}

// FineTuneRunStatus defines the observed state of FineTuneRun.
type FineTuneRunStatus struct {
	// Phase is the high-level lifecycle phase.
	// +kubebuilder:validation:Enum=Pending;DataPreparing;Training;Evaluating;QualityGate;Deploying;Succeeded;Failed
	// +optional
	Phase string `json:"phase,omitempty"`

	// ArgoWorkflow is the name of the created Argo Workflow.
	// +optional
	ArgoWorkflow string `json:"argoWorkflow,omitempty"`

	// StartTime is when the run started.
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is when the run completed.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Metrics holds training metrics.
	// +optional
	Metrics TrainingMetrics `json:"metrics,omitempty"`

	// MLflow holds MLflow tracking info.
	// +optional
	MLflow MLflowStatus `json:"mlflow,omitempty"`

	// QualityGate holds quality gate results.
	// +optional
	QualityGate QualityGateStatus `json:"qualityGate,omitempty"`

	// OutputModel holds info about the produced model artifact.
	// +optional
	OutputModel OutputModelStatus `json:"outputModel,omitempty"`

	// Conditions represent the latest observations.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// TrainingMetrics holds observed training metrics.
type TrainingMetrics struct {
	TrainLoss        string `json:"trainLoss,omitempty"`
	EvalLoss         string `json:"evalLoss,omitempty"`
	TrainingDuration string `json:"trainingDuration,omitempty"`
}

// MLflowStatus holds MLflow tracking information.
type MLflowStatus struct {
	RunID          string `json:"runId,omitempty"`
	ExperimentName string `json:"experimentName,omitempty"`
	ArtifactURI    string `json:"artifactUri,omitempty"`
}

// QualityGateStatus holds quality gate results.
type QualityGateStatus struct {
	Passed  bool   `json:"passed,omitempty"`
	Message string `json:"message,omitempty"`
}

// OutputModelStatus holds info about the produced model.
type OutputModelStatus struct {
	Source          string `json:"source,omitempty"`
	ModelDeployment string `json:"modelDeployment,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=ftr
// +kubebuilder:printcolumn:name="Base Model",type=string,JSONPath=`.spec.baseModel`
// +kubebuilder:printcolumn:name="Method",type=string,JSONPath=`.spec.method`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Loss",type=string,JSONPath=`.status.metrics.trainLoss`,priority=1
// +kubebuilder:printcolumn:name="Duration",type=string,JSONPath=`.status.metrics.trainingDuration`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// FineTuneRun is the Schema for the finetuneruns API.
type FineTuneRun struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FineTuneRunSpec   `json:"spec,omitempty"`
	Status FineTuneRunStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// FineTuneRunList contains a list of FineTuneRun.
type FineTuneRunList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FineTuneRun `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FineTuneRun{}, &FineTuneRunList{})
}
```

- [ ] **Step 2: Generate deepcopy and CRD manifests**

```bash
cd /home/rui/kube-llmops/operator
make generate && make manifests
```

- [ ] **Step 3: Verify all 3 CRD YAMLs exist**

```bash
ls -la /home/rui/kube-llmops/operator/config/crd/bases/
```

Expected: 3 YAML files (`llmops.kubellmops.io_modeldeployments.yaml`, `llmops.kubellmops.io_llmplatforms.yaml`, `llmops.kubellmops.io_finetuneruns.yaml`).

- [ ] **Step 4: Verify compilation**

```bash
cd /home/rui/kube-llmops/operator && go build ./...
```

- [ ] **Step 5: Commit**

```bash
cd /home/rui/kube-llmops
git add operator/api/ operator/config/crd/
git commit -m "feat(operator): FineTuneRun CRD types with full spec/status"
```

---

## Task 5: Engine Resolver

**Files:**
- Create: `operator/internal/engine/resolver.go`
- Create: `operator/internal/engine/resolver_test.go`

- [ ] **Step 1: Write the engine resolver tests**

Create `operator/internal/engine/resolver_test.go`:

```go
package engine

import (
	"testing"
)

func TestResolveEngine(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		explicit string
		want     string
	}{
		// Explicit engine overrides
		{"explicit vllm", "anything", "vllm", "vllm"},
		{"explicit tei", "anything", "tei", "tei"},
		{"explicit llamacpp", "anything", "llamacpp", "llamacpp"},
		{"auto falls through", "Qwen/Qwen2.5-7B", "auto", "vllm"},
		{"empty falls through", "Qwen/Qwen2.5-7B", "", "vllm"},

		// GGUF detection → llamacpp
		{"gguf in name", "TheBloke/Llama-2-7B-GGUF", "", "llamacpp"},
		{"gguf suffix", "model-gguf", "", "llamacpp"},
		{"GGUF uppercase", "TheBloke/Model-GGUF-Q4", "", "llamacpp"},

		// Reranker detection → tei
		{"rerank model", "BAAI/bge-reranker-base", "", "tei"},
		{"rerank in name", "cross-encoder/ms-marco-rerank", "", "tei"},

		// Embedding detection → tei
		{"bge embedding", "BAAI/bge-small-en-v1.5", "", "tei"},
		{"e5 embedding", "intfloat/e5-large-v2", "", "tei"},
		{"gte embedding", "thenlper/gte-large", "", "tei"},
		{"minilm", "sentence-transformers/all-MiniLM-L6-v2", "", "tei"},
		{"jina embed", "jinaai/jina-embeddings-v2", "", "tei"},
		{"nomic embed", "nomic-ai/nomic-embed-text", "", "tei"},
		{"all-mpnet", "sentence-transformers/all-mpnet-base-v2", "", "tei"},
		{"embedding keyword", "BAAI/bge-large-embedding-v1", "", "tei"},

		// LLM fallback → vllm
		{"standard llm", "Qwen/Qwen2.5-7B-Instruct", "", "vllm"},
		{"meta llama", "meta-llama/Llama-3-8B", "", "vllm"},
		{"mistral", "mistralai/Mistral-7B-v0.1", "", "vllm"},
		{"awq model", "cyankiwi/gemma-4-26B-A4B-it-AWQ-4bit", "", "vllm"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveEngine(tt.source, tt.explicit)
			if got != tt.want {
				t.Errorf("ResolveEngine(%q, %q) = %q, want %q", tt.source, tt.explicit, got, tt.want)
			}
		})
	}
}

func TestResolveModelType(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"reranker", "BAAI/bge-reranker-base", "reranker"},
		{"embedding bge", "BAAI/bge-small-en-v1.5", "embedding"},
		{"embedding e5", "intfloat/e5-large-v2", "embedding"},
		{"llm", "Qwen/Qwen2.5-7B-Instruct", "llm"},
		{"gguf is llm", "TheBloke/Model-GGUF", "llm"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveModelType(tt.source)
			if got != tt.want {
				t.Errorf("ResolveModelType(%q) = %q, want %q", tt.source, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/rui/kube-llmops/operator
go test ./internal/engine/ -v
```

Expected: FAIL — functions not found.

- [ ] **Step 3: Implement the engine resolver**

Create `operator/internal/engine/resolver.go`:

```go
package engine

import "strings"

// ResolveEngine determines the inference engine from the model source name.
// Ported from charts/kube-llmops-stack/templates/_helpers.tpl resolveEngine.
//
// Priority:
//  1. Explicit engine (if not "" or "auto")
//  2. Heuristic from source name
//  3. Fallback: "vllm"
func ResolveEngine(source, explicit string) string {
	if explicit != "" && explicit != "auto" {
		return explicit
	}
	s := strings.ToLower(source)

	// GGUF → llamacpp
	if strings.Contains(s, "gguf") {
		return "llamacpp"
	}

	// Reranker or embedding patterns → tei
	if isEmbeddingOrReranker(s) {
		return "tei"
	}

	return "vllm"
}

// ResolveModelType determines whether a model is an "llm", "embedding", or "reranker".
// Ported from charts/kube-llmops-stack/templates/_helpers.tpl resolveModelType.
func ResolveModelType(source string) string {
	s := strings.ToLower(source)
	if strings.Contains(s, "rerank") {
		return "reranker"
	}
	if isEmbedding(s) {
		return "embedding"
	}
	return "llm"
}

func isEmbeddingOrReranker(s string) bool {
	if strings.Contains(s, "rerank") {
		return true
	}
	return isEmbedding(s)
}

func isEmbedding(s string) bool {
	patterns := []string{
		"/bge-", "/e5-", "/gte-",
		"minilm",
		"/jina-embed", "jina-embeddings",
		"/nomic-embed", "nomic-embed",
		"/all-mpnet",
		"embedding",
	}
	for _, p := range patterns {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/rui/kube-llmops/operator
go test ./internal/engine/ -v
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/rui/kube-llmops
git add operator/internal/engine/
git commit -m "feat(operator): engine auto-detection resolver (ported from _helpers.tpl)"
```

---

## Task 6: Deployment Builder

**Files:**
- Create: `operator/internal/builder/deployment.go`
- Create: `operator/internal/builder/deployment_test.go`

- [ ] **Step 1: Write failing tests for BuildDeployment**

Create `operator/internal/builder/deployment_test.go`:

```go
package builder

import (
	"testing"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func newTestMD(name, source string, gpu int32) *v1alpha1.ModelDeployment {
	return &v1alpha1.ModelDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: v1alpha1.ModelDeploymentSpec{
			Source:      source,
			Replicas:    ptr.To(int32(1)),
			Resources:   v1alpha1.ModelResources{GPU: gpu, Memory: "16Gi", CPU: "4"},
			Accelerator: "nvidia",
		},
	}
}

func newTestPlatform() *v1alpha1.LLMPlatform {
	return &v1alpha1.LLMPlatform{
		ObjectMeta: metav1.ObjectMeta{Name: "kube-llmops", Namespace: "default"},
		Spec: v1alpha1.LLMPlatformSpec{
			ModelStore: v1alpha1.ModelStoreSpec{
				Enabled:  true,
				Endpoint: "kube-llmops-minio:9000",
				Bucket:   "models",
				AccessKey: "minioadmin",
				SecretKey: "minioadmin",
				Image:     "kube-llmops/model-loader:latest",
			},
		},
	}
}

func TestBuildDeployment_VLLMEngine(t *testing.T) {
	md := newTestMD("qwen", "Qwen/Qwen2.5-7B-Instruct", 1)
	platform := newTestPlatform()

	dep := BuildDeployment(md, "vllm", platform)

	if dep.Name != "qwen" {
		t.Errorf("Name = %q, want %q", dep.Name, "qwen")
	}
	if *dep.Spec.Replicas != 1 {
		t.Errorf("Replicas = %d, want 1", *dep.Spec.Replicas)
	}

	// Check main container
	containers := dep.Spec.Template.Spec.Containers
	if len(containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(containers))
	}
	c := containers[0]
	if c.Image != "vllm/vllm-openai:latest" {
		t.Errorf("Image = %q, want %q", c.Image, "vllm/vllm-openai:latest")
	}
	if c.Ports[0].ContainerPort != 8000 {
		t.Errorf("Port = %d, want 8000", c.Ports[0].ContainerPort)
	}

	// Check initContainer (model-loader)
	inits := dep.Spec.Template.Spec.InitContainers
	if len(inits) != 1 {
		t.Fatalf("expected 1 initContainer, got %d", len(inits))
	}
	if inits[0].Image != "kube-llmops/model-loader:latest" {
		t.Errorf("InitContainer image = %q, want model-loader", inits[0].Image)
	}

	// Check /dev/shm volume for vLLM
	foundShm := false
	for _, v := range dep.Spec.Template.Spec.Volumes {
		if v.Name == "shm" {
			foundShm = true
		}
	}
	if !foundShm {
		t.Error("vLLM deployment missing /dev/shm volume")
	}

	// Check GPU toleration
	foundToleration := false
	for _, tol := range dep.Spec.Template.Spec.Tolerations {
		if tol.Key == "nvidia.com/gpu" {
			foundToleration = true
		}
	}
	if !foundToleration {
		t.Error("Missing GPU toleration")
	}
}

func TestBuildDeployment_TEIEngine(t *testing.T) {
	md := newTestMD("bge", "BAAI/bge-small-en-v1.5", 0)
	platform := newTestPlatform()

	dep := BuildDeployment(md, "tei", platform)

	c := dep.Spec.Template.Spec.Containers[0]
	if c.Image != "ghcr.io/huggingface/text-embeddings-inference:cpu-1.6" {
		t.Errorf("TEI image = %q", c.Image)
	}
	if c.Ports[0].ContainerPort != 8080 {
		t.Errorf("TEI port = %d, want 8080", c.Ports[0].ContainerPort)
	}

	// TEI should NOT have /dev/shm
	for _, v := range dep.Spec.Template.Spec.Volumes {
		if v.Name == "shm" {
			t.Error("TEI should not have /dev/shm volume")
		}
	}
}

func TestBuildDeployment_LlamaCppEngine(t *testing.T) {
	md := newTestMD("llama-gguf", "TheBloke/Llama-2-7B-GGUF", 1)
	platform := newTestPlatform()

	dep := BuildDeployment(md, "llamacpp", platform)

	c := dep.Spec.Template.Spec.Containers[0]
	if c.Image != "ghcr.io/ggml-org/llama.cpp:server" {
		t.Errorf("llamacpp image = %q", c.Image)
	}
	if c.Ports[0].ContainerPort != 8080 {
		t.Errorf("llamacpp port = %d, want 8080", c.Ports[0].ContainerPort)
	}
}

func TestBuildDeployment_EngineArgs(t *testing.T) {
	md := newTestMD("qwen", "Qwen/Qwen2.5-7B-Instruct", 1)
	md.Spec.EngineArgs = map[string]string{
		"--max-model-len": "8192",
		"--dtype":         "half",
	}
	platform := newTestPlatform()

	dep := BuildDeployment(md, "vllm", platform)

	args := dep.Spec.Template.Spec.Containers[0].Args
	found := map[string]bool{}
	for _, a := range args {
		found[a] = true
	}
	if !found["--max-model-len"] || !found["8192"] {
		t.Errorf("Missing engine args in container args: %v", args)
	}
}

func TestBuildDeployment_AMDAccelerator(t *testing.T) {
	md := newTestMD("qwen", "Qwen/Qwen2.5-7B-Instruct", 1)
	md.Spec.Accelerator = "amd"
	platform := newTestPlatform()

	dep := BuildDeployment(md, "vllm", platform)

	foundToleration := false
	for _, tol := range dep.Spec.Template.Spec.Tolerations {
		if tol.Key == "amd.com/gpu" {
			foundToleration = true
		}
	}
	if !foundToleration {
		t.Error("Missing AMD GPU toleration")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/rui/kube-llmops/operator
go test ./internal/builder/ -v -run TestBuildDeployment
```

Expected: FAIL — function not found.

- [ ] **Step 3: Implement BuildDeployment**

Create `operator/internal/builder/deployment.go`:

```go
package builder

import (
	"fmt"
	"strings"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Engine configuration constants matching existing Helm chart defaults.
var (
	EngineImages = map[string]string{
		"vllm":     "vllm/vllm-openai:latest",
		"tei":      "ghcr.io/huggingface/text-embeddings-inference:cpu-1.6",
		"llamacpp": "ghcr.io/ggml-org/llama.cpp:server",
	}
	EnginePorts = map[string]int32{
		"vllm":     8000,
		"tei":      8080,
		"llamacpp": 8080,
	}
	EngineHealthPaths = map[string]string{
		"vllm":     "/health",
		"tei":      "/health",
		"llamacpp": "/health",
	}
)

// gpuResourceName returns the Kubernetes resource name for the given accelerator.
func gpuResourceName(accelerator string) corev1.ResourceName {
	switch accelerator {
	case "amd":
		return "amd.com/gpu"
	case "gaudi":
		return "habana.ai/gaudi"
	default:
		return "nvidia.com/gpu"
	}
}

// BuildDeployment creates a Kubernetes Deployment for a ModelDeployment CR.
func BuildDeployment(md *v1alpha1.ModelDeployment, engine string, platform *v1alpha1.LLMPlatform) *appsv1.Deployment {
	port := EnginePorts[engine]
	image := EngineImages[engine]
	healthPath := EngineHealthPaths[engine]
	replicas := int32(1)
	if md.Spec.Replicas != nil {
		replicas = *md.Spec.Replicas
	}

	labels := map[string]string{
		"app.kubernetes.io/name":      engine,
		"app.kubernetes.io/instance":  md.Name,
		"app.kubernetes.io/part-of":   "kube-llmops",
		"app.kubernetes.io/component": "model-serving",
		"kube-llmops/model":           md.Name,
		"kube-llmops/engine":          engine,
	}

	// Build main container args
	args := buildEngineArgs(md, engine)

	// Build resource requirements
	resources := buildResources(md, engine)

	// Build volumes + volume mounts
	volumes, volumeMounts := buildVolumes(md, engine)

	// Build env vars
	envVars := buildEnvVars(md, platform)

	// Build init containers
	initContainers := buildInitContainers(md, platform)

	// Build tolerations
	tolerations := buildTolerations(md)

	// Readiness/liveness probes
	readinessInitialDelay := int32(30)
	readinessFailureThreshold := int32(60)
	livenessInitialDelay := int32(240)
	if engine != "vllm" {
		readinessInitialDelay = 15
		readinessFailureThreshold = 20
		livenessInitialDelay = 30
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      md.Name,
			Namespace: md.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Strategy: appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType},
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
				"app.kubernetes.io/name": engine,
				"kube-llmops/model":      md.Name,
			}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					EnableServiceLinks:            boolPtr(false),
					TerminationGracePeriodSeconds: int64Ptr(90),
					InitContainers:                initContainers,
					Containers: []corev1.Container{
						{
							Name:            engine,
							Image:           image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{Name: "http", ContainerPort: port, Protocol: corev1.ProtocolTCP},
							},
							Args:         args,
							Env:          envVars,
							Resources:    resources,
							VolumeMounts: volumeMounts,
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{Path: healthPath, Port: intOrString(port)},
								},
								InitialDelaySeconds: readinessInitialDelay,
								PeriodSeconds:       10,
								TimeoutSeconds:      5,
								FailureThreshold:    readinessFailureThreshold,
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{Path: healthPath, Port: intOrString(port)},
								},
								InitialDelaySeconds: livenessInitialDelay,
								PeriodSeconds:       30,
								TimeoutSeconds:      5,
								FailureThreshold:    20,
							},
						},
					},
					Volumes:      volumes,
					Tolerations:  tolerations,
				},
			},
		},
	}

	return dep
}

func buildEngineArgs(md *v1alpha1.ModelDeployment, engine string) []string {
	var args []string

	switch engine {
	case "vllm":
		slug := strings.ReplaceAll(md.Spec.Source, "/", "--")
		args = append(args, "--model", fmt.Sprintf("/models/%s", slug))
		args = append(args, "--served-model-name", md.Name)
		args = append(args, "--host", "0.0.0.0", "--port", "8000")
		if md.Spec.PrefixCaching {
			args = append(args, "--enable-prefix-caching")
		}
	case "tei":
		slug := strings.ReplaceAll(md.Spec.Source, "/", "--")
		args = append(args, "--model-id", fmt.Sprintf("/models/%s", slug))
		args = append(args, "--hostname", "0.0.0.0", "--port", "8080")
	case "llamacpp":
		slug := strings.ReplaceAll(md.Spec.Source, "/", "--")
		args = append(args, "--model", fmt.Sprintf("/models/%s", slug))
		args = append(args, "--host", "0.0.0.0", "--port", "8080")
	}

	// Append user-specified engine args
	for k, v := range md.Spec.EngineArgs {
		args = append(args, k)
		if v != "" {
			args = append(args, v)
		}
	}

	return args
}

func buildResources(md *v1alpha1.ModelDeployment, engine string) corev1.ResourceRequirements {
	reqs := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{},
		Limits:   corev1.ResourceList{},
	}
	if md.Spec.Resources.CPU != "" {
		reqs.Requests[corev1.ResourceCPU] = resource.MustParse(md.Spec.Resources.CPU)
	}
	if md.Spec.Resources.Memory != "" {
		qty := resource.MustParse(md.Spec.Resources.Memory)
		reqs.Requests[corev1.ResourceMemory] = qty
		reqs.Limits[corev1.ResourceMemory] = qty
	}
	if md.Spec.Resources.GPU > 0 {
		gpuRes := gpuResourceName(md.Spec.Accelerator)
		qty := resource.MustParse(fmt.Sprintf("%d", md.Spec.Resources.GPU))
		reqs.Requests[gpuRes] = qty
		reqs.Limits[gpuRes] = qty
	}
	return reqs
}

func buildVolumes(md *v1alpha1.ModelDeployment, engine string) ([]corev1.Volume, []corev1.VolumeMount) {
	volumes := []corev1.Volume{
		{
			Name: "model-cache",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: md.Name + "-cache",
				},
			},
		},
	}
	mounts := []corev1.VolumeMount{
		{Name: "model-cache", MountPath: "/models"},
	}

	// vLLM needs /dev/shm for CUDA shared memory
	if engine == "vllm" {
		shmSize := resource.MustParse("8Gi")
		volumes = append(volumes, corev1.Volume{
			Name: "shm",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					Medium:    corev1.StorageMediumMemory,
					SizeLimit: &shmSize,
				},
			},
		})
		mounts = append(mounts, corev1.VolumeMount{Name: "shm", MountPath: "/dev/shm"})
	}

	return volumes, mounts
}

func buildEnvVars(md *v1alpha1.ModelDeployment, platform *v1alpha1.LLMPlatform) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "HF_HOME", Value: "/models/huggingface"},
	}
}

func buildInitContainers(md *v1alpha1.ModelDeployment, platform *v1alpha1.LLMPlatform) []corev1.Container {
	loaderImage := "kube-llmops/model-loader:latest"
	endpoint := "kube-llmops-minio:9000"
	bucket := "models"
	accessKey := "minioadmin"
	secretKey := "minioadmin"
	concurrency := "32"

	if platform != nil {
		ms := platform.Spec.ModelStore
		if ms.Image != "" {
			loaderImage = ms.Image
		}
		if ms.Endpoint != "" {
			endpoint = ms.Endpoint
		}
		if ms.Bucket != "" {
			bucket = ms.Bucket
		}
		if ms.AccessKey != "" {
			accessKey = ms.AccessKey
		}
		if ms.SecretKey != "" {
			secretKey = ms.SecretKey
		}
		if ms.HFTransferConcurrency > 0 {
			concurrency = fmt.Sprintf("%d", ms.HFTransferConcurrency)
		}
	}

	// Per-model override
	if md.Spec.ModelStore != nil {
		if md.Spec.ModelStore.Endpoint != "" {
			endpoint = md.Spec.ModelStore.Endpoint
		}
		if md.Spec.ModelStore.Bucket != "" {
			bucket = md.Spec.ModelStore.Bucket
		}
	}

	slug := strings.ReplaceAll(md.Spec.Source, "/", "--")

	return []corev1.Container{
		{
			Name:            "model-loader",
			Image:           loaderImage,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command:         []string{"bash", "-c"},
			Args: []string{fmt.Sprintf(
				`python3 -c "
from huggingface_hub import snapshot_download
from minio import Minio
import os, shutil, logging
from pathlib import Path

logging.basicConfig(level=logging.INFO, format='%%(asctime)s [model-loader] %%(levelname)s %%(message)s')
log = logging.getLogger('model-loader')

source = os.environ['MODEL_SOURCE']
target = Path(os.environ['MODEL_DIR']) / source.replace('/', '--')
target.mkdir(parents=True, exist_ok=True)
endpoint = os.environ.get('S3_ENDPOINT', '')
bucket = os.environ.get('S3_BUCKET', 'models')
ak = os.environ.get('S3_ACCESS_KEY', 'minioadmin')
sk = os.environ.get('S3_SECRET_KEY', 'minioadmin')
slug = source.replace('/', '--')
downloaded = False

if endpoint:
    try:
        mc = Minio(endpoint, access_key=ak, secret_key=sk, secure=False)
        prefix = slug + '/'
        objs = list(mc.list_objects(bucket, prefix=prefix, recursive=False))
        if objs:
            log.info(f'Found in MinIO: s3://{bucket}/{prefix}')
            for obj in mc.list_objects(bucket, prefix=prefix, recursive=True):
                rel = obj.object_name[len(prefix):]
                if not rel: continue
                fpath = target / rel
                fpath.parent.mkdir(parents=True, exist_ok=True)
                if fpath.exists() and fpath.stat().st_size == obj.size:
                    continue
                mc.fget_object(bucket, obj.object_name, str(fpath))
            downloaded = True
            log.info(f'MinIO sync complete')
    except Exception as e:
        log.warning(f'MinIO failed: {e}')

if not downloaded:
    log.info(f'Downloading from HuggingFace: {source}')
    snap = snapshot_download(repo_id=source, cache_dir=str(target / '.hf_cache'))
    snap_path = Path(snap)
    for f in snap_path.rglob('*'):
        if not f.is_file(): continue
        rel = f.relative_to(snap_path)
        dst = target / rel
        dst.parent.mkdir(parents=True, exist_ok=True)
        real_f = f.resolve()
        if dst.exists() and dst.stat().st_size == real_f.stat().st_size: continue
        shutil.copy2(str(real_f), str(dst))
    if endpoint:
        try:
            mc = Minio(endpoint, access_key=ak, secret_key=sk, secure=False)
            for fpath in target.rglob('*'):
                if not fpath.is_file() or '.hf_cache' in str(fpath): continue
                mc.fput_object(bucket, slug + '/' + str(fpath.relative_to(target)), str(fpath))
        except: pass

log.info('Model ready.')
"`)},
			Env: []corev1.EnvVar{
				{Name: "MODEL_SOURCE", Value: md.Spec.Source},
				{Name: "MODEL_DIR", Value: "/models"},
				{Name: "HF_HOME", Value: "/models/huggingface"},
				{Name: "HF_HUB_ENABLE_HF_TRANSFER", Value: "1"},
				{Name: "HF_TRANSFER_CONCURRENCY", Value: concurrency},
				{Name: "HF_HUB_DISABLE_XET", Value: "1"},
				{Name: "S3_ENDPOINT", Value: endpoint},
				{Name: "S3_BUCKET", Value: bucket},
				{Name: "S3_ACCESS_KEY", Value: accessKey},
				{Name: "S3_SECRET_KEY", Value: secretKey},
			},
			VolumeMounts: []corev1.VolumeMount{
				{Name: "model-cache", MountPath: "/models"},
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("2Gi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("8Gi"),
				},
			},
		},
	}
}

func buildTolerations(md *v1alpha1.ModelDeployment) []corev1.Toleration {
	if md.Spec.Resources.GPU <= 0 {
		return nil
	}
	key := string(gpuResourceName(md.Spec.Accelerator))
	return []corev1.Toleration{
		{Key: key, Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule},
	}
}

func boolPtr(b bool) *bool     { return &b }
func int64Ptr(i int64) *int64  { return &i }

func intOrString(port int32) intstr {
	return intstr{IntVal: port}
}
```

**Note:** The `intOrString` helper needs the `intstr` import. Replace the function and add the import:

At the top of the file, add to imports:
```go
"k8s.io/apimachinery/pkg/util/intstr"
```

Replace the `intOrString` function and all usages with direct `intstr.FromInt32(port)`:

```go
// In BuildDeployment, replace intOrString(port) with:
intstr.FromInt32(port)
```

Remove the `intOrString` helper function entirely.

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/rui/kube-llmops/operator
go test ./internal/builder/ -v -run TestBuildDeployment
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/rui/kube-llmops
git add operator/internal/builder/
git commit -m "feat(operator): deployment builder for ModelDeployment (vllm/tei/llamacpp)"
```

---

## Task 7: Service & PVC Builders

**Files:**
- Create: `operator/internal/builder/service.go`
- Create: `operator/internal/builder/pvc.go`
- Create: `operator/internal/builder/service_pvc_test.go`

- [ ] **Step 1: Write failing tests**

Create `operator/internal/builder/service_pvc_test.go`:

```go
package builder

import (
	"testing"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildService(t *testing.T) {
	md := &v1alpha1.ModelDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: "qwen", Namespace: "default"},
	}

	svc := BuildService(md, "vllm")

	if svc.Name != "qwen" {
		t.Errorf("Name = %q, want %q", svc.Name, "qwen")
	}
	if svc.Spec.Ports[0].Port != 8000 {
		t.Errorf("Port = %d, want 8000", svc.Spec.Ports[0].Port)
	}

	svcTei := BuildService(md, "tei")
	if svcTei.Spec.Ports[0].Port != 8080 {
		t.Errorf("TEI Port = %d, want 8080", svcTei.Spec.Ports[0].Port)
	}
}

func TestBuildPVC(t *testing.T) {
	md := &v1alpha1.ModelDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: "qwen", Namespace: "default"},
	}

	pvc := BuildPVC(md, "vllm")

	if pvc.Name != "qwen-cache" {
		t.Errorf("Name = %q, want %q", pvc.Name, "qwen-cache")
	}
	storage := pvc.Spec.Resources.Requests["storage"]
	if storage.String() != "50Gi" {
		t.Errorf("Storage = %q, want 50Gi", storage.String())
	}

	pvcTei := BuildPVC(md, "tei")
	storageTei := pvcTei.Spec.Resources.Requests["storage"]
	if storageTei.String() != "10Gi" {
		t.Errorf("TEI Storage = %q, want 10Gi", storageTei.String())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/rui/kube-llmops/operator
go test ./internal/builder/ -v -run "TestBuildService|TestBuildPVC"
```

- [ ] **Step 3: Implement BuildService**

Create `operator/internal/builder/service.go`:

```go
package builder

import (
	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// BuildService creates a ClusterIP Service for a ModelDeployment.
func BuildService(md *v1alpha1.ModelDeployment, engine string) *corev1.Service {
	port := EnginePorts[engine]

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      md.Name,
			Namespace: md.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":    engine,
				"app.kubernetes.io/instance": md.Name,
				"app.kubernetes.io/part-of":  "kube-llmops",
				"kube-llmops/model":          md.Name,
				"kube-llmops/engine":         engine,
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"app.kubernetes.io/name": engine,
				"kube-llmops/model":      md.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       port,
					TargetPort: intstr.FromInt32(port),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}
```

- [ ] **Step 4: Implement BuildPVC**

Create `operator/internal/builder/pvc.go`:

```go
package builder

import (
	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Default cache sizes per engine (matching Helm chart defaults).
var EngineCacheSizes = map[string]string{
	"vllm":     "50Gi",
	"tei":      "10Gi",
	"llamacpp": "30Gi",
}

// BuildPVC creates a PersistentVolumeClaim for model cache.
func BuildPVC(md *v1alpha1.ModelDeployment, engine string) *corev1.PersistentVolumeClaim {
	size := EngineCacheSizes[engine]
	if size == "" {
		size = "50Gi"
	}

	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      md.Name + "-cache",
			Namespace: md.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":    engine,
				"app.kubernetes.io/instance": md.Name,
				"app.kubernetes.io/part-of":  "kube-llmops",
				"kube-llmops/model":          md.Name,
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(size),
				},
			},
		},
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd /home/rui/kube-llmops/operator
go test ./internal/builder/ -v -run "TestBuildService|TestBuildPVC"
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
cd /home/rui/kube-llmops
git add operator/internal/builder/
git commit -m "feat(operator): service and PVC builders for ModelDeployment"
```

---

## Task 8: Gateway Client (LiteLLM)

**Files:**
- Create: `operator/internal/gateway/client.go`
- Create: `operator/internal/gateway/client_test.go`

- [ ] **Step 1: Write failing tests with httptest mock**

Create `operator/internal/gateway/client_test.go`:

```go
package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegisterModel(t *testing.T) {
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/model/new" || r.Method != http.MethodPost {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "model added"}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "test-key")
	err := c.RegisterModel(context.Background(), GatewayModel{
		ModelName: "qwen",
		LiteLLMParams: LiteLLMParams{
			Model:   "openai/qwen",
			APIBase: "http://qwen:8000/v1",
		},
	})
	if err != nil {
		t.Fatalf("RegisterModel failed: %v", err)
	}
	if gotBody["model_name"] != "qwen" {
		t.Errorf("model_name = %v, want qwen", gotBody["model_name"])
	}
}

func TestDeregisterModel(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/model/delete" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "")
	err := c.DeregisterModel(context.Background(), "qwen")
	if err != nil {
		t.Fatalf("DeregisterModel failed: %v", err)
	}
	if !called {
		t.Error("delete endpoint not called")
	}
}

func TestHealthCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "")
	err := c.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("HealthCheck failed: %v", err)
	}
}

func TestHealthCheck_Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "")
	err := c.HealthCheck(context.Background())
	if err == nil {
		t.Error("expected error for unhealthy gateway")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/rui/kube-llmops/operator
go test ./internal/gateway/ -v
```

- [ ] **Step 3: Implement the gateway client**

Create `operator/internal/gateway/client.go`:

```go
package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client defines the interface for communicating with the LiteLLM gateway.
type Client interface {
	RegisterModel(ctx context.Context, model GatewayModel) error
	DeregisterModel(ctx context.Context, modelID string) error
	HealthCheck(ctx context.Context) error
}

// GatewayModel represents a model registration request to LiteLLM.
type GatewayModel struct {
	ModelName     string        `json:"model_name"`
	LiteLLMParams LiteLLMParams `json:"litellm_params"`
}

// LiteLLMParams holds the LiteLLM-specific model configuration.
type LiteLLMParams struct {
	Model   string `json:"model"`
	APIBase string `json:"api_base"`
	APIKey  string `json:"api_key,omitempty"`
}

// HTTPClient implements Client using HTTP.
type HTTPClient struct {
	baseURL    string
	masterKey  string
	httpClient *http.Client
}

// NewHTTPClient creates a new LiteLLM gateway HTTP client.
func NewHTTPClient(baseURL, masterKey string) *HTTPClient {
	return &HTTPClient{
		baseURL:   baseURL,
		masterKey: masterKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *HTTPClient) RegisterModel(ctx context.Context, model GatewayModel) error {
	body, err := json.Marshal(model)
	if err != nil {
		return fmt.Errorf("marshal model: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/model/new", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.masterKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.masterKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("register model: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("register model: HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func (c *HTTPClient) DeregisterModel(ctx context.Context, modelID string) error {
	body, _ := json.Marshal(map[string]string{"id": modelID})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/model/delete", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.masterKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.masterKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("deregister model: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("deregister model: HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func (c *HTTPClient) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("gateway unhealthy: HTTP %d", resp.StatusCode)
	}
	return nil
}

// NoopClient is a no-op implementation for testing controllers without a real gateway.
type NoopClient struct{}

func (n *NoopClient) RegisterModel(ctx context.Context, model GatewayModel) error   { return nil }
func (n *NoopClient) DeregisterModel(ctx context.Context, modelID string) error      { return nil }
func (n *NoopClient) HealthCheck(ctx context.Context) error                          { return nil }
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/rui/kube-llmops/operator
go test ./internal/gateway/ -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/rui/kube-llmops
git add operator/internal/gateway/
git commit -m "feat(operator): LiteLLM gateway client with register/deregister/health"
```

---

## Task 9: ModelDeployment Controller

**Files:**
- Modify: `operator/internal/controller/modeldeployment_controller.go`
- Create: `operator/internal/controller/suite_test.go` (if not already created by scaffold)
- Modify: `operator/internal/controller/modeldeployment_controller_test.go`

- [ ] **Step 1: Set up the envtest suite**

Replace `operator/internal/controller/suite_test.go` with:

```go
package controller

import (
	"context"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	"github.com/kube-llmops/operator/internal/gateway"
)

var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
	ctx       context.Context
	cancel    context.CancelFunc
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = v1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())

	// Start controller manager
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())

	err = (&ModelDeploymentReconciler{
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		GatewayClient:  &gateway.NoopClient{},
	}).SetupWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = mgr.Start(ctx)
		Expect(err).NotTo(HaveOccurred())
	}()
})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
```

- [ ] **Step 2: Write controller integration tests**

Replace `operator/internal/controller/modeldeployment_controller_test.go` with:

```go
package controller

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
)

var _ = Describe("ModelDeployment Controller", func() {
	const (
		timeout  = 30 * time.Second
		interval = 250 * time.Millisecond
	)

	Context("When creating a ModelDeployment", func() {
		It("Should create a Deployment and Service", func() {
			md := &v1alpha1.ModelDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-model",
					Namespace: "default",
				},
				Spec: v1alpha1.ModelDeploymentSpec{
					Source:   "Qwen/Qwen2.5-7B-Instruct",
					Engine:   "vllm",
					Replicas: ptr.To(int32(1)),
					Resources: v1alpha1.ModelResources{
						GPU:    1,
						Memory: "16Gi",
						CPU:    "4",
					},
				},
			}
			Expect(k8sClient.Create(ctx, md)).Should(Succeed())

			// Verify Deployment is created
			depKey := types.NamespacedName{Name: "test-model", Namespace: "default"}
			createdDep := &appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, depKey, createdDep)
			}, timeout, interval).Should(Succeed())

			Expect(*createdDep.Spec.Replicas).To(Equal(int32(1)))
			Expect(createdDep.Spec.Template.Spec.Containers[0].Image).To(Equal("vllm/vllm-openai:latest"))

			// Verify Service is created
			svcKey := types.NamespacedName{Name: "test-model", Namespace: "default"}
			createdSvc := &corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(ctx, svcKey, createdSvc)
			}, timeout, interval).Should(Succeed())

			Expect(createdSvc.Spec.Ports[0].Port).To(Equal(int32(8000)))

			// Verify PVC is created
			pvcKey := types.NamespacedName{Name: "test-model-cache", Namespace: "default"}
			createdPVC := &corev1.PersistentVolumeClaim{}
			Eventually(func() error {
				return k8sClient.Get(ctx, pvcKey, createdPVC)
			}, timeout, interval).Should(Succeed())

			// Verify status is updated
			Eventually(func() string {
				updatedMD := &v1alpha1.ModelDeployment{}
				k8sClient.Get(ctx, depKey, updatedMD)
				return updatedMD.Status.Engine
			}, timeout, interval).Should(Equal("vllm"))
		})
	})
})
```

- [ ] **Step 3: Implement the ModelDeployment controller**

Replace `operator/internal/controller/modeldeployment_controller.go` with:

```go
package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	"github.com/kube-llmops/operator/internal/builder"
	"github.com/kube-llmops/operator/internal/engine"
	"github.com/kube-llmops/operator/internal/gateway"
)

const modelDeploymentFinalizer = "llmops.kubellmops.io/finalizer"

// ModelDeploymentReconciler reconciles a ModelDeployment object.
type ModelDeploymentReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	GatewayClient gateway.Client
}

// +kubebuilder:rbac:groups=llmops.kubellmops.io,resources=modeldeployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=llmops.kubellmops.io,resources=modeldeployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=llmops.kubellmops.io,resources=modeldeployments/finalizers,verbs=update
// +kubebuilder:rbac:groups=llmops.kubellmops.io,resources=llmplatforms,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete

func (r *ModelDeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the ModelDeployment
	md := &v1alpha1.ModelDeployment{}
	if err := r.Get(ctx, req.NamespacedName, md); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !md.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, md)
	}

	// Add finalizer
	if !controllerutil.ContainsFinalizer(md, modelDeploymentFinalizer) {
		controllerutil.AddFinalizer(md, modelDeploymentFinalizer)
		if err := r.Update(ctx, md); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Resolve engine
	resolvedEngine := engine.ResolveEngine(md.Spec.Source, md.Spec.Engine)
	log.Info("Resolved engine", "engine", resolvedEngine, "source", md.Spec.Source)

	// Look up LLMPlatform for model store config
	platform := r.findPlatform(ctx, md.Namespace)

	// Update phase to Deploying
	r.setPhase(ctx, md, "Deploying")

	// Ensure PVC
	desiredPVC := builder.BuildPVC(md, resolvedEngine)
	if err := controllerutil.SetControllerReference(md, desiredPVC, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	existingPVC := &corev1.PersistentVolumeClaim{}
	if err := r.Get(ctx, types.NamespacedName{Name: desiredPVC.Name, Namespace: desiredPVC.Namespace}, existingPVC); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Creating PVC", "name", desiredPVC.Name)
			if err := r.Create(ctx, desiredPVC); err != nil {
				return ctrl.Result{}, err
			}
		} else {
			return ctrl.Result{}, err
		}
	}

	// Ensure Deployment
	desiredDep := builder.BuildDeployment(md, resolvedEngine, platform)
	if err := controllerutil.SetControllerReference(md, desiredDep, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	existingDep := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Name: desiredDep.Name, Namespace: desiredDep.Namespace}, existingDep); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Creating Deployment", "name", desiredDep.Name)
			if err := r.Create(ctx, desiredDep); err != nil {
				return ctrl.Result{}, err
			}
		} else {
			return ctrl.Result{}, err
		}
	} else {
		// Update existing deployment
		existingDep.Spec = desiredDep.Spec
		if err := r.Update(ctx, existingDep); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Ensure Service
	desiredSvc := builder.BuildService(md, resolvedEngine)
	if err := controllerutil.SetControllerReference(md, desiredSvc, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	existingSvc := &corev1.Service{}
	if err := r.Get(ctx, types.NamespacedName{Name: desiredSvc.Name, Namespace: desiredSvc.Namespace}, existingSvc); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Creating Service", "name", desiredSvc.Name)
			if err := r.Create(ctx, desiredSvc); err != nil {
				return ctrl.Result{}, err
			}
		} else {
			return ctrl.Result{}, err
		}
	}

	// Update status
	return r.updateStatus(ctx, md, resolvedEngine)
}

func (r *ModelDeploymentReconciler) handleDeletion(ctx context.Context, md *v1alpha1.ModelDeployment) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(md, modelDeploymentFinalizer) {
		// Deregister from gateway
		if r.GatewayClient != nil {
			log.Info("Deregistering model from gateway", "model", md.Name)
			if err := r.GatewayClient.DeregisterModel(ctx, md.Name); err != nil {
				log.Error(err, "Failed to deregister model (non-fatal)")
			}
		}

		controllerutil.RemoveFinalizer(md, modelDeploymentFinalizer)
		if err := r.Update(ctx, md); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *ModelDeploymentReconciler) findPlatform(ctx context.Context, namespace string) *v1alpha1.LLMPlatform {
	platformList := &v1alpha1.LLMPlatformList{}
	if err := r.List(ctx, platformList, client.InNamespace(namespace)); err != nil {
		return nil
	}
	if len(platformList.Items) > 0 {
		return &platformList.Items[0]
	}
	return nil
}

func (r *ModelDeploymentReconciler) setPhase(ctx context.Context, md *v1alpha1.ModelDeployment, phase string) {
	md.Status.Phase = phase
	r.Status().Update(ctx, md)
}

func (r *ModelDeploymentReconciler) updateStatus(ctx context.Context, md *v1alpha1.ModelDeployment, resolvedEngine string) (ctrl.Result, error) {
	// Read current deployment status
	dep := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Name: md.Name, Namespace: md.Namespace}, dep); err != nil {
		return ctrl.Result{}, err
	}

	port := builder.EnginePorts[resolvedEngine]
	replicas := int32(1)
	if md.Spec.Replicas != nil {
		replicas = *md.Spec.Replicas
	}

	md.Status.Engine = resolvedEngine
	md.Status.ReadyReplicas = dep.Status.ReadyReplicas
	md.Status.TotalReplicas = replicas
	md.Status.Endpoint = fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", md.Name, md.Namespace, port)

	if dep.Status.ReadyReplicas >= replicas && replicas > 0 {
		md.Status.Phase = "Ready"
		meta.SetStatusCondition(&md.Status.Conditions, metav1.Condition{
			Type:               "Available",
			Status:             metav1.ConditionTrue,
			Reason:             "DeploymentReady",
			Message:            fmt.Sprintf("Deployment has %d/%d ready replicas", dep.Status.ReadyReplicas, replicas),
			LastTransitionTime: metav1.Now(),
		})

		// Register with gateway
		if r.GatewayClient != nil {
			prefix := "openai"
			if resolvedEngine == "tei" {
				prefix = "huggingface"
			}
			r.GatewayClient.RegisterModel(ctx, gateway.GatewayModel{
				ModelName: md.Name,
				LiteLLMParams: gateway.LiteLLMParams{
					Model:   fmt.Sprintf("%s/%s", prefix, md.Name),
					APIBase: md.Status.Endpoint,
					APIKey:  "no-key-needed",
				},
			})
			meta.SetStatusCondition(&md.Status.Conditions, metav1.Condition{
				Type:               "GatewayRegistered",
				Status:             metav1.ConditionTrue,
				Reason:             "Registered",
				Message:            "Model registered with LiteLLM gateway",
				LastTransitionTime: metav1.Now(),
			})
		}
	} else if dep.Status.ReadyReplicas > 0 {
		md.Status.Phase = "Degraded"
	} else {
		md.Status.Phase = "Deploying"
	}

	if err := r.Status().Update(ctx, md); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ModelDeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ModelDeployment{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}
```

- [ ] **Step 4: Install envtest binaries**

```bash
cd /home/rui/kube-llmops/operator
setup-envtest use 1.28.0
```

- [ ] **Step 5: Run envtest integration tests**

```bash
cd /home/rui/kube-llmops/operator
KUBEBUILDER_ASSETS=$(setup-envtest use 1.28.0 -p path) go test ./internal/controller/ -v -ginkgo.v
```

Expected: tests PASS (Deployment, Service, PVC created; status updated).

- [ ] **Step 6: Commit**

```bash
cd /home/rui/kube-llmops
git add operator/internal/controller/
git commit -m "feat(operator): ModelDeployment controller with reconciliation loop"
```

---

## Task 10: Helm Values Translator

**Files:**
- Create: `operator/internal/helmbridge/values.go`
- Create: `operator/internal/helmbridge/values_test.go`

- [ ] **Step 1: Write failing tests**

Create `operator/internal/helmbridge/values_test.go`:

```go
package helmbridge

import (
	"testing"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTranslateValues_Gateway(t *testing.T) {
	platform := &v1alpha1.LLMPlatform{
		ObjectMeta: metav1.ObjectMeta{Name: "kube-llmops"},
		Spec: v1alpha1.LLMPlatformSpec{
			Gateway: v1alpha1.GatewaySpec{
				Enabled: true,
				Routing: "latency-based-routing",
			},
		},
	}
	vals := TranslateValues(platform)

	litellm, ok := vals["litellm"].(map[string]interface{})
	if !ok {
		t.Fatal("litellm key missing")
	}
	if litellm["enabled"] != true {
		t.Error("litellm.enabled should be true")
	}
	if litellm["routingStrategy"] != "latency-based-routing" {
		t.Errorf("routingStrategy = %v", litellm["routingStrategy"])
	}
}

func TestTranslateValues_Modules(t *testing.T) {
	platform := &v1alpha1.LLMPlatform{
		ObjectMeta: metav1.ObjectMeta{Name: "kube-llmops"},
		Spec: v1alpha1.LLMPlatformSpec{
			Modules: v1alpha1.ModulesSpec{
				RAG:      v1alpha1.EnabledToggle{Enabled: true},
				Finetune: v1alpha1.EnabledToggle{Enabled: false},
				Security: v1alpha1.EnabledToggle{Enabled: false},
			},
		},
	}
	vals := TranslateValues(platform)

	global, ok := vals["global"].(map[string]interface{})
	if !ok {
		t.Fatal("global key missing")
	}
	modules := global["modules"].(map[string]interface{})
	rag := modules["rag"].(map[string]interface{})
	if rag["enabled"] != true {
		t.Error("global.modules.rag.enabled should be true")
	}
}

func TestTranslateValues_NodePort(t *testing.T) {
	platform := &v1alpha1.LLMPlatform{
		ObjectMeta: metav1.ObjectMeta{Name: "kube-llmops"},
		Spec: v1alpha1.LLMPlatformSpec{
			NodePort: v1alpha1.NodePortSpec{
				Enabled: true,
				Host:    "172.29.193.187",
			},
		},
	}
	vals := TranslateValues(platform)

	global := vals["global"].(map[string]interface{})
	np := global["nodePort"].(map[string]interface{})
	if np["enabled"] != true {
		t.Error("nodePort.enabled should be true")
	}
	if np["host"] != "172.29.193.187" {
		t.Errorf("nodePort.host = %v", np["host"])
	}
}

func TestTranslateValues_ModelStore(t *testing.T) {
	platform := &v1alpha1.LLMPlatform{
		ObjectMeta: metav1.ObjectMeta{Name: "kube-llmops"},
		Spec: v1alpha1.LLMPlatformSpec{
			ModelStore: v1alpha1.ModelStoreSpec{
				Enabled:  true,
				Endpoint: "minio:9000",
				Bucket:   "models",
			},
		},
	}
	vals := TranslateValues(platform)

	global := vals["global"].(map[string]interface{})
	ms := global["modelStore"].(map[string]interface{})
	if ms["endpoint"] != "minio:9000" {
		t.Errorf("modelStore.endpoint = %v", ms["endpoint"])
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/rui/kube-llmops/operator
go test ./internal/helmbridge/ -v
```

- [ ] **Step 3: Implement TranslateValues**

Create `operator/internal/helmbridge/values.go`:

```go
package helmbridge

import (
	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
)

// TranslateValues converts an LLMPlatform CR spec into a Helm values map
// compatible with charts/kube-llmops-stack.
func TranslateValues(platform *v1alpha1.LLMPlatform) map[string]interface{} {
	vals := map[string]interface{}{}

	// Global section
	global := map[string]interface{}{}

	// Modules
	global["modules"] = map[string]interface{}{
		"rag":      map[string]interface{}{"enabled": platform.Spec.Modules.RAG.Enabled},
		"finetune": map[string]interface{}{"enabled": platform.Spec.Modules.Finetune.Enabled},
		"security": map[string]interface{}{"enabled": platform.Spec.Modules.Security.Enabled},
	}

	// ModelStore
	ms := platform.Spec.ModelStore
	global["modelStore"] = map[string]interface{}{
		"endpoint":             ms.Endpoint,
		"bucket":               ms.Bucket,
		"accessKey":            ms.AccessKey,
		"secretKey":            ms.SecretKey,
		"hfTransferConcurrency": ms.HFTransferConcurrency,
		"image":                ms.Image,
	}

	// HF Token
	if platform.Spec.HFToken != "" {
		global["hfToken"] = platform.Spec.HFToken
	}

	// NodePort
	global["nodePort"] = map[string]interface{}{
		"enabled": platform.Spec.NodePort.Enabled,
		"host":    platform.Spec.NodePort.Host,
	}

	vals["global"] = global

	// LiteLLM (gateway)
	gw := platform.Spec.Gateway
	vals["litellm"] = map[string]interface{}{
		"enabled":         gw.Enabled,
		"routingStrategy": gw.Routing,
	}
	if gw.Image.Tag != "" {
		litellm := vals["litellm"].(map[string]interface{})
		litellm["image"] = map[string]interface{}{"tag": gw.Image.Tag}
	}

	// Observability
	obs := platform.Spec.Observability
	vals["observability"] = map[string]interface{}{
		"enabled": obs.Enabled,
	}
	vals["langfuse"] = map[string]interface{}{
		"enabled": obs.Langfuse.Enabled,
	}

	// Logging
	vals["logging"] = map[string]interface{}{
		"enabled": platform.Spec.Logging.Enabled,
	}

	// Keycloak
	vals["keycloak"] = map[string]interface{}{
		"enabled": platform.Spec.Keycloak.Enabled,
	}

	// PostgreSQL
	vals["postgresql"] = map[string]interface{}{
		"enabled": platform.Spec.PostgreSQL.Enabled,
	}

	// KEDA
	vals["keda"] = map[string]interface{}{
		"enabled": platform.Spec.KEDA.Enabled,
	}

	// Fluid (MinIO)
	vals["fluid"] = map[string]interface{}{
		"enabled": ms.Enabled,
	}

	// Ingress
	if platform.Spec.Ingress.Enabled {
		vals["ingress"] = map[string]interface{}{
			"enabled":   true,
			"className": platform.Spec.Ingress.ClassName,
			"host":      platform.Spec.Ingress.Host,
		}
	}

	return vals
}
```

- [ ] **Step 4: Run tests**

```bash
cd /home/rui/kube-llmops/operator
go test ./internal/helmbridge/ -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/rui/kube-llmops
git add operator/internal/helmbridge/
git commit -m "feat(operator): Helm values translator (LLMPlatform CR → Helm values)"
```

---

## Task 11: Helm SDK Client

**Files:**
- Create: `operator/internal/helmbridge/client.go`

- [ ] **Step 1: Implement the Helm SDK client**

Create `operator/internal/helmbridge/client.go`:

```go
package helmbridge

import (
	"fmt"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
)

// HelmClient defines the interface for Helm operations.
type HelmClient interface {
	Install(name, namespace, chartPath string, values map[string]interface{}) (*release.Release, error)
	Upgrade(name, namespace, chartPath string, values map[string]interface{}) (*release.Release, error)
	GetRelease(name, namespace string) (*release.Release, error)
	Uninstall(name, namespace string) error
}

// SDKClient implements HelmClient using the Helm SDK.
type SDKClient struct{}

func NewSDKClient() *SDKClient {
	return &SDKClient{}
}

func (c *SDKClient) actionConfig(namespace string) (*action.Configuration, error) {
	settings := cli.New()
	settings.SetNamespace(namespace)
	cfg := new(action.Configuration)
	if err := cfg.Init(settings.RESTClientGetter(), namespace, "secret", func(format string, v ...interface{}) {}); err != nil {
		return nil, fmt.Errorf("init helm config: %w", err)
	}
	return cfg, nil
}

func (c *SDKClient) Install(name, namespace, chartPath string, values map[string]interface{}) (*release.Release, error) {
	cfg, err := c.actionConfig(namespace)
	if err != nil {
		return nil, err
	}
	chart, err := loader.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("load chart: %w", err)
	}
	install := action.NewInstall(cfg)
	install.ReleaseName = name
	install.Namespace = namespace
	install.CreateNamespace = false
	install.Wait = false
	return install.Run(chart, values)
}

func (c *SDKClient) Upgrade(name, namespace, chartPath string, values map[string]interface{}) (*release.Release, error) {
	cfg, err := c.actionConfig(namespace)
	if err != nil {
		return nil, err
	}
	chart, err := loader.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("load chart: %w", err)
	}
	upgrade := action.NewUpgrade(cfg)
	upgrade.Namespace = namespace
	upgrade.Wait = false
	return upgrade.Run(name, chart, values)
}

func (c *SDKClient) GetRelease(name, namespace string) (*release.Release, error) {
	cfg, err := c.actionConfig(namespace)
	if err != nil {
		return nil, err
	}
	get := action.NewGet(cfg)
	return get.Run(name)
}

func (c *SDKClient) Uninstall(name, namespace string) error {
	cfg, err := c.actionConfig(namespace)
	if err != nil {
		return err
	}
	uninstall := action.NewUninstall(cfg)
	_, err = uninstall.Run(name)
	return err
}

// MockHelmClient is a test double for unit tests.
type MockHelmClient struct {
	LastValues map[string]interface{}
	InstallErr error
	UpgradeErr error
}

func (m *MockHelmClient) Install(name, namespace, chartPath string, values map[string]interface{}) (*release.Release, error) {
	m.LastValues = values
	if m.InstallErr != nil {
		return nil, m.InstallErr
	}
	return &release.Release{Name: name, Version: 1}, nil
}

func (m *MockHelmClient) Upgrade(name, namespace, chartPath string, values map[string]interface{}) (*release.Release, error) {
	m.LastValues = values
	if m.UpgradeErr != nil {
		return nil, m.UpgradeErr
	}
	return &release.Release{Name: name, Version: 2}, nil
}

func (m *MockHelmClient) GetRelease(name, namespace string) (*release.Release, error) {
	return &release.Release{Name: name, Version: 1}, nil
}

func (m *MockHelmClient) Uninstall(name, namespace string) error {
	return nil
}
```

- [ ] **Step 2: Add Helm SDK dependency**

```bash
cd /home/rui/kube-llmops/operator
go get helm.sh/helm/v3
go mod tidy
```

- [ ] **Step 3: Verify compilation**

```bash
cd /home/rui/kube-llmops/operator
go build ./...
```

- [ ] **Step 4: Commit**

```bash
cd /home/rui/kube-llmops
git add operator/internal/helmbridge/client.go operator/go.mod operator/go.sum
git commit -m "feat(operator): Helm SDK client for LLMPlatform controller"
```

---

## Task 12: LLMPlatform Controller

**Files:**
- Modify: `operator/internal/controller/llmplatform_controller.go`
- Modify: `operator/internal/controller/llmplatform_controller_test.go`

- [ ] **Step 1: Write envtest integration test**

Replace `operator/internal/controller/llmplatform_controller_test.go`:

```go
package controller

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
)

var _ = Describe("LLMPlatform Controller", func() {
	const (
		timeout  = 30 * time.Second
		interval = 250 * time.Millisecond
	)

	Context("When creating an LLMPlatform", func() {
		It("Should update status phase", func() {
			platform := &v1alpha1.LLMPlatform{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-platform",
					Namespace: "default",
				},
				Spec: v1alpha1.LLMPlatformSpec{
					Gateway: v1alpha1.GatewaySpec{Enabled: true},
					Observability: v1alpha1.ObservabilitySpec{Enabled: true},
					ModelStore: v1alpha1.ModelStoreSpec{
						Enabled:  true,
						Endpoint: "minio:9000",
						Bucket:   "models",
					},
				},
			}
			Expect(k8sClient.Create(ctx, platform)).Should(Succeed())

			// Verify status is updated
			key := types.NamespacedName{Name: "test-platform", Namespace: "default"}
			Eventually(func() string {
				p := &v1alpha1.LLMPlatform{}
				k8sClient.Get(ctx, key, p)
				return p.Status.Phase
			}, timeout, interval).ShouldNot(BeEmpty())
		})
	})
})
```

- [ ] **Step 2: Implement the LLMPlatform controller**

Replace `operator/internal/controller/llmplatform_controller.go`:

```go
package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	"github.com/kube-llmops/operator/internal/helmbridge"
)

// LLMPlatformReconciler reconciles an LLMPlatform object.
type LLMPlatformReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	HelmClient helmbridge.HelmClient
	ChartPath  string // path to charts/kube-llmops-stack
}

// +kubebuilder:rbac:groups=llmops.kubellmops.io,resources=llmplatforms,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=llmops.kubellmops.io,resources=llmplatforms/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=llmops.kubellmops.io,resources=llmplatforms/finalizers,verbs=update

func (r *LLMPlatformReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	platform := &v1alpha1.LLMPlatform{}
	if err := r.Get(ctx, req.NamespacedName, platform); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Translate CR spec to Helm values
	values := helmbridge.TranslateValues(platform)
	releaseName := "kube-llmops"

	// Check if release exists
	existing, err := r.HelmClient.GetRelease(releaseName, platform.Namespace)
	if err != nil {
		// Release doesn't exist → install
		log.Info("Installing Helm release", "name", releaseName)
		platform.Status.Phase = "Installing"
		r.Status().Update(ctx, platform)

		rel, err := r.HelmClient.Install(releaseName, platform.Namespace, r.ChartPath, values)
		if err != nil {
			platform.Status.Phase = "Failed"
			meta.SetStatusCondition(&platform.Status.Conditions, metav1.Condition{
				Type:               "HelmReleaseReady",
				Status:             metav1.ConditionFalse,
				Reason:             "InstallFailed",
				Message:            err.Error(),
				LastTransitionTime: metav1.Now(),
			})
			r.Status().Update(ctx, platform)
			return ctrl.Result{}, fmt.Errorf("helm install: %w", err)
		}
		platform.Status.HelmRelease = rel.Name
		platform.Status.HelmRevision = rel.Version
	} else {
		// Release exists → upgrade
		log.Info("Upgrading Helm release", "name", releaseName, "revision", existing.Version)
		platform.Status.Phase = "Upgrading"
		r.Status().Update(ctx, platform)

		rel, err := r.HelmClient.Upgrade(releaseName, platform.Namespace, r.ChartPath, values)
		if err != nil {
			platform.Status.Phase = "Failed"
			meta.SetStatusCondition(&platform.Status.Conditions, metav1.Condition{
				Type:               "HelmReleaseReady",
				Status:             metav1.ConditionFalse,
				Reason:             "UpgradeFailed",
				Message:            err.Error(),
				LastTransitionTime: metav1.Now(),
			})
			r.Status().Update(ctx, platform)
			return ctrl.Result{}, fmt.Errorf("helm upgrade: %w", err)
		}
		platform.Status.HelmRelease = rel.Name
		platform.Status.HelmRevision = rel.Version
	}

	// Set status to Ready
	platform.Status.Phase = "Ready"
	meta.SetStatusCondition(&platform.Status.Conditions, metav1.Condition{
		Type:               "HelmReleaseReady",
		Status:             metav1.ConditionTrue,
		Reason:             "Released",
		Message:            fmt.Sprintf("Helm release %s revision %d deployed", releaseName, platform.Status.HelmRevision),
		LastTransitionTime: metav1.Now(),
	})

	if err := r.Status().Update(ctx, platform); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *LLMPlatformReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.LLMPlatform{}).
		Complete(r)
}
```

- [ ] **Step 3: Register LLMPlatform controller in suite_test.go**

Add to `suite_test.go` BeforeSuite (after ModelDeployment controller registration):

```go
	err = (&LLMPlatformReconciler{
		Client:     mgr.GetClient(),
		Scheme:     mgr.GetScheme(),
		HelmClient: &helmbridge.MockHelmClient{},
		ChartPath:  "/nonexistent", // mock doesn't use chartPath
	}).SetupWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())
```

Add the import:
```go
	"github.com/kube-llmops/operator/internal/helmbridge"
```

- [ ] **Step 4: Run envtest**

```bash
cd /home/rui/kube-llmops/operator
KUBEBUILDER_ASSETS=$(setup-envtest use 1.28.0 -p path) go test ./internal/controller/ -v -ginkgo.v
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/rui/kube-llmops
git add operator/internal/controller/ operator/internal/helmbridge/
git commit -m "feat(operator): LLMPlatform controller with Helm SDK bridge"
```

---

## Task 13: Argo Workflow Builder

**Files:**
- Create: `operator/internal/builder/workflow.go`
- Create: `operator/internal/builder/workflow_test.go`

- [ ] **Step 1: Write failing tests**

Create `operator/internal/builder/workflow_test.go`:

```go
package builder

import (
	"testing"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildArgoWorkflow(t *testing.T) {
	ftr := &v1alpha1.FineTuneRun{
		ObjectMeta: metav1.ObjectMeta{Name: "gemma-lora-v1", Namespace: "default"},
		Spec: v1alpha1.FineTuneRunSpec{
			BaseModel:  "cyankiwi/gemma-4-26B-A4B-it-AWQ-4bit",
			OutputName: "gemma-4-lora-v1",
			Method:     "lora",
			DataSource: v1alpha1.DataSourceSpec{Type: "minio", Path: "s3://datasets/my-data/"},
			Training:   v1alpha1.TrainingSpec{Epochs: 3, BatchSize: 4, LearningRate: "2e-4", LoraRank: 16, LoraAlpha: 32},
			Resources:  v1alpha1.ModelResources{GPU: 1, Memory: "24Gi", CPU: "4"},
		},
	}

	wf := BuildArgoWorkflow(ftr, "kube-llmops")

	// Verify it's an Argo Workflow
	if wf.GetKind() != "Workflow" {
		t.Errorf("Kind = %q, want Workflow", wf.GetKind())
	}
	if wf.GetAPIVersion() != "argoproj.io/v1alpha1" {
		t.Errorf("APIVersion = %q", wf.GetAPIVersion())
	}
	if wf.GetName() == "" {
		t.Error("Name should not be empty")
	}
	if wf.GetNamespace() != "default" {
		t.Errorf("Namespace = %q", wf.GetNamespace())
	}

	// Verify DAG tasks exist
	spec, ok := wf.Object["spec"].(map[string]interface{})
	if !ok {
		t.Fatal("spec missing")
	}
	templates, ok := spec["templates"].([]interface{})
	if !ok {
		t.Fatal("templates missing")
	}
	// Should have main DAG + 6 task templates
	if len(templates) < 7 {
		t.Errorf("expected >= 7 templates, got %d", len(templates))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/rui/kube-llmops/operator
go test ./internal/builder/ -v -run TestBuildArgoWorkflow
```

- [ ] **Step 3: Implement BuildArgoWorkflow**

Create `operator/internal/builder/workflow.go`:

```go
package builder

import (
	"fmt"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// BuildArgoWorkflow creates an Argo Workflow (unstructured) for a FineTuneRun.
// Uses unstructured to avoid importing the full Argo dependency tree.
func BuildArgoWorkflow(ftr *v1alpha1.FineTuneRun, releaseName string) *unstructured.Unstructured {
	name := fmt.Sprintf("%s-%s", ftr.Name, ftr.Spec.OutputName[:min(8, len(ftr.Spec.OutputName))])
	mlflowURL := fmt.Sprintf("http://%s-mlflow:5000", releaseName)
	minioEndpoint := fmt.Sprintf("%s-minio:9000", releaseName)

	gpu := int64(ftr.Spec.Resources.GPU)

	wf := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Workflow",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": ftr.Namespace,
				"labels": map[string]interface{}{
					"app.kubernetes.io/name":    "finetune",
					"app.kubernetes.io/part-of": "kube-llmops",
					"kube-llmops/finetunerun":   ftr.Name,
				},
			},
			"spec": map[string]interface{}{
				"serviceAccountName":  fmt.Sprintf("%s-finetune", releaseName),
				"activeDeadlineSeconds": int64(21600),
				"entrypoint":          "main",
				"volumeClaimTemplates": []interface{}{
					map[string]interface{}{
						"metadata": map[string]interface{}{"name": "workspace"},
						"spec": map[string]interface{}{
							"accessModes": []interface{}{"ReadWriteOnce"},
							"resources": map[string]interface{}{
								"requests": map[string]interface{}{
									"storage": "10Gi",
								},
							},
						},
					},
				},
				"templates": []interface{}{
					// Main DAG
					map[string]interface{}{
						"name": "main",
						"dag": map[string]interface{}{
							"tasks": []interface{}{
								map[string]interface{}{"name": "prepare-data", "template": "prepare-data"},
								map[string]interface{}{"name": "finetune", "template": "finetune", "dependencies": []interface{}{"prepare-data"}},
								map[string]interface{}{"name": "merge-upload", "template": "merge-upload", "dependencies": []interface{}{"finetune"}},
								map[string]interface{}{"name": "evaluate", "template": "evaluate", "dependencies": []interface{}{"merge-upload"}},
								map[string]interface{}{"name": "quality-gate", "template": "quality-gate", "dependencies": []interface{}{"evaluate"}},
								map[string]interface{}{"name": "deploy", "template": "deploy", "dependencies": []interface{}{"quality-gate"}},
							},
						},
					},
					// prepare-data
					buildPrepareDataTemplate(ftr, minioEndpoint),
					// finetune
					buildFinetuneTemplate(ftr, mlflowURL, gpu),
					// merge-upload
					buildMergeUploadTemplate(ftr, mlflowURL, minioEndpoint),
					// evaluate
					buildEvaluateTemplate(ftr, releaseName),
					// quality-gate
					buildQualityGateTemplate(ftr, mlflowURL),
					// deploy
					buildDeployTemplate(ftr, releaseName),
				},
			},
		},
	}

	return wf
}

func buildPrepareDataTemplate(ftr *v1alpha1.FineTuneRun, minioEndpoint string) map[string]interface{} {
	return map[string]interface{}{
		"name": "prepare-data",
		"container": map[string]interface{}{
			"image":   "kube-llmops/model-loader:latest",
			"command": []interface{}{"bash", "-c"},
			"args":    []interface{}{fmt.Sprintf("echo 'Preparing data for %s from %s'", ftr.Spec.BaseModel, ftr.Spec.DataSource.Path)},
			"env": []interface{}{
				map[string]interface{}{"name": "S3_ENDPOINT", "value": minioEndpoint},
				map[string]interface{}{"name": "MODEL_SOURCE", "value": ftr.Spec.BaseModel},
			},
			"volumeMounts": []interface{}{
				map[string]interface{}{"name": "workspace", "mountPath": "/workspace"},
			},
		},
	}
}

func buildFinetuneTemplate(ftr *v1alpha1.FineTuneRun, mlflowURL string, gpu int64) map[string]interface{} {
	resources := map[string]interface{}{
		"requests": map[string]interface{}{
			"cpu":    ftr.Spec.Resources.CPU,
			"memory": ftr.Spec.Resources.Memory,
		},
	}
	if gpu > 0 {
		requests := resources["requests"].(map[string]interface{})
		requests["nvidia.com/gpu"] = gpu
		resources["limits"] = map[string]interface{}{
			"nvidia.com/gpu": gpu,
		}
	}

	return map[string]interface{}{
		"name": "finetune",
		"container": map[string]interface{}{
			"image":   "hiyouga/llamafactory:latest",
			"command": []interface{}{"bash", "-c"},
			"args":    []interface{}{fmt.Sprintf("llamafactory-cli train /workspace/train_config.yaml")},
			"env": []interface{}{
				map[string]interface{}{"name": "MLFLOW_TRACKING_URI", "value": mlflowURL},
				map[string]interface{}{"name": "MLFLOW_EXPERIMENT_NAME", "value": ftr.Spec.OutputName},
			},
			"volumeMounts": []interface{}{
				map[string]interface{}{"name": "workspace", "mountPath": "/workspace"},
			},
			"resources": resources,
		},
	}
}

func buildMergeUploadTemplate(ftr *v1alpha1.FineTuneRun, mlflowURL, minioEndpoint string) map[string]interface{} {
	return map[string]interface{}{
		"name": "merge-upload",
		"container": map[string]interface{}{
			"image":   "kube-llmops/model-loader:latest",
			"command": []interface{}{"bash", "-c"},
			"args":    []interface{}{fmt.Sprintf("echo 'Uploading %s to MinIO'", ftr.Spec.OutputName)},
			"env": []interface{}{
				map[string]interface{}{"name": "MLFLOW_TRACKING_URI", "value": mlflowURL},
				map[string]interface{}{"name": "S3_ENDPOINT", "value": minioEndpoint},
			},
			"volumeMounts": []interface{}{
				map[string]interface{}{"name": "workspace", "mountPath": "/workspace"},
			},
		},
	}
}

func buildEvaluateTemplate(ftr *v1alpha1.FineTuneRun, releaseName string) map[string]interface{} {
	return map[string]interface{}{
		"name": "evaluate",
		"container": map[string]interface{}{
			"image":   "python:3.13-slim",
			"command": []interface{}{"bash", "-c"},
			"args":    []interface{}{"echo 'Evaluating model'"},
			"volumeMounts": []interface{}{
				map[string]interface{}{"name": "workspace", "mountPath": "/workspace"},
			},
		},
	}
}

func buildQualityGateTemplate(ftr *v1alpha1.FineTuneRun, mlflowURL string) map[string]interface{} {
	return map[string]interface{}{
		"name": "quality-gate",
		"container": map[string]interface{}{
			"image":   "python:3.13-slim",
			"command": []interface{}{"bash", "-c"},
			"args":    []interface{}{"echo 'Quality gate check'"},
			"volumeMounts": []interface{}{
				map[string]interface{}{"name": "workspace", "mountPath": "/workspace"},
			},
		},
	}
}

func buildDeployTemplate(ftr *v1alpha1.FineTuneRun, releaseName string) map[string]interface{} {
	return map[string]interface{}{
		"name": "deploy",
		"container": map[string]interface{}{
			"image":   "bitnami/kubectl:latest",
			"command": []interface{}{"bash", "-c"},
			"args":    []interface{}{fmt.Sprintf("echo 'Deploying %s'", ftr.Spec.OutputName)},
			"volumeMounts": []interface{}{
				map[string]interface{}{"name": "workspace", "mountPath": "/workspace"},
			},
		},
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
```

- [ ] **Step 4: Run tests**

```bash
cd /home/rui/kube-llmops/operator
go test ./internal/builder/ -v -run TestBuildArgoWorkflow
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/rui/kube-llmops
git add operator/internal/builder/workflow.go operator/internal/builder/workflow_test.go
git commit -m "feat(operator): Argo Workflow builder for FineTuneRun"
```

---

## Task 14: FineTuneRun Controller

**Files:**
- Modify: `operator/internal/controller/finetunerun_controller.go`
- Modify: `operator/internal/controller/finetunerun_controller_test.go`

- [ ] **Step 1: Write envtest integration test**

Replace `operator/internal/controller/finetunerun_controller_test.go`:

```go
package controller

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
)

var _ = Describe("FineTuneRun Controller", func() {
	const (
		timeout  = 30 * time.Second
		interval = 250 * time.Millisecond
	)

	Context("When creating a FineTuneRun", func() {
		It("Should update phase to Pending", func() {
			ftr := &v1alpha1.FineTuneRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-finetune",
					Namespace: "default",
				},
				Spec: v1alpha1.FineTuneRunSpec{
					BaseModel:  "Qwen/Qwen2.5-7B-Instruct",
					OutputName: "qwen-lora-v1",
					Method:     "lora",
					DataSource: v1alpha1.DataSourceSpec{Type: "minio", Path: "s3://datasets/test/"},
				},
			}
			Expect(k8sClient.Create(ctx, ftr)).Should(Succeed())

			key := types.NamespacedName{Name: "test-finetune", Namespace: "default"}
			Eventually(func() string {
				f := &v1alpha1.FineTuneRun{}
				k8sClient.Get(ctx, key, f)
				return f.Status.Phase
			}, timeout, interval).ShouldNot(BeEmpty())
		})
	})
})
```

- [ ] **Step 2: Implement the FineTuneRun controller**

Replace `operator/internal/controller/finetunerun_controller.go`:

```go
package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	"github.com/kube-llmops/operator/internal/builder"
)

// FineTuneRunReconciler reconciles a FineTuneRun object.
type FineTuneRunReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	ReleaseName string // defaults to "kube-llmops"
}

// +kubebuilder:rbac:groups=llmops.kubellmops.io,resources=finetuneruns,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=llmops.kubellmops.io,resources=finetuneruns/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=llmops.kubellmops.io,resources=finetuneruns/finalizers,verbs=update
// +kubebuilder:rbac:groups=argoproj.io,resources=workflows,verbs=get;list;watch;create;update;patch;delete

func (r *FineTuneRunReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	ftr := &v1alpha1.FineTuneRun{}
	if err := r.Get(ctx, req.NamespacedName, ftr); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Skip if already completed or failed
	if ftr.Status.Phase == "Succeeded" || ftr.Status.Phase == "Failed" {
		return ctrl.Result{}, nil
	}

	releaseName := r.ReleaseName
	if releaseName == "" {
		releaseName = "kube-llmops"
	}

	// Check if Argo Workflow already exists
	if ftr.Status.ArgoWorkflow != "" {
		return r.syncWorkflowStatus(ctx, ftr)
	}

	// Create Argo Workflow
	log.Info("Creating Argo Workflow for FineTuneRun", "name", ftr.Name)
	wf := builder.BuildArgoWorkflow(ftr, releaseName)

	// Try to create the workflow (may fail if Argo CRD not installed)
	if err := r.Create(ctx, wf); err != nil {
		if errors.IsNotFound(err) {
			// Argo CRD not installed
			ftr.Status.Phase = "Failed"
			meta.SetStatusCondition(&ftr.Status.Conditions, metav1.Condition{
				Type:               "WorkflowCreated",
				Status:             metav1.ConditionFalse,
				Reason:             "ArgoCRDMissing",
				Message:            "Argo Workflows CRD not installed in cluster",
				LastTransitionTime: metav1.Now(),
			})
			r.Status().Update(ctx, ftr)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("create workflow: %w", err)
	}

	// Update status
	now := metav1.Now()
	ftr.Status.Phase = "Pending"
	ftr.Status.ArgoWorkflow = wf.GetName()
	ftr.Status.StartTime = &now
	meta.SetStatusCondition(&ftr.Status.Conditions, metav1.Condition{
		Type:               "WorkflowCreated",
		Status:             metav1.ConditionTrue,
		Reason:             "Created",
		Message:            fmt.Sprintf("Argo Workflow %s created", wf.GetName()),
		LastTransitionTime: metav1.Now(),
	})

	if err := r.Status().Update(ctx, ftr); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *FineTuneRunReconciler) syncWorkflowStatus(ctx context.Context, ftr *v1alpha1.FineTuneRun) (ctrl.Result, error) {
	wf := &unstructured.Unstructured{}
	wf.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "argoproj.io",
		Version: "v1alpha1",
		Kind:    "Workflow",
	})

	err := r.Get(ctx, types.NamespacedName{Name: ftr.Status.ArgoWorkflow, Namespace: ftr.Namespace}, wf)
	if err != nil {
		if errors.IsNotFound(err) {
			ftr.Status.Phase = "Failed"
			r.Status().Update(ctx, ftr)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Read workflow phase from status.phase
	wfPhase, _, _ := unstructured.NestedString(wf.Object, "status", "phase")
	switch wfPhase {
	case "Succeeded":
		now := metav1.Now()
		ftr.Status.Phase = "Succeeded"
		ftr.Status.CompletionTime = &now
	case "Failed", "Error":
		now := metav1.Now()
		ftr.Status.Phase = "Failed"
		ftr.Status.CompletionTime = &now
	case "Running":
		ftr.Status.Phase = "Training"
	default:
		ftr.Status.Phase = "Pending"
	}

	r.Status().Update(ctx, ftr)
	return ctrl.Result{}, nil
}

func (r *FineTuneRunReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.FineTuneRun{}).
		Complete(r)
}
```

- [ ] **Step 3: Register FineTuneRun controller in suite_test.go**

Add to `suite_test.go` BeforeSuite:

```go
	err = (&FineTuneRunReconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		ReleaseName: "kube-llmops",
	}).SetupWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())
```

- [ ] **Step 4: Run envtest**

```bash
cd /home/rui/kube-llmops/operator
KUBEBUILDER_ASSETS=$(setup-envtest use 1.28.0 -p path) go test ./internal/controller/ -v -ginkgo.v
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/rui/kube-llmops
git add operator/internal/controller/
git commit -m "feat(operator): FineTuneRun controller (creates Argo Workflows)"
```

---

## Task 15: Validation Webhooks

**Files:**
- Create: `operator/api/v1alpha1/modeldeployment_webhook.go`
- Create: `operator/api/v1alpha1/llmplatform_webhook.go`
- Create: `operator/api/v1alpha1/finetunerun_webhook.go`

- [ ] **Step 1: Create webhooks via kubebuilder**

```bash
cd /home/rui/kube-llmops/operator
kubebuilder create webhook --group llmops --version v1alpha1 --kind ModelDeployment --programmatic-validation
kubebuilder create webhook --group llmops --version v1alpha1 --kind LLMPlatform --programmatic-validation
kubebuilder create webhook --group llmops --version v1alpha1 --kind FineTuneRun --programmatic-validation
```

- [ ] **Step 2: Implement ModelDeployment webhook**

Replace `operator/api/v1alpha1/modeldeployment_webhook.go`:

```go
package v1alpha1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/validate-llmops-kubellmops-io-v1alpha1-modeldeployment,mutating=false,failurePolicy=fail,sideEffects=None,groups=llmops.kubellmops.io,resources=modeldeployments,verbs=create;update,versions=v1alpha1,name=vmodeldeployment.kb.io,admissionReviewVersions=v1

func (r *ModelDeployment) ValidateCreate() (admission.Warnings, error) {
	return r.validate()
}

func (r *ModelDeployment) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	return r.validate()
}

func (r *ModelDeployment) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}

func (r *ModelDeployment) validate() (admission.Warnings, error) {
	var warnings admission.Warnings

	if r.Spec.Source == "" {
		return warnings, fmt.Errorf("source is required")
	}

	validEngines := map[string]bool{"auto": true, "vllm": true, "tei": true, "llamacpp": true, "": true}
	if !validEngines[r.Spec.Engine] {
		return warnings, fmt.Errorf("engine must be one of: auto, vllm, tei, llamacpp")
	}

	if r.Spec.Replicas != nil && *r.Spec.Replicas < 0 {
		return warnings, fmt.Errorf("replicas must be non-negative")
	}

	if r.Spec.Resources.GPU < 0 {
		return warnings, fmt.Errorf("gpu count must be non-negative")
	}

	if r.Spec.Canary != nil {
		if r.Spec.Canary.Weight < 0 || r.Spec.Canary.Weight > 100 {
			return warnings, fmt.Errorf("canary weight must be between 0 and 100")
		}
	}

	validAccelerators := map[string]bool{"nvidia": true, "amd": true, "gaudi": true, "": true}
	if !validAccelerators[r.Spec.Accelerator] {
		return warnings, fmt.Errorf("accelerator must be one of: nvidia, amd, gaudi")
	}

	return warnings, nil
}

// Implement CustomValidator interface
var _ admission.CustomValidator = &ModelDeployment{}

func (r *ModelDeployment) Default() {}

func (r *ModelDeployment) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	md := obj.(*ModelDeployment)
	return md.validate()
}

func (r *ModelDeployment) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	md := newObj.(*ModelDeployment)
	return md.validate()
}

func (r *ModelDeployment) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
```

**Note:** Kubebuilder v4 may scaffold either the old-style (`ValidateCreate() error`) or new-style (`CustomValidator` interface) webhooks. Adjust the implementation to match what was scaffolded. The key validation logic remains the same — check the scaffolded file and adapt.

- [ ] **Step 3: Implement FineTuneRun webhook**

Create `operator/api/v1alpha1/finetunerun_webhook.go` with validation:

```go
package v1alpha1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/validate-llmops-kubellmops-io-v1alpha1-finetunerun,mutating=false,failurePolicy=fail,sideEffects=None,groups=llmops.kubellmops.io,resources=finetuneruns,verbs=create;update,versions=v1alpha1,name=vfinetunerun.kb.io,admissionReviewVersions=v1

func (r *FineTuneRun) validateFineTuneRun() (admission.Warnings, error) {
	if r.Spec.BaseModel == "" {
		return nil, fmt.Errorf("baseModel is required")
	}
	if r.Spec.OutputName == "" {
		return nil, fmt.Errorf("outputName is required")
	}
	validMethods := map[string]bool{"lora": true, "qlora": true, "full": true}
	if !validMethods[r.Spec.Method] {
		return nil, fmt.Errorf("method must be one of: lora, qlora, full")
	}
	if r.Spec.DataSource.Type == "minio" && r.Spec.DataSource.Path == "" {
		return nil, fmt.Errorf("dataSource.path is required when type is minio")
	}
	return nil, nil
}
```

- [ ] **Step 4: Verify compilation**

```bash
cd /home/rui/kube-llmops/operator
make generate && make manifests && go build ./...
```

- [ ] **Step 5: Commit**

```bash
cd /home/rui/kube-llmops
git add operator/api/v1alpha1/ operator/config/
git commit -m "feat(operator): validation webhooks for all 3 CRDs"
```

---

## Task 16: Operator Helm Chart

**Files:**
- Create: `operator/charts/kube-llmops-operator/Chart.yaml`
- Create: `operator/charts/kube-llmops-operator/values.yaml`
- Create: `operator/charts/kube-llmops-operator/templates/deployment.yaml`
- Create: `operator/charts/kube-llmops-operator/templates/serviceaccount.yaml`
- Create: `operator/charts/kube-llmops-operator/templates/rbac.yaml`

- [ ] **Step 1: Create Chart.yaml**

Create `operator/charts/kube-llmops-operator/Chart.yaml`:

```yaml
apiVersion: v2
name: kube-llmops-operator
description: Kubernetes Operator for kube-llmops LLMOps platform
type: application
version: 1.0.0
appVersion: "1.0.0"
home: https://github.com/GaeaRuiW/kube-llmops
keywords:
  - llm
  - llmops
  - kubernetes
  - operator
maintainers:
  - name: GaeaRuiW
```

- [ ] **Step 2: Create values.yaml**

Create `operator/charts/kube-llmops-operator/values.yaml`:

```yaml
replicaCount: 1

image:
  repository: kube-llmops/operator
  tag: latest
  pullPolicy: IfNotPresent

serviceAccount:
  create: true
  name: kube-llmops-operator

resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 256Mi

chartPath: /charts/kube-llmops-stack
```

- [ ] **Step 3: Create templates**

Create `operator/charts/kube-llmops-operator/templates/serviceaccount.yaml`:

```yaml
{{- if .Values.serviceAccount.create }}
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .Values.serviceAccount.name }}
  namespace: {{ .Release.Namespace }}
  labels:
    app.kubernetes.io/name: kube-llmops-operator
    app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
```

Create `operator/charts/kube-llmops-operator/templates/rbac.yaml`:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: {{ .Release.Name }}-operator
rules:
  - apiGroups: ["llmops.kubellmops.io"]
    resources: ["modeldeployments", "llmplatforms", "finetuneruns"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["llmops.kubellmops.io"]
    resources: ["modeldeployments/status", "llmplatforms/status", "finetuneruns/status"]
    verbs: ["get", "update", "patch"]
  - apiGroups: ["llmops.kubellmops.io"]
    resources: ["modeldeployments/finalizers", "llmplatforms/finalizers", "finetuneruns/finalizers"]
    verbs: ["update"]
  - apiGroups: ["apps"]
    resources: ["deployments"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: [""]
    resources: ["services", "persistentvolumeclaims", "configmaps", "secrets"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["argoproj.io"]
    resources: ["workflows", "workflowtemplates"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: {{ .Release.Name }}-operator
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: {{ .Release.Name }}-operator
subjects:
  - kind: ServiceAccount
    name: {{ .Values.serviceAccount.name }}
    namespace: {{ .Release.Namespace }}
```

Create `operator/charts/kube-llmops-operator/templates/deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Release.Name }}-operator
  namespace: {{ .Release.Namespace }}
  labels:
    app.kubernetes.io/name: kube-llmops-operator
    app.kubernetes.io/instance: {{ .Release.Name }}
spec:
  replicas: {{ .Values.replicaCount }}
  selector:
    matchLabels:
      app.kubernetes.io/name: kube-llmops-operator
      app.kubernetes.io/instance: {{ .Release.Name }}
  template:
    metadata:
      labels:
        app.kubernetes.io/name: kube-llmops-operator
        app.kubernetes.io/instance: {{ .Release.Name }}
    spec:
      serviceAccountName: {{ .Values.serviceAccount.name }}
      containers:
        - name: operator
          image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          args:
            - --chart-path={{ .Values.chartPath }}
          ports:
            - name: health
              containerPort: 8081
              protocol: TCP
          livenessProbe:
            httpGet:
              path: /healthz
              port: 8081
            initialDelaySeconds: 15
            periodSeconds: 20
          readinessProbe:
            httpGet:
              path: /readyz
              port: 8081
            initialDelaySeconds: 5
            periodSeconds: 10
          resources:
            {{- toYaml .Values.resources | nindent 12 }}
```

- [ ] **Step 4: Copy CRD manifests into chart**

```bash
mkdir -p /home/rui/kube-llmops/operator/charts/kube-llmops-operator/crds
cp /home/rui/kube-llmops/operator/config/crd/bases/*.yaml /home/rui/kube-llmops/operator/charts/kube-llmops-operator/crds/
```

- [ ] **Step 5: Verify chart renders**

```bash
helm template test /home/rui/kube-llmops/operator/charts/kube-llmops-operator/
```

Expected: YAML output with Deployment, ServiceAccount, ClusterRole, ClusterRoleBinding.

- [ ] **Step 6: Commit**

```bash
cd /home/rui/kube-llmops
git add operator/charts/
git commit -m "feat(operator): Helm chart for installing the operator"
```

---

## Task 17: Sample CRs

**Files:**
- Create: 7 YAML files in `operator/config/samples/`

- [ ] **Step 1: Create sample ModelDeployment CRs**

Create `operator/config/samples/modeldeployment_vllm.yaml`:

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

Create `operator/config/samples/modeldeployment_tei_embedding.yaml`:

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

Create `operator/config/samples/modeldeployment_tei_reranker.yaml`:

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

Create `operator/config/samples/modeldeployment_llamacpp.yaml`:

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

- [ ] **Step 2: Create sample LLMPlatform CRs**

Create `operator/config/samples/llmplatform_minimal.yaml`:

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

Create `operator/config/samples/llmplatform_full.yaml`:

```yaml
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

- [ ] **Step 3: Create sample FineTuneRun CR**

Create `operator/config/samples/finetunerun_lora.yaml`:

```yaml
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

- [ ] **Step 4: Commit**

```bash
cd /home/rui/kube-llmops
git add operator/config/samples/
git commit -m "feat(operator): sample CRs for all 3 CRD types"
```

---

## Task 18: Migration Tool

**Files:**
- Create: `operator/cmd/migrate/main.go`

- [ ] **Step 1: Implement the migration tool**

Create `operator/cmd/migrate/main.go`:

```go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"gopkg.in/yaml.v3"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: migrate <helm-release-name> [namespace]\n")
		os.Exit(1)
	}
	releaseName := os.Args[1]
	namespace := "default"
	if len(os.Args) > 2 {
		namespace = os.Args[2]
	}

	// Get Helm values
	out, err := exec.Command("helm", "get", "values", releaseName, "-n", namespace, "-o", "json").Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get Helm values: %v\n", err)
		os.Exit(1)
	}

	var values map[string]interface{}
	if err := json.Unmarshal(out, &values); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse values: %v\n", err)
		os.Exit(1)
	}

	os.MkdirAll("generated", 0o755)

	// Generate LLMPlatform CR
	platform := generatePlatform(values, releaseName)
	writeYAML("generated/llmplatform.yaml", platform)
	fmt.Println("Generated: generated/llmplatform.yaml")

	// Generate ModelDeployment CRs
	global, _ := values["global"].(map[string]interface{})
	if global != nil {
		models, _ := global["models"].([]interface{})
		for _, m := range models {
			model, ok := m.(map[string]interface{})
			if !ok {
				continue
			}
			md := generateModelDeployment(model)
			name, _ := model["name"].(string)
			filename := fmt.Sprintf("generated/modeldeployment_%s.yaml", strings.ReplaceAll(name, "-", "_"))
			writeYAML(filename, md)
			fmt.Printf("Generated: %s\n", filename)
		}
	}

	fmt.Println("\nReview the generated CRs, then:")
	fmt.Printf("  helm uninstall %s -n %s\n", releaseName, namespace)
	fmt.Println("  kubectl apply -f generated/")
}

func generatePlatform(values map[string]interface{}, name string) map[string]interface{} {
	platform := map[string]interface{}{
		"apiVersion": "llmops.kubellmops.io/v1alpha1",
		"kind":       "LLMPlatform",
		"metadata":   map[string]interface{}{"name": name},
		"spec":       map[string]interface{}{},
	}

	spec := platform["spec"].(map[string]interface{})

	// Gateway
	if litellm, ok := values["litellm"].(map[string]interface{}); ok {
		spec["gateway"] = map[string]interface{}{
			"enabled": litellm["enabled"],
		}
	}

	// Observability
	if obs, ok := values["observability"].(map[string]interface{}); ok {
		spec["observability"] = map[string]interface{}{
			"enabled": obs["enabled"],
		}
	}

	// Modules
	if global, ok := values["global"].(map[string]interface{}); ok {
		if modules, ok := global["modules"].(map[string]interface{}); ok {
			spec["modules"] = modules
		}
		if ms, ok := global["modelStore"].(map[string]interface{}); ok {
			spec["modelStore"] = ms
		}
		if np, ok := global["nodePort"].(map[string]interface{}); ok {
			spec["nodePort"] = np
		}
	}

	return platform
}

func generateModelDeployment(model map[string]interface{}) map[string]interface{} {
	md := map[string]interface{}{
		"apiVersion": "llmops.kubellmops.io/v1alpha1",
		"kind":       "ModelDeployment",
		"metadata":   map[string]interface{}{"name": model["name"]},
		"spec": map[string]interface{}{
			"source": model["source"],
		},
	}
	spec := md["spec"].(map[string]interface{})

	if replicas, ok := model["replicas"]; ok {
		spec["replicas"] = replicas
	}
	if resources, ok := model["resources"]; ok {
		spec["resources"] = resources
	}
	if engineArgs, ok := model["engineArgs"]; ok {
		spec["engineArgs"] = engineArgs
	}
	if engine, ok := model["engine"]; ok {
		spec["engine"] = engine
	}

	return md
}

func writeYAML(path string, data map[string]interface{}) {
	out, _ := yaml.Marshal(data)
	os.WriteFile(path, out, 0o644)
}
```

- [ ] **Step 2: Add yaml dependency**

```bash
cd /home/rui/kube-llmops/operator
go get gopkg.in/yaml.v3
go mod tidy
```

- [ ] **Step 3: Verify compilation**

```bash
cd /home/rui/kube-llmops/operator
go build ./cmd/migrate/
```

- [ ] **Step 4: Commit**

```bash
cd /home/rui/kube-llmops
git add operator/cmd/migrate/ operator/go.mod operator/go.sum
git commit -m "feat(operator): migration tool (Helm release → CRs)"
```

---

## Task 19: Architecture Document

**Files:**
- Create: `operator/docs/architecture/operator.md`

- [ ] **Step 1: Write the architecture document**

Create `operator/docs/architecture/operator.md` with the full technical reference. The document should cover all 12 sections from the spec (Section 8.1):

1. Design Philosophy
2. CRD API Reference
3. Controller Internals
4. Engine Auto-Detection
5. Model Lifecycle
6. LiteLLM Integration
7. Helm SDK Bridge
8. FineTuneRun Pipeline
9. RBAC Model
10. Failure Modes
11. Operator Observability
12. Security

Each section should be 300-500 words with diagrams, code examples, and configuration references. Refer to the spec at `docs/superpowers/specs/2026-04-05-phase6-operator-design.md` for the complete CRD field definitions and architecture diagrams.

- [ ] **Step 2: Commit**

```bash
cd /home/rui/kube-llmops
git add operator/docs/architecture/
git commit -m "docs(operator): architecture technical reference"
```

---

## Task 20: User Manual (English)

**Files:**
- Create: `operator/docs/user-guide/operator-guide-en.md`

- [ ] **Step 1: Write the English user manual**

Create `operator/docs/user-guide/operator-guide-en.md` with all 8 chapters from the spec (Section 8.2):

1. Quick Start
2. Core Concepts
3. Model Management
4. Platform Setup
5. Fine-tuning
6. Operations
7. Migration Guide
8. API Reference

Each chapter should include practical examples using the sample CRs from `operator/config/samples/`. Include exact `kubectl` commands and expected output. The Quick Start chapter should be a complete walkthrough from zero to a running model.

- [ ] **Step 2: Commit**

```bash
cd /home/rui/kube-llmops
git add operator/docs/user-guide/operator-guide-en.md
git commit -m "docs(operator): user manual (English)"
```

---

## Task 21: User Manual (Chinese)

**Files:**
- Create: `operator/docs/user-guide/operator-guide-zh.md`

- [ ] **Step 1: Write the Chinese user manual**

Create `operator/docs/user-guide/operator-guide-zh.md` with the same 8 chapters, natively written in Chinese (not machine-translated):

1. 快速开始
2. 核心概念
3. 模型管理
4. 平台配置
5. 模型微调
6. 运维指南
7. 迁移指南
8. API 参考

Mirror the structure and examples from the English manual, but use natural Chinese technical writing conventions.

- [ ] **Step 2: Commit**

```bash
cd /home/rui/kube-llmops
git add operator/docs/user-guide/operator-guide-zh.md
git commit -m "docs(operator): user manual (中文)"
```

---

## Final Verification

After all tasks are complete, run the full test suite:

```bash
cd /home/rui/kube-llmops/operator

# Unit tests
go test ./internal/engine/ ./internal/builder/ ./internal/gateway/ ./internal/helmbridge/ -v

# Integration tests (envtest)
KUBEBUILDER_ASSETS=$(setup-envtest use 1.28.0 -p path) go test ./internal/controller/ -v -ginkgo.v

# Build
go build ./...
go build ./cmd/migrate/

# Helm chart
helm template test charts/kube-llmops-operator/

# Existing Helm chart tests still pass
cd /home/rui/kube-llmops
python -m pytest tests/helm/ -v
```

---

## Addendum: Gaps Identified in Self-Review

The following items were identified during plan self-review against the spec. They should be addressed during implementation as enhancements to the base tasks above.

### A. Canary Deployment Logic (enhance Task 9)

The ModelDeployment controller must handle `spec.canary`:
- When `spec.canary` is set, create a **second** Deployment + Service for the canary model
- Register both models with LiteLLM gateway with weight routing
- Update `status.canary` with canary Deployment health
- On canary removal, delete canary resources and update gateway weights

### B. Gateway UpdateModel Method (enhance Task 8)

Add `UpdateModel()` to the gateway client interface:
```go
UpdateModel(ctx context.Context, modelID string, weight int32) error
```
This calls `POST /model/update` on LiteLLM to adjust weight routing when canary weights change.

### C. Spot GPU Tolerations (enhance Task 6)

`buildTolerations()` should handle `spec.spot`:
```go
// When spot.enabled, add provider-specific tolerations:
// aws:      kubernetes.io/os=linux, karpenter.sh/capacity-type=spot
// gcp:      cloud.google.com/gke-spot=true
// azure:    kubernetes.azure.com/scalesetpriority=spot
// karpenter: karpenter.sh/capacity-type=spot
```

### D. Component Health Monitoring (enhance Task 12)

The LLMPlatform controller should watch key Deployments/StatefulSets after Helm release completes:
- Check readiness of LiteLLM, Grafana, Prometheus, PostgreSQL, MinIO, etc.
- Populate `status.components` with per-component phase and endpoint
- Set overall phase to `Degraded` if any component is unhealthy

### E. LLMPlatform Finalizer (enhance Task 12)

Add a finalizer to `LLMPlatformReconciler` that calls `HelmClient.Uninstall()` on deletion:
```go
const llmPlatformFinalizer = "llmops.kubellmops.io/helm-cleanup"
```

### F. Test Count Target (~100 tests)

The base plan specifies ~38 tests. To reach ~100, add during implementation:
- **Engine resolver edge cases**: 10 more tests (empty string, special chars, case sensitivity)
- **Builder error paths**: 10 tests (nil platform, zero resources, missing fields)
- **Canary builder tests**: 8 tests (canary deployment, canary service, weight routing)
- **Gateway error handling**: 6 tests (timeout, 500 error, invalid JSON, auth failures)
- **Helm values edge cases**: 6 tests (all disabled, empty spec, partial spec)
- **Controller error paths**: 8 tests (missing PVC, gateway unreachable, Helm install failure)
- **Webhook validation**: 10 tests (invalid engine, negative replicas, missing required fields)
- **Migration tool**: 4 tests (valid release, empty models, missing values)
- **Total additions: ~62 → Grand total: ~100 tests**

### G. Full Argo Workflow Templates (enhance Task 13)

During implementation, the stub templates should be replaced with the actual inline Python scripts from `charts/kube-llmops-stack/charts/finetune/templates/workflow.yaml`. The existing Helm chart already contains the complete logic for prepare-data, finetune (LLaMA-Factory), merge-upload (MinIO + MLflow registry), evaluate, quality-gate, and deploy (canary). The Go builder should port these scripts faithfully.

### H. Main.go Wiring (enhance Task 1)

During implementation, update `cmd/main.go` to:
1. Parse `--chart-path` flag for LLMPlatform Helm SDK bridge
2. Parse `--gateway-url` and `--gateway-key` for LiteLLM client
3. Register webhook server if running with `--enable-webhooks`
4. Register all 3 controllers with proper dependencies
