# kube-llmops 基础设施测试计划 (Infra Test Plan)

> 版本: v1.0 | 日期: 2026-03-24 | 作者: Cloud-Native & AI Infra QA Architect

---

## 1. 测试目标

验证 kube-llmops Umbrella Helm Chart 在 **单节点 GPU 集群** 上的部署正确性、运行时可用性、
升级/卸载可逆性，以及 AI 基建组件（vLLM / TEI / LiteLLM / Dify）在异常条件下的韧性。

---

## 2. 测试范围边界

### 2.1 部署测试 (Deploy Verification)

| 检查点 | 描述 | 优先级 |
|--------|------|--------|
| Helm 模板渲染 | `helm template` 所有 profile（ci, minimal, single-node, standard, production）无报错 | P0 |
| 依赖解析 | `helm dependency update` 成功，所有 .tgz 归档可构建 | P0 |
| 全组件启动 | 所有 Pod 进入 Running/Completed 且容器 Ready | P0 |
| PVC 绑定 | 所有 PersistentVolumeClaim 状态为 Bound | P0 |
| Service 端点 | 所有 Service 至少有一个 Endpoints 不为空 | P0 |
| GPU 分配 | vLLM Pod 成功获取 `nvidia.com/gpu` 资源 | P0 |
| 健康端点 | 所有组件的 /health 端点返回 200 | P1 |
| Ingress 路由 | Traefik IngressRoute 可达（litellm/grafana/langfuse/keycloak） | P1 |
| Helm Hook | post-install smoke-test Job 成功完成 | P1 |
| ConfigMap/Secret | 所有配置正确挂载，无 MountError | P2 |

### 2.2 可用性验证 (Availability Verification)

| 检查点 | 描述 | 优先级 |
|--------|------|--------|
| AI 推理链路 | LiteLLM → vLLM 完成 `/v1/chat/completions` 请求 | P0 |
| Embedding 链路 | LiteLLM → TEI 完成 `/v1/embeddings` 请求 | P0 |
| Reranking 链路 | TEI reranker `/rerank` 端点可用 | P1 |
| RAG 全链路 | Dify KB 上传 → 索引 → 检索 → 生成 → 返回正确答案 | P0 |
| 可观测性 | Prometheus 抓取到 vLLM/DCGM 指标，Grafana 仪表板可展示 | P1 |
| 日志采集 | Fluent Bit → Loki 日志可在 Grafana 中查询 | P2 |
| Trace 链路 | LiteLLM → Langfuse 上报 trace，UI 可查看 | P1 |
| 安全防护 | LLM-Guard 拦截 prompt injection 攻击 | P1 |

### 2.3 升级/卸载验证 (Upgrade & Uninstall)

| 检查点 | 描述 | 优先级 |
|--------|------|--------|
| 滚动升级 | `helm upgrade` 后所有 Pod 滚动更新成功，无中断 | P1 |
| Quality Gate | pre-upgrade hook 正确检查 Ragas 指标阈值 | P1 |
| 数据持久化 | 升级后 PostgreSQL/ClickHouse/MinIO 数据完整 | P0 |
| 完整卸载 | `helm uninstall` 清除所有 K8s 资源（PVC 按策略保留） | P2 |
| 重新安装 | 卸载后重新 install 可成功（PVC 可复用或重建） | P2 |

---

## 3. 环境要求

### 3.1 集群要求

| 项目 | 最低要求 | 推荐配置 |
|------|---------|---------|
| K8s 版本 | v1.28+ | v1.30+ (K3s/RKE2) |
| 节点数量 | 1 (single-node) | 1+ |
| CPU | 16 核 | 16+ 核 |
| 内存 | 32 GB | 64 GB |
| GPU | 1x NVIDIA (8GB+ VRAM) | 1x NVIDIA (16GB+ VRAM) |
| 磁盘 | 200 GB SSD | 500 GB NVMe |
| 网络 | 集群内 DNS 可达 | + 外部可达 (模型下载) |

### 3.2 前置依赖

| 依赖 | 用途 | 安装方式 |
|------|------|---------|
| NVIDIA Driver | GPU 驱动 | `apt install nvidia-driver-560` |
| NVIDIA Container Toolkit | 容器内 GPU 访问 | `nvidia-ctk runtime configure` |
| NVIDIA Device Plugin | K8s GPU 资源发现 | DaemonSet (k8s-device-plugin) |
| Helm v3.12+ | Chart 部署 | `curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 \| bash` |
| kubectl | 集群管理 | K3s 自带 |
| Traefik | Ingress Controller | K3s 自带 |
| StorageClass | 持久卷供应 | K3s local-path (默认) |

### 3.3 测试工具栈

| 工具 | 版本 | 用途 |
|------|------|------|
| `helm` | v3.12+ | Chart 部署与 template 验证 |
| `kubectl` | v1.28+ | Pod/Service/PVC 状态检查 |
| `python3` + `kubernetes` SDK | 3.10+ | 自动化测试脚本 |
| `curl` / `jq` | 最新 | API 端点健康检查 |
| `uv` | 最新 | Python 脚本运行器 (PEP 723 inline deps) |
| `Playwright` | 最新 | E2E UI 测试 (Dify/Grafana/Langfuse) |
| `nvidia-smi` | 与驱动匹配 | GPU 状态验证 |

---

## 4. 测试架构

```
┌─────────────────────────────────────────────────────────────┐
│                    测试层次 (Test Pyramid)                     │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  L4  ┌─────────────────────┐  回归测试                       │
│      │  Quality Gate (Ragas)│  pre-upgrade hook              │
│      └─────────────────────┘                                │
│                                                             │
│  L3  ┌─────────────────────────────┐  质量测试                │
│      │  Ragas Eval CronJob (4 指标) │  每日 2:00 AM           │
│      └─────────────────────────────┘                        │
│                                                             │
│  L2  ┌───────────────────────────────────┐  E2E 测试         │
│      │  Playwright (Dify/Grafana/Langfuse)│                  │
│      │  + API 集成测试 (curl/Python)       │                  │
│      └───────────────────────────────────┘                  │
│                                                             │
│  L1  ┌─────────────────────────────────────────┐  基建验证    │
│      │  Bash/Python: Pod Ready + PVC Bound      │            │
│      │  + GPU Alloc + Health Endpoints          │            │
│      │  + Helm Template + Smoke Test            │            │
│      └─────────────────────────────────────────┘            │
│                                                             │
│  L0  ┌─────────────────────────────────────────────┐  静态   │
│      │  helm template + helm lint + YAML lint       │  检查   │
│      └─────────────────────────────────────────────┘        │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## 5. 组件健康端点矩阵

| 组件 | Service 名 | 端口 | 健康端点 | 预期响应 |
|------|-----------|------|---------|---------|
| vLLM | `vllm-qwen2-5-0-5b` | 8000 | `/health` | 200 OK |
| TEI Embedding | `tei-bge-small-en` | 8080 | `/health` | 200 OK |
| TEI Reranker | `tei-bge-reranker-base` | 8080 | `/health` | 200 OK |
| LiteLLM | `kube-llmops-litellm` | 4000 | `/health/liveliness` | 200 OK |
| PostgreSQL | `kube-llmops-litellm-pg` | 5432 | `pg_isready` | exit 0 |
| Langfuse | `kube-llmops-langfuse` | 3000 | `/api/public/health` | 200 `{"status":"OK"}` |
| Langfuse ClickHouse | `kube-llmops-langfuse-clickhouse` | 8123 | `/ping` | 200 "Ok." |
| Grafana | `kube-llmops-grafana` | 3000 | `/api/health` | 200 OK |
| Prometheus | `kube-llmops-prometheus` | 9090 | `/-/ready` | 200 |
| Keycloak | `kube-llmops-keycloak` | 8080 | `/health/ready` (port 9000) | 200 |
| MinIO | `kube-llmops-minio` | 9000 | `/minio/health/ready` | 200 |
| Dify API | `kube-llmops-dify-api` | 5001 | `/health` | 200 |
| LLM-Guard | `kube-llmops-llm-guard` | 8000 | `/healthz` | 200 |
| Loki | `kube-llmops-loki` | 3100 | `/ready` | 200 |
| Pushgateway | `kube-llmops-pushgateway` | 9091 | `/metrics` | 200 |

---

## 6. 风险矩阵

| 风险 | 影响 | 概率 | 缓解策略 |
|------|------|------|---------|
| GPU 资源不足 (UnexpectedAdmissionError) | vLLM 无法调度 | 高 | 脚本检查 allocatable GPU，提前告警 |
| 大镜像拉取超时 (ImagePullBackOff) | vLLM/LLM-Guard 镜像 > 5GB | 中 | 预拉取镜像，设置 imagePullPolicy |
| OOMKilled | LLM-Guard (~6GB RAM) | 中 | 验证 limits 设置合理 |
| PVC 绑定失败 | StorageClass 不匹配 | 低 | 检查 local-path provisioner 可用 |
| CrashLoopBackOff | DB 未就绪时组件启动 | 中 | 验证启动顺序和 failureThreshold |
| ConfigMap 热更新失效 | LiteLLM config 变更不生效 | 低 | 验证 checksum annotation 机制 |

---

## 7. 测试产出物

| 产出物 | 文件 | 描述 |
|--------|------|------|
| 测试计划 | `01-INFRA-TEST-PLAN.md` | 本文档 |
| 测试用例 | `02-TEST-CASES.md` | 详细测试用例 (5 个核心场景) |
| Bash 验证脚本 | `scripts/01-deploy-verify.sh` | Pod/PVC/GPU/Health 全面检查 |
| Python 测试脚本 | `scripts/02-k8s-resource-test.py` | K8s API 自动化验证 |
| Helm test 模板 | `scripts/03-helm-test-template.yaml` | 标准 helm test 钩子模板 |
| 异常场景脚本 | `scripts/04-edge-case-test.sh` | OOM/ImagePull/Eviction 模拟 |
| 执行指南 | `03-EXECUTION-GUIDE.md` | 保姆级实操步骤 |
| 测试报告模板 | `04-TEST-REPORT-TEMPLATE.md` | 执行结果记录模板 |
