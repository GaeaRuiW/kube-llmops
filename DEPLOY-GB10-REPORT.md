# kube-llmops 部署测试报告

## 目标环境

| 项目 | 详情 |
|------|------|
| **节点** | promaxgb10-5c13 (192.168.1.37) |
| **架构** | aarch64 (ARM64) — NVIDIA Grace Blackwell |
| **GPU** | NVIDIA GB10, CUDA 13.1, Compute Capability 12.1 |
| **内存** | 121 GiB **统一内存**（CPU+GPU 共享同一内存池） |
| **磁盘** | 1.9T NVMe |
| **K8s** | k3s v1.34.5+k3s1, containerd 2.1.5 |
| **模型** | Qwen/Qwen3.5-122B-A10B-GPTQ-Int4 (122B MoE, ~65GB) |
| **部署配置** | values-single-node.yaml + 自定义 override |

---

## 最终部署状态

### Pod 状态（15/15 Running）

| Pod | 状态 | 说明 |
|-----|------|------|
| vllm-qwen3-5-122b-gptq | 1/1 ✅ | 模型从 MinIO 加载，推理正常 |
| kube-llmops-litellm | 1/1 ✅ | API 网关 + Langfuse trace 正常 |
| kube-llmops-litellm-pg | 1/1 ✅ | PostgreSQL（LiteLLM + Langfuse 共用） |
| kube-llmops-minio | 1/1 ✅ | 模型存储 73.5GB |
| kube-llmops-langfuse | 1/1 ✅ | LLM 追踪 Web UI |
| kube-llmops-langfuse-worker | 1/1 ✅ | Trace 处理 |
| kube-llmops-langfuse-clickhouse | 1/1 ✅ | OLAP 存储 |
| kube-llmops-langfuse-redis | 1/1 ✅ | Worker 队列 |
| kube-llmops-keycloak | 1/1 ✅ | SSO/OIDC 中心 |
| kube-llmops-grafana | 1/1 ✅ | 监控面板 + SSO |
| kube-llmops-prometheus | 1/1 ✅ | 指标存储 |
| kube-llmops-otel-collector | 1/1 ✅ | 指标采集管道 |
| kube-llmops-dcgm-exporter | 1/1 ✅ | GPU 硬件指标 |
| kube-llmops-loki | 1/1 ✅ | 日志聚合 |
| kube-llmops-fluent-bit | 1/1 ✅ | 日志采集 |

### 服务访问地址

| 服务 | 地址 | 认证 |
|------|------|------|
| LiteLLM API | http://192.168.1.37:30400 | Bearer `sk-kube-llmops-dev` |
| Grafana | http://192.168.1.37:30300 | Keycloak SSO 或 admin/admin123! |
| Langfuse | http://192.168.1.37:30301 | Keycloak SSO 或 admin@kube-llmops.local/admin123! |
| Keycloak | http://192.168.1.37:30808 | admin/admin123! |
| Prometheus | http://192.168.1.37:30909 | 无 |
| MinIO Console | http://192.168.1.37:30901 | minioadmin/minioadmin |
| vLLM 直连 | http://192.168.1.37:30800 | 无 |

---

## 踩坑记录（按时间顺序）

### 坑 1：NVIDIA Device Plugin 检测不到 GPU

- **现象**：`Incompatible strategy detected auto`，Pod 日志显示找不到 GPU
- **根因**：k3s containerd 默认运行时是 `runc`，device plugin Pod 无法访问 NVIDIA 库
- **修复**：创建 `/var/lib/rancher/k3s/agent/etc/containerd/config.toml.tmpl`，将默认运行时设为 nvidia，重启 k3s
- **状态**：✅ 已修复

### 坑 2：Device Plugin v0.17.1 不支持 GB10 统一内存

- **现象**：`error getting device memory: Not Supported`
- **根因**：GB10 统一内存架构下 `nvmlDeviceGetMemoryInfo()` 返回 Not Supported，v0.17.1 不处理此情况
- **修复**：升级到 v0.19.0（v0.18.0+ 包含 "Ignore errors getting device memory" 补丁和 Blackwell 架构检测）
- **状态**：✅ 已修复

### 坑 3：vLLM 官方镜像无 ARM64 版本 (< v0.18.0)

- **现象**：`vllm/vllm-openai:v0.8.3` 仅 amd64
- **根因**：vLLM 在 v0.18.0 之前不提供 ARM64 镜像
- **修复**：使用 `vllm/vllm-openai:v0.18.0-cu130`（首个支持 arm64 + CUDA 13.0 的版本）
- **状态**：✅ 已修复
- **备注**：`latest` tag 也支持 arm64 但在 GB10 上模型加载后 EngineCore 无日志输出、疑似超时，v0.18.0-cu130 更稳定

### 坑 4：model-loader 镜像不可用 (GHCR 403)

- **现象**：`ghcr.io/gaearuiw/kube-llmops/model-loader:latest` 返回 403 Forbidden
- **根因**：GHCR 镜像仓库私有或未发布
- **修复**：改写 vLLM 部署模板中 model-loader init container，使用 `python:3.11-slim` 镜像 + 内联 Python 脚本，支持 S3(MinIO) 和 HuggingFace 双源下载
- **状态**：✅ 已修复（模板层面重写）

### 坑 5：Docker Hub DNS 间歇性解析失败

- **现象**：`dial tcp: lookup auth.docker.io: Try again`，镜像拉取反复失败
- **根因**：远程机器 DNS 解析偶发超时
- **修复**：手动 `k3s crictl pull` 预拉镜像 + 删除失败 Pod 重建
- **状态**：✅ 已修复（变通方案）
- **建议**：配置 Docker Hub 镜像加速器或本地 registry

### 坑 6：GPTQ 量化不支持 bfloat16

- **现象**：`torch.bfloat16 is not supported for quantization method gptq`
- **根因**：vLLM 默认使用 bfloat16，但 GPTQ 只支持 float16
- **修复**：添加 `--dtype float16` 到 engineArgs
- **状态**：✅ 已修复

### 坑 7：GB10 统一内存 Page Cache 导致 GPU 可用内存不足

- **现象**：`ValueError: Free memory on device cuda:0 (44.59/121.63 GiB) is less than desired GPU memory utilization`
- **根因**：GB10 CPU/GPU 共享内存。模型文件读取产生 ~65GB page cache，CUDA 的 `mem_get_info()` 将 page cache 视为"已用"内存，导致"可用"内存不足以满足 `gpu-memory-utilization` 要求
- **修复**：在 vLLM 部署模板中添加 `drop-cache` init container，每次 Pod 启动前自动执行 `echo 3 > /proc/sys/vm/drop_caches`
- **状态**：✅ 已修复（模板层面，永久生效）
- **这是 GB10 统一内存的核心坑，贯穿整个部署过程**

### 坑 8：Kubernetes cgroup 内存限制 vs 统一内存

- **现象**：EngineCore 进程无日志直接退出
- **根因**：vLLM Pod 的 `resources.limits.memory: 100Gi`，但在统一内存下，GPU 分配的 69GB + Python 进程 + 文件 I/O 超出 cgroup 限制，被 OOM Killed
- **修复**：修改模板去掉 vLLM 容器的 memory limit（只保留 request 用于调度），统一内存系统不应限制 cgroup
- **状态**：✅ 已修复（模板层面）

### 坑 9：模型文件下载不完整（shard 38/39 缺失）

- **现象**：模型加载成功但输出全是 `!!!!`
- **根因**：HuggingFace 下载因网络超时中断，shard 38 和 39 未完成下载。手动用 `curl` 补下载的文件损坏（`SafetensorError: incomplete metadata, file not fully covered`）。vLLM 用零权重替代缺失层导致乱码输出
- **修复**：删除损坏文件 → 用 `huggingface_hub` SDK 在独立 Job 中正确下载 → 设置 `HF_HUB_OFFLINE=1` 防止运行时再次触发网络请求填满 page cache
- **状态**：✅ 已修复
- **教训**：不要用 curl 直接下载 HuggingFace 的大文件（CDN 重定向可能导致截断），始终用 huggingface_hub 库

### 坑 10：vLLM `latest` tag 在 GB10 上 EngineCore 静默超时

- **现象**：`latest` 版本 EngineCore 加载模型时 40+ 分钟无日志、无进度
- **根因**：疑似 `latest` 版本在 Blackwell SM 12.1 上的 GPTQ-Marlin 内核编译/转换极慢或卡死
- **修复**：切回 `v0.18.0-cu130` 稳定工作
- **状态**：✅ 已修复（固定版本）

### 坑 11：Keycloak 在 ARM64 上 OOM (1Gi 不够)

- **现象**：Pod OOMKilled，exitCode 137
- **根因**：Keycloak 默认内存限制 1Gi，在 ARM64 JVM 上内存占用更高
- **修复**：增加到 requests 1Gi / limits 2Gi
- **状态**：✅ 已修复（override 层面）

### 坑 12：DCGM Exporter OOM (256Mi 不够)

- **现象**：Pod CrashLoopBackOff，OOMKilled
- **根因**：默认 256Mi 限制不足（特别是 GB10 DCGM 初始化开销较大）
- **修复**：通过 `kubectl patch` 增加到 requests 512Mi / limits 1Gi。另外发现 chart 中 resources 字段硬编码，已修改模板使其可配置
- **状态**：✅ 已修复（模板 + patch）

### 坑 13：DCGM 镜像 tag 不存在

- **现象**：`nvcr.io/nvidia/k8s/dcgm-exporter:3.6.0-4.2.0-ubuntu22.04` 返回 NotFound
- **根因**：`observability/values.yaml` 中更新了一个不存在的 tag
- **修复**：改用 `3.3.8-3.6.0-ubuntu22.04`（有 ARM64 镜像）
- **状态**：✅ 已修复
- **这是文档/代码 bug**

### 坑 14：LiteLLM SSO 是企业付费功能

- **现象**：SSO 登录按钮灰色；启用后所有 API 请求返回 `JWT Auth is an enterprise only feature`
- **根因**：LiteLLM 的 `JWT_PUBLIC_KEY_URL` 和 `GENERIC_*` SSO 环境变量需要企业版 license
- **修复**：禁用 LiteLLM SSO，使用 master key (`sk-kube-llmops-dev`) 登录 UI 和 API
- **状态**：⚠️ 已知限制，无法修复（需购买 license）
- **这是文档 bug**：文档中标注 LiteLLM SSO 可用，实际需要企业版

### 坑 15：Langfuse 422 错误 — ClickHouse 内存不足

- **现象**：Langfuse Web UI 大量 422 `UNPROCESSABLE_CONTENT` 错误
- **根因**：ClickHouse 容器 memory limit 512Mi 太小，查询时 `memory limit exceeded`
- **修复**：patch 到 limits 2Gi。helm upgrade 会重置，需要在 Langfuse chart values 中持久化
- **状态**：✅ 已修复（patch，未持久化到 chart values）

### 坑 16：Grafana DCGM Dashboard 内容错误

- **现象**：标有 "dcgm" tag 的 GPU dashboard 显示的是 request 数量、latency 等 LiteLLM 指标
- **根因**：`gpu-overview.json` 所有面板用的是 `vllm_*` 指标而非 DCGM GPU 硬件指标
- **修复**：重写 dashboard JSON，替换为真正的 DCGM 指标：`DCGM_FI_DEV_GPU_UTIL`（利用率）、`DCGM_FI_DEV_GPU_TEMP`（温度）、`DCGM_FI_DEV_POWER_USAGE`（功耗）、`DCGM_FI_DEV_SM_CLOCK`（频率）
- **状态**：✅ 已修复
- **这是文档/代码 bug**

### 坑 17：Grafana Dashboard Datasource UID 不匹配

- **现象**：Dashboard 面板显示 "No data"
- **根因**：Dashboard JSON 硬编码 `"uid": "prometheus"`，但 Grafana 自动生成的 datasource UID 是 `PBFA97CFB590B2093`
- **修复**：替换为正确的 UID
- **状态**：⚠️ 临时修复（UID 在重装后会变，需要改为模板变量 `${DS_PROMETHEUS}` 或使用 Grafana provisioning 固定 UID）

### 坑 18：Helm Upgrade 每次重置 NodePort

- **现象**：每次 `helm upgrade` 后所有 Service 被重置为 ClusterIP
- **根因**：Chart 模板中 Service type 硬编码为 ClusterIP，不支持通过 values 配置 NodePort
- **修复**：创建 `~/kube-llmops/patch-nodeports.sh` 脚本，每次 helm upgrade 后运行
- **状态**：⚠️ 变通方案（应该在 chart 模板层面支持 `service.type` + `service.nodePort` 配置）

### 坑 19：Ingress 不支持 IP 直接访问

- **现象**：Kubernetes Ingress `host` 字段不允许 IP 地址
- **根因**：Ingress spec 规范要求 host 是 DNS 名称
- **修复**：使用 Traefik IngressRoute CRD 为 IP 创建路由规则；主要服务改为 NodePort 直接通过 IP:Port 访问
- **状态**：✅ 已修复（NodePort 方案）
- **文档问题**：ingress 模板中提到的 `ipMode` 未实现

### 坑 20：Qwen3.5 Thinking 模型默认输出超长思维链

- **现象**：简单问题如 "hi bro" 生成数千字的内部推理过程
- **根因**：Qwen3.5-122B-A10B 是 thinking 模型，默认启用推理链输出
- **修复**：API 调用时添加 `"chat_template_kwargs": {"enable_thinking": false}` 关闭思维链
- **状态**：✅ 已提供方案（需要调用方处理，非平台层面）

---

## 对项目代码/文档的修改

### 模板文件修改（已同步到远程机器，未提交到 git）

| 文件 | 修改内容 |
|------|----------|
| `charts/.../vllm/templates/deployment.yaml` | 1) 添加 `drop-cache` init container（清 page cache）；2) 去掉 vLLM 容器的 memory limit；3) 重写 model-loader 支持 S3/MinIO + HuggingFace 双源；4) S3 source 时自动映射 `--model` 到本地路径 |
| `charts/.../observability/templates/dcgm-exporter.yaml` | resources 字段从硬编码改为可配置（支持 values 覆盖） |
| `charts/.../observability/dashboards/gpu-overview.json` | 重写为真正的 DCGM GPU 指标（利用率/温度/功耗/频率） |
| `charts/.../observability/dashboards/*.json` | 修复 datasource UID |
| `charts/.../litellm/templates/deployment.yaml` | 添加 GENERIC_* OIDC SSO 环境变量支持（虽然需企业版） |
| `charts/.../litellm/values.yaml` | 扩展 SSO 配置结构 |

### 新增文件

| 文件 | 说明 |
|------|------|
| `~/kube-llmops/my-override.yaml` | GB10 专用 override 配置 |
| `~/kube-llmops/patch-nodeports.sh` | helm upgrade 后恢复 NodePort 的脚本 |
| `DEPLOY-GB10.md` | 部署记录文档（初版，需更新） |

---

## 发现的项目 Bug / 文档问题

| # | 类型 | 问题 | 建议修复 |
|---|------|------|----------|
| 1 | **代码** | DCGM tag `3.6.0-4.2.0-ubuntu22.04` 不存在 | 改回 `3.3.8-3.6.0-ubuntu22.04` 或验证后更新 |
| 2 | **代码** | gpu-overview.json 用 vLLM 指标假装 DCGM | 替换为真正的 DCGM 指标 |
| 3 | **代码** | Dashboard datasource UID 硬编码 `"prometheus"` | 改用 Grafana 模板变量或 provisioning 固定 UID |
| 4 | **代码** | DCGM exporter resources 硬编码 | 改为从 values 读取 |
| 5 | **代码** | vLLM deployment 的 memory limit = memory request | 统一内存场景需要去掉 limit 或允许独立配置 |
| 6 | **代码** | model-loader 镜像 GHCR 不可用 | 发布公共镜像或改用通用 Python 镜像 + 内联脚本 |
| 7 | **代码** | ClickHouse 默认 512Mi 太小 | 建议最低 1Gi，Langfuse chart values 中增加 ClickHouse resources 配置 |
| 8 | **代码** | Service type 全部硬编码 ClusterIP | 支持通过 values 配置 NodePort |
| 9 | **代码** | Ingress `ipMode` 提到但未实现 | 实现或删除文档引用 |
| 10 | **文档** | LiteLLM SSO 标注为可用 | 注明需要企业版 license |
| 11 | **文档** | 未说明 GB10/Blackwell 统一内存的特殊处理 | 添加 GB10 部署指南（page cache、memory limit 等） |
| 12 | **文档** | 未说明 Qwen3.5 thinking 模型的思维链控制 | 添加 `enable_thinking` 参数说明 |

---

## 关键架构决策记录

### MinIO 作为模型统一存储

```
用户上传模型 → MinIO (s3://models/Qwen3.5-122B-A10B-GPTQ-Int4/)
                      ↓ model-loader init container (S3 sync, ~2.5分钟)
                      ↓ drop-cache init container (清 page cache)
                      ↓ vLLM (--model /models/Qwen3.5-122B-A10B-GPTQ-Int4)
```

### GB10 统一内存处理策略

```
Pod 启动 → drop-cache (清 page cache, 恢复 ~115GB 可用)
         → model-loader (从 MinIO 同步 65GB 到 PVC)
         → vLLM (加载 69GB 到 GPU 统一内存, ~15分钟)
         → Ready (KV cache 用剩余内存)
```

关键配置：
- `HF_HUB_OFFLINE=1`：防止 vLLM 运行时访问 HuggingFace（会产生 page cache）
- 无 memory limit：cgroup 不限制统一内存
- `--gpu-memory-utilization 0.80`：使用 80% 统一内存
- `--enforce-eager`：Blackwell GPU 需要 eager mode
- `--dtype float16`：GPTQ 不支持 bfloat16

---

## 性能数据

| 指标 | 数值 |
|------|------|
| 模型从 MinIO 同步到 PVC | ~2.5 分钟 (65GB, 本地网络) |
| 模型加载到 GPU | ~15 分钟 (39 shards, 每个 ~10秒) |
| GPU 内存占用 | 70,703 MiB (~69 GB) |
| 首次 HuggingFace 下载 | ~55 分钟 (取决于网络) |
| 推理延迟 (首 token) | ~3 秒 |
| 生成吞吐 | ~5 tokens/s |
