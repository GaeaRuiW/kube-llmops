# curl Examples

API usage examples for kube-llmops (v0.5.0). The default LLM is `gemma-4-26b-a4b`
(llama.cpp GGUF, source `nohurry/gemma-4-26B-A4B-it-heretic-GUFF`, q4_k_m ~16.87 GB).

> Replace `sk-kube-llmops-dev` with your actual master key in production.

## Quick Start — NodePort (no Ingress required)

When the stack is deployed with `--set global.nodePort.enabled=true`, LiteLLM is
exposed on port `30400` of every node. This is the fastest way to hit the gateway
from outside the cluster.

```bash
NODE_IP=$(kubectl get node -o jsonpath='{.items[0].status.addresses[0].address}')

curl -s http://$NODE_IP:30400/v1/chat/completions \
  -H "Authorization: Bearer sk-kube-llmops-dev" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemma-4-26b-a4b",
    "messages": [{"role": "user", "content": "Hello from NodePort!"}]
  }'
```

Other NodePort services: Grafana `:30300`, Langfuse `:30301`, Dify `:30500`,
Keycloak `:30808` (HTTP) / `:30809` (HTTPS), Prometheus `:30909`,
MinIO `:30900`/`:30901`, Headlamp `:30302`, MLflow `:30505`.

## Prerequisites (Ingress mode)

The examples below assume Ingress is enabled with `*.llmops.local`.

```bash
# Add hosts entry (if using local deployment)
NODE_IP=$(kubectl get node -o jsonpath='{.items[0].status.addresses[0].address}')
echo "$NODE_IP litellm.llmops.local" | sudo tee -a /etc/hosts
```

## Chat Completion

```bash
# Basic chat
curl -s http://litellm.llmops.local/v1/chat/completions \
  -H "Authorization: Bearer sk-kube-llmops-dev" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemma-4-26b-a4b",
    "messages": [{"role": "user", "content": "What is Kubernetes?"}],
    "temperature": 0.7,
    "max_tokens": 256
  }'
```

```bash
# Streaming
curl -sN http://litellm.llmops.local/v1/chat/completions \
  -H "Authorization: Bearer sk-kube-llmops-dev" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemma-4-26b-a4b",
    "messages": [{"role": "user", "content": "Explain PagedAttention in 3 sentences."}],
    "stream": true
  }'
```

## Embedding

```bash
curl -s http://litellm.llmops.local/v1/embeddings \
  -H "Authorization: Bearer sk-kube-llmops-dev" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "bge-small-en",
    "input": "kube-llmops deploys LLM infrastructure on Kubernetes"
  }' | python3 -c "import sys,json; d=json.load(sys.stdin); print(f'dims={len(d[\"data\"][0][\"embedding\"])}, model={d[\"model\"]}')"
```

## Reranking

```bash
curl -s http://litellm.llmops.local/v1/rerank \
  -H "Authorization: Bearer sk-kube-llmops-dev" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "bge-reranker-base",
    "query": "What is vLLM?",
    "documents": [
      "vLLM is a high-throughput LLM serving engine with PagedAttention.",
      "Kubernetes is a container orchestration platform.",
      "vLLM supports continuous batching for efficient GPU utilization."
    ],
    "top_n": 2
  }'
```

## Model List

```bash
curl -s http://litellm.llmops.local/v1/models \
  -H "Authorization: Bearer sk-kube-llmops-dev" | python3 -m json.tool
```

## Health Check

```bash
# LiteLLM health
curl -s http://litellm.llmops.local/health/liveliness

# Langfuse health
curl -s http://langfuse.llmops.local/api/public/health

# Prometheus targets
curl -s http://prometheus.llmops.local/api/v1/targets | \
  python3 -c "import sys,json; ts=json.load(sys.stdin)['data']['activeTargets']; [print(f'{t[\"labels\"].get(\"job\",\"?\"):30s} {t[\"health\"]}') for t in ts]"
```

## Langfuse Traces

```bash
# List recent traces
curl -s http://langfuse.llmops.local/api/public/traces?limit=5 \
  -u "pk-lf-kube-llmops:sk-lf-kube-llmops" | python3 -m json.tool
```

## Using with port-forward (no Ingress)

```bash
# Forward LiteLLM
kubectl port-forward svc/kube-llmops-litellm 4000:4000 &

# Then use localhost:4000 instead of litellm.llmops.local
curl -s http://localhost:4000/v1/chat/completions \
  -H "Authorization: Bearer sk-kube-llmops-dev" \
  -H "Content-Type: application/json" \
  -d '{"model":"gemma-4-26b-a4b","messages":[{"role":"user","content":"Hello!"}]}'
```
