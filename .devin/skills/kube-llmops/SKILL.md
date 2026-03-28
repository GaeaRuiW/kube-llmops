---
name: kube-llmops
description: "Deploy, manage, debug, and query the kube-llmops LLMOps platform — installation, model management, monitoring, RAG, troubleshooting"
argument-hint: "[install|status|models|logs|eval|debug|chat|embed]"
allowed-tools:
  - read
  - grep
  - glob
  - exec
  - edit
permissions:
  allow:
    - Read(**)
    - Exec(helm *)
    - Exec(kubectl *)
    - Exec(curl *)
    - Exec(docker *)
    - Write(charts/**)
    - Write(values*.yaml)
---

# kube-llmops Skill

You are an expert operator for the **kube-llmops** Kubernetes-native LLMOps platform.
This skill covers the full lifecycle: installation, model management, monitoring, RAG, evaluation, and troubleshooting.

## Project Context

@AGENTS.md

## Architecture

```
┌─ Ingress / NodePort ─────────────────────────────┐
│  LiteLLM (Gateway:4000) → vLLM (LLM:8000)        │
│                          → TEI (Embed:8080)        │
│                          → TEI (Rerank:8080)       │
│  Dify (RAG:5001/3000) → LiteLLM → pgvector        │
│  Langfuse (Trace:3000) ← LiteLLM callbacks         │
│  Prometheus:9090 + Pushgateway:9091 → Grafana:3000  │
│  LLM-Guard (Security:8000), Keycloak (SSO:8080)    │
│  MinIO (S3:9000) + PostgreSQL:5432                  │
└──────────────────────────────────────────────────┘
```

## Commands

The user may invoke this skill with a subcommand: `$ARGUMENTS`

Handle each subcommand as follows:

---

### `install` — Fresh Installation

1. Verify prerequisites:
   ```bash
   kubectl cluster-info
   helm version
   nvidia-smi   # optional, for GPU
   ```

2. Get the node IP:
   ```bash
   NODE_IP=$(kubectl get node -o jsonpath='{.items[0].status.addresses[0].address}')
   echo "Node IP: $NODE_IP"
   ```

3. Build model-loader image (if not exists):
   ```bash
   docker images | grep model-loader || docker build -t kube-llmops/model-loader:latest images/model-loader/
   ```

4. Install with NodePort access:
   ```bash
   cd charts/kube-llmops-stack && rm -f charts/*.tgz Chart.lock && helm dependency update .
   helm install kube-llmops charts/kube-llmops-stack \
     -f charts/kube-llmops-stack/values-single-node.yaml \
     --set global.nodePort.enabled=true \
     --set global.nodePort.host=$NODE_IP \
     --timeout 10m
   ```

5. Wait for pods and print access info:
   ```bash
   kubectl get pods -l app.kubernetes.io/part-of=kube-llmops --watch
   ```

6. Print the access URLs with NODE_IP:
   ```
   LiteLLM:    http://<NODE_IP>:30400
   Grafana:    http://<NODE_IP>:30300    (admin / admin123!)
   Langfuse:   http://<NODE_IP>:30301    (admin@kube-llmops.local / admin123!)
   Dify:       http://<NODE_IP>:30500    (admin@kube-llmops.local / Admin123!)
   Keycloak:   http://<NODE_IP>:30808    (admin / admin123!)
   Prometheus: http://<NODE_IP>:30909
   MinIO:      http://<NODE_IP>:30900    (minioadmin / minioadmin)
   ```

---

### `status` — Check Cluster Health

Run these checks and report a summary:

```bash
# Pod status
kubectl get pods -l app.kubernetes.io/part-of=kube-llmops -o wide --no-headers | awk '{printf "%-50s %s\n", $1, $3}'

# Key services
kubectl get svc | grep -E "\-np|litellm|grafana|langfuse"

# vLLM model ready?
kubectl exec deploy/kube-llmops-litellm -- python3 -c "
import urllib.request, json
try:
    r = json.loads(urllib.request.urlopen('http://localhost:4000/v1/models', timeout=5).read())
    print(f'Models available: {[m[\"id\"] for m in r[\"data\"]]}')
except: print('LiteLLM not ready yet')
"

# MinIO model cache
kubectl exec deploy/kube-llmops-litellm -- pip install -q minio 2>/dev/null
kubectl exec deploy/kube-llmops-litellm -- python3 -c "
from minio import Minio
c = Minio('kube-llmops-minio:9000', access_key='minioadmin', secret_key='minioadmin', secure=False)
if c.bucket_exists('models'):
    for p in set(o.object_name.split('/')[0] for o in c.list_objects('models', recursive=True)):
        print(f'  s3://models/{p}/')
"

# Ragas metrics
kubectl exec deploy/kube-llmops-litellm -- python3 -c "
import urllib.request, json
for m in ['ragas_faithfulness','ragas_answer_relevancy','ragas_context_precision','ragas_context_recall']:
    r = json.loads(urllib.request.urlopen(f'http://kube-llmops-prometheus:9090/api/v1/query?query={m}').read())
    v = r['data']['result'][0]['value'][1] if r['data']['result'] else 'N/A'
    print(f'  {m}: {v}')
" 2>/dev/null
```

---

### `models` — List and Manage Models

Show current models and their auto-detected engines:

```bash
# From values
grep -A5 "source:" charts/kube-llmops-stack/values-single-node.yaml | grep -E "name:|source:"

# Running deployments
kubectl get deploy | grep -E "vllm-|tei-|llamacpp-"

# Explain engine auto-detection
# The resolveEngine helper in _helpers.tpl detects engine from source name:
#   *GGUF* → llamacpp
#   *rerank* → tei
#   bge-*/e5-*/gte-*/embedding* → tei
#   everything else → vllm
```

To add a new model, tell the user to add it to `global.models` in values:
```yaml
global:
  models:
    - name: my-new-model
      source: org/model-name     # engine auto-detected
      resources:
        gpu: 1
        memory: 16Gi
```

Then upgrade:
```bash
helm upgrade kube-llmops charts/kube-llmops-stack -f charts/kube-llmops-stack/values-single-node.yaml --no-hooks
```

---

### `logs` — View Component Logs

```bash
# Specify component or show menu
# Components: vllm, tei, litellm, langfuse, dify, grafana, keycloak, prometheus, minio

# Example for a specific model:
kubectl logs deploy/vllm-qwen2-5-0-5b --tail=50
kubectl logs deploy/tei-bge-small-en --tail=50
kubectl logs deploy/kube-llmops-litellm --tail=50

# Model loader logs (init container):
kubectl logs deploy/tei-bge-small-en -c model-loader --tail=30
kubectl logs deploy/vllm-qwen2-5-0-5b -c model-loader --tail=30
```

---

### `eval` — Trigger RAG Evaluation

```bash
# Trigger manual Ragas evaluation
kubectl create job ragas-manual-$(date +%s) --from=cronjob/kube-llmops-ragas-eval

# Watch progress
kubectl logs -l job-name -f --tail=20

# Check results
kubectl exec deploy/kube-llmops-litellm -- python3 -c "
import urllib.request, json
for m in ['ragas_faithfulness','ragas_answer_relevancy','ragas_context_precision','ragas_context_recall']:
    r = json.loads(urllib.request.urlopen(f'http://kube-llmops-prometheus:9090/api/v1/query?query={m}').read())
    v = float(r['data']['result'][0]['value'][1]) if r['data']['result'] else 0
    status = 'PASS' if v >= 0.7 else 'FAIL'
    print(f'  {m:35s} {v:.4f}  [{status}]')
"
```

---

### `debug` — Troubleshooting

Run comprehensive diagnostics:

```bash
echo "=== Unhealthy Pods ==="
kubectl get pods -l app.kubernetes.io/part-of=kube-llmops | grep -v "Running\|Completed" | grep -v NAME

echo ""
echo "=== Recent Events ==="
kubectl get events --sort-by=.lastTimestamp --field-selector type!=Normal | tail -10

echo ""
echo "=== Resource Usage ==="
kubectl top pods -l app.kubernetes.io/part-of=kube-llmops --no-headers 2>/dev/null | sort -k3 -rh | head -10

echo ""
echo "=== GPU Status ==="
nvidia-smi --query-gpu=name,memory.used,memory.total,utilization.gpu --format=csv,noheader 2>/dev/null || echo "No GPU or nvidia-smi not available"

echo ""
echo "=== PVC Usage ==="
kubectl get pvc | grep -E "vllm-|tei-|llamacpp-|minio|prometheus"

echo ""
echo "=== Prometheus Alerts Firing ==="
kubectl exec deploy/kube-llmops-litellm -- python3 -c "
import urllib.request, json
r = json.loads(urllib.request.urlopen('http://kube-llmops-prometheus:9090/api/v1/alerts').read())
for a in r['data']['alerts']:
    if a['state'] == 'firing':
        print(f'  FIRING: {a[\"labels\"][\"alertname\"]} - {a[\"annotations\"].get(\"summary\",\"\")}')
" 2>/dev/null || echo "  Cannot reach Prometheus"
```

Common fixes:
- **vLLM OOMKilled**: Increase `resources.memory` or add `--max-model-len`
- **TEI CrashLoop**: Check model-loader logs: `kubectl logs <pod> -c model-loader`
- **LiteLLM 500**: Check config: `kubectl get cm kube-llmops-litellm-config -o yaml`
- **Dify 401**: Cookie issue — must use single-domain routing (path-based Ingress or NodePort)
- **Helm .tgz stale**: `cd charts/kube-llmops-stack && rm -f charts/*.tgz Chart.lock && helm dependency update .`

---

### `chat` — Quick Chat Test

```bash
NODE_IP=$(kubectl get node -o jsonpath='{.items[0].status.addresses[0].address}')
curl -s http://$NODE_IP:30400/v1/chat/completions \
  -H "Authorization: Bearer sk-kube-llmops-dev" \
  -H "Content-Type: application/json" \
  -d '{"model":"qwen2-5-0-5b","messages":[{"role":"user","content":"'"$ARGUMENTS"'"}],"max_tokens":256}' | python3 -m json.tool
```

If no message provided, use "Hello, what can you do?" as default.

---

### `embed` — Quick Embedding Test

```bash
NODE_IP=$(kubectl get node -o jsonpath='{.items[0].status.addresses[0].address}')
curl -s http://$NODE_IP:30400/v1/embeddings \
  -H "Authorization: Bearer sk-kube-llmops-dev" \
  -H "Content-Type: application/json" \
  -d '{"model":"bge-small-en","input":"'"${ARGUMENTS:-kube-llmops test embedding}"'"}' | python3 -c "
import sys, json
d = json.load(sys.stdin)
v = d['data'][0]['embedding']
print(f'Model: {d[\"model\"]}')
print(f'Dimensions: {len(v)}')
print(f'First 5: {v[:5]}')
"
```

---

### No subcommand / general question

If the user doesn't provide a subcommand, or asks a general question about kube-llmops, answer using the project knowledge in AGENTS.md and the codebase. Common tasks:

- **Upgrade**: `helm upgrade kube-llmops charts/kube-llmops-stack -f values-single-node.yaml --no-hooks`
- **Add model**: Edit `global.models` in values-single-node.yaml, then upgrade
- **Change port**: `--set global.nodePort.grafana=31000`
- **Enable HF token**: `--set global.hfToken=hf_xxx`
- **Run E2E tests**: `uv run tests/e2e/test_dify_model_provider.py`
- **Check smoke test**: `kubectl logs -l app.kubernetes.io/name=rag-smoke-test --tail=30`
