package main

import (
	"context"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/kube-llmops/dashboard/internal/auth"
	"github.com/kube-llmops/dashboard/internal/config"
	"github.com/kube-llmops/dashboard/internal/handler"
	"github.com/kube-llmops/dashboard/internal/kube"
	"github.com/kube-llmops/dashboard/internal/proxy"
	"github.com/kube-llmops/dashboard/internal/rbac"
	"github.com/kube-llmops/dashboard/internal/sse"
)

func main() {
	cfg := config.Load()

	// Database
	db, err := gorm.Open(postgres.Open(cfg.DB.DSN()), &gorm.Config{})
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	if err := db.AutoMigrate(&rbac.User{}, &rbac.Role{}, &rbac.Permission{}); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	if err := rbac.Seed(db); err != nil {
		log.Fatalf("seed: %v", err)
	}

	// K8s (warn but continue if unavailable - allows running outside cluster)
	kc, err := kube.NewClients(cfg.Namespace)
	if err != nil {
		log.Printf("WARN: K8s client unavailable: %v", err)
	}

	r := gin.Default()
	r.Use(cors.Default())

	// OIDC provider (warn-only if unavailable, for dev without Keycloak)
	var oidcProv *auth.OIDCProvider
	if cfg.OIDC.IssuerURL != "" {
		oidcProv, err = auth.NewOIDCProvider(cfg.OIDC.IssuerURL, cfg.OIDC.ClientID, cfg.OIDC.ClientSecret, cfg.OIDC.RedirectURL)
		if err != nil {
			log.Printf("WARN: OIDC provider unavailable: %v", err)
		} else {
			log.Printf("OIDC provider configured: %s", cfg.OIDC.IssuerURL)
		}
	} else {
		log.Printf("OIDC not configured (dev mode — SSO disabled)")
	}

	// SSE broker
	eventBroker := sse.NewBroker()

	// API routes
	api := r.Group("/api/v1")

	// Public auth routes (no middleware)
	api.GET("/auth/config", handler.AuthConfig(oidcProv))
	api.GET("/auth/login", handler.AuthLogin(oidcProv))
	api.GET("/auth/callback", handler.AuthCallback(oidcProv))
	api.POST("/auth/logout", handler.AuthLogout())
	api.GET("/events", sse.StreamEvents(eventBroker))

	// Protected routes
	protected := api.Group("")
	if oidcProv != nil {
		protected.Use(auth.JWTMiddleware(oidcProv, db))
	}

	// Auth (needs JWT to identify user)
	protected.GET("/auth/me", handler.GetCurrentUser(db))

	// Models
	models := protected.Group("/models")
	models.GET("", auth.RequirePermission("models", "view"), handler.ListModels(kc))
	models.POST("", auth.RequirePermission("models", "create"), handler.CreateModel(kc))
	models.GET("/:name", auth.RequirePermission("models", "view"), handler.GetModel(kc))
	models.PUT("/:name", auth.RequirePermission("models", "edit"), handler.UpdateModel(kc))
	models.DELETE("/:name", auth.RequirePermission("models", "delete"), handler.DeleteModel(kc))
	models.POST("/:name/scale", auth.RequirePermission("models", "edit"), handler.ScaleModel(kc))
	models.POST("/:name/canary", auth.RequirePermission("models", "edit"), handler.CanaryModel(kc))
	models.POST("/:name/promote", auth.RequirePermission("models", "edit"), handler.PromoteCanary(kc))
	models.POST("/:name/rollback", auth.RequirePermission("models", "edit"), handler.RollbackCanary(kc))
	models.GET("/:name/pods", auth.RequirePermission("models", "view"), handler.ListModelPods(kc))
	models.GET("/:name/logs", auth.RequirePermission("models", "view"), handler.StreamModelLogs(kc))
	models.POST("/:name/test", auth.RequirePermission("models", "edit"), handler.TestModel(kc))

	// Finetune
	ft := protected.Group("/finetune")
	ft.GET("", auth.RequirePermission("finetune", "view"), handler.ListFinetunes(kc))
	ft.POST("", auth.RequirePermission("finetune", "create"), handler.CreateFinetune(kc))
	ft.GET("/:name", auth.RequirePermission("finetune", "view"), handler.GetFinetune(kc))
	ft.DELETE("/:name", auth.RequirePermission("finetune", "delete"), handler.DeleteFinetune(kc))
	ft.GET("/:name/logs", auth.RequirePermission("finetune", "view"), handler.StreamFinetuneLogs(kc))

	// RAG
	rag := protected.Group("/rag")
	rag.GET("", auth.RequirePermission("rag", "view"), handler.ListKnowledgeBases(kc))
	rag.POST("", auth.RequirePermission("rag", "create"), handler.CreateKnowledgeBase(kc))
	rag.GET("/:id", auth.RequirePermission("rag", "view"), handler.GetKnowledgeBase(kc))
	rag.DELETE("/:id", auth.RequirePermission("rag", "delete"), handler.DeleteKnowledgeBase(kc))
	rag.POST("/:id/upload", auth.RequirePermission("rag", "edit"), handler.UploadDocument(kc))
	rag.POST("/:id/query", auth.RequirePermission("rag", "view"), handler.QueryKnowledgeBase(kc))

	// Platform
	platform := protected.Group("/platform")
	platform.GET("", auth.RequirePermission("platform", "view"), handler.GetPlatform(kc))
	platform.PUT("", auth.RequirePermission("platform", "edit"), handler.UpdatePlatform(kc))
	platform.GET("/components", auth.RequirePermission("platform", "view"), handler.GetComponents(kc))

	// Monitoring
	monitoring := protected.Group("/monitoring")
	monitoring.GET("", auth.RequirePermission("monitoring", "view"), handler.GetMonitoringSummary(kc))
	monitoring.GET("/notebooks", auth.RequirePermission("monitoring", "view"), handler.GetNotebooksSummary(kc))

	// Services
	services := protected.Group("/services")
	services.GET("", handler.ListServices(kc))
	services.GET("/:name", handler.GetServiceStatus(kc))

	// Users
	users := protected.Group("/users")
	users.GET("", auth.RequirePermission("users", "view"), handler.ListUsers(db))
	users.POST("", auth.RequirePermission("users", "create"), handler.CreateUser(db))
	users.GET("/:id", auth.RequirePermission("users", "view"), handler.GetUser(db))
	users.PUT("/:id", auth.RequirePermission("users", "edit"), handler.UpdateUser(db))
	users.DELETE("/:id", auth.RequirePermission("users", "delete"), handler.DeleteUser(db))
	users.PUT("/:id/roles", auth.RequirePermission("users", "edit"), handler.AssignRoles(db))

	// Roles
	roles := protected.Group("/roles")
	roles.GET("", auth.RequirePermission("roles", "view"), handler.ListRoles(db))
	roles.POST("", auth.RequirePermission("roles", "create"), handler.CreateRole(db))
	roles.GET("/:id", auth.RequirePermission("roles", "view"), handler.GetRole(db))
	roles.PUT("/:id", auth.RequirePermission("roles", "edit"), handler.UpdateRole(db))
	roles.DELETE("/:id", auth.RequirePermission("roles", "delete"), handler.DeleteRole(db))
	roles.PUT("/:id/permissions", auth.RequirePermission("roles", "edit"), handler.SetRolePermissions(db))

	// Permissions
	perms := protected.Group("/permissions")
	perms.GET("", auth.RequirePermission("permissions", "view"), handler.ListPermissions(db))
	perms.POST("", auth.RequirePermission("permissions", "create"), handler.CreatePermission(db))
	perms.PUT("/:id", auth.RequirePermission("permissions", "edit"), handler.UpdatePermission(db))
	perms.DELETE("/:id", auth.RequirePermission("permissions", "delete"), handler.DeletePermission(db))

	// Proxy routes
	proxy.SetupProxyRoutes(r, cfg)

	// Serve embedded SPA
	staticSub, err := getStaticFS()
	if err != nil {
		log.Fatalf("static fs: %v", err)
	}
	assetsSub, err := fs.Sub(staticSub, "assets")
	if err != nil {
		log.Fatalf("assets fs: %v", err)
	}
	r.StaticFS("/assets", http.FS(assetsSub))
	r.NoRoute(func(c *gin.Context) {
		f, err := fs.ReadFile(staticSub, "index.html")
		if err != nil {
			c.String(404, "index.html not found")
			return
		}
		c.Data(200, "text/html; charset=utf-8", f)
	})

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: r}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()
	log.Printf("Dashboard listening on :%s", cfg.Port)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}
