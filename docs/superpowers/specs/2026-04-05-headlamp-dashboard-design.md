# Design: Replace Custom Dashboard with Headlamp + Plugin

**Date:** 2026-04-05
**Status:** Draft

## Context

The current custom Go+React dashboard (`dashboard/`) uses a reverse proxy to embed
services (Grafana, Langfuse, Dify, etc.) inside iframes. The proxy approach is brittle —
Next.js apps (Langfuse, Dify) break due to Turbopack runtime asset-prefix mismatches,
and every SPA framework has its own set of proxy quirks. The maintenance cost outweighs
the benefit.

## Decision

Replace the custom dashboard with **Headlamp** (CNCF sandbox Kubernetes web UI) plus a
custom **Headlamp plugin** that provides service links and Grafana iframe embedding.

## What Gets Removed

| Item | Path |
|------|------|
| Go backend + React frontend | `dashboard/` (entire directory) |
| Dashboard subchart | `charts/kube-llmops-stack/charts/dashboard/` |
| Dashboard dependency in umbrella Chart.yaml | `dashboard` entry |
| Dashboard values | `dashboard.*` in `values-single-node.yaml` |
| Dashboard NodePort template lines | `nodeport-services.yaml` (dashboard block) |
| Dashboard tests | `tests/e2e/test_dashboard_*.py`, `tests/e2e/diagnose_*.py` |

The `images/model-loader/`, `images/model-resolver/`, proxy test files, and all other
charts remain untouched.

## What Gets Added

### 1. Headlamp Subchart

**Path:** `charts/kube-llmops-stack/charts/headlamp/`

A thin wrapper subchart that depends on the upstream Headlamp Helm chart. This keeps
the dependency structure consistent with other subcharts (local `Chart.yaml` + templates).

```yaml
# Chart.yaml
apiVersion: v2
name: headlamp
version: 0.1.0
dependencies:
  - name: headlamp
    version: "0.25.0"   # pin to latest stable
    repository: "https://kubernetes-sigs.github.io/headlamp/"
```

Key values wired from the parent chart:

| Value | Source | Purpose |
|-------|--------|---------|
| `config.oidc.clientID` | `global.sso.clientId` or `"headlamp"` | Keycloak OIDC |
| `config.oidc.clientSecret` | `global.sso.clientSecret` | Keycloak OIDC |
| `config.oidc.issuerURL` | computed from `global.nodePort.host` | Keycloak realm URL |
| `service.type` | `NodePort` when `global.nodePort.enabled` | External access |
| `service.nodePort` | `30302` (reuse old dashboard port) | Backward compat |
| `clusterRoleBinding.create` | `true` | Full cluster visibility |

OIDC is optional — when Keycloak is disabled, Headlamp falls back to service-account
token auth (the default).

### 2. Headlamp Plugin — `kube-llmops-portal`

**Path:** `plugins/kube-llmops-portal/`

A TypeScript Headlamp plugin with two pages, registered via `registerSidebarEntry` and
`registerRoute`.

#### Plugin Structure

```
plugins/kube-llmops-portal/
├── src/
│   ├── index.tsx              # Plugin entry: register sidebar + routes
│   ├── ServiceLinksPage.tsx   # Card grid with links to all services
│   └── MonitoringPage.tsx     # Grafana iframe embedding
├── package.json
└── tsconfig.json
```

#### Sidebar Registration

```
LLMOps (section header)
├── Service Links    → /kube-llmops/services
└── Monitoring       → /kube-llmops/monitoring
```

#### Service Links Page

A Material UI card grid. Each card shows:
- Service name + icon
- Short description
- "Open" button → `window.open()` to the NodePort URL

Services listed:

| Service | Default Port | Description |
|---------|-------------|-------------|
| Grafana | 30300 | Monitoring dashboards |
| Langfuse | 30301 | LLM tracing & analytics |
| LiteLLM | 30400 | AI gateway |
| Dify | 30500 | RAG platform |
| Keycloak | 30808 | SSO & identity |
| Prometheus | 30909 | Metrics |
| MinIO | 30901 | Object storage console |
| MLflow | 30505 | Experiment tracking |
| JupyterHub | 30888 | Notebooks |

The base URL (`NODE_IP`) is passed to the plugin via a ConfigMap that Headlamp mounts,
or read from `window.location.hostname` (since Headlamp itself is accessed via NodePort
on the same host). The port mapping is hardcoded in the plugin to match the Helm chart
defaults.

#### Monitoring Page

Embeds key Grafana dashboards via `<iframe>`. Uses Grafana kiosk mode:

```
http://{host}:30300/d/{uid}/?orgId=1&kiosk
```

Dashboards shown (tabs or dropdown selector):

| Tab | Grafana UID |
|-----|-------------|
| vLLM | `vllm-overview` |
| LiteLLM Gateway | `litellm-gateway` |
| System | `system-overview` |
| GPU | `gpu-overview` |
| SLO | `slo-overview` |
| Cost | `cost-usage` |

Additional dashboards (RAG Quality, Finetune, etc.) can be added later.

### 3. Grafana Configuration Changes

In `charts/kube-llmops-stack/charts/observability/templates/grafana.yaml`, add:

```yaml
GF_SECURITY_ALLOW_EMBEDDING: "true"
GF_AUTH_ANONYMOUS_ENABLED: "true"
GF_AUTH_ANONYMOUS_ORG_ROLE: "Viewer"
```

This allows:
- `allow_embedding` — removes `X-Frame-Options: deny` so Grafana renders in iframes
- Anonymous auth — the iframe doesn't need to authenticate (read-only Viewer role)

Existing OIDC auth is preserved — direct Grafana access still uses SSO.

### 4. Plugin Deployment

The plugin is built into a container image and loaded via an init container in the
Headlamp deployment.

```dockerfile
# plugins/kube-llmops-portal/Dockerfile
FROM node:22-alpine AS build
WORKDIR /plugin
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build
RUN mkdir -p /plugins/kube-llmops-portal && \
    cp dist/main.js package.json /plugins/kube-llmops-portal/

FROM busybox:stable
COPY --from=build /plugins /plugins
```

The Headlamp subchart values configure the init container:

```yaml
initContainers:
  - name: install-llmops-plugin
    image: "{{ .Values.plugin.image }}:{{ .Values.plugin.tag }}"
    command: ["sh", "-c", "cp -r /plugins/* /headlamp/plugins/"]
    volumeMounts:
      - name: plugins
        mountPath: /headlamp/plugins
```

### 5. NodePort Changes

In `nodeport-services.yaml`:
- Remove the `dashboard` NodePort block
- Add a `headlamp` NodePort block pointing to port 30302

## Architecture Diagram

```
Headlamp (:30302)                        Grafana (:30300)
┌──────────────────────────────┐         ┌─────────────────┐
│ K8s native UI                │         │ allow_embedding  │
│  - Pods, Deployments, Logs   │         │ anonymous=Viewer │
│  - Services, ConfigMaps      │         └────────▲────────┘
│                              │                  │ iframe
│ [Plugin] Service Links ──────┼──→ window.open() to :304xx
│ [Plugin] Monitoring ─────────┼──→ <iframe src="grafana:30300/d/...&kiosk">
└──────────────────────────────┘
```

## Migration Path

1. Build and push the plugin container image
2. Add the headlamp subchart + update Grafana config
3. Remove the old dashboard subchart and code
4. `helm dependency update` + `helm upgrade`
5. Users access Headlamp at the same `:30302` port

## What We Lose (Acceptable)

- RBAC/user management UI (Keycloak handles this directly)
- Model management UI (CLI / Helm values)
- Finetune wizard UI (Argo Workflows UI or CLI)
- Embedded Langfuse/Dify (users open them directly via links)
- Go API endpoints (not needed without the custom frontend)

## What We Gain

- Native K8s management (pods, logs, events, RBAC) out of the box
- Zero reverse proxy maintenance
- Plugin system for future extensions
- CNCF-backed project with active community
- Simpler deployment (no custom Go binary)
