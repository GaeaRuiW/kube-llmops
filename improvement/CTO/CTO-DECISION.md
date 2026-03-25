# CTO 全局技术改进与执行计划

> **决策人**: CTO (AI Infrastructure)
> **决策日期**: 2026-03-25
> **输入来源**: 产品评估报告 (PM)、深度架构评审 (Architect)、基础设施测试报告 (QA)、承诺 vs 实现差距清单
> **决策版本**: 基于 v0.2.0 (Phase 4, commit c1e91d4)

---

## 1. CTO 执行摘要 (Executive Summary)

kube-llmops 是一个**定位精准、架构根基正确、但工程成熟度尚未匹配商业野心**的 MVP 产品。

它正确识别了 "Kubernetes 原生 LLMOps 一键部署" 这个真实且高价值的市场空白,用 Umbrella Helm Chart 模式整合了 15 个子组件,实现了从 GPU 推理到 RAG 到可观测性的全栈覆盖。功能广度在同类开源项目中处于领先位置。Phase 4 完成后, RAG 基础设施覆盖度达到 15/15,Playwright E2E + K8s 基建测试共 135 项全部通过,核心推理+监控+追踪闭环已经跑通 --- 这不是一个纸上谈兵的项目,它有真实可运行的代码和可验证的测试。

**然而,当前制约项目发展的"阿喀琉斯之踵"是:生产工程质量 (Production Engineering Quality) 与文档承诺之间的巨大鸿沟。**

具体表现为三层断裂:

1. **安全层断裂**: 全平台 9 组密码硬编码明文、无 `existingSecret` 机制 --- 对于自称面向"金融/政府/军工"的私有化平台,这是致命伤。任何安全评审都会一票否决。

2. **可靠性层断裂**: 单体 PostgreSQL 承载 4 个数据库(litellm/langfuse/dify/dify_plugin),全部有状态组件零副本冗余、零 PDB --- 一次 `kubectl drain` 或一次 PG Pod 重启就能导致全平台瘫痪。这让"生产级"的定位变成空话。

3. **信任层断裂**: 文档声明了约 150 项能力,实际可用约 75 项,约 45 项仅存在于文档愿景中且未标注状态。开源用户一旦发现"Done"标注的功能实际不存在,信任将不可逆地流失。

**总结判断**: 这是一个**具备极佳商业潜力的 MVP,但距离可对外推广的生产级产品有 2-3 个月的工程深耕距离**。当前阶段不应再扩展功能广度,而应全力加固工程深度。先让 5 个客户在生产中平稳运行 3 个月,比发布 50 个新功能更有价值。

---

## 2. 视角冲突解决与权衡决策 (Trade-offs & Decisions)

### 冲突 1: PM 要求"收缩砍模块" vs 架构师认为"广度是平台价值"

| 角色 | 诉求 |
|------|------|
| **PM (初版)** | 砍掉 Alluxio、Harbor、Milvus、Envoy Gateway 模板代码,聚焦核心 |
| **PM (修正后)** | 保留 disabled-by-default 模板,但标注 experimental |
| **架构师** | 保留模块化结构,但引入分层架构解耦依赖 |

**CTO 裁决: 采纳 PM 修正后立场 + 架构师分层建议,但限定执行范围。**

理由: 在 AI Infra 平台语境下,广度本身就是核心价值主张 --- 用户选择 kube-llmops 恰恰是因为"一个 Helm Chart 搞定一切"。砍掉模块等于砍掉卖点。但我们必须做到:
- 所有非核心组件默认 `enabled: false`,Chart.yaml 中通过 `condition` 控制
- 文档中严格标注 `[STABLE]` / `[BETA]` / `[EXPERIMENTAL]` / `[PLANNED]` 状态
- 不在 README 核心功能列表中宣传 EXPERIMENTAL 及以下状态的功能
- Phase 1 不投入任何资源在非核心模块上,全部聚焦工程质量

### 冲突 2: PM 要求"PostgreSQL 拆分" vs 资源/时间约束

| 角色 | 诉求 |
|------|------|
| **PM** | P0 级:至少拆成 operator-pg 和 app-pg 两个实例 |
| **架构师** | FATAL-01:推荐拆成 3 个独立 StatefulSet,最低要求拆 2 个 |
| **QA** | 测试报告确认 5 个数据库共享 1 个 PG 实例,数据持久化测试通过但单点风险未覆盖 |

**CTO 裁决: Phase 1 不拆 PostgreSQL,改为 Phase 2 执行。Phase 1 优先加 PDB + existingSecret。**

理由: PostgreSQL 拆分是高成本重构(架构师估计 3 天,实际含测试至少 1 周),涉及数据库迁移、连接字符串变更、所有子 Chart 的 values 重构。Phase 1 的 2 周时间应该优先投入在**高回报/低成本**的修复上:
- PDB(半天工作量,立即消除 node drain 风险)
- 凭据随机化 + existingSecret(1 天,消除安全硬伤)
- Prometheus SD(1 天,解锁多模型支持)

PostgreSQL 拆分放到 Phase 2,此时团队已完成 Phase 1 的基建加固,有更充裕的时间做架构级变更。但 Phase 1 必须完成 `existingSecret` 字段,这为用户指向外部托管 PG 提供了逃生通道。

### 冲突 3: QA 要求"多硬件覆盖测试" vs 团队资源有限

| 角色 | 诉求 |
|------|------|
| **QA** | 在 WSL2 + GB10 两个环境测试,建议增加多 GPU 拓扑测试、压力测试 |
| **架构师** | HIGH-04: 需要 GPU 拓扑感知调度 (NFD + TopologyManager) |
| **PM** | 大量 AI 工程师没有 K8s 经验,应提供 Docker Compose demo |

**CTO 裁决: Phase 1 不投入多硬件测试和 Docker Compose。Phase 2 引入 CI 中的 kind 集群自动化测试。**

理由:
- GPU 拓扑感知是**多卡 Tensor Parallelism 场景**才需要的,当前 MVP 阶段的目标用户主要是单卡或少量卡的团队。这是 P2 优先级。
- Docker Compose demo 听起来诱人,但会分散精力并创造一个全新的维护表面。不如投入同等时间优化 `values-ci.yaml`(CPU-only profile)让用户在无 GPU 环境也能体验核心流程。
- QA 当前的 135 项测试覆盖已经相当扎实。Phase 2 重点是把这些测试集成到 CI pipeline 中(`chart-install-test` 必须从 `continue-on-error: true` 改为强制通过)。

### 冲突 4: 架构师要求"分层架构重构" vs PM 要求"快速修文档止损"

| 角色 | 诉求 |
|------|------|
| **架构师** | 引入 Layer 0/1/2/3 分层架构,重构依赖图 |
| **PM** | 立即修正文档中的虚假"Done"标注,收敛根目录 8 个 RAG-*.md |

**CTO 裁决: 两者都做,但文档修正 Phase 1 立即执行,分层架构 Phase 2 渐进推进。**

理由:
- 文档信任问题是**零成本高回报**的修复 --- 改几个 Markdown 文件就能消除用户的信任危机。这必须在 Phase 1 完成。
- 分层架构是正确的方向,但这是重构级工作,需要修改 Chart.yaml 依赖关系、所有子 Chart 的 values 传递方式、以及可能的 Ingress 模板拆分。放在 Phase 2 与 PostgreSQL 拆分一起推进更合理。

### 冲突 5: 架构师要求"mTLS / Service Mesh" vs 当前阶段的实际需求

**CTO 裁决: 驳回,归入 Phase 3 长期规划。**

理由: Service Mesh 引入 Istio/Linkerd/Cilium mTLS 的运维复杂度和性能开销,与项目"一键部署"的核心调性矛盾。当前阶段的安全优先级是:密码随机化 > NetworkPolicy 补全 > Egress 策略 > mTLS。在绝大多数私有化部署场景中(用户自己的数据中心内网),mTLS 不是 Day-1 需求。

---

## 3. 统一落地路线图 (Unified Actionable Roadmap)

### Phase 1: 救火与基建 (P0, 接下来 2 周)

> **目标**: 消除全部安全硬伤、建立基本生产存活能力、修复信任危机。
> **原则**: 只做"高回报/低成本"的修复,不做架构级重构。

| # | 任务 | 来源 | 工作量 | 验收标准 |
|---|------|------|--------|----------|
| 1 | **全组件 PodDisruptionBudget** | 架构师 FATAL-04 | 0.5 天 | 所有 Deployment/StatefulSet 有 PDB,`kubectl drain` 不会同时驱逐所有副本 |
| 2 | **凭据随机化 + existingSecret** | 架构师 FATAL-05, PM P0 | 1.5 天 | 所有 values.yaml 密码字段使用 `randAlphaNum` 默认值;所有子 Chart 支持 `existingSecret` 字段;NOTES.txt 输出凭据提醒 |
| 3 | **Prometheus 硬编码 target 替换为 K8s SD** | 架构师 FATAL-02, PM P0 | 1 天 | Prometheus 通过 kubernetes_sd_configs 自动发现 vLLM/TEI Pod,支持多模型多实例 |
| 4 | **文档一致性修复** | PM P1, 差距清单 | 1 天 | ARCHITECTURE.md 所有组件标注 `[IMPLEMENTED]`/`[PLANNED]`;README Features 表中 KEDA/Auto-Selection 标为 `Partial`;根目录 RAG-*.md 合并收敛 |
| 5 | **NOTES.txt 部署后引导** | PM P1 | 0.5 天 | `helm install` 后输出:组件状态检查命令、各 UI 访问地址、默认凭据列表、下一步操作指引 |
| 6 | **CI chart-install-test 强制通过** | 架构师 P1 | 0.5 天 | 移除 `continue-on-error: true`,安装失败则 CI 红灯 |
| 7 | **Milvus etcd 资源配置** | QA 发现 | 0.5 天 | 添加 `resources.requests` 使 QoS 提升至 Burstable |
| 8 | **LightRAG 健康检查** | QA 发现 | 0.5 天 | Deployment 模板添加 readinessProbe/livenessProbe |

**Phase 1 总工作量: ~6-7 人天,2 周可完成(含测试验证)**

### Phase 2: 稳定性与提效 (P1, 接下来 1 个月)

> **目标**: 解耦核心架构瓶颈、建立自动化测试体系、补齐运维文档。
> **原则**: 允许架构级重构,但每个变更必须有回归测试覆盖。

| # | 任务 | 来源 | 工作量 | 验收标准 |
|---|------|------|--------|----------|
| 1 | **PostgreSQL 拆分(至少 2 实例)** | 架构师 FATAL-01 | 5 天 | litellm 独占 operator-pg;langfuse/dify/dify_plugin 共享 app-pg;两个实例独立 StatefulSet + PVC |
| 2 | **values.schema.json** | 架构师 P1, PM P1 | 2 天 | `helm install --dry-run` 能捕获必填字段缺失和类型错误 |
| 3 | **AlertManager 子 Chart + 通知渠道** | 差距清单 #28 | 1.5 天 | Prometheus 告警 → AlertManager → Slack/Webhook;至少覆盖 PG 宕机、GPU 利用率过高、vLLM 请求延迟 3 个告警场景 |
| 4 | **Helm 标签标准统一** | 架构师 HIGH-07 | 1.5 天 | 所有子 Chart 使用统一的 `app.kubernetes.io/*` 标签体系;NetworkPolicy 和 Prometheus SD 基于标准标签选择 |
| 5 | **升级指南 + 迁移框架** | 架构师 P2, PM P1 | 1.5 天 | `docs/upgrade-guide.md` 包含 v0.1→v0.2→v0.3 迁移说明、DB schema 兼容性说明、回滚流程 |
| 6 | **NetworkPolicy 补全** | 差距清单 #34-35 | 2 天 | 所有 15 个子 Chart 均有 Ingress 策略;vLLM/LiteLLM/PG 添加 Egress 策略(仅允许必要出站) |
| 7 | **CI 自动化测试集成** | QA 测试体系 | 2 天 | QA 的 01-deploy-verify.sh + 02-k8s-resource-test.py 集成到 GitHub Actions;每个 PR 在 kind 集群上运行基建验证 |
| 8 | **Grafana 成本/团队用量 Dashboard** | PM P2, 差距清单 #56 | 2 天 | 按 API Key / 团队维度展示 Token 消耗、请求量、成本趋势;满足 README Use Case #2 的承诺 |
| 9 | **Langfuse OIDC 集成** | 差距清单 #37 | 0.5 天 | Langfuse 对接 Keycloak SSO,与 Grafana/MinIO/LiteLLM 一致 |
| 10 | **备份 CronJob 模板** | 差距清单 #32 | 1 天 | K8s CronJob 自动执行 pg_dump,支持保留策略(默认保留 7 天) |

**Phase 2 总工作量: ~19 人天,1 个月可完成(含回归测试)**

### Phase 3: 演进与护城河 (P2, 中长期 2-3 个月)

> **目标**: 建立竞争壁垒、支持企业级场景、拓展生态。
> **原则**: 功能扩展必须伴随测试覆盖和文档更新,不再产生新的"承诺-实现"差距。

| # | 方向 | 任务 | 理由 |
|---|------|------|------|
| 1 | **HA 加固** | PostgreSQL HA (CloudNativePG / Bitnami Replication);Prometheus HA (VictoriaMetrics Sidecar);MinIO 分布式模式 | 企业客户的 Day-0 要求 |
| 2 | **全栈可观测性深耕** | 从 GPU → Token → 成本的统一 Dashboard;Prompt A/B 测试指标;SLO 框架 (docs/slo.md) | 这是竞品都做不好的领域,是最强护城河方向 |
| 3 | **External Secrets Operator** | 集成 AWS Secrets Manager / HashiCorp Vault 的参考实现 | 企业安全合规的阻塞项 |
| 4 | **Model Resolver 集成** | 完成 init-container 集成,实现"模型自动选引擎" | README 核心卖点,当前是空头承诺 |
| 5 | **RAG 质量保障深耕** | Ragas 持续评估 + 回归门控 + 多维度告警 + 数据更新 Pipeline | 质量门控已是独特差异化,做深可成为核心壁垒 |
| 6 | **开发者体验** | Tilt/Skaffold 本地开发环境;pre-commit hook 自动 `helm dependency update`;ADR 决策记录 | 降低贡献者门槛,加速社区成长 |
| 7 | **GitOps 集成** | ArgoCD ApplicationSet 参考实现 | 企业客户的标准交付方式 |
| 8 | **多租户成熟化** | 基于 Namespace 的完整隔离(Phase 4 已有基础),加上跨租户计量和计费 API | 从工具到平台的关键演进 |
| 9 | **性能基线** | 压力测试套件 (k6/Locust);各组件扩展上限基准;发布性能报告 | 建立"生产级"声明的数据支撑 |

---

## 4. 研发任务拆解 --- Phase 1 Issue / PR (P0 级)

以下 5 个 Issue 可直接发布到 GitHub,按优先级排序:

---

### Issue #1: [P0/Security] 全组件凭据随机化 + existingSecret 支持

**标签**: `priority/P0`, `area/security`, `kind/hardening`

**背景**: 当前全部 9 组密码(PostgreSQL, Grafana, Keycloak, MinIO, LiteLLM Master Key, Langfuse, LLM-Guard, Dify, Langfuse 加密密钥)在 values.yaml 中硬编码明文。用户使用 Quick Start 部署时会直接使用默认密码,造成严重安全隐患。

**任务**:
1. 所有 values.yaml 中的密码字段改为使用 Helm `randAlphaNum` 生成随机默认值
2. 所有子 Chart 添加 `existingSecret` 字段,允许用户引用预创建的 K8s Secret
3. NOTES.txt 中添加醒目的凭据提醒段落,输出所有自动生成的凭据及修改指引
4. 确保 `helm upgrade` 不会因为随机密码重新生成而破坏已有部署(使用 `lookup` 函数保持幂等)

**验收标准 (DoD)**:
- [ ] `helm install` 全新部署时,所有密码为随机值(非固定默认值)
- [ ] `helm upgrade` 时,已有 Secret 不被覆盖
- [ ] 每个子 Chart 的 values.yaml 包含 `existingSecret` 字段并有注释说明
- [ ] NOTES.txt 输出包含"Security Notice"段落
- [ ] `helm template` 渲染结果中无任何硬编码密码

---

### Issue #2: [P0/Reliability] 全组件 PodDisruptionBudget

**标签**: `priority/P0`, `area/reliability`, `kind/hardening`

**背景**: 整个代码库中 PDB 数量为零。在 Kubernetes 节点排空(rolling update, spot instance 回收)期间,一个组件的所有副本可能被同时驱逐,导致不可控中断。

**任务**:
1. 为所有 Deployment 和 StatefulSet 添加 PDB 模板
2. 单副本组件: `minAvailable: 0` 或通过 `{{ if gt (int .Values.replicaCount) 1 }}` 条件控制
3. 多副本组件(LiteLLM, Langfuse 等): `minAvailable: 1`
4. PDB 模板使用标准 Helm `_helpers.tpl` 标签选择器

**验收标准 (DoD)**:
- [ ] 每个子 Chart 目录中包含 `pdb.yaml` 模板
- [ ] `helm template` 渲染所有 profile 时,PDB 资源正确生成
- [ ] `kubectl drain` 测试:多副本 Deployment 至少保留 1 个 Pod 可用
- [ ] QA 边缘测试脚本 (04-edge-case-test.sh) 增加 PDB 验证检查点

---

### Issue #3: [P0/Observability] Prometheus 静态 scrape target 替换为 Kubernetes 服务发现

**标签**: `priority/P0`, `area/observability`, `kind/bug`

**背景**: `charts/observability/templates/prometheus.yaml` 中 vLLM 的 scrape target 硬编码了模型服务名 `vllm-qwen2-5-0-5b:8000`。这导致:添加第二个模型需要手动编辑 ConfigMap;多 Release 部署会名称冲突;无法自动发现新增的 AI 服务实例。

**任务**:
1. 将 vLLM scrape_config 从 `static_configs` 改为 `kubernetes_sd_configs` (role: pod)
2. 使用 `relabel_configs` 基于 `app.kubernetes.io/part-of` 和 `app.kubernetes.io/component` 标签过滤
3. 按 `{{ .Release.Namespace }}` 限定命名空间,支持多 Release 部署
4. 同步更新 TEI、LiteLLM、Pushgateway 等其他 scrape target(如有硬编码)
5. OTel Collector 配置也做相应调整,按 Release 隔离

**验收标准 (DoD)**:
- [ ] `helm template` 渲染的 Prometheus ConfigMap 中无任何硬编码服务名
- [ ] 部署后 Prometheus Targets 页面显示通过 SD 发现的 vLLM/TEI 端点
- [ ] 在 values.yaml 中添加第二个模型后,Prometheus 自动发现新 target(无需手动修改)
- [ ] `vllm:num_requests_running` 等核心指标在 Grafana Dashboard 中正常展示

---

### Issue #4: [P0/Trust] 文档一致性修复 --- 消除"承诺 vs 实现"差距

**标签**: `priority/P0`, `area/docs`, `kind/documentation`

**背景**: 承诺 vs 实现差距清单显示:文档声明约 150 项能力,实际可用约 75 项,约 45 项仅存在于文档愿景中。ARCHITECTURE.md 未区分已实现和未来规划;README Features 表中多项标注与实际不符。这是开源项目信任的生命线。

**任务**:
1. ARCHITECTURE.md 中每个组件/功能添加状态标签: `[IMPLEMENTED]` / `[BETA]` / `[TEMPLATE-ONLY]` / `[PLANNED vX.X]`
2. README.md Features 对比表:KEDA Auto-Scaling 标为 `Partial (requires KEDA Operator)`;Engine Auto-Selection 标为 `Partial (manual in values.yaml)`
3. 根目录 RAG-PLAN.md, RAG-ASSESSMENT.md, RAG-DIRECTION.md, RAG-TODO.md, RAG-TEST-PLAN.md 合并收敛为 `docs/rag/` 目录下的 1-2 个文档
4. PLAN.md 中未实现的文档/工具从 checklist `[x]` 改为 `[ ]` 或标注 `[PLANNED]`

**验收标准 (DoD)**:
- [ ] ARCHITECTURE.md 中每个功能描述旁有状态标签
- [ ] README Features 对比表与实际代码状态一致(CI 中可考虑添加校验脚本)
- [ ] 根目录 Markdown 文件从 8+ 个收敛到 5 个以内(README, ARCHITECTURE, CHANGELOG, CONTRIBUTING, AGENTS)
- [ ] `docs/` 目录结构化,包含 `docs/rag/`, `docs/guides/` 子目录

---

### Issue #5: [P0/UX] NOTES.txt 部署后引导 + CI 安装测试强制通过

**标签**: `priority/P0`, `area/ux`, `kind/enhancement`

**背景**: 当前 `helm install` 完成后无任何引导输出,用户需要自行 `kubectl get pods -w` 摸索。同时 CI 的 `chart-install-test` 使用 `continue-on-error: true`,安装失败不会阻断合并。

**任务**:
1. 编写 `charts/kube-llmops-stack/templates/NOTES.txt`:
   - 输出部署状态检查命令 (`kubectl get pods`)
   - 输出各 UI 的访问地址(Grafana, Langfuse, Dify, LiteLLM, Keycloak)
   - 输出 `/etc/hosts` 配置指引
   - 输出安全提醒(默认凭据列表 + 修改方式)
   - 输出 Quick Verification 步骤(curl 健康检查命令)
2. CI 修复:`.github/workflows/test.yaml` 中 `chart-install-test` 移除 `continue-on-error: true`

**验收标准 (DoD)**:
- [ ] `helm install` 完成后终端输出包含:访问地址、凭据提醒、健康检查命令
- [ ] `helm status kube-llmops` 可重新查看引导信息
- [ ] CI 中 chart-install-test 失败时 workflow 状态为红色(非绿色)
- [ ] NOTES.txt 内容随 values profile 动态变化(如:ci profile 不输出 GPU 相关信息)

---

## 附录: 决策原则总结

| 原则 | 说明 |
|------|------|
| **广度保持,深度加强** | 不砍功能模块,但所有资源聚焦工程质量加固 |
| **信任优先于功能** | 修文档的 ROI 远高于加新功能 |
| **低成本高回报优先** | PDB(半天) > PG 拆分(1 周),Phase 1 只做前者 |
| **逃生通道优先于完美方案** | existingSecret(让用户自带外部 PG) > 内置 PG HA |
| **可验证优先于可宣传** | 每个 Phase 的产出必须有测试覆盖,不再产生新的"承诺-实现"差距 |

---

*本文档基于三份评估报告的综合分析。所有工作量估算基于单人全职投入,实际可根据团队规模并行压缩周期。*
