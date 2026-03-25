# Phase 2 Story 验收标准与测试方法

> **约定**: `RELEASE=kube-llmops`, `NS=default`
> **前置条件**: Phase 1 全部 Story 已完成

---

## Story 2.1: PostgreSQL 拆分

### AC-2.1.1: 两个独立 PG 实例存在

**测试方法:**
```bash
# 检查两个 PG Service
kubectl get svc -o name | grep -E "operator-pg|app-pg"

# 检查两个独立 PVC
kubectl get pvc -o custom-columns=NAME:.metadata.name,STATUS:.status.phase \
  | grep -E "operator-pg|app-pg"
```
**预期输出:** 两个 Service + 两个 Bound PVC
**失败判定:** 缺少任一 Service 或 PVC

### AC-2.1.2: LiteLLM 连接 operator-pg

**测试方法:**
```bash
kubectl get deployment -l app.kubernetes.io/name=litellm \
  -o jsonpath='{.items[0].spec.template.spec.containers[0].env}' \
  | python3 -c "
import sys, json
envs = json.load(sys.stdin)
for e in envs:
    if 'DATABASE' in e.get('name','').upper():
        val = e.get('value','') or '(from secret)'
        print(f'{e[\"name\"]}: {val}')
" | grep -i "operator-pg"
```
**预期输出:** DATABASE_URL 包含 `operator-pg`
**失败判定:** 仍然指向 `litellm-pg`

### AC-2.1.3: Langfuse/Dify 连接 app-pg

**测试方法:**
```bash
for deploy in langfuse dify-api; do
  echo "=== $deploy ==="
  kubectl get deployment -l app.kubernetes.io/component=$deploy \
    -o jsonpath='{.items[0].spec.template.spec.containers[0].env}' 2>/dev/null \
    | python3 -c "
import sys, json
envs = json.load(sys.stdin)
for e in envs:
    if 'DATABASE' in e.get('name','').upper() or 'POSTGRES' in e.get('name','').upper():
        print(f'  {e[\"name\"]}: contains app-pg? {\"app-pg\" in str(e)}')
"
done
```
**预期输出:** Langfuse 和 Dify 的 DATABASE_URL 包含 `app-pg`
**失败判定:** 仍然指向 `litellm-pg` 或 `operator-pg`

### AC-2.1.4: 故障隔离验证

**测试方法:**
```bash
# 杀掉 app-pg,验证 LiteLLM 不受影响
APP_PG=$(kubectl get pod -l app.kubernetes.io/component=app-pg -o name | head -1)
kubectl delete $APP_PG --grace-period=5

# 立即测试 LiteLLM 健康
sleep 5
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
  http://localhost:4000/health/liveliness \
  -H "Authorization: Bearer $(kubectl get secret $RELEASE-litellm-secret -o jsonpath='{.data.master-key}' | base64 -d)")

if [ "$HTTP_CODE" = "200" ]; then
  echo "PASS: LiteLLM healthy after app-pg failure"
else
  echo "FAIL: LiteLLM affected by app-pg failure (HTTP $HTTP_CODE)"
fi

# 等待 app-pg 恢复
kubectl wait --for=condition=Ready pod -l app.kubernetes.io/component=app-pg --timeout=120s
```
**预期输出:** `PASS: LiteLLM healthy after app-pg failure`
**失败判定:** LiteLLM 返回非 200

### AC-2.1.5: E2E 全链路通过

**测试方法:**
```bash
uv run tests/e2e/test_dify_model_provider.py
uv run tests/e2e/test_dify_rag_e2e.py
```
**预期输出:** 全部 14 个测试用例 PASS
**失败判定:** 任何测试 FAIL

---

## Story 2.2: values.schema.json

### AC-2.2.1: schema 文件存在且格式正确

**测试方法:**
```bash
python3 -c "
import json
with open('charts/kube-llmops-stack/values.schema.json') as f:
    schema = json.load(f)
print(f'Schema loaded: {len(schema.get(\"properties\",{}))} top-level properties')
assert '\$schema' in schema, 'Missing \$schema field'
print('PASS: Schema is valid JSON Schema')
"
```
**预期输出:** `PASS` + 属性数量
**失败判定:** JSON 解析错误或缺少 `$schema`

### AC-2.2.2: 缺少必填字段时 helm lint 报错

**测试方法:**
```bash
# 创建一个缺少必填字段的 values 文件
echo "vllm: {enabled: true}" > /tmp/bad-values.yaml
helm lint charts/kube-llmops-stack -f /tmp/bad-values.yaml --strict 2>&1
```
**预期输出:** 包含 error 或 warning 指出缺少必填字段
**失败判定:** lint 通过无错误

### AC-2.2.3: 类型错误时报错

**测试方法:**
```bash
echo 'vllm: {enabled: "yes"}' > /tmp/type-error.yaml
helm lint charts/kube-llmops-stack -f /tmp/type-error.yaml --strict 2>&1 | grep -i "error\|invalid"
```
**预期输出:** 报告类型不匹配错误
**失败判定:** lint 通过

### AC-2.2.4: 所有 profile 通过校验

**测试方法:**
```bash
for profile in ci minimal single-node standard production; do
  VALUES="charts/kube-llmops-stack/values-${profile}.yaml"
  if [ -f "$VALUES" ]; then
    helm lint charts/kube-llmops-stack -f "$VALUES" --strict 2>&1 | tail -1
    echo "--- $profile: $?"
  fi
done
```
**预期输出:** 所有 profile exit code 0
**失败判定:** 任何 profile lint 失败

---

## Story 2.3: AlertManager + 通知渠道

### AC-2.3.1: AlertManager Deployment 存在

**测试方法:**
```bash
kubectl get deployment -l app.kubernetes.io/name=alertmanager \
  -o custom-columns=NAME:.metadata.name,READY:.status.readyReplicas
```
**预期输出:** 1 个 Deployment,READY=1
**失败判定:** Deployment 不存在或 READY=0

### AC-2.3.2: Prometheus 连接 AlertManager

**测试方法:**
```bash
kubectl port-forward svc/$RELEASE-prometheus 9090:9090 &
sleep 2
curl -s http://localhost:9090/api/v1/alertmanagers \
  | python3 -c "
import sys, json
data = json.load(sys.stdin)
ams = data.get('data',{}).get('activeAlertmanagers',[])
print(f'Active AlertManagers: {len(ams)}')
for am in ams:
    print(f'  {am[\"url\"]}')
assert len(ams) >= 1, 'No active AlertManager'
print('PASS')
"
kill %1 2>/dev/null
```
**预期输出:** `Active AlertManagers: 1` + `PASS`
**失败判定:** 0 个 AlertManager

### AC-2.3.3: 告警通知端到端 (Webhook)

**测试方法:**
```bash
# 启动临时 webhook 接收器
python3 -c "
from http.server import HTTPServer, BaseHTTPRequestHandler
import json, threading
received = []
class H(BaseHTTPRequestHandler):
    def do_POST(self):
        data = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
        received.append(data)
        self.send_response(200); self.end_headers()
    def log_message(self, *a): pass
srv = HTTPServer(('0.0.0.0', 9999), H)
threading.Timer(60, srv.shutdown).start()
srv.serve_forever()
print(f'Received {len(received)} alerts')
" &
WEBHOOK_PID=$!

# 部署配置 webhook
helm upgrade $RELEASE charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  --set alertmanager.webhook.enabled=true \
  --set alertmanager.webhook.url=http://host.docker.internal:9999

# 模拟告警 (停止 vLLM 触发 VllmDown)
kubectl scale deployment -l app.kubernetes.io/name=vllm --replicas=0
sleep 90  # 等待告警触发 + AlertManager 发送

wait $WEBHOOK_PID
# 检查是否收到告警
```
**预期输出:** Webhook 接收到至少 1 条告警
**失败判定:** 60 秒内未收到告警

---

## Story 2.4: Helm 标签标准统一

### AC-2.4.1: 所有子 Chart 有 _helpers.tpl

**测试方法:**
```bash
for chart in charts/kube-llmops-stack/charts/*/; do
  NAME=$(basename $chart)
  if [ -f "${chart}templates/_helpers.tpl" ]; then
    echo "PASS: $NAME"
  else
    echo "FAIL: $NAME — missing _helpers.tpl"
  fi
done
```
**预期输出:** 所有 16 个子 Chart 显示 `PASS`
**失败判定:** 任何子 Chart 显示 `FAIL`

### AC-2.4.2: 所有资源包含标准标签

**测试方法:**
```bash
helm template test charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  | python3 -c "
import sys, yaml
required = ['app.kubernetes.io/name','app.kubernetes.io/instance','app.kubernetes.io/part-of']
fail_count = 0
for doc in yaml.safe_load_all(sys.stdin):
    if not doc or 'metadata' not in doc: continue
    labels = doc.get('metadata',{}).get('labels',{})
    name = doc['metadata'].get('name','?')
    kind = doc.get('kind','?')
    missing = [r for r in required if r not in labels]
    if missing and kind not in ('Namespace','ClusterRole','ClusterRoleBinding'):
        print(f'FAIL: {kind}/{name} missing labels: {missing}')
        fail_count += 1
print(f'Total failures: {fail_count}')
"
```
**预期输出:** `Total failures: 0`
**失败判定:** `Total failures > 0`

### AC-2.4.3: kubectl 全局选择器有效

**测试方法:**
```bash
EXPECTED=$(kubectl get pods --no-headers | grep -v Completed | wc -l)
SELECTED=$(kubectl get pods -l app.kubernetes.io/part-of=kube-llmops --no-headers | grep -v Completed | wc -l)
echo "Expected: $EXPECTED, Selected: $SELECTED"
if [ "$EXPECTED" -eq "$SELECTED" ]; then
  echo "PASS: All pods selected by part-of label"
else
  echo "FAIL: $((EXPECTED - SELECTED)) pods not labeled"
fi
```
**预期输出:** `PASS`
**失败判定:** 数量不匹配

---

## Story 2.5: 升级指南

### AC-2.5.1: 升级文档存在

**测试方法:**
```bash
test -f docs/guides/upgrade-guide.md && echo "PASS" || echo "FAIL"
grep -c "v0.1.*v0.2\|v0.2.*v0.3\|breaking\|rollback" docs/guides/upgrade-guide.md
```
**预期输出:** `PASS` + 至少 3 处提及版本迁移/回滚
**失败判定:** 文件不存在

### AC-2.5.2: pre-upgrade hook 增加基础设施检查

**测试方法:**
```bash
helm template test charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  | grep -B5 "pre-upgrade" | grep -i "health\|postgres\|litellm"
```
**预期输出:** pre-upgrade Job 包含 PG 和 LiteLLM 健康检查
**失败判定:** 仅检查 Ragas 指标

---

## Story 2.6: NetworkPolicy 补全

### AC-2.6.1: 所有服务有 Ingress NetworkPolicy

**测试方法:**
```bash
NP_COUNT=$(kubectl get networkpolicy -o name | wc -l)
SVC_COUNT=$(kubectl get svc -l app.kubernetes.io/part-of=kube-llmops -o name | wc -l)
echo "NetworkPolicies: $NP_COUNT, Services: $SVC_COUNT"
# NetworkPolicy 数量应 >= 服务数量 (deny-default + 每服务 1 条)
if [ "$NP_COUNT" -ge "$SVC_COUNT" ]; then
  echo "PASS"
else
  echo "FAIL: Not all services covered"
fi
```
**预期输出:** `PASS`
**失败判定:** NetworkPolicy 数量明显少于服务数量

### AC-2.6.2: PostgreSQL 仅接受授权连接

**测试方法:**
```bash
# 从一个未授权的 Pod 尝试连接 PG (应被拒绝)
kubectl run np-test --image=busybox --restart=Never --rm -it -- \
  sh -c "nc -zv $RELEASE-operator-pg 5432 -w 3 2>&1" 2>&1 | tail -3
```
**预期输出:** 连接超时或被拒绝 (非 `succeeded`)
**失败判定:** 连接成功 (NetworkPolicy 未生效)

### AC-2.6.3: Egress 规则生效

**测试方法:**
```bash
# 启用 Egress 后,从 vLLM Pod 测试外部连接
kubectl exec $(kubectl get pod -l app.kubernetes.io/name=vllm -o jsonpath='{.items[0].metadata.name}') \
  -- timeout 5 curl -s http://example.com 2>&1 | head -3
```
**预期输出 (Egress 启用时):** 连接超时 (仅允许 HTTPS 443)
**失败判定:** 成功连接到 HTTP 80 外部站点

---

## Story 2.7: CI 自动化测试集成

### AC-2.7.1: QA 脚本在 CI 中运行

**测试方法 (手动验证):**
1. 查看最近的 CI 运行日志
2. 搜索 `01-deploy-verify.sh` 或 `02-k8s-resource-test.py` 的输出

**预期结果:** CI 日志中包含 QA 脚本的测试结果输出
**失败判定:** CI 日志中无 QA 脚本相关内容

### AC-2.7.2: 测试结果 Artifact 可下载

**测试方法 (手动验证):**
1. 在 GitHub Actions 页面查看最近的 workflow 运行
2. 检查 Artifacts 列表中是否有 `infra-test-results`

**预期结果:** Artifact 存在且可下载
**失败判定:** 无 Artifact

---

## Story 2.8: 成本/团队 Dashboard

### AC-2.8.1: Dashboard 在 Grafana 中自动加载

**测试方法:**
```bash
kubectl port-forward svc/$RELEASE-grafana 3000:3000 &
sleep 2
curl -s http://admin:$(kubectl get secret $RELEASE-grafana-secret -o jsonpath='{.data.admin-password}' | base64 -d)@localhost:3000/api/search?query=cost \
  | python3 -c "
import sys, json
dashboards = json.load(sys.stdin)
print(f'Found {len(dashboards)} cost-related dashboards')
for d in dashboards:
    print(f'  - {d[\"title\"]}')
assert len(dashboards) >= 1, 'No cost dashboard found'
print('PASS')
"
kill %1 2>/dev/null
```
**预期输出:** 至少 1 个 cost-related Dashboard
**失败判定:** 0 个 Dashboard

### AC-2.8.2: 按 API Key 分组数据可见

**测试方法:**
```bash
# 用不同 API Key 发送请求
for key in sk-key-team-a sk-key-team-b; do
  curl -s http://localhost:4000/v1/chat/completions \
    -H "Authorization: Bearer $key" \
    -H "Content-Type: application/json" \
    -d '{"model":"qwen2-5-0-5b","messages":[{"role":"user","content":"test"}],"max_tokens":5}'
done
sleep 15

# 查询 LiteLLM 指标按 api_key 分组
curl -s "http://localhost:9090/api/v1/query?query=sum+by(api_key)(litellm_total_tokens)" \
  | python3 -c "import sys,json; d=json.load(sys.stdin); print(f'Series: {len(d[\"data\"][\"result\"])}')"
```
**预期输出:** `Series: >= 2` (按不同 API Key 分组)
**失败判定:** Series 为 0 或 1

---

## Story 2.9: Langfuse OIDC

### AC-2.9.1: SSO 登录按钮存在

**测试方法:**
```bash
curl -s https://langfuse.$INGRESS_HOST/ 2>/dev/null \
  | grep -i "sso\|openid\|keycloak\|sign.*in.*with"
```
**预期输出:** 页面包含 SSO 登录选项
**失败判定:** 仅有用户名/密码登录

### AC-2.9.2: Keycloak SSO 登录成功

**测试方法 (Playwright 或手动):**
1. 打开 Langfuse URL
2. 点击 SSO 登录
3. 重定向到 Keycloak 登录页面
4. 输入 Keycloak 用户凭据
5. 成功重定向回 Langfuse 并登录

**预期结果:** 成功登录 Langfuse,显示用户 Dashboard
**失败判定:** 重定向失败或登录报错

---

## Story 2.10: 备份 CronJob

### AC-2.10.1: CronJob 渲染且存在

**测试方法:**
```bash
helm template test charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-production.yaml \
  | grep "kind: CronJob" -A5 | grep "pg-backup"
```
**预期输出:** 包含 `pg-backup` CronJob
**失败判定:** CronJob 未渲染

### AC-2.10.2: 手动触发备份成功

**测试方法:**
```bash
kubectl create job pg-backup-manual --from=cronjob/$RELEASE-pg-backup

# 等待完成
kubectl wait --for=condition=Complete job/pg-backup-manual --timeout=120s

# 检查备份文件
kubectl exec $(kubectl get pod -l job-name=pg-backup-manual -o jsonpath='{.items[0].metadata.name}') \
  -- ls -la /backup/
```
**预期输出:** `/backup/` 中有 `full-backup-*.sql` 文件
**失败判定:** Job 失败或备份文件不存在

### AC-2.10.3: 保留策略生效

**测试方法:**
```bash
# 创建一个超过保留天数的假备份文件
kubectl exec $(kubectl get pod -l app.kubernetes.io/component=pg-backup -o name | head -1) \
  -- touch -d "10 days ago" /backup/full-backup-old.sql

# 触发备份
kubectl create job retention-test --from=cronjob/$RELEASE-pg-backup
kubectl wait --for=condition=Complete job/retention-test --timeout=120s

# 检查旧文件是否被清理
kubectl exec $(kubectl get pod -l job-name=retention-test -o name | head -1) \
  -- ls /backup/full-backup-old.sql 2>&1
```
**预期输出:** `No such file or directory` (旧备份已被删除)
**失败判定:** 旧文件仍然存在
