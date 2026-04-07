package handler

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kube-llmops/dashboard/internal/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ServiceInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Phase       string `json:"phase"`
	Endpoint    string `json:"endpoint,omitempty"`
	ProxyPath   string `json:"proxyPath"`
}

// serviceRegistry maps logical service names to their Kubernetes workload
// name suffix and metadata. The workload name is: kube-llmops-{deployKey}.
// Only services with a non-empty ProxyPath are shown to users (the proxy
// route must exist in proxy/registry.go for the iframe embed to work).
var serviceRegistry = []struct {
	Name        string
	Description string
	Icon        string
	DeployKey   string // suffix after "kube-llmops-" in the workload name
	ProxyPath   string
}{
	{"grafana", "Monitoring Dashboards", "dashboard", "grafana", "/services/grafana/"},
	{"langfuse", "LLM Tracing & Analytics", "search", "langfuse", "/services/langfuse/"},
	{"dify", "RAG Platform", "robot", "dify-api", "/services/dify/"},
	{"mlflow", "Experiment Tracking", "experiment", "mlflow", "/services/mlflow/"},
	{"minio", "Object Storage", "database", "minio", "/services/minio/"},
	{"keycloak", "Identity Management", "lock", "keycloak", "/services/keycloak/"},
	{"litellm", "AI Gateway", "api", "litellm", "/services/litellm/"},
	{"prometheus", "Metrics Query", "bar-chart", "prometheus", "/services/prometheus/"},
}

// workloadPhase checks a Kubernetes Deployment or StatefulSet and returns its
// health phase as one of: "Running", "Progressing", "Failed", or "NotFound".
func workloadPhase(ctx context.Context, kc *kube.Clients, name string) string {
	// Try Deployment first.
	deploy, err := kc.Clientset.AppsV1().Deployments(kc.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		ready := deploy.Status.ReadyReplicas
		desired := int32(1)
		if deploy.Spec.Replicas != nil {
			desired = *deploy.Spec.Replicas
		}
		if ready >= desired && desired > 0 {
			return "Running"
		}
		for _, cond := range deploy.Status.Conditions {
			if cond.Type == "Progressing" && cond.Status == "True" {
				return "Progressing"
			}
		}
		return "Failed"
	}

	// Fallback: try StatefulSet.
	sts, err := kc.Clientset.AppsV1().StatefulSets(kc.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		ready := sts.Status.ReadyReplicas
		desired := int32(1)
		if sts.Spec.Replicas != nil {
			desired = *sts.Spec.Replicas
		}
		if ready >= desired && desired > 0 {
			return "Running"
		}
		return "Progressing"
	}

	return "NotFound"
}

func ListServices(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(503, gin.H{"error": "K8s unavailable"})
			return
		}

		prefix := "kube-llmops"

		var services []ServiceInfo
		for _, sr := range serviceRegistry {
			svc := ServiceInfo{
				Name:        sr.Name,
				Description: sr.Description,
				Icon:        sr.Icon,
				Phase:       "NotFound",
				ProxyPath:   sr.ProxyPath,
			}

			workloadName := fmt.Sprintf("%s-%s", prefix, sr.DeployKey)
			svc.Phase = workloadPhase(c.Request.Context(), kc, workloadName)

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
