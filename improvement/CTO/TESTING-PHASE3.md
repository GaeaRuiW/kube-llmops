# Phase 3 Story 验收标准与测试方法

> **约定**: `RELEASE=kube-llmops`, `NS=default`
> **前置条件**: Phase 1 + Phase 2 全部完成

---

## Story 3.1: 有状态组件 HA 加固

### AC-3.1.1: PostgreSQL 主从副本就绪

**测试方法:**
```bash
# production profile 部署后检查 PG Pod 数量
kubectl get pods -l app.kubernetes.io/component=postgresql \
  -o custom-columns=NAME:.metadata.name,ROLE:.metadata.labels.role,READY:.status.containerStatuses[0].ready

# 预期: 1 primary + 1 replica
PRIMARY_COUNT=$(kubectl get pods -l role=primary,app.kubernetes.io/component=postgresql --no-headers | wc -l)
REPLICA_COUNT=$(kubectl get pods -l role=replica,app.kubernetes.io/component=postgresql --no-headers | wc -l)
echo "Primary: $PRIMARY_COUNT, Replica: $REPLICA_COUNT"
[ "$PRIMARY_COUNT" -ge 1 ] && [ "$REPLICA_COUNT" -ge 1 ] && echo "PASS" || echo "FAIL"
```
**预期输出:** `Primary: 1, Replica: 1` + `PASS`
**失败判定:** Replica 为 0

### AC-3.1.2: Primary Pod 删除后服务自动恢复

**测试方法:**
```bash
# 记录当前 primary
OLD_PRIMARY=$(kubectl get pod -l role=primary,app.kubernetes.io/component=postgresql -o name | head -1)
echo "Killing primary: $OLD_PRIMARY"

# 删除 primary
kubectl delete $OLD_PRIMARY --grace-period=5
START=$(date +%s)

# 等待 LiteLLM 恢复可用
for i in $(seq 1 30); do
  sleep 5
  CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:4000/health/liveliness 2>/dev/null || echo "000")
  if [ "$CODE" = "200" ]; then
    END=$(date +%s)
    echo "PASS: Service recovered in $((END-START))s"
    break
  fi
  echo "Waiting... ($i/30) HTTP=$CODE"
done
```
**预期输出:** 60 秒内恢复,输出 `PASS`
**失败判定:** 150 秒内未恢复

### AC-3.1.3: VictoriaMetrics 远端存储写入正常

**测试方法:**
```bash
# 检查 Prometheus remote_write 状态
kubectl port-forward svc/$RELEASE-prometheus 9090:9090 &
sleep 2
curl -s http://localhost:9090/api/v1/status/runtimeinfo \
  | python3 -c "
import sys, json
info = json.load(sys.stdin)
storage = info.get('data',{}).get('storageRetention','')
print(f'Storage retention: {storage}')
"

# 查询 VictoriaMetrics 中的历史数据
kubectl port-forward svc/$RELEASE-victoriametrics 8428:8428 &
sleep 2
curl -s "http://localhost:8428/api/v1/query?query=up" \
  | python3 -c "
import sys, json
d = json.load(sys.stdin)
series = len(d.get('data',{}).get('result',[]))
print(f'Series in VictoriaMetrics: {series}')
assert series > 0, 'No data in VictoriaMetrics'
print('PASS')
"
kill %1 %2 2>/dev/null
```
**预期输出:** VictoriaMetrics 中有数据,输出 `PASS`
**失败判定:** 0 条 series

### AC-3.1.4: 外部数据库文档可用

**测试方法:**
```bash
test -f docs/guides/external-databases.md && echo "PASS" || echo "FAIL"
grep -c "RDS\|Cloud SQL\|Azure" docs/guides/external-databases.md
```
**预期输出:** 文件存在 + 至少 3 处云数据库提及
**失败判定:** 文件不存在

---

## Story 3.2: 全栈可观测性深耕

### AC-3.2.1: Grafana Dashboard 总数 >= 8

**测试方法:**
```bash
kubectl port-forward svc/$RELEASE-grafana 3000:3000 &
sleep 2
ADMIN_PW=$(kubectl get secret $RELEASE-grafana-secret -o jsonpath='{.data.admin-password}' | base64 -d)
DASHBOARD_COUNT=$(curl -s "http://admin:${ADMIN_PW}@localhost:3000/api/search?type=dash-db" \
  | python3 -c "import sys,json; print(len(json.load(sys.stdin)))")
echo "Dashboard count: $DASHBOARD_COUNT"
[ "$DASHBOARD_COUNT" -ge 8 ] && echo "PASS" || echo "FAIL"
kill %1 2>/dev/null
```
**预期输出:** `>= 8` + `PASS`
**失败判定:** `< 8`

### AC-3.2.2: Infrastructure ROI Dashboard 数据可见

**测试方法:**
```bash
# 发送几个推理请求
for i in $(seq 1 5); do
  curl -s http://localhost:4000/v1/chat/completions \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d '{"model":"qwen2-5-0-5b","messages":[{"role":"user","content":"test '$i'"}],"max_tokens":10}' > /dev/null
done
sleep 15

# 查询 ROI 相关指标
for metric in "vllm:avg_generation_throughput_toks_per_s" "litellm_total_tokens"; do
  VAL=$(curl -s "http://localhost:9090/api/v1/query?query=$metric" \
    | python3 -c "import sys,json; r=json.load(sys.stdin)['data']['result']; print(len(r))")
  echo "$metric: $VAL series"
done
```
**预期输出:** 每个指标至少 1 个 series
**失败判定:** 0 series

### AC-3.2.3: SLO 文档存在

**测试方法:**
```bash
test -f docs/guides/slo-guide.md && echo "PASS" || echo "FAIL"
grep -cE "99\\.9|TTFT|Error Budget|Burn Rate" docs/guides/slo-guide.md
```
**预期输出:** 文件存在 + 至少 4 处 SLO 关键词
**失败判定:** 文件不存在

---

## Story 3.3: External Secrets Operator

### AC-3.3.1: ESO 启用后渲染 ExternalSecret 资源

**测试方法:**
```bash
helm template test charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  --set externalSecrets.enabled=true \
  --set externalSecrets.secretStore=my-vault \
  | grep "kind: ExternalSecret" | wc -l
```
**预期输出:** `>= 5` (每个需要密钥的子 Chart 至少 1 个)
**失败判定:** 0

### AC-3.3.2: ESO 禁用时不生成 ExternalSecret

**测试方法:**
```bash
helm template test charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  | grep "kind: ExternalSecret" | wc -l
```
**预期输出:** `0` (默认禁用)
**失败判定:** `> 0`

### AC-3.3.3: existingSecret 与 externalSecrets 互斥

**测试方法:**
```bash
helm template test charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  --set litellm.existingSecret=my-secret \
  --set externalSecrets.enabled=true \
  --set externalSecrets.secretStore=my-vault 2>&1
```
**预期输出:** 报错提示两者互斥
**失败判定:** 正常渲染无警告

### AC-3.3.4: 配置文档包含 Vault 和 AWS 示例

**测试方法:**
```bash
test -f docs/guides/external-secrets.md && echo "PASS" || echo "FAIL"
grep -c "vault\|aws.*secrets.*manager\|ClusterSecretStore" docs/guides/external-secrets.md
```
**预期输出:** 文件存在 + 至少 3 处关键词
**失败判定:** 文件不存在

---

## Story 3.4: Model Resolver 集成

### AC-3.4.1: autoDetect 启用后有 init-container

**测试方法:**
```bash
helm template test charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  --set vllm.autoDetect.enabled=true \
  | python3 -c "
import sys, yaml
for doc in yaml.safe_load_all(sys.stdin):
    if not doc or doc.get('kind') != 'Deployment': continue
    name = doc['metadata']['name']
    if 'vllm' not in name: continue
    inits = doc['spec']['template']['spec'].get('initContainers',[])
    resolver = [c for c in inits if 'resolver' in c.get('name','')]
    if resolver:
        print(f'PASS: {name} has model-resolver init-container')
    else:
        print(f'FAIL: {name} missing model-resolver init-container')
"
```
**预期输出:** `PASS`
**失败判定:** `FAIL`

### AC-3.4.2: autoDetect 禁用时无 init-container

**测试方法:**
```bash
helm template test charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  | grep "model-resolver" | wc -l
```
**预期输出:** `0`
**失败判定:** `> 0`

### AC-3.4.3: Model Resolver 单测全部通过

**测试方法:**
```bash
cd images/model-resolver && uv run pytest tests/ -v 2>&1 | tail -5
```
**预期输出:** `28 passed` (或更多)
**失败判定:** 任何测试 FAIL

---

## Story 3.5: RAG 质量保障深耕

### AC-3.5.1: Ragas 评估 7+ 个维度

**测试方法:**
```bash
# 手动触发 Ragas 评估
kubectl create job ragas-test --from=cronjob/$RELEASE-ragas-eval
kubectl wait --for=condition=Complete job/ragas-test --timeout=1800s

# 查询 Pushgateway 中的指标种类
curl -s http://localhost:9091/metrics \
  | grep "^ragas_" | sed 's/{.*//' | sort -u | wc -l
```
**预期输出:** `>= 7` 种不同的 ragas 指标
**失败判定:** `< 7`

### AC-3.5.2: 评估数据集 >= 500 条

**测试方法:**
```bash
python3 -c "
import json
with open('examples/eval/ragas-dataset.json') as f:
    data = json.load(f)
count = len(data.get('samples', data.get('questions', [])))
print(f'Dataset size: {count}')
assert count >= 500, f'Only {count} samples, need >= 500'
print('PASS')
"
```
**预期输出:** `>= 500` + `PASS`
**失败判定:** `< 500`

---

## Story 3.6: 开发者体验

### AC-3.6.1: Tiltfile 存在且可加载

**测试方法:**
```bash
test -f Tiltfile && echo "PASS: Tiltfile exists" || echo "FAIL"
# 语法检查 (不实际启动)
tilt ci --only=kube-llmops 2>&1 | head -5
```
**预期输出:** Tiltfile 存在
**失败判定:** 文件不存在

### AC-3.6.2: pre-commit hook 已配置

**测试方法:**
```bash
test -f .pre-commit-config.yaml && echo "PASS" || echo "FAIL"
grep "helm-dependency-update" .pre-commit-config.yaml && echo "PASS: hook defined" || echo "FAIL"
```
**预期输出:** 文件存在 + hook 定义
**失败判定:** 文件不存在或无 hook

### AC-3.6.3: ADR 目录和初始记录

**测试方法:**
```bash
ADR_COUNT=$(ls docs/adr/0*.md 2>/dev/null | wc -l)
echo "ADR count: $ADR_COUNT"
[ "$ADR_COUNT" -ge 6 ] && echo "PASS" || echo "FAIL"
test -f docs/adr/TEMPLATE.md && echo "PASS: Template exists" || echo "FAIL: No template"
```
**预期输出:** `>= 6` ADR + 模板存在
**失败判定:** ADR 少于 6 个

### AC-3.6.4: Makefile 命令可用

**测试方法:**
```bash
for cmd in dev dep-update lint test-infra; do
  if make -n $cmd &>/dev/null; then
    echo "PASS: make $cmd defined"
  else
    echo "FAIL: make $cmd not found"
  fi
done
```
**预期输出:** 所有 4 个命令 `PASS`
**失败判定:** 任何命令 `FAIL`

---

## Story 3.7: GitOps 集成 (ArgoCD)

### AC-3.7.1: ArgoCD manifests 存在

**测试方法:**
```bash
test -f manifests/argocd/application.yaml && echo "PASS" || echo "FAIL"
test -f manifests/argocd/applicationset.yaml && echo "PASS" || echo "FAIL"
```
**预期输出:** 两个文件都存在
**失败判定:** 任何文件缺失

### AC-3.7.2: Sync Wave 注解正确

**测试方法:**
```bash
helm template test charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  | python3 -c "
import sys, yaml
waves = {}
for doc in yaml.safe_load_all(sys.stdin):
    if not doc or 'metadata' not in doc: continue
    ann = doc.get('metadata',{}).get('annotations',{})
    wave = ann.get('argocd.argoproj.io/sync-wave','N/A')
    name = doc['metadata'].get('name','?')
    kind = doc.get('kind','?')
    if wave != 'N/A':
        waves.setdefault(wave,[]).append(f'{kind}/{name}')
for w in sorted(waves.keys()):
    print(f'Wave {w}: {len(waves[w])} resources')
    for r in waves[w][:3]:
        print(f'  - {r}')
"
```
**预期输出:** Wave 0 包含 PostgreSQL/Redis,Wave 3+ 包含 vLLM
**失败判定:** 无 sync-wave 注解

### AC-3.7.3: GitOps 文档存在

**测试方法:**
```bash
test -f docs/guides/gitops-argocd.md && echo "PASS" || echo "FAIL"
grep -c "ApplicationSet\|Sync Wave\|argocd" docs/guides/gitops-argocd.md
```
**预期输出:** 文件存在 + 多处关键词
**失败判定:** 文件不存在

---

## Story 3.8: 多租户成熟化

### AC-3.8.1: values 定义 tenant 后自动创建资源

**测试方法:**
```bash
# 检查 tenant namespace 存在
kubectl get ns team-alpha team-beta 2>/dev/null
echo "---"
# 检查 ResourceQuota
kubectl get resourcequota -n team-alpha
echo "---"
# 检查 NetworkPolicy
kubectl get networkpolicy -n team-alpha
```
**预期输出:** 两个 namespace + ResourceQuota + NetworkPolicy 都存在
**失败判定:** 任何资源缺失

### AC-3.8.2: 租户间网络隔离

**测试方法:**
```bash
# 从 team-alpha 的 Pod 尝试访问 team-beta
kubectl run isolation-test -n team-alpha --image=busybox --restart=Never --rm -it -- \
  sh -c "nc -zv <team-beta-svc>.team-beta.svc.cluster.local 5001 -w 3 2>&1"
```
**预期输出:** 连接被拒绝/超时
**失败判定:** 连接成功

### AC-3.8.3: Grafana Tenant Dashboard 可见

**测试方法:**
```bash
ADMIN_PW=$(kubectl get secret $RELEASE-grafana-secret -o jsonpath='{.data.admin-password}' | base64 -d)
curl -s "http://admin:${ADMIN_PW}@localhost:3000/api/search?query=tenant" \
  | python3 -c "import sys,json; d=json.load(sys.stdin); print(f'Tenant dashboards: {len(d)}')"
```
**预期输出:** `>= 1`
**失败判定:** `0`

---

## Story 3.9: 性能基线与压力测试

### AC-3.9.1: 压力测试脚本存在

**测试方法:**
```bash
ls tests/load/*.{js,py} 2>/dev/null | wc -l
```
**预期输出:** `>= 3`
**失败判定:** `0`

### AC-3.9.2: make bench 可执行

**测试方法:**
```bash
make -n bench 2>/dev/null && echo "PASS" || echo "FAIL"
```
**预期输出:** `PASS`
**失败判定:** `FAIL`

### AC-3.9.3: 性能报告文档存在且有数据

**测试方法:**
```bash
test -f docs/guides/performance-report.md && echo "PASS" || echo "FAIL"
grep -cE "P50|P95|P99|tokens/s|TTFT|sizing" docs/guides/performance-report.md
```
**预期输出:** 文件存在 + 至少 5 处性能关键词
**失败判定:** 文件不存在或内容为空

### AC-3.9.4: kube-llmops 开销对比数据

**测试方法:**
```bash
grep -i "overhead\|bare.*vllm\|comparison\|baseline" docs/guides/performance-report.md | head -5
```
**预期输出:** 包含与裸 vLLM 的对比数据
**失败判定:** 无对比内容
