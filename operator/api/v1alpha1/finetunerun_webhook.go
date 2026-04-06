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

import "fmt"

// +kubebuilder:webhook:path=/validate-llmops-kubeai-io-v1alpha1-finetunerun,mutating=false,failurePolicy=fail,sideEffects=None,groups=llmops.kubeai.io,resources=finetuneruns,verbs=create;update,versions=v1alpha1,name=vfinetunerun.kb.io,admissionReviewVersions=v1

// ValidateFineTuneRunSpec validates a FineTuneRunSpec.
func ValidateFineTuneRunSpec(spec *FineTuneRunSpec) error {
	if spec.BaseModel == "" {
		return fmt.Errorf("baseModel is required")
	}
	if spec.OutputName == "" {
		return fmt.Errorf("outputName is required")
	}
	validMethods := map[string]bool{"lora": true, "qlora": true, "full": true, "": true}
	if !validMethods[spec.Method] {
		return fmt.Errorf("method must be one of: lora, qlora, full (got %q)", spec.Method)
	}
	validTypes := map[string]bool{"minio": true, "huggingface": true, "pvc": true}
	if !validTypes[spec.DataSource.Type] {
		return fmt.Errorf("dataSource.type must be one of: minio, huggingface, pvc (got %q)", spec.DataSource.Type)
	}
	if spec.DataSource.Type == "minio" && spec.DataSource.Path == "" {
		return fmt.Errorf("dataSource.path is required when type is minio")
	}
	if spec.Training.Epochs < 0 {
		return fmt.Errorf("training.epochs must be >= 0")
	}
	if spec.Training.BatchSize < 0 {
		return fmt.Errorf("training.batchSize must be >= 0")
	}
	if spec.QualityGate.Enabled && !spec.Evaluation.Enabled {
		return fmt.Errorf("evaluation must be enabled when qualityGate is enabled")
	}
	return nil
}
