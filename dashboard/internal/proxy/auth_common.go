package proxy

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// BearerAuth forwards the Authorization header from the original request.
func BearerAuth(c *gin.Context, req *http.Request) {
	if auth := c.GetHeader("Authorization"); auth != "" {
		req.Header.Set("Authorization", auth)
	}
}
