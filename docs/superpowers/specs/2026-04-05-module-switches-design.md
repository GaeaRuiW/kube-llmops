# Module Switches Design Spec

**Date:** 2026-04-05
**Status:** Implemented in v0.5.0
**Goal:** Unified module-level feature toggles for RAG, fine-tuning, and security — one switch controls all related services, dashboards, and alert rules.

## Problem

Currently 19 subcharts each have independent `enabled` flags. Enabling RAG requires setting `dify.enabled`, `milvus.enabled`, `lightrag.enabled`, `rag-eval.enabled` individually. All 11 Grafana dashboards are unconditionally loaded via `.Files.Glob`, so disabled modules still show empty "no data" dashboards.

## Design

### Module Definitions

```yaml
modules:
  rag:
    enabled: false      # dify, milvus, lightrag, rag-eval
  finetune:
    enabled: false      # finetune, jupyterhub
  security:
    enabled: false      # security (llm-guard, presidio, networkPolicy, multiTenant)
```

Components NOT in any module keep their existing independent `enabled` flags:
- Core (always on by profile): vllm, llamacpp, tei, litellm, postgresql, fluid
- Standalone toggles: keycloak, langfuse, logging, keda, harbor, observability

### Module → Component Mapping

| Module | Components | Dashboards | Alert Rules |
|--------|-----------|------------|-------------|
| `modules.rag` | dify, milvus, lightrag, rag-eval | rag-quality.json, milvus-overview.json | FaithfulnessLow, FaithfulnessCritical, RelevancyLow, QualityRegression, EvalStale |
| `modules.finetune` | finetune, jupyterhub | finetune-overview.json | (none currently) |
| `modules.security` | security | tenant-overview.json | (none currently) |

### Priority: Sub-component Override Always Wins

Uses Helm native comma-separated condition paths. Helm evaluates left-to-right, first existing path wins:

```yaml
# Chart.yaml
- name: dify
  condition: dify.enabled,modules.rag.enabled
```

Behavior matrix:

| dify.enabled | modules.rag.enabled | Result |
|-------------|-------------------|--------|
| (undefined) | true | ON — fallback to module |
| (undefined) | false | OFF — fallback to module |
| true | false | ON — explicit override |
| false | true | OFF — explicit override |

Key requirement: `dify.enabled` must NOT exist in default values for this fallback to work. Remove from both subchart `values.yaml` and parent defaults.

### Chart.yaml Changes

Module-grouped subcharts get dual-path conditions:

```yaml
# RAG module
- name: dify
  condition: dify.enabled,modules.rag.enabled
- name: milvus
  condition: milvus.enabled,modules.rag.enabled
- name: lightrag
  condition: lightrag.enabled,modules.rag.enabled
- name: rag-eval
  condition: rag-eval.enabled,modules.rag.enabled

# Finetune module
- name: finetune
  condition: finetune.enabled,modules.finetune.enabled
- name: jupyterhub
  condition: jupyterhub.enabled,modules.finetune.enabled

# Security module
- name: security
  condition: security.enabled,modules.security.enabled
```

Non-module subcharts keep single conditions unchanged:

```yaml
- name: vllm
  condition: vllm.enabled
- name: litellm
  condition: litellm.enabled
# ... etc
```

### Values Changes

**Parent `values.yaml`:** Add modules block, remove `enabled` keys from module-grouped components.

```yaml
# NEW
modules:
  rag:
    enabled: false
  finetune:
    enabled: false
  security:
    enabled: false

# REMOVE these keys (were previously defined):
# dify:
#   enabled: true    ← delete this line
# milvus:
#   enabled: true    ← delete this line
# lightrag:
#   enabled: true    ← delete this line
# rag-eval:
#   enabled: true    ← delete this line
# finetune:
#   enabled: false   ← delete this line
# jupyterhub:
#   enabled: false   ← delete this line
# security:
#   enabled: true    ← delete this line
#
# Other config keys under these sections (e.g. dify.setup, security.llmGuard) remain.
```

**Subchart `values.yaml` files:** Remove `enabled:` line from each module-grouped subchart:
- `charts/dify/values.yaml` — remove `enabled: true`
- `charts/milvus/values.yaml` — remove `enabled: true`
- `charts/lightrag/values.yaml` — remove `enabled: true`
- `charts/rag-eval/values.yaml` — remove `enabled: true`
- `charts/finetune/values.yaml` — remove `enabled: true`
- `charts/security/values.yaml` — remove `enabled: true`

Note: JupyterHub may not have its own values.yaml if it's a simple subchart. Check and remove if present.

**`values-single-node.yaml`:** Replace individual enables with module switches.

```yaml
# BEFORE
dify:
  enabled: true
milvus:
  enabled: true
lightrag:
  enabled: true
rag-eval:
  enabled: true
security:
  enabled: true
finetune:
  enabled: false

# AFTER
modules:
  rag:
    enabled: true
  finetune:
    enabled: false
  security:
    enabled: true

# dify/milvus/lightrag/rag-eval/security sections still exist
# for their configuration, just without the `enabled:` key
```

**`values-ci.yaml`:** All modules off (minimal).

### Global Module State for Cross-Chart Access

Subcharts (especially observability) need to know module state. Pass via `global`:

```yaml
# In parent _helpers.tpl, define a helper:
{{- define "kube-llmops.modulesGlobal" -}}
modules:
  rag: {{ ((.Values.modules).rag).enabled | default false }}
  finetune: {{ ((.Values.modules).finetune).enabled | default false }}
  security: {{ ((.Values.modules).security).enabled | default false }}
{{- end -}}
```

Or simply rely on Helm's automatic `global` passthrough — add to parent values:

```yaml
global:
  modules:
    rag: false
    finetune: false
    security: false
```

And sync with `modules.*` in the values files. The simpler approach: just use `global.modules` directly as the source of truth for cross-chart access, and keep `modules.*` as the user-facing interface that maps to both Chart.yaml conditions and global values.

**Decision:** Use `global.modules` as the canonical cross-chart path. The parent chart values define:

```yaml
global:
  modules:
    rag: false
    finetune: false
    security: false
```

Chart.yaml conditions use `modules.rag.enabled` (top-level, for condition evaluation). We need both:

```yaml
# User-facing + Chart.yaml condition path
modules:
  rag:
    enabled: false
  finetune:
    enabled: false
  security:
    enabled: false

# Cross-chart access (observability needs this)
global:
  modules:
    rag: false
    finetune: false
    security: false
```

To keep them in sync without duplication, the parent `_helpers.tpl` can define a helper, but Helm values are static. The pragmatic solution: document that users should set both, or only use one. 

**Simplification:** Use ONLY `modules.rag.enabled` etc. The observability subchart accesses parent values via `.Values.global.modules` if we set them in global, OR we can pass them explicitly in the parent chart's values block for the observability subchart. Since the observability subchart is local, we can template its dashboard ConfigMap in the PARENT chart instead.

**Final decision:** Move the dashboard ConfigMap generation to the parent chart's templates directory. This gives the parent chart direct access to `modules.*` values without needing global passthrough. The observability subchart handles Grafana deployment/pods; the parent chart handles dashboard content.

### Dashboard Conditional Provisioning

**Current:** Single ConfigMap in `charts/observability/templates/grafana.yaml` using `.Files.Glob "dashboards/*.json"`.

**New:** Move dashboard ConfigMap to parent chart `templates/grafana-dashboards.yaml`. The parent chart has direct access to `modules.*` values.

```yaml
# templates/grafana-dashboards.yaml
{{- if .Values.observability.grafana.enabled | default false }}
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-grafana-dashboards
data:
  # Core dashboards (always included when grafana is on)
  vllm-overview.json: |
    {{- .Files.Get "charts/observability/dashboards/vllm-overview.json" | nindent 4 }}
  litellm-gateway.json: |
    {{- .Files.Get "charts/observability/dashboards/litellm-gateway.json" | nindent 4 }}
  slo-overview.json: |
    {{- .Files.Get "charts/observability/dashboards/slo-overview.json" | nindent 4 }}
  system-overview.json: |
    {{- .Files.Get "charts/observability/dashboards/system-overview.json" | nindent 4 }}
  gpu-overview.json: |
    {{- .Files.Get "charts/observability/dashboards/gpu-overview.json" | nindent 4 }}
  cost-usage.json: |
    {{- .Files.Get "charts/observability/dashboards/cost-usage.json" | nindent 4 }}
  infrastructure-roi.json: |
    {{- .Files.Get "charts/observability/dashboards/infrastructure-roi.json" | nindent 4 }}

  # RAG module dashboards
  {{- if (.Values.modules.rag).enabled }}
  rag-quality.json: |
    {{- .Files.Get "charts/observability/dashboards/rag-quality.json" | nindent 4 }}
  milvus-overview.json: |
    {{- .Files.Get "charts/observability/dashboards/milvus-overview.json" | nindent 4 }}
  {{- end }}

  # Finetune module dashboard
  {{- if (.Values.modules.finetune).enabled }}
  finetune-overview.json: |
    {{- .Files.Get "charts/observability/dashboards/finetune-overview.json" | nindent 4 }}
  {{- end }}

  # Security module dashboard
  {{- if (.Values.modules.security).enabled }}
  tenant-overview.json: |
    {{- .Files.Get "charts/observability/dashboards/tenant-overview.json" | nindent 4 }}
  {{- end }}
{{- end }}
```

Remove the old `.Files.Glob` ConfigMap from `charts/observability/templates/grafana.yaml`. Keep the dashboard provisioning provider ConfigMap and volume mounts there (they reference the ConfigMap name, which stays the same).

Note: `.Files.Get` in the parent chart can access subchart files via `charts/<subchart>/` path prefix. This is a documented Helm behavior — parent charts can read files from subchart directories.

### Prometheus Alert Rules

Move RAG-specific alert rules behind module condition. Currently in `charts/observability/templates/prometheus.yaml`.

**Approach:** Split alert rules into sections within the prometheus config, wrapped with conditionals that check `global.modules`. Since prometheus config is in the observability subchart, we need `global.modules` for this.

Add to parent values:

```yaml
global:
  modules:
    rag: false
    finetune: false
    security: false
```

In `values-single-node.yaml`, sync:

```yaml
global:
  modules:
    rag: true
    security: true

modules:
  rag:
    enabled: true
  security:
    enabled: true
```

In `charts/observability/templates/prometheus.yaml`, wrap RAG alerts:

```yaml
{{- if .Values.global.modules.rag }}
    - alert: FaithfulnessLow
      ...
    - alert: FaithfulnessCritical
      ...
    - alert: RelevancyLow
      ...
    - alert: QualityRegression
      ...
    - alert: EvalStale
      ...
{{- end }}
```

SLO alerts (TTFTSLOBreach, etc.) remain always-on — they're core model serving alerts.

### Backward Compatibility

| User scenario | Before | After |
|--------------|--------|-------|
| Custom values with `dify.enabled: true` | Works | Still works (explicit override) |
| `values-single-node.yaml` | Individual enables | Module switches |
| `values-ci.yaml` | All off | All off (modules default false) |
| `--set dify.enabled=true` | Works | Still works |
| `--set modules.rag.enabled=true` | N/A | Enables dify+milvus+lightrag+rag-eval |

### Test Updates

Update `tests/helm/test_phase5_templates.py` (or create new test file) to verify:

1. Module off → subcharts not rendered, dashboards not in ConfigMap
2. Module on → subcharts rendered, dashboards present
3. Module on + explicit disable → subchart not rendered, dashboard still present (module-level dashboard)
4. Module off + explicit enable → subchart rendered
5. Dashboard ConfigMap only contains expected dashboards per module combination

### Files Changed

| File | Change |
|------|--------|
| `Chart.yaml` | Dual-path conditions for 7 subcharts |
| `values.yaml` (parent) | Add `modules.*`, add `global.modules.*`, remove `enabled` from module-grouped components |
| `values-single-node.yaml` | Replace individual enables with module switches |
| `values-ci.yaml` | Verify modules default off |
| `charts/dify/values.yaml` | Remove `enabled:` line |
| `charts/milvus/values.yaml` | Remove `enabled:` line |
| `charts/lightrag/values.yaml` | Remove `enabled:` line |
| `charts/rag-eval/values.yaml` | Remove `enabled:` line |
| `charts/finetune/values.yaml` | Remove `enabled:` line |
| `charts/security/values.yaml` | Remove `enabled:` line |
| `charts/jupyterhub/values.yaml` | Remove `enabled:` line (if exists) |
| `templates/grafana-dashboards.yaml` | NEW: conditional dashboard ConfigMap |
| `charts/observability/templates/grafana.yaml` | Remove old dashboard ConfigMap (keep provisioner + volumes) |
| `charts/observability/templates/prometheus.yaml` | Wrap RAG alerts with `global.modules.rag` |
| `tests/helm/test_module_switches.py` | NEW: module switch tests |
| `AGENTS.md` | Document module switches |

### User Interface

```bash
# Enable RAG
helm install kube-llmops charts/kube-llmops-stack \
  --set modules.rag.enabled=true

# Enable RAG + finetune
helm install kube-llmops charts/kube-llmops-stack \
  --set modules.rag.enabled=true \
  --set modules.finetune.enabled=true

# Enable RAG but skip Milvus (use pgvector only)
helm install kube-llmops charts/kube-llmops-stack \
  --set modules.rag.enabled=true \
  --set milvus.enabled=false

# Force dify on without full RAG module
helm install kube-llmops charts/kube-llmops-stack \
  --set dify.enabled=true
```
