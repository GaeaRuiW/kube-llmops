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
	Enabled       bool          `json:"enabled,omitempty"`
	Routing       string        `json:"routing,omitempty"`
	Image         ImageSpec     `json:"image,omitempty"`
	RateLimiting  EnabledToggle `json:"rateLimiting,omitempty"`
	BudgetControl EnabledToggle `json:"budgetControl,omitempty"`
}

// ObservabilitySpec configures monitoring and tracing.
type ObservabilitySpec struct {
	Enabled  bool          `json:"enabled,omitempty"`
	Grafana  GrafanaSpec   `json:"grafana,omitempty"`
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
	Enabled               bool   `json:"enabled,omitempty"`
	Endpoint              string `json:"endpoint,omitempty"`
	Bucket                string `json:"bucket,omitempty"`
	AccessKey             string `json:"accessKey,omitempty"`
	SecretKey             string `json:"secretKey,omitempty"`
	HFTransferConcurrency int32  `json:"hfTransferConcurrency,omitempty"`
	Image                 string `json:"image,omitempty"`
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
