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
	"strings"
	"testing"

	"k8s.io/utils/ptr"
)

// ---------------------------------------------------------------------------
// ModelDeployment validation tests
// ---------------------------------------------------------------------------

func TestValidateModelDeploymentSpec_ValidSpec(t *testing.T) {
	spec := &ModelDeploymentSpec{
		Source:      "Qwen/Qwen2.5-7B-Instruct",
		Engine:      "vllm",
		Replicas:    ptr.To[int32](2),
		Accelerator: "nvidia",
		Resources:   ModelResources{GPU: 1, Memory: "16Gi", CPU: "4"},
	}
	if err := ValidateModelDeploymentSpec(spec); err != nil {
		t.Errorf("expected valid spec, got error: %v", err)
	}
}

func TestValidateModelDeploymentSpec_EmptySource(t *testing.T) {
	spec := &ModelDeploymentSpec{Source: ""}
	err := ValidateModelDeploymentSpec(spec)
	if err == nil {
		t.Fatal("expected error for empty source")
	}
	if !strings.Contains(err.Error(), "source is required") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidateModelDeploymentSpec_InvalidEngine(t *testing.T) {
	spec := &ModelDeploymentSpec{Source: "model/name", Engine: "invalid-engine"}
	err := ValidateModelDeploymentSpec(spec)
	if err == nil {
		t.Fatal("expected error for invalid engine")
	}
	if !strings.Contains(err.Error(), "engine must be one of") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidateModelDeploymentSpec_NegativeReplicas(t *testing.T) {
	spec := &ModelDeploymentSpec{
		Source:   "model/name",
		Replicas: ptr.To[int32](-1),
	}
	err := ValidateModelDeploymentSpec(spec)
	if err == nil {
		t.Fatal("expected error for negative replicas")
	}
	if !strings.Contains(err.Error(), "replicas must be >= 0") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidateModelDeploymentSpec_NegativeGPU(t *testing.T) {
	spec := &ModelDeploymentSpec{
		Source:    "model/name",
		Resources: ModelResources{GPU: -1},
	}
	err := ValidateModelDeploymentSpec(spec)
	if err == nil {
		t.Fatal("expected error for negative GPU")
	}
	if !strings.Contains(err.Error(), "gpu must be >= 0") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidateModelDeploymentSpec_InvalidAccelerator(t *testing.T) {
	spec := &ModelDeploymentSpec{
		Source:      "model/name",
		Accelerator: "tpu",
	}
	err := ValidateModelDeploymentSpec(spec)
	if err == nil {
		t.Fatal("expected error for invalid accelerator")
	}
	if !strings.Contains(err.Error(), "accelerator must be one of") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidateModelDeploymentSpec_ValidCanary(t *testing.T) {
	spec := &ModelDeploymentSpec{
		Source: "model/name",
		Canary: &CanaryConfig{
			Source: "model/canary",
			Weight: 20,
		},
	}
	if err := ValidateModelDeploymentSpec(spec); err != nil {
		t.Errorf("expected valid canary spec, got error: %v", err)
	}
}

func TestValidateModelDeploymentSpec_CanaryMissingSource(t *testing.T) {
	spec := &ModelDeploymentSpec{
		Source: "model/name",
		Canary: &CanaryConfig{
			Source: "",
			Weight: 20,
		},
	}
	err := ValidateModelDeploymentSpec(spec)
	if err == nil {
		t.Fatal("expected error for canary missing source")
	}
	if !strings.Contains(err.Error(), "canary.source is required") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidateModelDeploymentSpec_CanaryWeightOutOfRange(t *testing.T) {
	spec := &ModelDeploymentSpec{
		Source: "model/name",
		Canary: &CanaryConfig{
			Source: "model/canary",
			Weight: 150,
		},
	}
	err := ValidateModelDeploymentSpec(spec)
	if err == nil {
		t.Fatal("expected error for canary weight out of range")
	}
	if !strings.Contains(err.Error(), "canary.weight must be 0-100") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidateModelDeploymentSpec_ZeroReplicasValid(t *testing.T) {
	spec := &ModelDeploymentSpec{
		Source:   "model/name",
		Replicas: ptr.To[int32](0),
	}
	if err := ValidateModelDeploymentSpec(spec); err != nil {
		t.Errorf("expected zero replicas to be valid, got error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// LLMPlatform validation tests
// ---------------------------------------------------------------------------

func TestValidateLLMPlatformSpec_ValidSpec(t *testing.T) {
	spec := &LLMPlatformSpec{
		Gateway: GatewaySpec{
			Enabled: true,
			Routing: "least-busy",
		},
		ModelStore: ModelStoreSpec{
			Enabled:  true,
			Endpoint: "minio:9000",
			Bucket:   "models",
		},
	}
	if err := ValidateLLMPlatformSpec(spec); err != nil {
		t.Errorf("expected valid spec, got error: %v", err)
	}
}

func TestValidateLLMPlatformSpec_EmptySpecDefaults(t *testing.T) {
	spec := &LLMPlatformSpec{}
	if err := ValidateLLMPlatformSpec(spec); err != nil {
		t.Errorf("expected empty spec (all defaults) to pass, got error: %v", err)
	}
}

func TestValidateLLMPlatformSpec_ModelStoreEnabledWithoutEndpoint(t *testing.T) {
	spec := &LLMPlatformSpec{
		ModelStore: ModelStoreSpec{
			Enabled: true,
			Bucket:  "models",
		},
	}
	err := ValidateLLMPlatformSpec(spec)
	if err == nil {
		t.Fatal("expected error for modelStore enabled without endpoint")
	}
	if !strings.Contains(err.Error(), "modelStore.endpoint is required") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidateLLMPlatformSpec_ModelStoreEnabledWithoutBucket(t *testing.T) {
	spec := &LLMPlatformSpec{
		ModelStore: ModelStoreSpec{
			Enabled:  true,
			Endpoint: "minio:9000",
		},
	}
	err := ValidateLLMPlatformSpec(spec)
	if err == nil {
		t.Fatal("expected error for modelStore enabled without bucket")
	}
	if !strings.Contains(err.Error(), "modelStore.bucket is required") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidateLLMPlatformSpec_InvalidRoutingStrategy(t *testing.T) {
	spec := &LLMPlatformSpec{
		Gateway: GatewaySpec{
			Routing: "round-robin",
		},
	}
	err := ValidateLLMPlatformSpec(spec)
	if err == nil {
		t.Fatal("expected error for invalid routing strategy")
	}
	if !strings.Contains(err.Error(), "gateway.routing must be one of") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// ---------------------------------------------------------------------------
// FineTuneRun validation tests
// ---------------------------------------------------------------------------

func TestValidateFineTuneRunSpec_ValidSpec(t *testing.T) {
	spec := &FineTuneRunSpec{
		BaseModel:  "meta-llama/Llama-3-8B",
		OutputName: "my-finetuned-model",
		Method:     "lora",
		DataSource: DataSourceSpec{
			Type:   "minio",
			Path:   "s3://datasets/my-data/",
			Format: "alpaca",
		},
		Training: TrainingSpec{
			Epochs:    3,
			BatchSize: 4,
		},
	}
	if err := ValidateFineTuneRunSpec(spec); err != nil {
		t.Errorf("expected valid spec, got error: %v", err)
	}
}

func TestValidateFineTuneRunSpec_EmptyBaseModel(t *testing.T) {
	spec := &FineTuneRunSpec{
		BaseModel:  "",
		OutputName: "out",
		DataSource: DataSourceSpec{Type: "pvc"},
	}
	err := ValidateFineTuneRunSpec(spec)
	if err == nil {
		t.Fatal("expected error for empty baseModel")
	}
	if !strings.Contains(err.Error(), "baseModel is required") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidateFineTuneRunSpec_EmptyOutputName(t *testing.T) {
	spec := &FineTuneRunSpec{
		BaseModel:  "meta-llama/Llama-3-8B",
		OutputName: "",
		DataSource: DataSourceSpec{Type: "pvc"},
	}
	err := ValidateFineTuneRunSpec(spec)
	if err == nil {
		t.Fatal("expected error for empty outputName")
	}
	if !strings.Contains(err.Error(), "outputName is required") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidateFineTuneRunSpec_InvalidMethod(t *testing.T) {
	spec := &FineTuneRunSpec{
		BaseModel:  "meta-llama/Llama-3-8B",
		OutputName: "out",
		Method:     "dpo",
		DataSource: DataSourceSpec{Type: "pvc"},
	}
	err := ValidateFineTuneRunSpec(spec)
	if err == nil {
		t.Fatal("expected error for invalid method")
	}
	if !strings.Contains(err.Error(), "method must be one of") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidateFineTuneRunSpec_InvalidDataSourceType(t *testing.T) {
	spec := &FineTuneRunSpec{
		BaseModel:  "meta-llama/Llama-3-8B",
		OutputName: "out",
		DataSource: DataSourceSpec{Type: "s3"},
	}
	err := ValidateFineTuneRunSpec(spec)
	if err == nil {
		t.Fatal("expected error for invalid dataSource type")
	}
	if !strings.Contains(err.Error(), "dataSource.type must be one of") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidateFineTuneRunSpec_MinioWithoutPath(t *testing.T) {
	spec := &FineTuneRunSpec{
		BaseModel:  "meta-llama/Llama-3-8B",
		OutputName: "out",
		DataSource: DataSourceSpec{Type: "minio", Path: ""},
	}
	err := ValidateFineTuneRunSpec(spec)
	if err == nil {
		t.Fatal("expected error for minio without path")
	}
	if !strings.Contains(err.Error(), "dataSource.path is required when type is minio") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidateFineTuneRunSpec_QualityGateWithoutEvaluation(t *testing.T) {
	spec := &FineTuneRunSpec{
		BaseModel:  "meta-llama/Llama-3-8B",
		OutputName: "out",
		DataSource: DataSourceSpec{Type: "pvc"},
		QualityGate: QualityGateSpec{
			Enabled: true,
		},
		Evaluation: EvaluationSpec{
			Enabled: false,
		},
	}
	err := ValidateFineTuneRunSpec(spec)
	if err == nil {
		t.Fatal("expected error for qualityGate without evaluation")
	}
	if !strings.Contains(err.Error(), "evaluation must be enabled when qualityGate is enabled") {
		t.Errorf("unexpected error message: %v", err)
	}
}
