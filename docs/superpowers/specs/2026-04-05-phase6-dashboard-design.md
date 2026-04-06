# Phase 6 Sub-project 3: Web Dashboard — Design Spec

**Date:** 2026-04-05
**Status:** Approved
**Scope:** kube-llmops v1.0.0 — Phase 6 Sub-project 3 (Web Dashboard)
**Depends on:** Phase 6 Sub-project 1 (Operator + CRDs), Phase 6 Sub-project 2 (CLI)

---

## 1. Overview

A hybrid Web Dashboard for kube-llmops that combines CRD management (ModelDeployment, FineTuneRun, LLMPlatform) with embedded access to existing tools (Grafana, Langfuse, Dify, MLflow, JupyterHub). Includes a full dynamic RBAC system backed by PostgreSQL with Keycloak OIDC authentication.

**Target users:** Platform Admins and ML Engineers, differentiated by role-based permissions.

**Tech stack:**
- **Frontend:** React 18 + TypeScript + Vite + Ant Design 5 + TanStack Query + Zustand
- **Backend:** Go (Gin) + GORM + controller-runtime K8s client
- **Auth:** Keycloak OIDC + local RBAC (PostgreSQL)
- **Deployment:** Single binary (Go `embed.FS` serves React SPA), Helm subchart in kube-llmops-stack
- **Real-time:** Server-Sent Events (SSE) via K8s informer watch
- **NodePort:** 30302

---

## 2. Architecture

```
┌─ Browser ─────────────────────────────────────────────┐
│  React SPA (Vite + React 18 + TanStack Query + antd)  │
│  http://NODE_IP:30302                                  │
└──────────────┬────────────────────────────────────────┘
               │  /api/*   REST + SSE
               │  /*       Static SPA (embed.FS)
┌──────────────▼────────────────────────────────────────┐
│  Go Gin Server (:3000)                                 │
│  ┌──────────┐ ┌──────────┐ ┌─────────┐ ┌───────────┐ │
│  │ Auth     │ │ Model    │ │ FT/RAG  │ │ Platform  │ │
│  │(OIDC)    │ │ Handler  │ │ Handler │ │ Handler   │ │
│  └────┬─────┘ └────┬─────┘ └────┬────┘ └─────┬─────┘ │
│       │            │            │             │       │
│  ┌────▼──────┐ ┌───▼────────────▼─────────────▼─────┐ │
│  │ RBAC      │ │      K8s Client Layer               │ │
│  │ Middleware │ │  controller-runtime client (CRDs)   │ │
│  │ (PG)      │ │  kubernetes.Clientset (Pods, Logs)   │ │
│  └───────────┘ └──────────────────┬──────────────────┘ │
│                                   │                    │
│  ┌────────────────────────────────▼──────────────────┐ │
│  │  Reverse Proxy Layer                               │ │
│  │  Grafana / Langfuse / Dify / MLflow / JupyterHub  │ │
│  │  (auth passthrough, URL rewrite)                   │ │
│  └───────────────────────────────────────────────────┘ │
└───────────────────────────────────────────────────────┘
               │
    ┌──────────▼──────────┐
    │   K8s API Server    │
    │   Keycloak          │
    │   PostgreSQL        │
    │   Grafana/Dify/...  │
    └─────────────────────┘
```

**Key design decisions:**

1. **Single port `:3000`** — Gin serves REST API (`/api/v1/*`) and embedded SPA static files (`/*`).
2. **K8s Client** — Reuses `operator/api/v1alpha1` type definitions via Go module `replace` directive.
3. **Tool integration** — Backend reverse-proxies to Grafana/Langfuse/Dify/MLflow/JupyterHub with auth header injection, avoiding CORS and cross-domain cookie issues.
4. **SSE** — `/api/v1/events` endpoint. Backend watches K8s informer changes and pushes to connected browsers.
5. **NodePort 30302** — Added to existing `nodeport-services.yaml`.
6. **Services proxy** — All installed UI services (Grafana, Langfuse, Dify, MLflow, JupyterHub, MinIO, Keycloak, LiteLLM, Prometheus) are accessible via same-origin reverse proxy at `/services/*`, with per-service SSO passthrough.
7. **Theme** — Dark / Light / Auto (system preference), stored in localStorage, applied via Ant Design 5 `ConfigProvider`.

---

## 3. RBAC System

### 3.1 Model

Keycloak handles authentication (login, JWT tokens, user identity). Dashboard maintains its own RBAC in PostgreSQL.

```
┌──────────────┐     ┌──────────────────┐     ┌──────────────┐
│ users        │     │ roles            │     │ permissions  │
│──────────────│     │──────────────────│     │──────────────│
│ id (PK)      │     │ id (PK)          │     │ id (PK)      │
│ keycloak_id  │◄──┐ │ name             │  ┌─▶│ resource     │
│ email        │   │ │ description      │  │  │ action       │
│ display_name │   │ │ is_system (bool) │  │  │ description  │
│ avatar       │   │ │ created_at       │  │  │ is_system    │
│ enabled      │   │ │ updated_at       │  │  │ created_at   │
│ last_login   │   │ └──────────────────┘  │  └──────────────┘
│ created_at   │   │          │            │
└──────────────┘   │          ▼            │
       │           │ ┌──────────────────┐  │
       │           │ │ role_permissions │  │
       ▼           │ │──────────────────│  │
┌──────────────┐   │ │ role_id (FK)─────┘  │
│ user_roles   │   │ │ permission_id (FK)──┘
│──────────────│   │ └──────────────────┘
│ user_id (FK)─┘   │
│ role_id (FK)─────┘
└──────────────┘
```

### 3.2 Default Permissions (seeded on first install)

| resource | actions | description |
|----------|---------|-------------|
| `models` | `view`, `create`, `edit`, `delete`, `scale`, `canary` | Model deployment management |
| `finetune` | `view`, `create`, `delete` | Fine-tuning jobs |
| `rag` | `view`, `create`, `upload`, `delete`, `query` | RAG knowledge bases |
| `platform` | `view`, `edit` | Platform configuration |
| `monitoring` | `view` | Grafana/Langfuse/MLflow/JupyterHub embeds |
| `users` | `view`, `create`, `edit`, `delete` | User management |
| `roles` | `view`, `create`, `edit`, `delete` | Role management |
| `permissions` | `view`, `create`, `edit`, `delete` | Permission management |

Total: ~24 default permission entries. All marked `is_system=true`.

Admin can create additional custom permissions via the UI.

### 3.3 Default Roles (seeded, `is_system=true`, cannot be deleted)

| Role | Permissions |
|------|-------------|
| `admin` | ALL permissions |
| `editor` | models/* + finetune/* + rag/* + monitoring:view + platform:view |
| `viewer` | All `*:view` permissions |

Admin can create custom roles (e.g., `ml-engineer`, `rag-operator`) and assign arbitrary permission combinations.

### 3.4 Auth Flow

1. **Login:** Browser → Keycloak OIDC authorization code flow → JWT (contains `sub` = keycloak_id)
2. **Authorization:** Gin middleware extracts `keycloak_id` from JWT → queries Dashboard DB for user_roles → aggregates permissions → checks `resource:action` against requested endpoint
3. **User sync:** On first login, user record auto-created in local DB from Keycloak token claims (email, name). Admin can also bulk-sync via Keycloak Admin API.
4. **User management UI:** CRUD operations write both Keycloak (create/disable user) and Dashboard DB (role assignment).
5. **Role/Permission management UI:** Pure Dashboard DB operations, no Keycloak involvement.

### 3.5 Keycloak Integration

- Dashboard registered as OIDC client `dashboard` in Keycloak realm `kube-llmops`
- Client secret: `dashboard-oidc-secret`
- Service account enabled for Keycloak Admin API access (user CRUD)
- Service account granted `realm-management` client role

---

## 4. Pages & Navigation

### 4.1 Layout

Left sidebar + top header. Sidebar is collapsible with 3 groups:

```
kube-llmops (logo)

WORKLOADS
  Overview          /
  Models            /models
    Deploy New      /models/new
  Fine-tuning       /finetune
  RAG               /rag

SERVICES
  Services          /services
    Grafana         /services/grafana
    Langfuse        /services/langfuse
    Dify            /services/dify
    MLflow          /services/mlflow
    JupyterHub      /services/jupyterhub
    MinIO           /services/minio
    Keycloak        /services/keycloak
    LiteLLM         /services/litellm
    Prometheus      /services/prometheus

OBSERVE
  Monitoring        /monitoring
    Grafana         /monitoring/grafana
    Langfuse        /monitoring/traces
    MLflow          /monitoring/mlflow
  Logs              /logs

ADMIN
  Platform          /platform
  Users             /users
    Roles           /users/roles
    Permissions     /users/permissions
```

Top header: namespace selector, cluster health indicator, theme toggle (sun/moon/auto icons), user avatar + dropdown (profile, logout).

### 4.2 Page Details

**Overview (`/`):**
- KPI cards: model count (by phase), GPU utilization, requests/s, active fine-tune runs, KB count
- Component health grid (8 services from LLMPlatform CR status.components)
- Recent activity feed (SSE-driven)
- Summary charts (Grafana embed: request throughput, GPU utilization)

**Models (`/models`):**
- Table: name, source, engine, replicas (ready/total), phase, endpoint
- Row actions: scale, canary, promote, rollback, delete
- Detail page (`/models/:name`): status card, conditions timeline, pod list with status, log viewer (SSE stream), canary status with traffic slider
- Deploy wizard (`/models/new`): source input → auto-detect engine → resource config → engine args → dry-run preview → confirm

**Fine-tuning (`/finetune`):**
- Run list: name, base model, method (LoRA/QLoRA/Full), phase, duration, quality gate result
- Create wizard (`/finetune/new`): base model → data source → hyperparameters → evaluation toggle → quality gate thresholds → deploy toggle → confirm
- Detail page (`/finetune/:name`): Argo workflow DAG visualization, training metrics (loss curve), MLflow link (expand iframe), quality gate results, output model info

**RAG (`/rag`):**
- Knowledge base list (via Dify console API): name, doc count, word count, created date
- Detail page (`/rag/:id`): document list, upload button (drag & drop), indexing status, test query panel with relevance scores
- Dify console embed (expandable iframe)

**Monitoring (`/monitoring`):**
- Summary cards for each Grafana dashboard (11 dashboards): title, key metric preview
- Click to expand → iframe loads full Grafana dashboard with auth passthrough
- Langfuse traces summary + expandable iframe
- MLflow experiments summary + expandable iframe

**JupyterHub (`/notebooks`):**
- Summary: active notebook servers count, active users
- Expandable iframe to JupyterHub UI

**Services (`/services`):**
- Grid of cards for all installed UI services, auto-discovered from LLMPlatform CR `status.components`
- Each card: service icon, name, description, health status badge, "Open" button
- Only services with `phase=Ready` are shown; others appear greyed out with status
- Click "Open" → navigates to `/services/:name`, renders full-page iframe via same-origin reverse proxy
- SSO passthrough: user does NOT see a second login prompt (see Section 5.5 for per-service auth mechanism)

| Service | Internal URL | Auth Passthrough Method |
|---------|-------------|------------------------|
| Grafana | `{release}-grafana:3000` | `X-WEBAUTH-USER` header injection (auth.proxy mode) |
| Langfuse | `{release}-langfuse:3000` | Bearer token (service account API key) |
| Dify | `{release}-dify-web:3000` | Cookie session (difySession reuse) |
| MLflow | `{release}-mlflow:5000` | No auth (cluster-internal access) |
| JupyterHub | `{release}-jupyterhub:8000` | OAuth token relay |
| MinIO Console | `{release}-minio:9001` | Session token injection |
| Keycloak | `{release}-keycloak:8080` | Direct proxy (admin already authenticated) |
| LiteLLM | `{release}-litellm:4000` | Master key header injection |
| Prometheus | `{release}-prometheus:9090` | No auth (cluster-internal access) |

**Logs (`/logs`):**
- Aggregated pod logs via Loki (Grafana Explore embed)
- Namespace + pod selector filters

**Platform (`/platform`, Admin only):**
- LLMPlatform CR status card: phase, helm release/revision
- Component health table with endpoints and NodePorts
- Module toggles: RAG, Finetune, Security (edit → updates LLMPlatform CR)
- Gateway config: routing strategy, rate limiting, budget control
- NodePort / Ingress / SSO settings display

**Users (`/users`, Admin only):**
- User table: name, email, roles (tags), enabled, last login
- Create user: form → writes Keycloak + local DB
- Edit user: update info, enable/disable, reassign roles
- Delete user: removes from Keycloak + local DB

**Roles (`/users/roles`, Admin only):**
- Role table: name, description, permission count, user count, system flag
- Create/edit role: name, description, permission checkbox matrix (grouped by resource)
- System roles (admin/editor/viewer) are read-only, cannot be deleted

**Permissions (`/users/permissions`, Admin only):**
- Permission table: resource, action, description, system flag, role count
- Create permission: resource + action + description
- System permissions are read-only, cannot be deleted

**Profile (`/profile`):**
- Current user info (from Keycloak token)
- Assigned roles and effective permissions list

### 4.3 Theme Support

Three theme modes: **Dark**, **Light**, **Auto** (follows OS `prefers-color-scheme`).

- **Toggle location:** Top header, right side, next to user avatar. Icon cycles: sun (light) → moon (dark) → auto (system) icon.
- **Implementation:** Ant Design 5 `ConfigProvider` with `theme.algorithm`:
  - Light: `theme.defaultAlgorithm`
  - Dark: `theme.darkAlgorithm`
  - Auto: listens to `window.matchMedia('(prefers-color-scheme: dark)')` and switches dynamically
- **Persistence:** Stored in `localStorage('theme')` + Zustand `auth` store. Survives page reload.
- **Sidebar:** Dark sidebar uses fixed dark palette in all themes (already dark by default). Light theme lightens the main content area only.
- **Iframe embeds:** Grafana supports `&theme=dark`/`&theme=light` query param — appended automatically based on current theme. Other services use their default theme.

---

## 5. REST API

**Base path:** `/api/v1`
**Auth:** All endpoints require Bearer JWT (Keycloak-issued). Middleware validates token + checks RBAC.

### 5.1 Models (ModelDeployment CR)

```
GET    /api/v1/models                     → List (supports ?watch=true SSE)
POST   /api/v1/models                     → Deploy new model
GET    /api/v1/models/:name               → Detail (status + conditions)
PUT    /api/v1/models/:name               → Update (engine args, resources)
DELETE /api/v1/models/:name               → Delete
PATCH  /api/v1/models/:name/scale         → Scale {replicas: N}
PATCH  /api/v1/models/:name/canary        → Set canary {source, weight}
POST   /api/v1/models/:name/promote       → Promote canary to primary
POST   /api/v1/models/:name/rollback      → Rollback canary
GET    /api/v1/models/:name/pods          → Pod list (name, status, node)
GET    /api/v1/models/:name/logs          → Pod logs (SSE stream)
POST   /api/v1/models/:name/test          → Quick inference test
```

### 5.2 Fine-tuning (FineTuneRun CR)

```
GET    /api/v1/finetune                   → List
POST   /api/v1/finetune                   → Create fine-tune run
GET    /api/v1/finetune/:name             → Detail (metrics, QG, MLflow)
DELETE /api/v1/finetune/:name             → Delete
GET    /api/v1/finetune/:name/logs        → Training logs (SSE)
```

### 5.3 RAG (Dify proxy)

```
GET    /api/v1/rag/knowledgebases         → KB list
POST   /api/v1/rag/knowledgebases         → Create KB
GET    /api/v1/rag/knowledgebases/:id     → KB detail + document list
DELETE /api/v1/rag/knowledgebases/:id     → Delete KB
POST   /api/v1/rag/knowledgebases/:id/upload  → Upload document
POST   /api/v1/rag/knowledgebases/:id/query   → Test query
```

### 5.4 Platform (LLMPlatform CR)

```
GET    /api/v1/platform                   → Platform status + component health
PUT    /api/v1/platform                   → Update config (modules, gateway)
GET    /api/v1/platform/components        → Component endpoints + NodePorts
```

### 5.5 Services (Unified Portal with SSO Passthrough)

```
GET    /api/v1/services                        → List installed services (auto-discovered from LLMPlatform CR)
GET    /api/v1/services/:name/status           → Service health + endpoint info
ALL    /services/grafana/*                     → Grafana reverse proxy (X-WEBAUTH-USER injection)
ALL    /services/langfuse/*                    → Langfuse reverse proxy (Bearer token)
ALL    /services/dify/*                        → Dify reverse proxy (cookie session)
ALL    /services/mlflow/*                      → MLflow reverse proxy (no auth)
ALL    /services/jupyterhub/*                  → JupyterHub reverse proxy (OAuth relay)
ALL    /services/minio/*                       → MinIO Console reverse proxy (session token)
ALL    /services/keycloak/*                    → Keycloak reverse proxy (direct)
ALL    /services/litellm/*                     → LiteLLM reverse proxy (master key)
ALL    /services/prometheus/*                  → Prometheus reverse proxy (no auth)
```

Note: Service proxy routes are mounted at `/services/*` (not `/api/v1/`) to avoid path conflicts with service-internal routing. Each proxy strips the `/services/:name` prefix before forwarding.

### 5.6 Monitoring (Summary)

```
GET    /api/v1/monitoring/summary              → Aggregated summary (Prometheus queries)
GET    /api/v1/notebooks/summary               → JupyterHub status (active servers)
```

Note: Full Grafana/Langfuse/MLflow/JupyterHub iframe access goes through `/services/*` proxy (Section 5.5). The `/monitoring` page uses summary API + links to `/services/:name`.

### 5.7 Users & RBAC

```
GET    /api/v1/users                      → User list (local DB + Keycloak sync)
POST   /api/v1/users                      → Create user (→ Keycloak + local DB)
GET    /api/v1/users/:id                  → User detail
PUT    /api/v1/users/:id                  → Edit user
DELETE /api/v1/users/:id                  → Delete/disable user
PUT    /api/v1/users/:id/roles            → Assign roles {roleIds: [...]}

GET    /api/v1/roles                      → Role list
POST   /api/v1/roles                      → Create role
GET    /api/v1/roles/:id                  → Role detail + permission list
PUT    /api/v1/roles/:id                  → Update role (name/description)
DELETE /api/v1/roles/:id                  → Delete role (system roles protected)
PUT    /api/v1/roles/:id/permissions      → Set role permissions {permissionIds: [...]}

GET    /api/v1/permissions                → Permission list
POST   /api/v1/permissions                → Create permission {resource, action}
PUT    /api/v1/permissions/:id            → Update permission
DELETE /api/v1/permissions/:id            → Delete permission (system perms protected)
```

### 5.8 SSE & Auth

```
GET    /api/v1/events                     → SSE stream (model status, FT progress, component health)
GET    /api/v1/auth/me                    → Current user info + effective permissions
POST   /api/v1/auth/callback              → OIDC callback (exchange code → session)
POST   /api/v1/auth/refresh               → Refresh token
POST   /api/v1/auth/logout                → Logout
```

**Total: ~45 API endpoints + 9 service proxy routes**, organized into 8 handler files + 1 proxy module.

---

## 6. Tech Stack & Project Structure

### 6.1 Frontend

| Dependency | Purpose |
|------------|---------|
| React 18 | UI framework |
| Vite | Build tool, dev server proxies `/api` → Go backend |
| Ant Design 5 | Component library (Table, Form, Layout, Menu, Modal) |
| React Router 6 | Client-side routing |
| TanStack Query | Data fetching, caching, SSE integration |
| Zustand | Lightweight global state (user info, permissions, sidebar, theme) |
| TypeScript | Type safety |

```
dashboard/web/
├── package.json
├── vite.config.ts               # proxy /api → localhost:3000
├── src/
│   ├── main.tsx
│   ├── App.tsx                  # Router + AuthProvider + Layout
│   ├── api/
│   │   ├── client.ts            # base fetch with JWT auto-inject
│   │   ├── models.ts
│   │   ├── finetune.ts
│   │   ├── rag.ts
│   │   ├── platform.ts
│   │   ├── users.ts
│   │   └── monitoring.ts
│   ├── hooks/
│   │   ├── useSSE.ts            # SSE hook (EventSource + auto-reconnect)
│   │   ├── usePermission.ts     # RBAC check hook
│   │   └── useTheme.ts          # Theme hook (dark/light/auto + media query listener)
│   ├── store/
│   │   └── auth.ts              # Zustand: user, permissions, token, theme
│   ├── components/
│   │   ├── Layout/              # Sidebar + Header + Content
│   │   ├── PermissionGuard.tsx  # Show/hide UI by permission
│   │   ├── IframeEmbed.tsx      # Generic iframe embed + loading state
│   │   ├── ThemeToggle.tsx      # Sun/moon/auto icon toggle button
│   │   └── ServiceCard.tsx      # Service card with health badge + open button
│   └── pages/
│       ├── Overview/
│       ├── Models/              # List + Detail + DeployWizard
│       ├── Finetune/            # List + Detail + CreateWizard
│       ├── Rag/                 # List + Detail + Upload
│       ├── Services/            # Service grid + per-service iframe page
│       ├── Monitoring/          # Summary + links to /services/*
│       ├── Notebooks/           # JupyterHub summary + iframe
│       ├── Logs/
│       ├── Platform/
│       ├── Users/               # List + Roles + Permissions
│       └── Profile/
```

### 6.2 Backend

| Dependency | Purpose |
|------------|---------|
| gin-gonic/gin | HTTP framework |
| gin-contrib/cors | CORS middleware |
| coreos/go-oidc/v3 | OIDC token verification |
| gorm.io/gorm + gorm.io/driver/postgres | ORM for RBAC tables |
| sigs.k8s.io/controller-runtime/pkg/client | K8s CRD access |
| k8s.io/client-go | Pod, Service, Logs |
| operator/api/v1alpha1 | CRD type definitions (go module replace) |

```
dashboard/
├── go.mod
├── main.go                          # Gin setup + embed SPA + SSE + graceful shutdown
├── embed.go                         # //go:embed web/dist
├── internal/
│   ├── auth/
│   │   ├── oidc.go                  # Keycloak OIDC token verification
│   │   └── middleware.go            # JWT extract + RBAC check
│   ├── rbac/
│   │   ├── models.go                # GORM models (User, Role, Permission, etc.)
│   │   ├── service.go               # RBAC business logic
│   │   ├── seed.go                  # Seed default roles and permissions
│   │   └── sync.go                  # Keycloak → local DB user sync
│   ├── handler/
│   │   ├── models.go                # /api/v1/models handlers
│   │   ├── finetune.go              # /api/v1/finetune
│   │   ├── rag.go                   # /api/v1/rag (proxy to Dify)
│   │   ├── platform.go              # /api/v1/platform
│   │   ├── monitoring.go            # /api/v1/monitoring + /api/v1/notebooks
│   │   ├── services.go              # /api/v1/services (list + status)
│   │   ├── users.go                 # /api/v1/users CRUD
│   │   ├── roles.go                 # /api/v1/roles CRUD
│   │   └── permissions.go           # /api/v1/permissions CRUD
│   ├── sse/
│   │   └── broker.go                # SSE broker: K8s informer → client channels
│   ├── proxy/
│   │   ├── reverse.go               # Generic reverse proxy with path strip + header injection
│   │   ├── auth_grafana.go          # Grafana: X-WEBAUTH-USER header
│   │   ├── auth_langfuse.go         # Langfuse: Bearer token
│   │   ├── auth_dify.go             # Dify: cookie session (reuse difySession pattern)
│   │   ├── auth_minio.go            # MinIO: session token
│   │   ├── auth_litellm.go          # LiteLLM: master key header
│   │   └── registry.go             # Service registry: name → target URL + auth strategy
│   └── kube/
│       └── client.go                # K8s client init (reuses operator util patterns)
├── web/
│   └── dist/                        # React build artifacts (embed target)
└── Dockerfile                       # multi-stage build
```

### 6.3 Dockerfile (multi-stage)

Build context is the **repo root** (`kube-llmops/`) because the Go module uses a `replace` directive to reference `operator/api/v1alpha1`.

```dockerfile
# Stage 1: Build React
FROM node:20-alpine AS frontend
WORKDIR /app/web
COPY dashboard/web/package.json dashboard/web/package-lock.json ./
RUN npm ci
COPY dashboard/web/ .
RUN npm run build

# Stage 2: Build Go
FROM golang:1.22-alpine AS backend
WORKDIR /app/dashboard
COPY operator/api/ /app/operator/api/
COPY operator/go.mod operator/go.sum /app/operator/
COPY dashboard/go.mod dashboard/go.sum ./
COPY --from=frontend /app/web/dist ./web/dist
RUN go mod download
COPY dashboard/ .
RUN CGO_ENABLED=0 go build -o dashboard .

# Stage 3: Runtime
FROM gcr.io/distroless/static
COPY --from=backend /app/dashboard/dashboard /dashboard
ENTRYPOINT ["/dashboard"]
```

```
# go.mod replace directive
replace github.com/kube-llmops/operator => ../operator
```

Build command (from repo root):
```bash
docker build -t kube-llmops/dashboard:latest -f dashboard/Dockerfile .
```

Target image size: < 50MB.

---

## 7. Helm Subchart & Deployment

### 7.1 Subchart Structure

```
charts/kube-llmops-stack/charts/dashboard/
├── Chart.yaml
├── values.yaml
└── templates/
    ├── deployment.yaml
    ├── service.yaml
    ├── configmap.yaml         # Environment variable config
    ├── secret.yaml            # OIDC client secret, DB password
    ├── db-init-job.yaml       # Helm hook: init dashboard DB + seed RBAC
    └── _helpers.tpl
```

### 7.2 values.yaml Defaults

```yaml
dashboard:
  enabled: true
  image:
    repository: kube-llmops/dashboard
    tag: latest
  replicas: 1
  resources:
    requests:
      cpu: 100m
      memory: 128Mi
    limits:
      cpu: 500m
      memory: 256Mi

  database:
    host: "kube-llmops-postgresql"
    port: 5432
    name: "dashboard"
    user: "postgres"

  oidc:
    issuerUrl: ""              # auto: http://{release}-keycloak:8080/realms/kube-llmops
    clientId: "dashboard"
    clientSecret: "dashboard-oidc-secret"

  proxy:
    grafana: ""                # auto: http://{release}-grafana:3000
    langfuse: ""               # auto: http://{release}-langfuse:3000
    dify: ""                   # auto: http://{release}-dify-web:3000
    mlflow: ""                 # auto: http://{release}-mlflow:5000
    jupyterhub: ""             # auto: http://{release}-jupyterhub:8000
    minio: ""                  # auto: http://{release}-minio:9001
    keycloak: ""               # auto: http://{release}-keycloak:8080
    litellm: ""                # auto: http://{release}-litellm:4000
    prometheus: ""             # auto: http://{release}-prometheus:9090
```

### 7.3 Integration Points (existing files to modify)

| File | Change |
|------|--------|
| `Chart.yaml` | Add `dashboard` dependency |
| `values-single-node.yaml` | Add `dashboard:` config section |
| `nodeport-services.yaml` | Add dashboard NodePort 30302 |
| `keycloak/realm-configmap.yaml` | Add `dashboard` OIDC client (with service account) |
| `postgresql init-db.sql` | Add `CREATE DATABASE dashboard` |

### 7.4 NodePort

```yaml
# In nodeport-services.yaml
---
apiVersion: v1
kind: Service
metadata:
  name: {{ .Release.Name }}-dashboard-np
spec:
  type: NodePort
  selector:
    app.kubernetes.io/name: dashboard
  ports:
    - port: 3000
      targetPort: 3000
      nodePort: {{ $np.dashboard | default 30302 }}
```

### 7.5 DB Init Job

Helm hook (`post-install,post-upgrade`), runs Go binary with `--migrate` flag:
1. `CREATE TABLE IF NOT EXISTS` — users, roles, permissions, user_roles, role_permissions
2. Seed default permissions (~24 resource:action pairs, `is_system=true`)
3. Seed 3 system roles (admin/editor/viewer) + associate permissions
4. Idempotent: repeated upgrades do not duplicate data

---

## 8. Testing Strategy

### 8.1 Backend (Go)

| Level | Tool | Coverage |
|-------|------|----------|
| Unit | `go test` | RBAC service logic, permission check, seed data, proxy URL building |
| API | `httptest` + Gin test mode | Handler request/response, RBAC middleware blocking, error codes |
| Integration | `testcontainers-go` (PostgreSQL) | GORM migration, seed, full CRUD lifecycle |

~60 test cases covering:
- RBAC: permission check, role aggregation, system role protection, unauthorized rejection
- Handlers: CRUD happy path + validation + 404/403 errors
- SSE: event format and streaming
- Proxy: URL rewrite, header injection

### 8.2 Frontend (React)

| Level | Tool | Coverage |
|-------|------|----------|
| Component | Vitest + React Testing Library | Table rendering, form validation, permission guard |
| E2E | Playwright | Login → deploy model → view status → user management full flow |

### 8.3 Helm

| Type | Tool | Coverage |
|------|------|----------|
| Template | `pytest` + `helm template` | ConfigMap values, OIDC config, NodePort, DB init job |
| Smoke | Helm test hook | Pod readiness + `/api/v1/auth/me` returns 200 |

### 8.4 Acceptance Criteria

1. `go test ./...` all green
2. `npm test` all green
3. `helm template` no errors
4. Playwright E2E: login → deploy model → scale → user CRUD → role/permission CRUD → view Grafana embed
5. Docker build succeeds, image < 50MB

---

## 9. Out of Scope

- Multi-cluster dashboard (future)
- i18n / localization (future)
- Audit log (beyond Keycloak login events)
- Dashboard-specific alerting rules
