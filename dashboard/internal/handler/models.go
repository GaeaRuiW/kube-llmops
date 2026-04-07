package handler

import (
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kube-llmops/dashboard/internal/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// modelResponse is the JSON shape the frontend (types/index.ts ModelDeployment) expects.
type modelResponse struct {
	Metadata modelMeta   `json:"metadata"`
	Spec     modelSpec   `json:"spec"`
	Status   modelStatus `json:"status"`
}

type modelMeta struct {
	Name              string `json:"name"`
	Namespace         string `json:"namespace"`
	CreationTimestamp string `json:"creationTimestamp"`
}

type modelSpec struct {
	Source    string            `json:"source"`
	Engine   string            `json:"engine"`
	Replicas int32             `json:"replicas"`
	Resources modelResources   `json:"resources"`
}

type modelResources struct {
	GPU    string `json:"gpu"`
	Memory string `json:"memory"`
	CPU    string `json:"cpu"`
}

type modelStatus struct {
	Phase         string `json:"phase"`
	Engine        string `json:"engine"`
	Endpoint      string `json:"endpoint"`
	ReadyReplicas int32  `json:"readyReplicas"`
	TotalReplicas int32  `json:"totalReplicas"`
}

// ListModels returns model serving Deployments (label kube-llmops/model).
func ListModels(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "k8s client not available"})
			return
		}
		deploys, err := kc.Clientset.AppsV1().Deployments(kc.Namespace).List(c.Request.Context(), metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/component=model-serving",
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		models := make([]modelResponse, 0, len(deploys.Items))
		for _, d := range deploys.Items {
			labels := d.Labels
			modelName := labels["kube-llmops/model"]
			engine := labels["kube-llmops/engine"]
			if modelName == "" {
				continue
			}

			// Derive source from init container MODEL_SOURCE env var
			source := ""
			for _, init := range d.Spec.Template.Spec.InitContainers {
				for _, env := range init.Env {
					if env.Name == "MODEL_SOURCE" {
						source = env.Value
						break
					}
				}
			}

			replicas := int32(1)
			if d.Spec.Replicas != nil {
				replicas = *d.Spec.Replicas
			}

			// Resources from the main container
			gpu := "0"
			memory := ""
			cpu := ""
			if len(d.Spec.Template.Spec.Containers) > 0 {
				res := d.Spec.Template.Spec.Containers[0].Resources
				if g, ok := res.Limits["nvidia.com/gpu"]; ok {
					gpu = g.String()
				} else if g, ok := res.Requests["nvidia.com/gpu"]; ok {
					gpu = g.String()
				}
				if m, ok := res.Requests[corev1.ResourceMemory]; ok {
					memory = m.String()
				}
				if c, ok := res.Requests[corev1.ResourceCPU]; ok {
					cpu = c.String()
				}
			}

			// Derive phase from deployment conditions
			phase := "Pending"
			for _, cond := range d.Status.Conditions {
				if cond.Type == "Available" && cond.Status == "True" {
					phase = "Ready"
					break
				}
				if cond.Type == "Progressing" && cond.Status == "True" {
					phase = "Progressing"
				}
			}
			if d.Status.ReadyReplicas == 0 && d.Status.Replicas > 0 {
				phase = "Progressing"
			}

			// Derive endpoint from deployment name + engine port
			port := "8000"
			if engine == "tei" {
				port = "8080"
			} else if engine == "llamacpp" {
				port = "8080"
			}
			endpoint := fmt.Sprintf("http://%s:%s", d.Name, port)

			models = append(models, modelResponse{
				Metadata: modelMeta{
					Name:              modelName,
					Namespace:         d.Namespace,
					CreationTimestamp: d.CreationTimestamp.Format("2006-01-02T15:04:05Z"),
				},
				Spec: modelSpec{
					Source:    source,
					Engine:   engine,
					Replicas: replicas,
					Resources: modelResources{GPU: gpu, Memory: memory, CPU: cpu},
				},
				Status: modelStatus{
					Phase:         phase,
					Engine:        engine,
					Endpoint:      endpoint,
					ReadyReplicas: d.Status.ReadyReplicas,
					TotalReplicas: replicas,
				},
			})
		}
		c.JSON(http.StatusOK, models)
	}
}

// GetModel returns a single model by name (label kube-llmops/model=<name>).
func GetModel(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "k8s client not available"})
			return
		}
		name := c.Param("name")
		deploys, err := kc.Clientset.AppsV1().Deployments(kc.Namespace).List(c.Request.Context(), metav1.ListOptions{
			LabelSelector: "kube-llmops/model=" + name,
		})
		if err != nil || len(deploys.Items) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "model not found"})
			return
		}
		d := deploys.Items[0]
		// Reuse ListModels logic via a simple redirect to the list with a filter
		// For now, return the deployment directly
		c.JSON(http.StatusOK, gin.H{
			"metadata": gin.H{"name": name, "namespace": d.Namespace, "creationTimestamp": d.CreationTimestamp},
			"deployment": d.Name,
			"status":     gin.H{"readyReplicas": d.Status.ReadyReplicas, "replicas": d.Status.Replicas},
		})
	}
}

// CreateModel is a placeholder — model creation is done through Helm values.
func CreateModel(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "model creation is managed through Helm values; update global.models and run helm upgrade"})
	}
}

// UpdateModel is a placeholder — model updates are done through Helm values.
func UpdateModel(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "model updates are managed through Helm values"})
	}
}

// DeleteModel is a placeholder — model deletion is done through Helm values.
func DeleteModel(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "model deletion is managed through Helm values"})
	}
}

// ScaleModel patches the replicas on the underlying Deployment.
func ScaleModel(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "k8s client not available"})
			return
		}
		name := c.Param("name")
		var body struct {
			Replicas int32 `json:"replicas"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		deploys, err := kc.Clientset.AppsV1().Deployments(kc.Namespace).List(c.Request.Context(), metav1.ListOptions{
			LabelSelector: "kube-llmops/model=" + name,
		})
		if err != nil || len(deploys.Items) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "model not found"})
			return
		}
		d := &deploys.Items[0]
		d.Spec.Replicas = &body.Replicas
		_, err = kc.Clientset.AppsV1().Deployments(kc.Namespace).Update(c.Request.Context(), d, metav1.UpdateOptions{})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("scaled %s to %d replicas", name, body.Replicas)})
	}
}

// CanaryModel is a placeholder — canary is managed through LiteLLM weight routing.
func CanaryModel(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "canary deployment is managed through LiteLLM weight routing"})
	}
}

// PromoteCanary is a placeholder.
func PromoteCanary(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "canary promotion is managed through LiteLLM"})
	}
}

// RollbackCanary is a placeholder.
func RollbackCanary(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "canary rollback is managed through LiteLLM"})
	}
}

// ListModelPods lists pods matching the given model name label.
func ListModelPods(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "k8s client not available"})
			return
		}
		name := c.Param("name")
		pods, err := kc.Clientset.CoreV1().Pods(kc.Namespace).List(c.Request.Context(), metav1.ListOptions{
			LabelSelector: "kube-llmops/model=" + name,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		type podInfo struct {
			Name  string `json:"name"`
			Phase string `json:"phase"`
			Node  string `json:"node"`
			Ready bool   `json:"ready"`
		}
		result := make([]podInfo, 0, len(pods.Items))
		for _, p := range pods.Items {
			ready := false
			for _, cond := range p.Status.Conditions {
				if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
					ready = true
					break
				}
			}
			result = append(result, podInfo{
				Name:  p.Name,
				Phase: string(p.Status.Phase),
				Node:  p.Spec.NodeName,
				Ready: ready,
			})
		}
		c.JSON(http.StatusOK, result)
	}
}

// StreamModelLogs streams logs from the first pod matching the model name label via SSE.
func StreamModelLogs(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "k8s client not available"})
			return
		}
		name := c.Param("name")
		pods, err := kc.Clientset.CoreV1().Pods(kc.Namespace).List(c.Request.Context(), metav1.ListOptions{
			LabelSelector: "kube-llmops/model=" + name,
		})
		if err != nil || len(pods.Items) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "no pods found"})
			return
		}
		podName := pods.Items[0].Name
		tailLines := int64(100)
		req := kc.Clientset.CoreV1().Pods(kc.Namespace).GetLogs(podName, &corev1.PodLogOptions{
			Follow:    true,
			TailLines: &tailLines,
		})
		stream, err := req.Stream(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer stream.Close()

		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Flush()

		buf := make([]byte, 4096)
		for {
			n, err := stream.Read(buf)
			if n > 0 {
				c.SSEvent("log", string(buf[:n]))
				c.Writer.Flush()
			}
			if err != nil {
				if err != io.EOF {
					c.SSEvent("error", err.Error())
					c.Writer.Flush()
				}
				return
			}
		}
	}
}

// TestModel returns the endpoint for a model (by looking up its Deployment).
func TestModel(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "k8s client not available"})
			return
		}
		name := c.Param("name")
		deploys, err := kc.Clientset.AppsV1().Deployments(kc.Namespace).List(c.Request.Context(), metav1.ListOptions{
			LabelSelector: "kube-llmops/model=" + name,
		})
		if err != nil || len(deploys.Items) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "model not found"})
			return
		}
		d := deploys.Items[0]
		engine := d.Labels["kube-llmops/engine"]
		port := "8000"
		if engine == "tei" || engine == "llamacpp" {
			port = "8080"
		}
		endpoint := fmt.Sprintf("http://%s:%s", d.Name, port)
		c.JSON(http.StatusOK, gin.H{
			"model":    name,
			"endpoint": endpoint,
			"message":  "use this endpoint to test inference",
		})
	}
}
