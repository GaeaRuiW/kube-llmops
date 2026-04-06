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

// +kubebuilder:webhook:path=/validate-llmops-kubeai-io-v1alpha1-llmplatform,mutating=false,failurePolicy=fail,sideEffects=None,groups=llmops.kubeai.io,resources=llmplatforms,verbs=create;update,versions=v1alpha1,name=vllmplatform.kb.io,admissionReviewVersions=v1

// ValidateLLMPlatformSpec validates an LLMPlatformSpec.
func ValidateLLMPlatformSpec(spec *LLMPlatformSpec) error {
	if spec.ModelStore.Enabled {
		if spec.ModelStore.Endpoint == "" {
			return fmt.Errorf("modelStore.endpoint is required when modelStore is enabled")
		}
		if spec.ModelStore.Bucket == "" {
			return fmt.Errorf("modelStore.bucket is required when modelStore is enabled")
		}
	}
	validRouting := map[string]bool{
		"": true, "simple-shuffle": true, "least-busy": true,
		"latency-based-routing": true, "usage-based-routing": true,
	}
	if !validRouting[spec.Gateway.Routing] {
		return fmt.Errorf("gateway.routing must be one of: simple-shuffle, least-busy, latency-based-routing, usage-based-routing (got %q)", spec.Gateway.Routing)
	}
	return nil
}
