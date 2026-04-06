package printer

import (
	"bytes"
	"strings"
	"testing"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func sampleMD() *v1alpha1.ModelDeployment {
	return &v1alpha1.ModelDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: "qwen-7b", Namespace: "default"},
		Spec:       v1alpha1.ModelDeploymentSpec{Source: "Qwen/Qwen2.5-7B", Replicas: ptr.To(int32(2))},
		Status:     v1alpha1.ModelDeploymentStatus{Phase: "Ready", Engine: "vllm", ReadyReplicas: 2, TotalReplicas: 2, Endpoint: "http://qwen-7b:8000/v1"},
	}
}

func TestPrintModelDeploymentTable(t *testing.T) {
	var buf bytes.Buffer
	PrintModelDeployments(&buf, "table", []v1alpha1.ModelDeployment{*sampleMD()})
	out := buf.String()
	if !strings.Contains(out, "qwen-7b") {
		t.Errorf("expected name in output, got:\n%s", out)
	}
	if !strings.Contains(out, "vllm") {
		t.Errorf("expected engine in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Ready") {
		t.Errorf("expected phase in output, got:\n%s", out)
	}
}

func TestPrintModelDeploymentJSON(t *testing.T) {
	var buf bytes.Buffer
	PrintModelDeployments(&buf, "json", []v1alpha1.ModelDeployment{*sampleMD()})
	out := buf.String()
	if !strings.Contains(out, `"name"`) || !strings.Contains(out, "qwen-7b") {
		t.Errorf("expected JSON name field, got:\n%s", out)
	}
}

func TestPrintModelDeploymentYAML(t *testing.T) {
	var buf bytes.Buffer
	PrintModelDeployments(&buf, "yaml", []v1alpha1.ModelDeployment{*sampleMD()})
	out := buf.String()
	if !strings.Contains(out, "qwen-7b") {
		t.Errorf("expected YAML name field, got:\n%s", out)
	}
}

func TestPrintModelDeploymentWide(t *testing.T) {
	var buf bytes.Buffer
	PrintModelDeployments(&buf, "wide", []v1alpha1.ModelDeployment{*sampleMD()})
	out := buf.String()
	if !strings.Contains(out, "http://qwen-7b:8000/v1") {
		t.Errorf("expected endpoint in wide output, got:\n%s", out)
	}
}
