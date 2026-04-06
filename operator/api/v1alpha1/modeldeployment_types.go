/*
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
*/

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
