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

// +kubebuilder:webhook:path=/validate-llmops-kubeai-io-v1alpha1-modeldeployment,mutating=false,failurePolicy=fail,sideEffects=None,groups=llmops.kubeai.io,resources=modeldeployments,verbs=create;update,versions=v1alpha1,name=vmodeldeployment.kb.io,admissionReviewVersions=v1

// ValidateModelDeploymentSpec validates a ModelDeploymentSpec.
func ValidateModelDeploymentSpec(spec *ModelDeploymentSpec) error {
	if spec.Source == "" {
		return fmt.Errorf("source is required")
	}
	validEngines := map[string]bool{"auto": true, "vllm": true, "tei": true, "llamacpp": true, "": true}
	if !validEngines[spec.Engine] {
		return fmt.Errorf("engine must be one of: auto, vllm, tei, llamacpp (got %q)", spec.Engine)
	}
	if spec.Replicas != nil && *spec.Replicas < 0 {
		return fmt.Errorf("replicas must be >= 0")
	}
	if spec.Resources.GPU < 0 {
		return fmt.Errorf("gpu must be >= 0")
	}
	validAccelerators := map[string]bool{"nvidia": true, "amd": true, "gaudi": true, "": true}
	if !validAccelerators[spec.Accelerator] {
		return fmt.Errorf("accelerator must be one of: nvidia, amd, gaudi (got %q)", spec.Accelerator)
	}
	if spec.Canary != nil {
		if spec.Canary.Source == "" {
			return fmt.Errorf("canary.source is required")
		}
		if spec.Canary.Weight < 0 || spec.Canary.Weight > 100 {
			return fmt.Errorf("canary.weight must be 0-100")
		}
	}
	return nil
}
