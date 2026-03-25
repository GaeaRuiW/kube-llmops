# Phase 2: 稳定性与提效 --- 详细 Story 拆解

> **阶段目标**: 解耦核心架构瓶颈、建立自动化测试体系、补齐运维文档
> **时间窗口**: 1 个月 (P1 级)
> **总工作量**: ~19 人天
> **原则**: 允许架构级重构,但每个变更必须有回归测试覆盖
> **前置条件**: Phase 1 全部 8 个 Story 已完成并验证通过

---

## Story 2.1: PostgreSQL 拆分 --- 消除全平台单点故障

### 标签
`priority/P1` `area/architecture` `kind/refactoring` `effort/5d`

### 为什么要做 (Why)

当前整个平台共享**一个 PostgreSQL 实例** (由 `litellm` 子 Chart 部署, `charts/litellm/templates/postgresql.yaml`),通过 initdb 脚本创建 4 个数据库:

```
litellm-pg:5432
  ├── litellm      (API 密钥校验、开支追踪、限流 — 推理热路径)
  ├── langfuse     (追踪元数据 — OLAP 查询)
  ├── dify         (RAG 工作流、知识库 — 应用数据)
  └── dify_plugin  (插件守护进程状态)
```

架构师评审 (FATAL-01) 指出四个致命风险:

1. **爆炸半径**: Dify 的一条 `ALTER TABLE` 锁可以级联到 LiteLLM 的 API 密钥校验,导致**所有推理请求超时**。某个数据库连接池耗尽会饿死所有其他数据库。

2. **升级耦合**: Dify 1.x → 2.x 可能需要破坏性 Schema 变更。共享实例下无法在升级 Dify 的同时不冒 LiteLLM 宕机风险。

3. **备份粒度**: 无法独立备份/恢复 Dify 数据而不影响 LiteLLM 运行状态。

4. **资源竞争**: LiteLLM 的高频 API 密钥查找 (热路径, 低延迟要求) 与 Langfuse 的 OLAP 元数据查询在同一实例上竞争。

Phase 1 通过 `existingSecret` 提供了逃生通道 (用户可指向外部 PG),但内置架构的根本问题仍需解决。

### 要做什么 (What)

**任务 1: 创建独立的 PostgreSQL 基础设施子 Chart**

新建 `charts/kube-llmops-stack/charts/postgresql/` 作为统一的数据库基础设施层,替代当前嵌入在 `litellm/` 中的 PG 部署。采用架构师推荐的方案 B (最低要求):

```
operator-pg:5432   → litellm 数据库 (热路径, 低延迟要求)
app-pg:5432        → langfuse, dify, dify_plugin 数据库 (可容忍较高延迟)
```

两个独立的 Deployment/StatefulSet + PVC + Service,互不影响。

**任务 2: 重构 litellm 子 Chart --- 移除 PostgreSQL 部署**

- 从 `charts/litellm/templates/postgresql.yaml` 中移除 Deployment/Service/PVC/ConfigMap 定义
- 修改 `charts/litellm/templates/deployment.yaml` 中的 `DATABASE_URL` 环境变量,指向新的 `operator-pg` Service
- `charts/litellm/values.yaml` 中添加:
  ```yaml
  postgresql:
    # 使用内置 operator-pg
    host: ""  # 留空则使用 {{ .Release.Name }}-operator-pg
    # 或指向外部数据库
    externalHost: ""
    externalPort: 5432
  ```

**任务 3: 重构 langfuse 子 Chart --- 指向 app-pg**

当前 `charts/langfuse/templates/deployment.yaml` 第 45-50 行:
```yaml
DATABASE_URL:
  {{- if .Values.postgresql.enabled }}
  value: "postgresql://...@{{ $.Release.Name }}-langfuse-pg:5432/..."
  {{- else }}
  value: "postgresql://...@{{ $.Release.Name }}-litellm-pg:5432/..."
  {{- end }}
```

改为指向 `app-pg`:
```yaml
value: "postgresql://...@{{ .Values.postgresql.host | default (printf "%s-app-pg" $.Release.Name) }}:5432/..."
```

**任务 4: 重构 dify 子 Chart --- 指向 app-pg**

`charts/dify/templates/dify.yaml` 中 Dify API/Worker/Plugin Daemon 的 PostgreSQL 连接字符串同步修改。

**任务 5: 数据迁移文档**

创建 `docs/guides/postgresql-migration.md`:
- 从单实例迁移到双实例的步骤
- `pg_dump` / `pg_restore` 命令
- 迁移期间的服务中断预期
- 回滚流程
- 已有 `existingSecret` 用户无需迁移的说明

**任务 6: 回归测试**

- 全部 5 个 Playwright E2E 测试通过
- QA 部署验证脚本 (01-deploy-verify.sh) 通过
- Smoke Test Job 通过
- 手动验证: Dify 执行大查询时 LiteLLM API 响应无异常延迟

### 验收标准 (DoD)

- [ ] `helm template` 渲染出两个独立的 PostgreSQL Deployment: `operator-pg` 和 `app-pg`
- [ ] 两个 PG 实例有各自独立的 PVC (数据隔离)
- [ ] LiteLLM 连接 `operator-pg`,Langfuse/Dify 连接 `app-pg`
- [ ] `operator-pg` 被删除时,Langfuse/Dify 不受影响 (反之亦然)
- [ ] `existingSecret` / `externalHost` 仍然可用 (Phase 1 的逃生通道不被破坏)
- [ ] 全部 E2E 测试 + Smoke Test 通过
- [ ] 迁移文档就绪

---

## Story 2.2: values.schema.json 配置校验

### 标签
`priority/P1` `area/ux` `kind/enhancement` `effort/2d`

### 为什么要做 (Why)

当前 `charts/kube-llmops-stack/` 中不存在 `values.schema.json` 文件。这意味着用户在 `values.yaml` 中填错配置 (拼写错误、类型错误、缺少必填字段) 只有在 `helm install` 实际部署时才会报错,而且报错信息往往是 Go template 渲染错误,难以定位根因。

对于一个有 16 个子 Chart、数百个配置项的伞状 Chart,配置出错的概率极高。`values.schema.json` 可以:
1. 在 `helm install --dry-run` 阶段就捕获配置错误
2. 为 IDE (VSCode + Helm 插件) 提供自动补全和校验
3. 作为配置文档的单一事实来源

### 要做什么 (What)

**任务 1: 为顶层 Chart 创建 schema 框架**

创建 `charts/kube-llmops-stack/values.schema.json`,覆盖以下关键配置:

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "required": ["ingress"],
  "properties": {
    "ingress": {
      "type": "object",
      "properties": {
        "host": { "type": "string", "description": "Base domain for Ingress routes" },
        "className": { "type": "string", "default": "traefik" }
      }
    },
    "vllm": {
      "type": "object",
      "properties": {
        "enabled": { "type": "boolean", "default": true },
        "models": {
          "type": "array",
          "items": {
            "type": "object",
            "required": ["name", "modelId"],
            "properties": {
              "name": { "type": "string" },
              "modelId": { "type": "string" },
              "engine": { "type": "string", "enum": ["vllm", "tei", "llamacpp"] },
              "gpu": { "type": "integer", "minimum": 0 }
            }
          }
        }
      }
    }
    // ... 其他子 Chart 的 schema
  }
}
```

**任务 2: 为关键子 Chart 定义 schema**

重点覆盖用户最常修改的配置:
- `litellm.masterKey` (string, required)
- `litellm.postgresql.auth.*` (password fields)
- `vllm.models[]` (array of model configs)
- `observability.grafana.adminPassword` (string)
- `keycloak.adminPassword` (string)
- 所有 `*.enabled` 开关 (boolean)

**任务 3: CI 集成 schema 校验**

在 `.github/workflows/test.yaml` 的 `helm-template` job 中添加:
```yaml
- name: Validate values against schema
  run: |
    helm lint charts/kube-llmops-stack -f charts/kube-llmops-stack/values-single-node.yaml --strict
```

### 验收标准 (DoD)

- [ ] `charts/kube-llmops-stack/values.schema.json` 存在且格式正确
- [ ] `helm install --dry-run` 对缺失必填字段报错 (如删除 `ingress.host` 后)
- [ ] `helm install --dry-run` 对类型错误报错 (如 `vllm.enabled: "yes"` 非 boolean)
- [ ] CI 中所有 6 个 profile 通过 schema 校验
- [ ] schema 覆盖至少 50 个最常用的配置字段

---

## Story 2.3: AlertManager 子 Chart + 通知渠道

### 标签
`priority/P1` `area/observability` `kind/feature` `effort/1.5d`

### 为什么要做 (Why)

当前 Prometheus 配置中定义了 **11 条告警规则** (5 条 vllm-alerts + 6 条 rag-quality-alerts,位于 `charts/observability/templates/prometheus.yaml` 第 36-127 行),但项目中**没有 AlertManager 部署** (`charts/observability/templates/` 中无 alertmanager 相关模板)。

这意味着:告警规则触发后,**没有人会收到通知**。Prometheus 会在 UI 上显示 "FIRING" 状态,但没有 Slack/邮件/Webhook 通知。这等于没有告警。

对于一个自称"生产级"的平台,告警通知是 Day-1 必备能力。用户需要在以下场景及时收到通知:
- PostgreSQL 宕机 (全平台瘫痪)
- vLLM 服务下线 (推理不可用)
- GPU 利用率过高 / KV Cache 快满 (性能劣化预警)
- RAG 质量下降 (Ragas 分数低于阈值)

### 要做什么 (What)

**任务 1: 在 observability 子 Chart 中添加 AlertManager 部署**

在 `charts/observability/templates/` 中新增 `alertmanager.yaml`:

```yaml
# AlertManager Deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Release.Name }}-alertmanager
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: alertmanager
          image: prom/alertmanager:v0.27.0
          ports:
            - containerPort: 9093
          volumeMounts:
            - name: config
              mountPath: /etc/alertmanager
      volumes:
        - name: config
          configMap:
            name: {{ .Release.Name }}-alertmanager-config
---
# AlertManager ConfigMap (通知渠道配置)
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-alertmanager-config
data:
  alertmanager.yml: |
    global:
      resolve_timeout: 5m
    route:
      group_by: ['alertname', 'severity']
      group_wait: 30s
      group_interval: 5m
      repeat_interval: 4h
      receiver: 'default'
      routes:
        - match:
            severity: critical
          receiver: 'critical'
    receivers:
      - name: 'default'
        {{- if .Values.alertmanager.slack.enabled }}
        slack_configs:
          - api_url: {{ .Values.alertmanager.slack.webhookUrl }}
            channel: {{ .Values.alertmanager.slack.channel | default "#alerts" }}
        {{- end }}
        {{- if .Values.alertmanager.webhook.enabled }}
        webhook_configs:
          - url: {{ .Values.alertmanager.webhook.url }}
        {{- end }}
      - name: 'critical'
        {{- /* same as default but with shorter repeat_interval */ -}}
```

**任务 2: Prometheus 配置中添加 AlertManager 端点**

在 `prometheus.yaml` 的 Prometheus ConfigMap 中添加:
```yaml
alerting:
  alertmanagers:
    - static_configs:
        - targets: ["{{ .Release.Name }}-alertmanager:9093"]
```

**任务 3: values.yaml 中暴露通知渠道配置**

```yaml
alertmanager:
  enabled: true
  slack:
    enabled: false
    webhookUrl: ""
    channel: "#kube-llmops-alerts"
  webhook:
    enabled: false
    url: ""
  email:
    enabled: false
    to: ""
    from: ""
    smarthost: ""
```

**任务 4: 添加 Grafana AlertManager 数据源**

在 Grafana provisioning 中自动添加 AlertManager 作为数据源,用户可以在 Grafana UI 中查看告警状态。

### 验收标准 (DoD)

- [ ] `helm template` 渲染出 AlertManager Deployment + Service + ConfigMap
- [ ] Prometheus UI → Status → Alertmanagers 显示已连接的 AlertManager 实例
- [ ] 配置 Slack webhook 后,手动触发告警 (如停止 vLLM Pod) 可在 Slack 频道收到通知
- [ ] 配置 Webhook URL 后,告警触发时 Webhook 收到 POST 请求
- [ ] Grafana 中可查看 AlertManager 数据源和告警状态
- [ ] 至少 `VllmDown`, `PostgreSQL Down`, `RAGFaithfulnessLow` 三个告警场景测试通过

---

## Story 2.4: Helm 标签标准统一

### 标签
`priority/P1` `area/architecture` `kind/refactoring` `effort/1.5d`

### 为什么要做 (Why)

架构师评审 (HIGH-07) 发现标签使用不一致:
- vLLM 使用 `app.kubernetes.io/name: vllm`, `app.kubernetes.io/component: {{ $model.name }}`
- LiteLLM 使用 `app.kubernetes.io/name: litellm`
- Prometheus scrape 匹配 `kube_llmops_engine: vllm` (自定义标签)
- NetworkPolicy 匹配 `app.kubernetes.io/name: litellm` 和 `app.kubernetes.io/name: otel-collector`

且整个项目只有 3 个 `_helpers.tpl` 文件 (主 Chart + vLLM + Milvus),其他 13 个子 Chart 使用内联标签。

这导致:
1. Prometheus 服务发现 (Story 1.3) 依赖的标签选择器可能匹配不到某些组件
2. NetworkPolicy 选择器遗漏组件 (当前只覆盖 4/16 个服务)
3. `kubectl get pods -l app.kubernetes.io/part-of=kube-llmops` 无法列出所有组件

### 要做什么 (What)

**任务 1: 定义统一标签标准**

所有子 Chart 的资源必须携带以下标签:

```yaml
# 标准标签 (所有资源)
app.kubernetes.io/name: {{ .Chart.Name }}           # e.g., litellm, vllm, langfuse
app.kubernetes.io/instance: {{ .Release.Name }}      # e.g., kube-llmops
app.kubernetes.io/version: {{ .Chart.AppVersion }}   # e.g., 1.82.3
app.kubernetes.io/component: <组件特定>              # e.g., api, worker, postgresql
app.kubernetes.io/part-of: kube-llmops               # 固定值,用于全局选择
app.kubernetes.io/managed-by: {{ .Release.Service }} # Helm
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version }}
```

**任务 2: 为每个子 Chart 创建 `_helpers.tpl`**

当前只有 vllm 和 milvus 有 `_helpers.tpl`。为其余 13 个子 Chart 创建标准化的 helper 模板:

```yaml
# charts/<subchart>/templates/_helpers.tpl
{{- define "<subchart>.name" -}}
{{ .Chart.Name }}
{{- end }}

{{- define "<subchart>.fullname" -}}
{{ printf "%s-%s" .Release.Name .Chart.Name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "<subchart>.labels" -}}
app.kubernetes.io/name: {{ include "<subchart>.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | default .Chart.Version | quote }}
app.kubernetes.io/part-of: kube-llmops
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version }}
{{- end }}

{{- define "<subchart>.selectorLabels" -}}
app.kubernetes.io/name: {{ include "<subchart>.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
```

**任务 3: 更新所有模板中的标签引用**

将所有子 Chart 的 Deployment/Service/ConfigMap 等资源的 `metadata.labels` 和 `spec.selector.matchLabels` 替换为 helper 调用:

```yaml
metadata:
  labels:
    {{- include "<subchart>.labels" . | nindent 4 }}
spec:
  selector:
    matchLabels:
      {{- include "<subchart>.selectorLabels" . | nindent 6 }}
```

**任务 4: 更新依赖标签的组件**

- Prometheus SD relabel (Story 1.3) 基于新标准标签
- NetworkPolicy 选择器基于新标准标签
- OTel Collector 的 label selector 更新
- Grafana Dashboard 中按标签过滤的查询更新

### 验收标准 (DoD)

- [ ] 所有 16 个子 Chart 都有 `_helpers.tpl` 文件
- [ ] `helm template` 渲染的所有资源包含完整的标准标签集
- [ ] `kubectl get pods -l app.kubernetes.io/part-of=kube-llmops` 列出所有组件 Pod
- [ ] Prometheus SD 基于标准标签发现所有 target
- [ ] NetworkPolicy 基于标准标签选择 Pod
- [ ] 全部 E2E 测试通过 (标签变更不影响功能)

---

## Story 2.5: 升级指南 + 迁移框架

### 标签
`priority/P1` `area/docs` `kind/documentation` `effort/1.5d`

### 为什么要做 (Why)

项目从 v0.1.0 到 v0.2.0 已经发布,但**没有任何升级文档**。用户面对以下问题完全无处可查:
- `helm upgrade` 会发生什么?数据安全吗?
- values.yaml 格式在版本间有没有 breaking change?
- PostgreSQL schema 是否兼容?需要迁移吗?
- 升级失败怎么回滚?

Quality Gate (pre-upgrade hook) 检查 RAG 质量指标是良好的开端,但不检查基础设施健康。

### 要做什么 (What)

**任务 1: 创建 `docs/guides/upgrade-guide.md`**

包含:

1. **通用升级流程**:
   ```bash
   # 1. 备份数据
   kubectl exec <pg-pod> -- pg_dumpall > backup-$(date +%Y%m%d).sql
   
   # 2. 查看变更
   helm diff upgrade kube-llmops charts/kube-llmops-stack -f values.yaml
   
   # 3. 执行升级
   helm upgrade kube-llmops charts/kube-llmops-stack -f values.yaml --timeout 15m
   
   # 4. 验证
   kubectl get pods --watch
   helm test kube-llmops
   
   # 5. 回滚 (如果出问题)
   helm rollback kube-llmops
   ```

2. **版本特定迁移说明**:
   - v0.1.0 → v0.2.0: 新增了哪些组件、哪些 values 字段变更、PG schema 是否兼容
   - v0.2.0 → v0.3.0 (预告): PG 拆分迁移步骤

3. **Breaking Changes 清单**: 每个版本的不兼容变更列表

4. **回滚流程**: `helm rollback` 的详细步骤和注意事项

**任务 2: pre-upgrade hook 增加基础设施健康检查**

在 `charts/rag-eval/templates/quality-gate.yaml` 中扩展 pre-upgrade 检查,增加:
- PostgreSQL 连接性检查
- LiteLLM 健康检查
- 磁盘空间检查 (PVC 使用率)

**任务 3: CHANGELOG.md 规范化**

确保 CHANGELOG.md 中每个版本明确标注 "Breaking Changes" 和 "Migration Required" 段落。

### 验收标准 (DoD)

- [ ] `docs/guides/upgrade-guide.md` 存在,包含通用流程 + v0.1→v0.2 特定说明
- [ ] 文档包含回滚流程和数据备份步骤
- [ ] pre-upgrade hook 检查 PG 连接和 LiteLLM 健康
- [ ] CHANGELOG.md 每个版本有 "Breaking Changes" 段落

---

## Story 2.6: NetworkPolicy 补全 + Egress 规则

### 标签
`priority/P1` `area/security` `kind/hardening` `effort/2d`

### 为什么要做 (Why)

当前 `charts/security/templates/network-policies.yaml` 仅定义了 **5 个 NetworkPolicy** (外加 1 个 deny-default),只覆盖了 vLLM、LiteLLM、Prometheus、Grafana 四个服务。以下 **12 个服务完全无 NetworkPolicy 保护**:

| 无保护的服务 | 风险 |
|-------------|------|
| PostgreSQL | 任何 Pod 可直接连接数据库 |
| Redis (Langfuse + Dify 各一个) | 任何 Pod 可读写缓存 |
| ClickHouse | 任何 Pod 可查询追踪数据 |
| Langfuse | 无 Ingress 限制 |
| Dify API/Web/Worker/Plugin | 无 Ingress 限制 |
| Keycloak | 无 Ingress 限制 |
| MinIO | 任何 Pod 可读写对象存储 |
| TEI | 无 Ingress 限制 |
| Neo4j | 任何 Pod 可查询图数据库 |

更严重的是,**零 Egress 规则** --- 所有 Pod 可以自由访问外网。如果任何一个组件被攻陷 (如通过 Prompt Injection 攻击 vLLM),攻击者可以:
- 将数据外泄到外部服务器
- 使用 GPU 挖矿
- 作为跳板攻击内网其他服务

### 要做什么 (What)

**任务 1: 为所有组件添加 Ingress NetworkPolicy**

为每个服务定义"谁可以访问我":

| 目标服务 | 允许的源 | 端口 |
|----------|---------|------|
| PostgreSQL (operator-pg) | litellm | 5432 |
| PostgreSQL (app-pg) | langfuse, dify-api, dify-worker, dify-plugin-daemon | 5432 |
| Redis (Langfuse) | langfuse, langfuse-worker | 6379 |
| Redis (Dify) | dify-api, dify-worker, dify-plugin-daemon | 6379 |
| ClickHouse | langfuse, langfuse-worker | 8123, 9000 |
| Langfuse | Ingress Controller (Traefik) | 3000 |
| Dify API | Ingress Controller, dify-web, dify-setup-job | 5001 |
| Dify Web | Ingress Controller | 3000 |
| Keycloak | Ingress Controller, grafana, langfuse, litellm | 8080 |
| MinIO | langfuse, dify, litellm | 9000 |
| TEI | litellm | 8080 |
| Neo4j | lightrag | 7474, 7687 |
| LightRAG | Ingress Controller (可选) | 9621 |
| Milvus | litellm (如需要) | 19530 |

**任务 2: 为关键组件添加 Egress NetworkPolicy**

```yaml
# vLLM: 只允许回连 LiteLLM + 外部 HTTPS (模型下载)
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: vllm-egress
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: vllm
  policyTypes: ["Egress"]
  egress:
    - to:
        - podSelector:
            matchLabels:
              app.kubernetes.io/part-of: kube-llmops
    - to:
        - ipBlock:
            cidr: 0.0.0.0/0
      ports:
        - port: 443   # HTTPS (HuggingFace 模型下载)
        - port: 53     # DNS
          protocol: UDP
```

为 PostgreSQL、LiteLLM、Langfuse 等关键组件创建类似的 Egress 策略。

**任务 3: NetworkPolicy values 配置**

在 `security/values.yaml` 中允许用户按组件开关:
```yaml
networkPolicy:
  enabled: true
  ingress:
    postgresql: true
    redis: true
    clickhouse: true
    # ...
  egress:
    enabled: false  # 默认不启用 Egress (避免破坏模型下载)
    vllm: false
    litellm: false
```

### 验收标准 (DoD)

- [ ] 所有 16 个服务都有对应的 Ingress NetworkPolicy
- [ ] 默认拒绝策略 (deny-default) + 显式放行规则覆盖所有组件间通信路径
- [ ] 启用 Egress 后,vLLM 仅能访问内部服务 + HTTPS (模型下载)
- [ ] `kubectl exec <pg-pod> -- curl http://external-site.com` 在启用 Egress 后被拒绝
- [ ] 全部 E2E 测试通过 (NetworkPolicy 不阻断合法通信)
- [ ] NetworkPolicy 可通过 values 按组件开关

---

## Story 2.7: CI 自动化测试集成

### 标签
`priority/P1` `area/ci` `kind/enhancement` `effort/2d`

### 为什么要做 (Why)

QA 团队已经创建了一套 135 项测试 (01-deploy-verify.sh: 58 项, 02-k8s-resource-test.py: 66 项, 04-edge-case-test.sh: 11 项),但这些测试目前**只能手动执行**。它们不在 CI pipeline 中,每次 PR 合并都可能引入破坏部署的变更而无人知晓。

当前 CI 的测试覆盖:
- `lint.yaml`: 静态检查 (helm lint, yaml lint, shellcheck)
- `test.yaml`: 模板渲染 (6 profiles) + Model Resolver 单测 (28 tests) + chart-install-test (best-effort)
- `e2e.yaml`: 每周运行,kind 集群上的 E2E

缺失的是:**基础设施层的自动化验证** --- Pod 状态、PVC 绑定、GPU 调度、健康端点、资源配置。这正是 QA 脚本覆盖的内容。

### 要做什么 (What)

**任务 1: 将 QA 测试脚本集成到 CI**

在 `.github/workflows/test.yaml` 的 `chart-install-test` job 中,chart install 成功后运行 QA 脚本:

```yaml
- name: Run infra verification
  run: |
    # 适配 CI 环境 (无 GPU, CI profile)
    bash improvement/test/scripts/01-deploy-verify.sh --ci-mode
    
- name: Run K8s resource tests
  run: |
    uv run improvement/test/scripts/02-k8s-resource-test.py --ci-mode
```

**任务 2: 为 CI 环境适配测试脚本**

QA 脚本当前假设 GPU 环境。需要添加 `--ci-mode` 参数:
- 跳过 GPU 相关检查 (nvidia-smi, vLLM GPU allocation)
- 跳过 Phase 4 组件检查 (LightRAG, Milvus, Presidio --- CI profile 不启用)
- 降低超时时间 (CI 环境资源有限)
- 输出 JUnit XML 格式 (GitHub Actions 可解析)

**任务 3: 测试结果上传为 CI Artifact**

```yaml
- name: Upload test results
  if: always()
  uses: actions/upload-artifact@v4
  with:
    name: infra-test-results
    path: improvement/test/test-report-*.txt
```

**任务 4: kind 集群配置优化**

确保 kind 集群配置足以运行 CI profile:
- 内存限制合理 (LiteLLM ~1.5GB, PostgreSQL ~512MB)
- 镜像预拉取 (避免超时)
- StorageClass 可用 (local-path-provisioner)

### 验收标准 (DoD)

- [ ] 每个 PR 自动运行 QA 基建验证脚本
- [ ] CI 环境下 01-deploy-verify.sh 的 CI-mode 检查全部通过
- [ ] 测试结果以 Artifact 形式可在 GitHub Actions 中下载查看
- [ ] QA 脚本输出 JUnit XML 格式,PR 中可查看测试摘要
- [ ] 3 次连续 CI 运行稳定通过 (无 flaky test)

---

## Story 2.8: Grafana 成本/团队用量 Dashboard

### 标签
`priority/P1` `area/observability` `kind/feature` `effort/2d`

### 为什么要做 (Why)

README.md 的使用场景 #2 描述:

> "5 个团队都要用 GPU,怎么做 Token 预算限制和成本追踪？" → LiteLLM 网关 + Key 管理

但实际上,项目**没有任何成本/用量的 Grafana Dashboard** (差距清单 #56)。LiteLLM 本身记录了每个 API Key 的 Token 消耗和成本数据 (存储在 PostgreSQL),但这些数据没有被可视化。

用户看不到:
- 各团队/API Key 的 Token 消耗趋势
- 各模型的请求量和成本分布
- 预算消耗进度和告警

这是 README 中明确承诺但完全未兑现的能力。

### 要做什么 (What)

**任务 1: 创建 LiteLLM 指标采集**

LiteLLM 通过 Prometheus endpoint 暴露以下指标:
- `litellm_total_tokens` (按 model, api_key 分维度)
- `litellm_requests_total`
- `litellm_spend_total`

确认 Prometheus scrape 配置已采集这些指标 (Story 1.3 的 K8s SD 应已覆盖)。

**任务 2: 创建 Cost & Usage Dashboard**

在 `charts/observability/dashboards/` 中新增 `cost-usage-dashboard.json`:

| Panel | PromQL | 说明 |
|-------|--------|------|
| Token 消耗趋势 (按团队) | `sum by (api_key)(rate(litellm_total_tokens[1h]))` | 折线图,按 API Key 分组 |
| 请求量 (按模型) | `sum by (model)(rate(litellm_requests_total[1h]))` | 柱状图 |
| 成本趋势 (按模型) | `sum by (model)(increase(litellm_spend_total[24h]))` | 折线图 |
| 各团队成本排行 | `topk(10, sum by (api_key)(litellm_spend_total))` | 排行榜 |
| 预算使用率 | `litellm_spend_total / litellm_budget_total * 100` | 仪表盘 |
| Token 类型分布 | `sum by (type)(litellm_tokens{type=~"prompt\|completion"})` | 饼图 |

**任务 3: Grafana provisioning 自动加载**

确保 `charts/observability/templates/grafana.yaml` 的 Dashboard provisioning ConfigMap 包含新 Dashboard。

**任务 4: 添加成本相关告警规则**

在 Prometheus 告警规则中新增:
```yaml
- alert: TeamBudgetExceeded80Pct
  expr: (litellm_spend_total / litellm_budget_total) > 0.8
  labels:
    severity: warning
- alert: TeamBudgetExceeded100Pct
  expr: (litellm_spend_total / litellm_budget_total) > 1.0
  labels:
    severity: critical
```

### 验收标准 (DoD)

- [ ] Grafana 中新增 "Cost & Usage" Dashboard,含至少 6 个 Panel
- [ ] Dashboard 可按 API Key / 模型筛选
- [ ] 使用不同 API Key 发送请求后,Dashboard 数据正确分组显示
- [ ] 预算相关告警规则已添加到 Prometheus
- [ ] Dashboard JSON 通过 Grafana provisioning 自动加载 (无需手动导入)

---

## Story 2.9: Langfuse OIDC 集成

### 标签
`priority/P1` `area/security` `kind/enhancement` `effort/0.5d`

### 为什么要做 (Why)

当前平台中 Grafana、MinIO、LiteLLM 三个服务都已对接 Keycloak OIDC SSO,但 **Langfuse 是唯一没有对接的服务** (差距清单 #37)。

`charts/langfuse/values.yaml` 中 OIDC 配置默认为 disabled:
```yaml
oidc:
  enabled: false
```

模板中也没有相应的 `AUTH_*` 环境变量注入。这意味着 Langfuse 使用独立的用户名/密码认证,与其他所有服务的 SSO 体验不一致。用户需要单独记住 Langfuse 的凭据。

### 要做什么 (What)

**任务 1: Langfuse Deployment 添加 OIDC 环境变量**

在 `charts/langfuse/templates/deployment.yaml` 中添加:
```yaml
{{- if .Values.oidc.enabled }}
- name: AUTH_DISABLE_USERNAME_PASSWORD
  value: "true"
- name: AUTH_CUSTOM_CLIENT_ID
  value: "langfuse"
- name: AUTH_CUSTOM_CLIENT_SECRET
  value: {{ .Values.oidc.clientSecret }}
- name: AUTH_CUSTOM_ISSUER
  value: "https://keycloak.{{ .Values.global.ingress.host }}/realms/kube-llmops"
{{- end }}
```

**任务 2: Keycloak Realm 中添加 langfuse 客户端**

确认 `charts/keycloak/values.yaml` 的 `realm.clients` 列表中包含 `langfuse`:
```yaml
clients:
  - grafana
  - langfuse
```

**任务 3: values-single-node.yaml 中启用 OIDC**

```yaml
langfuse:
  oidc:
    enabled: true
    clientSecret: "langfuse-oidc-secret"
```

### 验收标准 (DoD)

- [ ] Langfuse 登录页面显示 "Login with SSO" 按钮
- [ ] 通过 Keycloak SSO 可成功登录 Langfuse
- [ ] Keycloak 用户创建后可同时访问 Grafana 和 Langfuse (统一身份)
- [ ] `oidc.enabled: false` 时仍可使用用户名/密码登录 (向后兼容)

---

## Story 2.10: 备份 CronJob 模板

### 标签
`priority/P1` `area/operations` `kind/feature` `effort/1d`

### 为什么要做 (Why)

差距清单 #32 指出: 项目提到了 `scripts/backup.sh` 但实际上架构师确认该脚本不存在。对于一个拥有 4+ 数据库、多个 PVC 的平台,**没有自动化备份是严重的运维缺陷**。

PostgreSQL 承载了全平台的关键数据:
- litellm: API 密钥、开支记录、限流配置
- langfuse: 追踪元数据
- dify: RAG 知识库配置、工作流定义
- dify_plugin: 插件状态

一次误操作或 PVC 损坏就意味着全部数据丢失。

### 要做什么 (What)

**任务 1: 创建 PostgreSQL 备份 CronJob 模板**

在 `charts/kube-llmops-stack/charts/litellm/templates/` (或 Phase 2 拆分后的 postgresql 子 Chart) 中添加 `backup-cronjob.yaml`:

```yaml
{{- if .Values.backup.enabled }}
apiVersion: batch/v1
kind: CronJob
metadata:
  name: {{ .Release.Name }}-pg-backup
spec:
  schedule: {{ .Values.backup.schedule | default "0 2 * * *" | quote }}
  jobTemplate:
    spec:
      template:
        spec:
          containers:
            - name: pg-backup
              image: pgvector/pgvector:pg17
              command:
                - /bin/bash
                - -c
                - |
                  TIMESTAMP=$(date +%Y%m%d-%H%M%S)
                  pg_dumpall -h {{ .Release.Name }}-operator-pg -U litellm \
                    > /backup/full-backup-${TIMESTAMP}.sql
                  
                  # 保留策略: 删除超过 N 天的备份
                  find /backup -name "*.sql" -mtime +{{ .Values.backup.retentionDays | default 7 }} -delete
                  
                  echo "Backup completed: full-backup-${TIMESTAMP}.sql"
                  ls -la /backup/
              env:
                - name: PGPASSWORD
                  valueFrom:
                    secretKeyRef:
                      name: {{ .Release.Name }}-operator-pg-secret
                      key: postgresql-password
              volumeMounts:
                - name: backup-storage
                  mountPath: /backup
          volumes:
            - name: backup-storage
              persistentVolumeClaim:
                claimName: {{ .Release.Name }}-pg-backup
          restartPolicy: OnFailure
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: {{ .Release.Name }}-pg-backup
spec:
  accessModes: ["ReadWriteOnce"]
  resources:
    requests:
      storage: {{ .Values.backup.storageSize | default "10Gi" }}
{{- end }}
```

**任务 2: values.yaml 暴露备份配置**

```yaml
backup:
  enabled: false   # 默认关闭,production profile 开启
  schedule: "0 2 * * *"   # 每天凌晨 2 点
  retentionDays: 7         # 保留 7 天
  storageSize: 10Gi
```

`values-production.yaml` 中:
```yaml
backup:
  enabled: true
```

**任务 3: 创建恢复文档**

`docs/guides/backup-restore.md`:
- 如何手动触发备份: `kubectl create job pg-backup-manual --from=cronjob/<name>`
- 如何恢复数据: `kubectl exec <pg-pod> -- psql -f /backup/full-backup-xxx.sql`
- MinIO 数据备份 (mc mirror)
- ClickHouse 数据备份

### 验收标准 (DoD)

- [ ] `values-production.yaml` 启用备份后,`helm template` 渲染出 CronJob + PVC
- [ ] 手动触发 CronJob 后,备份 SQL 文件出现在 PVC 中
- [ ] 保留策略生效: 超过 retentionDays 的备份文件被自动删除
- [ ] 恢复文档存在且步骤可执行
- [ ] `values-ci.yaml` 和 `values-single-node.yaml` 中备份默认关闭 (不影响开发体验)
