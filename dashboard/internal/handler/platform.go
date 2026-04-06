package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	"github.com/kube-llmops/dashboard/internal/kube"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetPlatform(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(503, gin.H{"error": "K8s unavailable"})
			return
		}
		var list v1alpha1.LLMPlatformList
		if err := kc.CR.List(c.Request.Context(), &list, client.InNamespace(kc.Namespace)); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if len(list.Items) == 0 {
			c.JSON(404, gin.H{"error": "no LLMPlatform found"})
			return
		}
		c.JSON(http.StatusOK, list.Items[0])
	}
}

func UpdatePlatform(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(503, gin.H{"error": "K8s unavailable"})
			return
		}
		var list v1alpha1.LLMPlatformList
		if err := kc.CR.List(c.Request.Context(), &list, client.InNamespace(kc.Namespace)); err != nil || len(list.Items) == 0 {
			c.JSON(404, gin.H{"error": "no LLMPlatform found"})
			return
		}
		platform := &list.Items[0]
		var update v1alpha1.LLMPlatformSpec
		if err := c.ShouldBindJSON(&update); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		platform.Spec = update
		if err := kc.CR.Update(c.Request.Context(), platform); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, platform)
	}
}

func GetComponents(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(503, gin.H{"error": "K8s unavailable"})
			return
		}
		var list v1alpha1.LLMPlatformList
		if err := kc.CR.List(c.Request.Context(), &list, client.InNamespace(kc.Namespace)); err != nil || len(list.Items) == 0 {
			c.JSON(404, gin.H{"error": "no LLMPlatform found"})
			return
		}
		c.JSON(http.StatusOK, list.Items[0].Status.Components)
	}
}
