package proxy

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// LiteLLMAuth injects the master key for LiteLLM API access.
func LiteLLMAuth(_ *gin.Context, req *http.Request) {
	masterKey := os.Getenv("LITELLM_MASTER_KEY")
	if masterKey == "" {
		masterKey = "sk-llmops-master-key"
	}
	req.Header.Set("Authorization", "Bearer "+masterKey)
}
