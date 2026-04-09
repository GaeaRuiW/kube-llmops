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

KEDA can scale models to zero replicas during idle periods. Configure a fallback model for cold-start coverage:

```yaml
keda:
  models:
    qwen2-5-0-5b:
      scaleToZero:
        enabled: true
        idleTimeout: 900          # 15 min idle before scale-down
        fallbackModel: "qwen3-8b" # serves requests during cold start
```

The fallback is auto-injected into LiteLLM's config — no manual `fallbacks` entry needed.

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
