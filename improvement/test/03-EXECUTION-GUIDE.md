# kube-llmops 测试执行指南 (Execution Guide)

> 版本: v1.0 | 日期: 2026-03-24

---

## 一、环境准备

### 1.1 集群环境确认

```bash
# 检查 K8s 集群状态
kubectl cluster-info
kubectl get nodes -o wide

# 检查 GPU 资源
kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}: CPU={.status.allocatable.cpu}, Memory={.status.allocatable.memory}, GPU={.status.allocatable.nvidia\.com/gpu}{"\n"}{end}'

# 检查 NVIDIA Device Plugin
kubectl get pods -n kube-system -l app.kubernetes.io/name=nvidia-device-plugin

# 检查 StorageClass
kubectl get storageclass

# 检查 Helm
helm version
```

### 1.2 DNS 配置

在运行测试的节点上添加 hosts 记录：

```bash
NODE_IP=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}')
echo "$NODE_IP litellm.llmops.local grafana.llmops.local langfuse.llmops.local keycloak.llmops.local prometheus.llmops.local" | sudo tee -a /etc/hosts
```

### 1.3 安装测试工具

```bash
# 确保 uv 可用 (Python 脚本运行器)
which uv || curl -LsSf https://astral.sh/uv/install.sh | sh

# 确保 jq 可用
which jq || sudo apt install -y jq

# 确保 curl 可用
which curl || sudo apt install -y curl
```

---

## 二、Helm 部署（如尚未部署）

### 2.1 全新部署

```bash
cd /path/to/kube-llmops

# 1. 构建依赖
cd charts/kube-llmops-stack
rm -f charts/*.tgz Chart.lock
helm dependency update .

# 2. 部署 (单节点 GPU 模式)
helm install kube-llmops . \
  -f values-single-node.yaml \
  --wait \
  --timeout 15m

# 3. 等待所有组件就绪 (可能需要 5-10 分钟)
kubectl get pods -w
```

### 2.2 核心部署参数说明

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `--wait` | 等待所有资源就绪后才返回 | 建议开启 |
| `--timeout 15m` | vLLM 模型加载需要较长时间 | 必须 >= 10m |
| `-f values-single-node.yaml` | 单节点配置 (1 GPU) | 必须指定 |
| `--namespace default` | 部署到 default 命名空间 | 默认 |

### 2.3 等待部署完成

```bash
# 方法 1: 持续观察 Pod 状态
watch -n 5 'kubectl get pods --no-headers | grep -v "Running\|Completed"'

# 方法 2: 等待特定组件
kubectl wait --for=condition=Ready pod -l app.kubernetes.io/name=vllm --timeout=600s
kubectl wait --for=condition=Ready pod -l app.kubernetes.io/name=litellm --timeout=300s
```

---

## 三、运行测试脚本

### 3.1 脚本 A: Bash 部署验证 (01-deploy-verify.sh)

此脚本进行全面的基础设施检查，包括 Pod/PVC/GPU/Service/健康端点。

```bash
cd /path/to/kube-llmops/improvement/test

# 运行（默认 namespace=default, release=kube-llmops）
bash scripts/01-deploy-verify.sh

# 指定命名空间
bash scripts/01-deploy-verify.sh my-namespace my-release

# 查看报告
cat test-report-*.txt
```

**脚本检查内容：**
1. 集群与节点状态 (kubectl/GPU/StorageClass)
2. Pod 状态 (Running/CrashLoopBackOff/Pending)
3. PVC 绑定状态 (Bound/Pending)
4. Service 端点可达性
5. GPU 资源分配
6. 组件健康端点 (HTTP 200)
7. Smoke Test Job 状态

**预期运行时间：** 2-3 分钟

### 3.2 脚本 B: Python K8s 资源测试 (02-k8s-resource-test.py)

此脚本使用 Kubernetes Python SDK 进行深度验证。

```bash
cd /path/to/kube-llmops/improvement/test

# 使用 uv 运行 (自动安装依赖)
uv run scripts/02-k8s-resource-test.py

# 跳过 AI 功能测试（更快）
uv run scripts/02-k8s-resource-test.py --skip-ai

# 指定命名空间
uv run scripts/02-k8s-resource-test.py --namespace my-namespace
```

**脚本检查内容：**
1. Pod 状态 + 关键组件 Ready
2. PVC 绑定 + 关键 PVC 存在
3. GPU 分配验证 (vLLM=1 GPU, TEI=CPU-only)
4. QoS Class (无 BestEffort 的关键 Pod)
5. Service Endpoints 非空
6. 健康端点 HTTP 200
7. Embedding 生成 (384 维)
8. LLM 推理 (数学题)
9. PostgreSQL 数据库完整性

**预期运行时间：** 3-5 分钟

### 3.3 脚本 C: Helm Test 模板 (03-helm-test-template.yaml)

此模板需要集成到 Chart 中才能使用 `helm test`。

```bash
# 1. 将模板复制到 Chart 的 templates 目录
cp scripts/03-helm-test-template.yaml \
   /path/to/kube-llmops/charts/kube-llmops-stack/templates/tests/infra-test.yaml

# 2. 在 values.yaml 中启用测试
# 在 values-single-node.yaml 末尾添加：
echo 'tests:
  enabled: true' >> /path/to/kube-llmops/charts/kube-llmops-stack/values-single-node.yaml

# 3. 升级部署
cd /path/to/kube-llmops/charts/kube-llmops-stack
rm -f charts/*.tgz Chart.lock && helm dependency update .
helm upgrade kube-llmops . -f values-single-node.yaml

# 4. 运行 Helm Test
helm test kube-llmops --timeout 5m

# 5. 查看测试日志
kubectl logs kube-llmops-infra-test
```

### 3.4 脚本 D: 边缘异常测试 (04-edge-case-test.sh)

此脚本测试 OOM/ImagePullBackOff/GPU 不足/数据持久化等异常场景。

```bash
cd /path/to/kube-llmops/improvement/test

# 运行
bash scripts/04-edge-case-test.sh

# 指定命名空间
bash scripts/04-edge-case-test.sh my-namespace
```

**注意：** 此脚本会进行破坏性操作（删除 PostgreSQL Pod 等），但会自动恢复。
建议在非生产环境运行。

**预期运行时间：** 5-8 分钟

---

## 四、测试执行顺序（推荐）

```
1. 01-deploy-verify.sh      ← 快速检查，确认基础设施就绪
2. 02-k8s-resource-test.py   ← 深度验证，包括 AI 功能
3. 04-edge-case-test.sh      ← 异常场景，会重启 Pod
4. helm test (可选)           ← 需要先集成模板
```

---

## 五、日志排查指南

### 5.1 高频排查命令（Top 3）

当某个 AI 组件 Pod 出现问题时，按以下顺序排查：

#### 命令 1: `kubectl describe pod` — 查看调度与启动事件

```bash
# 查看问题 Pod 的详细信息和事件
kubectl describe pod <POD_NAME>

# 重点看：
#   Events:     → 调度失败? 镜像拉取失败? Volume 挂载失败?
#   Conditions: → Ready=False 的原因
#   Last State: → 上次终止原因 (OOMKilled? Error?)
```

**常见错误及含义：**
| Events 关键词 | 含义 | 处理方式 |
|---------------|------|---------|
| `FailedScheduling` | 节点资源不足 | 检查 GPU/CPU/内存 allocatable |
| `Insufficient nvidia.com/gpu` | GPU 不够 | 缩减 replicas 或添加 GPU 节点 |
| `ImagePullBackOff` | 镜像拉取失败 | 检查网络/镜像名/仓库认证 |
| `FailedMount` | Volume 挂载失败 | 检查 PVC/StorageClass |
| `Unhealthy` | 健康检查失败 | 检查容器日志，增大 initialDelaySeconds |

#### 命令 2: `kubectl logs` — 查看容器日志

```bash
# 查看当前容器日志
kubectl logs <POD_NAME> -c <CONTAINER_NAME>

# 查看上次崩溃的日志（CrashLoopBackOff 必用）
kubectl logs <POD_NAME> -c <CONTAINER_NAME> --previous

# 查看初始化容器日志（模型下载失败时查看）
kubectl logs <POD_NAME> -c model-loader
kubectl logs <POD_NAME> -c db-migrate

# 持续查看日志
kubectl logs -f <POD_NAME>

# 查看最近 50 行
kubectl logs --tail=50 <POD_NAME>
```

**各组件关键日志：**
| 组件 | 关键日志关键词 | 含义 |
|------|---------------|------|
| vLLM | `CUDA out of memory` | GPU 显存不足，降低 gpu-memory-utilization |
| vLLM | `Model loaded` | 模型加载成功 |
| LiteLLM | `LiteLLM Proxy started` | 网关启动成功 |
| LiteLLM | `Connection refused` | 后端服务未就绪 |
| Langfuse | `ClickHouse connection error` | ClickHouse 未就绪 |
| Dify | `sqlalchemy.exc.OperationalError` | PostgreSQL 连接失败 |

#### 命令 3: `kubectl get events` — 集群事件全景

```bash
# 按时间排序查看最近事件
kubectl get events --sort-by='.lastTimestamp' | tail -30

# 只看 Warning 类型
kubectl get events --field-selector type=Warning

# 看特定 Pod 的事件
kubectl get events --field-selector involvedObject.name=<POD_NAME>
```

### 5.2 常见问题速查表

| 问题 | 现象 | 排查步骤 |
|------|------|---------|
| vLLM 卡在 Pending | GPU 不足 | `kubectl describe pod` → Events → Insufficient nvidia.com/gpu |
| vLLM CrashLoopBackOff | 模型加载失败/OOM | `kubectl logs --previous` → CUDA out of memory → 降低 gpu-memory-utilization |
| LiteLLM 502 | 后端未就绪 | `kubectl logs kube-llmops-litellm` → Connection refused → 等待 vLLM Ready |
| Langfuse 启动失败 | DB 未就绪 | `kubectl logs --previous kube-llmops-langfuse` → ClickHouse/PG 错误 |
| Dify 500 | Plugin Daemon 崩溃 | `kubectl logs kube-llmops-dify-plugin-daemon` |
| PVC Pending | StorageClass 问题 | `kubectl describe pvc` → Events → StorageClass not found |
| ConfigMap 不生效 | Pod 未重启 | 检查 Deployment 的 checksum annotation 是否更新 |

### 5.3 组件状态快速检查一条龙

```bash
# 一条命令查看全部组件状态
kubectl get pods -o custom-columns=NAME:.metadata.name,STATUS:.status.phase,READY:.status.containerStatuses[0].ready,RESTARTS:.status.containerStatuses[0].restartCount,AGE:.metadata.creationTimestamp --sort-by='.metadata.creationTimestamp'

# 找出所有不健康的 Pod
kubectl get pods --no-headers | grep -vE "Running|Completed" | awk '{print $1}' | xargs -I{} sh -c 'echo "=== {} ===" && kubectl describe pod {} | tail -20'

# 检查所有 Service 是否有 Endpoints
kubectl get endpoints --no-headers | awk '{if($2=="<none>") print "NO ENDPOINTS: "$1}'
```

---

## 六、远程机器 (gb10) 测试

### 6.1 SSH 连接配置

```bash
# 配置 SSH 免密登录
ssh-copy-id gb10@192.168.1.37

# 或者使用密码方式
ssh gb10@192.168.1.37
```

### 6.2 远程执行测试

```bash
# 将测试脚本复制到远程机器
scp -r improvement/test gb10@192.168.1.37:~/kube-llmops-test/

# SSH 到远程机器执行
ssh gb10@192.168.1.37 "cd ~/kube-llmops-test && bash scripts/01-deploy-verify.sh"
ssh gb10@192.168.1.37 "cd ~/kube-llmops-test && uv run scripts/02-k8s-resource-test.py"
```

### 6.3 远程 kubeconfig

确保远程机器的 kubectl 已配置指向正确的集群：

```bash
ssh gb10@192.168.1.37 "kubectl cluster-info && kubectl get nodes -o wide"
```

---

## 七、测试结果判读

### 通过标准

| 级别 | 标准 | 说明 |
|------|------|------|
| P0 (阻塞) | 0 FAIL | 所有 P0 测试必须通过 |
| P1 (重要) | <= 2 WARN | 可接受少量告警 |
| P2 (建议) | 不阻塞 | 仅记录改进建议 |

### 报告模板

测试完成后，`01-deploy-verify.sh` 会自动生成 `test-report-*.txt` 报告文件。
Python 脚本会在终端输出 Rich 格式的汇总表。

建议将输出重定向到文件保存：

```bash
bash scripts/01-deploy-verify.sh 2>&1 | tee deploy-verify-result.log
uv run scripts/02-k8s-resource-test.py 2>&1 | tee k8s-resource-result.log
bash scripts/04-edge-case-test.sh 2>&1 | tee edge-case-result.log
```
