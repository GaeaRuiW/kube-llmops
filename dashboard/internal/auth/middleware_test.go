package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/kube-llmops/dashboard/internal/rbac"
)

func TestRequirePermission_BlocksWithout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("permissions", []rbac.Permission{
			{Resource: "models", Action: "view"},
		})
		c.Next()
	})
	r.GET("/test", RequirePermission("models", "create"), func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestRequirePermission_AllowsWithPerm(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("permissions", []rbac.Permission{
			{Resource: "models", Action: "create"},
		})
		c.Next()
	})
	r.GET("/test", RequirePermission("models", "create"), func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestRequirePermission_NoPermsInContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// Set user but no permissions — simulates authenticated user with no perms
	r.Use(func(c *gin.Context) {
		c.Set("user", &rbac.User{})
		c.Next()
	})
	r.GET("/test", RequirePermission("models", "view"), func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestRequirePermission_DevMode_AllowsAll(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// No user, no permissions — dev mode (no OIDC)
	r.GET("/test", RequirePermission("models", "view"), func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200 in dev mode, got %d", w.Code)
	}
}
