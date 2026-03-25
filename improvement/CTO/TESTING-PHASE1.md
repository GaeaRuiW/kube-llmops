# Phase 1 Story 验收标准与测试方法

> 本文档为 Phase 1 每个 Story 的验收标准提供**可执行的测试方法**。
> 每个测试项均包含: 测试命令、预期输出、失败判定。
> **约定**: `RELEASE=kube-llmops`, `NS=default` (或实际部署的 namespace)

---

## Story 1.1: 全组件 PodDisruptionBudget

### AC-1.1.1: 每个子 Chart 包含 PDB 模板

**测试方法:**
```bash
# 扫描所有子 Chart 模板,统计 PDB 资源数量
helm template kube-llmops charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  | grep "kind: PodDisruptionBudget" | wc -l
```
**预期输出:** `>= 15` (至少覆盖 15 个核心 Deployment/StatefulSet)
**失败判定:** 输出为 0 或明显少于组件数

### AC-1.1.2: single-node profile 渲染出 PDB

**测试方法:**
```bash
helm template kube-llmops charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  | grep -A10 "kind: PodDisruptionBudget" \
  | grep -E "name:|maxUnavailable:|minAvailable:"
```
**预期输出:** 每个 PDB 有 name 和 maxUnavailable/minAvailable 字段
**失败判定:** PDB 模板渲染报错或缺少必要字段

### AC-1.1.3: production profile 多副本组件 PDB 为 minAvailable: 1

**测试方法:**
```bash
helm template kube-llmops charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-production.yaml \
  | python3 -c "
import sys, yaml
docs = yaml.safe_load_all(sys.stdin)
for doc in docs:
    if doc and doc.get('kind') == 'PodDisruptionBudget':
        name = doc['metadata']['name']
        spec = doc['spec']
        if 'minAvailable' in spec:
            print(f'PASS: {name} minAvailable={spec[\"minAvailable\"]}')
        elif 'maxUnavailable' in spec:
            print(f'PASS: {name} maxUnavailable={spec[\"maxUnavailable\"]}')
        else:
            print(f'FAIL: {name} no disruption policy')
"
```
**预期输出:** litellm, langfuse 等多副本组件显示 `minAvailable=1`
**失败判定:** 任何组件输出 `FAIL`

### AC-1.1.4: 部署后 PDB 实际生效

**测试方法:**
```bash
# 部署后检查
kubectl get pdb -o custom-columns=NAME:.metadata.name,MIN-AVAILABLE:.spec.minAvailable,MAX-UNAVAILABLE:.spec.maxUnavailable,ALLOWED-DISRUPTIONS:.status.disruptionsAllowed

# 验证关键组件有 PDB
for svc in litellm postgresql grafana prometheus keycloak minio; do
  COUNT=$(kubectl get pdb -o name 2>/dev/null | grep -c "$svc" || true)
  if [ "$COUNT" -ge 1 ]; then
    echo "PASS: PDB exists for $svc"
  else
    echo "FAIL: PDB missing for $svc"
  fi
done
```
**预期输出:** 所有关键组件显示 `PASS`
**失败判定:** 任何关键组件显示 `FAIL`

### AC-1.1.5: kubectl drain 不会同时驱逐所有副本

**测试方法:**
```bash
# 干跑测试 (不实际执行驱逐)
NODE=$(kubectl get nodes -o jsonpath='{.items[0].metadata.name}')
kubectl drain $NODE --dry-run=client --ignore-daemonsets --delete-emptydir-data 2>&1

# 检查输出中是否有 "Cannot evict pod" 错误 (单副本 PDB minAvailable=1 会阻断)
# 对于单副本组件使用 maxUnavailable=1 则不会阻断
```
**预期输出:** 干跑完成,无 `error` 关键字 (因为单副本使用 `maxUnavailable: 1`)
**失败判定:** 输出 `Cannot evict pod` 且非预期行为

---

## Story 1.2: 凭据随机化 + existingSecret

### AC-1.2.1: 全新安装时密码为随机值

**测试方法:**
```bash
# 全新安装
helm install test-release charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml --dry-run=false

# 获取 LiteLLM master key
MASTER_KEY=$(kubectl get secret test-release-litellm-secret \
  -o jsonpath='{.data.master-key}' | base64 -d)

# 验证不是硬编码值
echo "$MASTER_KEY"
if [ "$MASTER_KEY" = "sk-kube-llmops-dev" ] || [ "$MASTER_KEY" = "sk-kube-llmops-default" ]; then
  echo "FAIL: master key is still hardcoded default"
else
  echo "PASS: master key is randomized (length=${#MASTER_KEY})"
fi

# 验证所有密码都不是默认值
for secret_name in litellm-secret grafana-secret keycloak-secret; do
  FULL_NAME="test-release-${secret_name}"
  if kubectl get secret "$FULL_NAME" &>/dev/null; then
    echo "PASS: Secret $FULL_NAME exists"
  else
    echo "FAIL: Secret $FULL_NAME not found"
  fi
done
```
**预期输出:** 所有密码为随机字符串,长度 >= 24,均不等于已知默认值
**失败判定:** 任何密码等于 `sk-kube-llmops-dev`, `admin123!`, `minioadmin`, `llmops-pg-dev-pw` 等已知默认值

### AC-1.2.2: helm upgrade 不覆盖已有 Secret

**测试方法:**
```bash
# 记录当前密码
KEY_BEFORE=$(kubectl get secret $RELEASE-litellm-secret \
  -o jsonpath='{.data.master-key}')

# 执行 upgrade
helm upgrade $RELEASE charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml

# 比较密码
KEY_AFTER=$(kubectl get secret $RELEASE-litellm-secret \
  -o jsonpath='{.data.master-key}')

if [ "$KEY_BEFORE" = "$KEY_AFTER" ]; then
  echo "PASS: Secret preserved after upgrade"
else
  echo "FAIL: Secret changed after upgrade (before=$KEY_BEFORE, after=$KEY_AFTER)"
fi
```
**预期输出:** `PASS: Secret preserved after upgrade`
**失败判定:** 密码值在 upgrade 后发生变化

### AC-1.2.3: 两次 helm template 输出的密码不同

**测试方法:**
```bash
KEY1=$(helm template t1 charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  | grep -A2 "master-key:" | tail -1 | tr -d ' ')
KEY2=$(helm template t2 charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  | grep -A2 "master-key:" | tail -1 | tr -d ' ')

if [ "$KEY1" != "$KEY2" ]; then
  echo "PASS: Random passwords differ between renders"
else
  echo "FAIL: Passwords are identical — randomization not working"
fi
```
**预期输出:** `PASS`
**失败判定:** 两次渲染密码相同

### AC-1.2.4: existingSecret 功能验证

**测试方法:**
```bash
# 预创建 Secret
kubectl create secret generic my-litellm-secret \
  --from-literal=master-key=sk-my-custom-key-12345 \
  --from-literal=postgresql-password=my-pg-pass

# 使用 existingSecret 部署
helm template test charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  --set litellm.existingSecret=my-litellm-secret \
  | grep -c "kind: Secret" | head -1
# 预期: Secret 数量应少于不设 existingSecret 时的数量

# 检查 Deployment 引用了用户 Secret
helm template test charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  --set litellm.existingSecret=my-litellm-secret \
  | grep "my-litellm-secret"
```
**预期输出:** Deployment 中引用 `my-litellm-secret`,不再创建内置 litellm Secret
**失败判定:** 仍然创建内置 Secret,或 Deployment 未引用用户 Secret

### AC-1.2.5: NOTES.txt 包含安全提醒

**测试方法:**
```bash
helm template test charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  --show-only templates/NOTES.txt \
  | grep -i "security\|credential\|secret\|password"
```
**预期输出:** 包含 "Security Notice" 或类似段落,含 `kubectl get secret` 命令
**失败判定:** 输出为空

### AC-1.2.6: 模板中无明文密码

**测试方法:**
```bash
# 检查渲染结果中是否有已知默认密码
helm template test charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  | grep -E "sk-kube-llmops|admin123|minioadmin|llmops-pg-dev|langfuse-default|dify-default|llm-guard-kube"

# 检查 Deployment 的 env 中是否有直接 value (非 secretKeyRef)
helm template test charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  | python3 -c "
import sys, yaml
for doc in yaml.safe_load_all(sys.stdin):
    if not doc or doc.get('kind') != 'Deployment': continue
    for c in doc.get('spec',{}).get('template',{}).get('spec',{}).get('containers',[]):
        for env in c.get('env',[]):
            name = env.get('name','')
            if any(kw in name.upper() for kw in ['KEY','PASSWORD','SECRET','TOKEN']):
                if 'value' in env and 'valueFrom' not in env:
                    print(f'FAIL: {doc[\"metadata\"][\"name\"]} has plaintext {name}')
"
```
**预期输出:** 第一条 grep 无输出;第二条无 `FAIL` 行
**失败判定:** 发现硬编码密码或 plaintext 环境变量

---

## Story 1.3: Prometheus K8s 服务发现

### AC-1.3.1: Prometheus ConfigMap 无硬编码服务名

**测试方法:**
```bash
helm template test charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  --show-only charts/observability/templates/prometheus.yaml \
  | grep "static_configs" -A2 \
  | grep -v "localhost\|Release.Name\|$." \
  | grep -E "vllm-|tei-"
```
**预期输出:** 无输出 (所有 vllm/tei target 已改为 SD,不再有 static_configs 中的硬编码名)
**失败判定:** 输出中包含 `vllm-qwen` 或其他硬编码模型服务名

### AC-1.3.2: Prometheus Targets 页面显示 SD 发现的端点

**测试方法:**
```bash
# 端口转发
kubectl port-forward svc/$RELEASE-prometheus 9090:9090 &
sleep 3

# 查询 targets
curl -s http://localhost:9090/api/v1/targets \
  | python3 -c "
import sys, json
data = json.load(sys.stdin)
for target in data['data']['activeTargets']:
    job = target.get('labels',{}).get('job','')
    health = target.get('health','')
    addr = target.get('discoveredLabels',{}).get('__address__','')
    discovery = target.get('discoveredLabels',{}).get('__meta_kubernetes_pod_name','N/A')
    if 'vllm' in job or 'tei' in job:
        sd_type = 'kubernetes-sd' if discovery != 'N/A' else 'static'
        print(f'{job}: {addr} health={health} discovery={sd_type}')
"

kill %1 2>/dev/null
```
**预期输出:** vllm/tei job 显示 `discovery=kubernetes-sd`, `health=up`
**失败判定:** discovery 为 `static`,或 health 为 `down`

### AC-1.3.3: 多模型自动发现

**测试方法:**
```bash
# 在 values 中添加第二个模型后 helm upgrade
helm upgrade $RELEASE charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  --set "vllm.models[1].name=test-model" \
  --set "vllm.models[1].modelId=Qwen/Qwen2.5-1.5B" \
  --set "vllm.models[1].gpu=0"

# 等待 Prometheus 刷新 target (默认 30s 周期)
sleep 45

# 检查 Prometheus targets 中是否有新模型
curl -s http://localhost:9090/api/v1/targets \
  | grep -o '"model":"[^"]*"' | sort -u
```
**预期输出:** 包含两个不同的 model 标签
**失败判定:** 只有一个模型,或新模型 target 缺失

### AC-1.3.4: Grafana Dashboard 指标正常

**测试方法:**
```bash
# 发送一个推理请求触发指标
curl -s http://localhost:4000/v1/chat/completions \
  -H "Authorization: Bearer $(kubectl get secret $RELEASE-litellm-secret -o jsonpath='{.data.master-key}' | base64 -d)" \
  -H "Content-Type: application/json" \
  -d '{"model":"qwen2-5-0-5b","messages":[{"role":"user","content":"Hi"}],"max_tokens":5}'

sleep 10

# 查询关键指标
for metric in "vllm:num_requests_running" "vllm:request_success_total" "vllm:avg_generation_throughput_toks_per_s"; do
  RESULT=$(curl -s "http://localhost:9090/api/v1/query?query=$metric" \
    | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('data',{}).get('result',[])))")
  if [ "$RESULT" -ge 1 ]; then
    echo "PASS: $metric has $RESULT series"
  else
    echo "FAIL: $metric has 0 series"
  fi
done
```
**预期输出:** 所有指标显示 `PASS`
**失败判定:** 任何指标显示 `FAIL`

### AC-1.3.5: Prometheus RBAC 无报错

**测试方法:**
```bash
kubectl logs -l app.kubernetes.io/name=prometheus --tail=50 \
  | grep -iE "forbidden|unauthorized|rbac|permission" | head -5
```
**预期输出:** 无输出 (无 RBAC 错误)
**失败判定:** 出现 `forbidden` 或 `unauthorized` 日志

---

## Story 1.4: 文档一致性修复

### AC-1.4.1: ARCHITECTURE.md 每个功能有状态标签

**测试方法:**
```bash
# 统计状态标签数量
grep -cE "\[IMPLEMENTED\]|\[BETA\]|\[TEMPLATE-ONLY\]|\[PLANNED" ARCHITECTURE.md
```
**预期输出:** `>= 20` (至少 20 个功能/组件有状态标签)
**失败判定:** 数量明显少于文档中描述的组件数

### AC-1.4.2: README Features 表中 Partial 标注

**测试方法:**
```bash
grep -A20 "Feature.*kube-llmops.*Raw vLLM" README.md \
  | grep -iE "auto-selection|keda" \
  | grep -i "partial"
```
**预期输出:** 两行均包含 `Partial`
**失败判定:** 仍然标为 `Yes`

### AC-1.4.3: PLAN.md 中不存在虚假的 [x]

**测试方法:**
```bash
# 检查 Harbor, Fluid, KEDA 相关的 checklist 是否已改为 [ ] 或 [PARTIAL]
grep -n "\[x\].*\(Harbor\|Fluid\|auto-select\)" PLAN.md
```
**预期输出:** 无输出 (这些项不再标为 `[x]`)
**失败判定:** 仍有 `[x]` 标注

### AC-1.4.4: 根目录 RAG-*.md 已迁移

**测试方法:**
```bash
# 检查根目录是否还有 RAG-*.md
ls -1 RAG-*.md 2>/dev/null | wc -l

# 检查 docs/rag/ 是否存在
ls docs/rag/*.md 2>/dev/null | wc -l
```
**预期输出:** 根目录 0 个 RAG-*.md;`docs/rag/` >= 2 个文件
**失败判定:** 根目录仍有 RAG-*.md

### AC-1.4.5: 根目录文件数收敛

**测试方法:**
```bash
ls -1 *.md | wc -l
ls -1 *.md
```
**预期输出:** <= 12 个 .md 文件 (README x2, ARCHITECTURE x2, CHANGELOG x2, CONTRIBUTING x2, AGENTS, PLAN, DEPLOY-GB10 x2)
**失败判定:** > 15 个 (没有收敛)

---

## Story 1.5: NOTES.txt 部署后引导

### AC-1.5.1: helm install 输出包含完整引导信息

**测试方法:**
```bash
helm template test charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  --show-only templates/NOTES.txt > /tmp/notes.txt

# 检查关键段落
for keyword in "DEPLOYMENT STATUS" "ACCESS" "SECURITY" "VERIFICATION" "NEXT STEPS"; do
  if grep -qi "$keyword" /tmp/notes.txt; then
    echo "PASS: Contains section '$keyword'"
  else
    echo "FAIL: Missing section '$keyword'"
  fi
done

# 检查包含所有 UI 地址
for svc in litellm grafana langfuse dify keycloak; do
  if grep -qi "$svc" /tmp/notes.txt; then
    echo "PASS: Contains $svc access info"
  else
    echo "FAIL: Missing $svc access info"
  fi
done
```
**预期输出:** 所有检查显示 `PASS`
**失败判定:** 任何关键段落或服务地址缺失

### AC-1.5.2: helm status 可重新查看

**测试方法:**
```bash
helm status $RELEASE 2>/dev/null | grep -c "NOTES:"
```
**预期输出:** `1` (包含 NOTES 段落)
**失败判定:** 无 NOTES 输出

### AC-1.5.3: CI profile 不含 GPU 信息

**测试方法:**
```bash
helm template test charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-ci.yaml \
  --show-only templates/NOTES.txt \
  | grep -i "gpu\|vllm\|nvidia"
```
**预期输出:** 无输出 (CI 无 GPU 组件)
**失败判定:** 包含 GPU/vLLM 相关内容

---

## Story 1.6: CI chart-install-test 强制通过

### AC-1.6.1: continue-on-error 已移除

**测试方法:**
```bash
grep "continue-on-error" .github/workflows/test.yaml
```
**预期输出:** 无输出
**失败判定:** 仍然包含 `continue-on-error: true`

### AC-1.6.2: CI 安装失败时 workflow 变红

**测试方法 (手动验证):**
1. 创建一个故意会导致安装失败的 PR (如模板语法错误)
2. 推送并等待 CI 运行
3. 观察 GitHub Actions 中 `chart-install-test` job 状态

**预期结果:** job 显示红色 X,整个 workflow 失败
**失败判定:** job 显示绿色勾,workflow 通过

### AC-1.6.3: 正常情况下 CI 稳定通过

**测试方法 (手动验证):**
1. 在 main 分支或 PR 上触发 3 次 CI 运行
2. 检查 chart-install-test job 是否全部成功

**预期结果:** 3/3 通过
**失败判定:** 任何一次失败 (flaky test 需修复)

---

## Story 1.7: Milvus etcd 资源配置

### AC-1.7.1: etcd QoS 提升至 Burstable

**测试方法:**
```bash
kubectl get pod -l app.kubernetes.io/component=milvus-etcd \
  -o jsonpath='{range .items[*]}{.metadata.name}: QoS={.status.qosClass}{"\n"}{end}'
```
**预期输出:** `QoS=Burstable` (非 BestEffort)
**失败判定:** `QoS=BestEffort`

### AC-1.7.2: helm template 渲染包含 resources

**测试方法:**
```bash
helm template test charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  | python3 -c "
import sys, yaml
for doc in yaml.safe_load_all(sys.stdin):
    if not doc: continue
    kind = doc.get('kind','')
    name = doc.get('metadata',{}).get('name','')
    if 'etcd' in name.lower() and kind in ('Deployment','StatefulSet'):
        containers = doc['spec']['template']['spec']['containers']
        for c in containers:
            res = c.get('resources',{})
            req = res.get('requests',{})
            if req.get('cpu') and req.get('memory'):
                print(f'PASS: {name}/{c[\"name\"]} requests: cpu={req[\"cpu\"]}, memory={req[\"memory\"]}')
            else:
                print(f'FAIL: {name}/{c[\"name\"]} missing resource requests')
"
```
**预期输出:** `PASS` with cpu and memory requests
**失败判定:** `FAIL` or missing requests

---

## Story 1.8: LightRAG 健康检查探针

### AC-1.8.1: LightRAG Deployment 包含探针

**测试方法:**
```bash
helm template test charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  | python3 -c "
import sys, yaml
for doc in yaml.safe_load_all(sys.stdin):
    if not doc or doc.get('kind') != 'Deployment': continue
    name = doc['metadata']['name']
    if 'lightrag' not in name.lower() or 'neo4j' in name.lower(): continue
    containers = doc['spec']['template']['spec']['containers']
    for c in containers:
        rp = c.get('readinessProbe')
        lp = c.get('livenessProbe')
        print(f'{name}/{c[\"name\"]}:')
        print(f'  readinessProbe: {\"PASS\" if rp else \"FAIL - missing\"}')
        print(f'  livenessProbe:  {\"PASS\" if lp else \"FAIL - missing\"}')
"
```
**预期输出:** LightRAG Deployment 的 readinessProbe 和 livenessProbe 均为 `PASS`
**失败判定:** 任何探针显示 `FAIL`

### AC-1.8.2: LightRAG Pod 初始化期间未标记 Ready

**测试方法:**
```bash
# 删除 LightRAG Pod 触发重建
kubectl delete pod -l app.kubernetes.io/name=lightrag

# 立即观察 Pod 状态 (应为 0/1 Running,非 1/1)
for i in $(seq 1 5); do
  sleep 2
  STATUS=$(kubectl get pod -l app.kubernetes.io/name=lightrag \
    -o jsonpath='{.items[0].status.containerStatuses[0].ready}' 2>/dev/null)
  echo "t+${i}x2s: ready=$STATUS"
done

# 等待完全就绪
kubectl wait --for=condition=Ready pod -l app.kubernetes.io/name=lightrag --timeout=120s
echo "PASS: LightRAG became ready after initialization"
```
**预期输出:** 前几次检查 `ready=false`,最终变为 `ready=true`
**失败判定:** Pod 立即 `ready=true` (说明探针未配置或 initialDelaySeconds 过短)

### AC-1.8.3: Neo4j 同时具备 readiness 和 liveness 探针

**测试方法:**
```bash
kubectl get pod -l app.kubernetes.io/name=neo4j \
  -o jsonpath='{range .items[*].spec.containers[*]}{.name}: readiness={.readinessProbe.httpGet.path}, liveness={.livenessProbe.httpGet.path}{"\n"}{end}'
```
**预期输出:** `readiness=/, liveness=/` (两个探针都存在)
**失败判定:** liveness 为空
