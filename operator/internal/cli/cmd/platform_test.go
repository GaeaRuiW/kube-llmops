package cmd

import (
	"testing"
)

func TestBuildLLMPlatform_Defaults(t *testing.T) {
	lp := buildLLMPlatform(true, true, false, false, false, false, "")
	if !lp.Spec.Gateway.Enabled {
		t.Error("expected gateway enabled")
	}
	if !lp.Spec.Observability.Enabled {
		t.Error("expected observability enabled")
	}
	if !lp.Spec.ModelStore.Enabled {
		t.Error("expected modelStore enabled")
	}
	if lp.Spec.ModelStore.Endpoint != "kube-llmops-minio:9000" {
		t.Errorf("expected default modelStore endpoint, got %q", lp.Spec.ModelStore.Endpoint)
	}
	if !lp.Spec.PostgreSQL.Enabled {
		t.Error("expected postgresql enabled")
	}
	if lp.Spec.Modules.RAG.Enabled {
		t.Error("expected RAG disabled by default")
	}
}

func TestBuildLLMPlatform_WithNodePort(t *testing.T) {
	lp := buildLLMPlatform(true, true, false, false, false, false, "10.0.0.1")
	if !lp.Spec.NodePort.Enabled {
		t.Error("expected nodePort enabled when host provided")
	}
	if lp.Spec.NodePort.Host != "10.0.0.1" {
		t.Errorf("expected nodePort host 10.0.0.1, got %q", lp.Spec.NodePort.Host)
	}
}

func TestBuildLLMPlatform_AllModules(t *testing.T) {
	lp := buildLLMPlatform(true, true, true, true, true, true, "")
	if !lp.Spec.Modules.RAG.Enabled {
		t.Error("expected RAG enabled")
	}
	if !lp.Spec.Modules.Finetune.Enabled {
		t.Error("expected finetune enabled")
	}
	if !lp.Spec.Modules.Security.Enabled {
		t.Error("expected security enabled")
	}
	if !lp.Spec.Logging.Enabled {
		t.Error("expected logging enabled")
	}
}
