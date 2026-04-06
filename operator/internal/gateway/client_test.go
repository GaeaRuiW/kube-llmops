package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegisterModel(t *testing.T) {
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/model/new" || r.Method != http.MethodPost {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "model added"}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "test-key")
	err := c.RegisterModel(context.Background(), GatewayModel{
		ModelName: "qwen",
		LiteLLMParams: LiteLLMParams{
			Model:   "openai/qwen",
			APIBase: "http://qwen:8000/v1",
		},
	})
	if err != nil {
		t.Fatalf("RegisterModel failed: %v", err)
	}
	if gotBody["model_name"] != "qwen" {
		t.Errorf("model_name = %v, want qwen", gotBody["model_name"])
	}
}

func TestRegisterModel_AuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "my-secret-key")
	c.RegisterModel(context.Background(), GatewayModel{ModelName: "test"})
	if gotAuth != "Bearer my-secret-key" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer my-secret-key")
	}
}

func TestRegisterModel_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "invalid model"}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "")
	err := c.RegisterModel(context.Background(), GatewayModel{ModelName: "bad"})
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
}

func TestDeregisterModel(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/model/delete" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "")
	err := c.DeregisterModel(context.Background(), "qwen")
	if err != nil {
		t.Fatalf("DeregisterModel failed: %v", err)
	}
	if !called {
		t.Error("delete endpoint not called")
	}
}

func TestHealthCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "")
	err := c.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("HealthCheck failed: %v", err)
	}
}

func TestHealthCheck_Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "")
	err := c.HealthCheck(context.Background())
	if err == nil {
		t.Error("expected error for unhealthy gateway")
	}
}

func TestNoopClient(t *testing.T) {
	c := &NoopClient{}
	if err := c.RegisterModel(context.Background(), GatewayModel{}); err != nil {
		t.Errorf("NoopClient.RegisterModel error: %v", err)
	}
	if err := c.DeregisterModel(context.Background(), "test"); err != nil {
		t.Errorf("NoopClient.DeregisterModel error: %v", err)
	}
	if err := c.HealthCheck(context.Background()); err != nil {
		t.Errorf("NoopClient.HealthCheck error: %v", err)
	}
}
