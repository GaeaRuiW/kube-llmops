package builder

import (
	"testing"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func newTestMD(name, source string, gpu int32) *v1alpha1.ModelDeployment {
	return &v1alpha1.ModelDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: v1alpha1.ModelDeploymentSpec{
			Source:      source,
			Replicas:    ptr.To(int32(1)),
			Resources:   v1alpha1.ModelResources{GPU: gpu, Memory: "16Gi", CPU: "4"},
			Accelerator: "nvidia",
		},
	}
}

func newTestPlatform() *v1alpha1.LLMPlatform {
	return &v1alpha1.LLMPlatform{
		ObjectMeta: metav1.ObjectMeta{Name: "kube-llmops", Namespace: "default"},
		Spec: v1alpha1.LLMPlatformSpec{
			ModelStore: v1alpha1.ModelStoreSpec{
				Enabled:   true,
				Endpoint:  "kube-llmops-minio:9000",
				Bucket:    "models",
				AccessKey: "minioadmin",
				SecretKey: "minioadmin",
				Image:     "kube-llmops/model-loader:latest",
			},
		},
	}
}

func TestBuildDeployment_VLLMEngine(t *testing.T) {
	md := newTestMD("qwen-7b", "Qwen/Qwen2.5-7B-Instruct", 1)
	platform := newTestPlatform()
	dep := BuildDeployment(md, "vllm", platform)

	// Name
	if dep.Name != "qwen-7b" {
		t.Errorf("expected deployment name %q, got %q", "qwen-7b", dep.Name)
	}

	// Replicas
	if dep.Spec.Replicas == nil || *dep.Spec.Replicas != 1 {
		t.Errorf("expected 1 replica, got %v", dep.Spec.Replicas)
	}

	// Main container image
	mainContainer := dep.Spec.Template.Spec.Containers[0]
	if mainContainer.Image != "vllm/vllm-openai:latest" {
		t.Errorf("expected vllm image, got %q", mainContainer.Image)
	}

	// Port
	if len(mainContainer.Ports) == 0 || mainContainer.Ports[0].ContainerPort != 8000 {
		t.Errorf("expected port 8000, got %v", mainContainer.Ports)
	}

	// Init container image
	if len(dep.Spec.Template.Spec.InitContainers) == 0 {
		t.Fatal("expected init container, got none")
	}
	initContainer := dep.Spec.Template.Spec.InitContainers[0]
	if initContainer.Image != "kube-llmops/model-loader:latest" {
		t.Errorf("expected model-loader image, got %q", initContainer.Image)
	}

	// /dev/shm volume
	foundShm := false
	for _, v := range dep.Spec.Template.Spec.Volumes {
		if v.Name == "dshm" && v.EmptyDir != nil && v.EmptyDir.Medium == "Memory" {
			foundShm = true
			break
		}
	}
	if !foundShm {
		t.Error("expected /dev/shm volume (dshm) with Memory medium for vllm")
	}

	// GPU toleration
	foundToleration := false
	for _, tol := range dep.Spec.Template.Spec.Tolerations {
		if tol.Key == "nvidia.com/gpu" {
			foundToleration = true
			break
		}
	}
	if !foundToleration {
		t.Error("expected nvidia.com/gpu toleration")
	}
}

func TestBuildDeployment_TEIEngine(t *testing.T) {
	md := newTestMD("bge-small", "BAAI/bge-small-en-v1.5", 0)
	platform := newTestPlatform()
	dep := BuildDeployment(md, "tei", platform)

	mainContainer := dep.Spec.Template.Spec.Containers[0]

	// TEI image
	expectedImage := "ghcr.io/huggingface/text-embeddings-inference:cpu-1.6"
	if mainContainer.Image != expectedImage {
		t.Errorf("expected TEI image %q, got %q", expectedImage, mainContainer.Image)
	}

	// Port 8080
	if len(mainContainer.Ports) == 0 || mainContainer.Ports[0].ContainerPort != 8080 {
		t.Errorf("expected port 8080, got %v", mainContainer.Ports)
	}

	// NO /dev/shm volume
	for _, v := range dep.Spec.Template.Spec.Volumes {
		if v.Name == "dshm" {
			t.Error("TEI engine should NOT have /dev/shm volume")
		}
	}
}

func TestBuildDeployment_LlamaCppEngine(t *testing.T) {
	md := newTestMD("llama-gguf", "TheBloke/Llama-2-7B-GGUF", 1)
	platform := newTestPlatform()
	dep := BuildDeployment(md, "llamacpp", platform)

	mainContainer := dep.Spec.Template.Spec.Containers[0]

	// llamacpp image
	expectedImage := "ghcr.io/ggml-org/llama.cpp:server"
	if mainContainer.Image != expectedImage {
		t.Errorf("expected llamacpp image %q, got %q", expectedImage, mainContainer.Image)
	}

	// Port 8080
	if len(mainContainer.Ports) == 0 || mainContainer.Ports[0].ContainerPort != 8080 {
		t.Errorf("expected port 8080, got %v", mainContainer.Ports)
	}
}

func TestBuildDeployment_EngineArgs(t *testing.T) {
	md := newTestMD("qwen-7b", "Qwen/Qwen2.5-7B-Instruct", 1)
	md.Spec.EngineArgs = map[string]string{
		"--max-model-len": "4096",
		"--dtype":         "float16",
	}
	platform := newTestPlatform()
	dep := BuildDeployment(md, "vllm", platform)

	mainContainer := dep.Spec.Template.Spec.Containers[0]
	args := mainContainer.Args

	// Check that extra engine args appear in the container args.
	foundMaxModelLen := false
	foundDtype := false
	for i, a := range args {
		if a == "--max-model-len" && i+1 < len(args) && args[i+1] == "4096" {
			foundMaxModelLen = true
		}
		if a == "--dtype" && i+1 < len(args) && args[i+1] == "float16" {
			foundDtype = true
		}
	}
	if !foundMaxModelLen {
		t.Errorf("expected --max-model-len 4096 in args, got %v", args)
	}
	if !foundDtype {
		t.Errorf("expected --dtype float16 in args, got %v", args)
	}
}

func TestBuildDeployment_AMDAccelerator(t *testing.T) {
	md := newTestMD("qwen-7b", "Qwen/Qwen2.5-7B-Instruct", 1)
	md.Spec.Accelerator = "amd"
	platform := newTestPlatform()
	dep := BuildDeployment(md, "vllm", platform)

	foundToleration := false
	for _, tol := range dep.Spec.Template.Spec.Tolerations {
		if tol.Key == "amd.com/gpu" {
			foundToleration = true
			break
		}
	}
	if !foundToleration {
		t.Error("expected amd.com/gpu toleration")
	}
}

func TestBuildDeployment_MIGDevice(t *testing.T) {
	md := newTestMD("qwen-7b", "Qwen/Qwen2.5-7B-Instruct", 1)
	md.Spec.MIGDevice = "nvidia.com/mig-1g.5gb"
	platform := newTestPlatform()
	dep := BuildDeployment(md, "vllm", platform)

	// Toleration should use the MIG device name
	foundToleration := false
	for _, tol := range dep.Spec.Template.Spec.Tolerations {
		if tol.Key == "nvidia.com/mig-1g.5gb" {
			foundToleration = true
			break
		}
	}
	if !foundToleration {
		t.Error("expected nvidia.com/mig-1g.5gb toleration when MIGDevice is set")
	}

	// GPU resource should use the MIG device name
	mainContainer := dep.Spec.Template.Spec.Containers[0]
	if _, ok := mainContainer.Resources.Limits["nvidia.com/mig-1g.5gb"]; !ok {
		t.Error("expected nvidia.com/mig-1g.5gb in resource limits when MIGDevice is set")
	}
}
