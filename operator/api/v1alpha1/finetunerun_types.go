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
	Epochs                    int32  `json:"epochs,omitempty"`
	BatchSize                 int32  `json:"batchSize,omitempty"`
	LearningRate              string `json:"learningRate,omitempty"`
	GradientAccumulationSteps int32  `json:"gradientAccumulationSteps,omitempty"`
	WarmupRatio               string `json:"warmupRatio,omitempty"`
	LoraRank                  int32  `json:"loraRank,omitempty"`
	LoraAlpha                 int32  `json:"loraAlpha,omitempty"`
	LoraTarget                string `json:"loraTarget,omitempty"`
}

// EvaluationSpec configures post-training evaluation.
type EvaluationSpec struct {
	Enabled bool   `json:"enabled,omitempty"`
	Dataset string `json:"dataset,omitempty"`
}

// QualityGateSpec configures pass/fail thresholds.
type QualityGateSpec struct {
	Enabled    bool              `json:"enabled,omitempty"`
	Thresholds QualityThresholds `json:"thresholds,omitempty"`
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
