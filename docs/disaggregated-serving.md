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
