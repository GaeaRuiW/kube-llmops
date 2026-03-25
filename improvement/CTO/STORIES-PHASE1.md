# Phase 1: 救火与基建 --- 详细 Story 拆解

> **阶段目标**: 消除全部安全硬伤、建立基本生产存活能力、修复信任危机
> **时间窗口**: 2 周 (P0 级)
> **总工作量**: ~6-7 人天
> **原则**: 只做"高回报/低成本"的修复,不做架构级重构

---

## Story 1.1: 全组件 PodDisruptionBudget

### 标签
`priority/P0` `area/reliability` `kind/hardening` `effort/0.5d`

### 为什么要做 (Why)

当前整个代码库中 PDB 数量为**零** (`grep -r "PodDisruptionBudget" charts/` 返回空)。在 Kubernetes 环境中,节点排空 (`kubectl drain`) 是常规运维操作 --- 滚动更新、Spot 实例回收、硬件维护都会触发。没有 PDB 保护时,一个组件的**所有副本可能被同时驱逐**,造成不可控的服务中断。

对于本项目的影响尤其严重:PostgreSQL 只有 1 个副本,且承载了 litellm/langfuse/dify/dify_plugin 四个数据库。一次 `kubectl drain` 如果驱逐了 PG Pod,**整个平台瞬间瘫痪**。即使 LiteLLM/Langfuse 配置了 2 副本(production profile),没有 PDB 也可能被同时驱逐。

这是**成本最低、回报最高**的修复 --- 半天工作量,立即消除 node drain 导致的全平台宕机风险。

### 要做什么 (What)

**任务 1: 创建 PDB 模板文件**

为以下 16 个子 Chart 的每个 Deployment/StatefulSet 创建 `pdb.yaml` 模板:

| 子 Chart | 组件 | PDB 策略 |
|----------|------|----------|
| `litellm/` | litellm Deployment | `minAvailable: 1` (production 有 2 副本) |
| `litellm/` | postgresql Deployment | `minAvailable: 1` (始终单副本,防止被意外驱逐) |
| `langfuse/` | langfuse Deployment | `minAvailable: 1` |
| `langfuse/` | langfuse-worker Deployment | `minAvailable: 1` |
| `langfuse/` | clickhouse Deployment | `minAvailable: 1` |
| `langfuse/` | redis Deployment | `minAvailable: 1` |
| `dify/` | dify-api Deployment | `minAvailable: 1` |
| `dify/` | dify-web Deployment | `minAvailable: 1` |
| `dify/` | dify-worker Deployment | `minAvailable: 1` |
| `dify/` | dify-plugin-daemon Deployment | `minAvailable: 1` |
| `dify/` | dify-redis Deployment | `minAvailable: 1` |
| `vllm/` | vllm Deployment (per model) | `minAvailable: 0` (单副本 GPU 工作负载) |
| `tei/` | tei Deployment (per model) | `minAvailable: 0` |
| `observability/` | prometheus Deployment | `minAvailable: 1` |
| `observability/` | grafana Deployment | `minAvailable: 1` |
| `keycloak/` | keycloak Deployment | `minAvailable: 1` |
| `logging/` | loki Deployment | `minAvailable: 1` |
| `security/` | llm-guard Deployment | `minAvailable: 0` |
| `security/` | presidio-analyzer Deployment | `minAvailable: 0` |
| `security/` | presidio-anonymizer Deployment | `minAvailable: 0` |
| `lightrag/` | neo4j Deployment | `minAvailable: 1` |
| `lightrag/` | lightrag Deployment | `minAvailable: 0` |
| `milvus/` | milvus Deployment | `minAvailable: 0` |
| `milvus/` | milvus-etcd StatefulSet | `minAvailable: 0` |
| `fluid/` | minio StatefulSet | `minAvailable: 1` |

**任务 2: 条件化 PDB 生成**

PDB 对单副本 Deployment 的 `minAvailable: 1` 会阻塞 `kubectl drain`。需要通过 values 条件控制:

```yaml
# 模板示例
{{- if .Values.pdb.enabled }}
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: {{ include "<chart>.fullname" . }}
  labels:
    {{- include "<chart>.labels" . | nindent 4 }}
spec:
  {{- if gt (int .Values.replicaCount) 1 }}
  minAvailable: 1
  {{- else }}
  maxUnavailable: 1
  {{- end }}
  selector:
    matchLabels:
      {{- include "<chart>.selectorLabels" . | nindent 6 }}
{{- end }}
```

在各子 Chart 的 `values.yaml` 中添加:
```yaml
pdb:
  enabled: true
```

**任务 3: 更新测试脚本**

在 QA 的 `improvement/test/scripts/04-edge-case-test.sh` 中增加 PDB 验证检查点:
```bash
# 验证关键组件有 PDB
kubectl get pdb -o name | grep -c "litellm\|postgresql\|grafana\|prometheus"
```

### 验收标准 (DoD)

- [x] 每个子 Chart 目录中包含 `pdb.yaml` 模板(或在主模板文件中内联)
- [x] `helm template . -f values-single-node.yaml` 渲染结果中包含 PDB 资源
- [x] `helm template . -f values-production.yaml` 渲染结果中多副本组件的 PDB 为 `minAvailable: 1`
- [x] 部署后 `kubectl get pdb` 列出所有组件的 PDB
- [x] `kubectl drain <node> --dry-run` 不会报告"无法驱逐所有副本"的错误(单副本使用 `maxUnavailable: 1`)

---

## Story 1.2: 全组件凭据随机化 + existingSecret 支持

### 标签
`priority/P0` `area/security` `kind/hardening` `effort/1.5d`

### 为什么要做 (Why)

当前全平台至少 **17 个密码/密钥** 在 `values-single-node.yaml` 中硬编码明文:

| 凭据 | 当前默认值 | 风险等级 |
|------|-----------|---------|
| LiteLLM Master Key | `sk-kube-llmops-dev` | 极高 --- 控制所有 API 访问 |
| PostgreSQL 密码 | `llmops-pg-dev-pw` | 极高 --- 控制全平台数据 |
| Langfuse DB 密码 | `langfuse-default-pw` | 高 |
| Dify DB 密码 | `dify-default-pw` | 高 |
| Grafana 管理员密码 | `admin123!` | 高 |
| Keycloak 管理员密码 | `admin123!` | 极高 --- 控制全平台 SSO |
| MinIO 凭据 | `minioadmin/minioadmin` | 高 |
| LLM-Guard Token | `llm-guard-kube-llmops` | 中 |
| Langfuse Public/Secret Key | `pk-lf-kube-llmops` / `sk-lf-kube-llmops` | 高 |
| Grafana OIDC Secret | `grafana-oidc-secret` | 中 |
| Langfuse OIDC Secret | `langfuse-oidc-secret` | 中 |

项目自称面向"金融/政府/军工"私有化部署场景。在这些场景中,任何安全评审都会因硬编码密码**一票否决**。更危险的是,Quick Start 引导用户直接 `helm install` 使用默认值,用户甚至不会意识到密码需要修改。

### 要做什么 (What)

**任务 1: Secret 模板改造 --- 使用 `lookup` + `randAlphaNum` 实现幂等随机密码**

核心逻辑:首次安装时生成随机密码;后续 `helm upgrade` 时复用已有 Secret,不重新生成。

```yaml
# charts/litellm/templates/secret.yaml
{{- $existingSecret := lookup "v1" "Secret" .Release.Namespace (printf "%s-litellm-secret" .Release.Name) }}
apiVersion: v1
kind: Secret
metadata:
  name: {{ .Release.Name }}-litellm-secret
type: Opaque
data:
  {{- if $existingSecret }}
  # Upgrade: 保留已有密码
  master-key: {{ index $existingSecret.data "master-key" }}
  {{- else if .Values.existingSecret }}
  # 用户自带 Secret: 不生成
  {{- else }}
  # 首次安装: 生成随机密码
  master-key: {{ .Values.masterKey | default (randAlphaNum 32) | b64enc }}
  {{- end }}
```

**任务 2: 所有子 Chart 添加 `existingSecret` 字段**

在每个子 Chart 的 `values.yaml` 中添加:
```yaml
# 使用已有的 Kubernetes Secret 而非自动生成
# 如果指定了 existingSecret,Chart 将不创建 Secret 资源,而是引用此 Secret
existingSecret: ""
# existingSecret 中需要包含的 key:
#   master-key: <LiteLLM master key>
#   postgresql-password: <PostgreSQL password>
```

涉及的子 Chart 及其 Secret 字段:

| 子 Chart | existingSecret 需包含的 key |
|----------|---------------------------|
| `litellm/` | `master-key`, `postgresql-password`, `langfuse-public-key`, `langfuse-secret-key` |
| `langfuse/` | `postgresql-password`, `clickhouse-password`, `encryption-key`, `salt` |
| `dify/` | `postgresql-password`, `secret-key` |
| `observability/` | `grafana-admin-password`, `grafana-oidc-secret` |
| `keycloak/` | `admin-password` |
| `fluid/` (MinIO) | `root-user`, `root-password` |
| `security/` | `llm-guard-token` |
| `lightrag/` | `neo4j-password` |

**任务 3: Deployment 模板改造 --- 从 Secret 读取而非内联 values**

所有引用密码的 Deployment 环境变量从:
```yaml
env:
  - name: LITELLM_MASTER_KEY
    value: {{ .Values.masterKey }}
```
改为:
```yaml
env:
  - name: LITELLM_MASTER_KEY
    valueFrom:
      secretKeyRef:
        name: {{ .Values.existingSecret | default (printf "%s-litellm-secret" .Release.Name) }}
        key: master-key
```

**任务 4: values-*.yaml 移除硬编码密码**

所有 `values-single-node.yaml`, `values-minimal.yaml`, `values-standard.yaml`, `values-production.yaml` 中的密码字段改为注释说明:
```yaml
# masterKey: ""  # 留空则自动生成随机密钥,或设置 existingSecret 引用已有 Secret
```

**任务 5: NOTES.txt 输出安全提醒**

在 `charts/kube-llmops-stack/templates/NOTES.txt` (当前仅 31 行) 中增加 Security Notice 段落:
```
=== SECURITY NOTICE ===
The following credentials were auto-generated for this installation.
Retrieve them with:
  kubectl get secret {{ .Release.Name }}-litellm-secret -o jsonpath='{.data.master-key}' | base64 -d
  kubectl get secret {{ .Release.Name }}-grafana-secret -o jsonpath='{.data.admin-password}' | base64 -d
  ...
For production use, provide your own secrets via existingSecret values.
See: https://github.com/<repo>/docs/security-guide.md
```

### 验收标准 (DoD)

- [x] `helm install` 全新部署时,`kubectl get secret` 中所有密码为随机值(非固定默认值)
- [x] `helm upgrade` 时,已有 Secret 的值不变(通过 `lookup` 保持幂等)
- [x] 连续两次 `helm template` 输出的密码值不同(证明随机生成生效)
- [x] 每个子 Chart 的 `values.yaml` 包含 `existingSecret` 字段及注释说明
- [x] 指定 `existingSecret` 后,Chart 不创建 Secret 资源,Deployment 引用用户提供的 Secret
- [x] NOTES.txt 输出包含 "Security Notice" 段落和凭据获取命令
- [x] `helm template` 渲染结果中无任何明文密码(均通过 `secretKeyRef` 引用)
- [x] 所有 values-*.yaml 样例文件中密码字段为空或已注释

---

## Story 1.3: Prometheus 硬编码 scrape target 替换为 Kubernetes 服务发现

### 标签
`priority/P0` `area/observability` `kind/bug` `effort/1d`

### 为什么要做 (Why)

`charts/observability/templates/prometheus.yaml` 第 32 行硬编码了 vLLM 的 scrape target:

```yaml
- job_name: vllm
  static_configs:
    - targets: ["vllm-qwen2-5-0-5b:8000"]   # 硬编码模型名!
```

这导致三个严重问题:

1. **无法支持多模型**: 用户在 `values.yaml` 中配置第二个模型 (如 `deepseek-r1-70b`) 后,Prometheus 不会自动采集新模型的指标,必须手动编辑 ConfigMap --- 这完全违背了"一键部署"的核心价值主张。

2. **多 Release 名称冲突**: 如果在同一集群部署 staging 和 production 两个 Release,两者的 Prometheus 都会尝试抓取同一个 `vllm-qwen2-5-0-5b:8000`,造成指标混乱。

3. **与其他 scrape config 不一致**: 同一文件中 otel-collector (第 25 行) 和 pushgateway (第 29 行) 已经使用 `{{ $.Release.Name }}` 模板化,唯独 vLLM 是硬编码,说明这是一个遗漏而非设计决策。

### 要做什么 (What)

**任务 1: vLLM scrape config 改为 Kubernetes 服务发现**

将 `charts/observability/templates/prometheus.yaml` 中 vLLM 的 scrape 配置从 static_configs 改为 kubernetes_sd_configs:

```yaml
- job_name: vllm
  kubernetes_sd_configs:
    - role: pod
      namespaces:
        names: ["{{ $.Release.Namespace }}"]
  relabel_configs:
    # 只抓取属于当前 Release 的 vLLM Pod
    - source_labels: [__meta_kubernetes_pod_label_app_kubernetes_io_part_of]
      regex: "{{ include "kube-llmops.fullname" $ }}"
      action: keep
    - source_labels: [__meta_kubernetes_pod_label_app_kubernetes_io_name]
      regex: vllm
      action: keep
    # 使用 Pod IP + 端口
    - source_labels: [__meta_kubernetes_pod_ip]
      target_label: __address__
      replacement: "$1:8000"
    # 将模型名作为 label 保留,便于 Dashboard 筛选
    - source_labels: [__meta_kubernetes_pod_label_app_kubernetes_io_component]
      target_label: model
```

**任务 2: TEI scrape config 同步改造**

TEI 的 embedding 和 reranker 服务也应通过 SD 发现:

```yaml
- job_name: tei
  kubernetes_sd_configs:
    - role: pod
      namespaces:
        names: ["{{ $.Release.Namespace }}"]
  relabel_configs:
    - source_labels: [__meta_kubernetes_pod_label_app_kubernetes_io_part_of]
      regex: "{{ include "kube-llmops.fullname" $ }}"
      action: keep
    - source_labels: [__meta_kubernetes_pod_label_app_kubernetes_io_name]
      regex: tei
      action: keep
    - source_labels: [__meta_kubernetes_pod_ip]
      target_label: __address__
      replacement: "$1:8080"
    - source_labels: [__meta_kubernetes_pod_label_app_kubernetes_io_component]
      target_label: model
```

**任务 3: 确保 vLLM/TEI Pod 携带正确标签**

检查 `charts/vllm/templates/_helpers.tpl` 和 `charts/tei/templates/` 中的标签定义,确保 Pod 携带:
- `app.kubernetes.io/name: vllm` (或 `tei`)
- `app.kubernetes.io/component: <model-name>`
- `app.kubernetes.io/part-of: {{ .Release.Name }}`

当前 vLLM 的 `_helpers.tpl` 已有正确的标签定义,需确认 Prometheus SD 的 relabel 规则与之匹配。

**任务 4: 验证 Grafana Dashboard 兼容性**

Prometheus 指标名和 label 变更后,确认 `charts/observability/dashboards/` 中 4 个 Dashboard 的 PromQL 查询仍然有效。特别注意:
- `vllm:num_requests_running` 指标是否需要按新 label `model` 过滤
- 告警规则 (prometheus.yaml 第 36-127 行) 中引用的指标名是否需要调整

**任务 5: Prometheus RBAC 配置**

Kubernetes SD 需要 Prometheus 有权限读取 Pod 信息。检查是否已有 ClusterRole/ClusterRoleBinding 授权 Prometheus ServiceAccount 读取 Pod 资源。如果没有,需要添加:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: {{ .Release.Name }}-prometheus
rules:
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list", "watch"]
```

### 验收标准 (DoD)

- [x] `helm template` 渲染的 Prometheus ConfigMap 中**无任何硬编码服务名**
- [x] 部署后 Prometheus UI → Status → Targets 页面显示通过 kubernetes-sd 发现的 vLLM/TEI 端点
- [x] 在 `values.yaml` 中添加第二个 vLLM 模型后,`helm upgrade` 无需其他操作,Prometheus 自动发现新 target
- [x] `vllm:num_requests_running` 等核心指标在 Grafana vLLM Dashboard 中正常展示
- [x] 11 条 Prometheus 告警规则 (vllm-alerts + rag-quality-alerts) 仍然正常触发
- [x] Prometheus Pod 日志中无 RBAC 相关权限错误

---

## Story 1.4: 文档一致性修复 --- 消除"承诺 vs 实现"差距

### 标签
`priority/P0` `area/docs` `kind/documentation` `effort/1d`

### 为什么要做 (Why)

根据 `improvement/PROMISED-VS-IMPLEMENTED.md` 的审计结果:文档声明了约 **150 项**能力,实际可用约 **75 项**,约 **45 项仅存在于文档愿景中**且未标注状态。具体表现:

1. **ARCHITECTURE.md** (1085 行) 混淆了"当前实现"与"未来愿景" --- SGLang、Harbor、JupyterHub、MLflow、LLaMA-Factory、ArgoCD ApplicationSet、External Secrets Operator、Cilium mTLS 等均被描述为架构组成部分,但代码中为零实现或仅有空壳。

2. **README.md Features 对比表** (第 143-157 行) 中:
   - "Engine auto-selection" 标为 `Yes`,实际 Model Resolver 未集成到 Helm init-container
   - "KEDA autoscaling" 标为 `Yes`,实际需要预装外部 KEDA Operator 且默认 disabled

3. **PLAN.md** (1009 行) 中 v0.1.0/v0.2.0 的 checklist 全部标为 `[x]` (完成),但多项功能仅有模板或空壳 (Harbor, Fluid, KEDA)。

4. **根目录有 5 个 RAG-*.md 文件** (RAG-ASSESSMENT, RAG-DIRECTION, RAG-PLAN, RAG-TEST-PLAN, RAG-TODO) 分散在根目录,部分内容自相矛盾 (RAG-PLAN 标 Done 但 RAG-ASSESSMENT 说未实现),新用户进来会被淹没。

在开源社区中,文档与实现不一致是**信任的毒药** --- 贡献者或用户发现"Done"标注的功能实际不存在,会对项目永久失去信心。这是零成本高回报的修复。

### 要做什么 (What)

**任务 1: ARCHITECTURE.md 添加状态标签**

为 ARCHITECTURE.md 中每个组件/功能描述添加状态标签:

| 标签 | 含义 | 示例 |
|------|------|------|
| `[IMPLEMENTED]` | 代码完整、有测试覆盖 | vLLM, LiteLLM, Langfuse |
| `[BETA]` | 代码存在但未经充分生产验证 | Milvus, LightRAG, Presidio |
| `[TEMPLATE-ONLY]` | 有 Helm 模板但需要外部依赖或默认 disabled | KEDA, Envoy Gateway, Fluid |
| `[PLANNED vX.X]` | 仅有文档描述,零代码 | SGLang, JupyterHub, MLflow, ArgoCD |

需要标注的核心条目 (基于 PROMISED-VS-IMPLEMENTED.md 差距清单):
- SGLang 推理引擎 → `[PLANNED v0.5]`
- 双层网关 Tier-2 (Envoy) → `[TEMPLATE-ONLY]`
- KV-cache-aware 路由 → `[PLANNED v0.5]`
- ArgoCD Sync Waves → `[PLANNED v0.4]`
- JupyterHub / MLflow / LLaMA-Factory / Label Studio → `[PLANNED v0.4]`
- External Secrets Operator → `[PLANNED v0.3]`
- Cilium mTLS → `[PLANNED v0.4]`
- llm-d 解耦式推理 → `[PLANNED v0.5]`
- GPU Time-Slicing / MIG → `[PLANNED v0.5]`

**任务 2: README.md Features 对比表修正**

| 原标注 | 修正为 | 理由 |
|--------|--------|------|
| Engine auto-selection: `Yes` | `Partial` | Model Resolver 代码存在 (28 单测) 但未集成到 Helm init-container |
| KEDA autoscaling: `Yes` | `Partial` | 需要预装 KEDA Operator,默认 disabled,未经负载验证 |
| GPU monitoring (DCGM): `Yes` | `Yes*` | 添加注脚: WSL2 不可用,需宿主机 NVIDIA 驱动 |

**任务 3: PLAN.md checklist 修正**

将以下实际未完成的 `[x]` 改回 `[ ]` 或标注 `[PARTIAL]`:
- `[x] Model serving (auto-selected by Model Resolver)` → `[x] Model serving (manual engine selection; auto-detection planned)`
- `[x] Model cache (Fluid + MinIO)` → `[PARTIAL] Model cache (MinIO storage ready; Fluid requires external operator)`
- `[x] Model registry (Harbor)` → `[PARTIAL] Model registry (Harbor chart placeholder; not production-ready)`

**任务 4: RAG 文档收敛**

将根目录 5 个 RAG-*.md 文件合并迁移:
```
RAG-PLAN.md        → docs/rag/rag-plan.md      (保留,标注各项实际状态)
RAG-ASSESSMENT.md  → docs/rag/rag-assessment.md (保留,Phase 4 后更新版本)
RAG-DIRECTION.md   → 合并到 rag-assessment.md  (内容重叠)
RAG-TODO.md        → 合并到 rag-plan.md        (TODO 归入 Plan)
RAG-TEST-PLAN.md   → docs/rag/rag-test-plan.md (保留)
```

根目录仅保留: README.md, ARCHITECTURE.md, CHANGELOG.md, CONTRIBUTING.md, AGENTS.md, PLAN.md 及其中文版。

**任务 5: 同步更新中文版文档**

ARCHITECTURE.zh-CN.md, README.zh-CN.md 同步修正状态标签。

### 验收标准 (DoD)

- [x] ARCHITECTURE.md 中每个功能/组件描述旁有 `[IMPLEMENTED]`/`[BETA]`/`[TEMPLATE-ONLY]`/`[PLANNED]` 标签
- [x] README.md Features 对比表中 Engine auto-selection 和 KEDA 标为 `Partial`
- [x] PLAN.md 中未完成项不再标为 `[x]`
- [x] 根目录 Markdown 文件从 18 个收敛 (RAG-*.md 迁移到 `docs/rag/`)
- [x] `docs/` 目录包含 `docs/rag/` 子目录
- [x] 中文版文档同步更新

---

## Story 1.5: NOTES.txt 部署后引导增强

### 标签
`priority/P0` `area/ux` `kind/enhancement` `effort/0.5d`

### 为什么要做 (Why)

当前 `charts/kube-llmops-stack/templates/NOTES.txt` 仅有 31 行,只提供了 LiteLLM/Grafana/Langfuse 三个服务的 port-forward 命令。用户执行 `helm install` 后面临以下困境:

1. **不知道部署是否成功**: 没有 `kubectl get pods` 等状态检查指引,用户不知道要等多久、哪些 Pod 应该 Running
2. **不知道如何访问 UI**: 有 6 个 Ingress 资源 (litellm/grafana/langfuse/keycloak/dify-api/dify-web),NOTES.txt 只提到 3 个
3. **不知道凭据在哪里**: 与 Story 1.2 配合,随机生成的密码需要在 NOTES.txt 中告知用户如何获取
4. **不知道下一步做什么**: 缺少 Quick Verification 步骤,用户不知道如何验证部署是否正确

这是新用户上手的**第一个触点** --- 一个信息完整的 NOTES.txt 可以显著降低上手摩擦。

### 要做什么 (What)

**任务 1: 重写 NOTES.txt**

```
================================================================
  kube-llmops has been installed!
================================================================

1. CHECK DEPLOYMENT STATUS:
   kubectl get pods -l app.kubernetes.io/part-of={{ .Release.Name }} --watch

   Wait until all pods show "Running" or "Completed" status.
   vLLM model loading may take 3-10 minutes depending on model size.

2. ACCESS THE UIs:
{{- if .Values.ingress.host }}
   Add to /etc/hosts: <NODE_IP> litellm.{{ .Values.ingress.host }} grafana.{{ .Values.ingress.host }} ...

   LiteLLM Gateway:  https://litellm.{{ .Values.ingress.host }}
   Grafana:           https://grafana.{{ .Values.ingress.host }}
   Langfuse:          https://langfuse.{{ .Values.ingress.host }}
   Dify RAG:          https://dify.{{ .Values.ingress.host }}
   Keycloak SSO:      https://keycloak.{{ .Values.ingress.host }}
{{- else }}
   kubectl port-forward svc/{{ .Release.Name }}-litellm 4000:4000 &
   kubectl port-forward svc/{{ .Release.Name }}-grafana 3000:3000 &
   ...
{{- end }}

3. SECURITY NOTICE:
   Credentials were auto-generated. Retrieve them:
   kubectl get secret {{ .Release.Name }}-litellm-secret -o jsonpath='{.data.master-key}' | base64 -d
   kubectl get secret {{ .Release.Name }}-grafana-secret -o jsonpath='{.data.admin-password}' | base64 -d
   ...

4. QUICK VERIFICATION:
   # Test LLM inference
   curl -s http://localhost:4000/v1/chat/completions \
     -H "Authorization: Bearer $(kubectl get secret ...)" \
     -H "Content-Type: application/json" \
     -d '{"model":"{{ first model name }}", "messages":[{"role":"user","content":"Hello"}]}'

   # Test embedding
   curl -s http://localhost:4000/v1/embeddings ...

5. NEXT STEPS:
   - View GPU & LLM metrics: Grafana → vLLM Performance Dashboard
   - Check LLM traces: Langfuse → Traces
   - Upload documents for RAG: Dify → Knowledge Base
   - Read the full guide: https://github.com/<repo>/docs/getting-started.md
```

**任务 2: 条件化输出**

根据启用的组件动态显示内容:
- `{{- if .Values.vllm.enabled }}` 才显示 vLLM 相关内容
- `{{- if .Values.dify.enabled }}` 才显示 Dify 相关内容
- `{{- if .Values.keycloak.enabled }}` 才显示 SSO 相关内容

### 验收标准 (DoD)

- [x] `helm install` 完成后终端输出包含: 状态检查命令、所有 UI 访问地址、凭据获取方式、Quick Verification 步骤
- [x] `helm status kube-llmops` 可重新查看引导信息
- [x] ci profile (无 GPU) 的输出不包含 vLLM/GPU 相关信息
- [x] production profile 的输出包含所有组件信息
- [x] 有 Ingress host 时显示域名访问方式;无 host 时显示 port-forward 方式

---

## Story 1.6: CI chart-install-test 强制通过

### 标签
`priority/P0` `area/ci` `kind/bug` `effort/0.5d`

### 为什么要做 (Why)

`.github/workflows/test.yaml` 第 58 行:

```yaml
chart-install-test:
  continue-on-error: true    # ← 安装失败 CI 仍然绿灯!
```

当前注释说明原因是 "kind cluster on GitHub Actions is resource-constrained, image pulls for LiteLLM (~1.5GB) often exceed timeout"。

这意味着**即使 Helm Chart 安装完全失败,CI 也会显示绿色**。这是一个危险的假安全感 --- 开发者合并 PR 时以为通过了安装测试,实际上可能引入了破坏性变更。随着项目复杂度增长,这个漏洞的风险只会越来越大。

### 要做什么 (What)

**任务 1: 移除 `continue-on-error: true`**

```yaml
chart-install-test:
  runs-on: ubuntu-latest
  needs: [helm-template, python-tests]
  # continue-on-error: true  # REMOVED: install test must pass
```

**任务 2: 优化 CI profile 降低资源需求**

为了避免因 GitHub Actions 资源不足导致 CI 频繁失败,优化 `values-ci.yaml`:
- 确保 CI profile 只启用最小组件集 (LiteLLM + PostgreSQL)
- 使用轻量级镜像 tag
- 设置合理的 `--timeout` (如 10m)
- 添加镜像预拉取步骤 (`docker pull` 在 helm install 之前)

**任务 3: 添加超时和重试机制**

```yaml
steps:
  - name: Pre-pull critical images
    run: |
      docker pull ghcr.io/berriai/litellm:main-v1.82.3-stable
      docker pull pgvector/pgvector:pg17
      kind load docker-image ghcr.io/berriai/litellm:main-v1.82.3-stable
      kind load docker-image pgvector/pgvector:pg17

  - name: Install chart
    run: |
      helm install kube-llmops . -f values-ci.yaml --wait --timeout 10m
    timeout-minutes: 12
```

### 验收标准 (DoD)

- [x] `.github/workflows/test.yaml` 中 `chart-install-test` 无 `continue-on-error: true`
- [x] CI 在 chart 安装失败时 workflow 状态为红色
- [x] CI 在正常情况下 chart 安装测试能稳定通过 (连续 3 次 CI 运行)
- [x] CI profile 使用的镜像在测试步骤前已预拉取到 kind 集群

---

## Story 1.7: Milvus etcd 资源配置

### 标签
`priority/P0` `area/reliability` `kind/bug` `effort/0.5d`

### 为什么要做 (Why)

QA 测试报告 (`improvement/test/04-TEST-REPORT.md`) 发现:

> **Milvus etcd QoS=BestEffort** --- 无资源 requests/limits,在资源紧张时容易被驱逐

Kubernetes 的 QoS 等级直接决定 Pod 被驱逐的优先级:
- **Guaranteed**: requests == limits,最后被驱逐
- **Burstable**: 有 requests 但 < limits,中间优先级
- **BestEffort**: 无 requests/limits,**最先被驱逐**

etcd 是 Milvus 的**元数据存储**,一旦 etcd 被驱逐,Milvus 将无法正常工作 (无法读取 collection 信息、segment 分配等)。在资源紧张的单节点环境中 (LLM-Guard 需要 ~6GB RAM),BestEffort 的 etcd 极容易成为第一个被杀的 Pod。

### 要做什么 (What)

**任务 1: 为 Milvus etcd StatefulSet 添加资源配置**

在 `charts/milvus/templates/` 中的 etcd StatefulSet,添加:

```yaml
resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 512Mi
```

**任务 2: 在 milvus/values.yaml 中暴露资源配置**

```yaml
etcd:
  resources:
    requests:
      cpu: 100m
      memory: 128Mi
    limits:
      cpu: 500m
      memory: 512Mi
```

### 验收标准 (DoD)

- [x] Milvus etcd Pod 的 QoS Class 从 `BestEffort` 变为 `Burstable`
- [x] `kubectl get pod <etcd-pod> -o jsonpath='{.status.qosClass}'` 返回 `Burstable`
- [x] `helm template` 渲染的 etcd StatefulSet 包含 `resources.requests`
- [x] QA 测试脚本 02-k8s-resource-test.py 中 etcd QoS 检查通过

---

## Story 1.8: LightRAG 健康检查探针

### 标签
`priority/P0` `area/reliability` `kind/bug` `effort/0.5d`

### 为什么要做 (Why)

QA 测试报告发现:

> **LightRAG 无健康检查端点** --- 模板未配置 readinessProbe/livenessProbe

当前 `charts/lightrag/templates/lightrag.yaml` 中 LightRAG API Server (port 9621) 的 Deployment 完全没有健康探针。这意味着:

1. **Kubernetes 无法判断 LightRAG 是否就绪**: Pod 一启动就会被标记为 Ready,即使 LightRAG 还在初始化 Neo4j 连接。Service 会将流量发送到未就绪的 Pod,导致请求失败。

2. **LightRAG 挂死时 K8s 不会重启它**: 如果 LightRAG 进程 hang 住 (如 Neo4j 连接池耗尽),没有 liveness probe 触发自动重启,服务将持续不可用直到人工干预。

作为对比:同一文件中的 Neo4j 已经配置了 readinessProbe (`httpGet / on 7474`),唯独 LightRAG 遗漏了。

### 要做什么 (What)

**任务 1: 确认 LightRAG 健康检查端点**

LightRAG API Server 监听 9621 端口。需确认:
- LightRAG 是否有内置的 `/health` 端点
- 如果没有,使用 TCP Socket 探针作为替代

**任务 2: 添加 readinessProbe 和 livenessProbe**

在 `charts/lightrag/templates/lightrag.yaml` 的 LightRAG Deployment 中添加:

```yaml
readinessProbe:
  httpGet:
    path: /health    # 或 tcpSocket: { port: 9621 }
    port: 9621
  initialDelaySeconds: 15
  periodSeconds: 10
  failureThreshold: 10
livenessProbe:
  httpGet:
    path: /health
    port: 9621
  initialDelaySeconds: 30
  periodSeconds: 30
  failureThreshold: 3
```

readinessProbe 允许较长的初始化时间 (15s + 10 次失败 x 10s = 115s),因为 LightRAG 需要建立 Neo4j 连接。

**任务 3: 同时为 Neo4j 补充 livenessProbe**

当前 Neo4j 只有 readinessProbe 没有 livenessProbe。补充:

```yaml
livenessProbe:
  httpGet:
    path: /
    port: 7474
  initialDelaySeconds: 60
  periodSeconds: 30
  failureThreshold: 5
```

### 验收标准 (DoD)

- [x] `helm template` 渲染的 LightRAG Deployment 包含 readinessProbe 和 livenessProbe
- [x] 部署后 `kubectl describe pod <lightrag-pod>` 显示正确的探针配置
- [x] LightRAG Pod 在初始化完成前不会被标记为 Ready (观察 Pod 状态从 0/1 → 1/1 的过程)
- [x] Neo4j Deployment 同时包含 readinessProbe 和 livenessProbe
- [x] QA 部署验证脚本中 LightRAG 健康端点检查通过
