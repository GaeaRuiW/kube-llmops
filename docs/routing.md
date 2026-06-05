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

## Fallback Routing

Configure model fallback chains for redundancy:

```yaml
litellm:
  routerSettings:
    fallbacks:
      - qwen2-5-0-5b: [qwen3-8b]
      - llama-7b: [llama-13b, gpt-4]
```

When a model is unavailable (e.g., scaled to zero), LiteLLM routes to the next model in the chain.

### Scale-to-Zero Fallback

KEDA can scale models to zero replicas during idle periods. True scale-from-zero uses the KEDA HTTP add-on interceptor, because model-serving metrics such as `vllm:num_requests_waiting` disappear when the serving Deployment is at 0 replicas.

```yaml
global:
  models:
    - name: qwen2-5-0-5b
      source: Qwen/Qwen2.5-0.5B-Instruct
      scaleToZero:
        enabled: true
        fallbackModel: "qwen3-8b" # optional LiteLLM fallback during cold start

keda:
  enabled: true
  models:
    qwen2-5-0-5b:
      scaleToZero:
        idleTimeout: 900          # 15 min idle before scale-down
```

When enabled, LiteLLM routes the model to a per-model ExternalName alias for the KEDA HTTP add-on interceptor. The interceptor holds the first request while KEDA scales the backend Deployment from 0 to 1, then forwards the unchanged OpenAI-compatible request path to the real model Service.

Install both KEDA core and the HTTP add-on before enabling this:

```bash
helm repo add kedacore https://kedacore.github.io/charts
helm install keda kedacore/keda --namespace keda-system --create-namespace
helm install http-add-on kedacore/keda-add-ons-http --namespace keda-system
```

`keda.models.<name>.scaleToZero.enabled` is intentionally not supported as the activation switch, because LiteLLM cannot see sibling `keda.*` values. Put `enabled` and `fallbackModel` under `global.models[].scaleToZero`; keep KEDA-only tuning such as `idleTimeout` under `keda.models.<name>.scaleToZero`.

## Canary Routing

Split traffic between stable and canary model versions:

```yaml
global:
  models:
    - name: my-model
      source: org/model-v1
      canary:
        enabled: true
        source: org/model-v2
        weight: 10              # 10% to canary, 90% to stable
```

Promote by increasing weight to 100, then update `source` and remove the `canary` block.

## Monitoring Routing Decisions

Key Grafana panels for routing:
- **LiteLLM AI Gateway** dashboard: request distribution per model deployment
- **vLLM Model Serving** dashboard: per-replica latency, queue depth
- **SLO Overview** dashboard: TTFT/TPOT P95 per model

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| All requests go to one replica | Routing strategy is `simple-shuffle` | Switch to `latency-based-routing` |
| High latency variance | No prefix caching | Enable `prefixCaching: true` on the model |
| Cold start timeouts | No fallback for scale-to-zero | Add `scaleToZero.fallbackModel` |
| Session affinity not working | Missing `x-session-id` header | Client must set the header on each request |
