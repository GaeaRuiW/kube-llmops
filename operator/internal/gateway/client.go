package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client defines the interface for communicating with the LiteLLM gateway.
type Client interface {
	RegisterModel(ctx context.Context, model GatewayModel) error
	DeregisterModel(ctx context.Context, modelID string) error
	HealthCheck(ctx context.Context) error
}

// GatewayModel represents a model registration request to LiteLLM.
type GatewayModel struct {
	ModelName     string        `json:"model_name"`
	LiteLLMParams LiteLLMParams `json:"litellm_params"`
}

// LiteLLMParams holds the LiteLLM-specific model configuration.
type LiteLLMParams struct {
	Model   string `json:"model"`
	APIBase string `json:"api_base"`
	APIKey  string `json:"api_key,omitempty"`
}

// HTTPClient implements Client using HTTP.
type HTTPClient struct {
	baseURL    string
	masterKey  string
	httpClient *http.Client
}

// NewHTTPClient creates a new LiteLLM gateway HTTP client.
func NewHTTPClient(baseURL, masterKey string) *HTTPClient {
	return &HTTPClient{
		baseURL:    baseURL,
		masterKey:  masterKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *HTTPClient) RegisterModel(ctx context.Context, model GatewayModel) error {
	body, err := json.Marshal(model)
	if err != nil {
		return fmt.Errorf("marshal model: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/model/new", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.masterKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.masterKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("register model: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("register model: HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func (c *HTTPClient) DeregisterModel(ctx context.Context, modelID string) error {
	body, _ := json.Marshal(map[string]string{"id": modelID})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/model/delete", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.masterKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.masterKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("deregister model: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("deregister model: HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func (c *HTTPClient) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("gateway unhealthy: HTTP %d", resp.StatusCode)
	}
	return nil
}

// NoopClient is a no-op implementation for testing controllers without a real gateway.
type NoopClient struct{}

func (n *NoopClient) RegisterModel(ctx context.Context, model GatewayModel) error { return nil }
func (n *NoopClient) DeregisterModel(ctx context.Context, modelID string) error   { return nil }
func (n *NoopClient) HealthCheck(ctx context.Context) error                       { return nil }
