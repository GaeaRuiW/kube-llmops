# Disaggregated Serving (llm-d)

> **Experimental** -- llm-d API is unstable.

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
Client -> LiteLLM -> EPP -> Prefill Pod (KV produce)
                       -> Decode Pod (KV consume) -> Response
```

The Endpoint Picker (EPP) routes requests intelligently based on KV cache state.

## Rendered Resources

- `{name}-prefill` Deployment
- `{name}-decode` Deployment
- `{name}-pool` InferencePool
- `{name}-im` InferenceModel
- `{release}-epp` EPP Deployment

## When to Use Disaggregated Serving

Disaggregated serving (llm-d) separates the prefill and decode phases into independent services, each scaling independently.

**Use when:**
- Long input prompts (RAG context, documents) dominate prefill time
- Decode latency is critical (real-time chat, streaming)
- Different scaling profiles needed: many prefill workers for batch, fewer decode workers for streaming

**Do NOT use when:**
- Simple single-model deployment with uniform workload
- Model fits on a single GPU with acceptable latency
- Latency requirements are relaxed (batch processing)

## Performance Tuning

| Parameter | Effect | Recommendation |
|-----------|--------|----------------|
| `prefill.replicas` | Handles input tokenization + KV cache computation | Scale with input length and concurrency |
| `decode.replicas` | Handles autoregressive token generation | Scale with output length and streaming users |
| `prefill.resources.gpu` | GPU memory for KV cache computation | Match full model size |
| `decode.resources.gpu` | GPU memory for generation | Match full model size |

## Monitoring

Key metrics on the **vLLM Model Serving** Grafana dashboard:
- **Prefill latency**: Time to process input tokens (prefill pods)
- **Decode TPOT**: Time per output token (decode pods)
- **Queue depth**: Pending requests per phase

The EPP (Endpoint Picker) routes requests based on KV cache locality — requests with similar prefixes are routed to the same prefill pod.

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| High prefill latency | Insufficient prefill replicas | Increase `prefill.replicas` |
| Decode timeouts | KV cache transfer bottleneck | Check network bandwidth between pods |
| EPP routing errors | Gateway API CRDs not installed | Install Gateway API CRDs (see Prerequisites) |
| Pods stuck in Pending | No GPU nodes available | Check node GPU allocations |
