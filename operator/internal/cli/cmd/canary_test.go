package cmd

import (
	"testing"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestApplyCanary(t *testing.T) {
	md := &v1alpha1.ModelDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec:       v1alpha1.ModelDeploymentSpec{Source: "org/model-v1", Replicas: ptr.To(int32(1))},
	}
	applyCanary(md, "org/model-v2", 20)
	if md.Spec.Canary == nil {
		t.Fatal("expected canary to be set")
	}
	if md.Spec.Canary.Source != "org/model-v2" {
		t.Errorf("expected canary source %q, got %q", "org/model-v2", md.Spec.Canary.Source)
	}
	if md.Spec.Canary.Weight != 20 {
		t.Errorf("expected weight 20, got %d", md.Spec.Canary.Weight)
	}
}

func TestApplyPromote(t *testing.T) {
	md := &v1alpha1.ModelDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: v1alpha1.ModelDeploymentSpec{
			Source:   "org/model-v1",
			Replicas: ptr.To(int32(1)),
			Canary:   &v1alpha1.CanaryConfig{Source: "org/model-v2", Weight: 50},
		},
	}
	applyPromote(md)
	if md.Spec.Source != "org/model-v2" {
		t.Errorf("expected promoted source %q, got %q", "org/model-v2", md.Spec.Source)
	}
	if md.Spec.Canary != nil {
		t.Error("expected canary to be nil after promote")
	}
}

func TestApplyRollback(t *testing.T) {
	md := &v1alpha1.ModelDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: v1alpha1.ModelDeploymentSpec{
			Source:   "org/model-v1",
			Replicas: ptr.To(int32(1)),
			Canary:   &v1alpha1.CanaryConfig{Source: "org/model-v2", Weight: 50},
		},
	}
	applyRollback(md)
	if md.Spec.Source != "org/model-v1" {
		t.Errorf("expected source unchanged %q, got %q", "org/model-v1", md.Spec.Source)
	}
	if md.Spec.Canary != nil {
		t.Error("expected canary to be nil after rollback")
	}
}
