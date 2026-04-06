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
		const resourceName = "test-ftr"
		const namespace = "default"

		BeforeEach(func() {
			ftr := &v1alpha1.FineTuneRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: namespace,
				},
				Spec: v1alpha1.FineTuneRunSpec{
					BaseModel:  "meta-llama/Llama-3.1-8B",
					OutputName: "my-finetuned-model",
					Method:     "lora",
					DataSource: v1alpha1.DataSourceSpec{
						Type:   "huggingface",
						Path:   "tatsu-lab/alpaca",
						Format: "alpaca",
					},
				},
			}
			Expect(k8sClient.Create(ctx, ftr)).To(Succeed())
		})

		AfterEach(func() {
			resource := &v1alpha1.FineTuneRun{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: namespace}, resource)
			if err == nil {
				_ = k8sClient.Delete(ctx, resource)
			}
		})

		It("should set the status phase (Failed with ArgoCRDMissing since Argo CRD is not installed)", func() {
			Eventually(func() string {
				updated := &v1alpha1.FineTuneRun{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: namespace}, updated); err != nil {
					return ""
				}
				return updated.Status.Phase
			}, timeout, interval).ShouldNot(BeEmpty())
		})

		It("should have a condition with reason ArgoCRDMissing", func() {
			Eventually(func() string {
				updated := &v1alpha1.FineTuneRun{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: namespace}, updated); err != nil {
					return ""
				}
				for _, c := range updated.Status.Conditions {
					if c.Type == "WorkflowReady" {
						return c.Reason
					}
				}
				return ""
			}, timeout, interval).Should(Equal("ArgoCRDMissing"))
		})
	})
})
