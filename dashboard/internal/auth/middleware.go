package auth

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/kube-llmops/dashboard/internal/rbac"
)

type Claims struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

func JWTMiddleware(oidcProv *OIDCProvider, db *gorm.DB) gin.HandlerFunc {
	svc := rbac.NewService(db)
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}
		raw := strings.TrimPrefix(header, "Bearer ")

		idToken, err := oidcProv.Verify(c.Request.Context(), raw)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		var claims Claims
		if err := idToken.Claims(&claims); err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "bad claims"})
			return
		}

		user, err := svc.EnsureUser(claims.Sub, claims.Email, claims.Name)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "user sync failed"})
			return
		}
		now := time.Now()
		db.Model(user).Update("last_login", &now)

		perms, _ := svc.GetUserPermissions(user.ID)

		c.Set("user", user)
		c.Set("claims", claims)
		c.Set("permissions", perms)
		c.Next()
	}
}

func RequirePermission(resource, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		perms, ok := c.Get("permissions")
		if !ok {
			// No JWT middleware active (dev mode / no OIDC) — allow all
			if _, hasUser := c.Get("user"); !hasUser {
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "no permissions"})
			return
		}
		for _, p := range perms.([]rbac.Permission) {
			if p.Resource == resource && p.Action == action {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
	}
}
