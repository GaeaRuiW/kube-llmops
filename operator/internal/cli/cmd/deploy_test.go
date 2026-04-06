package cmd

import (
	"testing"
)

func TestBuildModelDeployment_Basic(t *testing.T) {
	md := buildModelDeployment("Qwen/Qwen2.5-7B-Instruct", "", "auto", 1, 1, "16Gi", "4", "nvidia", nil, false)
	if md.Name != "qwen2.5-7b-instruct" {
		t.Errorf("expected name %q, got %q", "qwen2.5-7b-instruct", md.Name)
	}
	if md.Spec.Source != "Qwen/Qwen2.5-7B-Instruct" {
		t.Errorf("expected source preserved, got %q", md.Spec.Source)
	}
	if *md.Spec.Replicas != 1 {
		t.Errorf("expected replicas 1, got %d", *md.Spec.Replicas)
	}
}

func TestBuildModelDeployment_CustomName(t *testing.T) {
	md := buildModelDeployment("Qwen/Qwen2.5-7B-Instruct", "my-model", "vllm", 2, 2, "32Gi", "8", "amd", nil, false)
	if md.Name != "my-model" {
		t.Errorf("expected custom name %q, got %q", "my-model", md.Name)
	}
	if md.Spec.Engine != "vllm" {
		t.Errorf("expected engine vllm, got %q", md.Spec.Engine)
	}
	if md.Spec.Resources.GPU != 2 {
		t.Errorf("expected 2 GPU, got %d", md.Spec.Resources.GPU)
	}
	if md.Spec.Accelerator != "amd" {
		t.Errorf("expected amd accelerator, got %q", md.Spec.Accelerator)
	}
}

func TestBuildModelDeployment_EngineArgs(t *testing.T) {
	args := []string{"--max-model-len=4096", "--dtype=float16"}
	md := buildModelDeployment("Qwen/Qwen2.5-7B", "", "auto", 1, 1, "16Gi", "4", "nvidia", args, false)
	if md.Spec.EngineArgs["--max-model-len"] != "4096" {
		t.Errorf("expected engine arg --max-model-len=4096, got %v", md.Spec.EngineArgs)
	}
	if md.Spec.EngineArgs["--dtype"] != "float16" {
		t.Errorf("expected engine arg --dtype=float16, got %v", md.Spec.EngineArgs)
	}
}

func TestBuildModelDeployment_PrefixCaching(t *testing.T) {
	md := buildModelDeployment("Qwen/Qwen2.5-7B", "", "auto", 1, 1, "16Gi", "4", "nvidia", nil, true)
	if !md.Spec.PrefixCaching {
		t.Error("expected prefixCaching=true")
	}
}

func TestParseEngineArgs(t *testing.T) {
	tests := []struct {
		input []string
		key   string
		val   string
	}{
		{[]string{"--max-model-len=4096"}, "--max-model-len", "4096"},
		{[]string{"--dtype=float16"}, "--dtype", "float16"},
		{[]string{"key=value"}, "key", "value"},
	}
	for _, tt := range tests {
		m := parseEngineArgs(tt.input)
		if m[tt.key] != tt.val {
			t.Errorf("parseEngineArgs(%v): expected %s=%s, got %v", tt.input, tt.key, tt.val, m)
		}
	}
}
