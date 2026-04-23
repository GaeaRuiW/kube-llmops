"""
Helm template unit tests for the 4-tier GPU monitoring dashboards (v1.0+).

Validates:
- DCGM exporter --kubernetes flag + kubelet socket mount + privileged + hostPID
- Prometheus external_labels.cluster is present
- All 4 dashboard JSONs (gpu-cluster, gpu-node, gpu-gpu, gpu-pod) are rendered
- Dashboard JSON files parse and have the expected UIDs and variables
- Drill-down URL templates are present in the right tiers

Usage:
    python -m pytest tests/helm/test_gpu_dashboards.py -v
"""

import json
import subprocess
from pathlib import Path

import pytest
import yaml

CHART_DIR = Path(__file__).parent.parent.parent / "charts" / "kube-llmops-stack"
VALUES_FILE = CHART_DIR / "values-single-node.yaml"
DASHBOARDS_DIR = CHART_DIR / "charts" / "observability" / "dashboards"


def helm_template(set_values=None):
    cmd = ["helm", "template", "test", str(CHART_DIR), "-f", str(VALUES_FILE)]
    for k, v in (set_values or {}).items():
        cmd += ["--set", f"{k}={v}"]
    result = subprocess.run(cmd, capture_output=True, text=True, timeout=60)
    if result.returncode != 0:
        raise RuntimeError(f"helm template failed: {result.stderr}")
    docs = [d for d in yaml.safe_load_all(result.stdout) if d]
    return docs


def find_resource(docs, kind, name_suffix):
    for d in docs:
        if d.get("kind") == kind and d.get("metadata", {}).get("name", "").endswith(name_suffix):
            return d
    return None


# ──────────────────────────────── DCGM --kubernetes ────────────────────────────────


def test_dcgm_exporter_has_kubernetes_flag():
    docs = helm_template()
    ds = find_resource(docs, "DaemonSet", "dcgm-exporter")
    assert ds is not None, "dcgm-exporter DaemonSet not rendered"
    containers = ds["spec"]["template"]["spec"]["containers"]
    assert len(containers) == 1
    args = containers[0].get("args", [])
    assert "--kubernetes=true" in args, f"missing --kubernetes=true; got {args}"


def test_dcgm_exporter_no_invalid_gpu_id_type():
    """Regression: --kubernetes-gpu-id-type=uuid is NOT a valid value (only uid/device-name).
    Passing it breaks the podMapper transformation and drops ALL metrics with:
      'unsupported KubernetesGPUIDType for MetricID uuid'
    We rely on the default (uid)."""
    docs = helm_template()
    ds = find_resource(docs, "DaemonSet", "dcgm-exporter")
    args = ds["spec"]["template"]["spec"]["containers"][0].get("args", [])
    for a in args:
        assert "kubernetes-gpu-id-type=uuid" not in a, f"invalid flag in args: {args}"


def test_otel_collector_injects_cluster_label():
    """Prometheus external_labels only apply to remote_write/federation, NOT local
    queries. The OTel collector's `resource` processor injects cluster as a real
    label on every metric, so dashboards can filter on $cluster."""
    docs = helm_template()
    cm = next(
        (d for d in docs if d.get("kind") == "ConfigMap" and "otel" in d["metadata"]["name"] and "config.yaml" in d.get("data", {})),
        None,
    )
    assert cm is not None, "otel ConfigMap with config.yaml not found"
    cfg = yaml.safe_load(cm["data"]["config.yaml"])
    processors = cfg.get("processors", {})
    assert "resource" in processors, f"resource processor missing; got {list(processors.keys())}"
    attrs = processors["resource"].get("attributes", [])
    cluster_attr = next((a for a in attrs if a.get("key") == "cluster"), None)
    assert cluster_attr is not None, f"cluster attribute missing; got {attrs}"
    assert cluster_attr.get("action") == "upsert"
    # Pipeline must actually use the processor
    metrics_pipe = cfg["service"]["pipelines"]["metrics"]
    assert "resource" in metrics_pipe["processors"], f"metrics pipeline must include resource processor: {metrics_pipe}"


def test_otel_collector_cluster_label_override():
    """clusterName should be honored by the resource processor."""
    docs = helm_template(set_values={"observability.prometheus.clusterName": "prod-us-west-2"})
    cm = next(
        (d for d in docs if d.get("kind") == "ConfigMap" and "otel" in d["metadata"]["name"] and "config.yaml" in d.get("data", {})),
        None,
    )
    cfg = yaml.safe_load(cm["data"]["config.yaml"])
    cluster_attr = next(a for a in cfg["processors"]["resource"]["attributes"] if a["key"] == "cluster")
    assert cluster_attr["value"] == "prod-us-west-2"


def test_dcgm_exporter_has_hostpid():
    docs = helm_template()
    ds = find_resource(docs, "DaemonSet", "dcgm-exporter")
    assert ds["spec"]["template"]["spec"].get("hostPID") is True, "hostPID must be true for kubelet socket access"


def test_dcgm_exporter_privileged():
    docs = helm_template()
    ds = find_resource(docs, "DaemonSet", "dcgm-exporter")
    sec = ds["spec"]["template"]["spec"]["containers"][0].get("securityContext", {})
    assert sec.get("privileged") is True, "privileged required for /var/lib/kubelet/pod-resources"


def test_dcgm_exporter_podresources_mount():
    docs = helm_template()
    ds = find_resource(docs, "DaemonSet", "dcgm-exporter")
    mounts = ds["spec"]["template"]["spec"]["containers"][0].get("volumeMounts", [])
    pr_mount = next((m for m in mounts if m["name"] == "pod-resources"), None)
    assert pr_mount is not None, "pod-resources volumeMount missing"
    assert pr_mount["mountPath"] == "/var/lib/kubelet/pod-resources"
    assert pr_mount.get("readOnly") is True


def test_dcgm_exporter_podresources_hostpath():
    docs = helm_template()
    ds = find_resource(docs, "DaemonSet", "dcgm-exporter")
    volumes = ds["spec"]["template"]["spec"].get("volumes", [])
    pr_vol = next((v for v in volumes if v["name"] == "pod-resources"), None)
    assert pr_vol is not None, "pod-resources volume missing"
    assert pr_vol["hostPath"]["path"] == "/var/lib/kubelet/pod-resources"


def test_dcgm_exporter_kubernetes_disable_opt_out():
    """When kubernetes.enabled=false, none of the privileged bits should render."""
    docs = helm_template(set_values={"observability.dcgmExporter.kubernetes.enabled": "false"})
    ds = find_resource(docs, "DaemonSet", "dcgm-exporter")
    assert ds is not None
    spec = ds["spec"]["template"]["spec"]
    assert spec.get("hostPID") is not True
    assert spec["containers"][0].get("args", []) == [] or "--kubernetes=true" not in spec["containers"][0].get(
        "args", []
    )
    assert spec["containers"][0].get("securityContext", {}).get("privileged") is not True


# ──────────────────────────────── Prometheus external_labels ────────────────────────


def test_prometheus_external_labels_cluster():
    docs = helm_template()
    cm = next(
        (d for d in docs if d.get("kind") == "ConfigMap" and "prometheus" in d["metadata"]["name"] and "prometheus.yml" in d.get("data", {})),
        None,
    )
    assert cm is not None, "prometheus ConfigMap with prometheus.yml not found"
    cfg = yaml.safe_load(cm["data"]["prometheus.yml"])
    ext = cfg.get("global", {}).get("external_labels", {})
    assert "cluster" in ext, f"global.external_labels.cluster missing; got {ext}"
    assert ext["cluster"] == "kube-llmops"  # default from values


def test_prometheus_external_labels_override():
    docs = helm_template(set_values={"observability.prometheus.clusterName": "prod-us-west-2"})
    cm = next(
        (d for d in docs if d.get("kind") == "ConfigMap" and "prometheus" in d["metadata"]["name"] and "prometheus.yml" in d.get("data", {})),
        None,
    )
    cfg = yaml.safe_load(cm["data"]["prometheus.yml"])
    assert cfg["global"]["external_labels"]["cluster"] == "prod-us-west-2"


# ──────────────────────────────── Dashboard JSONs ────────────────────────────────

TIERS = {
    "gpu-cluster": {
        "title": "GPU · L1 Cluster Overview",
        "vars": ["cluster"],
        "min_panels": 10,
    },
    "gpu-node": {
        "title": "GPU · L2 Node View",
        "vars": ["cluster", "node"],
        "min_panels": 10,
    },
    "gpu-gpu": {
        "title": "GPU · L3 Single GPU View",
        "vars": ["cluster", "node", "gpu"],
        "min_panels": 12,
    },
    "gpu-pod": {
        "title": "GPU · L4 Pod / Workload View",
        "vars": ["cluster", "namespace", "pod"],
        "min_panels": 12,
    },
}


@pytest.mark.parametrize("uid,spec", TIERS.items())
def test_dashboard_file_exists_and_valid(uid, spec):
    path = DASHBOARDS_DIR / f"{uid}.json"
    assert path.exists(), f"{path} missing"
    with open(path) as f:
        data = json.load(f)
    assert data["uid"] == uid
    assert data["title"] == spec["title"]
    assert len(data.get("panels", [])) >= spec["min_panels"], f"{uid} has only {len(data['panels'])} panels"
    var_names = [v["name"] for v in data.get("templating", {}).get("list", [])]
    assert set(var_names) == set(spec["vars"]), f"{uid} vars mismatch: expected {spec['vars']}, got {var_names}"


@pytest.mark.parametrize("uid", TIERS.keys())
def test_dashboard_rendered_in_configmap(uid):
    """Each GPU dashboard JSON must be included in the Grafana dashboards ConfigMap."""
    docs = helm_template()
    # There are multiple dashboard ConfigMaps; check any contains our uid
    found = False
    for d in docs:
        if d.get("kind") != "ConfigMap":
            continue
        for key, content in (d.get("data") or {}).items():
            if f"{uid}.json" == key or (isinstance(content, str) and f'"uid": "{uid}"' in content):
                found = True
                break
        if found:
            break
    assert found, f"dashboard {uid}.json not rendered into any ConfigMap"


# ──────────────────────────────── Drill-down URLs ────────────────────────────────


def _dashboard_content(uid):
    with open(DASHBOARDS_DIR / f"{uid}.json") as f:
        return f.read()


def test_l1_drills_to_l2():
    content = _dashboard_content("gpu-cluster")
    assert "/d/gpu-node/" in content, "L1 Node table should link to L2 gpu-node"
    assert "${__data.fields.Node}" in content or "${__data.fields.Hostname}" in content


def test_l2_drills_to_l3():
    content = _dashboard_content("gpu-node")
    assert "/d/gpu-gpu/" in content, "L2 Inventory table should link to L3 gpu-gpu"
    assert "var-gpu=" in content, "L2 drill-down should pass gpu UUID variable"


def test_l3_drills_to_l4():
    content = _dashboard_content("gpu-gpu")
    assert "/d/gpu-pod/" in content, "L3 running-pods table should link to L4 gpu-pod"
    assert "var-namespace=" in content or "var-pod=" in content


def test_l4_drills_to_l3():
    content = _dashboard_content("gpu-pod")
    assert "/d/gpu-gpu/" in content, "L4 Allocation table should link back to L3 per GPU"


def test_all_tiers_have_gpu_tag():
    for uid in TIERS:
        with open(DASHBOARDS_DIR / f"{uid}.json") as f:
            data = json.load(f)
        tags = data.get("tags", [])
        assert "gpu" in tags, f"{uid} missing 'gpu' tag; got {tags}"
        assert "kube-llmops" in tags, f"{uid} missing 'kube-llmops' tag; got {tags}"
