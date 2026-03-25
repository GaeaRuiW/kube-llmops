# kube-llmops 全局技术改进 --- Story 总览

> **CTO 决策文档集**
> **日期**: 2026-03-25
> **版本**: 基于 v0.2.0 (Phase 4, commit c1e91d4)

---

## 文档结构

```
improvement/CTO/
├── README.md                ← 本文件 (Story 总览与索引)
├── CTO-DECISION.md          ← CTO 执行摘要、冲突裁决、路线图概览
│
│  Story 详细拆解 (Why + What + DoD)
├── STORIES-PHASE1.md        ← Phase 1 详细 Story (8 个, P0 级, 2 周)
├── STORIES-PHASE2.md        ← Phase 2 详细 Story (10 个, P1 级, 1 个月)
├── STORIES-PHASE3.md        ← Phase 3 详细 Story (9 个, P2 级, 2-3 个月)
│
│  验收测试方法 (每条验收标准的可执行命令 + 预期输出 + 失败判定)
├── TESTING-PHASE1.md        ← Phase 1 全部 8 Story 的验收测试脚本
├── TESTING-PHASE2.md        ← Phase 2 全部 10 Story 的验收测试脚本
└── TESTING-PHASE3.md        ← Phase 3 全部 9 Story 的验收测试脚本
```

### 文档阅读指引

| 你是谁 | 先看什么 |
|--------|---------|
| **CTO / 技术总监** | `CTO-DECISION.md` → 本文件的全景图 |
| **开发者 (接手实现)** | `STORIES-PHASEx.md` → 找到自己负责的 Story → 看 Why + What |
| **QA / 测试工程师** | `TESTING-PHASEx.md` → 对应 Story 的验收测试命令 |
| **项目经理** | 本文件的全景图 → 依赖关系图 → 跟踪进度 |

---

## Story 全景图

### Phase 1: 救火与基建 (P0, 2 周, ~6-7 人天)

| Story | 标题 | 工作量 | 核心目标 |
|-------|------|--------|---------|
| **1.1** | 全组件 PodDisruptionBudget | 0.5d | 防止 node drain 导致全平台宕机 |
| **1.2** | 凭据随机化 + existingSecret | 1.5d | 消除 17 个硬编码密码的安全硬伤 |
| **1.3** | Prometheus K8s 服务发现 | 1d | 解除多模型/多实例的部署限制 |
| **1.4** | 文档一致性修复 | 1d | 消除 45 项"承诺 vs 实现"差距,修复信任危机 |
| **1.5** | NOTES.txt 部署后引导 | 0.5d | 降低新用户上手摩擦 |
| **1.6** | CI chart-install-test 强制通过 | 0.5d | 堵住 CI 安全漏洞 |
| **1.7** | Milvus etcd 资源配置 | 0.5d | 防止 etcd 被 K8s OOM 驱逐 |
| **1.8** | LightRAG 健康检查探针 | 0.5d | 实现 LightRAG 故障自动恢复 |

### Phase 2: 稳定性与提效 (P1, 1 个月, ~19 人天)

| Story | 标题 | 工作量 | 核心目标 |
|-------|------|--------|---------|
| **2.1** | PostgreSQL 拆分 (2 实例) | 5d | 消除全平台数据库单点故障 |
| **2.2** | values.schema.json | 2d | 配置错误前移到 lint 阶段发现 |
| **2.3** | AlertManager + 通知渠道 | 1.5d | 让 11 条告警规则真正触达运维人员 |
| **2.4** | Helm 标签标准统一 | 1.5d | 打通 Prometheus SD / NetworkPolicy / kubectl 选择器 |
| **2.5** | 升级指南 + 迁移框架 | 1.5d | 给用户提供版本升级的安全路径 |
| **2.6** | NetworkPolicy 补全 + Egress | 2d | 从 4/16 服务覆盖扩展到 16/16 + Egress |
| **2.7** | CI 自动化测试集成 | 2d | 135 项 QA 测试从手动变为每 PR 自动执行 |
| **2.8** | Grafana 成本/团队 Dashboard | 2d | 兑现 README 中的成本追踪承诺 |
| **2.9** | Langfuse OIDC 集成 | 0.5d | 补齐 SSO 最后一块拼图 |
| **2.10** | 备份 CronJob 模板 | 1d | 自动化数据备份,防止数据丢失 |

### Phase 3: 演进与护城河 (P2, 2-3 个月)

| Story | 标题 | 工作量 | 核心目标 |
|-------|------|--------|---------|
| **3.1** | 有状态组件 HA 加固 | 8d | 企业客户的 Day-0 硬性要求 |
| **3.2** | 全栈可观测性深耕 | 5d | 建立"GPU→Token→成本"的竞争壁垒 |
| **3.3** | External Secrets Operator | 3d | 打通企业密钥管理体系 |
| **3.4** | Model Resolver 集成 | 3d | 兑现"引擎自动选择"核心卖点 |
| **3.5** | RAG 质量保障深耕 | 5d | 深化质量门控差异化护城河 |
| **3.6** | 开发者体验提升 | 3d | Tilt/ADR/pre-commit 加速贡献者迭代 |
| **3.7** | GitOps 集成 (ArgoCD) | 3d | 企业标准交付方式 |
| **3.8** | 多租户成熟化 | 5d | 从"隔离"到"管理+计量"的平台进化 |
| **3.9** | 性能基线与压力测试 | 3d | 用数据支撑"生产级"声明 |

---

## 总计

| 阶段 | Story 数量 | 总工作量 | 时间窗口 |
|------|-----------|---------|---------|
| Phase 1 | 8 | ~6-7 人天 | 2 周 |
| Phase 2 | 10 | ~19 人天 | 1 个月 |
| Phase 3 | 9 | ~38 人天 | 2-3 个月 |
| **合计** | **27** | **~63 人天** | **~4 个月** |

---

## 依赖关系

```
Phase 1 (全部可并行, 互不依赖)
  ├── 1.1 PDB
  ├── 1.2 凭据随机化        ──→  1.5 NOTES.txt (依赖 Secret 获取命令)
  ├── 1.3 Prometheus SD     ──→  2.4 标签统一 (SD 依赖标准标签)
  ├── 1.4 文档修复
  ├── 1.5 NOTES.txt
  ├── 1.6 CI 强制通过       ──→  2.7 CI 测试集成
  ├── 1.7 Milvus etcd
  └── 1.8 LightRAG 探针

Phase 2
  ├── 2.1 PG 拆分           ──→  3.1 HA 加固 (在拆分后的 PG 上做 HA)
  ├── 2.2 Schema                  2.10 备份 CronJob (备份新的 PG 实例)
  ├── 2.3 AlertManager
  ├── 2.4 标签统一           ──→  2.6 NetworkPolicy (基于标准标签)
  ├── 2.5 升级指南
  ├── 2.6 NetworkPolicy
  ├── 2.7 CI 测试集成
  ├── 2.8 成本 Dashboard
  ├── 2.9 Langfuse OIDC
  └── 2.10 备份 CronJob

Phase 3 (大部分可并行)
  ├── 3.1 HA 加固
  ├── 3.2 可观测性深耕
  ├── 3.3 ESO              ←── 依赖 1.2 existingSecret 接口
  ├── 3.4 Model Resolver
  ├── 3.5 RAG 质量深耕
  ├── 3.6 开发者体验
  ├── 3.7 GitOps
  ├── 3.8 多租户
  └── 3.9 性能基线
```

---

## 核心决策原则 (来自 CTO-DECISION.md)

| 原则 | 说明 |
|------|------|
| **广度保持,深度加强** | 不砍功能模块,但所有资源聚焦工程质量加固 |
| **信任优先于功能** | 修文档的 ROI 远高于加新功能 |
| **低成本高回报优先** | PDB (0.5d) > PG 拆分 (5d),Phase 1 只做前者 |
| **逃生通道优先于完美方案** | existingSecret > 内置 PG HA |
| **可验证优先于可宣传** | 每个 Story 必须有测试覆盖的验收标准 |
