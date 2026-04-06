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

// ListFinetunes returns all FineTuneRuns in the configured namespace.
func ListFinetunes(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "k8s client not available"})
			return
		}
		var list v1alpha1.FineTuneRunList
		if err := kc.CR.List(c.Request.Context(), &list, client.InNamespace(kc.Namespace)); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, list.Items)
	}
}

// CreateFinetune creates a new FineTuneRun from the JSON body.
func CreateFinetune(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "k8s client not available"})
			return
		}
		var ft v1alpha1.FineTuneRun
		if err := c.ShouldBindJSON(&ft); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ft.Namespace = kc.Namespace
		if err := kc.CR.Create(c.Request.Context(), &ft); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, ft)
	}
}

// GetFinetune returns a single FineTuneRun by name.
func GetFinetune(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "k8s client not available"})
			return
		}
		name := c.Param("name")
		var ft v1alpha1.FineTuneRun
		key := client.ObjectKey{Namespace: kc.Namespace, Name: name}
		if err := kc.CR.Get(c.Request.Context(), key, &ft); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, ft)
	}
}

// DeleteFinetune deletes a FineTuneRun by name.
func DeleteFinetune(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "k8s client not available"})
			return
		}
		name := c.Param("name")
		ft := &v1alpha1.FineTuneRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: kc.Namespace,
			},
		}
		if err := kc.CR.Delete(c.Request.Context(), ft); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "deleted"})
	}
}

// StreamFinetuneLogs streams logs from the first pod matching the finetune name label via SSE.
func StreamFinetuneLogs(kc *kube.Clients) gin.HandlerFunc {
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
