package util

// ServiceInfo holds the K8s service name and port for a platform component.
type ServiceInfo struct {
	Name      string // K8s Service name
	Port      int32  // Target port on the service
	LocalPort int32  // Local port for port-forward
}

// ServiceMap maps friendly aliases to K8s service details.
var ServiceMap = map[string]ServiceInfo{
	"gateway":  {Name: "kube-llmops-litellm", Port: 4000, LocalPort: 4000},
	"grafana":  {Name: "kube-llmops-grafana", Port: 3000, LocalPort: 3000},
	"langfuse": {Name: "kube-llmops-langfuse", Port: 3000, LocalPort: 3001},
	"dify":     {Name: "kube-llmops-dify-web", Port: 3000, LocalPort: 5001},
	"minio":    {Name: "kube-llmops-minio", Port: 9001, LocalPort: 9001},
}
