# Getting Started with kube-llmops

This guide walks you through installing kube-llmops, verifying the deployment, and making your first LLM API call.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Install](#quick-install)
  - [One-Liner Install](#one-liner-install)
  - [Manual Install](#manual-install)
- [Choose a Profile](#choose-a-profile)
- [Verify Installation](#verify-installation)
- [Send Your First Request](#send-your-first-request)
- [Access the UIs](#access-the-uis)
- [GPU-Specific Tuning](#gpu-specific-tuning)
- [Customization](#customization)
- [Troubleshooting](#troubleshooting)
- [Uninstall](#uninstall)

---

## Prerequisites

| Requirement | Version | Notes |
|---|---|---|
| Kubernetes | 1.28+ | Any distribution: AKS, EKS, GKE, k3s, kind, minikube |
| Helm | 3.x | `brew install helm` or [install guide](https://helm.sh/docs/intro/install/) |
| kubectl | 1.28+ | Must be configured to talk to your cluster |
| GPU node | Optional | NVIDIA GPU with [GPU Operator](https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/) installed |

**Verify prerequisites:**

```bash
# Check versions
helm version --short
kubectl version --client --short

# Check cluster connectivity
kubectl cluster-info

# Check GPU availability (optional)
kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.capacity.nvidia\.com/gpu}{"\n"}{end}'
```

No GPU? No problem — use the `ci` profile which runs a tiny CPU-only model.

---

## Quick Install

### One-Liner Install

The fastest way to get started. This clones the repo into a temp directory, installs the Helm chart, and cleans up:

```bash
curl -sfL https://raw.githubusercontent.com/GaeaRuiW/kube-llmops/main/scripts/install.sh | bash
```

**Customize with environment variables:**

```bash
# CPU-only demo (no GPU required)
KUBE_LLMOPS_PROFILE=ci \
  curl -sfL https://raw.githubusercontent.com/GaeaRuiW/kube-llmops/main/scripts/install.sh | bash

# Custom namespace and release name
KUBE_LLMOPS_NAMESPACE=llmops \
KUBE_LLMOPS_RELEASE=my-llm \
KUBE_LLMOPS_PROFILE=standard \
  curl -sfL https://raw.githubusercontent.com/GaeaRuiW/kube-llmops/main/scripts/install.sh | bash
```

| Variable | Default | Description |
|---|---|---|
| `KUBE_LLMOPS_PROFILE` | `minimal` | Deployment profile: `ci`, `minimal`, `standard` |
| `KUBE_LLMOPS_NAMESPACE` | `default` | Kubernetes namespace |
| `KUBE_LLMOPS_RELEASE` | `kube-llmops` | Helm release name |
| `KUBE_LLMOPS_BRANCH` | `main` | Git branch to install from |

### Manual Install

If you prefer more control:

```bash
# 1. Clone the repo
git clone https://github.com/GaeaRuiW/kube-llmops.git
cd kube-llmops

# 2. Update Helm dependencies
helm dependency update charts/kube-llmops-stack

# 3. Install with your chosen profile
helm upgrade --install kube-llmops charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-minimal.yaml \
  --namespace default \
  --create-namespace \
  --wait --timeout 10m
```

---

## Choose a Profile

kube-llmops ships with pre-configured profiles for different environments:

| Profile | File | GPU | Models | Monitoring | Tracing | Best For |
|---|---|---|---|---|---|---|
| **ci** | `values-ci.yaml` | None | Tiny CPU model | Basic Prometheus + Grafana | Off | CI pipelines, quick demos |
| **minimal** | `values-minimal.yaml` | 1x | 1 small model (Qwen2.5-0.5B) | Prometheus + Grafana | Off | Development, learning |
| **standard** | `values-standard.yaml` | 4-8x | 2-3 models | Full OTel stack | Langfuse | Team / staging |

**How to choose:**

- **No GPU or just want to try it?** → `ci`
- **Single GPU dev machine?** → `minimal`
- **Multi-GPU team environment?** → `standard`

Switch profiles at any time by re-running `helm upgrade`:

```bash
helm upgrade kube-llmops charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-standard.yaml \
  --namespace default
```

---

## Verify Installation

### Check Pod Status

```bash
kubectl get pods -n default
```

Expected output (minimal profile):

```
NAME                                        READY   STATUS    RESTARTS   AGE
kube-llmops-litellm-0                       1/1     Running   0          2m
kube-llmops-litellm-postgresql-0            1/1     Running   0          2m
kube-llmops-vllm-qwen2-5-0-5b-xxx          1/1     Running   0          3m
kube-llmops-prometheus-server-xxx           2/2     Running   0          2m
kube-llmops-grafana-xxx                     1/1     Running   0          2m
kube-llmops-otel-collector-xxx              1/1     Running   0          2m
```

> **Note:** vLLM pods may take 3-10 minutes to become Ready as they download the model on first start. Use `kubectl logs -f` to monitor progress.

### Check Model Readiness

```bash
# Watch vLLM pod logs until you see "Uvicorn running on http://0.0.0.0:8000"
kubectl logs -f deployment/kube-llmops-vllm-qwen2-5-0-5b -n default
```

### Check Helm Release Status

```bash
helm status kube-llmops -n default
helm get values kube-llmops -n default
```

---

## Send Your First Request

Once all pods are Running, port-forward the LiteLLM gateway and send a chat completion:

```bash
# Port-forward LiteLLM
kubectl port-forward svc/kube-llmops-litellm 4000:4000 -n default &

# Send a request
curl http://localhost:4000/v1/chat/completions \
  -H "Authorization: Bearer sk-kube-llmops-dev" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen2-5-0-5b",
    "messages": [{"role": "user", "content": "Hello! What is Kubernetes?"}],
    "max_tokens": 256
  }'
```

Expected response (truncated):

```json
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "model": "qwen2-5-0-5b",
  "choices": [
    {
      "message": {
        "role": "assistant",
        "content": "Kubernetes is an open-source container orchestration platform..."
      }
    }
  ],
  "usage": {
    "prompt_tokens": 12,
    "completion_tokens": 64,
    "total_tokens": 76
  }
}
```

### Use with OpenAI Python SDK

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:4000/v1",
    api_key="sk-kube-llmops-dev",
)

response = client.chat.completions.create(
    model="qwen2-5-0-5b",
    messages=[{"role": "user", "content": "Hello! What is Kubernetes?"}],
    max_tokens=256,
)

print(response.choices[0].message.content)
```

---

## Access the UIs

Port-forward each service to access the web UIs:

```bash
# AI Gateway (LiteLLM)
kubectl port-forward svc/kube-llmops-litellm 4000:4000 -n default &

# Dashboards (Grafana)
kubectl port-forward svc/kube-llmops-grafana 3000:3000 -n default &

# LLM Tracing (Langfuse) — standard profile only
kubectl port-forward svc/kube-llmops-langfuse 3001:3000 -n default &
```

### Default Credentials

| Service | URL | Username | Password |
|---|---|---|---|
| **LiteLLM** (AI Gateway) | [http://localhost:4000/ui](http://localhost:4000/ui) | any username | `sk-kube-llmops-dev` |
| **Grafana** (Dashboards) | [http://localhost:3000](http://localhost:3000) | `admin` | `admin` |
| **Langfuse** (LLM Tracing) | [http://localhost:3001](http://localhost:3001) | `admin@kube-llmops.local` | `admin123!` |

> **⚠️ Security Warning:** These are development defaults. For production deployments, always override credentials:
>
> ```bash
> helm upgrade kube-llmops charts/kube-llmops-stack \
>   --set litellm.masterKey=sk-your-production-key \
>   --set observability.grafana.adminPassword=your-secure-password \
>   --set langfuse.init.userPassword=your-secure-password
> ```

### Grafana Dashboards

kube-llmops ships with 3 pre-provisioned Grafana dashboards:

1. **vLLM Engine** — Request throughput, TTFT, token latency, KV cache utilization
2. **LiteLLM Gateway** — API request rates, latency by model, error rates, cost tracking
3. **GPU Metrics** — GPU utilization, memory usage, temperature, power draw (requires DCGM Exporter)

Navigate to **Dashboards → Browse** in Grafana to find them.

---

## GPU-Specific Tuning

### NVIDIA Blackwell GPUs (B200, GB200)

FlashAttention 2 may hang on Blackwell (SM 12.0) GPUs. Add the TRITON_ATTN workaround:

```yaml
# In your values override file
models:
  - name: your-model
    engineArgs:
      - "--attention-backend"
      - "TRITON_ATTN"
```

Or via `--set`:

```bash
helm upgrade kube-llmops charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-minimal.yaml \
  --set 'models[0].engineArgs[0]=--attention-backend' \
  --set 'models[0].engineArgs[1]=TRITON_ATTN'
```

### GPU Memory Utilization

By default, vLLM uses 90% of GPU memory. Adjust per model:

```yaml
models:
  - name: your-model
    engineArgs:
      - "--gpu-memory-utilization"
      - "0.85"
```

### Multi-GPU (Tensor Parallelism)

For large models that don't fit on a single GPU:

```yaml
models:
  - name: large-model
    engine: vllm
    resources:
      limits:
        nvidia.com/gpu: "4"
    engineArgs:
      - "--tensor-parallel-size"
      - "4"
```

---

## Customization

### Add a New Model

Edit your values file or create an override:

```yaml
# my-values.yaml
models:
  - name: qwen2-5-0-5b
    modelId: Qwen/Qwen2.5-0.5B-Instruct
    engine: vllm
    enabled: true
    replicas: 1
    resources:
      limits:
        nvidia.com/gpu: "1"
      requests:
        memory: "4Gi"
        cpu: "2"

  - name: llama3-1-8b
    modelId: meta-llama/Llama-3.1-8B-Instruct
    engine: vllm
    enabled: true
    replicas: 1
    resources:
      limits:
        nvidia.com/gpu: "1"
      requests:
        memory: "16Gi"
        cpu: "4"
    extraEnv:
      - name: HF_TOKEN
        valueFrom:
          secretKeyRef:
            name: hf-token
            key: token
```

Then upgrade:

```bash
helm upgrade kube-llmops charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-minimal.yaml \
  -f my-values.yaml \
  --namespace default
```

### Change Resource Limits

Override resources for any component:

```yaml
# my-values.yaml
litellm:
  resources:
    requests:
      cpu: "500m"
      memory: "512Mi"
    limits:
      cpu: "2"
      memory: "2Gi"

observability:
  prometheus:
    server:
      resources:
        requests:
          memory: "1Gi"
        limits:
          memory: "4Gi"
```

### Enable Optional Components

Components like Langfuse, DCGM Exporter, and Loki are toggled per profile. Enable them individually:

```bash
# Enable Langfuse tracing
helm upgrade kube-llmops charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-minimal.yaml \
  --set langfuse.enabled=true

# Enable DCGM GPU metrics
helm upgrade kube-llmops charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-minimal.yaml \
  --set dcgmExporter.enabled=true

# Enable Loki logging
helm upgrade kube-llmops charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-minimal.yaml \
  --set loki.enabled=true \
  --set fluentBit.enabled=true
```

### Use a Private Model Registry (Hugging Face Token)

For gated models that require authentication:

```bash
# Create the secret
kubectl create secret generic hf-token \
  --from-literal=token=hf_your_token_here \
  -n default

# Reference it in your model config
# (see "Add a New Model" above for extraEnv example)
```

---

## Troubleshooting

### Pods Stuck in `Pending`

**Symptom:** Model serving pods stay in `Pending` state.

```bash
kubectl describe pod <pod-name> -n default
```

**Common causes:**

- **Insufficient GPU:** The node doesn't have enough GPUs. Check `kubectl describe node <node>` for GPU capacity.
- **No GPU plugin:** Install the [NVIDIA GPU Operator](https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/) or [device plugin](https://github.com/NVIDIA/k8s-device-plugin).
- **Resource limits too high:** Reduce `resources.requests` in your values file.

**Fix for no GPU (demo/CI):**

```bash
helm upgrade kube-llmops charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-ci.yaml \
  --namespace default
```

### vLLM Pod CrashLoopBackOff

**Symptom:** vLLM pod restarts repeatedly.

```bash
kubectl logs deployment/kube-llmops-vllm-qwen2-5-0-5b -n default --previous
```

**Common causes:**

- **OOM (Out of Memory):** Model is too large for the GPU. Try `--gpu-memory-utilization 0.80` or a smaller model.
- **CUDA version mismatch:** Ensure the vLLM image matches your CUDA driver version.
- **Model download failed:** Check if HF_TOKEN is needed for gated models, or if the model ID is correct.
- **FlashAttention hang on Blackwell:** Add `--attention-backend TRITON_ATTN` (see [GPU Tuning](#gpu-specific-tuning)).

### LiteLLM Returns 500 / Model Not Found

**Symptom:** API calls return `500 Internal Server Error` or `model not found`.

```bash
# Check LiteLLM logs
kubectl logs statefulset/kube-llmops-litellm -n default

# Check the generated config
kubectl get configmap kube-llmops-litellm-config -n default -o yaml
```

**Common causes:**

- **Model backend not ready:** Wait for the vLLM pod to become Ready (it takes time to download and load models).
- **Missing `/v1` in api_base:** The LiteLLM config must use `http://<service>:8000/v1` (with `/v1` suffix). This is handled automatically by the chart, but verify if you're using custom configs.
- **Model name mismatch:** The model name in your curl request must match the `litellm_params.model` name in the config.

### Grafana Shows No Data

**Symptom:** Dashboards are empty, no metrics displayed.

```bash
# Check Prometheus is scraping targets
kubectl port-forward svc/kube-llmops-prometheus-server 9090:80 -n default &
# Open http://localhost:9090/targets in browser
```

**Common causes:**

- **OTel Collector not running:** Check `kubectl get pods -n default | grep otel`.
- **Prometheus not scraping:** Verify the OTel Collector is configured to scrape the correct endpoints.
- **Dashboards not provisioned:** Dashboards are loaded via ConfigMaps. Check `kubectl get configmap -n default | grep dashboard`.

### Langfuse UI Not Accessible

**Symptom:** Port-forward works but Langfuse shows an error page.

**Common causes:**

- **NEXTAUTH_URL misconfigured:** Langfuse needs `NEXTAUTH_URL` set to the URL you access it from (e.g., `http://localhost:3001`). This is set via `langfuse.externalUrl` in values.
- **Next.js not binding to 0.0.0.0:** Ensure the Langfuse deployment has `HOSTNAME=0.0.0.0` in its environment. The chart handles this automatically.
- **PostgreSQL not ready:** Langfuse requires its database. Check `kubectl get pods | grep langfuse-postgresql`.

### ConfigMap Not Updated After `helm upgrade`

**Symptom:** You changed values but the running config didn't change.

**Workaround:**

```bash
# Delete the ConfigMap and re-run upgrade
kubectl delete configmap kube-llmops-litellm-config -n default
helm upgrade kube-llmops charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-minimal.yaml \
  --namespace default
```

This is a known Helm server-side apply (SSA) issue. The chart will recreate the ConfigMap.

### DCGM Exporter Not Working

**Symptom:** GPU metrics missing, DCGM Exporter pods failing.

**Common causes:**

- **WSL2 environment:** DCGM Exporter does not work in WSL2. GPU metrics are not available.
- **No NVIDIA driver:** DCGM requires the NVIDIA driver and `nvidia-smi` working on the node.
- **GPU Operator not installed:** DCGM Exporter needs the GPU Operator or standalone DCGM installation.

---

## Uninstall

### Remove kube-llmops

```bash
helm uninstall kube-llmops -n default
```

### Clean Up Persistent Data

The uninstall does not remove PVCs (model cache, database data). To fully clean up:

```bash
# List PVCs
kubectl get pvc -n default | grep kube-llmops

# Delete all kube-llmops PVCs
kubectl delete pvc -l app.kubernetes.io/instance=kube-llmops -n default
```

### Remove the Namespace (if created)

```bash
# Only if you used a dedicated namespace
kubectl delete namespace llmops
```

---

## Next Steps

- 📖 Read the [Architecture](../ARCHITECTURE.md) to understand the full stack
- 📊 Explore the [Grafana dashboards](../dashboards/) for monitoring
- 🔧 Check the [examples/](../examples/) directory for advanced configurations
- 🤝 See [CONTRIBUTING.md](../CONTRIBUTING.md) to contribute
