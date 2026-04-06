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

	"github.com/kube-llmops/dashboard/internal/config"
	"github.com/kube-llmops/dashboard/internal/kube"
	"github.com/kube-llmops/dashboard/internal/rbac"
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

	// API routes placeholder
	api := r.Group("/api/v1")
	_ = api
	_ = kc
	_ = cfg

	// Serve embedded SPA
	staticSub, err := getStaticFS()
	if err != nil {
		log.Fatalf("static fs: %v", err)
	}
	r.StaticFS("/assets", http.FS(staticSub))
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
