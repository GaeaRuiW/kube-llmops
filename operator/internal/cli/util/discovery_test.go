package util

import "testing"

func TestServiceMapping(t *testing.T) {
	tests := []struct {
		alias   string
		svcName string
		port    int32
	}{
		{"gateway", "kube-llmops-litellm", 4000},
		{"grafana", "kube-llmops-grafana", 3000},
		{"langfuse", "kube-llmops-langfuse", 3000},
		{"dify", "kube-llmops-dify-web", 3000},
		{"minio", "kube-llmops-minio", 9001},
	}
	for _, tt := range tests {
		t.Run(tt.alias, func(t *testing.T) {
			info, ok := ServiceMap[tt.alias]
			if !ok {
				t.Fatalf("service alias %q not found in ServiceMap", tt.alias)
			}
			if info.Name != tt.svcName {
				t.Errorf("expected service name %q, got %q", tt.svcName, info.Name)
			}
			if info.Port != tt.port {
				t.Errorf("expected port %d, got %d", tt.port, info.Port)
			}
		})
	}
}

func TestLocalPortMapping(t *testing.T) {
	tests := []struct {
		alias     string
		localPort int32
	}{
		{"gateway", 4000},
		{"grafana", 3000},
		{"langfuse", 3001},
		{"dify", 5001},
		{"minio", 9001},
	}
	for _, tt := range tests {
		t.Run(tt.alias, func(t *testing.T) {
			info := ServiceMap[tt.alias]
			if info.LocalPort != tt.localPort {
				t.Errorf("expected local port %d, got %d", tt.localPort, info.LocalPort)
			}
		})
	}
}
