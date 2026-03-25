# kube-llmops 基础设施测试报告 (Infra Test Report)

> 日期: 2026-03-24 | 版本: Phase 4 (c1e91d4) | 执行者: Cloud-Native & AI Infra QA Architect

---

## 测试环境

| 环境 | 节点 | K8s 版本 | GPU | 部署配置 |
|------|------|---------|-----|---------|
| 本地 (WSL2) | w-26bw4d4 | v1.34.5+k3s1 | 1x NVIDIA GPU (16 vCPU, 32GB RAM) | single-node (Qwen2.5-0.5B) + Phase 4 全组件 |
| GB10 远程 | promaxgb10-5c13 | v1.34.5+k3s1 | 1x NVIDIA GB10 | single-node (Qwen3-5-35B-A3B-GPTQ), Phase 4 组件未启用 |

---

## Phase 4 新增组件

| 组件 | 类型 | 端口 | 健康端点 | 状态 (本地) |
|------|------|------|---------|------------|
| LightRAG | Graph-based RAG | 9621 | - | Running |
| Neo4j | 图数据库 | 7687 (bolt), 7474 (http) | `/` :7474 | Running |
| Milvus | 向量数据库 | 19530 (gRPC), 9091 (http) | `/healthz` :9091 | Running |
| Milvus etcd | 元数据存储 | 2379 | - | Running |
| Presidio Analyzer | PII 检测 | 3000 | `/health` | Running |
| Presidio Anonymizer | PII 脱敏 | 3000 | `/health` | Running |
| Multi-tenant | 团队隔离 | - | - | 2 teams 创建 |

---

## 测试结果汇总

### 本地环境 (WSL2) — Phase 4 全组件

| 测试脚本 | 总数 | 通过 | 失败 | 告警 | 结果 |
|----------|------|------|------|------|------|
| 01-deploy-verify.sh | 58 | 57 | 0 | 1 | PASS |
| 02-k8s-resource-test.py | 66 | 64 | 0 | 2 | PASS |
| 04-edge-case-test.sh | 11 | 10 | 0 | 1 | PASS |
| **合计** | **135** | **131** | **0** | **4** | **PASS** |

### GB10 远程环境 — Phase 4 未启用

| 测试脚本 | 总数 | 通过 | 失败 | 告警 | 结果 |
|----------|------|------|------|------|------|
| 01-deploy-verify.sh | 58 | 39 | 0 | 19* | PASS |
| 04-edge-case-test.sh | 11 | 11 | 0 | 0 | PASS |
| **合计** | **69** | **50** | **0** | **19** | **PASS** |

> *GB10 未启用 Phase 4 组件 (LightRAG/Milvus/Presidio/Neo4j)，且使用不同模型 (Qwen3-5-35B)，告警均为"Service 不存在"属预期

---

## 详细测试结果

### 1. 集群基础设施 (两环境均 PASS)
- kubectl 连接: PASS
- 节点状态 Ready: PASS
- GPU allocatable >= 1: PASS
- NVIDIA Device Plugin: PASS
- StorageClass (local-path): PASS

### 2. Pod 状态 (本地 31 Pod 全部正常)

**原有组件 (两环境均 PASS):**
- PostgreSQL / MinIO / LiteLLM / Langfuse: PASS
- vLLM / TEI: PASS
- Dify API/Web/Worker/Plugin-Daemon: PASS
- Keycloak / Grafana / Prometheus: PASS
- LLM-Guard / Loki: PASS

**Phase 4 新组件 (本地 PASS):**
- LightRAG: 1/1 Running
- Neo4j: 1/1 Running
- Milvus: 2/2 Running (standalone + etcd)
- Presidio Analyzer: 1/1 Running
- Presidio Anonymizer: 1/1 Running

### 3. PVC 持久化 (本地 14 个 PVC 全部 Bound)

**新增 PVC (Phase 4):**
- `kube-llmops-neo4j-data`: 5Gi Bound
- `kube-llmops-milvus-data`: 10Gi Bound
- `etcd-data-kube-llmops-milvus-etcd-0`: 2Gi Bound

**原有 PVC:** 11 个全部 Bound

### 4. GPU 资源调度 (两环境均 PASS)
- vLLM 获取 1 GPU: PASS
- TEI CPU-only: PASS
- GPU 无超分: PASS
- GPU 不足调度行为 (Pending + Event): PASS

### 5. Service 端点 (本地 18/18 PASS)

**新增 Service (Phase 4):**
- `kube-llmops-lightrag:9621`: 1 endpoint
- `kube-llmops-neo4j:7687`: 1 endpoint
- `kube-llmops-milvus:19530`: 1 endpoint
- `kube-llmops-presidio-analyzer:3000`: 1 endpoint
- `kube-llmops-presidio-anonymizer:3000`: 1 endpoint

### 6. 健康端点 (本地 15/15 PASS)

**新增健康检查 (Phase 4):**
- Neo4j `/` :7474 → HTTP 200
- Milvus `/healthz` :9091 → HTTP 200
- Presidio Analyzer `/health` :3000 → HTTP 200
- Presidio Anonymizer `/health` :3000 → HTTP 200

### 7. AI 功能验证 (本地 PASS)
- Embedding 生成 (bge-small-en): PASS (384 维)
- LLM 推理 (qwen2-5-0-5b): PASS (回答: 4)

### 8. 数据持久化验证 (本地 PASS)
- PostgreSQL 5 个数据库完整: litellm, langfuse, dify, dify_plugin, postgres
- 14 个 PVC 挂载关系正确

### 9. 边缘异常场景 (两环境均 PASS)
- OOMKilled 检测: PASS
- ImagePullBackOff 检测: PASS
- QoS Class (关键组件 Burstable): PASS
- PostgreSQL 数据持久化 (Pod 重启): PASS
- GPU 资源不足调度拒绝: PASS

---

## 发现的问题与建议

### 已发现问题
1. **Milvus etcd QoS=BestEffort** — 无资源 requests/limits，在资源紧张时容易被驱逐
2. **Langfuse 历史重启 7 次** — 启动时 ClickHouse/PostgreSQL 未就绪，最终自愈
3. **LightRAG 无健康检查端点** — 模板未配置 readinessProbe/livenessProbe

### Phase 4 改进建议
1. **Milvus etcd**: 添加 `resources.requests` 使 QoS 提升至 Burstable
2. **LightRAG**: 添加 HTTP 健康检查 (port 9621) 到 Deployment 模板
3. **Neo4j PVC**: 考虑增大默认存储 (5Gi → 20Gi) 以支持大型知识图谱
4. **Presidio**: 考虑添加 PodDisruptionBudget 保障 PII 防护服务可用性

---

## 文件产出物

```
improvement/test/
├── 01-INFRA-TEST-PLAN.md          # 阶段一: 测试计划
├── 02-TEST-CASES.md               # 阶段二: 5 个核心测试用例
├── 03-EXECUTION-GUIDE.md          # 阶段四: 执行指南
├── 04-TEST-REPORT.md              # 本文件: 测试报告
└── scripts/
    ├── 01-deploy-verify.sh        # Bash 部署验证 (58 检查点, 含 Phase 4)
    ├── 02-k8s-resource-test.py    # Python K8s SDK 测试 (66 检查点, 含 Phase 4)
    ├── 03-helm-test-template.yaml # Helm test 模板
    └── 04-edge-case-test.sh       # 边缘异常测试 (11 检查点)
```
