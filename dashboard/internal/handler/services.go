package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	"github.com/kube-llmops/dashboard/internal/kube"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ServiceInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Phase       string `json:"phase"`
	Endpoint    string `json:"endpoint,omitempty"`
	ProxyPath   string `json:"proxyPath"`
}

var serviceRegistry = []struct {
	Name        string
	Description string
	Icon        string
	Component   string
	ProxyPath   string
}{
	{"grafana", "Monitoring Dashboards", "dashboard", "grafana", "/services/grafana/"},
	{"langfuse", "LLM Tracing & Analytics", "search", "langfuse", "/services/langfuse/"},
	{"dify", "RAG Platform", "robot", "dify", "/services/dify/"},
	{"mlflow", "Experiment Tracking", "experiment", "mlflow", "/services/mlflow/"},
	{"jupyterhub", "Notebook Development", "code", "jupyterhub", "/services/jupyterhub/"},
	{"minio", "Object Storage", "database", "minio", "/services/minio/"},
	{"keycloak", "Identity Management", "lock", "keycloak", "/services/keycloak/"},
	{"litellm", "AI Gateway", "api", "gateway", "/services/litellm/"},
	{"prometheus", "Metrics Query", "bar-chart", "prometheus", "/services/prometheus/"},
}

func ListServices(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(503, gin.H{"error": "K8s unavailable"})
			return
		}
		var platforms v1alpha1.LLMPlatformList
		kc.CR.List(c.Request.Context(), &platforms, client.InNamespace(kc.Namespace))

		components := map[string]*v1alpha1.ComponentStatus{}
		if len(platforms.Items) > 0 {
			cs := platforms.Items[0].Status.Components
			components["grafana"] = cs.Grafana
			components["langfuse"] = cs.Langfuse
			components["dify"] = cs.Dify
			components["minio"] = cs.MinIO
			components["gateway"] = cs.Gateway
			components["prometheus"] = cs.Prometheus
		}

		var services []ServiceInfo
		for _, sr := range serviceRegistry {
			svc := ServiceInfo{
				Name:        sr.Name,
				Description: sr.Description,
				Icon:        sr.Icon,
				Phase:       "Unknown",
				ProxyPath:   sr.ProxyPath,
			}
			if cs, ok := components[sr.Component]; ok && cs != nil {
				svc.Phase = cs.Phase
				svc.Endpoint = cs.Endpoint
			}
			services = append(services, svc)
		}
		c.JSON(http.StatusOK, services)
	}
}

func GetServiceStatus(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		for _, sr := range serviceRegistry {
			if sr.Name == name {
				c.JSON(http.StatusOK, gin.H{"name": name, "proxyPath": sr.ProxyPath})
				return
			}
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown service"})
	}
}
