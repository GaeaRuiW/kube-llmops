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
		const resourceName = "test-md"
		const namespace = "default"

		var md *v1alpha1.ModelDeployment

		BeforeEach(func() {
			md = &v1alpha1.ModelDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: namespace,
				},
				Spec: v1alpha1.ModelDeploymentSpec{
					Source:   "Qwen/Qwen2.5-7B-Instruct",
					Engine:   "auto",
					Replicas: ptr.To(int32(1)),
					Resources: v1alpha1.ModelResources{
						GPU:    1,
						Memory: "16Gi",
						CPU:    "4",
					},
				},
			}
			Expect(k8sClient.Create(ctx, md)).To(Succeed())
		})

		AfterEach(func() {
			resource := &v1alpha1.ModelDeployment{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: namespace}, resource)
			if err == nil {
				// Remove finalizer so we can delete
				resource.Finalizers = nil
				_ = k8sClient.Update(ctx, resource)
				_ = k8sClient.Delete(ctx, resource)
			}
		})

		It("should create a Deployment for the model", func() {
			dep := &appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: namespace}, dep)
			}, timeout, interval).Should(Succeed())

			Expect(dep.Labels["kube-llmops/engine"]).To(Equal("vllm"))
		})

		It("should create a Service for the model", func() {
			svc := &corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: namespace}, svc)
			}, timeout, interval).Should(Succeed())

			Expect(svc.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
		})

		It("should create a PVC for the model cache", func() {
			pvc := &corev1.PersistentVolumeClaim{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: resourceName + "-cache", Namespace: namespace}, pvc)
			}, timeout, interval).Should(Succeed())
		})

		It("should set the status engine field", func() {
			Eventually(func() string {
				updated := &v1alpha1.ModelDeployment{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: namespace}, updated); err != nil {
					return ""
				}
				return updated.Status.Engine
			}, timeout, interval).Should(Equal("vllm"))
		})
	})
})
