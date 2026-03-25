# /// script
# requires-python = ">=3.10"
# dependencies = [
#     "kubernetes>=31.0.0",
#     "rich>=14.0.0",
# ]
# ///
"""
kube-llmops K8s 资源自动化测试脚本
用途: 使用 kubernetes Python SDK 验证 PVC、GPU、Pod 状态、资源配额、健康端点
用法: uv run scripts/02-k8s-resource-test.py [--namespace default] [--release kube-llmops]
"""

import argparse
import json
import sys
import time
import urllib.request
import urllib.error
from dataclasses import dataclass, field
from typing import Optional

from kubernetes import client, config
from rich.console import Console
from rich.table import Table

console = Console()

# ---------- 测试结果收集 ----------
@dataclass
class TestResult:
    name: str
    status: str  # PASS / FAIL / WARN / SKIP
    detail: str = ""

@dataclass
class TestSuite:
    results: list[TestResult] = field(default_factory=list)

    def add(self, name: str, status: str, detail: str = ""):
        self.results.append(TestResult(name=name, status=status, detail=detail))
        icon = {"PASS": "[green]PASS[/]", "FAIL": "[red]FAIL[/]", "WARN": "[yellow]WARN[/]", "SKIP": "[dim]SKIP[/]"}
        console.print(f"  {icon.get(status, status)}  {name}  {detail}")

    @property
    def passed(self): return sum(1 for r in self.results if r.status == "PASS")
    @property
    def failed(self): return sum(1 for r in self.results if r.status == "FAIL")
    @property
    def warned(self): return sum(1 for r in self.results if r.status == "WARN")

    def summary(self):
        table = Table(title="kube-llmops Infra Test Summary")
        table.add_column("Metric", style="bold")
        table.add_column("Value")
        table.add_row("Total", str(len(self.results)))
        table.add_row("Passed", f"[green]{self.passed}[/]")
        table.add_row("Failed", f"[red]{self.failed}[/]")
        table.add_row("Warnings", f"[yellow]{self.warned}[/]")
        console.print(table)

        if self.failed > 0:
            console.print("\n[red bold]FAILED TESTS:[/]")
            for r in self.results:
                if r.status == "FAIL":
                    console.print(f"  [red]x[/] {r.name}: {r.detail}")

suite = TestSuite()


def load_kube_config():
    """加载 kubeconfig"""
    try:
        config.load_incluster_config()
    except config.ConfigException:
        config.load_kube_config()


# ============================================================================
# TEST GROUP 1: Pod 状态验证
# ============================================================================
def test_pod_status(v1: client.CoreV1Api, namespace: str, release: str):
    console.rule("[bold blue]1. Pod 状态验证")

    pods = v1.list_namespaced_pod(namespace=namespace)

    # 统计
    running = [p for p in pods.items if p.status.phase == "Running"]
    completed = [p for p in pods.items if p.status.phase == "Succeeded"]
    problem = [p for p in pods.items if p.status.phase not in ("Running", "Succeeded")]

    suite.add("Pod 总数 > 0", "PASS" if len(pods.items) > 0 else "FAIL",
              f"共 {len(pods.items)} 个 Pod")

    if not problem:
        suite.add("所有 Pod 状态正常", "PASS",
                   f"Running={len(running)}, Completed={len(completed)}")
    else:
        for p in problem:
            reason = ""
            if p.status.container_statuses:
                cs = p.status.container_statuses[0]
                if cs.state.waiting:
                    reason = cs.state.waiting.reason or ""
                elif cs.state.terminated:
                    reason = cs.state.terminated.reason or ""
            suite.add(f"Pod 异常: {p.metadata.name}", "FAIL",
                       f"phase={p.status.phase} reason={reason}")

    # 关键组件检查
    critical = {
        "postgresql": {"label": "app.kubernetes.io/name=litellm-postgresql", "min": 1},
        "litellm":    {"label": "app.kubernetes.io/name=litellm", "min": 1},
        "vllm":       {"label": "app.kubernetes.io/name=vllm", "min": 1},
        "langfuse":   {"label": "app.kubernetes.io/name=langfuse", "min": 1},
        "dify-api":   {"label": "app.kubernetes.io/name=dify-api", "min": 1},
        "grafana":    {"label": "app.kubernetes.io/name=grafana", "min": 1},
        "prometheus":  {"label": "app.kubernetes.io/name=prometheus", "min": 1},
        # Phase 4 新增
        "lightrag":   {"label": "app.kubernetes.io/name=lightrag", "min": 1},
        "neo4j":      {"label": "app.kubernetes.io/name=neo4j", "min": 1},
        "milvus":     {"label": "app.kubernetes.io/name=milvus", "min": 1},
        "presidio-analyzer":  {"label": "app.kubernetes.io/name=presidio-analyzer", "min": 1},
        "presidio-anonymizer": {"label": "app.kubernetes.io/name=presidio-anonymizer", "min": 1},
    }

    for name, spec in critical.items():
        label_sel = spec["label"]
        matched = v1.list_namespaced_pod(namespace=namespace, label_selector=label_sel)
        ready = [p for p in matched.items
                 if p.status.phase == "Running"
                 and all(c.ready for c in (p.status.container_statuses or []))]
        if len(ready) >= spec["min"]:
            suite.add(f"组件 {name} Ready", "PASS", f"{len(ready)}/{len(matched.items)}")
        elif matched.items:
            suite.add(f"组件 {name} Ready", "FAIL",
                       f"{len(ready)}/{len(matched.items)} Ready")
        else:
            suite.add(f"组件 {name} 存在", "WARN", "未找到匹配的 Pod")

    # CrashLoopBackOff 检测
    crash_pods = []
    for p in pods.items:
        if p.status.container_statuses:
            for cs in p.status.container_statuses:
                if cs.restart_count > 5:
                    crash_pods.append((p.metadata.name, cs.restart_count))
    if crash_pods:
        for name, restarts in crash_pods:
            suite.add(f"Pod 频繁重启: {name}", "WARN", f"重启 {restarts} 次")
    else:
        suite.add("无 Pod 频繁重启 (restart > 5)", "PASS")


# ============================================================================
# TEST GROUP 2: PVC 状态验证
# ============================================================================
def test_pvc_status(v1: client.CoreV1Api, namespace: str):
    console.rule("[bold blue]2. PVC 持久卷验证")

    pvcs = v1.list_namespaced_persistent_volume_claim(namespace=namespace)

    if not pvcs.items:
        suite.add("PVC 存在", "WARN", "未找到任何 PVC")
        return

    suite.add("PVC 总数 > 0", "PASS", f"共 {len(pvcs.items)} 个 PVC")

    bound = [p for p in pvcs.items if p.status.phase == "Bound"]
    pending = [p for p in pvcs.items if p.status.phase == "Pending"]

    if not pending:
        suite.add("所有 PVC 已绑定", "PASS", f"Bound={len(bound)}")
    else:
        for p in pending:
            suite.add(f"PVC 未绑定: {p.metadata.name}", "FAIL",
                       f"phase={p.status.phase} sc={p.spec.storage_class_name}")

    # 关键 PVC 检查
    expected_pvcs = [
        "litellm-pg-data",
        "prometheus-data",
        "grafana-data",
        "minio",
        "vllm-qwen2-5-0-5b-cache",
    ]
    for pvc_name_part in expected_pvcs:
        matched = [p for p in pvcs.items if pvc_name_part in p.metadata.name]
        if matched:
            pvc = matched[0]
            storage = pvc.spec.resources.requests.get("storage", "unknown")
            suite.add(f"PVC {pvc_name_part}", "PASS" if pvc.status.phase == "Bound" else "FAIL",
                       f"phase={pvc.status.phase} size={storage}")
        else:
            suite.add(f"PVC {pvc_name_part}", "WARN", "未找到")


# ============================================================================
# TEST GROUP 3: GPU 资源验证
# ============================================================================
def test_gpu_allocation(v1: client.CoreV1Api, namespace: str):
    console.rule("[bold blue]3. GPU 资源分配验证")

    # 节点 GPU
    nodes = v1.list_node()
    total_gpu = 0
    for node in nodes.items:
        alloc = node.status.allocatable or {}
        gpu = int(alloc.get("nvidia.com/gpu", "0"))
        total_gpu += gpu
        if gpu > 0:
            suite.add(f"节点 {node.metadata.name} GPU", "PASS", f"allocatable={gpu}")

    if total_gpu == 0:
        suite.add("集群 GPU 资源", "WARN", "无 GPU 可用")
        return

    suite.add("集群 GPU 总量", "PASS", f"total={total_gpu}")

    # vLLM Pod GPU 分配
    vllm_pods = v1.list_namespaced_pod(namespace=namespace,
                                        label_selector="app.kubernetes.io/name=vllm")
    for pod in vllm_pods.items:
        for container in pod.spec.containers:
            req = container.resources.requests or {}
            lim = container.resources.limits or {}
            gpu_req = req.get("nvidia.com/gpu", "0")
            gpu_lim = lim.get("nvidia.com/gpu", "0")
            if int(gpu_req) > 0:
                suite.add(f"vLLM {pod.metadata.name} GPU 请求", "PASS",
                           f"requests={gpu_req} limits={gpu_lim}")
            else:
                suite.add(f"vLLM {pod.metadata.name} GPU 请求", "FAIL",
                           "未请求 GPU")

    # TEI 应为 CPU-only
    tei_pods = v1.list_namespaced_pod(namespace=namespace,
                                       label_selector="app.kubernetes.io/name=tei")
    for pod in tei_pods.items:
        for container in pod.spec.containers:
            req = container.resources.requests or {}
            gpu_req = req.get("nvidia.com/gpu", "0")
            suite.add(f"TEI {pod.metadata.name} CPU-only", "PASS" if int(gpu_req) == 0 else "WARN",
                       f"gpu={gpu_req}")

    # GPU 超分检查
    gpu_requested = 0
    pods = v1.list_namespaced_pod(namespace=namespace)
    for pod in pods.items:
        if pod.status.phase not in ("Running", "Pending"):
            continue
        for container in pod.spec.containers:
            req = container.resources.requests or {}
            gpu_requested += int(req.get("nvidia.com/gpu", "0"))

    if gpu_requested <= total_gpu:
        suite.add("GPU 无超分", "PASS", f"requested={gpu_requested} / allocatable={total_gpu}")
    else:
        suite.add("GPU 超分", "WARN", f"requested={gpu_requested} > allocatable={total_gpu}")


# ============================================================================
# TEST GROUP 4: QoS Class 验证
# ============================================================================
def test_qos_class(v1: client.CoreV1Api, namespace: str):
    console.rule("[bold blue]4. Pod QoS Class 验证")

    pods = v1.list_namespaced_pod(namespace=namespace)

    best_effort = []
    for pod in pods.items:
        if pod.status.phase != "Running":
            continue
        qos = pod.status.qos_class
        if qos == "BestEffort":
            best_effort.append(pod.metadata.name)

    if not best_effort:
        suite.add("无 BestEffort QoS Pod", "PASS", "所有运行中 Pod 有资源约束")
    else:
        for name in best_effort:
            suite.add(f"Pod {name} QoS=BestEffort", "WARN", "容易被驱逐")


# ============================================================================
# TEST GROUP 5: Service 连通性验证
# ============================================================================
def test_service_endpoints(v1: client.CoreV1Api, namespace: str):
    console.rule("[bold blue]5. Service Endpoints 验证")

    services_to_check = [
        "kube-llmops-litellm",
        "kube-llmops-litellm-pg",
        "kube-llmops-langfuse",
        "kube-llmops-grafana",
        "kube-llmops-prometheus",
        "kube-llmops-keycloak",
        "kube-llmops-minio",
        "kube-llmops-dify-api",
        "kube-llmops-llm-guard",
        "kube-llmops-loki",
        # Phase 4
        "kube-llmops-lightrag",
        "kube-llmops-neo4j",
        "kube-llmops-milvus",
        "kube-llmops-presidio-analyzer",
        "kube-llmops-presidio-anonymizer",
        # Model serving
        "vllm-qwen2-5-0-5b",
        "tei-bge-small-en",
        "tei-bge-reranker-base",
    ]

    for svc_name in services_to_check:
        try:
            ep = v1.read_namespaced_endpoints(name=svc_name, namespace=namespace)
            addrs = []
            if ep.subsets:
                for subset in ep.subsets:
                    if subset.addresses:
                        addrs.extend(subset.addresses)
            if addrs:
                suite.add(f"Service {svc_name}", "PASS", f"{len(addrs)} endpoint(s)")
            else:
                suite.add(f"Service {svc_name}", "FAIL", "无可用 endpoint")
        except client.ApiException as e:
            if e.status == 404:
                suite.add(f"Service {svc_name}", "WARN", "Service 不存在")
            else:
                suite.add(f"Service {svc_name}", "FAIL", f"API 错误: {e.status}")


# ============================================================================
# TEST GROUP 6: 健康端点验证 (通过临时 curl Pod)
# ============================================================================
def test_health_endpoints(v1: client.CoreV1Api, namespace: str):
    console.rule("[bold blue]6. 健康端点验证 (in-cluster via curl pod)")

    import subprocess

    # 创建临时 curl Pod
    try:
        v1.delete_namespaced_pod("health-checker", namespace, grace_period_seconds=0)
    except client.ApiException:
        pass
    time.sleep(2)

    subprocess.run(
        ["kubectl", "run", "health-checker", "-n", namespace,
         "--image=curlimages/curl:8.12.1", "--restart=Never",
         "--command", "--", "sleep", "300"],
        capture_output=True, timeout=10
    )
    subprocess.run(
        ["kubectl", "wait", "--for=condition=Ready", "pod/health-checker",
         "-n", namespace, "--timeout=60s"],
        capture_output=True, timeout=70
    )

    health_endpoints = {
        "LiteLLM":      "http://kube-llmops-litellm:4000/health/liveliness",
        "vLLM":         "http://vllm-qwen2-5-0-5b:8000/health",
        "TEI-Embed":    "http://tei-bge-small-en:8080/health",
        "TEI-Reranker": "http://tei-bge-reranker-base:8080/health",
        "Langfuse":     "http://kube-llmops-langfuse:3000/api/public/health",
        "Grafana":      "http://kube-llmops-grafana:3000/api/health",
        "Prometheus":   "http://kube-llmops-prometheus:9090/-/ready",
        "MinIO":        "http://kube-llmops-minio:9000/minio/health/ready",
        "Dify-API":     "http://kube-llmops-dify-api:5001/health",
        "LLM-Guard":    "http://kube-llmops-llm-guard:8000/healthz",
        "Loki":         "http://kube-llmops-loki:3100/ready",
        # Phase 4
        "Neo4j":        "http://kube-llmops-neo4j:7474",
        "Milvus":       "http://kube-llmops-milvus:9091/healthz",
        "Presidio-Analyzer":   "http://kube-llmops-presidio-analyzer:3000/health",
        "Presidio-Anonymizer": "http://kube-llmops-presidio-anonymizer:3000/health",
    }

    for label, url in health_endpoints.items():
        try:
            result = subprocess.run(
                ["kubectl", "exec", "-n", namespace, "health-checker", "--",
                 "curl", "-s", "-o", "/dev/null", "-w", "%{http_code}",
                 "--connect-timeout", "5", "--max-time", "10", url],
                capture_output=True, text=True, timeout=20
            )
            code = result.stdout.strip()
            if code == "200":
                suite.add(f"健康检查 {label}", "PASS", f"HTTP {code}")
            else:
                suite.add(f"健康检查 {label}", "FAIL", f"HTTP {code}")
        except Exception as e:
            suite.add(f"健康检查 {label}", "WARN", f"exec 失败: {str(e)[:80]}")

    # 清理
    try:
        subprocess.run(
            ["kubectl", "delete", "pod", "health-checker", "-n", namespace,
             "--grace-period=0", "--force"],
            capture_output=True, timeout=15
        )
    except Exception:
        pass


# ============================================================================
# TEST GROUP 7: AI 功能验证
# ============================================================================
def test_ai_functionality(v1: client.CoreV1Api, namespace: str):
    console.rule("[bold blue]7. AI 功能验证 (Embedding + LLM)")

    import subprocess

    # 创建临时 curl Pod
    try:
        v1.delete_namespaced_pod("ai-tester", namespace, grace_period_seconds=0)
    except client.ApiException:
        pass
    time.sleep(2)

    subprocess.run(
        ["kubectl", "run", "ai-tester", "-n", namespace,
         "--image=curlimages/curl:8.12.1", "--restart=Never",
         "--command", "--", "sleep", "300"],
        capture_output=True, timeout=10
    )
    subprocess.run(
        ["kubectl", "wait", "--for=condition=Ready", "pod/ai-tester",
         "-n", namespace, "--timeout=60s"],
        capture_output=True, timeout=70
    )

    # Embedding 测试
    try:
        embed_data = json.dumps({"model": "bge-small-en", "input": "test query"})
        result = subprocess.run(
            ["kubectl", "exec", "-n", namespace, "ai-tester", "--",
             "curl", "-s", "-X", "POST",
             "http://kube-llmops-litellm:4000/v1/embeddings",
             "-H", "Authorization: Bearer sk-kube-llmops-dev",
             "-H", "Content-Type: application/json",
             "-d", embed_data,
             "--max-time", "30"],
            capture_output=True, text=True, timeout=40
        )
        data = json.loads(result.stdout)
        dim = len(data["data"][0]["embedding"])
        if dim == 384:
            suite.add("Embedding 生成", "PASS", f"维度={dim}")
        else:
            suite.add("Embedding 生成", "WARN", f"维度={dim} (预期 384)")
    except Exception as e:
        suite.add("Embedding 生成", "FAIL", str(e)[:100])

    # LLM 推理测试
    try:
        llm_data = json.dumps({
            "model": "qwen2-5-0-5b",
            "messages": [{"role": "user", "content": "What is 2+2? Answer with just the number."}],
            "max_tokens": 10
        })
        result = subprocess.run(
            ["kubectl", "exec", "-n", namespace, "ai-tester", "--",
             "curl", "-s", "-X", "POST",
             "http://kube-llmops-litellm:4000/v1/chat/completions",
             "-H", "Authorization: Bearer sk-kube-llmops-dev",
             "-H", "Content-Type: application/json",
             "-d", llm_data,
             "--max-time", "120"],
            capture_output=True, text=True, timeout=130
        )
        data = json.loads(result.stdout)
        content = data["choices"][0]["message"]["content"]
        if "4" in content:
            suite.add("LLM 推理", "PASS", f"回答: {content.strip()[:50]}")
        else:
            suite.add("LLM 推理", "WARN", f"回答可能不正确: {content.strip()[:50]}")
    except Exception as e:
        suite.add("LLM 推理", "FAIL", str(e)[:100])

    # 清理
    try:
        subprocess.run(
            ["kubectl", "delete", "pod", "ai-tester", "-n", namespace,
             "--grace-period=0", "--force"],
            capture_output=True, timeout=15
        )
    except Exception:
        pass


# ============================================================================
# TEST GROUP 8: 数据持久化验证
# ============================================================================
def test_data_persistence(v1: client.CoreV1Api, namespace: str):
    console.rule("[bold blue]8. 数据持久化验证")

    # 检查 PostgreSQL 数据库
    pg_pods = v1.list_namespaced_pod(
        namespace=namespace, label_selector="app.kubernetes.io/name=litellm-postgresql"
    )
    if not pg_pods.items:
        suite.add("PostgreSQL 数据持久化", "SKIP", "无 PostgreSQL Pod")
        return

    pg_pod = pg_pods.items[0].metadata.name
    from kubernetes.stream import stream as k8s_stream

    try:
        resp = k8s_stream(
            v1.connect_get_namespaced_pod_exec,
            pg_pod, namespace,
            command=["psql", "-U", "litellm", "-d", "litellm",
                     "-t", "-c", "SELECT datname FROM pg_database WHERE datistemplate = false ORDER BY datname;"],
            stderr=True, stdin=False, stdout=True, tty=False,
        )
        databases = [db.strip() for db in resp.strip().split("\n") if db.strip()]
        expected_dbs = {"litellm", "langfuse", "dify", "dify_plugin"}
        found_dbs = set(databases)
        missing = expected_dbs - found_dbs
        if not missing:
            suite.add("PostgreSQL 数据库完整", "PASS", f"databases={databases}")
        else:
            suite.add("PostgreSQL 数据库完整", "WARN", f"缺少: {missing}")
    except Exception as e:
        suite.add("PostgreSQL 数据库检查", "FAIL", str(e)[:100])

    # 检查 PVC 挂载
    pods = v1.list_namespaced_pod(namespace=namespace)
    pvc_mounts = []
    for pod in pods.items:
        if pod.status.phase != "Running":
            continue
        if pod.spec.volumes:
            for vol in pod.spec.volumes:
                if vol.persistent_volume_claim:
                    pvc_mounts.append(
                        f"{pod.metadata.name} -> {vol.persistent_volume_claim.claim_name}"
                    )

    suite.add("PVC 挂载关系", "PASS" if pvc_mounts else "WARN",
               f"{len(pvc_mounts)} 个 PVC 挂载")


# ============================================================================
# 主入口
# ============================================================================
def main():
    parser = argparse.ArgumentParser(description="kube-llmops K8s Resource Tests")
    parser.add_argument("--namespace", "-n", default="default")
    parser.add_argument("--release", "-r", default="kube-llmops")
    parser.add_argument("--skip-ai", action="store_true", help="跳过 AI 功能测试")
    args = parser.parse_args()

    console.rule("[bold green]kube-llmops Infrastructure Test Suite")
    console.print(f"Namespace: {args.namespace} | Release: {args.release}")
    console.print(f"Time: {time.strftime('%Y-%m-%d %H:%M:%S')}\n")

    load_kube_config()
    v1 = client.CoreV1Api()

    test_pod_status(v1, args.namespace, args.release)
    test_pvc_status(v1, args.namespace)
    test_gpu_allocation(v1, args.namespace)
    test_qos_class(v1, args.namespace)
    test_service_endpoints(v1, args.namespace)
    test_health_endpoints(v1, args.namespace)

    if not args.skip_ai:
        test_ai_functionality(v1, args.namespace)

    test_data_persistence(v1, args.namespace)

    console.print()
    suite.summary()

    sys.exit(1 if suite.failed > 0 else 0)


if __name__ == "__main__":
    main()
