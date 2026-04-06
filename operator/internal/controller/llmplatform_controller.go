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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	"github.com/kube-llmops/operator/internal/helmbridge"
)

// LLMPlatformReconciler reconciles a LLMPlatform object.
type LLMPlatformReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	HelmClient helmbridge.HelmClient
	ChartPath  string
}

// +kubebuilder:rbac:groups=llmops.kubellmops.io,resources=llmplatforms,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=llmops.kubellmops.io,resources=llmplatforms/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=llmops.kubellmops.io,resources=llmplatforms/finalizers,verbs=update

// Reconcile moves the cluster state toward the desired LLMPlatform state.
func (r *LLMPlatformReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 1. Fetch the LLMPlatform
	platform := &v1alpha1.LLMPlatform{}
	if err := r.Get(ctx, req.NamespacedName, platform); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// 2. Translate spec to Helm values
	values := helmbridge.TranslateValues(platform)

	releaseName := platform.Name
	namespace := platform.Namespace
	chartPath := r.ChartPath
	if chartPath == "" {
		chartPath = "charts/kube-llmops-stack"
	}

	// 3. Check if release exists
	existingRelease, err := r.HelmClient.GetRelease(releaseName, namespace)

	if err != nil || existingRelease == nil {
		// 4a. Install
		log.Info("installing Helm release", "name", releaseName)
		platform.Status.Phase = "Installing"
		setCondition(&platform.Status.Conditions, metav1.Condition{
			Type:               "HelmRelease",
			Status:             metav1.ConditionFalse,
			Reason:             "Installing",
			Message:            "Helm release is being installed",
			LastTransitionTime: metav1.NewTime(time.Now()),
			ObservedGeneration: platform.Generation,
		})
		if statusErr := r.Status().Update(ctx, platform); statusErr != nil {
			return ctrl.Result{}, statusErr
		}

		rel, installErr := r.HelmClient.Install(releaseName, namespace, chartPath, values)
		if installErr != nil {
			platform.Status.Phase = "Failed"
			setCondition(&platform.Status.Conditions, metav1.Condition{
				Type:               "HelmRelease",
				Status:             metav1.ConditionFalse,
				Reason:             "InstallFailed",
				Message:            fmt.Sprintf("Helm install failed: %v", installErr),
				LastTransitionTime: metav1.NewTime(time.Now()),
				ObservedGeneration: platform.Generation,
			})
			if statusErr := r.Status().Update(ctx, platform); statusErr != nil {
				return ctrl.Result{}, statusErr
			}
			return ctrl.Result{}, installErr
		}

		platform.Status.Phase = "Ready"
		platform.Status.HelmRelease = rel.Name
		platform.Status.HelmRevision = rel.Version
		setCondition(&platform.Status.Conditions, metav1.Condition{
			Type:               "HelmRelease",
			Status:             metav1.ConditionTrue,
			Reason:             "InstallSucceeded",
			Message:            "Helm release installed successfully",
			LastTransitionTime: metav1.NewTime(time.Now()),
			ObservedGeneration: platform.Generation,
		})
	} else {
		// 4b. Upgrade
		log.Info("upgrading Helm release", "name", releaseName)
		platform.Status.Phase = "Upgrading"
		setCondition(&platform.Status.Conditions, metav1.Condition{
			Type:               "HelmRelease",
			Status:             metav1.ConditionFalse,
			Reason:             "Upgrading",
			Message:            "Helm release is being upgraded",
			LastTransitionTime: metav1.NewTime(time.Now()),
			ObservedGeneration: platform.Generation,
		})
		if statusErr := r.Status().Update(ctx, platform); statusErr != nil {
			return ctrl.Result{}, statusErr
		}

		rel, upgradeErr := r.HelmClient.Upgrade(releaseName, namespace, chartPath, values)
		if upgradeErr != nil {
			platform.Status.Phase = "Failed"
			setCondition(&platform.Status.Conditions, metav1.Condition{
				Type:               "HelmRelease",
				Status:             metav1.ConditionFalse,
				Reason:             "UpgradeFailed",
				Message:            fmt.Sprintf("Helm upgrade failed: %v", upgradeErr),
				LastTransitionTime: metav1.NewTime(time.Now()),
				ObservedGeneration: platform.Generation,
			})
			if statusErr := r.Status().Update(ctx, platform); statusErr != nil {
				return ctrl.Result{}, statusErr
			}
			return ctrl.Result{}, upgradeErr
		}

		platform.Status.Phase = "Ready"
		platform.Status.HelmRelease = rel.Name
		platform.Status.HelmRevision = rel.Version
		setCondition(&platform.Status.Conditions, metav1.Condition{
			Type:               "HelmRelease",
			Status:             metav1.ConditionTrue,
			Reason:             "UpgradeSucceeded",
			Message:            "Helm release upgraded successfully",
			LastTransitionTime: metav1.NewTime(time.Now()),
			ObservedGeneration: platform.Generation,
		})
	}

	// 5. Final status update
	if err := r.Status().Update(ctx, platform); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LLMPlatformReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.LLMPlatform{}).
		Named("llmplatform").
		Complete(r)
}
