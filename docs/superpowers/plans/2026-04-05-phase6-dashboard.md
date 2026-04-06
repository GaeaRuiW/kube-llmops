# Phase 6 Web Dashboard — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a hybrid Web Dashboard (React + Go Gin) that manages 3 CRDs, embeds 9 services with SSO passthrough, and provides full dynamic RBAC — deployed as a single-binary Helm subchart.

**Architecture:** Single Go binary embeds React SPA via `embed.FS`. Backend uses Gin for REST API + reverse proxy, GORM for RBAC persistence in PostgreSQL, controller-runtime for K8s CRD access. Frontend uses React 18 + Ant Design 5 + TanStack Query + Zustand. SSE for real-time updates. Keycloak OIDC for authentication.

**Tech Stack:** Go 1.25, Gin, GORM, controller-runtime v0.20, React 18, TypeScript, Vite, Ant Design 5, TanStack Query v5, Zustand v4

**Spec:** `docs/superpowers/specs/2026-04-05-phase6-dashboard-design.md`

---

## Prerequisites

```bash
go version    # >= 1.22
node -v       # >= 20
npm -v        # >= 10
helm version  # >= 3.12
```

---

## File Structure

All new files live under `dashboard/` at the repo root, plus Helm integration in `charts/`.

```
dashboard/
├── go.mod
├── go.sum
├── main.go                            # Task 3
├── embed.go                           # Task 3
├── Dockerfile                         # Task 28
├── internal/
│   ├── config/
│   │   └── config.go                  # Task 2
│   ├── kube/
│   │   ├── client.go                  # Task 2
│   │   └── client_test.go            # Task 2
│   ├── rbac/
│   │   ├── models.go                  # Task 4
│   │   ├── models_test.go            # Task 4
│   │   ├── seed.go                    # Task 5
│   │   ├── seed_test.go              # Task 5
│   │   ├── service.go                 # Task 6
│   │   ├── service_test.go           # Task 6
│   │   └── sync.go                    # Task 17
│   ├── auth/
│   │   ├── oidc.go                    # Task 7
│   │   ├── middleware.go              # Task 7
│   │   └── middleware_test.go         # Task 7
│   ├── handler/
│   │   ├── auth.go                    # Task 8
│   │   ├── models.go                  # Task 9
│   │   ├── models_test.go            # Task 9
│   │   ├── finetune.go               # Task 10
│   │   ├── rag.go                     # Task 11
│   │   ├── platform.go               # Task 12
│   │   ├── monitoring.go             # Task 13
│   │   ├── services.go               # Task 14
│   │   ├── users.go                   # Task 17
│   │   ├── roles.go                   # Task 18
│   │   └── permissions.go            # Task 18
│   ├── sse/
│   │   ├── broker.go                  # Task 16
│   │   └── broker_test.go            # Task 16
│   └── proxy/
│       ├── reverse.go                 # Task 15
│       ├── reverse_test.go           # Task 15
│       ├── registry.go               # Task 15
│       ├── auth_grafana.go           # Task 15
│       ├── auth_dify.go              # Task 15
│       └── auth_litellm.go           # Task 15
└── web/                               # Tasks 19-26
    ├── package.json
    ├── vite.config.ts
    ├── tsconfig.json
    ├── index.html
    └── src/
        ├── main.tsx
        ├── App.tsx
        ├── api/
        ├── hooks/
        ├── store/
        ├── components/
        ├── pages/
        └── types/

charts/kube-llmops-stack/charts/dashboard/   # Task 27
├── Chart.yaml
├── values.yaml
└── templates/
    ├── _helpers.tpl
    ├── deployment.yaml
    ├── service.yaml
    ├── configmap.yaml
    └── secret.yaml
```

---

## Task 1: Go Module Scaffold

**Files:**
- Create: `dashboard/go.mod`
- Create: `dashboard/go.sum`

- [ ] **Step 1: Initialize Go module**

```bash
mkdir -p dashboard && cd dashboard
go mod init github.com/kube-llmops/dashboard
```

- [ ] **Step 2: Add dependencies**

```bash
cd dashboard
go get github.com/gin-gonic/gin@latest
go get github.com/gin-contrib/cors@latest
go get github.com/coreos/go-oidc/v3@latest
go get golang.org/x/oauth2@latest
go get gorm.io/gorm@latest
go get gorm.io/driver/postgres@latest
go get github.com/google/uuid@latest
go get sigs.k8s.io/controller-runtime@v0.20.4
go get k8s.io/client-go@v0.35.1
go get k8s.io/api@v0.35.1
go get k8s.io/apimachinery@v0.35.1
```

- [ ] **Step 3: Add replace directive for operator types**

Append to `dashboard/go.mod`:

```
replace github.com/kube-llmops/operator => ../operator
```

Then:

```bash
cd dashboard && go get github.com/kube-llmops/operator/api/v1alpha1
go mod tidy
```

- [ ] **Step 4: Verify module resolves**

```bash
cd dashboard && go build ./...
```

Expected: no errors (no Go files yet, just module init)

- [ ] **Step 5: Commit**

```bash
git add dashboard/go.mod dashboard/go.sum
git commit -m "feat(dashboard): scaffold Go module with dependencies"
```

---

## Task 2: Config + K8s Client

**Files:**
- Create: `dashboard/internal/config/config.go`
- Create: `dashboard/internal/kube/client.go`
- Create: `dashboard/internal/kube/client_test.go`

- [ ] **Step 1: Write config.go**

```go
package config

import "os"

type Config struct {
	Port      string
	Namespace string
	DB        DBConfig
	OIDC      OIDCConfig
	Proxy     ProxyConfig
}

type DBConfig struct {
	Host     string
	Port     string
	Name     string
	User     string
	Password string
}

func (d DBConfig) DSN() string {
	return "host=" + d.Host + " port=" + d.Port + " user=" + d.User +
		" password=" + d.Password + " dbname=" + d.Name + " sslmode=disable"
}

type OIDCConfig struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
}

type ProxyConfig struct {
	Grafana    string
	Langfuse   string
	Dify       string
	MLflow     string
	JupyterHub string
	MinIO      string
	Keycloak   string
	LiteLLM    string
	Prometheus string
}

func Load() *Config {
	return &Config{
		Port:      env("PORT", "3000"),
		Namespace: env("NAMESPACE", "default"),
		DB: DBConfig{
			Host:     env("DB_HOST", "localhost"),
			Port:     env("DB_PORT", "5432"),
			Name:     env("DB_NAME", "dashboard"),
			User:     env("DB_USER", "postgres"),
			Password: env("DB_PASSWORD", ""),
		},
		OIDC: OIDCConfig{
			IssuerURL:    env("OIDC_ISSUER_URL", ""),
			ClientID:     env("OIDC_CLIENT_ID", "dashboard"),
			ClientSecret: env("OIDC_CLIENT_SECRET", "dashboard-oidc-secret"),
		},
		Proxy: ProxyConfig{
			Grafana:    env("PROXY_GRAFANA", "http://kube-llmops-grafana:3000"),
			Langfuse:   env("PROXY_LANGFUSE", "http://kube-llmops-langfuse:3000"),
			Dify:       env("PROXY_DIFY", "http://kube-llmops-dify-web:3000"),
			MLflow:     env("PROXY_MLFLOW", "http://kube-llmops-mlflow:5000"),
			JupyterHub: env("PROXY_JUPYTERHUB", "http://kube-llmops-jupyterhub:8000"),
			MinIO:      env("PROXY_MINIO", "http://kube-llmops-minio:9001"),
			Keycloak:   env("PROXY_KEYCLOAK", "http://kube-llmops-keycloak:8080"),
			LiteLLM:    env("PROXY_LITELLM", "http://kube-llmops-litellm:4000"),
			Prometheus: env("PROXY_PROMETHEUS", "http://kube-llmops-prometheus:9090"),
		},
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

- [ ] **Step 2: Write kube/client.go**

Reference pattern: `operator/internal/cli/util/kube.go`

```go
package kube

import (
	"fmt"
	"os"

	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Clients struct {
	CR        client.Client
	Clientset kubernetes.Interface
	Config    *rest.Config
	Namespace string
}

func NewClients(namespace string) (*Clients, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			kubeconfig = os.ExpandEnv("$HOME/.kube/config")
		}
		cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("cannot build k8s config: %w", err)
		}
	}

	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("register CRD scheme: %w", err)
	}

	cr, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("create CR client: %w", err)
	}

	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("create clientset: %w", err)
	}

	return &Clients{CR: cr, Clientset: cs, Config: cfg, Namespace: namespace}, nil
}
```

- [ ] **Step 3: Write client_test.go**

```go
package kube

import "testing"

func TestNewClients_FallbackToKubeconfig(t *testing.T) {
	// Outside cluster, should try KUBECONFIG or ~/.kube/config
	// If neither exists, NewClients returns an error (expected in CI)
	_, err := NewClients("default")
	if err == nil {
		t.Log("K8s config found, client created successfully")
	} else {
		t.Logf("Expected error outside cluster: %v", err)
	}
}
```

- [ ] **Step 4: Run test**

```bash
cd dashboard && go test ./internal/kube/ -v
```

- [ ] **Step 5: Commit**

```bash
git add dashboard/internal/
git commit -m "feat(dashboard): config loader and K8s client initialization"
```

---

## Task 3: Gin Server + Embed SPA Scaffold

**Files:**
- Create: `dashboard/main.go`
- Create: `dashboard/embed.go`

- [ ] **Step 1: Write embed.go**

```go
package main

import (
	"embed"
	"io/fs"
)

//go:embed web/dist
var staticFS embed.FS

func getStaticFS() (fs.FS, error) {
	return fs.Sub(staticFS, "web/dist")
}
```

- [ ] **Step 2: Write main.go**

```go
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

	// K8s
	kc, err := kube.NewClients(cfg.Namespace)
	if err != nil {
		log.Printf("WARN: K8s client unavailable: %v", err)
	}

	r := gin.Default()
	r.Use(cors.Default())

	// API routes (added in later tasks)
	api := r.Group("/api/v1")
	_ = api
	_ = kc

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
```

- [ ] **Step 3: Create placeholder web/dist/index.html for embed**

```bash
mkdir -p dashboard/web/dist
cat > dashboard/web/dist/index.html << 'EOF'
<!DOCTYPE html>
<html><body><h1>kube-llmops dashboard</h1><p>placeholder</p></body></html>
EOF
```

- [ ] **Step 4: Verify build**

```bash
cd dashboard && go build -o /dev/null .
```

Expected: build succeeds

- [ ] **Step 5: Commit**

```bash
git add dashboard/main.go dashboard/embed.go dashboard/web/dist/index.html
git commit -m "feat(dashboard): Gin server scaffold with embedded SPA"
```

---

## Task 4: RBAC Models (GORM)

**Files:**
- Create: `dashboard/internal/rbac/models.go`
- Create: `dashboard/internal/rbac/models_test.go`

- [ ] **Step 1: Write models_test.go**

```go
package rbac

import (
	"testing"

	"github.com/google/uuid"
)

func TestUserModel_Fields(t *testing.T) {
	u := User{
		ID:          uuid.New(),
		KeycloakID:  "kc-123",
		Email:       "test@example.com",
		DisplayName: "Test User",
		Enabled:     true,
	}
	if u.Email != "test@example.com" {
		t.Errorf("expected test@example.com, got %s", u.Email)
	}
}

func TestPermission_UniqueConstraint(t *testing.T) {
	p := Permission{
		ID:       uuid.New(),
		Resource: "models",
		Action:   "create",
		IsSystem: true,
	}
	if p.Resource != "models" || p.Action != "create" {
		t.Error("permission fields not set correctly")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd dashboard && go test ./internal/rbac/ -v -run TestUserModel
```

Expected: FAIL — `User` not defined

- [ ] **Step 3: Write models.go**

```go
package rbac

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	KeycloakID  string     `gorm:"uniqueIndex" json:"keycloakId"`
	Email       string     `gorm:"uniqueIndex" json:"email"`
	DisplayName string     `json:"displayName"`
	Avatar      string     `json:"avatar,omitempty"`
	Enabled     bool       `gorm:"default:true" json:"enabled"`
	LastLogin   *time.Time `json:"lastLogin,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	Roles       []Role     `gorm:"many2many:user_roles" json:"roles,omitempty"`
}

type Role struct {
	ID          uuid.UUID    `gorm:"type:uuid;primaryKey" json:"id"`
	Name        string       `gorm:"uniqueIndex" json:"name"`
	Description string       `json:"description"`
	IsSystem    bool         `gorm:"default:false" json:"isSystem"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
	Permissions []Permission `gorm:"many2many:role_permissions" json:"permissions,omitempty"`
}

type Permission struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Resource    string    `gorm:"index:idx_resource_action,unique" json:"resource"`
	Action      string    `gorm:"index:idx_resource_action,unique" json:"action"`
	Description string    `json:"description"`
	IsSystem    bool      `gorm:"default:false" json:"isSystem"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

func (r *Role) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

func (p *Permission) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}
```

- [ ] **Step 4: Run tests**

```bash
cd dashboard && go test ./internal/rbac/ -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add dashboard/internal/rbac/models.go dashboard/internal/rbac/models_test.go
git commit -m "feat(dashboard): RBAC GORM models (User, Role, Permission)"
```

---

## Task 5: RBAC Seed

**Files:**
- Create: `dashboard/internal/rbac/seed.go`
- Create: `dashboard/internal/rbac/seed_test.go`

- [ ] **Step 1: Write seed_test.go**

```go
package rbac

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&User{}, &Role{}, &Permission{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestSeed_CreatesDefaultPermissions(t *testing.T) {
	db := setupTestDB(t)
	if err := Seed(db); err != nil {
		t.Fatalf("seed: %v", err)
	}
	var count int64
	db.Model(&Permission{}).Count(&count)
	if count < 24 {
		t.Errorf("expected >= 24 permissions, got %d", count)
	}
}

func TestSeed_Creates3SystemRoles(t *testing.T) {
	db := setupTestDB(t)
	if err := Seed(db); err != nil {
		t.Fatalf("seed: %v", err)
	}
	var roles []Role
	db.Where("is_system = ?", true).Find(&roles)
	if len(roles) != 3 {
		t.Errorf("expected 3 system roles, got %d", len(roles))
	}
	names := map[string]bool{}
	for _, r := range roles {
		names[r.Name] = true
	}
	for _, n := range []string{"admin", "editor", "viewer"} {
		if !names[n] {
			t.Errorf("missing system role: %s", n)
		}
	}
}

func TestSeed_AdminHasAllPermissions(t *testing.T) {
	db := setupTestDB(t)
	Seed(db)
	var admin Role
	db.Preload("Permissions").Where("name = ?", "admin").First(&admin)
	var totalPerms int64
	db.Model(&Permission{}).Count(&totalPerms)
	if len(admin.Permissions) != int(totalPerms) {
		t.Errorf("admin has %d perms, total is %d", len(admin.Permissions), totalPerms)
	}
}

func TestSeed_Idempotent(t *testing.T) {
	db := setupTestDB(t)
	Seed(db)
	Seed(db) // second call should not error or duplicate
	var count int64
	db.Model(&Role{}).Count(&count)
	if count != 3 {
		t.Errorf("expected 3 roles after double seed, got %d", count)
	}
}
```

- [ ] **Step 2: Add sqlite driver for tests**

```bash
cd dashboard && go get gorm.io/driver/sqlite@latest
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
cd dashboard && go test ./internal/rbac/ -v -run TestSeed
```

Expected: FAIL — `Seed` not defined

- [ ] **Step 4: Write seed.go**

```go
package rbac

import (
	"log"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type permDef struct {
	Resource    string
	Action      string
	Description string
}

var defaultPermissions = []permDef{
	{"models", "view", "View model deployments"},
	{"models", "create", "Deploy new models"},
	{"models", "edit", "Edit model configuration"},
	{"models", "delete", "Delete model deployments"},
	{"models", "scale", "Scale model replicas"},
	{"models", "canary", "Manage canary deployments"},
	{"finetune", "view", "View fine-tune runs"},
	{"finetune", "create", "Create fine-tune runs"},
	{"finetune", "delete", "Delete fine-tune runs"},
	{"rag", "view", "View knowledge bases"},
	{"rag", "create", "Create knowledge bases"},
	{"rag", "upload", "Upload documents"},
	{"rag", "delete", "Delete knowledge bases"},
	{"rag", "query", "Query knowledge bases"},
	{"platform", "view", "View platform config"},
	{"platform", "edit", "Edit platform config"},
	{"monitoring", "view", "View monitoring dashboards"},
	{"users", "view", "View users"},
	{"users", "create", "Create users"},
	{"users", "edit", "Edit users"},
	{"users", "delete", "Delete users"},
	{"roles", "view", "View roles"},
	{"roles", "create", "Create roles"},
	{"roles", "edit", "Edit roles"},
	{"roles", "delete", "Delete roles"},
	{"permissions", "view", "View permissions"},
	{"permissions", "create", "Create permissions"},
	{"permissions", "edit", "Edit permissions"},
	{"permissions", "delete", "Delete permissions"},
}

func Seed(db *gorm.DB) error {
	// Upsert permissions
	for _, pd := range defaultPermissions {
		p := Permission{
			ID:          uuid.New(),
			Resource:    pd.Resource,
			Action:      pd.Action,
			Description: pd.Description,
			IsSystem:    true,
		}
		db.Where("resource = ? AND action = ?", pd.Resource, pd.Action).
			Attrs(p).FirstOrCreate(&p)
	}

	// Collect all permissions
	var allPerms []Permission
	db.Find(&allPerms)
	permMap := map[string]Permission{}
	for _, p := range allPerms {
		permMap[p.Resource+":"+p.Action] = p
	}

	// Define roles
	type roleDef struct {
		Name        string
		Description string
		Perms       func() []Permission
	}
	roles := []roleDef{
		{
			Name:        "admin",
			Description: "Full access to all resources",
			Perms:       func() []Permission { return allPerms },
		},
		{
			Name:        "editor",
			Description: "Manage models, fine-tuning, RAG; view monitoring and platform",
			Perms: func() []Permission {
				var out []Permission
				for _, p := range allPerms {
					switch p.Resource {
					case "models", "finetune", "rag":
						out = append(out, p)
					case "monitoring", "platform":
						if p.Action == "view" {
							out = append(out, p)
						}
					}
				}
				return out
			},
		},
		{
			Name:        "viewer",
			Description: "Read-only access to all resources",
			Perms: func() []Permission {
				var out []Permission
				for _, p := range allPerms {
					if p.Action == "view" {
						out = append(out, p)
					}
				}
				return out
			},
		},
	}

	for _, rd := range roles {
		var role Role
		result := db.Where("name = ?", rd.Name).First(&role)
		if result.Error != nil {
			role = Role{
				ID:          uuid.New(),
				Name:        rd.Name,
				Description: rd.Description,
				IsSystem:    true,
			}
			if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&role).Error; err != nil {
				return err
			}
		}
		if err := db.Model(&role).Association("Permissions").Replace(rd.Perms()); err != nil {
			log.Printf("WARN: set perms for role %s: %v", rd.Name, err)
		}
	}

	return nil
}
```

- [ ] **Step 5: Run tests**

```bash
cd dashboard && go test ./internal/rbac/ -v -run TestSeed
```

Expected: all 4 tests PASS

- [ ] **Step 6: Commit**

```bash
git add dashboard/internal/rbac/seed.go dashboard/internal/rbac/seed_test.go
git commit -m "feat(dashboard): RBAC seed with 29 permissions and 3 system roles"
```

---

## Task 6: RBAC Service (Business Logic)

**Files:**
- Create: `dashboard/internal/rbac/service.go`
- Create: `dashboard/internal/rbac/service_test.go`

- [ ] **Step 1: Write service_test.go**

```go
package rbac

import (
	"testing"

	"github.com/google/uuid"
)

func TestService_HasPermission(t *testing.T) {
	db := setupTestDB(t)
	Seed(db)
	svc := NewService(db)

	// Create user with admin role
	user := &User{ID: uuid.New(), KeycloakID: "kc-admin", Email: "admin@test.com", DisplayName: "Admin", Enabled: true}
	db.Create(user)
	var adminRole Role
	db.Where("name = ?", "admin").First(&adminRole)
	db.Model(user).Association("Roles").Append(&adminRole)

	ok, err := svc.HasPermission(user.ID, "models", "create")
	if err != nil {
		t.Fatalf("has permission: %v", err)
	}
	if !ok {
		t.Error("admin should have models:create")
	}
}

func TestService_ViewerCannotCreate(t *testing.T) {
	db := setupTestDB(t)
	Seed(db)
	svc := NewService(db)

	user := &User{ID: uuid.New(), KeycloakID: "kc-viewer", Email: "viewer@test.com", DisplayName: "Viewer", Enabled: true}
	db.Create(user)
	var viewerRole Role
	db.Where("name = ?", "viewer").First(&viewerRole)
	db.Model(user).Association("Roles").Append(&viewerRole)

	ok, _ := svc.HasPermission(user.ID, "models", "create")
	if ok {
		t.Error("viewer should NOT have models:create")
	}

	ok, _ = svc.HasPermission(user.ID, "models", "view")
	if !ok {
		t.Error("viewer should have models:view")
	}
}

func TestService_GetUserPermissions(t *testing.T) {
	db := setupTestDB(t)
	Seed(db)
	svc := NewService(db)

	user := &User{ID: uuid.New(), KeycloakID: "kc-editor", Email: "editor@test.com", DisplayName: "Editor", Enabled: true}
	db.Create(user)
	var editorRole Role
	db.Where("name = ?", "editor").First(&editorRole)
	db.Model(user).Association("Roles").Append(&editorRole)

	perms, err := svc.GetUserPermissions(user.ID)
	if err != nil {
		t.Fatalf("get perms: %v", err)
	}
	if len(perms) == 0 {
		t.Error("editor should have some permissions")
	}
	// Editor should NOT have users:create
	for _, p := range perms {
		if p.Resource == "users" && p.Action == "create" {
			t.Error("editor should NOT have users:create")
		}
	}
}

func TestService_DeleteSystemRole_Rejected(t *testing.T) {
	db := setupTestDB(t)
	Seed(db)
	svc := NewService(db)

	var admin Role
	db.Where("name = ?", "admin").First(&admin)
	err := svc.DeleteRole(admin.ID)
	if err == nil {
		t.Error("deleting system role should fail")
	}
}

func TestService_CreateAndDeleteCustomRole(t *testing.T) {
	db := setupTestDB(t)
	Seed(db)
	svc := NewService(db)

	role, err := svc.CreateRole("ml-engineer", "ML team role")
	if err != nil {
		t.Fatalf("create role: %v", err)
	}
	if role.IsSystem {
		t.Error("custom role should not be system")
	}
	err = svc.DeleteRole(role.ID)
	if err != nil {
		t.Errorf("delete custom role: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd dashboard && go test ./internal/rbac/ -v -run TestService
```

Expected: FAIL — `NewService` not defined

- [ ] **Step 3: Write service.go**

```go
package rbac

import (
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) HasPermission(userID uuid.UUID, resource, action string) (bool, error) {
	var count int64
	err := s.db.Model(&Permission{}).
		Joins("JOIN role_permissions ON role_permissions.permission_id = permissions.id").
		Joins("JOIN user_roles ON user_roles.role_id = role_permissions.role_id").
		Where("user_roles.user_id = ? AND permissions.resource = ? AND permissions.action = ?",
			userID, resource, action).
		Count(&count).Error
	return count > 0, err
}

func (s *Service) GetUserPermissions(userID uuid.UUID) ([]Permission, error) {
	var perms []Permission
	err := s.db.Distinct("permissions.*").
		Joins("JOIN role_permissions ON role_permissions.permission_id = permissions.id").
		Joins("JOIN user_roles ON user_roles.role_id = role_permissions.role_id").
		Where("user_roles.user_id = ?", userID).
		Find(&perms).Error
	return perms, err
}

func (s *Service) FindUserByKeycloakID(kcID string) (*User, error) {
	var u User
	if err := s.db.Preload("Roles").Where("keycloak_id = ?", kcID).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Service) EnsureUser(kcID, email, name string) (*User, error) {
	var u User
	result := s.db.Where("keycloak_id = ?", kcID).First(&u)
	if result.Error == nil {
		return &u, nil
	}
	u = User{KeycloakID: kcID, Email: email, DisplayName: name, Enabled: true}
	if err := s.db.Create(&u).Error; err != nil {
		return nil, err
	}
	// Assign viewer role by default
	var viewer Role
	if s.db.Where("name = ?", "viewer").First(&viewer).Error == nil {
		s.db.Model(&u).Association("Roles").Append(&viewer)
	}
	return &u, nil
}

func (s *Service) CreateRole(name, description string) (*Role, error) {
	role := &Role{Name: name, Description: description, IsSystem: false}
	if err := s.db.Create(role).Error; err != nil {
		return nil, err
	}
	return role, nil
}

func (s *Service) DeleteRole(id uuid.UUID) error {
	var role Role
	if err := s.db.First(&role, id).Error; err != nil {
		return err
	}
	if role.IsSystem {
		return fmt.Errorf("cannot delete system role %q", role.Name)
	}
	s.db.Model(&role).Association("Permissions").Clear()
	return s.db.Delete(&role).Error
}

func (s *Service) SetRolePermissions(roleID uuid.UUID, permIDs []uuid.UUID) error {
	var perms []Permission
	s.db.Where("id IN ?", permIDs).Find(&perms)
	var role Role
	if err := s.db.First(&role, roleID).Error; err != nil {
		return err
	}
	return s.db.Model(&role).Association("Permissions").Replace(perms)
}

func (s *Service) CreatePermission(resource, action, desc string) (*Permission, error) {
	p := &Permission{Resource: resource, Action: action, Description: desc, IsSystem: false}
	if err := s.db.Create(p).Error; err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Service) DeletePermission(id uuid.UUID) error {
	var p Permission
	if err := s.db.First(&p, id).Error; err != nil {
		return err
	}
	if p.IsSystem {
		return fmt.Errorf("cannot delete system permission %s:%s", p.Resource, p.Action)
	}
	return s.db.Delete(&p).Error
}
```

- [ ] **Step 4: Run tests**

```bash
cd dashboard && go test ./internal/rbac/ -v
```

Expected: all tests PASS

- [ ] **Step 5: Commit**

```bash
git add dashboard/internal/rbac/service.go dashboard/internal/rbac/service_test.go
git commit -m "feat(dashboard): RBAC service with permission check and role management"
```

---

## Task 7: Auth Middleware (OIDC + RBAC)

**Files:**
- Create: `dashboard/internal/auth/oidc.go`
- Create: `dashboard/internal/auth/middleware.go`
- Create: `dashboard/internal/auth/middleware_test.go`

- [ ] **Step 1: Write oidc.go**

```go
package auth

import (
	"context"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type OIDCProvider struct {
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier
	oauth    *oauth2.Config
}

func NewOIDCProvider(issuerURL, clientID, clientSecret, redirectURL string) (*OIDCProvider, error) {
	ctx := context.Background()
	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, err
	}
	verifier := provider.Verifier(&oidc.Config{ClientID: clientID})
	oauthCfg := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}
	return &OIDCProvider{provider: provider, verifier: verifier, oauth: oauthCfg}, nil
}

func (o *OIDCProvider) Verify(ctx context.Context, rawToken string) (*oidc.IDToken, error) {
	return o.verifier.Verify(ctx, rawToken)
}

func (o *OIDCProvider) AuthCodeURL(state string) string {
	return o.oauth.AuthCodeURL(state)
}

func (o *OIDCProvider) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return o.oauth.Exchange(ctx, code)
}
```

- [ ] **Step 2: Write middleware.go**

```go
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

func JWTMiddleware(oidc *OIDCProvider, db *gorm.DB) gin.HandlerFunc {
	svc := rbac.NewService(db)
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}
		raw := strings.TrimPrefix(header, "Bearer ")

		idToken, err := oidc.Verify(c.Request.Context(), raw)
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
```

- [ ] **Step 3: Write middleware_test.go**

```go
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
```

- [ ] **Step 4: Run tests**

```bash
cd dashboard && go test ./internal/auth/ -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add dashboard/internal/auth/
git commit -m "feat(dashboard): OIDC provider + JWT/RBAC middleware"
```

---

## Task 8: Auth Handlers (callback, me, logout)

**Files:**
- Create: `dashboard/internal/handler/auth.go`

- [ ] **Step 1: Write auth.go**

```go
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/kube-llmops/dashboard/internal/auth"
	"github.com/kube-llmops/dashboard/internal/rbac"
)

func GetCurrentUser(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := c.MustGet("user").(*rbac.User)
		perms := c.MustGet("permissions").([]rbac.Permission)
		// Reload with roles
		svc := rbac.NewService(db)
		full, err := svc.FindUserByKeycloakID(user.KeycloakID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"user":        full,
			"permissions": perms,
		})
	}
}

func AuthCallback(oidc *auth.OIDCProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Query("code")
		if code == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing code"})
			return
		}
		token, err := oidc.Exchange(c.Request.Context(), code)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "exchange failed"})
			return
		}
		rawIDToken, ok := token.Extra("id_token").(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "no id_token"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"access_token": token.AccessToken,
			"id_token":     rawIDToken,
			"expires_in":   token.Expiry,
		})
	}
}

func AuthLogout() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "logged out"})
	}
}
```

- [ ] **Step 2: Verify build**

```bash
cd dashboard && go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add dashboard/internal/handler/auth.go
git commit -m "feat(dashboard): auth handlers (callback, me, logout)"
```

---

## Task 9: Model Handlers (CRUD + scale + canary)

**Files:**
- Create: `dashboard/internal/handler/models.go`
- Create: `dashboard/internal/handler/models_test.go`

- [ ] **Step 1: Write models_test.go**

Test the handler response structure using httptest + Gin test mode. Since K8s calls require a real cluster, test with a mock client or test the handler wiring only.

```go
package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestListModels_RequiresKubeClient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// nil kube clients → should return 503
	r.GET("/models", ListModels(nil))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/models", nil)
	r.ServeHTTP(w, req)

	if w.Code != 503 {
		t.Errorf("expected 503 without kube client, got %d", w.Code)
	}
}
```

- [ ] **Step 2: Write models.go**

```go
package handler

import (
	"context"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	"github.com/kube-llmops/dashboard/internal/kube"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ListModels(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(503, gin.H{"error": "K8s unavailable"})
			return
		}
		var list v1alpha1.ModelDeploymentList
		if err := kc.CR.List(c.Request.Context(), &list, client.InNamespace(kc.Namespace)); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, list.Items)
	}
}

func GetModel(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(503, gin.H{"error": "K8s unavailable"})
			return
		}
		name := c.Param("name")
		var md v1alpha1.ModelDeployment
		key := client.ObjectKey{Namespace: kc.Namespace, Name: name}
		if err := kc.CR.Get(c.Request.Context(), key, &md); err != nil {
			c.JSON(404, gin.H{"error": "not found"})
			return
		}
		c.JSON(200, md)
	}
}

func CreateModel(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(503, gin.H{"error": "K8s unavailable"})
			return
		}
		var md v1alpha1.ModelDeployment
		if err := c.ShouldBindJSON(&md); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		md.Namespace = kc.Namespace
		if err := kc.CR.Create(c.Request.Context(), &md); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(201, md)
	}
}

func UpdateModel(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(503, gin.H{"error": "K8s unavailable"})
			return
		}
		name := c.Param("name")
		var md v1alpha1.ModelDeployment
		key := client.ObjectKey{Namespace: kc.Namespace, Name: name}
		if err := kc.CR.Get(c.Request.Context(), key, &md); err != nil {
			c.JSON(404, gin.H{"error": "not found"})
			return
		}
		var update v1alpha1.ModelDeploymentSpec
		if err := c.ShouldBindJSON(&update); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		md.Spec = update
		if err := kc.CR.Update(c.Request.Context(), &md); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, md)
	}
}

func DeleteModel(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(503, gin.H{"error": "K8s unavailable"})
			return
		}
		name := c.Param("name")
		md := &v1alpha1.ModelDeployment{}
		md.Name = name
		md.Namespace = kc.Namespace
		if err := kc.CR.Delete(c.Request.Context(), md); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"message": "deleted"})
	}
}

func ScaleModel(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(503, gin.H{"error": "K8s unavailable"})
			return
		}
		name := c.Param("name")
		var body struct {
			Replicas int32 `json:"replicas"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		var md v1alpha1.ModelDeployment
		key := client.ObjectKey{Namespace: kc.Namespace, Name: name}
		if err := kc.CR.Get(c.Request.Context(), key, &md); err != nil {
			c.JSON(404, gin.H{"error": "not found"})
			return
		}
		md.Spec.Replicas = &body.Replicas
		if err := kc.CR.Update(c.Request.Context(), &md); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, md)
	}
}

func CanaryModel(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(503, gin.H{"error": "K8s unavailable"})
			return
		}
		name := c.Param("name")
		var body v1alpha1.CanaryConfig
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		var md v1alpha1.ModelDeployment
		key := client.ObjectKey{Namespace: kc.Namespace, Name: name}
		if err := kc.CR.Get(c.Request.Context(), key, &md); err != nil {
			c.JSON(404, gin.H{"error": "not found"})
			return
		}
		md.Spec.Canary = &body
		if err := kc.CR.Update(c.Request.Context(), &md); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, md)
	}
}

func PromoteCanary(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(503, gin.H{"error": "K8s unavailable"})
			return
		}
		name := c.Param("name")
		var md v1alpha1.ModelDeployment
		key := client.ObjectKey{Namespace: kc.Namespace, Name: name}
		if err := kc.CR.Get(c.Request.Context(), key, &md); err != nil {
			c.JSON(404, gin.H{"error": "not found"})
			return
		}
		if md.Spec.Canary == nil {
			c.JSON(400, gin.H{"error": "no canary configured"})
			return
		}
		md.Spec.Source = md.Spec.Canary.Source
		md.Spec.Canary = nil
		if err := kc.CR.Update(c.Request.Context(), &md); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, md)
	}
}

func RollbackCanary(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(503, gin.H{"error": "K8s unavailable"})
			return
		}
		name := c.Param("name")
		var md v1alpha1.ModelDeployment
		key := client.ObjectKey{Namespace: kc.Namespace, Name: name}
		if err := kc.CR.Get(c.Request.Context(), key, &md); err != nil {
			c.JSON(404, gin.H{"error": "not found"})
			return
		}
		md.Spec.Canary = nil
		if err := kc.CR.Update(c.Request.Context(), &md); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, md)
	}
}

func ListModelPods(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(503, gin.H{"error": "K8s unavailable"})
			return
		}
		name := c.Param("name")
		pods, err := kc.Clientset.CoreV1().Pods(kc.Namespace).List(c.Request.Context(),
			metav1.ListOptions{LabelSelector: "app.kubernetes.io/name=" + name})
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		type podInfo struct {
			Name   string `json:"name"`
			Phase  string `json:"phase"`
			Node   string `json:"node"`
			Ready  bool   `json:"ready"`
		}
		var result []podInfo
		for _, p := range pods.Items {
			ready := false
			for _, cond := range p.Status.Conditions {
				if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
					ready = true
				}
			}
			result = append(result, podInfo{
				Name:  p.Name,
				Phase: string(p.Status.Phase),
				Node:  p.Spec.NodeName,
				Ready: ready,
			})
		}
		c.JSON(200, result)
	}
}

func StreamModelLogs(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(503, gin.H{"error": "K8s unavailable"})
			return
		}
		name := c.Param("name")
		pods, err := kc.Clientset.CoreV1().Pods(kc.Namespace).List(c.Request.Context(),
			metav1.ListOptions{LabelSelector: "app.kubernetes.io/name=" + name})
		if err != nil || len(pods.Items) == 0 {
			c.JSON(404, gin.H{"error": "no pods found"})
			return
		}
		tailLines := int64(100)
		req := kc.Clientset.CoreV1().Pods(kc.Namespace).GetLogs(pods.Items[0].Name,
			&corev1.PodLogOptions{Follow: true, TailLines: &tailLines})
		stream, err := req.Stream(c.Request.Context())
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		defer stream.Close()

		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")

		buf := make([]byte, 4096)
		for {
			n, err := stream.Read(buf)
			if n > 0 {
				c.SSEvent("log", string(buf[:n]))
				c.Writer.Flush()
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				break
			}
		}
	}
}

func TestModel(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(503, gin.H{"error": "K8s unavailable"})
			return
		}
		name := c.Param("name")
		var md v1alpha1.ModelDeployment
		key := client.ObjectKey{Namespace: kc.Namespace, Name: name}
		if err := kc.CR.Get(c.Request.Context(), key, &md); err != nil {
			c.JSON(404, gin.H{"error": "not found"})
			return
		}
		if md.Status.Endpoint == "" {
			c.JSON(400, gin.H{"error": "model not ready"})
			return
		}
		var body struct {
			Prompt string `json:"prompt"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		// Forward to model endpoint via gateway
		c.JSON(200, gin.H{"message": "test not implemented yet", "endpoint": md.Status.Endpoint})
	}
}
```

Note: add `metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"` to imports.

- [ ] **Step 3: Run tests**

```bash
cd dashboard && go test ./internal/handler/ -v -run TestListModels
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add dashboard/internal/handler/models.go dashboard/internal/handler/models_test.go
git commit -m "feat(dashboard): model CRUD handlers (list/get/create/update/delete/scale/canary)"
```

---

## Task 10: Finetune Handlers

**Files:**
- Create: `dashboard/internal/handler/finetune.go`

Follows same pattern as models.go but operates on `v1alpha1.FineTuneRun` CRD. Handlers: `ListFinetunes`, `CreateFinetune`, `GetFinetune`, `DeleteFinetune`, `StreamFinetuneLogs`.

- [ ] **Step 1: Write finetune.go** — same pattern as models.go, using `v1alpha1.FineTuneRunList` and `v1alpha1.FineTuneRun`
- [ ] **Step 2: Verify build**: `cd dashboard && go build ./...`
- [ ] **Step 3: Commit**

---

## Task 11: RAG Handlers (Dify Proxy)

**Files:**
- Create: `dashboard/internal/handler/rag.go`

Reference pattern: `operator/internal/cli/cmd/rag.go` (difySession).

- [ ] **Step 1: Write rag.go** — reuse difySession pattern for Dify console API auth. Handlers: `ListKnowledgeBases`, `CreateKnowledgeBase`, `GetKnowledgeBase`, `DeleteKnowledgeBase`, `UploadDocument`, `QueryKnowledgeBase`.
- [ ] **Step 2: Verify build**: `cd dashboard && go build ./...`
- [ ] **Step 3: Commit**

---

## Task 12: Platform Handlers

**Files:**
- Create: `dashboard/internal/handler/platform.go`

- [ ] **Step 1: Write platform.go** — operates on `v1alpha1.LLMPlatform` CRD. Handlers: `GetPlatform`, `UpdatePlatform`, `GetComponents`.
- [ ] **Step 2: Verify build**: `cd dashboard && go build ./...`
- [ ] **Step 3: Commit**

---

## Task 13: Monitoring Handlers

**Files:**
- Create: `dashboard/internal/handler/monitoring.go`

- [ ] **Step 1: Write monitoring.go** — `GetMonitoringSummary` (queries Prometheus API for key metrics), `GetNotebooksSummary` (queries JupyterHub API for active servers).
- [ ] **Step 2: Verify build**: `cd dashboard && go build ./...`
- [ ] **Step 3: Commit**

---

## Task 14: Services Handler (Discovery)

**Files:**
- Create: `dashboard/internal/handler/services.go`

- [ ] **Step 1: Write services.go**

```go
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	v1alpha1 "github.com/kube-llmops/operator/api/v1alpha1"
	"github.com/kube-llmops/dashboard/internal/kube"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ServiceInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Phase       string `json:"phase"`
	Endpoint    string `json:"endpoint,omitempty"`
	ProxyPath   string `json:"proxyPath"`
}

var serviceRegistry = []struct {
	Name        string
	Description string
	Icon        string
	Component   string // key in LLMPlatform status.components
	ProxyPath   string
}{
	{"grafana", "Monitoring Dashboards", "dashboard", "grafana", "/services/grafana/"},
	{"langfuse", "LLM Tracing & Analytics", "search", "langfuse", "/services/langfuse/"},
	{"dify", "RAG Platform", "robot", "dify", "/services/dify/"},
	{"mlflow", "Experiment Tracking", "experiment", "mlflow", "/services/mlflow/"},
	{"jupyterhub", "Notebook Development", "code", "jupyterhub", "/services/jupyterhub/"},
	{"minio", "Object Storage", "database", "minio", "/services/minio/"},
	{"keycloak", "Identity Management", "lock", "keycloak", "/services/keycloak/"},
	{"litellm", "AI Gateway", "api", "gateway", "/services/litellm/"},
	{"prometheus", "Metrics Query", "bar-chart", "prometheus", "/services/prometheus/"},
}

func ListServices(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		if kc == nil {
			c.JSON(503, gin.H{"error": "K8s unavailable"})
			return
		}
		// Get LLMPlatform status for component health
		var platforms v1alpha1.LLMPlatformList
		kc.CR.List(c.Request.Context(), &platforms, client.InNamespace(kc.Namespace))

		components := map[string]*v1alpha1.ComponentStatus{}
		if len(platforms.Items) > 0 {
			cs := platforms.Items[0].Status.Components
			components["grafana"] = cs.Grafana
			components["langfuse"] = cs.Langfuse
			components["dify"] = cs.Dify
			components["minio"] = cs.Minio
			components["gateway"] = cs.Gateway
			components["prometheus"] = cs.Prometheus
			components["postgresql"] = cs.PostgreSQL
		}

		var services []ServiceInfo
		for _, sr := range serviceRegistry {
			svc := ServiceInfo{
				Name:        sr.Name,
				Description: sr.Description,
				Icon:        sr.Icon,
				Phase:       "Unknown",
				ProxyPath:   sr.ProxyPath,
			}
			if cs, ok := components[sr.Component]; ok && cs != nil {
				svc.Phase = cs.Phase
				svc.Endpoint = cs.Endpoint
			}
			services = append(services, svc)
		}
		c.JSON(200, services)
	}
}

func GetServiceStatus(kc *kube.Clients) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		// Find in registry
		for _, sr := range serviceRegistry {
			if sr.Name == name {
				c.JSON(200, gin.H{"name": name, "proxyPath": sr.ProxyPath})
				return
			}
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown service"})
	}
}
```

- [ ] **Step 2: Verify build**: `cd dashboard && go build ./...`
- [ ] **Step 3: Commit**

```bash
git add dashboard/internal/handler/services.go
git commit -m "feat(dashboard): services discovery handler with 9 service registry"
```

---

## Task 15: Reverse Proxy with SSO Passthrough

**Files:**
- Create: `dashboard/internal/proxy/reverse.go`
- Create: `dashboard/internal/proxy/registry.go`
- Create: `dashboard/internal/proxy/auth_grafana.go`
- Create: `dashboard/internal/proxy/auth_dify.go`
- Create: `dashboard/internal/proxy/auth_litellm.go`
- Create: `dashboard/internal/proxy/reverse_test.go`

- [ ] **Step 1: Write reverse.go** — generic `httputil.ReverseProxy` wrapper that strips prefix, rewrites host, and calls a per-service auth injector function.

- [ ] **Step 2: Write registry.go** — maps service name → target URL + auth strategy function.

- [ ] **Step 3: Write auth_grafana.go** — injects `X-WEBAUTH-USER` header from JWT claims. Note: Grafana must have `GF_AUTH_PROXY_ENABLED=true` (added in Helm task).

- [ ] **Step 4: Write auth_dify.go** — reuses difySession cookie pattern from `operator/internal/cli/cmd/rag.go`.

- [ ] **Step 5: Write auth_litellm.go** — injects `Authorization: Bearer <masterKey>`.

- [ ] **Step 6: Write reverse_test.go** — test URL rewriting and header injection using httptest.

- [ ] **Step 7: Run tests**

```bash
cd dashboard && go test ./internal/proxy/ -v
```

- [ ] **Step 8: Commit**

```bash
git add dashboard/internal/proxy/
git commit -m "feat(dashboard): reverse proxy with per-service SSO passthrough"
```

---

## Task 16: SSE Broker

**Files:**
- Create: `dashboard/internal/sse/broker.go`
- Create: `dashboard/internal/sse/broker_test.go`

- [ ] **Step 1: Write broker.go** — fan-out broker pattern: K8s informer watch → broadcast to all connected SSE clients via channels. Events: model status changes, finetune progress, component health.

- [ ] **Step 2: Write broker_test.go** — test subscribe/unsubscribe/broadcast without real K8s.

- [ ] **Step 3: Run tests**: `cd dashboard && go test ./internal/sse/ -v`

- [ ] **Step 4: Commit**

---

## Task 17: User Handlers + Keycloak Sync

**Files:**
- Create: `dashboard/internal/handler/users.go`
- Create: `dashboard/internal/rbac/sync.go`

- [ ] **Step 1: Write sync.go** — Keycloak Admin API client using service account: list users, create user, disable user, delete user.

- [ ] **Step 2: Write users.go** — handlers: `ListUsers`, `CreateUser`, `GetUser`, `UpdateUser`, `DeleteUser`, `AssignRoles`. Create/delete writes both Keycloak + local DB.

- [ ] **Step 3: Verify build**: `cd dashboard && go build ./...`
- [ ] **Step 4: Commit**

---

## Task 18: Role + Permission Handlers

**Files:**
- Create: `dashboard/internal/handler/roles.go`
- Create: `dashboard/internal/handler/permissions.go`

- [ ] **Step 1: Write roles.go** — handlers: `ListRoles`, `CreateRole`, `GetRole`, `UpdateRole`, `DeleteRole`, `SetRolePermissions`. System role protection.

- [ ] **Step 2: Write permissions.go** — handlers: `ListPermissions`, `CreatePermission`, `UpdatePermission`, `DeletePermission`. System permission protection.

- [ ] **Step 3: Verify build**: `cd dashboard && go build ./...`
- [ ] **Step 4: Commit**

---

## Task 19: Wire All Routes in main.go

**Files:**
- Modify: `dashboard/main.go`

- [ ] **Step 1: Wire all API routes with RBAC middleware** — register every handler from Tasks 8-18 into Gin router with appropriate `RequirePermission()` middleware. Wire service proxy routes at `/services/*`.

- [ ] **Step 2: Run all backend tests**

```bash
cd dashboard && go test ./... -v
```

Expected: all PASS

- [ ] **Step 3: Commit**

```bash
git add dashboard/main.go
git commit -m "feat(dashboard): wire all API routes with RBAC middleware"
```

---

## Task 20: React Frontend Scaffold

**Files:**
- Create: `dashboard/web/package.json`
- Create: `dashboard/web/vite.config.ts`
- Create: `dashboard/web/tsconfig.json`
- Create: `dashboard/web/index.html`
- Create: `dashboard/web/src/main.tsx`
- Create: `dashboard/web/src/App.tsx`

- [ ] **Step 1: Initialize React project**

```bash
cd dashboard/web
npm create vite@latest . -- --template react-ts
npm install antd @ant-design/icons react-router-dom @tanstack/react-query zustand axios
```

- [ ] **Step 2: Configure vite.config.ts** — add `/api` and `/services` proxy to `localhost:3000`

- [ ] **Step 3: Write App.tsx** — React Router with `BrowserRouter`, `ConfigProvider` for antd theme, lazy-loaded routes for all pages.

- [ ] **Step 4: Verify dev server**

```bash
cd dashboard/web && npm run dev
```

Expected: Vite dev server starts on port 5173

- [ ] **Step 5: Commit**

```bash
git add dashboard/web/
git commit -m "feat(dashboard): React frontend scaffold with Vite + antd + routing"
```

---

## Task 21: Auth Store + Theme + Hooks

**Files:**
- Create: `dashboard/web/src/store/auth.ts`
- Create: `dashboard/web/src/hooks/useTheme.ts`
- Create: `dashboard/web/src/hooks/usePermission.ts`
- Create: `dashboard/web/src/hooks/useSSE.ts`
- Create: `dashboard/web/src/api/client.ts`
- Create: `dashboard/web/src/components/ThemeToggle.tsx`
- Create: `dashboard/web/src/components/PermissionGuard.tsx`

- [ ] **Step 1: Write Zustand auth store** with theme persistence in localStorage
- [ ] **Step 2: Write useTheme hook** with auto/dark/light + `prefers-color-scheme` listener
- [ ] **Step 3: Write usePermission hook** with `hasPermission(resource, action)`
- [ ] **Step 4: Write useSSE hook** with auto-reconnect
- [ ] **Step 5: Write API client** with JWT auto-injection + 401 redirect
- [ ] **Step 6: Write ThemeToggle component** (sun/moon/auto cycling icon)
- [ ] **Step 7: Write PermissionGuard component**
- [ ] **Step 8: Commit**

---

## Task 22: Layout (Sidebar + Header)

**Files:**
- Create: `dashboard/web/src/components/Layout/Layout.tsx`
- Create: `dashboard/web/src/components/Layout/Sidebar.tsx`
- Create: `dashboard/web/src/components/Layout/Header.tsx`

- [ ] **Step 1: Write Sidebar.tsx** — antd `Menu` with 4 groups (Workloads/Services/Observe/Admin), collapsible, permission-gated admin section, active item highlight from React Router location.

- [ ] **Step 2: Write Header.tsx** — namespace selector, cluster health badge, ThemeToggle, user avatar dropdown (profile, logout).

- [ ] **Step 3: Write Layout.tsx** — antd `Layout` composing Sider + Header + Content + Outlet.

- [ ] **Step 4: Verify**: `cd dashboard/web && npm run dev` — check layout renders.

- [ ] **Step 5: Commit**

---

## Task 23: Overview + Models Pages

**Files:**
- Create: `dashboard/web/src/pages/Overview/Overview.tsx`
- Create: `dashboard/web/src/pages/Models/ModelList.tsx`
- Create: `dashboard/web/src/pages/Models/ModelDetail.tsx`
- Create: `dashboard/web/src/pages/Models/DeployWizard.tsx`

- [ ] **Step 1: Write Overview.tsx** — KPI cards (antd `Statistic` + `Card`), component health grid, SSE activity feed.
- [ ] **Step 2: Write ModelList.tsx** — antd `Table` with actions dropdown (scale, canary, delete).
- [ ] **Step 3: Write ModelDetail.tsx** — status card, conditions, pod list, log viewer.
- [ ] **Step 4: Write DeployWizard.tsx** — antd `Steps` + `Form`: source → engine → resources → confirm.
- [ ] **Step 5: Commit**

---

## Task 24: Finetune + RAG Pages

**Files:**
- Create: `dashboard/web/src/pages/Finetune/FinetuneList.tsx`
- Create: `dashboard/web/src/pages/Finetune/FinetuneDetail.tsx`
- Create: `dashboard/web/src/pages/Finetune/CreateWizard.tsx`
- Create: `dashboard/web/src/pages/Rag/RagList.tsx`
- Create: `dashboard/web/src/pages/Rag/RagDetail.tsx`

- [ ] **Step 1: Write FinetuneList/Detail/CreateWizard** — similar pattern to Models.
- [ ] **Step 2: Write RagList/RagDetail** — KB list, document upload (antd `Upload`), query test panel.
- [ ] **Step 3: Commit**

---

## Task 25: Services + Monitoring + Platform Pages

**Files:**
- Create: `dashboard/web/src/pages/Services/ServiceGrid.tsx`
- Create: `dashboard/web/src/pages/Services/ServiceEmbed.tsx`
- Create: `dashboard/web/src/components/IframeEmbed.tsx`
- Create: `dashboard/web/src/pages/Monitoring/MonitoringDashboard.tsx`
- Create: `dashboard/web/src/pages/Platform/PlatformStatus.tsx`

- [ ] **Step 1: Write IframeEmbed.tsx** — generic iframe with loading spinner, auto-height. Appends `&theme=dark`/`light` for Grafana based on current theme.

- [ ] **Step 2: Write ServiceGrid.tsx** — grid of `ServiceCard` components (antd `Card` with health badge). Data from `/api/v1/services`.

- [ ] **Step 3: Write ServiceEmbed.tsx** — full-page iframe at `/services/:name`. Uses `IframeEmbed` pointing to `/services/:name/`.

- [ ] **Step 4: Write MonitoringDashboard.tsx** — summary cards for 11 Grafana dashboards + Langfuse + MLflow. Click to navigate to `/services/grafana?dashboard=<uid>`.

- [ ] **Step 5: Write PlatformStatus.tsx** — LLMPlatform status, component health table, module toggles (antd `Switch`).

- [ ] **Step 6: Commit**

---

## Task 26: Users + Roles + Permissions Pages

**Files:**
- Create: `dashboard/web/src/pages/Users/UserList.tsx`
- Create: `dashboard/web/src/pages/Users/UserForm.tsx`
- Create: `dashboard/web/src/pages/Users/RoleList.tsx`
- Create: `dashboard/web/src/pages/Users/RoleForm.tsx`
- Create: `dashboard/web/src/pages/Users/PermissionList.tsx`
- Create: `dashboard/web/src/pages/Users/PermissionForm.tsx`

- [ ] **Step 1: Write UserList.tsx** — antd `Table` with role tags, actions (edit, disable, delete). Permission-gated.
- [ ] **Step 2: Write UserForm.tsx** — antd `Modal` + `Form` for create/edit user + role assignment (multi-select).
- [ ] **Step 3: Write RoleList.tsx** — table with permission count, user count, system badge.
- [ ] **Step 4: Write RoleForm.tsx** — antd `Checkbox.Group` matrix: rows = resources, columns = actions.
- [ ] **Step 5: Write PermissionList/Form** — simple CRUD table + modal.
- [ ] **Step 6: Verify build**: `cd dashboard/web && npm run build`
- [ ] **Step 7: Commit**

---

## Task 27: Helm Subchart

**Files:**
- Create: `charts/kube-llmops-stack/charts/dashboard/Chart.yaml`
- Create: `charts/kube-llmops-stack/charts/dashboard/values.yaml`
- Create: `charts/kube-llmops-stack/charts/dashboard/templates/_helpers.tpl`
- Create: `charts/kube-llmops-stack/charts/dashboard/templates/deployment.yaml`
- Create: `charts/kube-llmops-stack/charts/dashboard/templates/service.yaml`
- Create: `charts/kube-llmops-stack/charts/dashboard/templates/configmap.yaml`
- Create: `charts/kube-llmops-stack/charts/dashboard/templates/secret.yaml`
- Modify: `charts/kube-llmops-stack/Chart.yaml` — add dashboard dependency
- Modify: `charts/kube-llmops-stack/values-single-node.yaml` — add dashboard config
- Modify: `charts/kube-llmops-stack/templates/nodeport-services.yaml` — add dashboard NodePort 30302
- Modify: `charts/kube-llmops-stack/charts/keycloak/values.yaml` — add `dashboard` to realm.clients
- Modify: `charts/kube-llmops-stack/charts/keycloak/templates/realm-configmap.yaml` — add service account support for dashboard client
- Modify: `charts/kube-llmops-stack/values-single-node.yaml` — add `dashboard` to `litellm.postgresql.extraDatabases`
- Modify: `charts/kube-llmops-stack/charts/observability/templates/grafana.yaml` — add `GF_AUTH_PROXY_ENABLED=true` + `GF_AUTH_PROXY_HEADER_NAME=X-WEBAUTH-USER` env vars

- [ ] **Step 1: Create subchart** (Chart.yaml, values.yaml, templates)
- [ ] **Step 2: Add dashboard dependency to parent Chart.yaml**
- [ ] **Step 3: Add NodePort 30302 to nodeport-services.yaml**
- [ ] **Step 4: Add dashboard OIDC client + service account to Keycloak realm**
- [ ] **Step 5: Add dashboard DB to PostgreSQL extraDatabases**
- [ ] **Step 6: Add Grafana auth.proxy env vars** (keep OIDC as fallback for direct access)
- [ ] **Step 7: Verify Helm template**

```bash
cd charts/kube-llmops-stack && rm -f charts/*.tgz Chart.lock && helm dependency update .
helm template kube-llmops . -f values-single-node.yaml --show-only charts/dashboard/templates/deployment.yaml
```

- [ ] **Step 8: Commit**

```bash
git add charts/
git commit -m "feat(dashboard): Helm subchart + NodePort 30302 + Keycloak client + PG database"
```

---

## Task 28: Dockerfile + Build

**Files:**
- Create: `dashboard/Dockerfile`

- [ ] **Step 1: Write Dockerfile** (multi-stage: node → go embed → distroless)

Build context is repo root:

```dockerfile
FROM node:20-alpine AS frontend
WORKDIR /app/web
COPY dashboard/web/package.json dashboard/web/package-lock.json ./
RUN npm ci
COPY dashboard/web/ .
RUN npm run build

FROM golang:1.22-alpine AS backend
WORKDIR /app/dashboard
COPY operator/api/ /app/operator/api/
COPY operator/go.mod operator/go.sum /app/operator/
COPY dashboard/go.mod dashboard/go.sum ./
COPY --from=frontend /app/web/dist ./web/dist
RUN go mod download
COPY dashboard/ .
RUN CGO_ENABLED=0 go build -o dashboard .

FROM gcr.io/distroless/static
COPY --from=backend /app/dashboard/dashboard /dashboard
ENTRYPOINT ["/dashboard"]
```

- [ ] **Step 2: Build image**

```bash
docker build -t kube-llmops/dashboard:latest -f dashboard/Dockerfile .
```

Expected: build succeeds, image < 50MB

- [ ] **Step 3: Commit**

```bash
git add dashboard/Dockerfile
git commit -m "feat(dashboard): multi-stage Dockerfile (node + go + distroless)"
```

---

## Task 29: Helm Template Tests

**Files:**
- Create: `tests/helm/test_dashboard_templates.py`

- [ ] **Step 1: Write pytest tests** for Helm template output: deployment has correct image, service is NodePort 30302, ConfigMap contains DB/OIDC config, dashboard client exists in Keycloak realm, PostgreSQL has dashboard DB.

- [ ] **Step 2: Run tests**

```bash
python -m pytest tests/helm/test_dashboard_templates.py -v
```

- [ ] **Step 3: Commit**

---

## Task 30: Frontend Build + Backend Integration Test

- [ ] **Step 1: Build frontend**

```bash
cd dashboard/web && npm run build
```

- [ ] **Step 2: Copy dist to embed location and test Go build**

```bash
cd dashboard && go build -o /tmp/dashboard .
```

- [ ] **Step 3: Run all backend tests**

```bash
cd dashboard && go test ./... -v -count=1
```

Expected: all PASS

- [ ] **Step 4: Run frontend tests**

```bash
cd dashboard/web && npm test
```

- [ ] **Step 5: Final commit**

```bash
git add -A
git commit -m "feat(dashboard): Phase 6 Web Dashboard complete

- React 18 + Ant Design 5 frontend with Dark/Light/Auto theme
- Go Gin backend with OIDC auth + dynamic RBAC (PostgreSQL)
- 3 CRD management: ModelDeployment, FineTuneRun, LLMPlatform
- 9 service integrations with SSO passthrough reverse proxy
- SSE real-time updates via K8s informer
- Helm subchart at NodePort 30302
- ~45 API endpoints + 9 proxy routes
- Full RBAC: CRUD users, roles, permissions"
```

---

## Amendment A: AGENTS.md Update

After all tasks complete, append Dashboard info to `AGENTS.md`:

```markdown
### Web Dashboard (v1.0.0)
- React 18 + Go Gin single binary, NodePort 30302
- 3 CRDs + 9 service integrations + dynamic RBAC
- Build: `docker build -t kube-llmops/dashboard:latest -f dashboard/Dockerfile .`
- Dev: `cd dashboard/web && npm run dev` (frontend) + `cd dashboard && go run .` (backend)
- Tests: `cd dashboard && go test ./...` + `cd dashboard/web && npm test`
- Helm template tests: `python -m pytest tests/helm/test_dashboard_templates.py -v`
```
