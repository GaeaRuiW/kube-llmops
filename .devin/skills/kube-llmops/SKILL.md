---
name: kube-llmops
description: "Deploy, manage, debug, and query the kube-llmops LLMOps platform — installation, model management, monitoring, RAG, troubleshooting"
argument-hint: "[install|status|models|logs|eval|debug|chat|embed|dashboard]"
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

### `dashboard` — List or Query Grafana Dashboards

If invoked with no argument (`/kube-llmops dashboard`), list all dashboards.
If invoked with a dashboard name or uid (`/kube-llmops dashboard rag-quality`), show that dashboard's panel details and current data.

**Available dashboards (10):**

| UID | Title | Focus |
|-----|-------|-------|
| `vllm-overview` | vLLM Model Serving Overview | Request latency, throughput, KV cache, GPU |
| `litellm-gateway` | LiteLLM AI Gateway | Routing, cost, tokens, rate limiting |
| `gpu-overview` | GPU & Infrastructure Overview | DCGM metrics, utilization, memory |
| `rag-quality` | RAG Quality - Ragas Metrics | Faithfulness, relevancy, precision, recall |
| `cost-usage` | Cost & Usage | Per-model cost tracking |
| `slo-overview` | SLO Overview | Availability, latency, quality SLOs |
| `infra-roi` | Infrastructure ROI | GPU-to-token-to-cost efficiency |
| `tenant-overview` | Tenant Overview | Per-team resource usage |
| `milvus-overview` | Milvus Vector Database | Milvus collection, search, insert metrics |
| `system-overview` | System Overview | Node CPU, memory, disk, network, pod resource table |

**List all dashboards:**

```bash
kubectl exec deploy/kube-llmops-litellm -- python3 -c "
import urllib.request, json, base64
creds = base64.b64encode(b'admin:admin123!').decode()
req = urllib.request.Request('http://kube-llmops-grafana:3000/api/search?type=dash-db',
    headers={'Authorization': f'Basic {creds}'})
dashboards = json.loads(urllib.request.urlopen(req).read())
print(f'Grafana Dashboards ({len(dashboards)} total):')
print(f'{\"UID\":22s} {\"Title\":50s} {\"URL\":<s}')
print('-' * 100)
NODE_IP = '$(kubectl get node -o jsonpath=\"{.items[0].status.addresses[0].address}\")'
for d in dashboards:
    url = f'http://{NODE_IP}:30300/d/{d[\"uid\"]}'
    print(f'{d[\"uid\"]:22s} {d[\"title\"]:50s} {url}')
"
```

**Get a specific dashboard's panels and live data:**

Use the `$ARGUMENTS` after `dashboard` as the UID. For example: `/kube-llmops dashboard rag-quality`

```bash
DASH_UID="${ARGUMENTS#dashboard }"
DASH_UID="${DASH_UID:-rag-quality}"   # default

kubectl exec deploy/kube-llmops-litellm -- python3 -c "
import urllib.request, json, base64

uid = '$DASH_UID'.strip()
creds = base64.b64encode(b'admin:admin123!').decode()

# Get dashboard definition
req = urllib.request.Request(f'http://kube-llmops-grafana:3000/api/dashboards/uid/{uid}',
    headers={'Authorization': f'Basic {creds}'})
try:
    data = json.loads(urllib.request.urlopen(req).read())
except Exception as e:
    print(f'Dashboard \"{uid}\" not found. Use /kube-llmops dashboard to list all.')
    exit(1)

dash = data['dashboard']
print(f'Dashboard: {dash[\"title\"]}')
print(f'UID: {uid}')
print(f'Panels: {len(dash.get(\"panels\",[]))}')
print()

# Show each panel and query its live data
for panel in dash.get('panels', []):
    pid = panel.get('id', '?')
    title = panel.get('title', 'Untitled')
    ptype = panel.get('type', '?')
    print(f'  Panel {pid}: {title} ({ptype})')

    # Try to query each target's expr from Prometheus
    for t in panel.get('targets', []):
        expr = t.get('expr', '')
        if not expr:
            continue
        try:
            prom_url = f'http://kube-llmops-prometheus:9090/api/v1/query?query={urllib.parse.quote(expr)}'
            pr = json.loads(urllib.request.urlopen(prom_url, timeout=3).read())
            results = pr.get('data', {}).get('result', [])
            if results:
                for r in results[:3]:
                    labels = ', '.join(f'{k}={v}' for k, v in r['metric'].items() if k != '__name__')
                    val = r['value'][1]
                    legend = t.get('legendFormat', '').replace('{{ \"{{\" }}', '').replace('{{ \"}}\" }}', '') or labels
                    print(f'    {legend}: {val}')
            else:
                print(f'    {expr}: (no data)')
        except:
            print(f'    {expr}: (query failed)')
    print()
"
```

**Open a dashboard in browser (print URL):**

```bash
NODE_IP=$(kubectl get node -o jsonpath='{.items[0].status.addresses[0].address}')
echo "Open: http://$NODE_IP:30300/d/${DASH_UID:-rag-quality}"
```

---

### No subcommand / general question

If the user doesn't provide a subcommand, or asks a general question about kube-llmops, answer using the project knowledge in AGENTS.md and the codebase. Common tasks:

- **Upgrade**: `helm upgrade kube-llmops charts/kube-llmops-stack -f values-single-node.yaml --no-hooks`
- **Add model**: Edit `global.models` in values-single-node.yaml, then upgrade
- **Change port**: `--set global.nodePort.grafana=31000`
- **Enable HF token**: `--set global.hfToken=hf_xxx`
- **Enable NodePort**: `--set global.nodePort.enabled=true --set global.nodePort.host=<NODE_IP>`
- **Build model-loader**: `docker build -t kube-llmops/model-loader:latest images/model-loader/`
- **Run E2E tests**: `uv run tests/e2e/test_dify_model_provider.py`
- **Check smoke test**: `kubectl logs -l app.kubernetes.io/name=rag-smoke-test --tail=30`
- **System metrics**: node-exporter (CPU/mem/disk) + kube-state-metrics (pod/deploy status)
- **10 dashboards**: vllm, litellm, gpu, rag-quality, cost, slo, infra-roi, tenant, milvus, system-overview
