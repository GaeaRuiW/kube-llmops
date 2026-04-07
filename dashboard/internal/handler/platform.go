package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kube-llmops/dashboard/internal/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// platformSummary is a simplified overview returned by GetPlatform.
type platformSummary struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	Phase      string `json:"phase"`
	Components int    `json:"componentCount"`
	Models     int    `json:"modelCount"`
}

func GetPlatform(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(503, gin.H{"error": "K8s unavailable"})
			return
		}

		// Count total deployments in the namespace with kube-llmops labels
		deploys, err := kc.Clientset.AppsV1().Deployments(kc.Namespace).List(c.Request.Context(), metav1.ListOptions{})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		totalComponents := len(deploys.Items)
		modelCount := 0
		allReady := true
		for _, d := range deploys.Items {
			if _, ok := d.Labels["kube-llmops/model"]; ok {
				modelCount++
			}
			ready := d.Status.ReadyReplicas
			desired := int32(1)
			if d.Spec.Replicas != nil {
				desired = *d.Spec.Replicas
			}
			if ready < desired {
				allReady = false
			}
		}

		phase := "Running"
		if !allReady {
			phase = "Degraded"
		}
		if totalComponents == 0 {
			phase = "NotInstalled"
		}

		c.JSON(http.StatusOK, platformSummary{
			Name:       "kube-llmops",
			Namespace:  kc.Namespace,
			Phase:      phase,
			Components: totalComponents,
			Models:     modelCount,
		})
	}
}

func UpdatePlatform(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "platform configuration is managed through Helm values; run helm upgrade to apply changes"})
	}
}

// componentInfo represents a service component's status.
type componentInfo struct {
	Name          string `json:"name"`
	Phase         string `json:"phase"`
	ReadyReplicas int32  `json:"readyReplicas"`
	TotalReplicas int32  `json:"totalReplicas"`
}

func GetComponents(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(503, gin.H{"error": "K8s unavailable"})
			return
		}
		deploys, err := kc.Clientset.AppsV1().Deployments(kc.Namespace).List(c.Request.Context(), metav1.ListOptions{})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		var components []componentInfo
		for _, d := range deploys.Items {
			desired := int32(1)
			if d.Spec.Replicas != nil {
				desired = *d.Spec.Replicas
			}
			phase := "Running"
			if d.Status.ReadyReplicas < desired {
				phase = "Progressing"
			}
			if d.Status.ReadyReplicas == 0 && desired > 0 {
				phase = "Pending"
			}
			components = append(components, componentInfo{
				Name:          d.Name,
				Phase:         phase,
				ReadyReplicas: d.Status.ReadyReplicas,
				TotalReplicas: desired,
			})
		}
		c.JSON(http.StatusOK, components)
	}
}
