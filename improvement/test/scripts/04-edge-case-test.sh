#!/usr/bin/env bash
# ============================================================================
# kube-llmops 边缘异常场景测试脚本 (Edge Case Tests)
# 用途: 测试 OOM、ImagePullBackOff、ConfigMap 热更新、Pod 驱逐等边缘场景
# 用法: bash scripts/04-edge-case-test.sh [--namespace default]
# ============================================================================
set -uo pipefail

NAMESPACE="${1:-default}"
PASS=0; FAIL=0; WARN=0; TOTAL=0

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; NC='\033[0m'

log_pass() { PASS=$((PASS+1)); TOTAL=$((TOTAL+1)); echo -e "${GREEN}[PASS]${NC} $1"; }
log_fail() { FAIL=$((FAIL+1)); TOTAL=$((TOTAL+1)); echo -e "${RED}[FAIL]${NC} $1"; }
log_warn() { WARN=$((WARN+1)); TOTAL=$((TOTAL+1)); echo -e "${YELLOW}[WARN]${NC} $1"; }
log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
section()  { echo -e "\n${BLUE}━━━ $1 ━━━${NC}"; }

# ============================================================================
# TEST 1: OOMKilled 检测
# ============================================================================
section "TEST 1/5: OOMKilled 行为验证"

log_info "创建一个会触发 OOM 的测试 Pod (memory limit=64Mi, alloc=128Mi)"

kubectl delete pod oom-test -n "$NAMESPACE" --ignore-not-found=true --grace-period=0 --force 2>/dev/null || true
sleep 2

cat <<'EOF' | kubectl apply -n "$NAMESPACE" -f -
apiVersion: v1
kind: Pod
metadata:
  name: oom-test
  labels:
    test: edge-case
spec:
  containers:
  - name: oom
    image: python:3.12-slim
    resources:
      limits:
        memory: 64Mi
      requests:
        memory: 32Mi
    command: ["python3", "-c", "x = bytearray(1024*1024*128)"]
  restartPolicy: Never
EOF

log_info "等待 Pod 完成 (最多 60s)..."
for i in $(seq 1 60); do
    STATUS=$(kubectl get pod oom-test -n "$NAMESPACE" -o jsonpath='{.status.phase}' 2>/dev/null || echo "unknown")
    if [ "$STATUS" = "Failed" ] || [ "$STATUS" = "Succeeded" ]; then
        break
    fi
    sleep 1
done

TERM_REASON=$(kubectl get pod oom-test -n "$NAMESPACE" -o jsonpath='{.status.containerStatuses[0].state.terminated.reason}' 2>/dev/null || echo "")
if [ "$TERM_REASON" = "OOMKilled" ]; then
    log_pass "OOM 场景: Pod 被正确标记为 OOMKilled"
else
    log_warn "OOM 场景: 终止原因=$TERM_REASON (预期 OOMKilled)"
fi

# 检查 Events
OOM_EVENTS=$(kubectl get events -n "$NAMESPACE" --field-selector involvedObject.name=oom-test --no-headers 2>/dev/null | wc -l)
if [ "$OOM_EVENTS" -gt 0 ]; then
    log_pass "OOM Events 已记录 ($OOM_EVENTS 条)"
else
    log_warn "未找到 OOM 相关 Events"
fi

kubectl delete pod oom-test -n "$NAMESPACE" --ignore-not-found=true --grace-period=0 --force 2>/dev/null || true

# ============================================================================
# TEST 2: ImagePullBackOff 行为验证
# ============================================================================
section "TEST 2/5: ImagePullBackOff 行为验证"

log_info "创建一个使用不存在镜像的 Pod"

kubectl delete pod imagepull-test -n "$NAMESPACE" --ignore-not-found=true --grace-period=0 --force 2>/dev/null || true
sleep 2

cat <<'EOF' | kubectl apply -n "$NAMESPACE" -f -
apiVersion: v1
kind: Pod
metadata:
  name: imagepull-test
  labels:
    test: edge-case
spec:
  containers:
  - name: test
    image: nonexistent-registry.example.com/fake-image:v999
    command: ["sleep", "3600"]
  restartPolicy: Never
EOF

log_info "等待 30s 观察镜像拉取行为..."
sleep 30

POD_STATUS=$(kubectl get pod imagepull-test -n "$NAMESPACE" -o jsonpath='{.status.containerStatuses[0].state.waiting.reason}' 2>/dev/null || echo "")
if [ "$POD_STATUS" = "ImagePullBackOff" ] || [ "$POD_STATUS" = "ErrImagePull" ]; then
    log_pass "ImagePull 失败: Pod 状态=$POD_STATUS (符合预期)"
else
    log_warn "ImagePull 行为: 状态=$POD_STATUS (预期 ImagePullBackOff/ErrImagePull)"
fi

# 检查 Events 是否记录拉取失败
PULL_EVENTS=$(kubectl get events -n "$NAMESPACE" --field-selector involvedObject.name=imagepull-test --no-headers 2>/dev/null | grep -c "pull" || echo 0)
if [ "$PULL_EVENTS" -gt 0 ]; then
    log_pass "ImagePull Events 已记录"
else
    log_warn "未找到 ImagePull Events"
fi

kubectl delete pod imagepull-test -n "$NAMESPACE" --ignore-not-found=true --grace-period=0 --force 2>/dev/null || true

# ============================================================================
# TEST 3: QoS Class 与驱逐优先级验证
# ============================================================================
section "TEST 3/5: QoS Class 与驱逐优先级"

BEST_EFFORT_COUNT=0
BURSTABLE_COUNT=0
GUARANTEED_COUNT=0

while IFS= read -r line; do
    POD_NAME=$(echo "$line" | awk '{print $1}')
    QOS=$(echo "$line" | awk '{print $2}')
    case "$QOS" in
        BestEffort) BEST_EFFORT_COUNT=$((BEST_EFFORT_COUNT+1)) ;;
        Burstable) BURSTABLE_COUNT=$((BURSTABLE_COUNT+1)) ;;
        Guaranteed) GUARANTEED_COUNT=$((GUARANTEED_COUNT+1)) ;;
    esac
done < <(kubectl get pods -n "$NAMESPACE" --no-headers -o custom-columns=NAME:.metadata.name,QOS:.status.qosClass 2>/dev/null | grep -E "Running")

log_info "QoS 分布: Guaranteed=$GUARANTEED_COUNT Burstable=$BURSTABLE_COUNT BestEffort=$BEST_EFFORT_COUNT"

# 关键组件不应为 BestEffort
CRITICAL_LABELS=(
    "app.kubernetes.io/name=litellm-postgresql"
    "app.kubernetes.io/name=litellm"
    "app.kubernetes.io/name=vllm"
    "app.kubernetes.io/name=langfuse"
    "app.kubernetes.io/name=dify-api"
)

for label in "${CRITICAL_LABELS[@]}"; do
    NAME=$(echo "$label" | sed 's/.*=//')
    QOS=$(kubectl get pods -n "$NAMESPACE" -l "$label" -o jsonpath='{.items[0].status.qosClass}' 2>/dev/null || echo "unknown")
    if [ "$QOS" != "BestEffort" ] && [ "$QOS" != "unknown" ]; then
        log_pass "$NAME QoS=$QOS (不会被优先驱逐)"
    elif [ "$QOS" = "unknown" ]; then
        log_warn "$NAME: 未找到 Pod"
    else
        log_fail "$NAME QoS=BestEffort (容易被驱逐!)"
    fi
done

# ============================================================================
# TEST 4: Pod 重启后数据持久化快速验证
# ============================================================================
section "TEST 4/5: Pod 重启后数据持久化"

# 记录 PostgreSQL 数据库列表
PG_POD=$(kubectl get pod -n "$NAMESPACE" -l app.kubernetes.io/name=litellm-postgresql -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)

if [ -n "$PG_POD" ]; then
    # 记录重启前数据
    DB_BEFORE=$(kubectl exec -n "$NAMESPACE" "$PG_POD" -- psql -U litellm -d litellm -t -c "SELECT datname FROM pg_database WHERE datistemplate = false ORDER BY datname;" 2>/dev/null | tr -d ' ' | sort)
    log_info "PostgreSQL 重启前数据库: $(echo "$DB_BEFORE" | tr '\n' ',')"

    # 删除 Pod 触发重建
    log_info "删除 PostgreSQL Pod 触发重建..."
    kubectl delete pod "$PG_POD" -n "$NAMESPACE" --grace-period=30
    log_info "等待 PostgreSQL Pod 重新就绪..."
    kubectl wait --for=condition=Ready pod -n "$NAMESPACE" -l app.kubernetes.io/name=litellm-postgresql --timeout=120s 2>/dev/null

    # 验证数据
    sleep 5
    NEW_PG_POD=$(kubectl get pod -n "$NAMESPACE" -l app.kubernetes.io/name=litellm-postgresql -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    DB_AFTER=$(kubectl exec -n "$NAMESPACE" "$NEW_PG_POD" -- psql -U litellm -d litellm -t -c "SELECT datname FROM pg_database WHERE datistemplate = false ORDER BY datname;" 2>/dev/null | tr -d ' ' | sort)
    log_info "PostgreSQL 重启后数据库: $(echo "$DB_AFTER" | tr '\n' ',')"

    if [ "$DB_BEFORE" = "$DB_AFTER" ]; then
        log_pass "PostgreSQL 数据持久化: 重启后数据库列表一致"
    else
        log_fail "PostgreSQL 数据持久化: 重启后数据库列表不一致"
    fi
else
    log_warn "未找到 PostgreSQL Pod，跳过持久化测试"
fi

# ============================================================================
# TEST 5: GPU 资源不足调度行为验证
# ============================================================================
section "TEST 5/5: GPU 资源不足调度行为"

GPU_ALLOCATABLE=$(kubectl get nodes -o jsonpath='{range .items[*]}{.status.allocatable.nvidia\.com/gpu}{"\n"}{end}' 2>/dev/null | awk '{s+=$1} END{print s+0}')

if [ "$GPU_ALLOCATABLE" -ge 1 ]; then
    log_info "集群 GPU=$GPU_ALLOCATABLE，创建一个请求 GPU 的测试 Pod"

    kubectl delete pod gpu-exhaust-test -n "$NAMESPACE" --ignore-not-found=true --grace-period=0 --force 2>/dev/null || true
    sleep 2

    # 请求超出集群可用的 GPU 数量
    EXCESS_GPU=$((GPU_ALLOCATABLE + 1))

    cat <<EOF | kubectl apply -n "$NAMESPACE" -f -
apiVersion: v1
kind: Pod
metadata:
  name: gpu-exhaust-test
  labels:
    test: edge-case
spec:
  containers:
  - name: gpu-test
    image: nvidia/cuda:12.4.0-base-ubuntu22.04
    command: ["sleep", "10"]
    resources:
      limits:
        nvidia.com/gpu: "$EXCESS_GPU"
      requests:
        nvidia.com/gpu: "$EXCESS_GPU"
  restartPolicy: Never
EOF

    log_info "等待 30s 观察调度行为 (请求 $EXCESS_GPU GPU，集群仅有 $GPU_ALLOCATABLE)..."
    sleep 30

    PHASE=$(kubectl get pod gpu-exhaust-test -n "$NAMESPACE" -o jsonpath='{.status.phase}' 2>/dev/null || echo "unknown")
    if [ "$PHASE" = "Pending" ]; then
        # 检查 Events 中是否有 Insufficient nvidia.com/gpu
        SCHED_EVENTS=$(kubectl describe pod gpu-exhaust-test -n "$NAMESPACE" 2>/dev/null | grep -c "Insufficient nvidia.com/gpu" || echo 0)
        if [ "$SCHED_EVENTS" -gt 0 ]; then
            log_pass "GPU 不足: Pod Pending + Insufficient nvidia.com/gpu 事件"
        else
            log_pass "GPU 不足: Pod Pending (调度器正确拒绝)"
        fi
    else
        log_warn "GPU 不足: Pod 状态=$PHASE (预期 Pending)"
    fi

    kubectl delete pod gpu-exhaust-test -n "$NAMESPACE" --ignore-not-found=true --grace-period=0 --force 2>/dev/null || true
else
    log_warn "集群无 GPU，跳过 GPU 调度测试"
fi

# ============================================================================
# 汇总
# ============================================================================
section "边缘异常测试汇总"

echo -e "\n${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "  总测试: $TOTAL | ${GREEN}通过: $PASS${NC} | ${RED}失败: $FAIL${NC} | ${YELLOW}警告: $WARN${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

if [ "$FAIL" -gt 0 ]; then
    echo -e "\n${RED}边缘异常测试有失败项${NC}"
    exit 1
else
    echo -e "\n${GREEN}边缘异常测试全部通过${NC}"
    exit 0
fi
