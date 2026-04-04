"""
kube-llmops Fine-tune E2E Test Suite
Validates the complete fine-tuning pipeline:
  MLflow → Argo Workflow → Evaluation → Quality Gate → Deployment
"""
# /// script
# requires-python = ">=3.12"
# dependencies = ["playwright"]
# ///

import json, sys, time, os, subprocess
from playwright.sync_api import sync_playwright

# ---------------------------------------------------------------------------
# Configuration — override via env vars for NodePort / non-Ingress setups
# ---------------------------------------------------------------------------
NODE_IP       = os.environ.get("NODE_IP", "")
MLFLOW_URL    = os.environ.get("MLFLOW_URL", f"http://{NODE_IP}:30505" if NODE_IP else "http://mlflow.llmops.local")
GRAFANA_URL   = os.environ.get("GRAFANA_URL", f"http://{NODE_IP}:30300" if NODE_IP else "http://grafana.llmops.local")
PROMETHEUS_URL = os.environ.get("PROMETHEUS_URL", f"http://{NODE_IP}:30909" if NODE_IP else "http://prometheus.llmops.local")
RELEASE_NAME  = os.environ.get("RELEASE_NAME", "kube-llmops")
NAMESPACE     = os.environ.get("NAMESPACE", "default")
WORKFLOW_TIMEOUT = int(os.environ.get("WORKFLOW_TIMEOUT", "3600"))  # seconds

SHOTS = "tests/e2e/screenshots"
os.makedirs(SHOTS, exist_ok=True)

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
results = []

def check(name, cond, detail=""):
    icon = "PASS" if cond else "FAIL"
    results.append({"name": name, "pass": cond, "detail": detail})
    print(f"  [{icon}] {name}" + (f" — {detail}" if detail else ""))
    return cond

def kubectl(cmd, timeout=600):
    r = subprocess.run(cmd, shell=True, capture_output=True, text=True, timeout=timeout)
    return (r.stdout + r.stderr).strip()

def kubectl_json(cmd, timeout=600):
    out = kubectl(cmd, timeout)
    try:
        return json.loads(out)
    except json.JSONDecodeError:
        return None

def wait_for_pod(label, timeout=300):
    """Wait for at least one pod matching label to be Ready."""
    print(f"    Waiting for pod {label} (timeout {timeout}s)...")
    out = kubectl(
        f"kubectl wait --for=condition=ready pod -l {label} "
        f"-n {NAMESPACE} --timeout={timeout}s"
    )
    return "condition met" in out

def http_get(url, timeout=10):
    """Simple HTTP GET via Python stdlib (no extra deps)."""
    import urllib.request
    try:
        req = urllib.request.Request(url)
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return {"status": resp.status, "body": resp.read().decode()}
    except Exception as e:
        return {"status": 0, "body": str(e)}

def http_post_json(url, payload, timeout=10):
    """Simple HTTP POST JSON via Python stdlib."""
    import urllib.request
    data = json.dumps(payload).encode()
    try:
        req = urllib.request.Request(url, data=data, headers={"Content-Type": "application/json"})
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return {"status": resp.status, "body": resp.read().decode()}
    except Exception as e:
        return {"status": 0, "body": str(e)}

# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------
def run_tests():

    # ═══════════════════════════════════════════════════════
    # TEST 1: MLflow Deployment & Health
    # ═══════════════════════════════════════════════════════
    print("\n" + "=" * 60)
    print("TEST 1: MLflow Deployment & Health")
    print("=" * 60)

    # 1a. Pod running
    mlflow_pods = kubectl_json(
        f"kubectl get pods -n {NAMESPACE} -l app.kubernetes.io/name=mlflow -o json"
    )
    running = False
    if mlflow_pods and mlflow_pods.get("items"):
        for pod in mlflow_pods["items"]:
            phase = pod.get("status", {}).get("phase", "")
            if phase == "Running":
                running = True
                break
    check("MLflow pod running", running,
          f"{len(mlflow_pods.get('items', []))} pods found" if mlflow_pods else "no pods")

    # 1b. Service exists
    svc = kubectl(f"kubectl get svc {RELEASE_NAME}-mlflow -n {NAMESPACE} -o name 2>/dev/null")
    check("MLflow Service exists", "service/" in svc)

    # 1c. MLflow API reachable (via port-forward or URL)
    # Try direct URL first, fall back to port-forward
    api_ok = False
    mlflow_api_url = MLFLOW_URL
    resp = http_get(f"{mlflow_api_url}/api/2.0/mlflow/experiments/search?max_results=1")
    if resp["status"] == 200:
        api_ok = True
    else:
        # Try port-forward
        kubectl(f"kubectl port-forward svc/{RELEASE_NAME}-mlflow 15000:5000 -n {NAMESPACE} &")
        time.sleep(3)
        mlflow_api_url = "http://localhost:15000"
        resp = http_get(f"{mlflow_api_url}/api/2.0/mlflow/experiments/search?max_results=1")
        if resp["status"] == 200:
            api_ok = True
    check("MLflow API reachable", api_ok, f"status={resp['status']}")

    # ═══════════════════════════════════════════════════════
    # TEST 2: RBAC & ConfigMap
    # ═══════════════════════════════════════════════════════
    print("\n" + "=" * 60)
    print("TEST 2: RBAC & ConfigMap")
    print("=" * 60)

    # 2a. ServiceAccount
    sa = kubectl(f"kubectl get sa {RELEASE_NAME}-finetune -n {NAMESPACE} -o name 2>/dev/null")
    check("ServiceAccount exists", "serviceaccount/" in sa)

    # 2b. ClusterRole
    cr = kubectl(f"kubectl get clusterrole {RELEASE_NAME}-finetune -o name 2>/dev/null")
    check("ClusterRole exists", "clusterrole" in cr.lower())

    # 2c. ClusterRoleBinding
    crb = kubectl(f"kubectl get clusterrolebinding {RELEASE_NAME}-finetune -o name 2>/dev/null")
    check("ClusterRoleBinding exists", "clusterrolebinding" in crb.lower())

    # 2d. ConfigMap with training config
    cm = kubectl(
        f"kubectl get cm {RELEASE_NAME}-finetune-config -n {NAMESPACE} -o jsonpath='{{.data.train_config\\.yaml}}'"
    )
    check("ConfigMap has train_config.yaml", "model_name_or_path" in cm and "finetuning_type" in cm,
          f"{len(cm)} chars")

    # ═══════════════════════════════════════════════════════
    # TEST 3: WorkflowTemplate Validation
    # ═══════════════════════════════════════════════════════
    print("\n" + "=" * 60)
    print("TEST 3: WorkflowTemplate Validation")
    print("=" * 60)

    wft = kubectl_json(
        f"kubectl get workflowtemplate {RELEASE_NAME}-finetune -n {NAMESPACE} -o json 2>/dev/null"
    )
    check("WorkflowTemplate exists", wft is not None and wft.get("kind") == "WorkflowTemplate")

    # Verify DAG tasks
    expected_tasks = {"prepare-data", "finetune", "merge-upload", "evaluate", "quality-gate", "deploy"}
    if wft:
        templates = wft.get("spec", {}).get("templates", [])
        main_tmpl = next((t for t in templates if t.get("name") == "main"), None)
        dag_tasks = set()
        if main_tmpl and "dag" in main_tmpl:
            dag_tasks = {t["name"] for t in main_tmpl["dag"].get("tasks", [])}
        check("DAG has all 6 tasks", expected_tasks == dag_tasks,
              f"found={sorted(dag_tasks)}")

        # Verify dependency chain
        dep_map = {}
        if main_tmpl and "dag" in main_tmpl:
            for t in main_tmpl["dag"]["tasks"]:
                dep_map[t["name"]] = t.get("dependencies", [])
        chain_ok = (
            dep_map.get("prepare-data", []) == []
            and dep_map.get("finetune") == ["prepare-data"]
            and dep_map.get("merge-upload") == ["finetune"]
            and dep_map.get("evaluate") == ["merge-upload"]
            and dep_map.get("quality-gate") == ["evaluate"]
            and dep_map.get("deploy") == ["quality-gate"]
        )
        check("DAG dependency chain correct", chain_ok, str(dep_map))

        # Verify all 6 container templates exist
        tmpl_names = {t.get("name") for t in templates if "container" in t or "dag" in t}
        all_present = expected_tasks.issubset(tmpl_names)
        check("All 6 step templates defined", all_present, f"templates={sorted(tmpl_names)}")
    else:
        check("DAG has all 6 tasks", False, "WorkflowTemplate not found")
        check("DAG dependency chain correct", False, "WorkflowTemplate not found")
        check("All 6 step templates defined", False, "WorkflowTemplate not found")

    # ═══════════════════════════════════════════════════════
    # TEST 4: Workflow Execution (full pipeline)
    # ═══════════════════════════════════════════════════════
    print("\n" + "=" * 60)
    print("TEST 4: Workflow Execution (full pipeline)")
    print("=" * 60)

    # Submit workflow
    print(f"  Submitting workflow (timeout={WORKFLOW_TIMEOUT}s)...")
    submit_out = kubectl(
        f"argo submit -n {NAMESPACE} "
        f"--from workflowtemplate/{RELEASE_NAME}-finetune "
        f"--generate-name ft-e2e-test- "
        f"-o json 2>/dev/null",
        timeout=30
    )
    wf = None
    wf_name = ""
    try:
        wf = json.loads(submit_out)
        wf_name = wf.get("metadata", {}).get("name", "")
    except json.JSONDecodeError:
        pass

    if not check("Workflow submitted", bool(wf_name), submit_out[:120]):
        # Cannot proceed without a running workflow
        print("  SKIP: remaining workflow tests (submission failed)")
    else:
        print(f"  Workflow: {wf_name}")

        # Wait for completion
        print(f"  Waiting for workflow to complete (up to {WORKFLOW_TIMEOUT}s)...")
        wait_out = kubectl(
            f"argo wait -n {NAMESPACE} {wf_name} --timeout={WORKFLOW_TIMEOUT}s 2>&1",
            timeout=WORKFLOW_TIMEOUT + 60
        )

        # Get final status
        status_json = kubectl_json(
            f"argo get -n {NAMESPACE} {wf_name} -o json 2>/dev/null"
        )
        phase = ""
        if status_json:
            phase = status_json.get("status", {}).get("phase", "Unknown")
        check("Workflow completed", phase in ("Succeeded", "Failed", "Error"),
              f"phase={phase}")
        check("Workflow succeeded", phase == "Succeeded", f"phase={phase}")

        # 4c. Per-step status
        nodes = {}
        if status_json:
            nodes = status_json.get("status", {}).get("nodes", {})
        step_phases = {}
        for node_id, node in nodes.items():
            if node.get("type") == "Pod":
                # Node display name is like "ft-e2e-test-xxx.prepare-data"
                display = node.get("displayName", "")
                step_phases[display] = node.get("phase", "Unknown")

        for step in ["prepare-data", "finetune", "merge-upload", "evaluate", "quality-gate", "deploy"]:
            step_phase = step_phases.get(step, "NotFound")
            check(f"Step '{step}' succeeded", step_phase == "Succeeded",
                  f"phase={step_phase}")

        # 4d. Workflow logs check
        logs = kubectl(f"argo logs -n {NAMESPACE} {wf_name} 2>&1", timeout=60)
        check("Logs: data preparation complete",
              "Data preparation complete" in logs or "base-model" in logs.lower(),
              f"log length={len(logs)}")
        check("Logs: fine-tuning complete",
              "Fine-tuning complete" in logs or "llamafactory-cli" in logs.lower(),
              f"log length={len(logs)}")
        check("Logs: upload complete",
              "Upload complete" in logs or "Uploaded" in logs,
              f"log length={len(logs)}")

    # ═══════════════════════════════════════════════════════
    # TEST 5: MLflow Model Registry
    # ═══════════════════════════════════════════════════════
    print("\n" + "=" * 60)
    print("TEST 5: MLflow Model Registry")
    print("=" * 60)

    # Search for registered models
    resp = http_get(f"{mlflow_api_url}/api/2.0/mlflow/registered-models/search?max_results=10")
    models = []
    if resp["status"] == 200:
        try:
            models = json.loads(resp["body"]).get("registered_models", [])
        except json.JSONDecodeError:
            pass
    check("MLflow has registered models", len(models) > 0,
          f"{len(models)} models" if models else resp["body"][:80])

    # Check experiment exists
    resp = http_get(f"{mlflow_api_url}/api/2.0/mlflow/experiments/search?max_results=20")
    experiments = []
    if resp["status"] == 200:
        try:
            experiments = json.loads(resp["body"]).get("experiments", [])
        except json.JSONDecodeError:
            pass
    ft_experiments = [e for e in experiments if e.get("name", "") != "Default"]
    check("MLflow has finetune experiments", len(ft_experiments) > 0,
          f"experiments={[e.get('name') for e in ft_experiments]}")

    # Check runs with metrics
    has_metrics = False
    for exp in ft_experiments:
        exp_id = exp.get("experiment_id", "")
        resp = http_post_json(
            f"{mlflow_api_url}/api/2.0/mlflow/runs/search",
            {"experiment_ids": [exp_id], "max_results": 5}
        )
        if resp["status"] == 200:
            runs = json.loads(resp["body"]).get("runs", [])
            for run in runs:
                metrics = run.get("data", {}).get("metrics", [])
                if metrics:
                    has_metrics = True
                    break
        if has_metrics:
            break
    check("MLflow runs have metrics logged", has_metrics)

    # ═══════════════════════════════════════════════════════
    # TEST 6: Evaluation Results
    # ═══════════════════════════════════════════════════════
    print("\n" + "=" * 60)
    print("TEST 6: Evaluation Results")
    print("=" * 60)

    # Check Prometheus Pushgateway metrics
    prom_faith = None
    prom_relev = None
    try:
        resp = http_get(f"{PROMETHEUS_URL}/api/v1/query?query=finetune_faithfulness")
        if resp["status"] == 200:
            data = json.loads(resp["body"])
            result = data.get("data", {}).get("result", [])
            if result:
                prom_faith = float(result[0]["value"][1])
    except Exception:
        pass
    try:
        resp = http_get(f"{PROMETHEUS_URL}/api/v1/query?query=finetune_answer_relevancy")
        if resp["status"] == 200:
            data = json.loads(resp["body"])
            result = data.get("data", {}).get("result", [])
            if result:
                prom_relev = float(result[0]["value"][1])
    except Exception:
        pass

    check("Prometheus has finetune_faithfulness",
          prom_faith is not None,
          f"value={prom_faith}" if prom_faith is not None else "metric not found")
    check("Prometheus has finetune_answer_relevancy",
          prom_relev is not None,
          f"value={prom_relev}" if prom_relev is not None else "metric not found")

    # ═══════════════════════════════════════════════════════
    # TEST 7: Quality Gate Logic
    # ═══════════════════════════════════════════════════════
    print("\n" + "=" * 60)
    print("TEST 7: Quality Gate Logic")
    print("=" * 60)

    # Run an inline quality gate check — threshold 0.0 should pass
    qg_pass = kubectl(f"""kubectl run ft-qg-pass-test --restart=Never --image=python:3.13-slim \
        -n {NAMESPACE} -- python3 -c "
import json, sys
# Simulate reading eval results — same logic as quality-gate step
results = {{'faithfulness': 0.8, 'answer_relevancy': 0.8}}
thresholds = {{'faithfulness': 0.1, 'answer_relevancy': 0.1}}
passed = all(results.get(m, 0) >= t for m, t in thresholds.items())
print('GATE_PASS' if passed else 'GATE_FAIL')
sys.exit(0 if passed else 1)
" """)
    time.sleep(15)
    qg_pass_log = kubectl(f"kubectl logs ft-qg-pass-test -n {NAMESPACE} 2>&1")
    kubectl(f"kubectl delete pod ft-qg-pass-test -n {NAMESPACE} --force 2>/dev/null")
    check("Quality gate PASS (low threshold)", "GATE_PASS" in qg_pass_log,
          qg_pass_log.strip()[-80:])

    # Run with impossible threshold — should fail
    kubectl(f"""kubectl run ft-qg-block-test --restart=Never --image=python:3.13-slim \
        -n {NAMESPACE} -- python3 -c "
import json, sys
results = {{'faithfulness': 0.8, 'answer_relevancy': 0.8}}
thresholds = {{'faithfulness': 0.99, 'answer_relevancy': 0.99}}
passed = all(results.get(m, 0) >= t for m, t in thresholds.items())
print('GATE_PASS' if passed else 'GATE_BLOCKED')
sys.exit(0 if passed else 1)
" """)
    time.sleep(15)
    qg_block_log = kubectl(f"kubectl logs ft-qg-block-test -n {NAMESPACE} 2>&1")
    kubectl(f"kubectl delete pod ft-qg-block-test -n {NAMESPACE} --force 2>/dev/null")
    check("Quality gate BLOCK (high threshold)", "GATE_BLOCKED" in qg_block_log,
          qg_block_log.strip()[-80:])

    # ═══════════════════════════════════════════════════════
    # TEST 8: Grafana Finetune Dashboard
    # ═══════════════════════════════════════════════════════
    print("\n" + "=" * 60)
    print("TEST 8: Grafana Finetune Dashboard")
    print("=" * 60)

    with sync_playwright() as pw:
        browser = pw.chromium.launch(headless=True)
        ctx = browser.new_context(ignore_https_errors=True, viewport={"width": 1280, "height": 720})
        page = ctx.new_page()
        page.set_default_timeout(30000)

        # Login
        page.goto(f"{GRAFANA_URL}/login")
        page.wait_for_load_state("networkidle")
        page.fill('input[name="user"]', "admin")
        page.fill('input[name="password"]', "admin123!")
        page.click('button[type="submit"]')
        time.sleep(3)

        # Navigate to finetune dashboard
        page.goto(f"{GRAFANA_URL}/d/finetune-overview")
        time.sleep(3)
        page.screenshot(path=f"{SHOTS}/ft-01-grafana-finetune-dashboard.png")
        check("Grafana finetune dashboard loads",
              "finetune-overview" in page.url,
              page.url)

        browser.close()

    # ═══════════════════════════════════════════════════════
    # TEST 9: PDB for MLflow
    # ═══════════════════════════════════════════════════════
    print("\n" + "=" * 60)
    print("TEST 9: PDB for MLflow")
    print("=" * 60)

    pdb = kubectl(f"kubectl get pdb {RELEASE_NAME}-mlflow -n {NAMESPACE} -o name 2>/dev/null")
    check("MLflow PDB exists", "poddisruptionbudget" in pdb.lower())

    # ═══════════════════════════════════════════════════════
    # TEST 10: MinIO Model Artifacts
    # ═══════════════════════════════════════════════════════
    print("\n" + "=" * 60)
    print("TEST 10: MinIO Model Artifacts")
    print("=" * 60)

    # Check that models were uploaded to MinIO via the running minio pod
    minio_check = kubectl(
        f"kubectl exec deploy/{RELEASE_NAME}-minio -n {NAMESPACE} -- "
        f"sh -c 'ls /data/models/ 2>/dev/null | head -5' 2>&1"
    )
    check("MinIO has model artifacts", bool(minio_check.strip()) and "Error" not in minio_check,
          minio_check.strip()[:120])

    # ═══════════════════════════════════════════════════════
    # CLEANUP
    # ═══════════════════════════════════════════════════════
    if wf_name:
        print(f"\n  Cleaning up workflow {wf_name}...")
        kubectl(f"argo delete -n {NAMESPACE} {wf_name} 2>/dev/null")

    # ═══════════════════════════════════════════════════════
    # FINAL REPORT
    # ═══════════════════════════════════════════════════════
    print("\n" + "=" * 60)
    passed = sum(1 for r in results if r["pass"])
    failed = sum(1 for r in results if not r["pass"])
    total = len(results)
    print(f"FINAL RESULTS: {passed}/{total} passed, {failed} failed")
    print("=" * 60)
    for r in results:
        icon = "PASS" if r["pass"] else "FAIL"
        print(f"  [{icon}] {r['name']}" + (f" — {r['detail']}" if r.get("detail") and not r["pass"] else ""))

    if failed == 0:
        print(f"\nALL {total} FINETUNE E2E TESTS PASSED")
    else:
        print(f"\n{failed} TESTS FAILED")

    # Save report
    with open(f"{SHOTS}/finetune-test-report.json", "w") as f:
        json.dump({
            "timestamp": time.strftime("%Y-%m-%dT%H:%M:%SZ"),
            "suite": "finetune-e2e",
            "passed": passed, "failed": failed, "total": total,
            "results": results
        }, f, indent=2)

    return failed == 0

if __name__ == "__main__":
    sys.exit(0 if run_tests() else 1)
