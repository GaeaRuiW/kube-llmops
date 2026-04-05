# Phase 5: Advanced Inference — Design Spec

> Smart routing, SLO guarantees, cost control, disaggregated serving, multi-accelerator support.

**Date:** 2026-04-05
**Status:** Approved
**Scope:** kube-llmops v0.5.0

---

## 1. Intelligent Routing

### 1.1 Latency-Based Routing

**Problem:** LiteLLM uses `simple-shuffle` (round-robin). When a model has multiple replicas, requests hit random Pods regardless of load, causing uneven latency.

**Solution:** Switch default routing strategy to `latency-based-routing` (LiteLLM native).

**Changes:**

| File | Change |
|------|--------|
| `litellm/values.yaml` | New `routingStrategy` field, default `latency-based-routing` |
| `litellm/templates/configmap.yaml` | Render `routing_strategy` + `routing_strategy_args` from values |

**Values schema:**

```yaml
litellm:
  routingStrategy: latency-based-routing   # simple-shuffle | latency-based-routing
  routingStrategyArgs:
    ttl: 60
    lowest_latency_buffer: 0.2
```

**ConfigMap output:**

```yaml
router_settings:
  routing_strategy: latency-based-routing
  routing_strategy_args:
    ttl: 60
    lowest_latency_buffer: 0.2
```

### 1.2 Prefix Cache + Session Affinity

**Problem:** vLLM prefix caching (`--enable-prefix-caching`) is ineffective when round-robin routing sends identical system prompts to different Pods.

**Solution — two layers:**

**Layer 1: vLLM prefix caching flag.**
Add `prefixCaching` option to model config. When enabled, inject `--enable-prefix-caching` into vLLM args.

```yaml
global:
  models:
    - name: qwen3-8b
      prefixCaching: true       # default false, injects --enable-prefix-caching
```

Template logic in `vllm/templates/deployment.yaml`:

```yaml
{{- if $model.prefixCaching }}
            - "--enable-prefix-caching"
{{- end }}
```

**Layer 2: Session affinity via consistent hashing.**
For multi-replica models, route requests with the same `user` field to the same Pod.

LiteLLM does not natively support hash-based routing. Implementation approach:
- Add an optional Envoy sidecar proxy in front of the vLLM Service using consistent hashing on `x-session-id` header
- This is the only supported affinity mechanism (Kubernetes `sessionAffinity: ClientIP` is insufficient because LiteLLM proxies all requests from a single IP)

```yaml
litellm:
  sessionAffinity:
    enabled: false              # opt-in
    header: "x-session-id"     # hash key
```

When `sessionAffinity.enabled`, render an Envoy proxy ConfigMap + Deployment that sits between LiteLLM and vLLM, using `ring_hash` load balancing on the configured header.

**Layer 3: Prefix cache observability.**
Add panels to `dashboards/vllm-overview.json`:
- Prefix cache hit rate: `vllm:prefix_cache_hit_rate{model_name="<name>"}`
- Prefix cache queries: `rate(vllm:prefix_cache_queries_total[5m])`

### 1.3 Verification

- `helm template` renders latency-based-routing in configmap
- `helm template` with `prefixCaching: true` includes `--enable-prefix-caching`
- Deploy, send 10 identical-system-prompt requests, verify via Langfuse traces that latency-based routing distributes load unevenly (favoring faster Pod)
- Grafana prefix cache hit rate panel shows data

---

## 2. SLO-Aware Autoscaling

### 2.1 Multi-Trigger KEDA ScaledObject

**Problem:** KEDA only scales on `num_requests_waiting` (queue depth). Production needs latency SLO guarantees.

**Solution:** Extend ScaledObject template to support multiple Prometheus triggers. Any trigger breaching threshold causes scale-up.

**Values schema:**

```yaml
keda:
  enabled: true
  minReplicas: 1
  maxReplicas: 4
  cooldownPeriod: 300
  pollingInterval: 15
  triggers:
    requestsWaiting:
      enabled: true
      threshold: "2"
    ttftP95:
      enabled: true
      threshold: "3"            # seconds
      window: "2m"
    tpotP95:
      enabled: false            # opt-in
      threshold: "0.1"          # seconds per token
      window: "2m"
  models:                       # per-model overrides
    qwen3-8b:
      maxReplicas: 8
      triggers:
        ttftP95:
          threshold: "2"
```

### 2.2 Template Changes

`keda/templates/scaledobject.yaml` — loop over enabled triggers:

```yaml
triggers:
  {{- if $triggers.requestsWaiting.enabled }}
  - type: prometheus
    metadata:
      serverAddress: http://{{ $prometheusAddr }}
      query: |
        {{ $engineMetric }}{model_name="{{ $name }}"}
      threshold: {{ $triggers.requestsWaiting.threshold | quote }}
  {{- end }}
  {{- if $triggers.ttftP95.enabled }}
  - type: prometheus
    metadata:
      serverAddress: http://{{ $prometheusAddr }}
      query: |
        histogram_quantile(0.95, rate(vllm:time_to_first_token_seconds_bucket{model_name="{{ $name }}"}[{{ $triggers.ttftP95.window }}]))
      threshold: {{ $triggers.ttftP95.threshold | quote }}
  {{- end }}
  {{- if $triggers.tpotP95.enabled }}
  - type: prometheus
    metadata:
      serverAddress: http://{{ $prometheusAddr }}
      query: |
        histogram_quantile(0.95, rate(vllm:time_per_output_token_seconds_bucket{model_name="{{ $name }}"}[{{ $triggers.tpotP95.window }}]))
      threshold: {{ $triggers.tpotP95.threshold | quote }}
  {{- end }}
```

### 2.3 SLO Dashboard Panels

Add to `dashboards/slo-overview.json`:

| Panel | Query |
|-------|-------|
| TTFT P95 vs SLO | `histogram_quantile(0.95, ...)` with threshold line |
| TPOT P95 vs SLO | same pattern |
| SLO Breach Count | count of threshold crossings |
| HPA Replica Count | `kube_horizontalpodautoscaler_status_current_replicas` |

### 2.4 Alert Rules

Add to `observability/templates/prometheus.yaml`:

```yaml
- alert: TTFTSLOBreach
  expr: histogram_quantile(0.95, rate(vllm:time_to_first_token_seconds_bucket[5m])) > 3
  for: 2m
  labels:
    severity: warning
  annotations:
    summary: "P95 TTFT exceeds SLO threshold"

- alert: TTFTSLOCritical
  expr: histogram_quantile(0.95, rate(vllm:time_to_first_token_seconds_bucket[5m])) > 5
  for: 1m
  labels:
    severity: critical
```

### 2.5 Verification

- Helm tests: single trigger, multi-trigger, per-model override all render correctly
- Deploy, `kubectl get scaledobject -o yaml` confirms multi-trigger
- Load test exceeding TTFT threshold triggers KEDA scale-up
- Grafana SLO panels show data with threshold lines

---

## 3. GPU Cost Optimization

### 3.1 Scale-to-Zero

**Problem:** Idle models occupy GPU indefinitely. KEDA supports `minReplicaCount: 0` but LLM cold start is slow (30s–5min).

**Solution:** KEDA scale-to-zero + LiteLLM fallback during cold start.

**Values schema:**

```yaml
keda:
  models:
    qwen2-5-0-5b:
      minReplicas: 0
      scaleToZero:
        enabled: true
        idleTimeout: 900            # 15min idle before scaling to zero
        activationThreshold: "1"    # 1 request wakes up
        fallbackModel: "qwen3-8b"   # serve from this model during cold start
```

**Template changes:**

ScaledObject: set `minReplicaCount: 0`, `idleReplicaCount: 0`. Use KEDA's `activationValue` for wake-up trigger.

LiteLLM configmap: when `scaleToZero.fallbackModel` is set, generate fallback config:

```yaml
- model_name: qwen2-5-0-5b
  litellm_params:
    model: openai/qwen2-5-0-5b
    api_base: ...
  model_info:
    metadata:
      fallbacks: ["qwen3-8b"]
```

### 3.2 Spot/Preemptible GPU Tolerations

**Problem:** No support for scheduling on cheaper Spot/Preemptible GPU nodes.

**Solution:** Per-model `spotToleration` field that injects cloud-agnostic Spot tolerations.

```yaml
global:
  models:
    - name: qwen2-5-0-5b
      spotToleration: true
```

Template logic in `vllm/templates/deployment.yaml` and `llamacpp/templates/deployment.yaml`:

```yaml
{{- if $model.spotToleration }}
      tolerations:
        {{- include "kube-llmops.gpuToleration" $ | nindent 8 }}
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
```

Add graceful drain to all engine Deployments:

```yaml
terminationGracePeriodSeconds: 90
lifecycle:
  preStop:
    exec:
      command: ["/bin/sh", "-c", "sleep 5"]
```

### 3.3 MIG GPU Sharing

**Problem:** Small models (embedding, reranker) don't need a full GPU but occupy one.

**Solution:** Optional `migDevice` field that replaces `nvidia.com/gpu` with a MIG device name.

```yaml
global:
  models:
    - name: bge-small-en
      resources:
        gpu: 0
        migDevice: "nvidia.com/mig-1g.5gb"
        memory: 2Gi
```

Template logic in `_helpers.tpl` and all engine Deployment templates:

```yaml
{{- if $model.resources.migDevice }}
    {{ $model.resources.migDevice }}: 1
{{- else if gt $gpu 0 }}
    nvidia.com/gpu: {{ $gpu }}
{{- end }}
```

Only activates when user explicitly sets `migDevice`. Default behavior unchanged.

### 3.4 Cost Dashboard

Add panels to `dashboards/cost-usage.json`:

| Panel | Metric | Description |
|-------|--------|-------------|
| GPU Idle Rate | `DCGM_FI_PROF_GR_ENGINE_ACTIVE` | Per-model GPU idle percentage |
| Scale-to-Zero Savings | custom recording rule | Hours saved × estimated GPU cost |
| Spot vs On-Demand | node labels | Ratio of Pods on Spot nodes |
| GPU Utilization per Model | DCGM + model label | Identify over-provisioned models |

### 3.5 Verification

- Helm tests: scaleToZero on/off, spotToleration on/off, migDevice all render correctly
- Deploy with `minReplicas: 0`, stop requests for 15min, confirm Pod scales to 0
- Send request, confirm KEDA wakes Pod + LiteLLM fallback serves during cold start
- Cost dashboard panels show data

---

## 4. llm-d Disaggregated Serving (Experimental)

### 4.1 Overview

**Problem:** Monolithic vLLM serving couples prefill (compute-heavy) and decode (memory-heavy). Can't scale independently.

**Solution:** Optional disaggregated mode using llm-d (CNCF sandbox). Feature-gated, zero impact when disabled.

**Risk:** llm-d API is unstable. This is marked **experimental**.

### 4.2 User Interface

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

When `disaggregated.enabled: false` (default), nothing changes. Zero new resources rendered.

### 4.3 New Templates

**`vllm/templates/disaggregated.yaml`** (only when `disaggregated.enabled`):

1. **Prefill Deployment** — vLLM with `--disaggregated-prefill-role=prefill`
2. **Decode Deployment** — vLLM with `--disaggregated-prefill-role=decode`
3. **InferencePool CRD** — `inference.networking.x-k8s.io/v1alpha2`
4. **InferenceModel CRD** — maps model name to pool

**`vllm/templates/epp.yaml`** (only when any model has `disaggregated.enabled`):

Endpoint Picker (EPP) Deployment — the intelligent router that directs requests to prefill or decode Pods based on KV cache state.

```yaml
image: us-central1-docker.pkg.dev/k8s-staging-llm-d/gateway-api-inference-extension/epp:main
args:
  - --poolName={{ $name }}-pool
  - --poolNamespace={{ .Release.Namespace }}
```

### 4.4 LiteLLM Integration

When disaggregated, LiteLLM `api_base` points to Gateway API instead of direct vLLM:

```yaml
{{- if $model.disaggregated.enabled }}
    api_base: http://{{ $name }}-gateway.{{ $.Release.Namespace }}.svc.cluster.local:8080/v1
{{- else }}
    api_base: http://{{ $prefix }}-{{ $name }}.{{ $.Release.Namespace }}.svc.cluster.local:8000/v1
{{- end }}
```

### 4.5 Prerequisites

Gateway API CRDs must be pre-installed:

```bash
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.2.1/standard-install.yaml
```

NOTES.txt warns when `disaggregated.enabled: true` is detected.

### 4.6 Verification

- Helm template: `disaggregated.enabled: false` renders zero llm-d resources
- Helm template: `disaggregated.enabled: true` renders Prefill + Decode Deployments + InferencePool + InferenceModel + EPP
- (Cluster with Gateway API CRDs) Deploy and verify EPP starts, InferencePool ready, requests routed through Gateway

---

## 5. Graceful Model Updates

### 5.1 Canary Model Deployment

**Problem:** Model version updates via `helm upgrade` cause downtime during model loading.

**Solution:** Canary deployment with weight-based traffic splitting in LiteLLM.

**Values schema:**

```yaml
global:
  models:
    - name: qwen3-8b
      source: Qwen/Qwen3-8B
      replicas: 2
      canary:
        enabled: true
        source: Qwen/Qwen3-8B-v2
        weight: 10                  # 10% to canary
        replicas: 1
```

### 5.2 Template Changes

**`vllm/templates/deployment.yaml`** — render additional canary Deployment when `canary.enabled`:

```yaml
{{- if $model.canary.enabled }}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ $prefix }}-{{ $name }}-canary
  labels:
    app.kubernetes.io/component: canary
spec:
  replicas: {{ $model.canary.replicas | default 1 }}
  # Same template as primary but with canary.source
{{- end }}
```

**`vllm/templates/service.yaml`** — canary Service for the canary Deployment.

**`litellm/templates/configmap.yaml`** — two endpoints for same model name with weight:

```yaml
{{- if $model.canary.enabled }}
  - model_name: {{ $model.name }}
    litellm_params:
      model: openai/{{ $model.name }}
      api_base: http://{{ $prefix }}-{{ $model.name }}...
      weight: {{ sub 100 (int $model.canary.weight) }}
  - model_name: {{ $model.name }}
    litellm_params:
      model: openai/{{ $model.name }}-canary
      api_base: http://{{ $prefix }}-{{ $model.name }}-canary...
      weight: {{ $model.canary.weight }}
{{- end }}
```

### 5.3 Promotion Flow

Manual via `helm upgrade`:

1. Deploy canary at 10% → observe Grafana
2. Increase to 50% → observe
3. Promote: update `source` to new version, set `canary.enabled: false`

No automated promotion. Model updates are high-risk and require human judgment.

### 5.4 Dashboard

Add to `dashboards/litellm-gateway.json`:

| Panel | Description |
|-------|-------------|
| Canary vs Primary Latency | P50/P95 comparison |
| Canary vs Primary Error Rate | Error rate comparison |
| Canary Traffic Weight | Current weight distribution |

### 5.5 Verification

- Helm template: `canary.enabled: false` renders only primary (backward compatible)
- Helm template: `canary.enabled: true` renders primary + canary Deployments + Services, LiteLLM configmap has weight fields
- Deploy, send 100 requests, Langfuse traces confirm ~90/10 traffic split
- Grafana canary panels show comparison data

---

## 6. Multi-Accelerator Support

### 6.1 Accelerator Abstraction

**Problem:** Templates hardcode `nvidia.com/gpu` resource name and NVIDIA-specific tolerations.

**Solution:** `global.accelerator` field with helper functions.

```yaml
global:
  accelerator: nvidia           # nvidia | amd | gaudi
```

### 6.2 Helper Functions

New helpers in `_helpers.tpl`:

| Helper | nvidia | amd | gaudi |
|--------|--------|-----|-------|
| `gpuResourceName` | `nvidia.com/gpu` | `amd.com/gpu` | `habana.ai/gaudi` |
| `gpuToleration` | `nvidia.com/gpu` key | `amd.com/gpu` key | `habana.ai/gaudi` key |
| `vllmDefaultImage` | `vllm/vllm-openai:<tag>` | `vllm/vllm-openai:latest-rocm` | `vault.habana.ai/.../vllm-fork:latest` |

User can always override via `vllm.image.repository`/`tag`. Helpers provide sensible defaults.

### 6.3 Engine Argument Defaults

Inject accelerator-specific vLLM defaults when user hasn't set them:

| Arg | NVIDIA | AMD ROCm | Intel Gaudi |
|-----|--------|----------|-------------|
| `--device` | (default cuda) | (default cuda) | `hpu` |
| `--enforce-eager` | optional | recommended | required |
| `--block-size` | 16 (default) | 16 (default) | `128` |
| `--dtype` | `half`/`auto` | `half`/`auto` | `bfloat16` |

### 6.4 GPU Monitoring Adaptation

| Accelerator | Exporter | Key Metric |
|-------------|----------|------------|
| NVIDIA | DCGM Exporter (existing) | `DCGM_FI_DEV_GPU_UTIL` |
| AMD | AMD SMI Exporter | `amd_gpu_use_percent` |
| Intel Gaudi | Habana Metric Exporter | `habana_runtime_metric_compute_utilization` |

Conditional rendering in `observability/templates/`:
- `dcgm-exporter.yaml`: only when `accelerator == nvidia`
- New `gpu-exporter.yaml`: AMD SMI or Habana exporter based on accelerator

GPU dashboard (`gpu-overview.json`) uses template variables to select correct metric names.

### 6.5 Change Scope

All engine Deployment templates (`vllm`, `llamacpp`, `tei`) replace hardcoded `nvidia.com/gpu` with `{{ include "kube-llmops.gpuResourceName" . }}`.

### 6.6 Verification

- Helm template: default (nvidia) renders identically to current (backward compatible)
- Helm template: `accelerator: amd` → `amd.com/gpu` resource, ROCm image, no DCGM
- Helm template: `accelerator: gaudi` → `habana.ai/gaudi` resource, `--device=hpu`, `--block-size=128`
- All 3 accelerators × all values profiles render successfully
- Existing 35 finetune Helm tests still pass (regression)

---

## 7. Documentation

### 7.1 New Docs

| File | Content |
|------|---------|
| `docs/large-model-deployment.md` | Multi-GPU TP, MoE config, memory estimation, quantization |
| `docs/speculative-decoding.md` | Draft model config via engineArgs, selection principles |
| `docs/kserve-integration.md` | Coexistence guide (KServe + kube-llmops gateway) |
| `docs/routing.md` | Routing strategies, prefix caching, session affinity |
| `docs/disaggregated-serving.md` | llm-d architecture, prerequisites, usage |
| `docs/model-updates.md` | Blue-green/canary flow, promotion steps |

### 7.2 Updated Docs

| File | Change |
|------|--------|
| `ARCHITECTURE.md` | Phase 5 roadmap reflects new feature list |
| `AGENTS.md` | New commands, paths, test commands |
| `README.md` / `README.zh-CN.md` | Version banner, feature table, roadmap |
| `CHANGELOG.md` | v0.5.0 entry |

---

## Implementation Order

```
Batch 1: Routing + Autoscaling
  #1  Latency-Based Routing
  #2  Prefix Cache + Session Affinity
  #3  SLO-Aware Autoscaling (multi-trigger KEDA)
  → Verify: helm tests + deploy + load test

Batch 2: Cost Optimization
  #4  Scale-to-Zero
  #5  Spot Tolerations + Graceful Drain
  #6  MIG GPU Sharing
  → Verify: helm tests + scale-to-zero behavior + cost dashboard

Batch 3: Advanced Deployment
  #7  Canary Model Deployment
  #8  llm-d Disaggregated Serving
  → Verify: helm tests + canary weight routing + (optional) llm-d cluster test

Batch 4: Multi-Accelerator + Docs
  #9  Multi-Accelerator (ROCm, Gaudi)
  #10 Documentation (6 new docs + updates)
  → Verify: 3 accelerator × profiles render + regression tests + doc examples
```

Each batch ends with a verification checkpoint. Proceed to next batch only after all tests pass.
