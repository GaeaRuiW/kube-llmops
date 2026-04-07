package proxy

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kube-llmops/dashboard/internal/auth"
)

// GrafanaAuth injects X-WEBAUTH-USER header for Grafana auth.proxy mode.
// When no OIDC claims are present (dev mode), a default admin user is injected
// so Grafana accepts the proxied request without requiring its own login.
func GrafanaAuth(c *gin.Context, req *http.Request) {
	claims, ok := c.Get("claims")
	if !ok {
		// Dev mode (no OIDC): inject default admin so Grafana doesn't redirect to login
		req.Header.Set("X-WEBAUTH-USER", "admin@kube-llmops.local")
		req.Header.Set("X-WEBAUTH-NAME", "Admin")
		req.Header.Set("X-WEBAUTH-EMAIL", "admin@kube-llmops.local")
		return
	}
	cl := claims.(auth.Claims)
	req.Header.Set("X-WEBAUTH-USER", cl.Email)
	if cl.Name != "" {
		req.Header.Set("X-WEBAUTH-NAME", cl.Name)
	}
	req.Header.Set("X-WEBAUTH-EMAIL", cl.Email)
}
