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

var _ = Describe("LLMPlatform Controller", func() {
	const (
		timeout  = 30 * time.Second
		interval = 250 * time.Millisecond
	)

	Context("When creating an LLMPlatform", func() {
		const resourceName = "test-platform"
		const namespace = "default"

		BeforeEach(func() {
			platform := &v1alpha1.LLMPlatform{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: namespace,
				},
				Spec: v1alpha1.LLMPlatformSpec{
					Gateway: v1alpha1.GatewaySpec{
						Enabled: true,
					},
					Observability: v1alpha1.ObservabilitySpec{
						Enabled: true,
					},
				},
			}
			Expect(k8sClient.Create(ctx, platform)).To(Succeed())
		})

		AfterEach(func() {
			resource := &v1alpha1.LLMPlatform{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: namespace}, resource)
			if err == nil {
				_ = k8sClient.Delete(ctx, resource)
			}
		})

		It("should set the status phase", func() {
			Eventually(func() string {
				updated := &v1alpha1.LLMPlatform{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: namespace}, updated); err != nil {
					return ""
				}
				return updated.Status.Phase
			}, timeout, interval).ShouldNot(BeEmpty())
		})

		It("should set the helm release name in status", func() {
			Eventually(func() string {
				updated := &v1alpha1.LLMPlatform{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: namespace}, updated); err != nil {
					return ""
				}
				return updated.Status.HelmRelease
			}, timeout, interval).ShouldNot(BeEmpty())
		})
	})
})
