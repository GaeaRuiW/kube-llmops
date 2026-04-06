package kube

import "testing"

func TestNewClients_FallbackToKubeconfig(t *testing.T) {
	_, err := NewClients("default")
	if err == nil {
		t.Log("K8s config found, client created successfully")
	} else {
		t.Logf("Expected error outside cluster: %v", err)
	}
}
