# kube-llmops RAG 测试计划

> 每个 Phase 的验收标准、测试数据设计、评估方法。
> **原则：不通过验收的 Phase 不算完成，不进入下一阶段。**

---

## 一、测试分层

```
┌──────────────────────────────────────────────────┐
│  Level 4: 质量回归测试                             │  "RAG 质量有没有变差"
│  Ragas eval on data/model/prompt change            │
├──────────────────────────────────────────────────┤
│  Level 3: RAG 端到端质量评估                       │  "RAG 回答得好不好"
│  Ragas metrics: Faithfulness / Relevance / Precision│
├──────────────────────────────────────────────────┤
│  Level 2: RAG 端到端功能测试                       │  "RAG 能不能跑通"
│  上传文档 → embedding → 检索 → 回答 → trace        │
├──────────────────────────────────────────────────┤
│  Level 1: 基础设施连通性测试 (Smoke Test)          │  "组件能不能连通"
│  TEI 可用 / pgvector 可写 / LiteLLM 可调          │
└──────────────────────────────────────────────────┘
```

每一层验证的问题不同，失败的含义也不同：

| Level | 验证什么 | 失败含义 | 运行时机 |
|-------|---------|---------|---------|
| L1 Smoke | 组件健康 | 部署有问题 | 每次 helm install/upgrade |
| L2 E2E | 功能链路 | 集成有 bug | 每次 RAG 组件变更 |
| L3 Quality | 回答质量 | 模型/数据/prompt 需调优 | 每日定时 |
| L4 Regression | 质量趋势 | 这次变更让质量变差了 | 数据/模型/prompt 变更时 |

---

## 二、测试数据设计

### 2.1 Smoke Test 内置数据（Level 1）

3 条自包含文本，不依赖任何外部数据。覆盖基本场景：

```json
{
  "documents": [
    {
      "id": "smoke-001",
      "content": "kube-llmops is a Kubernetes-native LLMOps platform that provides model serving with vLLM, an AI gateway with LiteLLM, observability with Prometheus and Langfuse, and SSO with Keycloak. It supports 14 sub-charts deployed via a single Helm umbrella chart.",
      "metadata": {"source": "smoke-test", "language": "en"}
    },
    {
      "id": "smoke-002",
      "content": "Langfuse v3 requires ClickHouse for OLAP trace storage, Redis for async worker queues, and S3-compatible storage (MinIO) for event and media blob uploads. The ENCRYPTION_KEY environment variable must be a 64-character hex string.",
      "metadata": {"source": "smoke-test", "language": "en"}
    },
    {
      "id": "smoke-003",
      "content": "NVIDIA Grace Blackwell (GB10) uses unified memory architecture where CPU and GPU share the same memory pool. The drop-cache init container clears page cache before model loading to prevent CUDA from misreporting available memory.",
      "metadata": {"source": "smoke-test", "language": "en"}
    }
  ],
  "queries": [
    {
      "query": "What components does Langfuse v3 require?",
      "expected_doc_ids": ["smoke-002"],
      "expected_keywords": ["ClickHouse", "Redis", "S3"]
    },
    {
      "query": "How many sub-charts does kube-llmops have?",
      "expected_doc_ids": ["smoke-001"],
      "expected_keywords": ["14"]
    },
    {
      "query": "Why does GB10 need drop-cache?",
      "expected_doc_ids": ["smoke-003"],
      "expected_keywords": ["unified memory", "page cache"]
    }
  ]
}
```

**验收标准**：
- 3/3 query 的 top-1 检索结果命中 expected_doc_id
- 3/3 query 的 LLM 回答包含 expected_keywords

### 2.2 RAG 评估数据集（Level 3 — Ragas）

用于 Ragas 质量评估的数据集，需要构造更丰富的场景：

#### 数据集结构

```json
{
  "eval_samples": [
    {
      "question": "用户问题",
      "ground_truth": "标准答案（人工编写）",
      "contexts": ["检索到的上下文段落（由 RAG 系统生成，非预设）"],
      "answer": "LLM 的实际回答（由 RAG 系统生成，非预设）"
    }
  ]
}
```

Ragas 需要 `question` + `ground_truth` 是预先准备的，`contexts` 和 `answer` 是运行时由 RAG 系统生成的。

#### 评估数据分类

| 类别 | 样本数 | 目的 | 示例 |
|------|--------|------|------|
| **事实查询** | 10 | 精确信息检索 | "LiteLLM 的默认端口是多少？" → "4000" |
| **多跳推理** | 5 | 需要综合多段信息 | "Langfuse v3 相比 v2 多了哪些基础设施组件？" |
| **否定问题** | 5 | 知识库中不存在的信息 | "kube-llmops 支持 TensorRT-LLM 吗？" → "不支持/未提及" |
| **模糊查询** | 5 | 非精确措辞 | "怎么让 GPU 监控数据显示在面板上？" → DCGM + Grafana |
| **中文查询** | 5 | 多语言检索 | "如何配置单点登录？" → Keycloak SSO |
| **对抗样本** | 5 | 幻觉诱导 | "kube-llmops 的 React 前端框架用的是什么版本？" → 无前端 |
| **合计** | **35** | | |

#### 数据来源

评估数据基于我们自己的文档生成（吃自己的狗粮）：

```
数据源：
├── README.md / README.zh-CN.md
├── ARCHITECTURE.md
├── CHANGELOG.md
├── docs/getting-started.md
├── charts/*/values.yaml（关键配置项）
└── docs/rag/rag-direction.md
```

先人工编写 15 条核心样本（事实 + 否定 + 对抗），再用 **Ragas TestsetGenerator** 自动扩展到 35 条。

#### Ragas 评估指标及阈值

| 指标 | 含义 | P1 达标线 | P2 生产线 | P3 企业线 |
|------|------|----------|----------|----------|
| **Faithfulness** | 回答是否忠于上下文 | ≥ 0.7 | ≥ 0.85 | ≥ 0.95 |
| **Answer Relevancy** | 回答是否相关 | ≥ 0.7 | ≥ 0.8 | ≥ 0.9 |
| **Context Precision** | 相关文档是否排在前面 | ≥ 0.6 | ≥ 0.75 | ≥ 0.85 |
| **Context Recall** | 是否检索到所有相关信息 | ≥ 0.5 | ≥ 0.7 | ≥ 0.85 |
| **Hallucination Rate** | 回答中无依据的声称比例 | ≤ 0.3 | ≤ 0.15 | ≤ 0.05 |

### 2.3 回归测试数据（Level 4）

回归测试使用与 Level 3 **完全相同的数据集**，关键是对比**前后两次评估的分数变化**：

```
回归判定规则：
  IF new_score < old_score - threshold:
    BLOCK deployment
    ALERT "RAG quality regression detected"

默认阈值:
  faithfulness_drop_threshold: 0.05    # 下降超过 5% 告警
  relevancy_drop_threshold: 0.05
  precision_drop_threshold: 0.05
```

---

## 三、Phase 验收标准

### Phase 1 验收：RAG 能跑通

| # | 验收项 | 验证方法 | 通过标准 |
|---|--------|---------|---------|
| 1.1 | TEI embedding 可用 | `curl /v1/embeddings` | 返回正确维度向量（bge-m3: 1024） |
| 1.2 | LiteLLM 路由 embedding | 通过 LiteLLM 调 TEI | 同上，经过 LiteLLM 代理 |
| 1.3 | pgvector 写入 | INSERT embedding | 写入成功，SELECT 可查 |
| 1.4 | Dense 检索 | cosine similarity search | top-1 命中预期文档 |
| 1.5 | Dify 端到端 | 上传 PDF → 提问 → 回答 | 回答包含文档中的信息 |
| 1.6 | Langfuse trace | 查看 trace | LLM generation span 存在 |
| 1.7 | Smoke Test 通过 | rag-smoke-test Job | 所有 step PASS |

**自动化**：Smoke Test Job 覆盖 1.1-1.4, 1.6。1.5 需人工验证一次。

### Phase 2 验收：RAG 质量可衡量

| # | 验收项 | 验证方法 | 通过标准 |
|---|--------|---------|---------|
| 2.1 | Reranking 可用 | curl /rerank | 返回重排序结果 |
| 2.2 | Hybrid 检索 | SQL hybrid_search | dense + sparse 分数都有 |
| 2.3 | Ragas eval 能跑 | CronJob 执行成功 | 5 个指标全有数值 |
| 2.4 | Ragas 达标 | eval 结果 | Faithfulness ≥ 0.7, Relevancy ≥ 0.7 |
| 2.5 | Grafana dashboard | 打开 RAG Quality dashboard | 6 个 panel 全有数据 |
| 2.6 | Prometheus 告警 | 触发低质量场景 | 告警规则触发 |
| 2.7 | RAG trace spans | Langfuse 查看 | embed → retrieve → rerank → generate 4 段 span |

**自动化**：2.1-2.3 由扩展的 Smoke Test 覆盖。2.4 由 Ragas CronJob 持续监控。

### Phase 3 验收：RAG 可上生产

| # | 验收项 | 验证方法 | 通过标准 |
|---|--------|---------|---------|
| 3.1 | LLM-Guard 拦截 | 发送 injection payload | 返回 4xx + blocked 指标 +1 |
| 3.2 | PII 检测 | 发送含身份证号的请求 | 输出中 PII 被标记/脱敏 |
| 3.3 | 质量门控 | 降低 eval 阈值 → helm upgrade | upgrade 被阻断 |
| 3.4 | 质量门控放行 | 恢复正常阈值 → helm upgrade | upgrade 成功 |
| 3.5 | Ragas 生产达标 | eval 结果 | Faithfulness ≥ 0.85, Hallucination ≤ 0.15 |
| 3.6 | 回归检测 | 换差模型 → 跑 eval | 检测到回归，告警触发 |

**自动化**：3.1-3.2 由 Smoke Test guardrails step 覆盖。3.3-3.4 由 CI pipeline 覆盖。

### Phase 4 验收：企业级能力

| # | 验收项 | 验证方法 | 通过标准 |
|---|--------|---------|---------|
| 4.1 | LightRAG 知识图谱 | 图检索查询 | 返回实体关系路径 |
| 4.2 | 多租户隔离 | 用户 A 查不到用户 B 的数据 | metadata filter 生效 |
| 4.3 | Milvus 大规模 | 100K+ 文档检索 | P95 延迟 < 200ms |

---

## 四、测试执行架构

```
┌─────────────────────────────────────────────────────┐
│  GitHub Actions CI                                    │
│  ┌─────────────┐ ┌──────────────┐ ┌──────────────┐  │
│  │ Lint + Build │ │ Helm Template│ │ kind e2e     │  │
│  │ (已有 ✅)    │ │ (已有 ✅)    │ │ (已有 ✅)    │  │
│  └─────────────┘ └──────────────┘ └──────────────┘  │
│  ┌──────────────────────────────────────────────┐    │
│  │ NEW: RAG Integration Test                     │    │
│  │ kind cluster + TEI + pgvector + LiteLLM       │    │
│  │ → Smoke Test Job → assert all PASS            │    │
│  └──────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────┐
│  部署后自动运行 (K8s CronJob)                         │
│  ┌─────────────┐ ┌──────────────┐                    │
│  │ Smoke Test   │ │ Ragas Eval   │                    │
│  │ (每日 6:00)  │ │ (每日 7:00)  │                    │
│  │ → Prometheus │ │ → Prometheus │                    │
│  └──────┬──────┘ └──────┬───────┘                    │
│         └───────┬───────┘                            │
│                 ▼                                     │
│  ┌──────────────────────────────────────────────┐    │
│  │ Grafana: RAG Health + Quality Dashboard       │    │
│  │ Alert: smoke_test_failed / quality_regression │    │
│  └──────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────┐
│  Helm Upgrade Quality Gate                            │
│  pre-upgrade hook → Ragas eval → pass? → upgrade      │
│                                   fail? → abort       │
└─────────────────────────────────────────────────────┘
```

---

## 五、Ragas 集成技术细节

### 5.1 Eval CronJob 架构

```yaml
# charts/kube-llmops-stack/charts/rag-eval/templates/cronjob.yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: {{ .Release.Name }}-rag-eval
spec:
  schedule: "0 7 * * *"  # 每日 07:00
  jobTemplate:
    spec:
      template:
        spec:
          containers:
            - name: eval
              image: python:3.11-slim
              command: ["python", "/app/eval.py"]
              env:
                - name: LITELLM_URL
                  value: "http://{{ .Release.Name }}-litellm:4000"
                - name: LITELLM_API_KEY
                  value: {{ .Values.litellm.masterKey }}
                - name: LANGFUSE_HOST
                  value: "http://{{ .Release.Name }}-langfuse:3000"
                - name: PG_URL
                  value: "postgresql://..."
                - name: EVAL_MODEL
                  value: {{ .Values.rag.eval.model | default "the-deployed-model" }}
                - name: PROMETHEUS_PUSHGATEWAY
                  value: "http://{{ .Release.Name }}-prometheus:9090"
              volumeMounts:
                - name: eval-data
                  mountPath: /app/data
                - name: eval-script
                  mountPath: /app
          volumes:
            - name: eval-data
              configMap:
                name: {{ .Release.Name }}-rag-eval-dataset
            - name: eval-script
              configMap:
                name: {{ .Release.Name }}-rag-eval-script
```

### 5.2 Eval 脚本核心逻辑

```python
# eval.py (内联在 ConfigMap，不依赖自建镜像)
# pip install ragas langfuse openai prometheus_client

from ragas import evaluate
from ragas.metrics import faithfulness, answer_relevancy, context_precision, context_recall
from datasets import Dataset

# 1. 加载 eval 数据集 (question + ground_truth)
eval_data = load_json("/app/data/eval-dataset.json")

# 2. 对每个 question，调用 RAG pipeline 获取 contexts + answer
for sample in eval_data:
    # 2a. Embedding + 检索
    contexts = rag_retrieve(sample["question"])
    # 2b. LLM 生成
    answer = rag_generate(sample["question"], contexts)
    sample["contexts"] = contexts
    sample["answer"] = answer

# 3. 构造 Ragas Dataset
dataset = Dataset.from_dict({
    "question": [s["question"] for s in eval_data],
    "answer": [s["answer"] for s in eval_data],
    "contexts": [s["contexts"] for s in eval_data],
    "ground_truth": [s["ground_truth"] for s in eval_data],
})

# 4. 运行 Ragas 评估 (使用已部署的 LLM 作为 judge)
result = evaluate(
    dataset,
    metrics=[faithfulness, answer_relevancy, context_precision, context_recall],
    llm=litellm_wrapper,  # 复用 LiteLLM，不引入外部 API
)

# 5. 推送 Prometheus 指标
push_metrics({
    "rag_faithfulness": result["faithfulness"],
    "rag_answer_relevancy": result["answer_relevancy"],
    "rag_context_precision": result["context_precision"],
    "rag_context_recall": result["context_recall"],
})

# 6. 记录到 Langfuse
log_to_langfuse(result)
```

### 5.3 Eval LLM 选择策略

Ragas 内部用 LLM-as-judge 评分。这个 judge LLM 怎么选：

| 策略 | 方案 | 优点 | 缺点 |
|------|------|------|------|
| **复用已部署模型** | 用 LiteLLM 路由到 vLLM | 零额外成本 | 小模型 judge 质量差 |
| **外部强模型** | 配 OpenAI GPT-4 API key | judge 质量最高 | 需要外部 API + 费用 |
| **专用 judge 模型** | 部署 Prometheus-2 (7B) | 专为评估训练 | 额外 GPU 占用 |

**建议**：
- 开发/测试：复用已部署模型（零成本）
- 生产：外部 GPT-4 或 Claude（通过 LiteLLM 路由，只需加一个 API key）
- 评估结果标注 judge 模型版本（可追溯）

---

## 六、测试数据管理

### 存储位置

```
charts/kube-llmops-stack/charts/rag-eval/
├── templates/
│   ├── cronjob.yaml          # Ragas 定时评估
│   ├── job-smoke-test.yaml   # Smoke Test
│   ├── configmap-dataset.yaml # 评估数据集
│   └── configmap-script.yaml  # 评估脚本
├── data/
│   ├── smoke-test.json       # 3 条内置数据 (Level 1)
│   └── eval-dataset.json     # 35 条评估数据 (Level 3)
└── values.yaml
```

### 数据版本管理

评估数据集随 Helm chart 版本管理：
- 修改数据集 → PR → CI 自动跑 eval → 对比前后分数
- 数据集变更记录在 CHANGELOG

### 数据集扩展方式

```
Phase 1: 人工编写 15 条（事实 10 + 否定 5）
Phase 2: Ragas TestsetGenerator 扩展到 35 条
Phase 3: 收集线上真实 query（Langfuse 导出）→ 扩展到 100+
Phase 4: 按租户/领域分数据集
```
