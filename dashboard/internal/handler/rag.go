package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kube-llmops/dashboard/internal/kube"
)

func ListKnowledgeBases(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(503, gin.H{"error": "K8s unavailable"})
			return
		}
		// TODO: proxy to Dify console API /console/api/datasets
		c.JSON(http.StatusOK, gin.H{"items": []interface{}{}, "total": 0})
	}
}

func CreateKnowledgeBase(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(503, gin.H{"error": "K8s unavailable"})
			return
		}
		var body struct {
			Name        string `json:"name" binding:"required"`
			Description string `json:"description"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		// TODO: proxy to Dify console API POST /console/api/datasets
		c.JSON(http.StatusCreated, gin.H{"name": body.Name, "message": "created"})
	}
}

func GetKnowledgeBase(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(503, gin.H{"error": "K8s unavailable"})
			return
		}
		id := c.Param("id")
		// TODO: proxy to Dify
		c.JSON(http.StatusOK, gin.H{"id": id})
	}
}

func DeleteKnowledgeBase(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(503, gin.H{"error": "K8s unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "deleted"})
	}
}

func UploadDocument(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(503, gin.H{"error": "K8s unavailable"})
			return
		}
		// TODO: proxy file upload to Dify
		c.JSON(http.StatusOK, gin.H{"message": "uploaded"})
	}
}

func QueryKnowledgeBase(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(503, gin.H{"error": "K8s unavailable"})
			return
		}
		var body struct {
			Query string `json:"query" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		// TODO: proxy to Dify
		c.JSON(http.StatusOK, gin.H{"query": body.Query, "results": []interface{}{}})
	}
}
