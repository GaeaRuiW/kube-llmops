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
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	"github.com/kube-llmops/operator/internal/builder"
	"github.com/kube-llmops/operator/internal/engine"
	"github.com/kube-llmops/operator/internal/gateway"
)

const modelDeploymentFinalizer = "llmops.kubellmops.io/finalizer"

// ModelDeploymentReconciler reconciles a ModelDeployment object.
type ModelDeploymentReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	GatewayClient gateway.Client
}

// +kubebuilder:rbac:groups=llmops.kubellmops.io,resources=modeldeployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=llmops.kubellmops.io,resources=modeldeployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=llmops.kubellmops.io,resources=modeldeployments/finalizers,verbs=update
// +kubebuilder:rbac:groups=llmops.kubellmops.io,resources=llmplatforms,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete

// Reconcile moves the cluster state toward the desired ModelDeployment state.
func (r *ModelDeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 1. Fetch the ModelDeployment
	md := &v1alpha1.ModelDeployment{}
	if err := r.Get(ctx, req.NamespacedName, md); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// 2. Handle deletion
	if !md.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(md, modelDeploymentFinalizer) {
			// Deregister from gateway (best-effort)
			if r.GatewayClient != nil {
				if err := r.GatewayClient.DeregisterModel(ctx, md.Name); err != nil {
					log.Error(err, "failed to deregister model from gateway")
				}
			}
			controllerutil.RemoveFinalizer(md, modelDeploymentFinalizer)
			if err := r.Update(ctx, md); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// 3. Add finalizer if missing
	if !controllerutil.ContainsFinalizer(md, modelDeploymentFinalizer) {
		controllerutil.AddFinalizer(md, modelDeploymentFinalizer)
		if err := r.Update(ctx, md); err != nil {
			return ctrl.Result{}, err
		}
	}

	// 4. Resolve engine
	resolvedEngine := engine.ResolveEngine(md.Spec.Source, md.Spec.Engine)

	// 5. Find LLMPlatform in same namespace
	var platform *v1alpha1.LLMPlatform
	platformList := &v1alpha1.LLMPlatformList{}
	if err := r.List(ctx, platformList, client.InNamespace(md.Namespace)); err == nil && len(platformList.Items) > 0 {
		platform = &platformList.Items[0]
	}

	// 6. Ensure PVC
	pvc := builder.BuildPVC(md, resolvedEngine)
	existingPVC := &corev1.PersistentVolumeClaim{}
	if err := r.Get(ctx, types.NamespacedName{Name: pvc.Name, Namespace: pvc.Namespace}, existingPVC); err != nil {
		if apierrors.IsNotFound(err) {
			if err := controllerutil.SetControllerReference(md, pvc, r.Scheme); err != nil {
				return ctrl.Result{}, err
			}
			if err := r.Create(ctx, pvc); err != nil {
				return ctrl.Result{}, err
			}
			log.Info("created PVC", "name", pvc.Name)
		} else {
			return ctrl.Result{}, err
		}
	}

	// 7. Ensure Deployment (create or update)
	desiredDep := builder.BuildDeployment(md, resolvedEngine, platform)
	existingDep := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Name: desiredDep.Name, Namespace: desiredDep.Namespace}, existingDep); err != nil {
		if apierrors.IsNotFound(err) {
			if err := controllerutil.SetControllerReference(md, desiredDep, r.Scheme); err != nil {
				return ctrl.Result{}, err
			}
			if err := r.Create(ctx, desiredDep); err != nil {
				return ctrl.Result{}, err
			}
			log.Info("created Deployment", "name", desiredDep.Name)
		} else {
			return ctrl.Result{}, err
		}
	} else {
		// Update existing deployment
		existingDep.Spec = desiredDep.Spec
		existingDep.Labels = desiredDep.Labels
		if err := r.Update(ctx, existingDep); err != nil {
			return ctrl.Result{}, err
		}
	}

	// 8. Ensure Service
	svc := builder.BuildService(md, resolvedEngine)
	existingSvc := &corev1.Service{}
	if err := r.Get(ctx, types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, existingSvc); err != nil {
		if apierrors.IsNotFound(err) {
			if err := controllerutil.SetControllerReference(md, svc, r.Scheme); err != nil {
				return ctrl.Result{}, err
			}
			if err := r.Create(ctx, svc); err != nil {
				return ctrl.Result{}, err
			}
			log.Info("created Service", "name", svc.Name)
		} else {
			return ctrl.Result{}, err
		}
	}

	// 9. Update status
	// Re-read the deployment to get current replica status
	dep := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Name: md.Name, Namespace: md.Namespace}, dep); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	port := builder.EnginePort(resolvedEngine)
	md.Status.Engine = resolvedEngine
	md.Status.ReadyReplicas = dep.Status.ReadyReplicas
	md.Status.TotalReplicas = dep.Status.Replicas
	md.Status.Endpoint = fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", md.Name, md.Namespace, port)

	// Determine phase
	desired := int32(1)
	if md.Spec.Replicas != nil {
		desired = *md.Spec.Replicas
	}
	switch {
	case dep.Status.ReadyReplicas >= desired && desired > 0:
		md.Status.Phase = "Ready"
	case dep.Status.ReadyReplicas > 0:
		md.Status.Phase = "Degraded"
	default:
		md.Status.Phase = "Deploying"
	}

	// Set condition
	now := metav1.NewTime(time.Now())
	readyCond := metav1.Condition{
		Type:               "Ready",
		LastTransitionTime: now,
		ObservedGeneration: md.Generation,
	}
	if md.Status.Phase == "Ready" {
		readyCond.Status = metav1.ConditionTrue
		readyCond.Reason = "AllReplicasReady"
		readyCond.Message = "All replicas are ready"
	} else {
		readyCond.Status = metav1.ConditionFalse
		readyCond.Reason = "ReplicasNotReady"
		readyCond.Message = fmt.Sprintf("%d/%d replicas ready", dep.Status.ReadyReplicas, desired)
	}
	setCondition(&md.Status.Conditions, readyCond)

	// Re-fetch to get latest resourceVersion (child resource creation may have changed it)
	computedStatus := md.Status
	if err := r.Get(ctx, req.NamespacedName, md); err != nil {
		return ctrl.Result{}, err
	}
	md.Status = computedStatus

	if err := r.Status().Update(ctx, md); err != nil {
		return ctrl.Result{}, err
	}

	// 10. When Ready, register with gateway
	if md.Status.Phase == "Ready" && r.GatewayClient != nil {
		model := gateway.GatewayModel{
			ModelName: md.Name,
			LiteLLMParams: gateway.LiteLLMParams{
				Model:   "openai/" + md.Name,
				APIBase: md.Status.Endpoint,
			},
		}
		if err := r.GatewayClient.RegisterModel(ctx, model); err != nil {
			log.Error(err, "failed to register model with gateway")
		}
	}

	return ctrl.Result{}, nil
}

// setCondition adds or updates a condition in the conditions slice.
func setCondition(conditions *[]metav1.Condition, cond metav1.Condition) {
	for i, existing := range *conditions {
		if existing.Type == cond.Type {
			(*conditions)[i] = cond
			return
		}
	}
	*conditions = append(*conditions, cond)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ModelDeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ModelDeployment{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Named("modeldeployment").
		Complete(r)
}
