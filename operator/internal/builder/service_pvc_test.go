package builder

import (
	"testing"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildService(t *testing.T) {
	md := &v1alpha1.ModelDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: "qwen", Namespace: "default"},
	}

	// --- vllm engine ---
	svc := BuildService(md, "vllm")

	if svc.Name != "qwen" {
		t.Errorf("expected service name 'qwen', got %q", svc.Name)
	}
	if svc.Namespace != "default" {
		t.Errorf("expected namespace 'default', got %q", svc.Namespace)
	}
	if svc.Spec.Type != corev1.ServiceTypeClusterIP {
		t.Errorf("expected ClusterIP service type, got %v", svc.Spec.Type)
	}
	if len(svc.Spec.Ports) != 1 {
		t.Fatalf("expected 1 port, got %d", len(svc.Spec.Ports))
	}
	if svc.Spec.Ports[0].Port != 8000 {
		t.Errorf("expected vllm port 8000, got %d", svc.Spec.Ports[0].Port)
	}
	if svc.Spec.Ports[0].Name != "http" {
		t.Errorf("expected port name 'http', got %q", svc.Spec.Ports[0].Name)
	}
	if svc.Spec.Ports[0].TargetPort.IntVal != 8000 {
		t.Errorf("expected vllm targetPort 8000, got %d", svc.Spec.Ports[0].TargetPort.IntVal)
	}

	// Verify labels
	expectedLabels := map[string]string{
		"app.kubernetes.io/name":     "vllm",
		"app.kubernetes.io/instance": "qwen",
		"app.kubernetes.io/part-of":  "kube-llmops",
		"kube-llmops/model":          "qwen",
		"kube-llmops/engine":         "vllm",
	}
	for k, v := range expectedLabels {
		if svc.Labels[k] != v {
			t.Errorf("expected label %s=%s, got %s", k, v, svc.Labels[k])
		}
	}

	// Verify selector
	if svc.Spec.Selector["app.kubernetes.io/name"] != "vllm" {
		t.Errorf("expected selector app.kubernetes.io/name=vllm, got %q", svc.Spec.Selector["app.kubernetes.io/name"])
	}
	if svc.Spec.Selector["kube-llmops/model"] != "qwen" {
		t.Errorf("expected selector kube-llmops/model=qwen, got %q", svc.Spec.Selector["kube-llmops/model"])
	}

	// --- tei engine ---
	svcTei := BuildService(md, "tei")
	if svcTei.Spec.Ports[0].Port != 8080 {
		t.Errorf("expected tei port 8080, got %d", svcTei.Spec.Ports[0].Port)
	}
	if svcTei.Labels["kube-llmops/engine"] != "tei" {
		t.Errorf("expected engine label 'tei', got %q", svcTei.Labels["kube-llmops/engine"])
	}

	// --- llamacpp engine ---
	svcLlama := BuildService(md, "llamacpp")
	if svcLlama.Spec.Ports[0].Port != 8080 {
		t.Errorf("expected llamacpp port 8080, got %d", svcLlama.Spec.Ports[0].Port)
	}
}

func TestBuildPVC(t *testing.T) {
	md := &v1alpha1.ModelDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: "qwen", Namespace: "default"},
	}

	// --- vllm engine ---
	pvc := BuildPVC(md, "vllm")

	if pvc.Name != "qwen-cache" {
		t.Errorf("expected pvc name 'qwen-cache', got %q", pvc.Name)
	}
	if pvc.Namespace != "default" {
		t.Errorf("expected namespace 'default', got %q", pvc.Namespace)
	}

	if len(pvc.Spec.AccessModes) != 1 || pvc.Spec.AccessModes[0] != corev1.ReadWriteOnce {
		t.Errorf("expected access mode ReadWriteOnce, got %v", pvc.Spec.AccessModes)
	}

	expectedStorage := resource.MustParse("50Gi")
	actualStorage := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	if !actualStorage.Equal(expectedStorage) {
		t.Errorf("expected vllm storage 50Gi, got %s", actualStorage.String())
	}

	// Verify labels
	expectedLabels := map[string]string{
		"app.kubernetes.io/name":     "vllm",
		"app.kubernetes.io/instance": "qwen",
		"app.kubernetes.io/part-of":  "kube-llmops",
		"kube-llmops/model":          "qwen",
		"kube-llmops/engine":         "vllm",
	}
	for k, v := range expectedLabels {
		if pvc.Labels[k] != v {
			t.Errorf("expected label %s=%s, got %s", k, v, pvc.Labels[k])
		}
	}

	// --- tei engine ---
	pvcTei := BuildPVC(md, "tei")
	teiStorage := pvcTei.Spec.Resources.Requests[corev1.ResourceStorage]
	expectedTei := resource.MustParse("10Gi")
	if !teiStorage.Equal(expectedTei) {
		t.Errorf("expected tei storage 10Gi, got %s", teiStorage.String())
	}

	// --- llamacpp engine ---
	pvcLlama := BuildPVC(md, "llamacpp")
	llamaStorage := pvcLlama.Spec.Resources.Requests[corev1.ResourceStorage]
	expectedLlama := resource.MustParse("30Gi")
	if !llamaStorage.Equal(expectedLlama) {
		t.Errorf("expected llamacpp storage 30Gi, got %s", llamaStorage.String())
	}
}
