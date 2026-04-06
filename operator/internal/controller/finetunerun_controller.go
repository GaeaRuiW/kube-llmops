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
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	"github.com/kube-llmops/operator/internal/builder"
)

// FineTuneRunReconciler reconciles a FineTuneRun object.
type FineTuneRunReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	ReleaseName string
}

// +kubebuilder:rbac:groups=llmops.kubellmops.io,resources=finetuneruns,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=llmops.kubellmops.io,resources=finetuneruns/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=llmops.kubellmops.io,resources=finetuneruns/finalizers,verbs=update
// +kubebuilder:rbac:groups=argoproj.io,resources=workflows,verbs=get;list;watch;create;update;patch;delete

var argoWorkflowGVR = schema.GroupVersionResource{
	Group:    "argoproj.io",
	Version:  "v1alpha1",
	Resource: "workflows",
}

// Reconcile moves the cluster state toward the desired FineTuneRun state.
func (r *FineTuneRunReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 1. Fetch the FineTuneRun
	ftr := &v1alpha1.FineTuneRun{}
	if err := r.Get(ctx, req.NamespacedName, ftr); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Skip if already terminal
	if ftr.Status.Phase == "Succeeded" || ftr.Status.Phase == "Failed" {
		return ctrl.Result{}, nil
	}

	// 2. If ArgoWorkflow already in status → sync workflow status
	if ftr.Status.ArgoWorkflow != "" {
		wf := &unstructured.Unstructured{}
		wf.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "argoproj.io",
			Version: "v1alpha1",
			Kind:    "Workflow",
		})
		err := r.Get(ctx, types.NamespacedName{
			Name:      ftr.Status.ArgoWorkflow,
			Namespace: ftr.Namespace,
		}, wf)
		if err != nil {
			if apierrors.IsNotFound(err) {
				// Workflow was deleted
				ftr.Status.Phase = "Failed"
				setCondition(&ftr.Status.Conditions, metav1.Condition{
					Type:               "WorkflowReady",
					Status:             metav1.ConditionFalse,
					Reason:             "WorkflowNotFound",
					Message:            "Argo Workflow was deleted",
					LastTransitionTime: metav1.NewTime(time.Now()),
					ObservedGeneration: ftr.Generation,
				})
				if statusErr := r.refetchAndUpdateStatus(ctx, req, ftr); statusErr != nil {
					return ctrl.Result{}, statusErr
				}
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, err
		}

		// Map Argo phase to FineTuneRun phase
		argoPhase, _, _ := unstructured.NestedString(wf.Object, "status", "phase")
		switch argoPhase {
		case "Succeeded":
			ftr.Status.Phase = "Succeeded"
			now := metav1.NewTime(time.Now())
			ftr.Status.CompletionTime = &now
		case "Failed", "Error":
			ftr.Status.Phase = "Failed"
			now := metav1.NewTime(time.Now())
			ftr.Status.CompletionTime = &now
		case "Running":
			ftr.Status.Phase = "Training"
		default:
			ftr.Status.Phase = "Pending"
		}

		setCondition(&ftr.Status.Conditions, metav1.Condition{
			Type:               "WorkflowReady",
			Status:             conditionStatus(argoPhase == "Succeeded"),
			Reason:             fmt.Sprintf("ArgoPhase%s", capitalizeFirst(argoPhase)),
			Message:            fmt.Sprintf("Argo Workflow phase: %s", argoPhase),
			LastTransitionTime: metav1.NewTime(time.Now()),
			ObservedGeneration: ftr.Generation,
		})

		if err := r.refetchAndUpdateStatus(ctx, req, ftr); err != nil {
			return ctrl.Result{}, err
		}

		// Requeue if not terminal
		if ftr.Status.Phase != "Succeeded" && ftr.Status.Phase != "Failed" {
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		return ctrl.Result{}, nil
	}

	// 3. Create Argo Workflow
	releaseName := r.ReleaseName
	if releaseName == "" {
		releaseName = "kube-llmops"
	}

	wf := builder.BuildArgoWorkflow(ftr, releaseName)

	// Set owner reference
	if err := controllerutil.SetControllerReference(ftr, wf, r.Scheme); err != nil {
		log.Error(err, "failed to set owner reference on Argo Workflow")
		// Continue anyway - the workflow can still be created
	}

	if err := r.Create(ctx, wf); err != nil {
		// 4. Handle Argo CRD missing
		if isNoCRDError(err) {
			log.Error(err, "Argo Workflows CRD not installed")
			ftr.Status.Phase = "Failed"
			setCondition(&ftr.Status.Conditions, metav1.Condition{
				Type:               "WorkflowReady",
				Status:             metav1.ConditionFalse,
				Reason:             "ArgoCRDMissing",
				Message:            "Argo Workflows CRD is not installed in the cluster",
				LastTransitionTime: metav1.NewTime(time.Now()),
				ObservedGeneration: ftr.Generation,
			})
			if statusErr := r.refetchAndUpdateStatus(ctx, req, ftr); statusErr != nil {
				return ctrl.Result{}, statusErr
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// 5. Update status
	log.Info("created Argo Workflow", "name", wf.GetName())
	now := metav1.NewTime(time.Now())
	ftr.Status.Phase = "Pending"
	ftr.Status.ArgoWorkflow = wf.GetName()
	ftr.Status.StartTime = &now
	setCondition(&ftr.Status.Conditions, metav1.Condition{
		Type:               "WorkflowReady",
		Status:             metav1.ConditionFalse,
		Reason:             "WorkflowCreated",
		Message:            "Argo Workflow has been created",
		LastTransitionTime: now,
		ObservedGeneration: ftr.Generation,
	})

	if err := r.refetchAndUpdateStatus(ctx, req, ftr); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// refetchAndUpdateStatus re-fetches the FineTuneRun to get the latest resourceVersion,
// restores the computed status, and then performs the status subresource update.
func (r *FineTuneRunReconciler) refetchAndUpdateStatus(ctx context.Context, req ctrl.Request, ftr *v1alpha1.FineTuneRun) error {
	status := ftr.Status
	if err := r.Get(ctx, req.NamespacedName, ftr); err != nil {
		return err
	}
	ftr.Status = status
	return r.Status().Update(ctx, ftr)
}

// isNoCRDError checks if the error indicates a missing CRD (resource not found on the API server).
func isNoCRDError(err error) bool {
	if apierrors.IsNotFound(err) {
		return true
	}
	// Handle "no matches for kind" errors from missing CRDs
	if apimeta.IsNoMatchError(err) {
		return true
	}
	// discovery/no-match errors
	if meta, ok := err.(*apierrors.StatusError); ok {
		return meta.ErrStatus.Code == 404
	}
	return false
}

// conditionStatus returns the appropriate metav1.ConditionStatus for a boolean.
func conditionStatus(ok bool) metav1.ConditionStatus {
	if ok {
		return metav1.ConditionTrue
	}
	return metav1.ConditionFalse
}

// capitalizeFirst capitalizes the first letter of a string for use in Reason fields.
func capitalizeFirst(s string) string {
	if s == "" {
		return "Unknown"
	}
	b := []byte(s)
	if b[0] >= 'a' && b[0] <= 'z' {
		b[0] -= 32
	}
	return string(b)
}

// SetupWithManager sets up the controller with the Manager.
func (r *FineTuneRunReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.FineTuneRun{}).
		Named("finetunerun").
		Complete(r)
}
