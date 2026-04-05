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
