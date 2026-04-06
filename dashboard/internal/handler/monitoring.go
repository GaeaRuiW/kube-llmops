package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kube-llmops/dashboard/internal/kube"
)

func GetMonitoringSummary(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(503, gin.H{"error": "K8s unavailable"})
			return
		}
		// Summary: list dashboards and their embed URLs
		dashboards := []gin.H{
			{"uid": "vllm-overview", "title": "vLLM Model Serving"},
			{"uid": "litellm-gateway", "title": "LiteLLM AI Gateway"},
			{"uid": "gpu-overview", "title": "GPU & Infrastructure"},
			{"uid": "rag-quality", "title": "RAG Quality (Ragas)"},
			{"uid": "cost-usage", "title": "Cost & Usage"},
			{"uid": "slo-overview", "title": "SLO Overview"},
			{"uid": "infra-roi", "title": "Infrastructure ROI"},
			{"uid": "tenant-overview", "title": "Tenant Overview"},
			{"uid": "milvus-overview", "title": "Milvus Vector DB"},
			{"uid": "system-overview", "title": "System CPU/Memory/Disk/Network"},
			{"uid": "finetune-overview", "title": "Fine-tuning Pipeline"},
		}
		c.JSON(http.StatusOK, gin.H{"dashboards": dashboards})
	}
}

func GetNotebooksSummary(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(503, gin.H{"error": "K8s unavailable"})
			return
		}
		// TODO: query JupyterHub API for active servers
		c.JSON(http.StatusOK, gin.H{"servers": []interface{}{}, "total": 0})
	}
}
