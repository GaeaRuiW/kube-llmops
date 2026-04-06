package helmbridge

import (
	"testing"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTranslateValues_Gateway(t *testing.T) {
	platform := &v1alpha1.LLMPlatform{
		ObjectMeta: metav1.ObjectMeta{Name: "kube-llmops"},
		Spec: v1alpha1.LLMPlatformSpec{
			Gateway: v1alpha1.GatewaySpec{
				Enabled: true,
				Routing: "latency-based-routing",
			},
		},
	}
	vals := TranslateValues(platform)

	litellm, ok := vals["litellm"].(map[string]interface{})
	if !ok {
		t.Fatal("litellm key missing")
	}
	if litellm["enabled"] != true {
		t.Error("litellm.enabled should be true")
	}
	if litellm["routingStrategy"] != "latency-based-routing" {
		t.Errorf("routingStrategy = %v", litellm["routingStrategy"])
	}
}

func TestTranslateValues_Modules(t *testing.T) {
	platform := &v1alpha1.LLMPlatform{
		ObjectMeta: metav1.ObjectMeta{Name: "kube-llmops"},
		Spec: v1alpha1.LLMPlatformSpec{
			Modules: v1alpha1.ModulesSpec{
				RAG:      v1alpha1.EnabledToggle{Enabled: true},
				Finetune: v1alpha1.EnabledToggle{Enabled: false},
				Security: v1alpha1.EnabledToggle{Enabled: false},
			},
		},
	}
	vals := TranslateValues(platform)

	global, ok := vals["global"].(map[string]interface{})
	if !ok {
		t.Fatal("global key missing")
	}
	modules := global["modules"].(map[string]interface{})
	rag := modules["rag"].(map[string]interface{})
	if rag["enabled"] != true {
		t.Error("global.modules.rag.enabled should be true")
	}
	finetune := modules["finetune"].(map[string]interface{})
	if finetune["enabled"] != false {
		t.Error("global.modules.finetune.enabled should be false")
	}
}

func TestTranslateValues_NodePort(t *testing.T) {
	platform := &v1alpha1.LLMPlatform{
		ObjectMeta: metav1.ObjectMeta{Name: "kube-llmops"},
		Spec: v1alpha1.LLMPlatformSpec{
			NodePort: v1alpha1.NodePortSpec{
				Enabled: true,
				Host:    "172.29.193.187",
			},
		},
	}
	vals := TranslateValues(platform)

	global := vals["global"].(map[string]interface{})
	np := global["nodePort"].(map[string]interface{})
	if np["enabled"] != true {
		t.Error("nodePort.enabled should be true")
	}
	if np["host"] != "172.29.193.187" {
		t.Errorf("nodePort.host = %v", np["host"])
	}
}

func TestTranslateValues_ModelStore(t *testing.T) {
	platform := &v1alpha1.LLMPlatform{
		ObjectMeta: metav1.ObjectMeta{Name: "kube-llmops"},
		Spec: v1alpha1.LLMPlatformSpec{
			ModelStore: v1alpha1.ModelStoreSpec{
				Enabled:  true,
				Endpoint: "minio:9000",
				Bucket:   "models",
			},
		},
	}
	vals := TranslateValues(platform)

	global := vals["global"].(map[string]interface{})
	ms := global["modelStore"].(map[string]interface{})
	if ms["endpoint"] != "minio:9000" {
		t.Errorf("modelStore.endpoint = %v", ms["endpoint"])
	}
}

func TestTranslateValues_GatewayImage(t *testing.T) {
	platform := &v1alpha1.LLMPlatform{
		ObjectMeta: metav1.ObjectMeta{Name: "kube-llmops"},
		Spec: v1alpha1.LLMPlatformSpec{
			Gateway: v1alpha1.GatewaySpec{
				Enabled: true,
				Image:   v1alpha1.ImageSpec{Tag: "v1.82.3"},
			},
		},
	}
	vals := TranslateValues(platform)

	litellm := vals["litellm"].(map[string]interface{})
	img := litellm["image"].(map[string]interface{})
	if img["tag"] != "v1.82.3" {
		t.Errorf("image.tag = %v", img["tag"])
	}
}

func TestTranslateValues_Ingress(t *testing.T) {
	platform := &v1alpha1.LLMPlatform{
		ObjectMeta: metav1.ObjectMeta{Name: "kube-llmops"},
		Spec: v1alpha1.LLMPlatformSpec{
			Ingress: v1alpha1.IngressSpec{
				Enabled:   true,
				ClassName: "traefik",
				Host:      "llmops.local",
			},
		},
	}
	vals := TranslateValues(platform)

	ingress := vals["ingress"].(map[string]interface{})
	if ingress["enabled"] != true {
		t.Error("ingress.enabled should be true")
	}
	if ingress["className"] != "traefik" {
		t.Errorf("ingress.className = %v", ingress["className"])
	}
}

func TestTranslateValues_NoIngress(t *testing.T) {
	platform := &v1alpha1.LLMPlatform{
		ObjectMeta: metav1.ObjectMeta{Name: "kube-llmops"},
		Spec:       v1alpha1.LLMPlatformSpec{},
	}
	vals := TranslateValues(platform)

	if _, ok := vals["ingress"]; ok {
		t.Error("ingress key should not be present when disabled")
	}
}
