package proxy

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kube-llmops/dashboard/internal/auth"
)

// GrafanaAuth injects X-WEBAUTH-USER header for Grafana auth.proxy mode.
func GrafanaAuth(c *gin.Context, req *http.Request) {
	claims, ok := c.Get("claims")
	if !ok {
		return
	}
	cl := claims.(auth.Claims)
	req.Header.Set("X-WEBAUTH-USER", cl.Email)
	if cl.Name != "" {
		req.Header.Set("X-WEBAUTH-NAME", cl.Name)
	}
	req.Header.Set("X-WEBAUTH-EMAIL", cl.Email)
}
