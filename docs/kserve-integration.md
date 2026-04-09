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

## Detailed Integration

### Routing KServe Models Through kube-llmops

Add KServe InferenceService endpoints as external models in LiteLLM:

```yaml
litellm:
  embeddingModels:
    - name: kserve-model
      apiBase: http://kserve-model.kserve-ns.svc.cluster.local/v1
      apiKey: "no-key-needed"
```

This routes requests through LiteLLM's gateway, enabling:
- Unified API key management
- Cost tracking and rate limiting
- Langfuse tracing for all models (KServe + native)

### Network Policies

When both systems coexist, ensure KServe pods can communicate:

```yaml
security:
  networkPolicy:
    enabled: true
    # KServe namespaces are excluded from default-deny by label selector
```

kube-llmops NetworkPolicies only affect pods with `app.kubernetes.io/part-of: kube-llmops` labels — KServe pods in other namespaces are unaffected.

### Migration Path

To migrate a KServe model to kube-llmops native serving:

1. Add the model to `global.models` with the same name
2. LiteLLM routes to both (KServe external + native vLLM) with latency-based routing
3. Remove the KServe `embeddingModels` entry once the native deployment is healthy
4. Delete the KServe InferenceService

## Decision Matrix

| Feature | kube-llmops | KServe |
|---------|------------|--------|
| GPU autoscaling | KEDA + Prometheus | KPA / HPA |
| Model format | HuggingFace (auto-download) | Any (user-managed storage) |
| Gateway | LiteLLM (unified) | Istio/Knative |
| Observability | Grafana + Langfuse (integrated) | Separate setup |
| Scale-to-zero | KEDA idleReplicaCount | Knative (built-in) |
| Fine-tuning | Integrated (Argo + LLaMA-Factory) | Not included |
