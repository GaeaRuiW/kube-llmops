# Phase 3: 演进与护城河 --- 详细 Story 拆解

> **阶段目标**: 建立竞争壁垒、支持企业级场景、拓展生态
> **时间窗口**: 2-3 个月 (P2 级)
> **原则**: 功能扩展必须伴随测试覆盖和文档更新,不再产生新的"承诺-实现"差距
> **前置条件**: Phase 1 + Phase 2 全部完成并验证通过

---

## Story 3.1: 有状态组件高可用 (HA) 加固

### 标签
`priority/P2` `area/architecture` `kind/feature` `effort/8d`

### 为什么要做 (Why)

架构师评审 (FATAL-03) 指出:即使在 `values-production.yaml` 中,**所有有状态组件均为单副本**:

| 组件 | 当前副本数 | 故障影响 |
|------|-----------|---------|
| PostgreSQL | 1 | 全平台瘫痪 (litellm/langfuse/dify 全部宕机) |
| Keycloak | 1 | 无法新登录,SSO Token 刷新失败 |
| Prometheus | 1 | 告警失明,仪表盘空白 |
| Grafana | 1 | 无法访问仪表盘 |
| ClickHouse | 1 | Langfuse 追踪摄入停止 |
| Loki | 1 | 日志摄入停止 |
| MinIO | 1 | 对象存储不可用 |

Phase 2 拆分 PostgreSQL 为 2 实例降低了爆炸半径,但每个实例仍是单副本。一次 Pod 重启就能导致对应功能组瘫痪。对于企业客户而言,有状态组件的 HA 是采购评估的 Day-0 硬性要求。

### 要做什么 (What)

**任务 1: PostgreSQL HA (主从复制)**

两个方案选其一:

| 方案 | 优点 | 缺点 | 推荐度 |
|------|------|------|--------|
| CloudNativePG Operator | 声明式管理、自动 failover、PITR 备份 | 需要安装外部 Operator | 生产首选 |
| Bitnami PostgreSQL Chart | 内置 replication.enabled、无外部依赖 | failover 需手动、功能较基础 | 快速方案 |

推荐方案: 短期使用 Bitnami PostgreSQL Chart (降低依赖),长期提供 CloudNativePG 的文档和 values overlay。

```yaml
# values-production.yaml
postgresql:
  architecture: replication
  primary:
    resources:
      requests: { cpu: 500m, memory: 1Gi }
  readReplicas:
    replicaCount: 1
    resources:
      requests: { cpu: 250m, memory: 512Mi }
```

**任务 2: Prometheus HA (VictoriaMetrics 或 Thanos Sidecar)**

单实例 Prometheus 上限约 200 万活跃时序。对于 50+ 工程师的团队,这个上限会在数周内暴露。

方案: 引入 VictoriaMetrics 作为远端存储,Prometheus 通过 `remote_write` 写入:
```yaml
# prometheus.yaml ConfigMap
remote_write:
  - url: "http://{{ .Release.Name }}-victoriametrics:8428/api/v1/write"
```

VictoriaMetrics 支持多副本、数据压缩 (10x)、长期保留。

**任务 3: MinIO 分布式模式**

当前 MinIO 为单节点 StatefulSet。分布式模式至少需要 4 节点:
```yaml
minio:
  mode: distributed
  replicas: 4
  resources:
    requests: { cpu: 250m, memory: 512Mi }
```

仅在 production profile 启用。

**任务 4: 外部数据库文档**

创建 `docs/guides/external-databases.md`:
- 如何配置指向 AWS RDS / Cloud SQL / Azure Database
- 连接字符串格式
- 所需的数据库/用户/扩展 (pgvector)
- 网络配置 (VPC peering, Security Group)

### 验收标准 (DoD)

- [ ] `values-production.yaml` 启用 PG HA 后,`kubectl get pods` 显示 1 primary + 1 replica
- [ ] 手动删除 PG primary Pod 后,replica 自动提升为 primary (如使用 CloudNativePG) 或服务自动恢复
- [ ] Prometheus `remote_write` 到 VictoriaMetrics 后,历史指标在 VM 中可查询
- [ ] MinIO 分布式模式 4 节点部署后,删除 1 个节点数据不丢失
- [ ] 外部数据库文档就绪,包含 AWS/GCP/Azure 三种云的配置示例
- [ ] 全部 E2E 测试在 HA 配置下通过

---

## Story 3.2: 全栈可观测性深耕 --- 从 GPU 到 Token 到成本的统一视图

### 标签
`priority/P2` `area/observability` `kind/feature` `effort/5d`

### 为什么要做 (Why)

PM 评估报告指出 "全栈可观测性" 是 kube-llmops **最有潜力的中等护城河**。竞品 (vLLM 裸部署、KAITO、KServe) 通常只关注单一维度的指标,而 kube-llmops 有机会提供从 GPU 硬件到 Token 消耗到业务成本的统一可观测视图。这是企业客户最看重的差异化能力。

当前的可观测性覆盖已有良好基础 (5 个 Dashboard, 11 条告警规则),但缺少:
- GPU → Token → 成本的**关联分析** (一个 GPU 小时产出了多少 Token? 成本效率如何?)
- Prompt A/B 测试指标 (差距清单 #57)
- SLO 框架 (差距清单 #52)
- 结构化日志解析 (vLLM/LiteLLM 的 Request ID / Token 级字段)

### 要做什么 (What)

**任务 1: GPU-to-Token-to-Cost 关联 Dashboard**

创建 "Infrastructure ROI" Dashboard:

| Panel | 数据来源 | 说明 |
|-------|---------|------|
| GPU 利用率 vs Token 吞吐量 | DCGM + vLLM metrics | 散点图,显示 GPU 利用率与 token/s 的关系 |
| 每 GPU-hour 产出 Token 数 | 计算指标 | 衡量 GPU 成本效率 |
| 模型性价比排行 | LiteLLM spend + vLLM throughput | 各模型的 $/1K tokens 排行 |
| TTFT / TPS 趋势 (P50/P95/P99) | vLLM metrics | 延迟分布 |
| KV Cache 利用率 & 命中率 | vLLM metrics | 内存效率 |
| 队列深度 vs GPU 利用率 | vLLM + DCGM | 容量规划 |

**任务 2: Prompt A/B 测试指标 Panel**

在 Langfuse 中,不同 prompt 版本通过 `metadata.prompt_version` 标识。创建 Dashboard:

| Panel | 说明 |
|-------|------|
| 各 Prompt 版本的平均延迟 | 按 prompt_version 分组 |
| 各 Prompt 版本的 Token 消耗 | prompt tokens + completion tokens |
| 各 Prompt 版本的用户满意度 (如有反馈) | Langfuse scores |
| Ragas 质量指标按版本对比 | faithfulness / relevancy 趋势 |

**任务 3: SLO 框架**

创建 `docs/guides/slo-guide.md` + Grafana SLO Dashboard:

| SLO | 目标 | 指标 |
|-----|------|------|
| LLM 推理可用性 | 99.9% | `rate(vllm_requests_success) / rate(vllm_requests_total)` |
| P95 TTFT | < 2s | `histogram_quantile(0.95, vllm_time_to_first_token)` |
| P95 TPS | > 30 tokens/s | `histogram_quantile(0.95, vllm_tokens_per_second)` |
| RAG 质量 | faithfulness > 0.7 | `ragas_faithfulness` |

SLO Dashboard 包含: 30 天滚动窗口的 SLI 趋势、Error Budget 剩余、Burn Rate 告警。

**任务 4: 结构化日志解析**

在 Fluent Bit 配置中添加针对 vLLM 和 LiteLLM 的日志解析器:
```ini
[PARSER]
    Name   litellm_json
    Format json
    Time_Key timestamp
    
[FILTER]
    Name   parser
    Match  kube.litellm.*
    Parser litellm_json
    Key_Name log
```

提取字段: `request_id`, `model`, `tokens_prompt`, `tokens_completion`, `latency_ms`, `api_key`。

### 验收标准 (DoD)

- [ ] Grafana 新增 "Infrastructure ROI" Dashboard,含 GPU-to-Token-to-Cost 关联 Panel
- [ ] Grafana 新增 "Prompt A/B Testing" Dashboard
- [ ] Grafana 新增 "SLO Overview" Dashboard (可用性、延迟、质量)
- [ ] `docs/guides/slo-guide.md` 包含 SLO 定义、告警配置、Error Budget 解读
- [ ] Fluent Bit 解析 LiteLLM JSON 日志,Loki 中可按 request_id 查询
- [ ] Grafana Dashboard 总数从 5 个增加到 8+ 个

---

## Story 3.3: External Secrets Operator 集成

### 标签
`priority/P2` `area/security` `kind/feature` `effort/3d`

### 为什么要做 (Why)

Phase 1 (Story 1.2) 通过 `existingSecret` 提供了手动管理 Secret 的能力。但在企业环境中,Secret 通常存储在集中化的密钥管理服务中 (AWS Secrets Manager, HashiCorp Vault, Azure Key Vault)。手动创建 K8s Secret 然后通过 `existingSecret` 引用是一种运维负担。

External Secrets Operator (ESO) 可以自动将外部密钥管理服务中的 Secret 同步到 K8s Secret,实现声明式的密钥管理。这是企业安全合规的阻塞项 --- 很多企业有"所有密钥必须存储在 Vault"的硬性策略。

ARCHITECTURE.md (第 813 行) 提到了 ESO 但零实现。

### 要做什么 (What)

**任务 1: ESO ExternalSecret 模板**

在每个子 Chart 中添加可选的 `external-secret.yaml` 模板:

```yaml
{{- if .Values.externalSecrets.enabled }}
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: {{ .Release.Name }}-litellm-external
spec:
  refreshInterval: {{ .Values.externalSecrets.refreshInterval | default "1h" }}
  secretStoreRef:
    name: {{ .Values.externalSecrets.secretStore }}
    kind: {{ .Values.externalSecrets.secretStoreKind | default "ClusterSecretStore" }}
  target:
    name: {{ .Release.Name }}-litellm-secret
    creationPolicy: Owner
  data:
    - secretKey: master-key
      remoteRef:
        key: {{ .Values.externalSecrets.keyPrefix }}/litellm/master-key
    - secretKey: postgresql-password
      remoteRef:
        key: {{ .Values.externalSecrets.keyPrefix }}/postgresql/password
{{- end }}
```

**任务 2: SecretStore 参考实现**

提供 AWS Secrets Manager 和 HashiCorp Vault 的 SecretStore 配置示例:

`docs/guides/external-secrets.md`:
```yaml
# AWS Secrets Manager
apiVersion: external-secrets.io/v1beta1
kind: ClusterSecretStore
metadata:
  name: aws-secrets-manager
spec:
  provider:
    aws:
      service: SecretsManager
      region: us-east-1
      auth:
        jwt:
          serviceAccountRef:
            name: external-secrets-sa
---
# HashiCorp Vault
apiVersion: external-secrets.io/v1beta1
kind: ClusterSecretStore
metadata:
  name: hashicorp-vault
spec:
  provider:
    vault:
      server: "https://vault.example.com"
      path: "secret"
      auth:
        kubernetes:
          mountPath: "kubernetes"
          role: "kube-llmops"
```

**任务 3: values.yaml 配置**

```yaml
externalSecrets:
  enabled: false
  secretStore: ""            # ClusterSecretStore 名称
  secretStoreKind: "ClusterSecretStore"
  refreshInterval: "1h"
  keyPrefix: "kube-llmops"   # 远端密钥路径前缀
```

**任务 4: 确保与 Phase 1 existingSecret 兼容**

优先级: `externalSecrets.enabled` > `existingSecret` > 自动生成。三种模式互斥,模板中用条件判断。

### 验收标准 (DoD)

- [ ] 启用 ESO 后,`helm template` 渲染出 ExternalSecret 资源 (不渲染内置 Secret)
- [ ] 文档包含 AWS Secrets Manager + HashiCorp Vault 两种 SecretStore 配置示例
- [ ] ESO 同步的 Secret 被 Deployment 正确引用
- [ ] `externalSecrets.enabled: false` 时行为不变 (向后兼容)
- [ ] `existingSecret` 与 `externalSecrets` 互斥检查 (同时设置时 lint 报错)

---

## Story 3.4: Model Resolver 集成到 vLLM 部署流程

### 标签
`priority/P2` `area/feature` `kind/enhancement` `effort/3d`

### 为什么要做 (Why)

README.md Features 对比表中 "Engine auto-selection (GPTQ→vLLM, GGUF→llama.cpp)" 标为 `Yes`。实际上 Model Resolver 代码存在且有 28 个单测,但**从未作为 init-container 集成到 vLLM Deployment 模板中**。

当前用户必须在 `values.yaml` 中手动指定 `engine: vllm` 或 `engine: llamacpp`。对于不熟悉模型格式的用户 (如不知道 GPTQ 模型应该用 vLLM 还是 llama.cpp),这是一个高摩擦点。

Phase 1 已将 README 标注修正为 `Partial`,Phase 3 需要完成实际集成,兑现承诺。

### 要做什么 (What)

**任务 1: 将 Model Resolver 镜像构建集成到 CI**

确认 `.github/workflows/build.yaml` 已构建 `model-resolver` 镜像并推送到 GHCR。

**任务 2: 在 vLLM Deployment 中添加 init-container**

修改 `charts/vllm/templates/deployment.yaml`:

```yaml
initContainers:
  {{- if .Values.autoDetect.enabled }}
  - name: model-resolver
    image: {{ .Values.autoDetect.image }}:{{ .Values.autoDetect.tag }}
    env:
      - name: MODEL_ID
        value: {{ .modelId }}
      - name: HF_TOKEN
        valueFrom:
          secretKeyRef:
            name: {{ .Release.Name }}-hf-token
            key: token
            optional: true
    command:
      - python
      - -m
      - model_resolver
      - --model-id={{ .modelId }}
      - --output=/shared/engine-config.json
    volumeMounts:
      - name: shared
        mountPath: /shared
  {{- end }}
  - name: model-loader
    # ... 现有的模型下载 init-container
```

主容器读取 `/shared/engine-config.json` 获取引擎参数 (quantization, dtype, max_model_len 等)。

**任务 3: values.yaml 配置**

```yaml
autoDetect:
  enabled: false          # 默认关闭,向后兼容
  image: ghcr.io/<repo>/model-resolver
  tag: latest
```

**任务 4: E2E 测试**

添加测试: 不指定 `engine` 时,Model Resolver 自动检测 GPTQ 模型并选择 vLLM。

### 验收标准 (DoD)

- [ ] `autoDetect.enabled: true` 时,vLLM Deployment 包含 `model-resolver` init-container
- [ ] Model Resolver 正确检测 GPTQ 模型格式并输出 vLLM 引擎配置
- [ ] Model Resolver 正确检测 GGUF 模型格式并输出 llama.cpp 引擎配置
- [ ] `autoDetect.enabled: false` 时行为不变 (用户手动指定 engine)
- [ ] README Features 表可更新为 `Yes` (不再是 Partial)
- [ ] Model Resolver 28 个单测 + 新增集成测试全部通过

---

## Story 3.5: RAG 质量保障深耕

### 标签
`priority/P2` `area/rag` `kind/feature` `effort/5d`

### 为什么要做 (Why)

PM 评估报告指出 "评估与质量门控" 是**潜在的强护城河** --- 这是其他竞品都没有做到的能力。当前已有:
- Ragas CronJob: 4 指标 (faithfulness, relevancy, precision, recall)
- Quality Gate: pre-upgrade hook 检查阈值
- Grafana Dashboard: 5 条 RAG 质量告警规则

但这只是起点。要建立真正的竞争壁垒,需要:
1. 更丰富的评估维度
2. 自动化的数据更新和回归验证
3. 与 CI/CD 集成的质量门禁

### 要做什么 (What)

**任务 1: 扩展 Ragas 评估维度**

在 `charts/rag-eval/templates/ragas-cronjob.yaml` 中增加指标:

| 指标 | 说明 | 阈值 |
|------|------|------|
| `answer_correctness` | 答案与 ground truth 的正确度 | ≥ 0.7 |
| `context_utilization` | 上下文利用效率 | ≥ 0.6 |
| `hallucination_rate` | 幻觉率 (答案中无依据的内容占比) | ≤ 0.2 |
| `latency_p95` | RAG 端到端 P95 延迟 | ≤ 5s |

**任务 2: 数据更新 Pipeline**

创建 `charts/rag-eval/templates/data-update-cronjob.yaml`:
- 定期从指定源 (Git repo, S3 bucket) 拉取最新的评估数据集
- 更新 Dify 知识库中的文档
- 触发 Ragas 评估
- 比较新旧数据集的质量指标差异

**任务 3: 模型热换回归测试**

创建 `charts/rag-eval/templates/model-regression-job.yaml`:
- 当 vLLM 模型更新时 (通过 Helm upgrade),自动触发 RAG 回归测试
- 如果新模型的质量指标低于旧模型,告警通知
- 可配置为阻断升级 (与 Quality Gate 集成)

**任务 4: 扩展评估数据集**

当前 105 个样本。扩展到 500+ 样本,覆盖:
- 多语言 (中/英)
- 多领域 (技术文档, 法律, 金融)
- 边界情况 (长文档, 多跳推理, 模糊查询)

**任务 5: 质量仪表盘增强**

在 Grafana RAG Quality Dashboard 中增加:
- 各维度指标的历史趋势 (30 天滚动)
- 指标间的相关性分析
- 数据集版本与质量的关联

### 验收标准 (DoD)

- [ ] Ragas CronJob 评估 7+ 个质量维度
- [ ] 数据更新 Pipeline 可自动拉取新数据并触发评估
- [ ] 模型更新后自动运行回归测试,质量下降时告警
- [ ] 评估数据集扩展到 500+ 样本
- [ ] Grafana Dashboard 显示扩展后的质量维度趋势

---

## Story 3.6: 开发者体验提升

### 标签
`priority/P2` `area/dx` `kind/enhancement` `effort/3d`

### 为什么要做 (Why)

架构师评审指出开发者体验的几个痛点:

1. **`.tgz` 缓存陷阱**: 编辑子 Chart 模板后必须运行 `helm dependency update`,否则 Helm 使用旧的 `.tgz` 归档。这在 AGENTS.md 中已记录为 "Critical Gotcha",说明是高频发生的问题。

2. **无本地开发环境**: 没有 Tilt/Skaffold/DevSpace 配置。开发者每次修改都需要完整的 `helm upgrade` 周期,迭代速度慢。

3. **无架构决策记录 (ADR)**: 技术选型的 "为什么" 散落在 ARCHITECTURE.md 和 commit message 中,新贡献者无法理解历史决策的上下文。

### 要做什么 (What)

**任务 1: Tilt 本地开发环境**

创建 `Tiltfile`:
```python
# Tiltfile
load('ext://helm_resource', 'helm_resource')

helm_resource(
  'kube-llmops',
  'charts/kube-llmops-stack',
  flags=['--values=charts/kube-llmops-stack/values-minimal.yaml'],
  deps=['charts/kube-llmops-stack/charts/'],
)

# 监听子 Chart 变更,自动 helm dependency update + upgrade
watch_file('charts/kube-llmops-stack/charts/')
```

**任务 2: pre-commit hook 自动 helm dependency update**

创建 `.pre-commit-config.yaml`:
```yaml
repos:
  - repo: local
    hooks:
      - id: helm-dependency-update
        name: Rebuild Helm chart archives
        entry: bash -c 'cd charts/kube-llmops-stack && rm -f charts/*.tgz Chart.lock && helm dependency update .'
        language: system
        files: 'charts/kube-llmops-stack/charts/.*/templates/.*'
```

**任务 3: ADR 模板和初始决策记录**

创建 `docs/adr/` 目录:

```
docs/adr/
├── 0001-umbrella-helm-chart-pattern.md
├── 0002-vllm-over-sglang-for-inference.md
├── 0003-litellm-as-ai-gateway.md
├── 0004-langfuse-over-jaeger-for-tracing.md
├── 0005-dify-as-rag-platform.md
├── 0006-pgvector-as-default-vector-store.md
└── TEMPLATE.md
```

ADR 模板:
```markdown
# ADR-XXXX: <标题>
## 状态: Accepted / Superseded / Deprecated
## 上下文: 面对什么问题?
## 决策: 选择了什么方案?
## 理由: 为什么选这个?考虑了哪些替代方案?
## 影响: 对系统有什么影响?
```

**任务 4: Makefile 增强**

在 `Makefile` 中添加:
```makefile
dev:          ## Start Tilt local development
	tilt up

dep-update:   ## Rebuild Helm chart archives (after subchart changes)
	cd charts/kube-llmops-stack && rm -f charts/*.tgz Chart.lock && helm dependency update .

lint:         ## Run all linters
	helm lint charts/kube-llmops-stack -f charts/kube-llmops-stack/values-single-node.yaml
	
test-infra:   ## Run infrastructure verification tests
	bash improvement/test/scripts/01-deploy-verify.sh
```

### 验收标准 (DoD)

- [ ] `tilt up` 启动本地开发环境,修改子 Chart 模板后自动 upgrade
- [ ] pre-commit hook 在子 Chart 模板变更时自动运行 `helm dependency update`
- [ ] `docs/adr/` 包含至少 6 个初始 ADR + 模板
- [ ] `make dev` / `make dep-update` / `make lint` / `make test-infra` 命令可用
- [ ] CONTRIBUTING.md 更新,包含 Tilt 开发环境的使用说明

---

## Story 3.7: GitOps 集成 (ArgoCD ApplicationSet)

### 标签
`priority/P2` `area/operations` `kind/feature` `effort/3d`

### 为什么要做 (Why)

ARCHITECTURE.md (第 280-291 行) 描述了 ArgoCD Sync Waves 的部署编排方案,但 `manifests/argocd/` 目录仅有 `.gitkeep`,零实现 (差距清单 #9)。

GitOps 是企业客户部署 K8s 应用的标准方式。没有 ArgoCD 集成意味着:
- 企业客户需要自己编写 Application manifest,这是额外的集成成本
- 无法利用 ArgoCD 的 Sync Waves 保证组件部署顺序 (PG → LiteLLM → vLLM → Dify)
- 无法通过 Git PR 审批流程控制部署变更

### 要做什么 (What)

**任务 1: ArgoCD Application manifest**

创建 `manifests/argocd/application.yaml`:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: kube-llmops
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/<repo>.git
    targetRevision: HEAD
    path: charts/kube-llmops-stack
    helm:
      valueFiles:
        - values-production.yaml
  destination:
    server: https://kubernetes.default.svc
    namespace: kube-llmops
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
```

**任务 2: ApplicationSet (多环境)**

创建 `manifests/argocd/applicationset.yaml`:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: kube-llmops-environments
spec:
  generators:
    - list:
        elements:
          - env: staging
            values: values-minimal.yaml
            cluster: staging-cluster
          - env: production
            values: values-production.yaml
            cluster: production-cluster
  template:
    metadata:
      name: 'kube-llmops-{{env}}'
    spec:
      source:
        helm:
          valueFiles:
            - '{{values}}'
      destination:
        server: '{{cluster}}'
        namespace: 'kube-llmops-{{env}}'
```

**任务 3: Sync Waves 注解**

在子 Chart 模板中添加 ArgoCD Sync Wave 注解,确保部署顺序:

| Wave | 组件 |
|------|------|
| 0 | PostgreSQL, Redis, MinIO |
| 1 | Keycloak |
| 2 | LiteLLM, Langfuse |
| 3 | vLLM, TEI |
| 4 | Dify, LightRAG |
| 5 | Observability, Security |
| 6 | Smoke Test |

```yaml
metadata:
  annotations:
    argocd.argoproj.io/sync-wave: "0"
```

**任务 4: 文档**

创建 `docs/guides/gitops-argocd.md`:
- ArgoCD 安装前提
- Application 部署步骤
- 多环境 ApplicationSet 配置
- Sync Wave 工作原理
- 故障排查

### 验收标准 (DoD)

- [ ] `manifests/argocd/` 包含 Application + ApplicationSet 配置
- [ ] ArgoCD 安装后,apply Application manifest 可成功部署 kube-llmops
- [ ] Sync Waves 确保 PostgreSQL 在 LiteLLM 之前部署完成
- [ ] ApplicationSet 支持 staging + production 双环境
- [ ] `docs/guides/gitops-argocd.md` 文档就绪

---

## Story 3.8: 多租户成熟化

### 标签
`priority/P2` `area/feature` `kind/feature` `effort/5d`

### 为什么要做 (Why)

Phase 4 已实现基础的多租户隔离 (2 个 team namespace + ResourceQuota + NetworkPolicy)。但这只是"隔离"层面的多租户,缺少"管理"和"计量"层面的能力:

1. **租户管理**: 创建/删除租户需要手动创建 Namespace + ResourceQuota + NetworkPolicy,没有声明式配置
2. **资源计量**: 无法看到各租户的实际 GPU/CPU/Memory 使用量
3. **计费集成**: 无法基于资源使用量生成计费报告
4. **自助服务**: 租户管理员无法自行管理 API Key 和预算

这些是从"开发者工具"进化为"企业平台"的关键能力。

### 要做什么 (What)

**任务 1: 租户 CRD 或 Helm Values 声明**

在 `values.yaml` 中定义租户列表:

```yaml
multiTenant:
  enabled: true
  teams:
    - name: team-alpha
      namespace: team-alpha
      resourceQuota:
        gpu: 2
        cpu: "16"
        memory: 32Gi
        pods: 50
      litellm:
        apiKey: ""         # 自动生成
        budgetLimit: 1000  # 月预算 ($)
    - name: team-beta
      namespace: team-beta
      resourceQuota:
        gpu: 1
        cpu: "8"
        memory: 16Gi
```

**任务 2: 租户自动化 Provisioning**

创建 `charts/kube-llmops-stack/templates/tenants.yaml`:
- 自动创建 Namespace
- 自动创建 ResourceQuota
- 自动创建 NetworkPolicy (租户间隔离)
- 自动在 LiteLLM 中创建对应的 API Key 和预算

**任务 3: 租户资源使用 Dashboard**

在 Grafana 中新增 "Tenant Overview" Dashboard:

| Panel | 说明 |
|-------|------|
| 各租户 GPU 使用率 | `sum by (namespace)(kube_pod_container_resource_requests{resource="nvidia.com/gpu"})` |
| 各租户 CPU/Memory 使用率 | Kubernetes 指标按 namespace 分组 |
| 各租户 Token 消耗 | LiteLLM 按 API Key 分组 |
| 各租户预算消耗进度 | 月预算使用百分比 |
| ResourceQuota 使用率 | 各 namespace 的配额使用情况 |

**任务 4: 计费 API (可选)**

创建简单的计费数据导出:
- CronJob 定期汇总各租户的资源使用量
- 输出 CSV/JSON 报告到 MinIO
- 可被外部计费系统消费

### 验收标准 (DoD)

- [ ] `values.yaml` 中定义 2 个 tenant 后,`helm install` 自动创建对应 Namespace + ResourceQuota + NetworkPolicy
- [ ] 各租户间 Pod 网络隔离 (team-alpha 无法访问 team-beta 的 Pod)
- [ ] Grafana "Tenant Overview" Dashboard 按租户显示资源使用量
- [ ] LiteLLM 中各租户有独立的 API Key 和预算
- [ ] 文档说明多租户的配置和管理流程

---

## Story 3.9: 性能基线与压力测试

### 标签
`priority/P2` `area/testing` `kind/feature` `effort/3d`

### 为什么要做 (Why)

项目声称"生产级",但没有任何性能数据支撑。架构师评审中的扩展上限分析全部基于理论估算:

| 组件 | 理论上限 | 实测数据 |
|------|---------|---------|
| Prometheus | ~200 万活跃时序 | **无** |
| Loki | ~10MB/s 摄入 | **无** |
| LiteLLM | 受限于 PG 连接数 | **无** |
| vLLM | 受限于 GPU 数量 | **无** |

没有性能基线意味着:
1. 无法给用户提供 sizing 建议 (多少 GPU 支撑多少并发)
2. 无法发现性能回归 (新版本是否比旧版本慢)
3. "生产级"声明缺乏数据支撑

### 要做什么 (What)

**任务 1: 创建压力测试套件**

使用 k6 或 Locust 创建 `tests/load/`:

```javascript
// tests/load/vllm-inference.js (k6)
import http from 'k6/http';

export const options = {
  stages: [
    { duration: '2m', target: 10 },   // 预热
    { duration: '5m', target: 50 },   // 正常负载
    { duration: '5m', target: 100 },  // 峰值负载
    { duration: '2m', target: 0 },    // 恢复
  ],
};

export default function () {
  const res = http.post('http://litellm:4000/v1/chat/completions', 
    JSON.stringify({
      model: 'qwen2-5-0-5b',
      messages: [{ role: 'user', content: 'What is 2+2?' }],
      max_tokens: 50,
    }),
    { headers: { 'Authorization': 'Bearer sk-...', 'Content-Type': 'application/json' } }
  );
  check(res, { 'status is 200': (r) => r.status === 200 });
}
```

**任务 2: 性能基线测试矩阵**

| 测试场景 | 并发 | 持续时间 | 采集指标 |
|---------|------|---------|---------|
| LLM 推理 (短 prompt) | 1/10/50/100 | 5min | TTFT P50/P95/P99, TPS, 错误率 |
| LLM 推理 (长 prompt) | 1/10/50 | 5min | 同上 |
| Embedding 生成 | 10/50/100/500 | 5min | 延迟 P50/P95/P99, 吞吐量 |
| RAG 端到端 | 1/10/50 | 5min | 检索延迟, 生成延迟, 总延迟 |
| Langfuse 追踪写入 | 100/500/1000 events/s | 10min | 写入延迟, 队列深度 |

**任务 3: 性能报告模板**

创建 `docs/guides/performance-report.md`:
- 测试环境配置 (GPU 型号, CPU, Memory)
- 各场景的基线数据
- 瓶颈分析和 sizing 建议
- 与裸 vLLM 部署的性能对比 (量化 kube-llmops 的开销)

**任务 4: CI 集成 (可选)**

在 e2e.yaml 中添加性能回归检测:
```yaml
- name: Run performance baseline
  run: |
    k6 run tests/load/vllm-inference.js --out json=results.json
    # 检查 P95 延迟不超过基线的 120%
    python tests/load/check_regression.py results.json
```

### 验收标准 (DoD)

- [ ] `tests/load/` 包含至少 3 个压力测试脚本 (LLM 推理, Embedding, RAG E2E)
- [ ] 每个测试场景有明确的并发梯度和采集指标
- [ ] `docs/guides/performance-report.md` 包含基线数据和 sizing 建议
- [ ] 性能报告包含"kube-llmops 开销"对比 (vs 裸 vLLM)
- [ ] Makefile 中 `make bench` 命令可一键运行压力测试
