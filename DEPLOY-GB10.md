# kube-llmops 部署记录：NVIDIA GB10 单节点 + Qwen3.5-122B-A10B-GPTQ-Int4

## 目标环境

| 项目 | 详情 |
|------|------|
| **节点** | promaxgb10-5c13 (192.168.1.37) |
| **架构** | aarch64 (ARM64) |
| **OS** | Ubuntu 24.04.4 LTS |
| **CPU** | 20 cores (Grace CPU) |
| **内存** | 121 GiB 统一内存 (CPU + GPU 共享) |
| **GPU** | NVIDIA GB10, CUDA 13.1, Compute Capability 12.1 |
| **磁盘** | 1.9T NVMe |
| **K8s** | k3s v1.34.5+k3s1, containerd 2.1.5 |
| **模型** | Qwen/Qwen3.5-122B-A10B-GPTQ-Int4 (122B 参数 MoE, GPTQ-Int4 量化, ~65GB) |

---

## 先决条件修复

### 问题 1：NVIDIA Device Plugin 检测不到 GPU

**现象**: device plugin 日志报 `Incompatible strategy detected auto`

**根因**: containerd 默认运行时为 `runc`，device plugin Pod 无法访问 NVIDIA 库

**修复**:
```bash
# 创建 containerd 模板，将默认运行时改为 nvidia
sudo bash -c 'cat > /var/lib/rancher/k3s/agent/etc/containerd/config.toml.tmpl << EOF
{{ template "base" . }}

[plugins."io.containerd.cri.v1.runtime".containerd]
  default_runtime_name = "nvidia"
EOF'
sudo systemctl restart k3s
```

### 问题 2：Device Plugin v0.17.1 不支持 GB10 统一内存

**现象**: `error getting device memory: Not Supported`

**根因**: GB10 使用 CPU/GPU 统一内存，传统 NVML `GetMemoryInfo()` 返回 "Not Supported"

**修复**: 升级到 v0.19.0（v0.18.0+ 包含 "Ignore errors getting device memory" 补丁）
```bash
sudo k3s kubectl set image daemonset/nvidia-device-plugin-daemonset \
  -n kube-system nvidia-device-plugin-ctr=nvcr.io/nvidia/k8s-device-plugin:v0.19.0
```

---

## 部署步骤

### 1. 安装 Helm
```bash
curl -fsSL https://get.helm.sh/helm-v3.20.1-linux-arm64.tar.gz -o /tmp/helm.tar.gz
cd /tmp && tar -xzf helm.tar.gz
sudo mv linux-arm64/helm /usr/local/bin/helm
```

### 2. 克隆代码
```bash
git clone https://github.com/GaeaRuiW/kube-llmops.git ~/kube-llmops
cd ~/kube-llmops
```

### 3. 创建 Override 配置文件 (`my-override.yaml`)

基于 `values-single-node.yaml`，覆盖以下内容：

```yaml
# Override for single-node: Qwen3.5-122B-A10B-GPTQ-Int4 on NVIDIA GB10 (arm64)

vllm:
  enabled: true
  image:
    repository: vllm/vllm-openai
    tag: v0.18.0-cu130             # arm64 + CUDA 13.0
  modelLoader:
    enabled: false                  # vLLM 自行从 HuggingFace 下载
    hfToken: "hf_YOUR_TOKEN"        # HuggingFace API Token
  modelCache:
    enabled: true
    size: 150Gi                     # 模型约 65GB
  models:
    - name: qwen3-5-122b-gptq
      source: Qwen/Qwen3.5-122B-A10B-GPTQ-Int4
      engine: vllm
      replicas: 1
      resources:
        gpu: 1
        memory: 100Gi
        cpu: 8
      engineArgs:
        --quantization: gptq_marlin  # 使用 Marlin GPTQ 内核（比 gptq 更快）
        --dtype: float16             # GPTQ 仅支持 float16
        --gpu-memory-utilization: "0.80"
        --max-model-len: "4096"
        --enforce-eager: ""          # Blackwell GPU 需要 eager mode
        --trust-remote-code: ""
  readinessProbe:
    initialDelaySeconds: 120
    periodSeconds: 15
    timeoutSeconds: 10
    failureThreshold: 200
  livenessProbe:
    initialDelaySeconds: 1800       # 30 min（模型下载 + 加载需要时间）
    periodSeconds: 30
    timeoutSeconds: 10
    failureThreshold: 60

litellm:
  models:
    - name: qwen3-5-122b-gptq
      source: Qwen/Qwen3.5-122B-A10B-GPTQ-Int4
      engine: vllm

observability:
  dcgmExporter:
    enabled: false                  # DCGM 不支持 GB10 统一内存

fluid:
  enabled: false                    # Alluxio arm64 不确定
keycloak:
  enabled: false                    # 简化初始部署
```

### 4. 更新依赖 & 部署
```bash
helm dependency update charts/kube-llmops-stack

sudo env KUBECONFIG=/etc/rancher/k3s/k3s.yaml \
  helm upgrade --install kube-llmops charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  -f my-override.yaml \
  --namespace default --create-namespace --timeout 30m
```

### 5. 模型下载前清理 Page Cache（重要！）

GB10 统一内存中，模型文件的 page cache 会占用大量内存。vLLM 在加载时检测到可用内存不足会报错：
```
ValueError: Free memory on device cuda:0 (44.59/121.63 GiB) is less than desired GPU memory utilization (0.8, 97.3 GiB)
```

**解决方法**：在 vLLM 模型下载完毕、开始加载权重前清理 page cache：
```bash
sudo sh -c 'sync && echo 3 > /proc/sys/vm/drop_caches'
# 然后删除 vLLM pod 让它重建
sudo k3s kubectl delete pod -l kube-llmops/model=qwen3-5-122b-gptq
```

---

## 部署过程中遇到的问题及解决

### 问题 3：vLLM 官方镜像无 ARM64 版本（< v0.18.0）

**现象**: `vllm/vllm-openai:v0.8.3` 仅有 amd64 架构
**解决**: 使用 v0.18.0+ 版本（首个支持 arm64 的版本）
- `vllm/vllm-openai:v0.18.0-cu130` — arm64 + CUDA 13.0

### 问题 4：model-loader 镜像不可用

**现象**: `ghcr.io/gaearuiw/kube-llmops/model-loader:latest` 返回 403 Forbidden
**解决**: 设置 `vllm.modelLoader.enabled: false`，让 vLLM 直接从 HuggingFace 下载模型。
仍需设置 `vllm.modelLoader.hfToken` 以将 HF_TOKEN 注入到主容器。

### 问题 5：Docker Hub DNS 间歇性解析失败

**现象**: 镜像拉取报 `dial tcp: lookup auth.docker.io: Try again`
**解决**: 手动重试（删除 Pod 让 k8s 重建），或手动 pull：
```bash
sudo k3s crictl pull vllm/vllm-openai:v0.18.0-cu130
```

### 问题 6：GPTQ 不支持 bfloat16

**现象**: `torch.bfloat16 is not supported for quantization method gptq`
**解决**: 在 engineArgs 中添加 `--dtype: float16`

### 问题 7：统一内存 Page Cache 导致 GPU 可用内存不足

**现象**: 模型下载后 page cache 占满内存，vLLM 报 `Free memory < desired utilization`
**解决**: 清理 page cache 后重启 Pod（见上文第 5 步）

---

## 最终部署状态

| 组件 | 镜像 | 状态 |
|------|------|------|
| **vLLM** (Qwen3.5-122B) | vllm/vllm-openai:v0.18.0-cu130 | Running (1/1) |
| **LiteLLM** (API Gateway) | ghcr.io/berriai/litellm:main-stable | Running (1/1) |
| **PostgreSQL** | postgres:16-alpine | Running (1/1) |
| **Prometheus** | prom/prometheus:v2.54.1 | Running (1/1) |
| **Grafana** | grafana/grafana:11.3.0 | Running (1/1) |
| **OTel Collector** | otel/opentelemetry-collector-contrib:0.114.0 | Running (1/1) |
| **Langfuse** | langfuse/langfuse:3.160.0 | Running (1/1) |
| **Langfuse Worker** | langfuse/langfuse-worker:3.160.0 | Running (1/1) |
| **ClickHouse** | clickhouse/clickhouse-server:24.12-alpine | Running (1/1) |
| **Redis** | redis:7-alpine | Running (1/1) |
| **Loki** | grafana/loki:3.4.2 | Running (1/1) |
| **Fluent Bit** | fluent/fluent-bit:4.0.2 | Running (1/1) |

### 资源使用

- **GPU 内存**: 70,703 MiB (~69 GB) — 模型权重
- **模型加载时间**: ~15 分钟（从磁盘加载到 GPU 内存）
- **模型下载时间**: ~55 分钟（从 HuggingFace 下载 65GB）

### 访问方式

| 服务 | ClusterIP 端口 | 认证 |
|------|----------------|------|
| LiteLLM API | :4000 | Bearer `sk-kube-llmops-dev` |
| Grafana | :3000 | admin / `admin123!` |
| Langfuse | :3000 | admin@kube-llmops.local / `admin123!` |
| vLLM 直连 | :8000 | 无 |

### API 调用示例
```bash
curl http://<LITELLM_IP>:4000/v1/chat/completions \
  -H "Authorization: Bearer sk-kube-llmops-dev" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen3-5-122b-gptq",
    "messages": [{"role": "user", "content": "你好！"}],
    "max_tokens": 100
  }'
```
