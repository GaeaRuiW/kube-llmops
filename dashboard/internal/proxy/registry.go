package proxy

import (
	"github.com/gin-gonic/gin"
	"github.com/kube-llmops/dashboard/internal/config"
)

// SetupProxyRoutes registers all service proxy routes on the Gin engine.
func SetupProxyRoutes(r *gin.Engine, cfg *config.Config) {
	// Grafana — auth.proxy with X-WEBAUTH-USER header
	r.Any("/services/grafana/*path", NewReverseProxy(
		cfg.Proxy.Grafana, "/services/grafana", GrafanaAuth,
	))

	// Langfuse — Bearer token passthrough
	r.Any("/services/langfuse/*path", NewReverseProxy(
		cfg.Proxy.Langfuse, "/services/langfuse", BearerAuth,
	))

	// Dify — no auth injection (uses its own session cookies)
	r.Any("/services/dify/*path", NewReverseProxy(
		cfg.Proxy.Dify, "/services/dify", nil,
	))

	// MLflow — no auth needed
	r.Any("/services/mlflow/*path", NewReverseProxy(
		cfg.Proxy.MLflow, "/services/mlflow", nil,
	))

	// JupyterHub — Bearer token
	r.Any("/services/jupyterhub/*path", NewReverseProxy(
		cfg.Proxy.JupyterHub, "/services/jupyterhub", BearerAuth,
	))

	// MinIO — no auth injection (uses session cookies)
	r.Any("/services/minio/*path", NewReverseProxy(
		cfg.Proxy.MinIO, "/services/minio", nil,
	))

	// Keycloak — no auth injection
	r.Any("/services/keycloak/*path", NewReverseProxy(
		cfg.Proxy.Keycloak, "/services/keycloak", nil,
	))

	// LiteLLM — master key auth
	r.Any("/services/litellm/*path", NewReverseProxy(
		cfg.Proxy.LiteLLM, "/services/litellm", LiteLLMAuth,
	))

	// Prometheus — no auth needed
	r.Any("/services/prometheus/*path", NewReverseProxy(
		cfg.Proxy.Prometheus, "/services/prometheus", nil,
	))
}
