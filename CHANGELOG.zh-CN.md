[English](CHANGELOG.md) | **中文**

# 更新日志

本文件记录了本项目的所有重要变更。

本文件格式基于 [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)，
并且本项目遵循 [语义化版本](https://semver.org/spec/v2.0.0.html) 规范。

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

- Keycloak 部署 + 初始化脚本（`scripts/init-keycloak.sh`）
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

- Umbrella Helm chart，包含 12 个子 chart
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
