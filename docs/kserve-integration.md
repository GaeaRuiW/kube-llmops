# KServe Integration Guide

kube-llmops can coexist with KServe on the same cluster.

## Architecture

```
KServe InferenceService -> Istio/Knative -> Model Pods
kube-llmops              -> LiteLLM      -> vLLM/TEI Pods
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
