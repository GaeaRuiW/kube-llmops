# Phase 5: Advanced Inference — Implementation Plan

> **STATUS: COMPLETED** — Implemented in v0.5.0. All tasks executed successfully.
> See: `charts/kube-llmops-stack/charts/{litellm,keda,vllm,llamacpp,tei}/`, `tests/helm/test_phase5_templates.py` (39 tests passing), `docs/{routing,large-model-deployment,speculative-decoding,kserve-integration,disaggregated-serving,model-updates}.md`

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add intelligent routing, SLO-aware autoscaling, GPU cost optimization, canary deployments, disaggregated serving, and multi-accelerator support to the kube-llmops Helm chart.

**Architecture:** Extend existing subchart templates (litellm, keda, vllm, llamacpp, tei, observability) with new values fields and conditional template logic. All features are opt-in via values — disabled by default, zero impact on existing deployments. A single test file validates all new template features.

**Tech Stack:** Helm 3 templates (Go templating), Grafana JSON dashboards, pytest + PyYAML for Helm template tests, KEDA ScaledObject CRDs, Gateway API inference extension CRDs (llm-d).

---

## File Structure

### New Files

| File | Responsibility |
|------|---------------|
| `tests/helm/test_phase5_templates.py` | All Phase 5 Helm template tests (routing, KEDA multi-trigger, scale-to-zero, spot, MIG, canary, llm-d, accelerator) |
| `charts/.../vllm/templates/disaggregated.yaml` | llm-d Prefill/Decode Deployments + InferencePool/InferenceModel CRDs |
| `charts/.../vllm/templates/epp.yaml` | llm-d Endpoint Picker (EPP) Deployment |
| `charts/.../observability/templates/gpu-exporter.yaml` | AMD SMI / Habana GPU exporter (conditional on accelerator) |
| `docs/routing.md` | Routing strategies, prefix caching, session affinity |
| `docs/large-model-deployment.md` | Multi-GPU TP, MoE, quantization, memory estimation |
| `docs/speculative-decoding.md` | Draft model config via engineArgs |
| `docs/kserve-integration.md` | KServe coexistence guide |
| `docs/disaggregated-serving.md` | llm-d architecture, prerequisites, usage |
| `docs/model-updates.md` | Canary deployment flow, promotion steps |

### Modified Files

| File | Changes |
|------|---------|
| `charts/.../litellm/values.yaml` | Add `routingStrategy`, `routingStrategyArgs` |
| `charts/.../litellm/templates/configmap.yaml` | Render routing_strategy from values (not hardcoded), canary weight entries, fallback config |
| `charts/.../keda/values.yaml` | Add `triggers` section (ttftP95, tpotP95), `scaleToZero` per-model |
| `charts/.../keda/templates/scaledobject.yaml` | Multi-trigger rendering, `minReplicaCount: 0` + `idleReplicaCount` |
| `charts/.../vllm/values.yaml` | Document `prefixCaching`, `spotToleration`, `canary`, `disaggregated` model fields |
| `charts/.../vllm/templates/deployment.yaml` | `--enable-prefix-caching`, spot tolerations, `terminationGracePeriodSeconds`, MIG device, canary Deployment |
| `charts/.../vllm/templates/service.yaml` | Canary Service |
| `charts/.../llamacpp/templates/deployment.yaml` | Spot tolerations, `terminationGracePeriodSeconds`, accelerator resource name |
| `charts/.../tei/templates/deployment.yaml` | Accelerator resource name |
| `charts/.../templates/_helpers.tpl` | `gpuResourceName`, `gpuToleration`, `vllmDefaultImage` helpers |
| `charts/.../observability/templates/prometheus.yaml` | SLO alert rules (TTFTSLOBreach, TTFTSLOCritical) |
| `charts/.../observability/templates/dcgm-exporter.yaml` | Conditional on `global.accelerator != "nvidia"` |
| `charts/.../observability/dashboards/slo-overview.json` | TTFT/TPOT SLO panels, HPA replica count |
| `charts/.../observability/dashboards/cost-usage.json` | GPU idle rate, scale-to-zero savings |
| `charts/.../observability/dashboards/vllm-overview.json` | Prefix cache hit rate panel |
| `charts/.../observability/dashboards/litellm-gateway.json` | Canary vs primary latency/error panels |
| `ARCHITECTURE.md` | Phase 5 roadmap update |
| `AGENTS.md` | New test commands, file paths |
| `CHANGELOG.md` | v0.5.0 entry |

---

## Batch 1: Routing + Autoscaling

### Task 1: Test infrastructure + latency-based routing tests

**Files:**
- Create: `tests/helm/test_phase5_templates.py`

- [ ] **Step 1: Create test file with helpers and first test class**

```python
"""
Helm template unit tests for Phase 5: Advanced Inference features.

Usage:
    cd kube-llmops
    python -m pytest tests/helm/test_phase5_templates.py -v
"""

import subprocess
import json
import yaml
import pytest
from pathlib import Path

CHART_DIR = Path(__file__).parent.parent.parent / "charts" / "kube-llmops-stack"

# Minimal model set for template rendering
SINGLE_MODEL = {
    "global.models[0].name": "test-model",
    "global.models[0].source": "org/test-model",
    "global.models[0].resources.gpu": "1",
    "global.models[0].resources.memory": "16Gi",
}


def helm_template(set_values=None, values_files=None, show_only=None):
    """Run helm template and return parsed YAML documents."""
    cmd = ["helm", "template", "test", str(CHART_DIR)]
    for vf in (values_files or []):
        cmd += ["-f", str(vf)]
    for k, v in (set_values or {}).items():
        cmd += ["--set", f"{k}={v}"]
    if show_only:
        cmd += ["--show-only", show_only]
    result = subprocess.run(cmd, capture_output=True, text=True, timeout=60)
    if result.returncode != 0:
        raise RuntimeError(f"helm template failed: {result.stderr}")
    docs = []
    for doc in yaml.safe_load_all(result.stdout):
        if doc:
            docs.append(doc)
    return docs


def find_by_kind(docs, kind, name_contains=None):
    """Find documents of a specific kind, optionally filtering by name."""
    found = [d for d in docs if d.get("kind") == kind]
    if name_contains:
        found = [d for d in found if name_contains in d["metadata"]["name"]]
    return found


def get_configmap_data(docs, name_contains):
    """Extract parsed YAML from a ConfigMap's data field."""
    cms = find_by_kind(docs, "ConfigMap", name_contains)
    assert len(cms) >= 1, f"No ConfigMap found containing '{name_contains}'"
    raw = cms[0]["data"].get("config.yaml", "")
    return yaml.safe_load(raw)


class TestRoutingStrategy:
    """Test LiteLLM routing strategy configuration."""

    def test_default_routing_is_latency_based(self):
        docs = helm_template(set_values=SINGLE_MODEL)
        config = get_configmap_data(docs, "litellm-config")
        assert config["router_settings"]["routing_strategy"] == "latency-based-routing"

    def test_custom_routing_strategy(self):
        vals = {**SINGLE_MODEL, "litellm.routingStrategy": "simple-shuffle"}
        docs = helm_template(set_values=vals)
        config = get_configmap_data(docs, "litellm-config")
        assert config["router_settings"]["routing_strategy"] == "simple-shuffle"

    def test_routing_strategy_args_rendered(self):
        vals = {
            **SINGLE_MODEL,
            "litellm.routingStrategyArgs.ttl": "120",
            "litellm.routingStrategyArgs.lowest_latency_buffer": "0.3",
        }
        docs = helm_template(set_values=vals)
        config = get_configmap_data(docs, "litellm-config")
        rs = config["router_settings"]
        assert rs["routing_strategy_args"]["ttl"] == 120
        assert rs["routing_strategy_args"]["lowest_latency_buffer"] == 0.3
```

- [ ] **Step 2: Run test to verify it fails**

Run: `python -m pytest tests/helm/test_phase5_templates.py::TestRoutingStrategy -v`
Expected: FAIL — `routing_strategy` is still `simple-shuffle` (hardcoded in configmap.yaml line 73)

- [ ] **Step 3: Commit test file**

```bash
git add tests/helm/test_phase5_templates.py
git commit -m "test: Phase 5 routing strategy tests (red)"
```

### Task 2: Implement latency-based routing in LiteLLM

**Files:**
- Modify: `charts/kube-llmops-stack/charts/litellm/values.yaml:23-30`
- Modify: `charts/kube-llmops-stack/charts/litellm/templates/configmap.yaml:71-81`

- [ ] **Step 1: Add routing fields to litellm values.yaml**

In `charts/kube-llmops-stack/charts/litellm/values.yaml`, replace lines 23-30:

Old:
```yaml
# Router settings (load balancing, fallback, retries)
routerSettings:
  numRetries: 3
  timeout: 120
  allowedFails: 3         # Failures before marking deployment unhealthy
  cooldownTime: 60        # Seconds to cooldown unhealthy deployment
  # fallbacks:            # Model fallback chain, e.g.:
  #   - model-a: [model-b]
```

New:
```yaml
# Router settings (load balancing, fallback, retries)
routerSettings:
  numRetries: 3
  timeout: 120
  allowedFails: 3         # Failures before marking deployment unhealthy
  cooldownTime: 60        # Seconds to cooldown unhealthy deployment
  # fallbacks:            # Model fallback chain, e.g.:
  #   - model-a: [model-b]

# Routing strategy: latency-based-routing (default) or simple-shuffle
routingStrategy: latency-based-routing
# Additional args for the routing strategy
routingStrategyArgs:
  ttl: 60                          # Cache latency data for N seconds
  lowest_latency_buffer: 0.2       # 20% buffer before switching to faster model
```

- [ ] **Step 2: Update configmap.yaml to render routing_strategy from values**

In `charts/kube-llmops-stack/charts/litellm/templates/configmap.yaml`, replace lines 71-81:

Old:
```yaml
    {{- if or .Values.models (and .Values.global .Values.global.models) }}
    router_settings:
      routing_strategy: simple-shuffle
      num_retries: {{ .Values.routerSettings.numRetries | default 3 }}
      timeout: {{ .Values.routerSettings.timeout | default 120 }}
      {{- if .Values.routerSettings.fallbacks }}
      fallbacks: {{ .Values.routerSettings.fallbacks | toJson }}
      {{- end }}
      allowed_fails: {{ .Values.routerSettings.allowedFails | default 3 }}
      cooldown_time: {{ .Values.routerSettings.cooldownTime | default 60 }}
    {{- end }}
```

New:
```yaml
    {{- if or .Values.models (and .Values.global .Values.global.models) }}
    router_settings:
      routing_strategy: {{ .Values.routingStrategy | default "latency-based-routing" }}
      {{- if .Values.routingStrategyArgs }}
      routing_strategy_args:
        {{- range $key, $val := .Values.routingStrategyArgs }}
        {{ $key }}: {{ $val }}
        {{- end }}
      {{- end }}
      num_retries: {{ .Values.routerSettings.numRetries | default 3 }}
      timeout: {{ .Values.routerSettings.timeout | default 120 }}
      {{- if .Values.routerSettings.fallbacks }}
      fallbacks: {{ .Values.routerSettings.fallbacks | toJson }}
      {{- end }}
      allowed_fails: {{ .Values.routerSettings.allowedFails | default 3 }}
      cooldown_time: {{ .Values.routerSettings.cooldownTime | default 60 }}
    {{- end }}
```

- [ ] **Step 3: Run tests to verify they pass**

Run: `python -m pytest tests/helm/test_phase5_templates.py::TestRoutingStrategy -v`
Expected: 3 PASSED

- [ ] **Step 4: Commit**

```bash
git add charts/kube-llmops-stack/charts/litellm/values.yaml \
       charts/kube-llmops-stack/charts/litellm/templates/configmap.yaml
git commit -m "feat(litellm): latency-based routing (default), configurable strategy"
```

### Task 3: Prefix caching tests + implementation

**Files:**
- Modify: `tests/helm/test_phase5_templates.py`
- Modify: `charts/kube-llmops-stack/charts/vllm/templates/deployment.yaml:197-208`

- [ ] **Step 1: Add prefix caching tests to test file**

Append to `tests/helm/test_phase5_templates.py`:

```python
class TestPrefixCaching:
    """Test vLLM prefix caching flag."""

    def test_prefix_caching_disabled_by_default(self):
        docs = helm_template(
            set_values=SINGLE_MODEL,
            show_only="charts/vllm/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment")
        assert len(deps) == 1
        container_args = deps[0]["spec"]["template"]["spec"]["containers"][0]["args"][0]
        assert "--enable-prefix-caching" not in container_args

    def test_prefix_caching_enabled(self):
        vals = {**SINGLE_MODEL, "global.models[0].prefixCaching": "true"}
        docs = helm_template(
            set_values=vals,
            show_only="charts/vllm/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment")
        assert len(deps) == 1
        container_args = deps[0]["spec"]["template"]["spec"]["containers"][0]["args"][0]
        assert "--enable-prefix-caching" in container_args
```

- [ ] **Step 2: Run test to verify it fails**

Run: `python -m pytest tests/helm/test_phase5_templates.py::TestPrefixCaching::test_prefix_caching_enabled -v`
Expected: FAIL — `--enable-prefix-caching` not found in args

- [ ] **Step 3: Add prefix caching flag to vllm deployment.yaml**

In `charts/kube-llmops-stack/charts/vllm/templates/deployment.yaml`, find lines 198-208 (the `exec vllm serve` block). Insert the prefix caching flag after the `--served-model-name` line:

Old:
```yaml
              exec vllm serve "$MODEL_PATH" \
                --host 0.0.0.0 \
                --port 8000 \
                --served-model-name {{ $model.name | quote }} \
                {{- if gt (int $gpu) 1 }}
                --tensor-parallel-size {{ $gpu | quote }} \
                {{- end }}
                {{- range $key, $val := $model.engineArgs }}
```

New:
```yaml
              exec vllm serve "$MODEL_PATH" \
                --host 0.0.0.0 \
                --port 8000 \
                --served-model-name {{ $model.name | quote }} \
                {{- if $model.prefixCaching }}
                --enable-prefix-caching \
                {{- end }}
                {{- if gt (int $gpu) 1 }}
                --tensor-parallel-size {{ $gpu | quote }} \
                {{- end }}
                {{- range $key, $val := $model.engineArgs }}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `python -m pytest tests/helm/test_phase5_templates.py::TestPrefixCaching -v`
Expected: 2 PASSED

- [ ] **Step 5: Commit**

```bash
git add tests/helm/test_phase5_templates.py \
       charts/kube-llmops-stack/charts/vllm/templates/deployment.yaml
git commit -m "feat(vllm): prefix caching flag per model"
```

### Task 4: Multi-trigger KEDA tests

**Files:**
- Modify: `tests/helm/test_phase5_templates.py`

- [ ] **Step 1: Add KEDA multi-trigger tests**

Append to `tests/helm/test_phase5_templates.py`:

```python
KEDA_BASE = {
    **SINGLE_MODEL,
    "keda.enabled": "true",
}


class TestKedaMultiTrigger:
    """Test KEDA ScaledObject multi-trigger configuration."""

    def test_single_trigger_default(self):
        """Default: only requestsWaiting trigger."""
        docs = helm_template(
            set_values=KEDA_BASE,
            show_only="charts/keda/templates/scaledobject.yaml",
        )
        sos = find_by_kind(docs, "ScaledObject")
        assert len(sos) == 1
        triggers = sos[0]["spec"]["triggers"]
        assert len(triggers) == 1
        assert "num_requests_waiting" in triggers[0]["metadata"]["query"]

    def test_ttft_trigger_added(self):
        """Enable TTFT P95 trigger — should produce 2 triggers."""
        vals = {**KEDA_BASE, "keda.triggers.ttftP95.enabled": "true"}
        docs = helm_template(
            set_values=vals,
            show_only="charts/keda/templates/scaledobject.yaml",
        )
        sos = find_by_kind(docs, "ScaledObject")
        assert len(sos) == 1
        triggers = sos[0]["spec"]["triggers"]
        assert len(triggers) == 2
        queries = [t["metadata"]["query"] for t in triggers]
        assert any("num_requests_waiting" in q for q in queries)
        assert any("time_to_first_token" in q for q in queries)

    def test_all_three_triggers(self):
        """Enable all 3 triggers."""
        vals = {
            **KEDA_BASE,
            "keda.triggers.ttftP95.enabled": "true",
            "keda.triggers.tpotP95.enabled": "true",
        }
        docs = helm_template(
            set_values=vals,
            show_only="charts/keda/templates/scaledobject.yaml",
        )
        sos = find_by_kind(docs, "ScaledObject")
        triggers = sos[0]["spec"]["triggers"]
        assert len(triggers) == 3
        queries = [t["metadata"]["query"] for t in triggers]
        assert any("time_per_output_token" in q for q in queries)

    def test_per_model_ttft_threshold_override(self):
        """Per-model override for TTFT threshold."""
        vals = {
            **KEDA_BASE,
            "keda.triggers.ttftP95.enabled": "true",
            "keda.triggers.ttftP95.threshold": "3",
            "keda.models.test-model.triggers.ttftP95.threshold": "1.5",
        }
        docs = helm_template(
            set_values=vals,
            show_only="charts/keda/templates/scaledobject.yaml",
        )
        sos = find_by_kind(docs, "ScaledObject")
        triggers = sos[0]["spec"]["triggers"]
        ttft_trigger = [t for t in triggers if "time_to_first_token" in t["metadata"]["query"]][0]
        assert ttft_trigger["metadata"]["threshold"] == "1.5"
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `python -m pytest tests/helm/test_phase5_templates.py::TestKedaMultiTrigger -v`
Expected: All 4 FAIL — template only renders 1 trigger, no ttftP95/tpotP95 support

- [ ] **Step 3: Commit red tests**

```bash
git add tests/helm/test_phase5_templates.py
git commit -m "test: KEDA multi-trigger tests (red)"
```

### Task 5: Implement multi-trigger KEDA ScaledObject

**Files:**
- Modify: `charts/kube-llmops-stack/charts/keda/values.yaml`
- Modify: `charts/kube-llmops-stack/charts/keda/templates/scaledobject.yaml`

- [ ] **Step 1: Add trigger configuration to keda/values.yaml**

Replace the entire content of `charts/kube-llmops-stack/charts/keda/values.yaml`:

```yaml
enabled: false

# Prerequisite: KEDA operator must be installed separately
#   helm repo add kedacore https://kedacore.github.io/charts
#   helm install keda kedacore/keda -n keda-system --create-namespace

# Default autoscaling settings for all models
defaults:
  minReplicas: 1
  maxReplicas: 4
  cooldownPeriod: 300       # 5 min before scale-down
  pollingInterval: 15       # Check metrics every 15s

  # Prometheus endpoint (auto-detected from observability chart)
  prometheusAddress: ""     # Auto: http://<release>-prometheus:9090

# Per-engine pending-requests trigger configuration
# Models are auto-detected from global.models via resolveEngine helper
engines:
  vllm:
    metric: "vllm:num_requests_waiting"
    labelName: model_name           # label key used in the Prometheus metric
    threshold: "2"                  # scale when > 2 requests waiting
  llamacpp:
    metric: "llamacpp_requests_processing"
    labelName: model                # label key (relabeled from kube-llmops/model)
    threshold: "2"

# Multi-trigger configuration (any trigger breaching threshold causes scale-up)
triggers:
  requestsWaiting:
    enabled: true                   # Default: always enabled (queue-depth trigger)
  ttftP95:
    enabled: false                  # Time to First Token P95 (vLLM only)
    threshold: "3"                  # seconds
    window: "2m"
  tpotP95:
    enabled: false                  # Time per Output Token P95 (vLLM only)
    threshold: "0.1"               # seconds per token
    window: "2m"

# Per-model overrides (optional)
# models:
#   my-model-name:
#     minReplicas: 2
#     maxReplicas: 8
#     threshold: "5"
#     triggers:
#       ttftP95:
#         threshold: "1.5"
#     scaleToZero:
#       enabled: true
#       idleTimeout: 900
#       fallbackModel: "other-model"
```

- [ ] **Step 2: Rewrite keda/templates/scaledobject.yaml for multi-trigger**

Replace the entire content of `charts/kube-llmops-stack/charts/keda/templates/scaledobject.yaml`:

```yaml
{{- if .Values.enabled }}
{{- $promAddr := .Values.defaults.prometheusAddress | default (printf "http://%s-prometheus.%s.svc.cluster.local:9090" .Release.Name .Release.Namespace) }}

{{/* Collect models from global.models */}}
{{- $models := list }}
{{- if and .Values.global .Values.global.models }}
  {{- $models = .Values.global.models }}
{{- end }}

{{- range $idx, $model := $models }}
{{- $engine := include "kube-llmops.resolveEngine" $model | trim }}
{{- if or (eq $engine "vllm") (eq $engine "llamacpp") }}

{{/* Per-engine config */}}
{{- $engineCfg := index $.Values.engines $engine | default dict }}
{{- $metric    := $engineCfg.metric    | default "unknown" }}
{{- $labelName := $engineCfg.labelName | default "model_name" }}
{{- $threshold := $engineCfg.threshold | default "2" }}

{{/* Per-model overrides */}}
{{- $override    := dict }}
{{- if $.Values.models }}
{{-   $override = index $.Values.models $model.name | default dict }}
{{- end }}
{{- $minReplicas := $override.minReplicas | default $.Values.defaults.minReplicas }}
{{- $maxReplicas := $override.maxReplicas | default $.Values.defaults.maxReplicas }}
{{- $threshold    = $override.threshold   | default $threshold }}

{{/* Scale-to-zero config */}}
{{- $s2z := default dict $override.scaleToZero }}
{{- $s2zEnabled := $s2z.enabled | default false }}
{{- $idleTimeout := $s2z.idleTimeout | default 900 }}
{{- if $s2zEnabled }}
{{-   $minReplicas = 0 }}
{{- end }}

{{/* Per-model trigger overrides */}}
{{- $overrideTriggers := default dict $override.triggers }}

{{/* Resolve trigger thresholds (per-model override > global) */}}
{{- $globalTriggers := $.Values.triggers }}
{{- $ttftThreshold := ($globalTriggers.ttftP95).threshold | default "3" }}
{{- $ttftWindow := ($globalTriggers.ttftP95).window | default "2m" }}
{{- $tpotThreshold := ($globalTriggers.tpotP95).threshold | default "0.1" }}
{{- $tpotWindow := ($globalTriggers.tpotP95).window | default "2m" }}
{{- if $overrideTriggers.ttftP95 }}
{{-   $ttftThreshold = $overrideTriggers.ttftP95.threshold | default $ttftThreshold }}
{{- end }}
{{- if $overrideTriggers.tpotP95 }}
{{-   $tpotThreshold = $overrideTriggers.tpotP95.threshold | default $tpotThreshold }}
{{- end }}

---
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: {{ $engine }}-{{ $model.name }}-scaler
  labels:
    app.kubernetes.io/name: keda-scaler
    app.kubernetes.io/instance: {{ $.Release.Name }}
    app.kubernetes.io/part-of: kube-llmops
    kube-llmops/model: {{ $model.name }}
    kube-llmops/engine: {{ $engine }}
spec:
  scaleTargetRef:
    name: {{ $engine }}-{{ $model.name }}
  minReplicaCount: {{ $minReplicas }}
  maxReplicaCount: {{ $maxReplicas }}
  cooldownPeriod:  {{ $.Values.defaults.cooldownPeriod }}
  pollingInterval: {{ $.Values.defaults.pollingInterval }}
  {{- if $s2zEnabled }}
  idleReplicaCount: 0
  advanced:
    horizontalPodAutoscalerConfig:
      behavior:
        scaleDown:
          stabilizationWindowSeconds: {{ $idleTimeout }}
  {{- end }}
  triggers:
    {{/* Trigger 1: Queue depth (always enabled unless explicitly disabled) */}}
    {{- if (($globalTriggers.requestsWaiting).enabled | default true) }}
    - type: prometheus
      metadata:
        serverAddress: {{ $promAddr }}
        query: sum({{ $metric }}{{"{"}}{{ $labelName }}="{{ $model.name }}"{{"}"}} ) or vector(0)
        threshold: {{ $threshold | quote }}
        metricName: pending_requests_{{ $model.name }}
      {{- if $s2zEnabled }}
        activationThreshold: "1"
      {{- end }}
    {{- end }}
    {{/* Trigger 2: TTFT P95 (vLLM only) */}}
    {{- if and (eq $engine "vllm") (($globalTriggers.ttftP95).enabled | default false) }}
    - type: prometheus
      metadata:
        serverAddress: {{ $promAddr }}
        query: histogram_quantile(0.95, rate(vllm:time_to_first_token_seconds_bucket{{"{"}}model_name="{{ $model.name }}"{{"}"}[{{ $ttftWindow }}])) or vector(0)
        threshold: {{ $ttftThreshold | quote }}
        metricName: ttft_p95_{{ $model.name }}
    {{- end }}
    {{/* Trigger 3: TPOT P95 (vLLM only) */}}
    {{- if and (eq $engine "vllm") (($globalTriggers.tpotP95).enabled | default false) }}
    - type: prometheus
      metadata:
        serverAddress: {{ $promAddr }}
        query: histogram_quantile(0.95, rate(vllm:time_per_output_token_seconds_bucket{{"{"}}model_name="{{ $model.name }}"{{"}"}[{{ $tpotWindow }}])) or vector(0)
        threshold: {{ $tpotThreshold | quote }}
        metricName: tpot_p95_{{ $model.name }}
    {{- end }}
{{- end }}
{{- end }}
{{- end }}
```

- [ ] **Step 3: Rebuild chart archives**

```bash
cd charts/kube-llmops-stack && rm -f charts/*.tgz Chart.lock && helm dependency update . && cd ../..
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `python -m pytest tests/helm/test_phase5_templates.py::TestKedaMultiTrigger -v`
Expected: 4 PASSED

- [ ] **Step 5: Run existing finetune tests for regression**

Run: `python -m pytest tests/helm/test_finetune_templates.py -v`
Expected: All 35+ PASSED (no regression)

- [ ] **Step 6: Commit**

```bash
git add charts/kube-llmops-stack/charts/keda/
git commit -m "feat(keda): multi-trigger ScaledObject (queue + TTFT P95 + TPOT P95)"
```

### Task 6: SLO alert rules

**Files:**
- Modify: `tests/helm/test_phase5_templates.py`
- Modify: `charts/kube-llmops-stack/charts/observability/templates/prometheus.yaml:147-188`

- [ ] **Step 1: Add SLO alert tests**

Append to `tests/helm/test_phase5_templates.py`:

```python
class TestSLOAlerts:
    """Test SLO alert rules in Prometheus config."""

    def test_ttft_slo_alerts_exist(self):
        docs = helm_template(
            set_values=SINGLE_MODEL,
            show_only="charts/observability/templates/prometheus.yaml",
        )
        cms = find_by_kind(docs, "ConfigMap", "prometheus-config")
        assert len(cms) >= 1
        rules_raw = cms[0]["data"]["rules.yml"]
        rules = yaml.safe_load(rules_raw)
        all_alert_names = []
        for group in rules["groups"]:
            for rule in group.get("rules", []):
                if "alert" in rule:
                    all_alert_names.append(rule["alert"])
        assert "TTFTSLOBreach" in all_alert_names
        assert "TTFTSLOCritical" in all_alert_names
```

- [ ] **Step 2: Run test to verify it fails**

Run: `python -m pytest tests/helm/test_phase5_templates.py::TestSLOAlerts -v`
Expected: FAIL — alerts not found

- [ ] **Step 3: Add SLO alerts to prometheus.yaml**

In `charts/kube-llmops-stack/charts/observability/templates/prometheus.yaml`, after the `rag-quality-alerts` group (after line 188, before the `---` separator), insert a new alert group:

Old (end of rules.yml section):
```yaml
          - alert: RAGEvalStale
            expr: time() - ragas_evaluation_timestamp > 172800
            for: 0m
            labels:
              severity: warning
            annotations:
              summary: "RAG evaluation data is stale (>48h)"
              description: "Last Ragas evaluation was more than 48 hours ago"
```

New (append after RAGEvalStale):
```yaml
          - alert: RAGEvalStale
            expr: time() - ragas_evaluation_timestamp > 172800
            for: 0m
            labels:
              severity: warning
            annotations:
              summary: "RAG evaluation data is stale (>48h)"
              description: "Last Ragas evaluation was more than 48 hours ago"
      - name: slo-alerts
        rules:
          - alert: TTFTSLOBreach
            expr: histogram_quantile(0.95, sum(rate(vllm:time_to_first_token_seconds_bucket[5m])) by (le, model_name)) > 3
            for: 2m
            labels:
              severity: warning
            annotations:
              summary: "P95 TTFT exceeds SLO threshold (3s)"
              description: "Model {{`{{ $labels.model_name }}`}} P95 TTFT is {{`{{ $value }}`}}s"
          - alert: TTFTSLOCritical
            expr: histogram_quantile(0.95, sum(rate(vllm:time_to_first_token_seconds_bucket[5m])) by (le, model_name)) > 5
            for: 1m
            labels:
              severity: critical
            annotations:
              summary: "P95 TTFT critically high (>5s)"
              description: "Model {{`{{ $labels.model_name }}`}} P95 TTFT is {{`{{ $value }}`}}s"
          - alert: TPOTSLOBreach
            expr: histogram_quantile(0.95, sum(rate(vllm:time_per_output_token_seconds_bucket[5m])) by (le, model_name)) > 0.15
            for: 2m
            labels:
              severity: warning
            annotations:
              summary: "P95 TPOT exceeds SLO threshold (150ms/token)"
              description: "Model {{`{{ $labels.model_name }}`}} P95 TPOT is {{`{{ $value }}`}}s/token"
```

- [ ] **Step 4: Rebuild chart archives and run test**

```bash
cd charts/kube-llmops-stack && rm -f charts/*.tgz Chart.lock && helm dependency update . && cd ../..
python -m pytest tests/helm/test_phase5_templates.py::TestSLOAlerts -v
```
Expected: PASSED

- [ ] **Step 5: Commit**

```bash
git add charts/kube-llmops-stack/charts/observability/templates/prometheus.yaml \
       tests/helm/test_phase5_templates.py
git commit -m "feat(observability): SLO alert rules (TTFT/TPOT breach + critical)"
```

### Task 7: SLO + prefix cache dashboard panels

**Files:**
- Modify: `charts/kube-llmops-stack/charts/observability/dashboards/slo-overview.json`
- Modify: `charts/kube-llmops-stack/charts/observability/dashboards/vllm-overview.json`

- [ ] **Step 1: Add SLO dashboard panels**

Read `slo-overview.json`, parse it as JSON, and add 3 new panels to the `panels` array after the existing 4 panels. The new panels:

Panel 5 — TTFT P95 vs SLO (timeseries, gridPos: h=8, w=12, x=0, y=4):
```json
{
  "id": 5,
  "title": "TTFT P95 vs SLO Threshold",
  "type": "timeseries",
  "gridPos": {"h": 8, "w": 12, "x": 0, "y": 4},
  "datasource": {"type": "prometheus", "uid": "prometheus"},
  "targets": [
    {
      "expr": "histogram_quantile(0.95, sum(rate(vllm:time_to_first_token_seconds_bucket[5m])) by (le, model_name))",
      "legendFormat": "{{model_name}} P95 TTFT"
    },
    {
      "expr": "vector(3)",
      "legendFormat": "SLO Threshold (3s)"
    }
  ],
  "fieldConfig": {
    "defaults": {
      "unit": "s",
      "custom": {"lineWidth": 2, "fillOpacity": 10}
    },
    "overrides": [
      {
        "matcher": {"id": "byName", "options": "SLO Threshold (3s)"},
        "properties": [
          {"id": "custom.lineStyle", "value": {"fill": "dash", "dash": [10, 10]}},
          {"id": "color", "value": {"fixedColor": "red", "mode": "fixed"}}
        ]
      }
    ]
  }
}
```

Panel 6 — TPOT P95 vs SLO (timeseries, gridPos: h=8, w=12, x=12, y=4):
```json
{
  "id": 6,
  "title": "TPOT P95 vs SLO Threshold",
  "type": "timeseries",
  "gridPos": {"h": 8, "w": 12, "x": 12, "y": 4},
  "datasource": {"type": "prometheus", "uid": "prometheus"},
  "targets": [
    {
      "expr": "histogram_quantile(0.95, sum(rate(vllm:time_per_output_token_seconds_bucket[5m])) by (le, model_name))",
      "legendFormat": "{{model_name}} P95 TPOT"
    },
    {
      "expr": "vector(0.1)",
      "legendFormat": "SLO Threshold (100ms)"
    }
  ],
  "fieldConfig": {
    "defaults": {
      "unit": "s",
      "custom": {"lineWidth": 2, "fillOpacity": 10}
    },
    "overrides": [
      {
        "matcher": {"id": "byName", "options": "SLO Threshold (100ms)"},
        "properties": [
          {"id": "custom.lineStyle", "value": {"fill": "dash", "dash": [10, 10]}},
          {"id": "color", "value": {"fixedColor": "red", "mode": "fixed"}}
        ]
      }
    ]
  }
}
```

Panel 7 — HPA Replica Count (timeseries, gridPos: h=8, w=24, x=0, y=12):
```json
{
  "id": 7,
  "title": "HPA Replica Count by Model",
  "type": "timeseries",
  "gridPos": {"h": 8, "w": 24, "x": 0, "y": 12},
  "datasource": {"type": "prometheus", "uid": "prometheus"},
  "targets": [
    {
      "expr": "kube_horizontalpodautoscaler_status_current_replicas{horizontalpodautoscaler=~\".*-scaler\"}",
      "legendFormat": "{{horizontalpodautoscaler}} current"
    },
    {
      "expr": "kube_horizontalpodautoscaler_status_desired_replicas{horizontalpodautoscaler=~\".*-scaler\"}",
      "legendFormat": "{{horizontalpodautoscaler}} desired"
    }
  ],
  "fieldConfig": {
    "defaults": {
      "unit": "short",
      "custom": {"lineWidth": 2, "fillOpacity": 5}
    }
  }
}
```

Write the updated JSON back to `slo-overview.json`.

- [ ] **Step 2: Add prefix cache panel to vllm-overview.json**

Read `vllm-overview.json`, add 1 new panel after the existing panels:

Panel 9 — Prefix Cache Hit Rate (timeseries, gridPos: h=8, w=24, x=0, y=24):
```json
{
  "id": 9,
  "title": "Prefix Cache Hit Rate",
  "type": "timeseries",
  "gridPos": {"h": 8, "w": 24, "x": 0, "y": 24},
  "datasource": {"type": "prometheus", "uid": "prometheus"},
  "targets": [
    {
      "expr": "vllm:prefix_cache_hit_rate{model_name=~\"$model\"}",
      "legendFormat": "{{model_name}} Hit Rate"
    },
    {
      "expr": "rate(vllm:prefix_cache_queries_total{model_name=~\"$model\"}[5m])",
      "legendFormat": "{{model_name}} Queries/s"
    }
  ],
  "fieldConfig": {
    "defaults": {
      "unit": "percentunit",
      "custom": {"lineWidth": 2, "fillOpacity": 10}
    }
  }
}
```

- [ ] **Step 3: Rebuild chart archives**

```bash
cd charts/kube-llmops-stack && rm -f charts/*.tgz Chart.lock && helm dependency update . && cd ../..
```

- [ ] **Step 4: Validate dashboards are valid JSON**

```bash
python3 -c "
import json, glob
for f in glob.glob('charts/kube-llmops-stack/charts/observability/dashboards/*.json'):
    json.load(open(f))
    print(f'OK: {f}')
"
```
Expected: All 11 files print OK

- [ ] **Step 5: Commit**

```bash
git add charts/kube-llmops-stack/charts/observability/dashboards/slo-overview.json \
       charts/kube-llmops-stack/charts/observability/dashboards/vllm-overview.json
git commit -m "feat(dashboards): SLO TTFT/TPOT panels, HPA replica count, prefix cache hit rate"
```

### Task 8: Batch 1 verification

- [ ] **Step 1: Run all Phase 5 tests**

```bash
python -m pytest tests/helm/test_phase5_templates.py -v
```
Expected: All tests PASS

- [ ] **Step 2: Run existing tests for regression**

```bash
python -m pytest tests/helm/test_finetune_templates.py -v
```
Expected: All 35+ PASS

- [ ] **Step 3: Full helm template render**

```bash
helm template test charts/kube-llmops-stack -f charts/kube-llmops-stack/values-single-node.yaml | grep -c '^kind:'
```
Expected: ~150+ resources render without errors

---

## Batch 2: Cost Optimization

### Task 9: Scale-to-zero tests

**Files:**
- Modify: `tests/helm/test_phase5_templates.py`

- [ ] **Step 1: Add scale-to-zero tests**

Append to `tests/helm/test_phase5_templates.py`:

```python
class TestScaleToZero:
    """Test KEDA scale-to-zero configuration."""

    def test_scale_to_zero_disabled_by_default(self):
        docs = helm_template(
            set_values=KEDA_BASE,
            show_only="charts/keda/templates/scaledobject.yaml",
        )
        sos = find_by_kind(docs, "ScaledObject")
        assert sos[0]["spec"]["minReplicaCount"] == 1
        assert "idleReplicaCount" not in sos[0]["spec"]

    def test_scale_to_zero_enabled(self):
        vals = {
            **KEDA_BASE,
            "keda.models.test-model.scaleToZero.enabled": "true",
            "keda.models.test-model.scaleToZero.idleTimeout": "600",
        }
        docs = helm_template(
            set_values=vals,
            show_only="charts/keda/templates/scaledobject.yaml",
        )
        sos = find_by_kind(docs, "ScaledObject")
        assert sos[0]["spec"]["minReplicaCount"] == 0
        assert sos[0]["spec"]["idleReplicaCount"] == 0
        assert sos[0]["spec"]["advanced"]["horizontalPodAutoscalerConfig"]["behavior"]["scaleDown"]["stabilizationWindowSeconds"] == 600

    def test_scale_to_zero_activation_threshold(self):
        vals = {
            **KEDA_BASE,
            "keda.models.test-model.scaleToZero.enabled": "true",
        }
        docs = helm_template(
            set_values=vals,
            show_only="charts/keda/templates/scaledobject.yaml",
        )
        sos = find_by_kind(docs, "ScaledObject")
        triggers = sos[0]["spec"]["triggers"]
        queue_trigger = triggers[0]
        assert queue_trigger["metadata"]["activationThreshold"] == "1"
```

- [ ] **Step 2: Run tests to verify they pass (already implemented in Task 5)**

Run: `python -m pytest tests/helm/test_phase5_templates.py::TestScaleToZero -v`
Expected: All 3 PASSED (scale-to-zero logic was included in the scaledobject.yaml rewrite in Task 5)

- [ ] **Step 3: Commit tests**

```bash
git add tests/helm/test_phase5_templates.py
git commit -m "test: scale-to-zero KEDA tests (green)"
```

### Task 10: LiteLLM fallback for scale-to-zero cold start

**Files:**
- Modify: `tests/helm/test_phase5_templates.py`
- Modify: `charts/kube-llmops-stack/charts/litellm/templates/configmap.yaml:37-42`

- [ ] **Step 1: Add fallback tests**

Append to `tests/helm/test_phase5_templates.py`:

```python
TWO_MODELS = {
    "global.models[0].name": "small-model",
    "global.models[0].source": "org/small-model",
    "global.models[0].resources.gpu": "1",
    "global.models[1].name": "big-model",
    "global.models[1].source": "org/big-model",
    "global.models[1].resources.gpu": "2",
}


class TestScaleToZeroFallback:
    """Test LiteLLM fallback config for scale-to-zero models."""

    def test_no_fallback_by_default(self):
        docs = helm_template(set_values=TWO_MODELS)
        config = get_configmap_data(docs, "litellm-config")
        for entry in config["model_list"]:
            assert "metadata" not in entry.get("model_info", {}) or \
                   "fallbacks" not in entry.get("model_info", {}).get("metadata", {})

    def test_fallback_rendered_when_set(self):
        vals = {
            **TWO_MODELS,
            "global.models[0].scaleToZero.fallbackModel": "big-model",
        }
        docs = helm_template(set_values=vals)
        config = get_configmap_data(docs, "litellm-config")
        small_entry = [e for e in config["model_list"] if e["model_name"] == "small-model"][0]
        assert small_entry["model_info"]["metadata"]["fallbacks"] == ["big-model"]
```

- [ ] **Step 2: Run test to verify it fails**

Run: `python -m pytest tests/helm/test_phase5_templates.py::TestScaleToZeroFallback::test_fallback_rendered_when_set -v`
Expected: FAIL — no `model_info` rendered in configmap

- [ ] **Step 3: Add fallback rendering to litellm configmap**

In `charts/kube-llmops-stack/charts/litellm/templates/configmap.yaml`, modify the global models loop (lines 37-42). After the `api_key` line, add conditional `model_info`:

Old:
```yaml
      - model_name: {{ $model.name }}
        litellm_params:
          model: {{ $litellmPrefix }}/{{ $model.name }}
          api_base: http://{{ $prefix }}-{{ $model.name }}.{{ $.Release.Namespace }}.svc.cluster.local:{{ $port }}{{ $apiSuffix }}
          api_key: "no-key-needed"
    {{- end }}
```

New:
```yaml
      - model_name: {{ $model.name }}
        litellm_params:
          model: {{ $litellmPrefix }}/{{ $model.name }}
          api_base: http://{{ $prefix }}-{{ $model.name }}.{{ $.Release.Namespace }}.svc.cluster.local:{{ $port }}{{ $apiSuffix }}
          api_key: "no-key-needed"
        {{- if and $model.scaleToZero $model.scaleToZero.fallbackModel }}
        model_info:
          metadata:
            fallbacks:
              - {{ $model.scaleToZero.fallbackModel }}
        {{- end }}
    {{- end }}
```

- [ ] **Step 4: Rebuild and run tests**

```bash
cd charts/kube-llmops-stack && rm -f charts/*.tgz Chart.lock && helm dependency update . && cd ../..
python -m pytest tests/helm/test_phase5_templates.py::TestScaleToZeroFallback -v
```
Expected: 2 PASSED

- [ ] **Step 5: Commit**

```bash
git add charts/kube-llmops-stack/charts/litellm/templates/configmap.yaml \
       tests/helm/test_phase5_templates.py
git commit -m "feat(litellm): fallback model config for scale-to-zero cold start"
```

### Task 11: Spot tolerations + graceful drain tests + implementation

**Files:**
- Modify: `tests/helm/test_phase5_templates.py`
- Modify: `charts/kube-llmops-stack/charts/vllm/templates/deployment.yaml:271-276`
- Modify: `charts/kube-llmops-stack/charts/llamacpp/templates/deployment.yaml:136-141`

- [ ] **Step 1: Add spot toleration and graceful drain tests**

Append to `tests/helm/test_phase5_templates.py`:

```python
class TestSpotToleration:
    """Test spot/preemptible GPU tolerations."""

    def test_no_spot_tolerations_by_default(self):
        docs = helm_template(
            set_values=SINGLE_MODEL,
            show_only="charts/vllm/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment")
        tolerations = deps[0]["spec"]["template"]["spec"].get("tolerations", [])
        toleration_keys = [t["key"] for t in tolerations]
        assert "karpenter.sh/capacity-type" not in toleration_keys

    def test_spot_tolerations_added(self):
        vals = {**SINGLE_MODEL, "global.models[0].spotToleration": "true"}
        docs = helm_template(
            set_values=vals,
            show_only="charts/vllm/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment")
        tolerations = deps[0]["spec"]["template"]["spec"]["tolerations"]
        toleration_keys = [t["key"] for t in tolerations]
        # Must include cloud-agnostic spot tolerations
        assert "kubernetes.azure.com/scalesetpriority" in toleration_keys
        assert "cloud.google.com/gke-spot" in toleration_keys
        assert "karpenter.sh/capacity-type" in toleration_keys


class TestGracefulDrain:
    """Test terminationGracePeriodSeconds and preStop hook."""

    def test_termination_grace_period_set(self):
        docs = helm_template(
            set_values=SINGLE_MODEL,
            show_only="charts/vllm/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment")
        spec = deps[0]["spec"]["template"]["spec"]
        assert spec["terminationGracePeriodSeconds"] == 90

    def test_prestop_hook_exists(self):
        docs = helm_template(
            set_values=SINGLE_MODEL,
            show_only="charts/vllm/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment")
        container = deps[0]["spec"]["template"]["spec"]["containers"][0]
        prestop = container["lifecycle"]["preStop"]["exec"]["command"]
        assert "sleep" in " ".join(prestop)
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `python -m pytest tests/helm/test_phase5_templates.py::TestSpotToleration -v`
Expected: FAIL — no spot tolerations rendered

Run: `python -m pytest tests/helm/test_phase5_templates.py::TestGracefulDrain -v`
Expected: FAIL — no `terminationGracePeriodSeconds` or lifecycle hook

- [ ] **Step 3: Add spot tolerations + graceful drain to vllm/templates/deployment.yaml**

In `charts/kube-llmops-stack/charts/vllm/templates/deployment.yaml`:

**Add terminationGracePeriodSeconds** — after `enableServiceLinks: false` (line 49):

Old:
```yaml
      enableServiceLinks: false
      initContainers:
```

New:
```yaml
      enableServiceLinks: false
      terminationGracePeriodSeconds: 90
      initContainers:
```

**Add lifecycle to the vllm container** — after the last `volumeMounts` entry (before the closing of the container, around line 270):

Old:
```yaml
            {{- if $.Values.customCACert.enabled }}
            - name: ca-bundle
              mountPath: /ca-out
              readOnly: true
            {{- end }}
      {{- if gt (int $gpu) 0 }}
```

New:
```yaml
            {{- if $.Values.customCACert.enabled }}
            - name: ca-bundle
              mountPath: /ca-out
              readOnly: true
            {{- end }}
          lifecycle:
            preStop:
              exec:
                command: ["/bin/sh", "-c", "sleep 5"]
      {{- if gt (int $gpu) 0 }}
```

**Add spot tolerations** — modify the tolerations block (lines 271-276):

Old:
```yaml
      {{- if gt (int $gpu) 0 }}
      tolerations:
        - key: {{ $.Values.gpuToleration.key }}
          operator: {{ $.Values.gpuToleration.operator }}
          effect: {{ $.Values.gpuToleration.effect }}
      {{- end }}
```

New:
```yaml
      {{- if gt (int $gpu) 0 }}
      tolerations:
        - key: {{ $.Values.gpuToleration.key }}
          operator: {{ $.Values.gpuToleration.operator }}
          effect: {{ $.Values.gpuToleration.effect }}
        {{- if $model.spotToleration }}
        - key: kubernetes.azure.com/scalesetpriority
          value: spot
          effect: NoSchedule
        - key: cloud.google.com/gke-spot
          value: "true"
          effect: NoSchedule
        - key: karpenter.sh/capacity-type
          value: spot
          effect: NoSchedule
        {{- end }}
      {{- end }}
```

- [ ] **Step 4: Apply same changes to llamacpp deployment**

In `charts/kube-llmops-stack/charts/llamacpp/templates/deployment.yaml`:

**Add terminationGracePeriodSeconds** — in the pod spec, before `initContainers:` (around line 42):

Old:
```yaml
    spec:
      initContainers:
```

New:
```yaml
    spec:
      terminationGracePeriodSeconds: 90
      initContainers:
```

**Add lifecycle hook** — after the last `volumeMounts` in the container (around line 135):

Old:
```yaml
          volumeMounts:
            - name: model-cache
              mountPath: /models
      {{- if gt $gpu 0 }}
      tolerations:
```

New:
```yaml
          volumeMounts:
            - name: model-cache
              mountPath: /models
          lifecycle:
            preStop:
              exec:
                command: ["/bin/sh", "-c", "sleep 5"]
      {{- if gt $gpu 0 }}
      tolerations:
```

**Add spot tolerations** (lines 136-141):

Old:
```yaml
      {{- if gt $gpu 0 }}
      tolerations:
        - key: {{ $.Values.gpuToleration.key }}
          operator: {{ $.Values.gpuToleration.operator }}
          effect: {{ $.Values.gpuToleration.effect }}
      {{- end }}
```

New:
```yaml
      {{- if gt $gpu 0 }}
      tolerations:
        - key: {{ $.Values.gpuToleration.key }}
          operator: {{ $.Values.gpuToleration.operator }}
          effect: {{ $.Values.gpuToleration.effect }}
        {{- if $model.spotToleration }}
        - key: kubernetes.azure.com/scalesetpriority
          value: spot
          effect: NoSchedule
        - key: cloud.google.com/gke-spot
          value: "true"
          effect: NoSchedule
        - key: karpenter.sh/capacity-type
          value: spot
          effect: NoSchedule
        {{- end }}
      {{- end }}
```

- [ ] **Step 5: Rebuild and run tests**

```bash
cd charts/kube-llmops-stack && rm -f charts/*.tgz Chart.lock && helm dependency update . && cd ../..
python -m pytest tests/helm/test_phase5_templates.py::TestSpotToleration tests/helm/test_phase5_templates.py::TestGracefulDrain -v
```
Expected: 4 PASSED

- [ ] **Step 6: Commit**

```bash
git add charts/kube-llmops-stack/charts/vllm/templates/deployment.yaml \
       charts/kube-llmops-stack/charts/llamacpp/templates/deployment.yaml \
       tests/helm/test_phase5_templates.py
git commit -m "feat(engines): spot tolerations + graceful drain (90s + preStop sleep)"
```

### Task 12: MIG GPU sharing tests + implementation

**Files:**
- Modify: `tests/helm/test_phase5_templates.py`
- Modify: `charts/kube-llmops-stack/charts/vllm/templates/deployment.yaml`

- [ ] **Step 1: Add MIG tests**

Append to `tests/helm/test_phase5_templates.py`:

```python
class TestMIGDevice:
    """Test MIG GPU device support."""

    def test_default_gpu_resource(self):
        docs = helm_template(
            set_values=SINGLE_MODEL,
            show_only="charts/vllm/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment")
        resources = deps[0]["spec"]["template"]["spec"]["containers"][0]["resources"]
        assert "nvidia.com/gpu" in resources["requests"]

    def test_mig_device_replaces_gpu(self):
        vals = {
            **SINGLE_MODEL,
            "global.models[0].resources.gpu": "0",
            "global.models[0].resources.migDevice": "nvidia.com/mig-1g.5gb",
        }
        docs = helm_template(
            set_values=vals,
            show_only="charts/vllm/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment")
        resources = deps[0]["spec"]["template"]["spec"]["containers"][0]["resources"]
        assert "nvidia.com/gpu" not in resources["requests"]
        assert resources["requests"]["nvidia.com/mig-1g.5gb"] == "1"
```

- [ ] **Step 2: Run test to verify MIG test fails**

Run: `python -m pytest tests/helm/test_phase5_templates.py::TestMIGDevice::test_mig_device_replaces_gpu -v`
Expected: FAIL — `nvidia.com/mig-1g.5gb` not in resources

- [ ] **Step 3: Add MIG support to vllm deployment.yaml**

In `charts/kube-llmops-stack/charts/vllm/templates/deployment.yaml`, modify the resources section. Find the requests block (around lines 248-253):

Old:
```yaml
          resources:
            requests:
              cpu: {{ $cpu | quote }}
              memory: {{ $memory }}
              {{- if gt (int $gpu) 0 }}
              nvidia.com/gpu: {{ $gpu | quote }}
              {{- end }}
            limits:
              {{- if not $.Values.unifiedMemory.enabled }}
              memory: {{ $memoryLimit | default $memory }}
              {{- end }}
              {{- if gt (int $gpu) 0 }}
              nvidia.com/gpu: {{ $gpu | quote }}
              {{- end }}
```

New:
```yaml
          resources:
            requests:
              cpu: {{ $cpu | quote }}
              memory: {{ $memory }}
              {{- if $resources.migDevice }}
              {{ $resources.migDevice }}: "1"
              {{- else if gt (int $gpu) 0 }}
              nvidia.com/gpu: {{ $gpu | quote }}
              {{- end }}
            limits:
              {{- if not $.Values.unifiedMemory.enabled }}
              memory: {{ $memoryLimit | default $memory }}
              {{- end }}
              {{- if $resources.migDevice }}
              {{ $resources.migDevice }}: "1"
              {{- else if gt (int $gpu) 0 }}
              nvidia.com/gpu: {{ $gpu | quote }}
              {{- end }}
```

- [ ] **Step 4: Rebuild and run tests**

```bash
cd charts/kube-llmops-stack && rm -f charts/*.tgz Chart.lock && helm dependency update . && cd ../..
python -m pytest tests/helm/test_phase5_templates.py::TestMIGDevice -v
```
Expected: 2 PASSED

- [ ] **Step 5: Commit**

```bash
git add charts/kube-llmops-stack/charts/vllm/templates/deployment.yaml \
       tests/helm/test_phase5_templates.py
git commit -m "feat(vllm): MIG GPU device support (nvidia.com/mig-*)"
```

### Task 13: Cost dashboard panels

**Files:**
- Modify: `charts/kube-llmops-stack/charts/observability/dashboards/cost-usage.json`

- [ ] **Step 1: Add cost dashboard panels**

Read `cost-usage.json`, add 2 new panels after the existing 6:

Panel 7 — GPU Idle Rate (timeseries, gridPos: h=8, w=12, x=0, y=12):
```json
{
  "id": 7,
  "title": "GPU Idle Rate by Model",
  "type": "timeseries",
  "gridPos": {"h": 8, "w": 12, "x": 0, "y": 12},
  "datasource": {"type": "prometheus", "uid": "prometheus"},
  "targets": [
    {
      "expr": "1 - avg(DCGM_FI_PROF_GR_ENGINE_ACTIVE) by (model_name)",
      "legendFormat": "{{model_name}} idle %"
    }
  ],
  "fieldConfig": {
    "defaults": {
      "unit": "percentunit",
      "min": 0,
      "max": 1,
      "custom": {"lineWidth": 2, "fillOpacity": 20}
    }
  }
}
```

Panel 8 — Scale-to-Zero Events (timeseries, gridPos: h=8, w=12, x=12, y=12):
```json
{
  "id": 8,
  "title": "Scale-to-Zero Events",
  "type": "timeseries",
  "gridPos": {"h": 8, "w": 12, "x": 12, "y": 12},
  "datasource": {"type": "prometheus", "uid": "prometheus"},
  "targets": [
    {
      "expr": "kube_horizontalpodautoscaler_status_current_replicas{horizontalpodautoscaler=~\".*-scaler\"} == 0",
      "legendFormat": "{{horizontalpodautoscaler}} at zero"
    },
    {
      "expr": "changes(kube_horizontalpodautoscaler_status_current_replicas{horizontalpodautoscaler=~\".*-scaler\"}[1h])",
      "legendFormat": "{{horizontalpodautoscaler}} scale events/h"
    }
  ],
  "fieldConfig": {
    "defaults": {
      "unit": "short",
      "custom": {"lineWidth": 2, "fillOpacity": 10}
    }
  }
}
```

- [ ] **Step 2: Validate JSON and rebuild**

```bash
python3 -c "import json; json.load(open('charts/kube-llmops-stack/charts/observability/dashboards/cost-usage.json'))"
cd charts/kube-llmops-stack && rm -f charts/*.tgz Chart.lock && helm dependency update . && cd ../..
```

- [ ] **Step 3: Commit**

```bash
git add charts/kube-llmops-stack/charts/observability/dashboards/cost-usage.json
git commit -m "feat(dashboards): GPU idle rate + scale-to-zero events panels"
```

### Task 14: Batch 2 verification

- [ ] **Step 1: Run all Phase 5 tests**

```bash
python -m pytest tests/helm/test_phase5_templates.py -v
```
Expected: All tests PASS

- [ ] **Step 2: Run regression tests**

```bash
python -m pytest tests/helm/test_finetune_templates.py -v
```
Expected: All 35+ PASS

- [ ] **Step 3: Full render check**

```bash
helm template test charts/kube-llmops-stack -f charts/kube-llmops-stack/values-single-node.yaml --set keda.enabled=true --set keda.triggers.ttftP95.enabled=true 2>&1 | head -5
```
Expected: Renders without error

---

## Batch 3: Advanced Deployment

### Task 15: Canary deployment tests

**Files:**
- Modify: `tests/helm/test_phase5_templates.py`

- [ ] **Step 1: Add canary deployment tests**

Append to `tests/helm/test_phase5_templates.py`:

```python
CANARY_MODEL = {
    "global.models[0].name": "test-model",
    "global.models[0].source": "org/test-model-v1",
    "global.models[0].resources.gpu": "1",
    "global.models[0].resources.memory": "16Gi",
    "global.models[0].canary.enabled": "true",
    "global.models[0].canary.source": "org/test-model-v2",
    "global.models[0].canary.weight": "10",
    "global.models[0].canary.replicas": "1",
}


class TestCanaryDeployment:
    """Test canary model deployment."""

    def test_no_canary_by_default(self):
        docs = helm_template(
            set_values=SINGLE_MODEL,
            show_only="charts/vllm/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment")
        assert len(deps) == 1
        assert "canary" not in deps[0]["metadata"]["name"]

    def test_canary_deployment_rendered(self):
        docs = helm_template(
            set_values=CANARY_MODEL,
            show_only="charts/vllm/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment")
        assert len(deps) == 2
        names = [d["metadata"]["name"] for d in deps]
        assert "vllm-test-model" in names
        assert "vllm-test-model-canary" in names

    def test_canary_service_rendered(self):
        docs = helm_template(
            set_values=CANARY_MODEL,
            show_only="charts/vllm/templates/service.yaml",
        )
        svcs = find_by_kind(docs, "Service")
        assert len(svcs) == 2
        names = [s["metadata"]["name"] for s in svcs]
        assert "vllm-test-model" in names
        assert "vllm-test-model-canary" in names

    def test_canary_litellm_weight_routing(self):
        docs = helm_template(set_values=CANARY_MODEL)
        config = get_configmap_data(docs, "litellm-config")
        entries = [e for e in config["model_list"] if e["model_name"] == "test-model"]
        assert len(entries) == 2
        weights = sorted([e["litellm_params"].get("weight", 100) for e in entries])
        assert weights == [10, 90]

    def test_canary_uses_different_source(self):
        docs = helm_template(
            set_values=CANARY_MODEL,
            show_only="charts/vllm/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment", "canary")
        assert len(deps) == 1
        args = deps[0]["spec"]["template"]["spec"]["containers"][0]["args"][0]
        assert "org--test-model-v2" in args
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `python -m pytest tests/helm/test_phase5_templates.py::TestCanaryDeployment -v`
Expected: All FAIL — no canary logic exists

- [ ] **Step 3: Commit red tests**

```bash
git add tests/helm/test_phase5_templates.py
git commit -m "test: canary deployment tests (red)"
```

### Task 16: Implement canary deployment in vllm templates

**Files:**
- Modify: `charts/kube-llmops-stack/charts/vllm/templates/deployment.yaml`
- Modify: `charts/kube-llmops-stack/charts/vllm/templates/service.yaml`

- [ ] **Step 1: Add canary Deployment to vllm deployment.yaml**

At the very end of `charts/kube-llmops-stack/charts/vllm/templates/deployment.yaml`, before the final `{{- end }}` closing tags, append a canary Deployment block. Find the end of the file (lines 299-301):

Old:
```yaml
        {{- end }}
{{- end }}
{{- end }}
```

New:
```yaml
        {{- end }}
{{- if and $model.canary $model.canary.enabled }}
{{- $canarySource := $model.canary.source }}
{{- $canarySlug := replace "/" "--" $canarySource }}
{{- $canaryReplicas := $model.canary.replicas | default 1 }}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: vllm-{{ $model.name }}-canary
  labels:
    app.kubernetes.io/name: vllm
    app.kubernetes.io/instance: {{ $.Release.Name }}
    app.kubernetes.io/component: canary
    app.kubernetes.io/part-of: kube-llmops
    kube-llmops/model: {{ $model.name }}-canary
    kube-llmops/engine: vllm
spec:
  replicas: {{ $canaryReplicas }}
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app.kubernetes.io/name: vllm
      app.kubernetes.io/instance: {{ $.Release.Name }}
      kube-llmops/model: {{ $model.name }}-canary
  template:
    metadata:
      labels:
        app.kubernetes.io/name: vllm
        app.kubernetes.io/instance: {{ $.Release.Name }}
        app.kubernetes.io/component: canary
        app.kubernetes.io/part-of: kube-llmops
        kube-llmops/model: {{ $model.name }}-canary
        kube-llmops/engine: vllm
    spec:
      enableServiceLinks: false
      terminationGracePeriodSeconds: 90
      initContainers:
        - name: model-loader
          image: {{ (default dict (default dict $.Values.global).modelStore).image | default "kube-llmops/model-loader:latest" }}
          imagePullPolicy: IfNotPresent
          command: ["bash", "-c"]
          args:
            - |
{{ include "kube-llmops.modelLoaderScript" . | indent 14 }}
          env:
{{ include "kube-llmops.modelLoaderEnv" (dict "model" (dict "source" $canarySource "name" $model.name) "root" $ "mountPath" "/models" "hfHome" "/models/huggingface" "hfToken" $.Values.modelLoader.hfToken) | indent 12 }}
          volumeMounts:
            - name: model-cache
              mountPath: /models
          resources:
            requests:
              cpu: "1"
              memory: 2Gi
            limits:
              cpu: "4"
              memory: 8Gi
      containers:
        - name: vllm
          image: "{{ $.Values.image.repository }}:{{ $.Values.image.tag }}"
          imagePullPolicy: {{ $.Values.image.pullPolicy }}
          ports:
            - name: http
              containerPort: 8000
              protocol: TCP
          command: ["/bin/bash", "-c"]
          args:
            - |
              MODEL_PATH="/models/{{ $canarySlug }}"
              if [ ! -d "$MODEL_PATH" ] || [ -z "$(ls -A "$MODEL_PATH" 2>/dev/null)" ]; then
                MODEL_PATH="{{ $canarySource }}"
              fi
              EXTRA_ARGS=""
              exec vllm serve "$MODEL_PATH" \
                --host 0.0.0.0 \
                --port 8000 \
                --served-model-name {{ $model.name | quote }} \
                {{- if $model.prefixCaching }}
                --enable-prefix-caching \
                {{- end }}
                {{- if gt (int $gpu) 1 }}
                --tensor-parallel-size {{ $gpu | quote }} \
                {{- end }}
                {{- range $key, $val := $model.engineArgs }}
                {{ $key }} {{ if $val }}{{ $val }}{{ end }} \
                {{- end }}
                $EXTRA_ARGS
          env:
            - name: HF_HOME
              value: /models/huggingface
          readinessProbe:
            httpGet:
              path: /health
              port: 8000
            initialDelaySeconds: {{ $.Values.readinessProbe.initialDelaySeconds }}
            periodSeconds: {{ $.Values.readinessProbe.periodSeconds }}
            timeoutSeconds: {{ $.Values.readinessProbe.timeoutSeconds }}
            failureThreshold: {{ $.Values.readinessProbe.failureThreshold }}
          livenessProbe:
            httpGet:
              path: /health
              port: 8000
            initialDelaySeconds: {{ $.Values.livenessProbe.initialDelaySeconds }}
            periodSeconds: {{ $.Values.livenessProbe.periodSeconds }}
            timeoutSeconds: {{ $.Values.livenessProbe.timeoutSeconds }}
            failureThreshold: {{ $.Values.livenessProbe.failureThreshold }}
          resources:
            requests:
              cpu: {{ $cpu | quote }}
              memory: {{ $memory }}
              {{- if gt (int $gpu) 0 }}
              nvidia.com/gpu: {{ $gpu | quote }}
              {{- end }}
            limits:
              memory: {{ $memoryLimit | default $memory }}
              {{- if gt (int $gpu) 0 }}
              nvidia.com/gpu: {{ $gpu | quote }}
              {{- end }}
          volumeMounts:
            - name: model-cache
              mountPath: /models
            - name: shm
              mountPath: /dev/shm
          lifecycle:
            preStop:
              exec:
                command: ["/bin/sh", "-c", "sleep 5"]
      {{- if gt (int $gpu) 0 }}
      tolerations:
        - key: {{ $.Values.gpuToleration.key }}
          operator: {{ $.Values.gpuToleration.operator }}
          effect: {{ $.Values.gpuToleration.effect }}
      {{- end }}
      volumes:
        - name: model-cache
          emptyDir:
            sizeLimit: {{ $.Values.modelCache.size }}
        - name: shm
          emptyDir:
            medium: Memory
            sizeLimit: 8Gi
{{- end }}
{{- end }}
{{- end }}
```

- [ ] **Step 2: Add canary Service to vllm service.yaml**

At the end of `charts/kube-llmops-stack/charts/vllm/templates/service.yaml`, before the final `{{- end }}` tags, add:

Old:
```yaml
  selector:
    app.kubernetes.io/name: vllm
    app.kubernetes.io/instance: {{ $.Release.Name }}
    kube-llmops/model: {{ $model.name }}
{{- end }}
{{- end }}
```

New:
```yaml
  selector:
    app.kubernetes.io/name: vllm
    app.kubernetes.io/instance: {{ $.Release.Name }}
    kube-llmops/model: {{ $model.name }}
{{- if and $model.canary $model.canary.enabled }}
---
apiVersion: v1
kind: Service
metadata:
  name: vllm-{{ $model.name }}-canary
  labels:
    app.kubernetes.io/name: vllm
    app.kubernetes.io/instance: {{ $.Release.Name }}
    app.kubernetes.io/component: canary
    app.kubernetes.io/part-of: kube-llmops
    kube-llmops/model: {{ $model.name }}-canary
    kube-llmops/engine: vllm
spec:
  type: {{ $.Values.service.type }}
  ports:
    - port: {{ $.Values.service.port }}
      targetPort: http
      protocol: TCP
      name: http
  selector:
    app.kubernetes.io/name: vllm
    app.kubernetes.io/instance: {{ $.Release.Name }}
    kube-llmops/model: {{ $model.name }}-canary
{{- end }}
{{- end }}
{{- end }}
```

- [ ] **Step 3: Rebuild charts**

```bash
cd charts/kube-llmops-stack && rm -f charts/*.tgz Chart.lock && helm dependency update . && cd ../..
```

- [ ] **Step 4: Run deployment + service tests**

Run: `python -m pytest tests/helm/test_phase5_templates.py::TestCanaryDeployment::test_no_canary_by_default tests/helm/test_phase5_templates.py::TestCanaryDeployment::test_canary_deployment_rendered tests/helm/test_phase5_templates.py::TestCanaryDeployment::test_canary_service_rendered tests/helm/test_phase5_templates.py::TestCanaryDeployment::test_canary_uses_different_source -v`
Expected: 4 PASSED

- [ ] **Step 5: Commit**

```bash
git add charts/kube-llmops-stack/charts/vllm/templates/deployment.yaml \
       charts/kube-llmops-stack/charts/vllm/templates/service.yaml
git commit -m "feat(vllm): canary deployment + service for model updates"
```

### Task 17: Canary weight routing in LiteLLM configmap

**Files:**
- Modify: `charts/kube-llmops-stack/charts/litellm/templates/configmap.yaml:17-42`

- [ ] **Step 1: Add canary weight routing to litellm configmap**

In `charts/kube-llmops-stack/charts/litellm/templates/configmap.yaml`, modify the global models loop. Replace the existing model entry block (lines 37-42 area, after the fallback model_info addition):

Old:
```yaml
      - model_name: {{ $model.name }}
        litellm_params:
          model: {{ $litellmPrefix }}/{{ $model.name }}
          api_base: http://{{ $prefix }}-{{ $model.name }}.{{ $.Release.Namespace }}.svc.cluster.local:{{ $port }}{{ $apiSuffix }}
          api_key: "no-key-needed"
        {{- if and $model.scaleToZero $model.scaleToZero.fallbackModel }}
        model_info:
          metadata:
            fallbacks:
              - {{ $model.scaleToZero.fallbackModel }}
        {{- end }}
    {{- end }}
```

New:
```yaml
      {{- if and $model.canary $model.canary.enabled }}
      - model_name: {{ $model.name }}
        litellm_params:
          model: {{ $litellmPrefix }}/{{ $model.name }}
          api_base: http://{{ $prefix }}-{{ $model.name }}.{{ $.Release.Namespace }}.svc.cluster.local:{{ $port }}{{ $apiSuffix }}
          api_key: "no-key-needed"
          weight: {{ sub 100 (int ($model.canary.weight | default 10)) }}
      - model_name: {{ $model.name }}
        litellm_params:
          model: {{ $litellmPrefix }}/{{ $model.name }}
          api_base: http://{{ $prefix }}-{{ $model.name }}-canary.{{ $.Release.Namespace }}.svc.cluster.local:{{ $port }}{{ $apiSuffix }}
          api_key: "no-key-needed"
          weight: {{ $model.canary.weight | default 10 }}
      {{- else }}
      - model_name: {{ $model.name }}
        litellm_params:
          model: {{ $litellmPrefix }}/{{ $model.name }}
          api_base: http://{{ $prefix }}-{{ $model.name }}.{{ $.Release.Namespace }}.svc.cluster.local:{{ $port }}{{ $apiSuffix }}
          api_key: "no-key-needed"
        {{- if and $model.scaleToZero $model.scaleToZero.fallbackModel }}
        model_info:
          metadata:
            fallbacks:
              - {{ $model.scaleToZero.fallbackModel }}
        {{- end }}
      {{- end }}
    {{- end }}
```

- [ ] **Step 2: Rebuild and run canary weight test**

```bash
cd charts/kube-llmops-stack && rm -f charts/*.tgz Chart.lock && helm dependency update . && cd ../..
python -m pytest tests/helm/test_phase5_templates.py::TestCanaryDeployment::test_canary_litellm_weight_routing -v
```
Expected: PASSED

- [ ] **Step 3: Run all canary tests**

Run: `python -m pytest tests/helm/test_phase5_templates.py::TestCanaryDeployment -v`
Expected: 5 PASSED

- [ ] **Step 4: Commit**

```bash
git add charts/kube-llmops-stack/charts/litellm/templates/configmap.yaml
git commit -m "feat(litellm): canary weight routing in model config"
```

### Task 18: Canary dashboard panels

**Files:**
- Modify: `charts/kube-llmops-stack/charts/observability/dashboards/litellm-gateway.json`

- [ ] **Step 1: Add canary panels to litellm-gateway dashboard**

Read `litellm-gateway.json`, add a new row + 2 panels at the end:

Row panel (id=103):
```json
{
  "id": 103,
  "title": "Canary Deployment",
  "type": "row",
  "gridPos": {"h": 1, "w": 24, "x": 0, "y": 20},
  "collapsed": false
}
```

Panel 9 — Canary vs Primary Latency (timeseries, gridPos: h=8, w=12, x=0, y=21):
```json
{
  "id": 9,
  "title": "Canary vs Primary Latency (P95)",
  "type": "timeseries",
  "gridPos": {"h": 8, "w": 12, "x": 0, "y": 21},
  "datasource": {"type": "prometheus", "uid": "prometheus"},
  "targets": [
    {
      "expr": "histogram_quantile(0.95, sum(rate(vllm:e2e_request_latency_seconds_bucket{model_name!~\".*-canary\"}[5m])) by (le, model_name))",
      "legendFormat": "{{model_name}} primary P95"
    },
    {
      "expr": "histogram_quantile(0.95, sum(rate(vllm:e2e_request_latency_seconds_bucket{model_name=~\".*-canary\"}[5m])) by (le, model_name))",
      "legendFormat": "{{model_name}} canary P95"
    }
  ],
  "fieldConfig": {
    "defaults": {"unit": "s", "custom": {"lineWidth": 2, "fillOpacity": 10}}
  }
}
```

Panel 10 — Canary Traffic Weight (gauge, gridPos: h=8, w=12, x=12, y=21):
```json
{
  "id": 10,
  "title": "Canary Traffic Weight",
  "type": "gauge",
  "gridPos": {"h": 8, "w": 12, "x": 12, "y": 21},
  "datasource": {"type": "prometheus", "uid": "prometheus"},
  "targets": [
    {
      "expr": "sum(rate(litellm_request_total{model=~\".*-canary\"}[5m])) / sum(rate(litellm_request_total[5m]))",
      "legendFormat": "Canary %"
    }
  ],
  "fieldConfig": {
    "defaults": {
      "unit": "percentunit",
      "min": 0,
      "max": 1,
      "thresholds": {
        "steps": [
          {"color": "green", "value": null},
          {"color": "yellow", "value": 0.25},
          {"color": "red", "value": 0.5}
        ]
      }
    }
  }
}
```

- [ ] **Step 2: Validate JSON and commit**

```bash
python3 -c "import json; json.load(open('charts/kube-llmops-stack/charts/observability/dashboards/litellm-gateway.json'))"
git add charts/kube-llmops-stack/charts/observability/dashboards/litellm-gateway.json
git commit -m "feat(dashboards): canary vs primary latency + traffic weight panels"
```

### Task 19: llm-d disaggregated serving tests

**Files:**
- Modify: `tests/helm/test_phase5_templates.py`

- [ ] **Step 1: Add llm-d tests**

Append to `tests/helm/test_phase5_templates.py`:

```python
DISAGG_MODEL = {
    "global.models[0].name": "big-model",
    "global.models[0].source": "org/big-model",
    "global.models[0].resources.gpu": "4",
    "global.models[0].resources.memory": "64Gi",
    "global.models[0].disaggregated.enabled": "true",
    "global.models[0].disaggregated.prefill.replicas": "2",
    "global.models[0].disaggregated.prefill.resources.gpu": "4",
    "global.models[0].disaggregated.prefill.resources.memory": "64Gi",
    "global.models[0].disaggregated.decode.replicas": "4",
    "global.models[0].disaggregated.decode.resources.gpu": "2",
    "global.models[0].disaggregated.decode.resources.memory": "32Gi",
}


class TestDisaggregatedServing:
    """Test llm-d disaggregated serving templates."""

    def test_no_disaggregated_resources_by_default(self):
        docs = helm_template(
            set_values=SINGLE_MODEL,
            show_only="charts/vllm/templates/disaggregated.yaml",
        )
        # Should render nothing when disaggregated is not enabled
        assert len(docs) == 0

    def test_disaggregated_creates_prefill_and_decode(self):
        docs = helm_template(
            set_values=DISAGG_MODEL,
            show_only="charts/vllm/templates/disaggregated.yaml",
        )
        deps = find_by_kind(docs, "Deployment")
        assert len(deps) == 2
        names = [d["metadata"]["name"] for d in deps]
        assert "big-model-prefill" in names
        assert "big-model-decode" in names

    def test_disaggregated_creates_inference_pool(self):
        docs = helm_template(
            set_values=DISAGG_MODEL,
            show_only="charts/vllm/templates/disaggregated.yaml",
        )
        pools = find_by_kind(docs, "InferencePool")
        assert len(pools) == 1
        assert pools[0]["metadata"]["name"] == "big-model-pool"

    def test_disaggregated_creates_inference_model(self):
        docs = helm_template(
            set_values=DISAGG_MODEL,
            show_only="charts/vllm/templates/disaggregated.yaml",
        )
        models = find_by_kind(docs, "InferenceModel")
        assert len(models) == 1
        assert models[0]["spec"]["modelName"] == "big-model"

    def test_epp_deployment_created(self):
        docs = helm_template(
            set_values=DISAGG_MODEL,
            show_only="charts/vllm/templates/epp.yaml",
        )
        deps = find_by_kind(docs, "Deployment")
        assert len(deps) == 1
        assert "epp" in deps[0]["metadata"]["name"]

    def test_no_epp_when_disabled(self):
        docs = helm_template(
            set_values=SINGLE_MODEL,
            show_only="charts/vllm/templates/epp.yaml",
        )
        assert len(docs) == 0
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `python -m pytest tests/helm/test_phase5_templates.py::TestDisaggregatedServing -v`
Expected: All FAIL — templates don't exist yet

- [ ] **Step 3: Commit red tests**

```bash
git add tests/helm/test_phase5_templates.py
git commit -m "test: llm-d disaggregated serving tests (red)"
```

### Task 20: Implement llm-d templates

**Files:**
- Create: `charts/kube-llmops-stack/charts/vllm/templates/disaggregated.yaml`
- Create: `charts/kube-llmops-stack/charts/vllm/templates/epp.yaml`

- [ ] **Step 1: Create disaggregated.yaml**

Create `charts/kube-llmops-stack/charts/vllm/templates/disaggregated.yaml`:

```yaml
{{/*
llm-d disaggregated serving: separate Prefill + Decode Deployments.
Feature-gated: only renders when model.disaggregated.enabled is true.
Prerequisite: Gateway API CRDs must be installed.
*/}}
{{- $models := .Values.models }}
{{- if and .Values.global .Values.global.models }}
{{- $models = .Values.global.models }}
{{- end }}
{{- range $idx, $model := $models }}
{{- $resolvedEngine := include "kube-llmops.resolveEngine" $model | trim }}
{{- if and (eq $resolvedEngine "vllm") $model.disaggregated $model.disaggregated.enabled }}
{{- $prefill := $model.disaggregated.prefill }}
{{- $decode := $model.disaggregated.decode }}
{{- $slug := replace "/" "--" $model.source }}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ $model.name }}-prefill
  labels:
    app.kubernetes.io/name: vllm
    app.kubernetes.io/instance: {{ $.Release.Name }}
    app.kubernetes.io/component: prefill
    app.kubernetes.io/part-of: kube-llmops
    kube-llmops/model: {{ $model.name }}
    kube-llmops/role: prefill
spec:
  replicas: {{ $prefill.replicas | default 1 }}
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app.kubernetes.io/name: vllm
      app.kubernetes.io/instance: {{ $.Release.Name }}
      kube-llmops/model: {{ $model.name }}
      kube-llmops/role: prefill
  template:
    metadata:
      labels:
        app.kubernetes.io/name: vllm
        app.kubernetes.io/instance: {{ $.Release.Name }}
        app.kubernetes.io/component: prefill
        app.kubernetes.io/part-of: kube-llmops
        kube-llmops/model: {{ $model.name }}
        kube-llmops/role: prefill
    spec:
      terminationGracePeriodSeconds: 90
      initContainers:
        - name: model-loader
          image: {{ (default dict (default dict $.Values.global).modelStore).image | default "kube-llmops/model-loader:latest" }}
          imagePullPolicy: IfNotPresent
          command: ["bash", "-c"]
          args:
            - |
{{ include "kube-llmops.modelLoaderScript" . | indent 14 }}
          env:
{{ include "kube-llmops.modelLoaderEnv" (dict "model" $model "root" $ "mountPath" "/models" "hfHome" "/models/huggingface") | indent 12 }}
          volumeMounts:
            - name: model-cache
              mountPath: /models
          resources:
            requests:
              cpu: "1"
              memory: 2Gi
            limits:
              cpu: "4"
              memory: 8Gi
      containers:
        - name: vllm
          image: "{{ $.Values.image.repository }}:{{ $.Values.image.tag }}"
          imagePullPolicy: {{ $.Values.image.pullPolicy }}
          ports:
            - name: http
              containerPort: 8000
          command: ["/bin/bash", "-c"]
          args:
            - |
              MODEL_PATH="/models/{{ $slug }}"
              if [ ! -d "$MODEL_PATH" ] || [ -z "$(ls -A "$MODEL_PATH" 2>/dev/null)" ]; then
                MODEL_PATH="{{ $model.source }}"
              fi
              exec vllm serve "$MODEL_PATH" \
                --host 0.0.0.0 --port 8000 \
                --served-model-name {{ $model.name | quote }} \
                --kv-transfer-config '{"kv_connector":"PyNcclConnector","kv_role":"kv_producer"}' \
                {{- range $key, $val := $model.engineArgs }}
                {{ $key }} {{ if $val }}{{ $val }}{{ end }} \
                {{- end }}
                --tensor-parallel-size {{ (default dict $prefill.resources).gpu | default 1 | quote }}
          env:
            - name: HF_HOME
              value: /models/huggingface
          resources:
            requests:
              cpu: "4"
              memory: {{ (default dict $prefill.resources).memory | default "64Gi" }}
              nvidia.com/gpu: {{ (default dict $prefill.resources).gpu | default 4 | quote }}
            limits:
              memory: {{ (default dict $prefill.resources).memory | default "64Gi" }}
              nvidia.com/gpu: {{ (default dict $prefill.resources).gpu | default 4 | quote }}
          volumeMounts:
            - name: model-cache
              mountPath: /models
            - name: shm
              mountPath: /dev/shm
      tolerations:
        - key: {{ $.Values.gpuToleration.key }}
          operator: {{ $.Values.gpuToleration.operator }}
          effect: {{ $.Values.gpuToleration.effect }}
      volumes:
        - name: model-cache
          emptyDir:
            sizeLimit: {{ $.Values.modelCache.size }}
        - name: shm
          emptyDir:
            medium: Memory
            sizeLimit: 8Gi
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ $model.name }}-decode
  labels:
    app.kubernetes.io/name: vllm
    app.kubernetes.io/instance: {{ $.Release.Name }}
    app.kubernetes.io/component: decode
    app.kubernetes.io/part-of: kube-llmops
    kube-llmops/model: {{ $model.name }}
    kube-llmops/role: decode
spec:
  replicas: {{ $decode.replicas | default 1 }}
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app.kubernetes.io/name: vllm
      app.kubernetes.io/instance: {{ $.Release.Name }}
      kube-llmops/model: {{ $model.name }}
      kube-llmops/role: decode
  template:
    metadata:
      labels:
        app.kubernetes.io/name: vllm
        app.kubernetes.io/instance: {{ $.Release.Name }}
        app.kubernetes.io/component: decode
        app.kubernetes.io/part-of: kube-llmops
        kube-llmops/model: {{ $model.name }}
        kube-llmops/role: decode
    spec:
      terminationGracePeriodSeconds: 90
      initContainers:
        - name: model-loader
          image: {{ (default dict (default dict $.Values.global).modelStore).image | default "kube-llmops/model-loader:latest" }}
          imagePullPolicy: IfNotPresent
          command: ["bash", "-c"]
          args:
            - |
{{ include "kube-llmops.modelLoaderScript" . | indent 14 }}
          env:
{{ include "kube-llmops.modelLoaderEnv" (dict "model" $model "root" $ "mountPath" "/models" "hfHome" "/models/huggingface") | indent 12 }}
          volumeMounts:
            - name: model-cache
              mountPath: /models
          resources:
            requests:
              cpu: "1"
              memory: 2Gi
            limits:
              cpu: "4"
              memory: 8Gi
      containers:
        - name: vllm
          image: "{{ $.Values.image.repository }}:{{ $.Values.image.tag }}"
          imagePullPolicy: {{ $.Values.image.pullPolicy }}
          ports:
            - name: http
              containerPort: 8000
          command: ["/bin/bash", "-c"]
          args:
            - |
              MODEL_PATH="/models/{{ $slug }}"
              if [ ! -d "$MODEL_PATH" ] || [ -z "$(ls -A "$MODEL_PATH" 2>/dev/null)" ]; then
                MODEL_PATH="{{ $model.source }}"
              fi
              exec vllm serve "$MODEL_PATH" \
                --host 0.0.0.0 --port 8000 \
                --served-model-name {{ $model.name | quote }} \
                --kv-transfer-config '{"kv_connector":"PyNcclConnector","kv_role":"kv_consumer"}' \
                {{- range $key, $val := $model.engineArgs }}
                {{ $key }} {{ if $val }}{{ $val }}{{ end }} \
                {{- end }}
                --tensor-parallel-size {{ (default dict $decode.resources).gpu | default 1 | quote }}
          env:
            - name: HF_HOME
              value: /models/huggingface
          resources:
            requests:
              cpu: "4"
              memory: {{ (default dict $decode.resources).memory | default "32Gi" }}
              nvidia.com/gpu: {{ (default dict $decode.resources).gpu | default 2 | quote }}
            limits:
              memory: {{ (default dict $decode.resources).memory | default "32Gi" }}
              nvidia.com/gpu: {{ (default dict $decode.resources).gpu | default 2 | quote }}
          volumeMounts:
            - name: model-cache
              mountPath: /models
            - name: shm
              mountPath: /dev/shm
      tolerations:
        - key: {{ $.Values.gpuToleration.key }}
          operator: {{ $.Values.gpuToleration.operator }}
          effect: {{ $.Values.gpuToleration.effect }}
      volumes:
        - name: model-cache
          emptyDir:
            sizeLimit: {{ $.Values.modelCache.size }}
        - name: shm
          emptyDir:
            medium: Memory
            sizeLimit: 8Gi
---
apiVersion: inference.networking.x-k8s.io/v1alpha2
kind: InferencePool
metadata:
  name: {{ $model.name }}-pool
  labels:
    app.kubernetes.io/part-of: kube-llmops
spec:
  targetPortNumber: 8000
  selector:
    kube-llmops/model: {{ $model.name }}
---
apiVersion: inference.networking.x-k8s.io/v1alpha2
kind: InferenceModel
metadata:
  name: {{ $model.name }}-im
  labels:
    app.kubernetes.io/part-of: kube-llmops
spec:
  modelName: {{ $model.name }}
  pool:
    name: {{ $model.name }}-pool
{{- end }}
{{- end }}
```

- [ ] **Step 2: Create epp.yaml**

Create `charts/kube-llmops-stack/charts/vllm/templates/epp.yaml`:

```yaml
{{/*
llm-d Endpoint Picker (EPP): intelligent router for disaggregated serving.
Only renders if any model has disaggregated.enabled: true.
*/}}
{{- $hasDisagg := false }}
{{- $models := .Values.models }}
{{- if and .Values.global .Values.global.models }}
{{- $models = .Values.global.models }}
{{- end }}
{{- range $model := $models }}
{{- if and $model.disaggregated $model.disaggregated.enabled }}
{{- $hasDisagg = true }}
{{- end }}
{{- end }}
{{- if $hasDisagg }}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ $.Release.Name }}-epp
  labels:
    app.kubernetes.io/name: epp
    app.kubernetes.io/instance: {{ $.Release.Name }}
    app.kubernetes.io/component: disaggregated
    app.kubernetes.io/part-of: kube-llmops
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: epp
      app.kubernetes.io/instance: {{ $.Release.Name }}
  template:
    metadata:
      labels:
        app.kubernetes.io/name: epp
        app.kubernetes.io/instance: {{ $.Release.Name }}
        app.kubernetes.io/component: disaggregated
        app.kubernetes.io/part-of: kube-llmops
    spec:
      containers:
        - name: epp
          image: us-central1-docker.pkg.dev/k8s-staging-llm-d/gateway-api-inference-extension/epp:main
          args:
            {{- range $model := $models }}
            {{- if and $model.disaggregated $model.disaggregated.enabled }}
            - --poolName={{ $model.name }}-pool
            - --poolNamespace={{ $.Release.Namespace }}
            {{- end }}
            {{- end }}
          ports:
            - name: grpc
              containerPort: 9002
          resources:
            requests:
              cpu: 200m
              memory: 256Mi
            limits:
              cpu: "1"
              memory: 512Mi
{{- end }}
```

- [ ] **Step 3: Rebuild and run tests**

```bash
cd charts/kube-llmops-stack && rm -f charts/*.tgz Chart.lock && helm dependency update . && cd ../..
python -m pytest tests/helm/test_phase5_templates.py::TestDisaggregatedServing -v
```
Expected: 6 PASSED

- [ ] **Step 4: Commit**

```bash
git add charts/kube-llmops-stack/charts/vllm/templates/disaggregated.yaml \
       charts/kube-llmops-stack/charts/vllm/templates/epp.yaml
git commit -m "feat(vllm): llm-d disaggregated serving (prefill/decode split, EPP)"
```

### Task 21: Batch 3 verification

- [ ] **Step 1: Run all Phase 5 tests**

```bash
python -m pytest tests/helm/test_phase5_templates.py -v
```
Expected: All tests PASS

- [ ] **Step 2: Run regression**

```bash
python -m pytest tests/helm/test_finetune_templates.py -v
```
Expected: All 35+ PASS

- [ ] **Step 3: Full render (with canary model)**

```bash
helm template test charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  --set 'global.models[0].canary.enabled=true' \
  --set 'global.models[0].canary.source=cyankiwi/gemma-4-v2' \
  --set 'global.models[0].canary.weight=10' \
  2>&1 | grep -c '^kind:'
```
Expected: More resources than default (canary Deployment + Service), no errors

---

## Batch 4: Multi-Accelerator + Docs

### Task 22: Multi-accelerator helper tests

**Files:**
- Modify: `tests/helm/test_phase5_templates.py`

- [ ] **Step 1: Add multi-accelerator tests**

Append to `tests/helm/test_phase5_templates.py`:

```python
class TestMultiAccelerator:
    """Test multi-accelerator support (nvidia, amd, gaudi)."""

    def test_default_nvidia_gpu_resource(self):
        docs = helm_template(
            set_values=SINGLE_MODEL,
            show_only="charts/vllm/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment")
        resources = deps[0]["spec"]["template"]["spec"]["containers"][0]["resources"]
        assert "nvidia.com/gpu" in resources["requests"]

    def test_amd_gpu_resource(self):
        vals = {**SINGLE_MODEL, "global.accelerator": "amd"}
        docs = helm_template(
            set_values=vals,
            show_only="charts/vllm/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment")
        resources = deps[0]["spec"]["template"]["spec"]["containers"][0]["resources"]
        assert "amd.com/gpu" in resources["requests"]
        assert "nvidia.com/gpu" not in resources["requests"]

    def test_gaudi_gpu_resource(self):
        vals = {**SINGLE_MODEL, "global.accelerator": "gaudi"}
        docs = helm_template(
            set_values=vals,
            show_only="charts/vllm/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment")
        resources = deps[0]["spec"]["template"]["spec"]["containers"][0]["resources"]
        assert "habana.ai/gaudi" in resources["requests"]

    def test_amd_toleration(self):
        vals = {**SINGLE_MODEL, "global.accelerator": "amd"}
        docs = helm_template(
            set_values=vals,
            show_only="charts/vllm/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment")
        tolerations = deps[0]["spec"]["template"]["spec"]["tolerations"]
        keys = [t["key"] for t in tolerations]
        assert "amd.com/gpu" in keys

    def test_dcgm_only_for_nvidia(self):
        vals = {**SINGLE_MODEL, "global.accelerator": "amd"}
        docs = helm_template(set_values=vals)
        dcgm = find_by_kind(docs, "DaemonSet", "dcgm")
        assert len(dcgm) == 0

    def test_dcgm_present_for_nvidia(self):
        docs = helm_template(set_values=SINGLE_MODEL)
        dcgm = find_by_kind(docs, "DaemonSet", "dcgm")
        assert len(dcgm) == 1

    def test_llamacpp_amd_gpu_resource(self):
        gguf_model = {
            "global.models[0].name": "test-gguf",
            "global.models[0].source": "org/test-GGUF",
            "global.models[0].resources.gpu": "1",
            "global.accelerator": "amd",
        }
        docs = helm_template(
            set_values=gguf_model,
            show_only="charts/llamacpp/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment")
        if len(deps) > 0:
            resources = deps[0]["spec"]["template"]["spec"]["containers"][0]["resources"]
            assert "amd.com/gpu" in resources["requests"]
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `python -m pytest tests/helm/test_phase5_templates.py::TestMultiAccelerator -v`
Expected: amd/gaudi tests FAIL

- [ ] **Step 3: Commit red tests**

```bash
git add tests/helm/test_phase5_templates.py
git commit -m "test: multi-accelerator tests (red)"
```

### Task 23: Implement multi-accelerator helpers

**Files:**
- Modify: `charts/kube-llmops-stack/templates/_helpers.tpl`

- [ ] **Step 1: Add accelerator helpers to _helpers.tpl**

Append at the end of `charts/kube-llmops-stack/templates/_helpers.tpl` (after line 367):

```yaml

{{/*
GPU resource name based on global.accelerator.
Usage: {{ include "kube-llmops.gpuResourceName" . }}
Output: "nvidia.com/gpu", "amd.com/gpu", or "habana.ai/gaudi"
*/}}
{{- define "kube-llmops.gpuResourceName" -}}
{{- $accel := (default dict .Values.global).accelerator | default "nvidia" -}}
{{- if eq $accel "amd" -}}
amd.com/gpu
{{- else if eq $accel "gaudi" -}}
habana.ai/gaudi
{{- else -}}
nvidia.com/gpu
{{- end -}}
{{- end -}}

{{/*
GPU toleration key based on global.accelerator.
Usage: {{ include "kube-llmops.gpuTolerationKey" . }}
*/}}
{{- define "kube-llmops.gpuTolerationKey" -}}
{{- $accel := (default dict .Values.global).accelerator | default "nvidia" -}}
{{- if eq $accel "amd" -}}
amd.com/gpu
{{- else if eq $accel "gaudi" -}}
habana.ai/gaudi
{{- else -}}
nvidia.com/gpu
{{- end -}}
{{- end -}}
```

- [ ] **Step 2: Commit helpers**

```bash
git add charts/kube-llmops-stack/templates/_helpers.tpl
git commit -m "feat(helpers): gpuResourceName + gpuTolerationKey for multi-accelerator"
```

### Task 24: Update all engine deployments to use accelerator helpers

**Files:**
- Modify: `charts/kube-llmops-stack/charts/vllm/templates/deployment.yaml`
- Modify: `charts/kube-llmops-stack/charts/llamacpp/templates/deployment.yaml`
- Modify: `charts/kube-llmops-stack/charts/tei/templates/deployment.yaml`
- Modify: `charts/kube-llmops-stack/charts/observability/templates/dcgm-exporter.yaml`

- [ ] **Step 1: Update vllm deployment.yaml**

Replace all occurrences of hardcoded `nvidia.com/gpu` in the primary Deployment (NOT the canary block) with `{{ include "kube-llmops.gpuResourceName" $ }}`:

In the resources.requests section:
```yaml
              {{- else if gt (int $gpu) 0 }}
              {{ include "kube-llmops.gpuResourceName" $ }}: {{ $gpu | quote }}
```

In the resources.limits section:
```yaml
              {{- else if gt (int $gpu) 0 }}
              {{ include "kube-llmops.gpuResourceName" $ }}: {{ $gpu | quote }}
```

Also update the toleration key. Change:
```yaml
        - key: {{ $.Values.gpuToleration.key }}
```
to:
```yaml
        - key: {{ include "kube-llmops.gpuTolerationKey" $ }}
```

Apply the same change everywhere `nvidia.com/gpu` appears in the primary deployment — there are 2 resource occurrences and 1 toleration.

Also update the canary block's GPU resources and tolerations with the same helper calls.

- [ ] **Step 2: Update llamacpp deployment.yaml**

Same substitutions in `charts/kube-llmops-stack/charts/llamacpp/templates/deployment.yaml`:

Replace `nvidia.com/gpu` (lines 126, 131) with `{{ include "kube-llmops.gpuResourceName" $ }}`.
Replace toleration key (line 138) with `{{ include "kube-llmops.gpuTolerationKey" $ }}`.

- [ ] **Step 3: Update tei deployment.yaml**

Same substitutions in `charts/kube-llmops-stack/charts/tei/templates/deployment.yaml`:

Replace `nvidia.com/gpu` (lines 119, 123) with `{{ include "kube-llmops.gpuResourceName" $ }}`.
Replace toleration key (line 130) with `{{ include "kube-llmops.gpuTolerationKey" $ }}`.

- [ ] **Step 4: Make DCGM exporter conditional on nvidia**

In `charts/kube-llmops-stack/charts/observability/templates/dcgm-exporter.yaml`, change line 1:

Old:
```yaml
{{- if .Values.dcgmExporter.enabled }}
```

New:
```yaml
{{- if and .Values.dcgmExporter.enabled (ne ((default dict .Values.global).accelerator | default "nvidia") "amd") (ne ((default dict .Values.global).accelerator | default "nvidia") "gaudi") }}
```

Also update the hardcoded toleration in dcgm-exporter.yaml (line 50):

Old:
```yaml
      tolerations:
        - key: nvidia.com/gpu
          operator: Exists
          effect: NoSchedule
```

New:
```yaml
      tolerations:
        - key: {{ include "kube-llmops.gpuTolerationKey" $ }}
          operator: Exists
          effect: NoSchedule
```

- [ ] **Step 5: Rebuild and run all multi-accelerator tests**

```bash
cd charts/kube-llmops-stack && rm -f charts/*.tgz Chart.lock && helm dependency update . && cd ../..
python -m pytest tests/helm/test_phase5_templates.py::TestMultiAccelerator -v
```
Expected: All PASSED (7-8 tests)

- [ ] **Step 6: Run full regression**

```bash
python -m pytest tests/helm/test_phase5_templates.py tests/helm/test_finetune_templates.py -v
```
Expected: All PASS

- [ ] **Step 7: Commit**

```bash
git add charts/kube-llmops-stack/charts/vllm/templates/deployment.yaml \
       charts/kube-llmops-stack/charts/llamacpp/templates/deployment.yaml \
       charts/kube-llmops-stack/charts/tei/templates/deployment.yaml \
       charts/kube-llmops-stack/charts/observability/templates/dcgm-exporter.yaml
git commit -m "feat: multi-accelerator GPU resource names + conditional DCGM exporter"
```

### Task 25: Documentation

**Files:**
- Create: `docs/routing.md`
- Create: `docs/large-model-deployment.md`
- Create: `docs/speculative-decoding.md`
- Create: `docs/kserve-integration.md`
- Create: `docs/disaggregated-serving.md`
- Create: `docs/model-updates.md`

- [ ] **Step 1: Create docs/routing.md**

```markdown
# Routing Strategies

## Latency-Based Routing (Default)

kube-llmops uses LiteLLM's `latency-based-routing` strategy by default. When a model has multiple replicas, requests are routed to the replica with the lowest observed latency.

```yaml
litellm:
  routingStrategy: latency-based-routing
  routingStrategyArgs:
    ttl: 60
    lowest_latency_buffer: 0.2
```

To switch to round-robin: `--set litellm.routingStrategy=simple-shuffle`

## Prefix Caching

Enable vLLM prefix caching per model to cache KV state for repeated system prompts:

```yaml
global:
  models:
    - name: my-model
      prefixCaching: true
```

This injects `--enable-prefix-caching` into the vLLM command. Monitor hit rate via the **vLLM Model Serving** Grafana dashboard.

## Session Affinity (Advanced)

For multi-replica models with prefix caching, configure session affinity to route identical system prompts to the same Pod:

```yaml
litellm:
  sessionAffinity:
    enabled: true
    header: "x-session-id"
```

This deploys an Envoy sidecar with consistent hashing on the configured header.
```

- [ ] **Step 2: Create docs/large-model-deployment.md**

```markdown
# Large Model Deployment Guide

## Tensor Parallelism (Multi-GPU)

For models too large for a single GPU, use tensor parallelism:

```yaml
global:
  models:
    - name: llama-70b
      source: meta-llama/Llama-3.3-70B-Instruct
      resources:
        gpu: 4          # 4-way tensor parallel
        memory: 128Gi
```

vLLM automatically sets `--tensor-parallel-size` based on `resources.gpu`.

## Memory Estimation

Rule of thumb for FP16/BF16: **model params x 2 bytes + 20% overhead**.

| Model Size | FP16 Weight | Min GPU Memory |
|-----------|-------------|----------------|
| 7B | ~14 GB | 1x 24GB GPU |
| 13B | ~26 GB | 2x 24GB GPU |
| 70B | ~140 GB | 4x 80GB GPU |

## Quantization

AWQ and GPTQ models use less memory:

```yaml
- name: model-awq
  source: org/model-AWQ
  engineArgs:
    --dtype: "half"
    --quantization: "awq"
```

## MoE Models (Mixture of Experts)

MoE models (e.g., Mixtral, DeepSeek) activate only a subset of parameters per token. Weight memory is full size but compute is lower.

```yaml
- name: deepseek-r1
  source: deepseek-ai/DeepSeek-R1
  resources:
    gpu: 8
    memory: 256Gi
```

## MIG GPU Sharing

For small models (embedding, reranker), use MIG to share a GPU:

```yaml
- name: bge-small
  source: BAAI/bge-small-en-v1.5
  resources:
    gpu: 0
    migDevice: "nvidia.com/mig-1g.5gb"
    memory: 2Gi
```
```

- [ ] **Step 3: Create docs/speculative-decoding.md**

```markdown
# Speculative Decoding

Speculative decoding uses a small "draft" model to predict multiple tokens, then verifies them with the main model in a single forward pass. This can reduce latency by 2-3x.

## Configuration

Use `engineArgs` to configure speculative decoding:

```yaml
global:
  models:
    - name: llama-70b
      source: meta-llama/Llama-3.3-70B-Instruct
      resources:
        gpu: 4
        memory: 128Gi
      engineArgs:
        --speculative-model: "meta-llama/Llama-3.2-1B"
        --num-speculative-tokens: "5"
        --speculative-max-model-len: "2048"
```

## Draft Model Selection

- Choose a model from the same family (e.g., Llama-1B for Llama-70B)
- Smaller is better — the draft model should be fast
- Vocabulary must match between draft and main model

## Monitoring

Speculative decoding acceptance rate is visible in the vLLM Model Serving dashboard under token throughput metrics.
```

- [ ] **Step 4: Create docs/kserve-integration.md**

```markdown
# KServe Integration Guide

kube-llmops can coexist with KServe on the same cluster.

## Architecture

```
KServe InferenceService → Istio/Knative → Model Pods
kube-llmops              → LiteLLM      → vLLM/TEI Pods
```

## Coexistence

1. Install KServe and kube-llmops in different namespaces
2. KServe manages models via InferenceService CRDs
3. kube-llmops manages models via Helm values

To route KServe-managed models through kube-llmops gateway:

```yaml
litellm:
  models:
    - name: kserve-model
      apiBase: http://kserve-model.kserve-ns.svc.cluster.local/v1
```

## When to Use Each

- **kube-llmops**: Full LLMOps stack (gateway, monitoring, RAG, fine-tuning)
- **KServe**: When you need Knative serverless scaling or custom transformers
```

- [ ] **Step 5: Create docs/disaggregated-serving.md**

```markdown
# Disaggregated Serving (llm-d)

> **Experimental** — llm-d API is unstable.

## Overview

Disaggregated serving separates LLM inference into **prefill** (compute-heavy) and **decode** (memory-heavy) phases, running them on independent Pods.

## Prerequisites

Install Gateway API CRDs:

```bash
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.2.1/standard-install.yaml
```

## Configuration

```yaml
global:
  models:
    - name: deepseek-r1
      source: deepseek-ai/DeepSeek-R1
      engine: vllm
      disaggregated:
        enabled: true
        prefill:
          replicas: 2
          resources:
            gpu: 4
            memory: 64Gi
        decode:
          replicas: 4
          resources:
            gpu: 2
            memory: 32Gi
```

## Architecture

```
Client → LiteLLM → EPP → Prefill Pod (KV produce)
                       → Decode Pod (KV consume) → Response
```

The Endpoint Picker (EPP) routes requests intelligently based on KV cache state.

## Rendered Resources

- `{name}-prefill` Deployment
- `{name}-decode` Deployment
- `{name}-pool` InferencePool
- `{name}-im` InferenceModel
- `{release}-epp` EPP Deployment
```

- [ ] **Step 6: Create docs/model-updates.md**

```markdown
# Model Updates (Canary Deployment)

## Overview

Update model versions with zero downtime using canary deployments.

## Configuration

```yaml
global:
  models:
    - name: qwen3-8b
      source: Qwen/Qwen3-8B
      replicas: 2
      canary:
        enabled: true
        source: Qwen/Qwen3-8B-v2
        weight: 10
        replicas: 1
```

## Promotion Flow

1. **Deploy canary at 10%**
   ```bash
   helm upgrade kube-llmops charts/kube-llmops-stack \
     --set 'global.models[0].canary.enabled=true' \
     --set 'global.models[0].canary.weight=10'
   ```

2. **Monitor** — Check LiteLLM AI Gateway dashboard for canary vs primary latency

3. **Increase to 50%**
   ```bash
   helm upgrade ... --set 'global.models[0].canary.weight=50'
   ```

4. **Promote** — Update primary source, disable canary
   ```bash
   helm upgrade ... \
     --set 'global.models[0].source=Qwen/Qwen3-8B-v2' \
     --set 'global.models[0].canary.enabled=false'
   ```

## Rollback

Set canary weight to 0 or disable canary:

```bash
helm upgrade ... --set 'global.models[0].canary.enabled=false'
```
```

- [ ] **Step 7: Commit all docs**

```bash
git add docs/routing.md docs/large-model-deployment.md docs/speculative-decoding.md \
       docs/kserve-integration.md docs/disaggregated-serving.md docs/model-updates.md
git commit -m "docs: Phase 5 documentation (routing, large models, canary, llm-d, accelerators)"
```

### Task 26: Update existing docs

**Files:**
- Modify: `ARCHITECTURE.md`
- Modify: `AGENTS.md`
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Update ARCHITECTURE.md Phase 5 section**

Find the Phase 5 roadmap section and update it to reflect implemented features. Check the checkboxes for implemented items and update the feature list to match what was built.

- [ ] **Step 2: Update AGENTS.md**

Add to the test table:

```
| Phase 5 Templates | pytest | 25+ | routing, KEDA multi-trigger, canary, llm-d, accelerator |
```

Add to Key Commands:
```bash
# Run Phase 5 Helm template tests
python -m pytest tests/helm/test_phase5_templates.py -v
```

Add new docs to File Layout.

- [ ] **Step 3: Update CHANGELOG.md**

Add v0.5.0 entry:

```markdown
## [0.5.0] - YYYY-MM-DD

### Added
- Latency-based routing (default strategy, replacing simple-shuffle)
- Prefix caching flag per model (`prefixCaching: true`)
- Multi-trigger KEDA autoscaling (queue depth + TTFT P95 + TPOT P95)
- SLO alert rules (TTFTSLOBreach, TTFTSLOCritical, TPOTSLOBreach)
- Scale-to-zero with LiteLLM fallback for cold start
- Spot/preemptible GPU tolerations (AWS, GCP, Azure, Karpenter)
- Graceful drain (terminationGracePeriodSeconds: 90 + preStop hook)
- MIG GPU device support (nvidia.com/mig-*)
- Canary model deployment with weight-based traffic splitting
- llm-d disaggregated serving (experimental, prefill/decode split)
- Multi-accelerator support (nvidia, amd, gaudi)
- 6 new documentation pages (routing, large models, speculative, kserve, llm-d, canary)
- SLO dashboard panels (TTFT/TPOT vs threshold, HPA replica count)
- Cost dashboard panels (GPU idle rate, scale-to-zero events)
- Canary dashboard panels (latency comparison, traffic weight)
- Prefix cache hit rate panel in vLLM dashboard

### Changed
- Default routing strategy: simple-shuffle → latency-based-routing
- GPU resource names use helper function (supports nvidia/amd/gaudi)
- DCGM exporter conditional on nvidia accelerator
```

- [ ] **Step 4: Commit**

```bash
git add ARCHITECTURE.md AGENTS.md CHANGELOG.md
git commit -m "docs: update ARCHITECTURE, AGENTS, CHANGELOG for v0.5.0"
```

### Task 27: Final verification

- [ ] **Step 1: Run ALL tests**

```bash
python -m pytest tests/helm/test_phase5_templates.py tests/helm/test_finetune_templates.py -v
```
Expected: All tests PASS (60+ total)

- [ ] **Step 2: Full render — default profile**

```bash
helm template test charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml 2>&1 | grep -c '^kind:'
```
Expected: ~150+ resources, no errors

- [ ] **Step 3: Full render — with KEDA + canary**

```bash
helm template test charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  --set keda.enabled=true \
  --set keda.triggers.ttftP95.enabled=true \
  --set 'global.models[0].canary.enabled=true' \
  --set 'global.models[0].canary.source=test/v2' \
  --set 'global.models[0].canary.weight=10' \
  2>&1 | grep -c '^kind:'
```
Expected: More resources than default, no errors

- [ ] **Step 4: Full render — AMD accelerator**

```bash
helm template test charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  --set global.accelerator=amd 2>&1 | head -5
```
Expected: Renders without error

- [ ] **Step 5: Full render — Gaudi accelerator**

```bash
helm template test charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  --set global.accelerator=gaudi 2>&1 | head -5
```
Expected: Renders without error

- [ ] **Step 6: Validate all dashboard JSON files**

```bash
python3 -c "
import json, glob
for f in sorted(glob.glob('charts/kube-llmops-stack/charts/observability/dashboards/*.json')):
    d = json.load(open(f))
    print(f'OK: {f} ({len(d[\"panels\"])} panels)')
"
```
Expected: All 11 files OK

- [ ] **Step 7: Final commit if any outstanding changes**

```bash
git status
# If clean: nothing to do
# If dirty: stage and commit remaining changes
```
