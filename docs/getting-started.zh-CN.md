[English](getting-started.md) | **中文**

# kube-llmops 快速入门

本指南将引导你完成 kube-llmops 的安装、部署验证以及发送第一个 LLM API 请求。

## 目录

- [前置条件](#前置条件)
- [快速安装](#快速安装)
  - [一键安装](#一键安装)
  - [手动安装](#手动安装)
- [选择部署配置](#选择部署配置)
- [验证安装](#验证安装)
- [发送第一个请求](#发送第一个请求)
- [访问 Web 界面](#访问-web-界面)
- [GPU 专项调优](#gpu-专项调优)
- [自定义配置](#自定义配置)
- [故障排查](#故障排查)
- [卸载](#卸载)

---

## 前置条件

| 要求 | 版本 | 备注 |
|---|---|---|
| Kubernetes | 1.28+ | 任意发行版均可：AKS、EKS、GKE、k3s、kind、minikube |
| Helm | 3.x | `brew install helm` 或参考[安装指南](https://helm.sh/docs/intro/install/) |
| kubectl | 1.28+ | 需已配置好与集群的连接 |
| GPU 节点 | 可选 | NVIDIA GPU 并已安装 [GPU Operator](https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/) |

**验证前置条件：**

```bash
# Check versions
helm version --short
kubectl version --client --short

# Check cluster connectivity
kubectl cluster-info

# Check GPU availability (optional)
kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.capacity.nvidia\.com/gpu}{"\n"}{end}'
```

没有 GPU？没关系——使用 `ci` 配置，它会运行一个仅需 CPU 的超小模型。

---

## 快速安装

### 一键安装

最快的上手方式。该命令会将仓库克隆到临时目录，安装 Helm chart，然后自动清理：

```bash
curl -sfL https://raw.githubusercontent.com/GaeaRuiW/kube-llmops/main/scripts/install.sh | bash
```

**通过环境变量自定义：**

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

| 变量 | 默认值 | 说明 |
|---|---|---|
| `KUBE_LLMOPS_PROFILE` | `minimal` | 部署配置：`ci`、`minimal`、`standard` |
| `KUBE_LLMOPS_NAMESPACE` | `default` | Kubernetes namespace |
| `KUBE_LLMOPS_RELEASE` | `kube-llmops` | Helm release 名称 |
| `KUBE_LLMOPS_BRANCH` | `main` | 安装所用的 Git 分支 |

### 手动安装

如果你想要更多控制权：

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

## 选择部署配置

kube-llmops 针对不同环境提供了预配置的部署方案：

| 配置 | 文件 | GPU | 模型 | 监控 | 追踪 | 适用场景 |
|---|---|---|---|---|---|---|
| **ci** | `values-ci.yaml` | 无 | 仅 CPU 的超小模型 | 基础 Prometheus + Grafana | 关闭 | CI 流水线、快速演示 |
| **minimal** | `values-minimal.yaml` | 1 块 | 1 个小模型 (Qwen2.5-0.5B) | Prometheus + Grafana | 关闭 | 开发、学习 |
| **standard** | `values-standard.yaml` | 4-8 块 | 2-3 个模型 | 完整 OTel 栈 | Langfuse | 团队 / 预发布环境 |

**如何选择：**

- **没有 GPU 或只是想试试？** → `ci`
- **单 GPU 开发机？** → `minimal`
- **多 GPU 团队环境？** → `standard`

随时可以通过重新运行 `helm upgrade` 来切换配置：

```bash
helm upgrade kube-llmops charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-standard.yaml \
  --namespace default
```

---

## 验证安装

### 检查 Pod 状态

```bash
kubectl get pods -n default
```

预期输出（minimal 配置）：

```
NAME                                        READY   STATUS    RESTARTS   AGE
kube-llmops-litellm-0                       1/1     Running   0          2m
kube-llmops-litellm-postgresql-0            1/1     Running   0          2m
kube-llmops-vllm-qwen2-5-0-5b-xxx          1/1     Running   0          3m
kube-llmops-prometheus-server-xxx           2/2     Running   0          2m
kube-llmops-grafana-xxx                     1/1     Running   0          2m
kube-llmops-otel-collector-xxx              1/1     Running   0          2m
```

> **注意：** vLLM pod 首次启动时需要下载模型，可能需要 3-10 分钟才能变为 Ready 状态。可使用 `kubectl logs -f` 查看进度。

### 检查模型就绪状态

```bash
# Watch vLLM pod logs until you see "Uvicorn running on http://0.0.0.0:8000"
kubectl logs -f deployment/kube-llmops-vllm-qwen2-5-0-5b -n default
```

### 检查 Helm Release 状态

```bash
helm status kube-llmops -n default
helm get values kube-llmops -n default
```

---

## 发送第一个请求

当所有 pod 都处于 Running 状态后，通过 port-forward 转发 LiteLLM 网关并发送一个聊天补全请求：

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

预期响应（已截断）：

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

### 使用 OpenAI Python SDK

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

## 访问 Web 界面

通过 port-forward 转发各个服务以访问 Web 界面：

```bash
# AI Gateway (LiteLLM)
kubectl port-forward svc/kube-llmops-litellm 4000:4000 -n default &

# Dashboards (Grafana)
kubectl port-forward svc/kube-llmops-grafana 3000:3000 -n default &

# LLM Tracing (Langfuse) — standard profile only
kubectl port-forward svc/kube-llmops-langfuse 3001:3000 -n default &
```

### 默认凭据

| 服务 | URL | 用户名 | 密码 |
|---|---|---|---|
| **LiteLLM**（AI 网关） | [http://localhost:4000/ui](http://localhost:4000/ui) | 任意用户名 | `sk-kube-llmops-dev` |
| **Grafana**（监控仪表盘） | [http://localhost:3000](http://localhost:3000) | `admin` | `admin` |
| **Langfuse**（LLM 追踪） | [http://localhost:3001](http://localhost:3001) | `admin@kube-llmops.local` | `admin123!` |

> **⚠️ 安全警告：** 以上均为开发环境的默认凭据。生产部署时，请务必覆盖这些凭据：
>
> ```bash
> helm upgrade kube-llmops charts/kube-llmops-stack \
>   --set litellm.masterKey=sk-your-production-key \
>   --set observability.grafana.adminPassword=your-secure-password \
>   --set langfuse.init.userPassword=your-secure-password
> ```

### Grafana 仪表盘

kube-llmops 预置了 3 个 Grafana 仪表盘：

1. **vLLM Engine** — 请求吞吐量、首 token 延迟（TTFT）、token 生成延迟、KV 缓存利用率
2. **LiteLLM Gateway** — API 请求速率、按模型划分的延迟、错误率、成本追踪
3. **GPU Metrics** — GPU 利用率、显存使用量、温度、功耗（需要 DCGM Exporter）

在 Grafana 中导航至 **Dashboards → Browse** 即可找到。

---

## GPU 专项调优

### NVIDIA Blackwell GPU（B200、GB200）

FlashAttention 2 在 Blackwell（SM 12.0）GPU 上可能会卡死。请添加 TRITON_ATTN 作为替代方案：

```yaml
# In your values override file
models:
  - name: your-model
    engineArgs:
      - "--attention-backend"
      - "TRITON_ATTN"
```

或通过 `--set` 参数：

```bash
helm upgrade kube-llmops charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-minimal.yaml \
  --set 'models[0].engineArgs[0]=--attention-backend' \
  --set 'models[0].engineArgs[1]=TRITON_ATTN'
```

### GPU 显存利用率

默认情况下，vLLM 使用 90% 的 GPU 显存。可按模型单独调整：

```yaml
models:
  - name: your-model
    engineArgs:
      - "--gpu-memory-utilization"
      - "0.85"
```

### 多 GPU（张量并行）

当大模型无法放入单块 GPU 时：

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

## 自定义配置

### 添加新模型

编辑你的 values 文件或创建覆盖文件：

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

然后执行升级：

```bash
helm upgrade kube-llmops charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-minimal.yaml \
  -f my-values.yaml \
  --namespace default
```

### 修改资源限制

覆盖任意组件的资源配置：

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

### 启用可选组件

Langfuse、DCGM Exporter 和 Loki 等组件按配置启用或关闭。可单独启用它们：

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

### 使用私有模型仓库（Hugging Face Token）

对于需要认证的受限模型：

```bash
# Create the secret
kubectl create secret generic hf-token \
  --from-literal=token=hf_your_token_here \
  -n default

# Reference it in your model config
# (see "Add a New Model" above for extraEnv example)
```

---

## 故障排查

### Pod 卡在 `Pending` 状态

**症状：** 模型服务 pod 持续处于 `Pending` 状态。

```bash
kubectl describe pod <pod-name> -n default
```

**常见原因：**

- **GPU 不足：** 节点没有足够的 GPU。通过 `kubectl describe node <node>` 检查 GPU 容量。
- **未安装 GPU 插件：** 安装 [NVIDIA GPU Operator](https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/) 或 [device plugin](https://github.com/NVIDIA/k8s-device-plugin)。
- **资源限制过高：** 在 values 文件中降低 `resources.requests` 的值。

**无 GPU 时的解决方案（演示 / CI）：**

```bash
helm upgrade kube-llmops charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-ci.yaml \
  --namespace default
```

### vLLM Pod CrashLoopBackOff

**症状：** vLLM pod 反复重启。

```bash
kubectl logs deployment/kube-llmops-vllm-qwen2-5-0-5b -n default --previous
```

**常见原因：**

- **OOM（内存不足）：** 模型对 GPU 来说太大。尝试 `--gpu-memory-utilization 0.80` 或使用更小的模型。
- **CUDA 版本不匹配：** 确保 vLLM 镜像与你的 CUDA 驱动版本匹配。
- **模型下载失败：** 检查受限模型是否需要 HF_TOKEN，或模型 ID 是否正确。
- **FlashAttention 在 Blackwell 上卡死：** 添加 `--attention-backend TRITON_ATTN`（参见 [GPU 专项调优](#gpu-专项调优)）。

### LiteLLM 返回 500 / 找不到模型

**症状：** API 调用返回 `500 Internal Server Error` 或 `model not found`。

```bash
# Check LiteLLM logs
kubectl logs statefulset/kube-llmops-litellm -n default

# Check the generated config
kubectl get configmap kube-llmops-litellm-config -n default -o yaml
```

**常见原因：**

- **模型后端未就绪：** 等待 vLLM pod 变为 Ready 状态（下载和加载模型需要时间）。
- **api_base 缺少 `/v1`：** LiteLLM 配置中必须使用 `http://<service>:8000/v1`（带 `/v1` 后缀）。chart 会自动处理此项，但如果你使用自定义配置请注意检查。
- **模型名称不匹配：** curl 请求中的模型名称必须与配置中 `litellm_params.model` 的名称一致。

### Grafana 无数据显示

**症状：** 仪表盘为空，没有指标显示。

```bash
# Check Prometheus is scraping targets
kubectl port-forward svc/kube-llmops-prometheus-server 9090:80 -n default &
# Open http://localhost:9090/targets in browser
```

**常见原因：**

- **OTel Collector 未运行：** 检查 `kubectl get pods -n default | grep otel`。
- **Prometheus 未抓取数据：** 确认 OTel Collector 配置了正确的抓取端点。
- **仪表盘未加载：** 仪表盘通过 ConfigMap 加载。检查 `kubectl get configmap -n default | grep dashboard`。

### Langfuse 界面无法访问

**症状：** port-forward 正常但 Langfuse 显示错误页面。

**常见原因：**

- **NEXTAUTH_URL 配置错误：** Langfuse 需要将 `NEXTAUTH_URL` 设置为你访问它的 URL（例如 `http://localhost:3001`）。可通过 values 中的 `langfuse.externalUrl` 设置。
- **Next.js 未绑定到 0.0.0.0：** 确保 Langfuse 部署环境变量中包含 `HOSTNAME=0.0.0.0`。chart 会自动处理此项。
- **PostgreSQL 未就绪：** Langfuse 依赖其数据库。检查 `kubectl get pods | grep langfuse-postgresql`。

### `helm upgrade` 后 ConfigMap 未更新

**症状：** 修改了 values 但运行中的配置没有变化。

**解决方法：**

```bash
# Delete the ConfigMap and re-run upgrade
kubectl delete configmap kube-llmops-litellm-config -n default
helm upgrade kube-llmops charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-minimal.yaml \
  --namespace default
```

这是 Helm server-side apply（SSA）的已知问题。chart 会重新创建该 ConfigMap。

### DCGM Exporter 不工作

**症状：** GPU 指标缺失，DCGM Exporter pod 运行失败。

**常见原因：**

- **WSL2 环境：** DCGM Exporter 不支持在 WSL2 中运行。GPU 指标不可用。
- **无 NVIDIA 驱动：** DCGM 要求节点上安装了 NVIDIA 驱动且 `nvidia-smi` 可正常运行。
- **未安装 GPU Operator：** DCGM Exporter 需要 GPU Operator 或独立安装的 DCGM。

---

## 卸载

### 移除 kube-llmops

```bash
helm uninstall kube-llmops -n default
```

### 清理持久化数据

卸载不会删除 PVC（模型缓存、数据库数据）。如需完全清理：

```bash
# List PVCs
kubectl get pvc -n default | grep kube-llmops

# Delete all kube-llmops PVCs
kubectl delete pvc -l app.kubernetes.io/instance=kube-llmops -n default
```

### 删除 namespace（如已创建）

```bash
# Only if you used a dedicated namespace
kubectl delete namespace llmops
```

---

## 下一步

- 📖 阅读[架构文档](../ARCHITECTURE.md)以了解完整技术栈
- 📊 浏览 [Grafana 仪表盘](../dashboards/)进行监控
- 🔧 查看 [examples/](../examples/) 目录获取高级配置示例
- 🤝 参阅 [CONTRIBUTING.md](../CONTRIBUTING.md) 了解如何贡献
