package handler

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	"github.com/kube-llmops/dashboard/internal/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ListModels returns all ModelDeployments in the configured namespace.
func ListModels(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "k8s client not available"})
			return
		}
		var list v1alpha1.ModelDeploymentList
		if err := kc.CR.List(c.Request.Context(), &list, client.InNamespace(kc.Namespace)); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, list.Items)
	}
}

// GetModel returns a single ModelDeployment by name.
func GetModel(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "k8s client not available"})
			return
		}
		name := c.Param("name")
		var md v1alpha1.ModelDeployment
		key := client.ObjectKey{Namespace: kc.Namespace, Name: name}
		if err := kc.CR.Get(c.Request.Context(), key, &md); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, md)
	}
}

// CreateModel creates a new ModelDeployment from the JSON body.
func CreateModel(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "k8s client not available"})
			return
		}
		var md v1alpha1.ModelDeployment
		if err := c.ShouldBindJSON(&md); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		md.Namespace = kc.Namespace
		if err := kc.CR.Create(c.Request.Context(), &md); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, md)
	}
}

// UpdateModel updates the spec of an existing ModelDeployment.
func UpdateModel(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "k8s client not available"})
			return
		}
		name := c.Param("name")
		var existing v1alpha1.ModelDeployment
		key := client.ObjectKey{Namespace: kc.Namespace, Name: name}
		if err := kc.CR.Get(c.Request.Context(), key, &existing); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		var updated v1alpha1.ModelDeployment
		if err := c.ShouldBindJSON(&updated); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		existing.Spec = updated.Spec
		if err := kc.CR.Update(c.Request.Context(), &existing); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, existing)
	}
}

// DeleteModel deletes a ModelDeployment by name.
func DeleteModel(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "k8s client not available"})
			return
		}
		name := c.Param("name")
		md := &v1alpha1.ModelDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: kc.Namespace,
			},
		}
		if err := kc.CR.Delete(c.Request.Context(), md); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "deleted"})
	}
}

// ScaleModel patches the replicas field of a ModelDeployment.
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
		var md v1alpha1.ModelDeployment
		key := client.ObjectKey{Namespace: kc.Namespace, Name: name}
		if err := kc.CR.Get(c.Request.Context(), key, &md); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		md.Spec.Replicas = &body.Replicas
		if err := kc.CR.Update(c.Request.Context(), &md); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, md)
	}
}

// CanaryModel sets a canary configuration on a ModelDeployment.
func CanaryModel(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "k8s client not available"})
			return
		}
		name := c.Param("name")
		var canary v1alpha1.CanaryConfig
		if err := c.ShouldBindJSON(&canary); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		var md v1alpha1.ModelDeployment
		key := client.ObjectKey{Namespace: kc.Namespace, Name: name}
		if err := kc.CR.Get(c.Request.Context(), key, &md); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		md.Spec.Canary = &canary
		if err := kc.CR.Update(c.Request.Context(), &md); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, md)
	}
}

// PromoteCanary promotes the canary source to the main source and clears canary config.
func PromoteCanary(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "k8s client not available"})
			return
		}
		name := c.Param("name")
		var md v1alpha1.ModelDeployment
		key := client.ObjectKey{Namespace: kc.Namespace, Name: name}
		if err := kc.CR.Get(c.Request.Context(), key, &md); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if md.Spec.Canary == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no canary configured"})
			return
		}
		md.Spec.Source = md.Spec.Canary.Source
		md.Spec.Canary = nil
		if err := kc.CR.Update(c.Request.Context(), &md); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, md)
	}
}

// RollbackCanary clears the canary configuration from a ModelDeployment.
func RollbackCanary(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "k8s client not available"})
			return
		}
		name := c.Param("name")
		var md v1alpha1.ModelDeployment
		key := client.ObjectKey{Namespace: kc.Namespace, Name: name}
		if err := kc.CR.Get(c.Request.Context(), key, &md); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		md.Spec.Canary = nil
		if err := kc.CR.Update(c.Request.Context(), &md); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, md)
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
			LabelSelector: "app.kubernetes.io/name=" + name,
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
			LabelSelector: "app.kubernetes.io/name=" + name,
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

// TestModel returns the endpoint for a ModelDeployment (placeholder for inference testing).
func TestModel(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "k8s client not available"})
			return
		}
		name := c.Param("name")
		var md v1alpha1.ModelDeployment
		key := client.ObjectKey{Namespace: kc.Namespace, Name: name}
		if err := kc.CR.Get(c.Request.Context(), key, &md); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"model":    name,
			"endpoint": md.Status.Endpoint,
			"message":  "use this endpoint to test inference",
		})
	}
}
