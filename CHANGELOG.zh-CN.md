[English](CHANGELOG.md) | **中文**

# 更新日志

本文件记录了本项目的所有重要变更。

本文件格式基于 [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)，
并且本项目遵循 [语义化版本](https://semver.org/spec/v2.0.0.html) 规范。

## [1.0.0] - 2026-04-23

> **里程碑：** Phase 6 完成 — Operator + CLI + Dashboard 三件套 GA。

### 新增

#### kubectl-llmops CLI
- `operator/cmd/kubectl-llmops/` — Go 二进制，可作为 kubectl 插件使用（`kubectl llmops <cmd>`）
- 构建：`cd operator && make build-cli` 或 `make install-cli`（安装到 `$GOPATH/bin`）
- 15 个顶层命令 + 3 个子命令组
- 模型生命周期：`deploy`、`list`、`status`、`scale`、`delete`
  - `kubectl llmops deploy Qwen/Qwen2.5-7B-Instruct` 自动检测引擎并创建 `ModelDeployment`
  - 支持所有常用模型选项（`--gpu`、`--memory`、`--replicas`、`--engine-arg`、`--prefix-caching`、`--accelerator`）
- 金丝雀部署：`canary`、`promote`、`rollback`
  - `kubectl llmops canary <name> --target <new-source> --weight 20` 创建加权金丝雀
- 开发者体验：`logs`、`test`、`endpoint`、`port-forward`、`dashboard`
  - `kubectl llmops test <name> --prompt "Hello" --stream` 直接请求 LiteLLM
  - `port-forward --service=gateway|grafana|langfuse|dify|minio` 自动发现服务
- `finetune {create,list,status,logs,delete}` — 驱动 `FineTuneRun` CR，读取 MLflow 指标 + 质量门结果
- `platform {init,status,update}` — 驱动 `LLMPlatform` CR，`update --enable rag --disable security` 切换模块开关
- `rag {list-kb,create-kb,upload,delete-kb,query,eval}` — 直接调用 Dify Console API（自动发现端点、从 `kube-llmops-dify-setup` ConfigMap 读取凭证）
- `migrate <helm-release>` — 单向转换：Helm release → LLMPlatform + ModelDeployment CR
- 全局标志：`-n/--namespace`、`-o table|json|yaml|wide`、`--kubeconfig`、`--context`
- 引擎自动检测复用 `internal/engine/resolver.go`（与 operator 相同逻辑）

### 变更
- Chart 版本升级：umbrella `kube-llmops-stack` + 20 个子 chart + operator chart → 1.0.0
- Phase 6 路线图项目（Operator、CLI、Web Dashboard）全部标记完成

### 说明
这是 Phase 6（Platform UX）里程碑版本。之前需要 `helm upgrade --set ...` 或
`kubectl edit llmplatform` 的命令式操作，现在都可以通过 `kubectl llmops` 完成。

## [0.5.0] - 2026-04-12

### 新增

#### Kubernetes Operator（LLMPlatform CR）
- `operator/` 目录：基于 controller-runtime 的 Go 语言 Kubernetes Operator
- 三个 CRD：`LLMPlatform`（完整平台）、`ModelDeployment`（单模型）、`FineTuneRun`（微调任务）
- Operator Helm chart：`operator/charts/kube-llmops-operator/`（构建时嵌入 umbrella chart）
- 声明式平台管理，作为直接 `helm install` 的替代方案
- Reconciler 将 LLMPlatform spec 翻译为 Helm values，通过 Helm SDK 驱动 install/upgrade
- LLMPlatform spec 新增 `Models []ModelSpec` 字段，在 CR 中声明模型
- 内置卡住版本恢复：`FixStuckRelease()` 处理 pending-install/pending-upgrade
- 基于 Generation 的 reconcile 循环防护（`status.observedGeneration == metadata.generation` 时跳过）
- Operator SA 使用 `cluster-admin` RBAC（Helm chart 里有通配符绑定如 Headlamp，RBAC escalation 防护要求）
- 示例 CR：`operator/config/samples/llmplatform_full.yaml`
- 构建脚本：`operator/build.sh`（staging 目录 `_build_charts/` 避免删除 operator 自己的 chart）
- 用户指南：`operator/docs/user-guide/operator-guide-en.md` + `operator-guide-zh.md`
- 架构文档：`operator/docs/architecture/operator.md`

#### Headlamp Kubernetes UI（替代旧版自定义 dashboard）
- `headlamp` 子 chart 封装上游 Headlamp chart
- `kube-llmops-portal` 插件，两个页面：
  - **Service Links**：所有平台服务的 NodePort URL 卡片网格
  - **Monitoring**：通过 iframe 嵌入 Grafana 仪表板
- 启用 NodePort 时监听 30302 端口
- Keycloak OIDC SSO 自动配置（issuer URL 从 `nodePort.host` 计算）
- Grafana 配置 `allow_embedding=true` + 匿名 Viewer 访问以支持 iframe
- 插件镜像：`kube-llmops/headlamp-plugin:latest`（通过 `docker build plugins/kube-llmops-portal/` 构建）
- 23 条 Helm 模板测试（`tests/helm/test_headlamp_templates.py`）

#### 高级推理（Phase 5）
- 基于延迟的路由（默认策略，取代 simple-shuffle）
- 每模型前缀缓存开关（`prefixCaching: true`）
- 会话亲和性 via Envoy sidecar（`litellm.sessionAffinity.enabled`）
- 多触发器 KEDA 自动扩缩（队列深度 + TTFT P95 + TPOT P95）
- SLO 告警规则（TTFTSLOBreach、TTFTSLOCritical、TPOTSLOBreach）
- 规模到零 + LiteLLM 冷启动 fallback
- Spot/抢占式 GPU tolerations（AWS、GCP、Azure、Karpenter）
- 优雅下线（`terminationGracePeriodSeconds: 90` + preStop hook）
- MIG GPU 设备支持（`nvidia.com/mig-*`）
- 金丝雀模型部署，基于权重的流量分割
- llm-d 解耦式推理（实验性，prefill/decode 分离）
- 多加速器支持（nvidia、amd、gaudi）
- Envoy AI Gateway 集成 InferencePool + InferenceModel CRD（IGW 扩展）

#### 模块开关
- `global.modules.{rag,finetune,security}.enabled` 顶层开关
- Chart.yaml 双路径条件：`<subchart>.enabled,global.modules.<module>.enabled` — 显式覆盖优先
- 仪表板和 Prometheus 告警规则按模块自动包含/排除
- 19 条 Helm 模板测试（`tests/helm/test_module_switches.py`）

#### Keycloak HTTPS + K8s OIDC（Headlamp SSO）
- `keycloak.tls.enabled` 字段，支持自签（默认）或用户提供的证书
- `keycloak.tls.selfSigned` + `keycloak.tls.existingSecret` 选项
- HTTPS NodePort :30809（HTTP :30808 之外新增）
- OIDC RBAC 绑定，Headlamp → Keycloak → K8s API Server 完整链路
- 完整 k3s + 自签 CA 配置记录于 `AGENTS.md`

#### 基础设施
- `harbor` 子 chart：私有容器镜像仓库 + 模型制品仓库
- `postgresql` 独立子 chart：共享 PostgreSQL（pgvector/pgvector:pg17）— 之前嵌套在 LiteLLM 下
- `postgresql` 自动创建数据库：litellm、langfuse、dify、dify_plugin、mlflow
- 每个 DB 自动启用 `pgvector` 扩展

#### Split GGUF 支持（llama.cpp）
- llama.cpp 部署处理多分片 GGUF 文件（如 Q8_0 9 分片）
- Model-loader 新增 `allowPatterns` 字段，选择性下载（如 `"*q4_k_m*"`）
- Pod 启动 hook 创建规范命名 `{prefix}-NNNNN-of-NNNNN.gguf` 的符号链接
- `--model` 参数使用 shell wrapper 动态读取首分片路径
- `Recreate` 部署策略防止滚动更新期间 GPU 死锁
- 已在 NVIDIA GB10（Blackwell ARM64, 128GB）上使用 Gemma-4-31B Q8_0（9 分片, ~31GB）测试

#### 文档
- 6 个新文档页面（routing、large models、speculative、kserve、llm-d、canary）
- SLO 仪表板面板（TTFT/TPOT vs 阈值、HPA 副本数）
- 成本仪表板面板（GPU 空闲率、scale-to-zero 事件）
- 金丝雀仪表板面板（延迟对比、流量权重）
- vLLM 仪表板中的前缀缓存命中率面板

### 变更
- 默认路由策略：simple-shuffle → latency-based-routing
- `values-single-node.yaml` 默认 LLM：`nohurry/gemma-4-26B-A4B-it-heretic-GUFF`（llama.cpp, q4_k_m, ~16.87GB）
- vLLM 默认镜像：`vllm/vllm-openai:gemma4-cu130`（自定义构建 — 上游不支持 Gemma 4 架构）
- llama.cpp 默认镜像：`ghcr.io/ggml-org/llama.cpp:server-cuda-b8672`（多架构，支持 CUDA 13）
- 引擎自动检测：新增容错 `*GUFF*` 模式（部分 HF 仓库误拼 GGUF）
- GPU 资源名使用 helper 函数（支持 nvidia/amd/gaudi）
- DCGM exporter 仅在 nvidia 加速器下启用
- 移除自定义 dashboard — 改用 Headlamp + kube-llmops-portal 插件
- Prometheus 告警规则：5 条 → 8 条（新增 3 条 SLO 告警）
- Grafana 仪表板：10 个 → 11 个（新增 finetune-overview）

### 修复
- vLLM：移除 `--disable-access-log-for-endpoints` 参数（vLLM 0.9+ 已废弃）
- LiteLLM：`routingStrategy` 空字符串不再覆盖子 chart 默认值
- 模型名：DNS-1035 校验已记录（不能有 `.` — `qwen25-7b` ✅，`qwen2.5-7b` ❌）
- Model loader：CPU-only 模型显式设置 `gpu: 0`（之前 chart 默认 1）
- Operator build 脚本：staging 目录 `_build_charts/` 不再删除 operator 自己的 chart

## [0.4.0] - 2026-04-04

### 新增

#### 微调管道（Argo Workflows + LLaMA-Factory）
- `finetune` 子 chart 带 Argo Workflows DAG：prepare-data → finetune → merge-upload → evaluate → quality-gate → deploy
- LLaMA-Factory 集成：LoRA、QLoRA、全量微调
- 数据源：MinIO（s3://）、HuggingFace datasets、PVC 挂载
- 质量门控步骤，可配置指标阈值（eval_loss、accuracy、bleu、rouge）
- 金丝雀部署 via LiteLLM 权重路由（可配置 canary 百分比）
- 人工审批 via webhook 通知（Slack/钉钉/通用）
- 基于 ConfigMap 从 Helm values 生成训练配置
- RBAC：Argo 工作流执行的 ServiceAccount + ClusterRole
- MLflow 的 PodDisruptionBudget
- Alpaca 格式训练数据样例（`examples/finetune/sample-data.json`）

#### MLflow 实验追踪
- MLflow Deployment，PostgreSQL backend + MinIO artifact store
- 复用现有 PostgreSQL（数据库 `mlflow`）和 MinIO 基础设施
- 启用 NodePort 时通过 :30505 暴露
- 集成到微调工作流用于指标记录和模型注册表

#### JupyterHub（交互式 ML 开发）
- JupyterHub 子 chart 带 KubeSpawner，提供 GPU notebook 环境
- 3 种 GPU profile：cpu（默认）、gpu-small（1 GPU, 8Gi）、gpu-large（2 GPU, 16Gi）
- Keycloak OIDC SSO 集成（keycloak.enabled 时自动配置）
- Hub 可用性的 PodDisruptionBudget
- `global.nodePort.enabled=true` 时 NodePort :30888
- `values-production.yaml` 中默认启用

#### Terraform 模块（基础设施即代码）
- `terraform/aws-eks/` — EKS 集群带 GPU 节点组（g5.xlarge）、EBS CSI、GP3 存储
- `terraform/gcp-gke/` — GKE 标准集群带 T4 GPU 节点池、Workload Identity
- `terraform/azure-aks/` — AKS 集群带 NC6s_v3 GPU 池、Azure CNI、Premium SSD
- 所有模块：NVIDIA GPU Operator、可选 KEDA、kube-llmops Helm release
- 跨云一致的 GPU 污点（`nvidia.com/gpu=present:NoSchedule`）
- 每个模块的 README 含前置条件、成本估算、拆除说明

#### Grafana 仪表板
- 微调管道仪表板（`finetune-overview`）：作业状态、训练损失、GPU 利用率、步骤进度
- 总仪表板数：10 → 11

#### Model Loader 性能
- hf-transfer 并发从 8 提升到 32（`HF_TRANSFER_CONCURRENCY` 环境变量）
- 通过 `global.modelStore.hfTransferConcurrency` 在 values 中可配置
- 应用于 model-preload Job、model-loader init-container 和 finetune 工作流步骤

## [0.3.1] - 2026-03-29

### 新增

#### 引擎自动选择
- `resolveEngine` Helm 模板：根据模型 source 名称自动选择 vllm/tei/llamacpp
- `resolveModelType` 模板：自动检测 embedding/reranker/llm，用于 LiteLLM 路由前缀
- `global.models` 统一模型列表：一处定义所有模型，无需按子 chart 分散
- `engine:` 字段现在可选（显式设置时仍可覆盖自动检测）

#### 统一模型分发
- 预构建 `model-loader` Docker 镜像（`images/model-loader/Dockerfile`）
- 三个引擎（vllm/tei/llamacpp）均配备 model-loader init-container
- MinIO 优先下载：检查 MinIO 缓存 → 回退 HuggingFace → 上传回 MinIO
- `hf-transfer` 多线程下载（Rust 实现，速度提升 3-5 倍）
- 模型预加载 Helm Job（post-install/post-upgrade hook，批量填充 MinIO）
- `global.hfToken` 全局 HF Token（一个 Token 供所有引擎使用）
- 可配置并行下载数和每文件并发数
- 断点续传重试

#### NodePort 访问
- `global.nodePort.enabled=true` 一键暴露所有服务到固定端口（30400-30909）
- NodePort SSO：OIDC URL 根据 `global.nodePort.host` 自动计算
- 7 个服务暴露：LiteLLM、Grafana、Langfuse、Dify、Keycloak、Prometheus、MinIO

#### 系统监控
- Node Exporter DaemonSet（`quay.io/prometheus/node-exporter:v1.9.0`）
- Kube State Metrics Deployment（`rancher/mirrored-kube-state-metrics:v2.15.0`）
- System Overview Grafana 仪表盘：CPU、内存、磁盘、网络、Pod 数量、资源表
- 仪表盘总数：9 → 10

#### 开发者体验
- `examples/curl/` — API 调用示例（chat、embedding、rerank、health、traces）
- `examples/python/` — Python SDK 示例（chat、streaming、Langfuse tracing）
- RAG Quality 仪表盘新增 Prompt A/B 质量对比面板
- `/kube-llmops` Devin Skill（9 个子命令）

### 变更
- `values-single-node.yaml`：模型定义迁移至 `global.models`（原为按子 chart 分散）
- `values-single-node.yaml`：新增 `global.modelStore` MinIO 端点配置
- 所有子 chart 模板：读取 `global.models`，回退 `.Values.models`
- Prometheus：自定义 cAdvisor 采集替换为 node-exporter + kube-state-metrics
- Prometheus RBAC：权限范围调整为 pod/node/service/endpoint

### 修复
- Helm NOTES.txt 在 dify/rag-eval 未配置时 nil pointer 崩溃
- vLLM drop-cache init-container 顺序修正（移至 model-loader 之后）
- xet 协议在 arm64 大模型下载时卡死
- Prompt A/B 面板：从 barchart 切换为 bargauge（instant 查询）

## [0.3.0] - 2026-03-25

### 新增

#### RAG 基础设施（全栈）
- Dify v1.13.2 全栈部署（API + Web + Worker + Plugin Daemon + Redis）
- Dify Plugin Daemon，PVC 持久化 + `.difypkg` 插件内嵌 Secret
- 自动化 Setup Job：创建管理员账户、安装插件、配置 LLM + Embedding 模型提供者
- Dify 单域名 path-based Ingress 路由（兼容 SameSite cookie 认证）
- TEI Embedding 服务，`bge-small-en-v1.5`（384 维）自动下载
- TEI Reranking 服务，`bge-reranker-base`，`/rerank` 端点
- LiteLLM Embedding 路由（`huggingface/bge-small-en` + `drop_params: true`）

#### RAG 评估与质量
- Ragas CronJob，4 项指标：faithfulness、answer_relevancy、context_precision、context_recall
- 105 条评估样本（15 篇文档 × 9 类别，含标准答案）
- Ragas 指标 → Pushgateway → Prometheus → Grafana 流水线
- 质量门控 Helm pre-upgrade hook（质量回退时阻断部署）
- 5 条 Prometheus 告警规则：FaithfulnessLow/Critical、RelevancyLow、QualityRegression、EvalStale

#### RAG 安全与企业级功能
- LLM-Guard PromptInjection 扫描器（拦截直接 + 隐蔽注入攻击）
- Presidio PII 检测 + 脱敏（EMAIL/PERSON/URL）
- LightRAG 知识图谱 + Neo4j 后端
- Milvus 向量数据库（单机模式，gRPC + HTTP + 监控）
- 多租户 Namespace 隔离（ResourceQuota + NetworkPolicy 按团队隔离）

#### 可观测性（9 个 Grafana 仪表盘）
- RAG 质量仪表盘（4 个仪表 + 趋势 + 历史）
- 基础设施 ROI 仪表盘
- SLO 概览仪表盘
- 租户概览仪表盘
- 成本与用量仪表盘
- AlertManager 集成 + 通知渠道
- 共 9 个仪表盘（vLLM、LiteLLM、GPU、RAG Quality、Cost、SLO、Infra ROI、Tenant、Milvus）

#### 基础设施加固（27 项 CTO 改进）
- 所有 14+ 组件的 PodDisruptionBudget
- 凭证随机化 + `existingSecret` 支持
- Prometheus Kubernetes 服务发现（RBAC + `kubernetes_sd_configs`）
- PostgreSQL 拆分架构（operator-pg + app-pg）
- `values.schema.json` 配置校验
- NetworkPolicy 补全（PG/Redis/MinIO/LiteLLM）
- PostgreSQL 备份 CronJob
- External Secrets Operator 模板（Vault 后端）
- ArgoCD Application + ApplicationSet（sync waves 1-6）
- Makefile：`dev`、`lint`、`test-infra`、`bench` 目标
- 6 个架构决策记录（ADR）
- 3 个性能测试脚本 + 报告模板
- HA 生产配置（多副本 + remote_write）

#### 测试
- Playwright E2E：Model Provider（5/5 PASS）+ RAG E2E（9/9 PASS）
- Smoke Test Job：5/5 PASS（embedding + LLM + Langfuse + trace + reranker）

### 变更
- PostgreSQL 镜像：`postgres:16-alpine` → `pgvector/pgvector:pg16`，自动启用 `vector` 扩展
- PostgreSQL 初始化脚本创建 4 个数据库：litellm、langfuse、dify、dify_plugin
- `.gitignore` 更新：新增 `Chart.lock`、`screenshots/`、`test-report` 模式

### 修复
- Dify 401 认证问题：从跨域路由切换为单域名 path-based 路由（SameSite=Lax cookies）
- LiteLLM TEI Embedding：需要 `huggingface/` 前缀（不是 `openai/`），`drop_params: true`，无 `/v1` 后缀
- Helm `.tgz` 缓存：子 chart 模板修改被过期的归档文件覆盖

## [0.2.0] - 2026-03-21

### 新增

#### LLM 追踪（Langfuse v3）

- Langfuse v2 → v3（3.160.0）升级，完整基础设施栈
- ClickHouse（24.12-alpine）用于 OLAP 追踪/分析存储
- Redis（7-alpine）用于异步工作队列
- S3/MinIO 集成，用于事件和媒体 blob 存储
- `ENCRYPTION_KEY` 支持敏感数据加密
- MCP（Model Context Protocol）提示词功能

#### 基础设施自动化

- PostgreSQL `extraDatabases` 通过 `/docker-entrypoint-initdb.d/` 自动创建
- MinIO `defaultBuckets` 启动时自动创建（先 mkdir 再启动服务）
- 幂等初始化脚本（重启安全，使用 IF NOT EXISTS）

#### Keycloak SSO

- Keycloak Helm 子 chart，自动配置 realm、客户端、角色和用户
- 为 Grafana、Langfuse、MinIO、LiteLLM 创建 OIDC 客户端
- 所有服务的 Traefik Ingress（`*.llmops.local`）

### 变更

- Langfuse 镜像：`2.95.11` → `3.160.0`
- 父 chart 改用子 chart 默认 tag，不再使用 `latest`
- 移除过期的 `.tgz` chart 包（Helm 改用目录源）

### 修复

- Langfuse v3 启动时 ZodError（根因：缺少 S3 blob 存储配置）
- ClickHouse 单节点部署（`CLICKHOUSE_CLUSTER_ENABLED=false`）
- vLLM Blackwell GPU 崩溃：启用 `--enforce-eager` + `--attention-backend TRITON_ATTN`
- PostgreSQL `langfuse` 数据库在全新部署时未自动创建

## [0.1.0] - 2026-03-19

### 新增

#### 模型服务

- vLLM 子 chart，支持 GPU、模型缓存（PVC）、自定义 CA 证书
- llama.cpp 子 chart，用于 GGUF 模型服务
- TEI 子 chart，用于嵌入模型服务
- Model Resolver：自动检测模型格式（GGUF→llama.cpp，GPTQ/AWQ→vLLM，embedding→TEI）
- GPU 工作负载采用 Recreate 部署策略（防止滚动更新死锁）
- 支持按模型配置 `extraEnv` 和 `engineArgs`

#### AI 网关

- LiteLLM 子 chart，使用 PostgreSQL 后端
- 从 `models[]` values 自动生成 LiteLLM 配置
- API 密钥认证（master key）
- 多模型路由，采用 simple-shuffle 策略
- 兼容 OpenAI 的 `/v1/chat/completions` 端点

#### 可观测性

- Prometheus，支持远程写入接收器
- Grafana，自动配置 3 个仪表盘（vLLM、LiteLLM Gateway、GPU）
- OpenTelemetry Collector（Prometheus 采集 + OTLP 接收器）
- DCGM Exporter，用于 NVIDIA GPU 指标（可选）
- Grafana 中自动配置 Loki 数据源

#### LLM 追踪

- Langfuse v2，支持自动配置（LANGFUSE_INIT_* 环境变量）
- LiteLLM → Langfuse 回调（追踪模型、token 数、延迟、成本）
- 可配置外部 URL，支持 port-forward/ingress

#### 日志

- Fluent Bit DaemonSet，用于容器日志采集
- Loki，用于日志存储和查询
- Grafana Loki 数据源，用于日志浏览

#### 自动扩缩容（模板，需要 KEDA operator）

- 为每个 vLLM 模型部署创建 KEDA ScaledObject
- Prometheus 触发器：等待中的请求数、TTFT P95

#### 分布式缓存（模板，需要 Fluid operator）

- MinIO，用于 S3 兼容的模型存储
- 为每个模型创建 Fluid Dataset + AlluxioRuntime

#### 模型注册中心（模板，需要 Harbor）

- Harbor 凭证 ConfigMap + Secret
- OCI 模型源集成点

#### 安全（模板）

- NetworkPolicy：默认拒绝 + 按组件配置允许规则
- OIDC/SSO ConfigMap，用于 Keycloak/Dex 集成
- Grafana OIDC 自动配置

#### 基础设施

- Umbrella Helm chart，包含 14 个子 chart
- 4 种部署配置：ci、minimal、standard、production
- 一键安装脚本（`scripts/install.sh`）
- 3 个 CI 工作流：lint、test、build
- 完善的 README，包含凭证信息表

### 修复

- LiteLLM api_base 缺少 `/v1` 后缀（导致所有模型路由失败）
- Grafana 仪表盘 PVC 路径冲突
- Langfuse Next.js 未绑定到 0.0.0.0（导致 port-forward 失败）
- Langfuse NEXTAUTH_URL 重定向到内部 Kubernetes URL
- GPU 滚动更新死锁（改用 Recreate 策略）

### 已知问题

- DCGM Exporter 在 WSL2 环境下可能无法正常工作
- Helm SSA 在升级时可能无法更新 ConfigMap（解决方法：先删除 ConfigMap）
