#!/usr/bin/env bash
# ============================================================================
# kube-llmops 部署验证脚本 (Deploy Verification Script)
# 用途: 一键检查所有 K8s 资源状态、PVC、GPU、健康端点
# 用法: bash scripts/01-deploy-verify.sh [--namespace default] [--release kube-llmops]
# ============================================================================
set -uo pipefail

# ---------- 参数解析 ----------
NAMESPACE="${1:-default}"
RELEASE="${2:-kube-llmops}"
PASS=0; FAIL=0; WARN=0; TOTAL=0
REPORT_FILE="test-report-$(date +%Y%m%d-%H%M%S).txt"

# ---------- 颜色输出 ----------
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; NC='\033[0m'

log_pass() { PASS=$((PASS+1)); TOTAL=$((TOTAL+1)); echo -e "${GREEN}[PASS]${NC} $1"; echo "[PASS] $1" >> "$REPORT_FILE"; }
log_fail() { FAIL=$((FAIL+1)); TOTAL=$((TOTAL+1)); echo -e "${RED}[FAIL]${NC} $1"; echo "[FAIL] $1" >> "$REPORT_FILE"; }
log_warn() { WARN=$((WARN+1)); TOTAL=$((TOTAL+1)); echo -e "${YELLOW}[WARN]${NC} $1"; echo "[WARN] $1" >> "$REPORT_FILE"; }
log_info() { echo -e "${BLUE}[INFO]${NC} $1"; echo "[INFO] $1" >> "$REPORT_FILE"; }
section()  { echo -e "\n${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"; echo -e "${BLUE}  $1${NC}"; echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"; }

echo "kube-llmops Infra Test Report - $(date)" > "$REPORT_FILE"
echo "Namespace: $NAMESPACE | Release: $RELEASE" >> "$REPORT_FILE"
echo "==========================================" >> "$REPORT_FILE"

# ============================================================================
# SECTION 1: 集群与节点检查
# ============================================================================
section "1/7  集群与节点状态"

# 1.1 kubectl 连通性
if kubectl cluster-info &>/dev/null; then
    log_pass "kubectl 连接集群成功"
else
    log_fail "kubectl 无法连接集群"
    echo "FATAL: 无法连接集群，终止测试" >&2; exit 1
fi

# 1.2 节点状态
NOT_READY=$(kubectl get nodes --no-headers 2>/dev/null | grep -v " Ready " | wc -l | tr -d ' ')
TOTAL_NODES=$(kubectl get nodes --no-headers 2>/dev/null | wc -l | tr -d ' ')
if [ "$NOT_READY" -eq 0 ]; then
    log_pass "所有节点 ($TOTAL_NODES) 状态 Ready"
else
    log_fail "$NOT_READY/$TOTAL_NODES 节点未就绪"
fi

# 1.3 GPU 资源
GPU_ALLOCATABLE=$(kubectl get nodes -o jsonpath='{range .items[*]}{.status.allocatable.nvidia\.com/gpu}{"\n"}{end}' 2>/dev/null | awk '{s+=$1} END{print s+0}')
if [ "$GPU_ALLOCATABLE" -ge 1 ]; then
    log_pass "集群 GPU 总量: $GPU_ALLOCATABLE"
else
    log_warn "集群无可分配 GPU ($GPU_ALLOCATABLE)"
fi

# 1.4 NVIDIA Device Plugin
NDP_PODS=$(kubectl get pods -n kube-system --no-headers 2>/dev/null | grep -i "nvidia-device-plugin" | grep -c "Running" || echo 0)
NDP_PODS=$(echo "$NDP_PODS" | tr -d ' ')
if [ "$NDP_PODS" -ge 1 ]; then
    log_pass "NVIDIA Device Plugin 运行中 ($NDP_PODS pods)"
else
    log_warn "NVIDIA Device Plugin 未检测到"
fi

# 1.5 StorageClass
SC_COUNT=$(kubectl get storageclass --no-headers 2>/dev/null | wc -l)
DEFAULT_SC=$(kubectl get storageclass -o jsonpath='{.items[?(@.metadata.annotations.storageclass\.kubernetes\.io/is-default-class=="true")].metadata.name}' 2>/dev/null)
if [ "$SC_COUNT" -ge 1 ]; then
    log_pass "StorageClass 可用 ($SC_COUNT 个), 默认: ${DEFAULT_SC:-无}"
else
    log_fail "无可用 StorageClass"
fi

# ============================================================================
# SECTION 2: Pod 状态检查
# ============================================================================
section "2/7  Pod 状态检查"

# 获取所有 Pod 状态
TOTAL_PODS=$(kubectl get pods -n "$NAMESPACE" --no-headers 2>/dev/null | wc -l)
RUNNING_PODS=$(kubectl get pods -n "$NAMESPACE" --no-headers 2>/dev/null | grep -c "Running" || echo 0)
COMPLETED_PODS=$(kubectl get pods -n "$NAMESPACE" --no-headers 2>/dev/null | grep -c "Completed" || echo 0)
PROBLEM_PODS=$(kubectl get pods -n "$NAMESPACE" --no-headers 2>/dev/null | grep -vE "Running|Completed" || true)

log_info "总 Pod 数: $TOTAL_PODS | Running: $RUNNING_PODS | Completed: $COMPLETED_PODS"

if [ -z "$PROBLEM_PODS" ]; then
    log_pass "所有 Pod 状态正常 (Running/Completed)"
else
    PROBLEM_COUNT=$(echo "$PROBLEM_PODS" | grep -c . || echo 0)
    log_fail "$PROBLEM_COUNT 个 Pod 状态异常:"
    echo "$PROBLEM_PODS" | while read -r line; do
        POD_NAME=$(echo "$line" | awk '{print $1}')
        POD_STATUS=$(echo "$line" | awk '{print $3}')
        echo -e "  ${RED}↳ $POD_NAME ($POD_STATUS)${NC}"
    done
fi

# 2.2 关键组件逐一检查 (labels match actual deployment)
CRITICAL_COMPONENTS=(
    "app.kubernetes.io/name=litellm-postgresql:PostgreSQL"
    "app.kubernetes.io/name=minio:MinIO"
    "app.kubernetes.io/name=litellm:LiteLLM"
    "app.kubernetes.io/name=langfuse:Langfuse"
    "app.kubernetes.io/name=vllm:vLLM"
    "app.kubernetes.io/name=tei:TEI"
    "app.kubernetes.io/name=dify-api:Dify-API"
    "app.kubernetes.io/name=dify-web:Dify-Web"
    "app.kubernetes.io/name=keycloak:Keycloak"
    "app.kubernetes.io/name=grafana:Grafana"
    "app.kubernetes.io/name=prometheus:Prometheus"
    "app.kubernetes.io/name=llm-guard:LLM-Guard"
    "app.kubernetes.io/name=loki:Loki"
    # Phase 4 新增组件
    "app.kubernetes.io/name=lightrag:LightRAG"
    "app.kubernetes.io/name=neo4j:Neo4j"
    "app.kubernetes.io/name=milvus:Milvus"
    "app.kubernetes.io/name=presidio-analyzer:Presidio-Analyzer"
    "app.kubernetes.io/name=presidio-anonymizer:Presidio-Anonymizer"
)

for comp in "${CRITICAL_COMPONENTS[@]}"; do
    LABEL="${comp%%:*}"
    NAME="${comp##*:}"
    READY=$(kubectl get pods -n "$NAMESPACE" -l "$LABEL" --no-headers 2>/dev/null | grep -c "Running" || echo 0)
    TOTAL_C=$(kubectl get pods -n "$NAMESPACE" -l "$LABEL" --no-headers 2>/dev/null | wc -l)
    if [ "$TOTAL_C" -eq 0 ]; then
        log_warn "$NAME: 无 Pod 匹配 (label=$LABEL)"
    elif [ "$READY" -eq "$TOTAL_C" ]; then
        log_pass "$NAME: $READY/$TOTAL_C Running"
    else
        log_fail "$NAME: $READY/$TOTAL_C Running"
    fi
done

# ============================================================================
# SECTION 3: PVC 检查
# ============================================================================
section "3/7  PersistentVolumeClaim 检查"

PVC_TOTAL=$(kubectl get pvc -n "$NAMESPACE" --no-headers 2>/dev/null | wc -l | tr -d ' ')
PVC_BOUND=$(kubectl get pvc -n "$NAMESPACE" --no-headers 2>/dev/null | grep -c "Bound" || echo 0)
PVC_BOUND=$(echo "$PVC_BOUND" | tr -d ' ')
PVC_PENDING=$(kubectl get pvc -n "$NAMESPACE" --no-headers 2>/dev/null | grep -c "Pending" || echo 0)
PVC_PENDING=$(echo "$PVC_PENDING" | tr -d ' ')

log_info "PVC 总量: $PVC_TOTAL | Bound: $PVC_BOUND | Pending: $PVC_PENDING"

if [ "$PVC_PENDING" -eq 0 ] && [ "$PVC_TOTAL" -gt 0 ]; then
    log_pass "所有 PVC ($PVC_BOUND) 已绑定"
else
    if [ "$PVC_PENDING" -gt 0 ]; then
        log_fail "$PVC_PENDING 个 PVC 未绑定"
        kubectl get pvc -n "$NAMESPACE" --no-headers | grep "Pending" | while read -r line; do
            echo -e "  ${RED}↳ $(echo "$line" | awk '{print $1, $2, $4}')${NC}"
        done
    fi
fi

# PVC 详情
kubectl get pvc -n "$NAMESPACE" --no-headers 2>/dev/null | while read -r line; do
    PVC_NAME=$(echo "$line" | awk '{print $1}')
    PVC_STATUS=$(echo "$line" | awk '{print $2}')
    PVC_SIZE=$(echo "$line" | awk '{print $4}')
    if [ "$PVC_STATUS" = "Bound" ]; then
        log_pass "PVC $PVC_NAME: $PVC_STATUS ($PVC_SIZE)"
    else
        log_fail "PVC $PVC_NAME: $PVC_STATUS"
    fi
done

# ============================================================================
# SECTION 4: Service & Endpoints 检查
# ============================================================================
section "4/7  Service 端点检查"

SERVICES=(
    "kube-llmops-litellm:4000:LiteLLM"
    "kube-llmops-litellm-pg:5432:PostgreSQL"
    "kube-llmops-langfuse:3000:Langfuse"
    "kube-llmops-grafana:3000:Grafana"
    "kube-llmops-prometheus:9090:Prometheus"
    "kube-llmops-keycloak:8080:Keycloak"
    "kube-llmops-minio:9000:MinIO"
    "kube-llmops-dify-api:5001:Dify-API"
    "kube-llmops-llm-guard:8000:LLM-Guard"
    "kube-llmops-loki:3100:Loki"
    # Phase 4 新增
    "kube-llmops-lightrag:9621:LightRAG"
    "kube-llmops-neo4j:7687:Neo4j-Bolt"
    "kube-llmops-milvus:19530:Milvus"
    "kube-llmops-presidio-analyzer:3000:Presidio-Analyzer"
    "kube-llmops-presidio-anonymizer:3000:Presidio-Anonymizer"
    # 模型服务
    "vllm-qwen2-5-0-5b:8000:vLLM"
    "tei-bge-small-en:8080:TEI-Embed"
    "tei-bge-reranker-base:8080:TEI-Reranker"
)

for svc in "${SERVICES[@]}"; do
    SVC_NAME="${svc%%:*}"
    REST="${svc#*:}"
    SVC_PORT="${REST%%:*}"
    SVC_LABEL="${REST##*:}"
    EP_COUNT=$(kubectl get endpoints "$SVC_NAME" -n "$NAMESPACE" -o jsonpath='{.subsets[*].addresses}' 2>/dev/null | grep -c "ip" || echo 0)
    if [ "$EP_COUNT" -ge 1 ]; then
        log_pass "$SVC_LABEL ($SVC_NAME:$SVC_PORT): $EP_COUNT endpoint(s)"
    else
        SVC_EXISTS=$(kubectl get svc "$SVC_NAME" -n "$NAMESPACE" --no-headers 2>/dev/null | wc -l)
        if [ "$SVC_EXISTS" -eq 0 ]; then
            log_warn "$SVC_LABEL ($SVC_NAME): Service 不存在"
        else
            log_fail "$SVC_LABEL ($SVC_NAME:$SVC_PORT): 无可用 endpoint"
        fi
    fi
done

# ============================================================================
# SECTION 5: GPU 资源分配检查
# ============================================================================
section "5/7  GPU 资源分配"

# vLLM GPU 分配
VLLM_PODS=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=vllm -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.containers[0].resources.requests.nvidia\.com/gpu}{"\t"}{.spec.containers[0].resources.limits.nvidia\.com/gpu}{"\t"}{.spec.nodeName}{"\n"}{end}' 2>/dev/null)

if [ -n "$VLLM_PODS" ]; then
    echo "$VLLM_PODS" | while IFS=$'\t' read -r pod req lim node; do
        if [ -n "$req" ] && [ "$req" != "0" ]; then
            log_pass "vLLM $pod: GPU requests=$req limits=$lim node=$node"
        else
            log_fail "vLLM $pod: 未请求 GPU 资源"
        fi
    done
else
    log_warn "未找到 vLLM Pod"
fi

# TEI 应为 CPU-only
TEI_GPU=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=tei -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.containers[0].resources.requests.nvidia\.com/gpu}{"\n"}{end}' 2>/dev/null)
if [ -n "$TEI_GPU" ]; then
    echo "$TEI_GPU" | while IFS=$'\t' read -r pod gpu; do
        if [ -z "$gpu" ] || [ "$gpu" = "0" ]; then
            log_pass "TEI $pod: CPU-only (无 GPU 请求) — 符合预期"
        else
            log_info "TEI $pod: 使用 GPU=$gpu"
        fi
    done
fi

# 集群 GPU 使用率
GPU_REQUESTED=$(kubectl get pods -n "$NAMESPACE" -o jsonpath='{range .items[*]}{.spec.containers[*].resources.requests.nvidia\.com/gpu}{"\n"}{end}' 2>/dev/null | awk '{s+=$1} END{print s+0}')
log_info "集群 GPU 使用率: $GPU_REQUESTED / $GPU_ALLOCATABLE"

# ============================================================================
# SECTION 6: 健康端点检查
# ============================================================================
section "6/7  健康端点检查 (通过临时 Pod)"

# 启动一个临时 curl Pod 用于健康检查
log_info "启动临时 health-checker Pod..."
kubectl delete pod health-checker -n "$NAMESPACE" --ignore-not-found=true --grace-period=0 --force 2>/dev/null || true
sleep 2
kubectl run health-checker -n "$NAMESPACE" --image=curlimages/curl:8.12.1 --restart=Never --command -- sleep 300 2>/dev/null || true
kubectl wait --for=condition=Ready pod/health-checker -n "$NAMESPACE" --timeout=60s 2>/dev/null || true
CURL_POD="health-checker"

HEALTH_CHECKS=(
    "http://kube-llmops-litellm:4000/health/liveliness:LiteLLM"
    "http://vllm-qwen2-5-0-5b:8000/health:vLLM"
    "http://tei-bge-small-en:8080/health:TEI-Embed"
    "http://tei-bge-reranker-base:8080/health:TEI-Reranker"
    "http://kube-llmops-langfuse:3000/api/public/health:Langfuse"
    "http://kube-llmops-grafana:3000/api/health:Grafana"
    "http://kube-llmops-prometheus:9090/-/ready:Prometheus"
    "http://kube-llmops-minio:9000/minio/health/ready:MinIO"
    "http://kube-llmops-dify-api:5001/health:Dify-API"
    "http://kube-llmops-llm-guard:8000/healthz:LLM-Guard"
    "http://kube-llmops-loki:3100/ready:Loki"
    # Phase 4 新增
    "http://kube-llmops-neo4j:7474:Neo4j"
    "http://kube-llmops-milvus:9091/healthz:Milvus"
    "http://kube-llmops-presidio-analyzer:3000/health:Presidio-Analyzer"
    "http://kube-llmops-presidio-anonymizer:3000/health:Presidio-Anonymizer"
)

for check in "${HEALTH_CHECKS[@]}"; do
    URL="${check%%:*}:${check#*:}"
    URL="${check%:*}"
    LABEL="${check##*:}"
    # Extract URL properly
    URL=$(echo "$check" | rev | cut -d: -f2- | rev)
    LABEL=$(echo "$check" | rev | cut -d: -f1 | rev)

    HTTP_CODE=$(kubectl exec -n "$NAMESPACE" "$CURL_POD" -- curl -s -o /dev/null -w '%{http_code}' --connect-timeout 5 --max-time 10 "$URL" 2>/dev/null || echo "000")
    if [ "$HTTP_CODE" = "200" ]; then
        log_pass "$LABEL 健康检查: HTTP $HTTP_CODE"
    elif [ "$HTTP_CODE" = "000" ]; then
        log_fail "$LABEL 健康检查: 连接超时/不可达"
    else
        log_warn "$LABEL 健康检查: HTTP $HTTP_CODE"
    fi
done

# 清理临时 Pod
kubectl delete pod health-checker -n "$NAMESPACE" --grace-period=0 --force 2>/dev/null || true

# ============================================================================
# SECTION 7: Helm Hook (Smoke Test) 检查
# ============================================================================
section "7/7  Helm Hook & Smoke Test"

# 检查 smoke test job
SMOKE_JOBS=$(kubectl get jobs -n "$NAMESPACE" -l app.kubernetes.io/component=smoke-test --no-headers 2>/dev/null)
if [ -n "$SMOKE_JOBS" ]; then
    SMOKE_STATUS=$(echo "$SMOKE_JOBS" | awk '{print $2}')
    SMOKE_NAME=$(echo "$SMOKE_JOBS" | awk '{print $1}')
    if echo "$SMOKE_STATUS" | grep -q "1/1"; then
        log_pass "Smoke Test Job ($SMOKE_NAME): 已完成"
    else
        log_warn "Smoke Test Job ($SMOKE_NAME): 状态=$SMOKE_STATUS"
    fi
else
    # 检查 hook 型 job (可能已被 hook-delete-policy 清理)
    SMOKE_PODS=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=smoke-test --no-headers 2>/dev/null)
    if [ -n "$SMOKE_PODS" ]; then
        SMOKE_POD_STATUS=$(echo "$SMOKE_PODS" | awk '{print $3}' | head -1)
        if [ "$SMOKE_POD_STATUS" = "Completed" ]; then
            log_pass "Smoke Test Pod: Completed"
        else
            log_warn "Smoke Test Pod 状态: $SMOKE_POD_STATUS"
        fi
    else
        log_warn "未找到 Smoke Test Job/Pod (可能已被清理)"
    fi
fi

# ============================================================================
# 测试报告汇总
# ============================================================================
section "测试报告汇总"

echo -e "\n${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "  总测试: $TOTAL | ${GREEN}通过: $PASS${NC} | ${RED}失败: $FAIL${NC} | ${YELLOW}警告: $WARN${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

echo "" >> "$REPORT_FILE"
echo "==========================================" >> "$REPORT_FILE"
echo "SUMMARY: Total=$TOTAL Pass=$PASS Fail=$FAIL Warn=$WARN" >> "$REPORT_FILE"
echo "Report saved to: $REPORT_FILE"

if [ "$FAIL" -gt 0 ]; then
    echo -e "\n${RED}部署验证未通过，请检查上述 FAIL 项${NC}"
    exit 1
else
    echo -e "\n${GREEN}部署验证通过！${NC}"
    exit 0
fi
