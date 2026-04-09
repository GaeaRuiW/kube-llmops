"""
Helm template unit tests for the headlamp subchart.
Validates that helm template renders correct K8s manifests.

Usage:
    cd kube-llmops
    pip install pytest pyyaml
    python -m pytest tests/helm/test_dashboard_templates.py -v
"""

import subprocess
import yaml
import pytest
from pathlib import Path

CHART_DIR = Path(__file__).parent.parent.parent / "charts" / "kube-llmops-stack"
VALUES_FILE = CHART_DIR / "values-single-node.yaml"


def helm_template(set_values=None, show_only=None):
    """Run helm template and return parsed YAML documents."""
    cmd = ["helm", "template", "test", str(CHART_DIR), "-f", str(VALUES_FILE)]
    for k, v in (set_values or {}).items():
        cmd += ["--set", f"{k}={v}"]
    if show_only:
        cmd += ["--show-only", show_only]
    result = subprocess.run(cmd, capture_output=True, text=True, timeout=60)
    if result.returncode != 0:
        raise RuntimeError(f"helm template failed: {result.stderr}")
    docs = []
    for doc in yaml.safe_load_all(result.stdout):
        if doc:
            docs.append(doc)
    return docs


def find_by_kind(docs, kind, name_contains=None):
    """Find documents of a specific kind, optionally filtering by name."""
    found = [d for d in docs if d.get("kind") == kind]
    if name_contains:
        found = [d for d in found if name_contains in d["metadata"]["name"]]
    return found


class TestHeadlampDeployment:
    """Tests for headlamp Deployment manifest."""

    def test_deployment_exists(self):
        docs = helm_template(show_only="charts/headlamp/charts/headlamp/templates/deployment.yaml")
        assert len(docs) == 1
        dep = docs[0]
        assert dep["kind"] == "Deployment"
        assert "headlamp" in dep["metadata"]["name"]

    def test_deployment_image(self):
        docs = helm_template(show_only="charts/headlamp/charts/headlamp/templates/deployment.yaml")
        dep = docs[0]
        container = dep["spec"]["template"]["spec"]["containers"][0]
        assert "headlamp" in container["image"]

    def test_deployment_port(self):
        docs = helm_template(show_only="charts/headlamp/charts/headlamp/templates/deployment.yaml")
        dep = docs[0]
        container = dep["spec"]["template"]["spec"]["containers"][0]
        ports = [p["containerPort"] for p in container.get("ports", [])]
        assert 4466 in ports

    def test_deployment_replicas(self):
        docs = helm_template(show_only="charts/headlamp/charts/headlamp/templates/deployment.yaml")
        dep = docs[0]
        assert dep["spec"]["replicas"] == 1

    def test_deployment_service_account(self):
        docs = helm_template(show_only="charts/headlamp/charts/headlamp/templates/deployment.yaml")
        dep = docs[0]
        sa = dep["spec"]["template"]["spec"].get("serviceAccountName", "")
        assert "headlamp" in sa

    def test_deployment_has_liveness_probe(self):
        docs = helm_template(show_only="charts/headlamp/charts/headlamp/templates/deployment.yaml")
        dep = docs[0]
        container = dep["spec"]["template"]["spec"]["containers"][0]
        assert "livenessProbe" in container

    def test_deployment_has_readiness_probe(self):
        docs = helm_template(show_only="charts/headlamp/charts/headlamp/templates/deployment.yaml")
        dep = docs[0]
        container = dep["spec"]["template"]["spec"]["containers"][0]
        assert "readinessProbe" in container


class TestHeadlampService:
    """Tests for headlamp Service manifest."""

    def test_service_exists(self):
        docs = helm_template(show_only="charts/headlamp/charts/headlamp/templates/service.yaml")
        assert len(docs) >= 1
        svc = docs[0]
        assert svc["kind"] == "Service"
        assert "headlamp" in svc["metadata"]["name"]

    def test_service_port(self):
        docs = helm_template(show_only="charts/headlamp/charts/headlamp/templates/service.yaml")
        svc = docs[0]
        ports = svc["spec"]["ports"]
        port_numbers = [p["port"] for p in ports]
        assert 80 in port_numbers

    def test_service_type_clusterip(self):
        docs = helm_template(show_only="charts/headlamp/charts/headlamp/templates/service.yaml")
        svc = docs[0]
        assert svc["spec"]["type"] == "ClusterIP"


class TestHeadlampNodePort:
    """Tests for headlamp NodePort in parent template."""

    def test_nodeport_exists(self):
        docs = helm_template(
            set_values={"global.nodePort.enabled": "true", "global.nodePort.host": "10.0.0.1"},
            show_only="templates/nodeport-services.yaml",
        )
        svc_names = [d["metadata"]["name"] for d in docs if d["kind"] == "Service"]
        headlamp_svcs = [n for n in svc_names if "headlamp" in n]
        assert len(headlamp_svcs) >= 1, f"No headlamp NodePort found in: {svc_names}"

    def test_nodeport_is_30302(self):
        docs = helm_template(
            set_values={"global.nodePort.enabled": "true", "global.nodePort.host": "10.0.0.1"},
            show_only="templates/nodeport-services.yaml",
        )
        for doc in docs:
            if doc["kind"] == "Service" and "headlamp" in doc["metadata"]["name"]:
                node_ports = [p.get("nodePort") for p in doc["spec"]["ports"]]
                assert 30302 in node_ports, f"Expected NodePort 30302, got {node_ports}"
                return
        pytest.fail("Headlamp NodePort service not found")

    def test_nodeport_target_port(self):
        docs = helm_template(
            set_values={"global.nodePort.enabled": "true", "global.nodePort.host": "10.0.0.1"},
            show_only="templates/nodeport-services.yaml",
        )
        for doc in docs:
            if doc["kind"] == "Service" and "headlamp" in doc["metadata"]["name"]:
                target_ports = [p.get("targetPort") for p in doc["spec"]["ports"]]
                assert 4466 in target_ports, f"Expected targetPort 4466, got {target_ports}"
                return
        pytest.fail("Headlamp NodePort service not found")


class TestHeadlampPlugin:
    """Tests for headlamp plugin init container and volumes."""

    def test_plugin_init_container_exists(self):
        docs = helm_template(show_only="charts/headlamp/charts/headlamp/templates/deployment.yaml")
        dep = docs[0]
        init_containers = dep["spec"]["template"]["spec"].get("initContainers", [])
        init_names = [c["name"] for c in init_containers]
        assert "install-llmops-plugin" in init_names, f"Plugin init container not found in: {init_names}"

    def test_plugin_init_container_image(self):
        docs = helm_template(show_only="charts/headlamp/charts/headlamp/templates/deployment.yaml")
        dep = docs[0]
        init_containers = dep["spec"]["template"]["spec"].get("initContainers", [])
        plugin_container = [c for c in init_containers if c["name"] == "install-llmops-plugin"][0]
        assert "headlamp-plugin" in plugin_container["image"]

    def test_plugins_volume_exists(self):
        docs = helm_template(show_only="charts/headlamp/charts/headlamp/templates/deployment.yaml")
        dep = docs[0]
        volumes = dep["spec"]["template"]["spec"].get("volumes", [])
        volume_names = [v["name"] for v in volumes]
        assert "plugins" in volume_names, f"plugins volume not found in: {volume_names}"

    def test_plugins_volume_mounted_on_container(self):
        docs = helm_template(show_only="charts/headlamp/charts/headlamp/templates/deployment.yaml")
        dep = docs[0]
        container = dep["spec"]["template"]["spec"]["containers"][0]
        mount_paths = [m["mountPath"] for m in container.get("volumeMounts", [])]
        assert "/headlamp/plugins" in mount_paths

    def test_plugins_dir_arg(self):
        docs = helm_template(show_only="charts/headlamp/charts/headlamp/templates/deployment.yaml")
        dep = docs[0]
        container = dep["spec"]["template"]["spec"]["containers"][0]
        args = container.get("args", [])
        assert "-plugins-dir=/headlamp/plugins" in args

    def test_ca_bundle_init_container_exists(self):
        docs = helm_template(show_only="charts/headlamp/charts/headlamp/templates/deployment.yaml")
        dep = docs[0]
        init_containers = dep["spec"]["template"]["spec"].get("initContainers", [])
        init_names = [c["name"] for c in init_containers]
        assert "build-ca-bundle" in init_names, f"CA bundle init container not found in: {init_names}"


class TestHeadlampRBAC:
    """Tests for headlamp RBAC resources."""

    def test_serviceaccount_exists(self):
        docs = helm_template(show_only="charts/headlamp/charts/headlamp/templates/serviceaccount.yaml")
        assert len(docs) >= 1
        sa = docs[0]
        assert sa["kind"] == "ServiceAccount"
        assert "headlamp" in sa["metadata"]["name"]

    def test_clusterrolebinding_exists(self):
        docs = helm_template(show_only="charts/headlamp/charts/headlamp/templates/clusterrolebinding.yaml")
        assert len(docs) >= 1
        crb = docs[0]
        assert crb["kind"] == "ClusterRoleBinding"
        assert "headlamp" in crb["metadata"]["name"]


class TestHeadlampIntegration:
    """Tests for headlamp integrations with other components."""

    def test_keycloak_has_headlamp_client(self):
        """Headlamp OIDC client should be registered in Keycloak realm."""
        docs = helm_template(show_only="charts/keycloak/templates/realm-configmap.yaml")
        assert len(docs) >= 1
        cm = docs[0]
        realm_data = cm["data"].get("realm-import.json", "") or cm["data"].get("realm.json", "")
        assert "headlamp" in realm_data, "Headlamp client not found in Keycloak realm"

    def test_oidc_args_in_deployment(self):
        """Headlamp deployment should contain OIDC args for Keycloak integration."""
        docs = helm_template(show_only="charts/headlamp/charts/headlamp/templates/deployment.yaml")
        dep = docs[0]
        container = dep["spec"]["template"]["spec"]["containers"][0]
        args = container.get("args", [])
        args_str = " ".join(args)
        assert "-oidc-client-id" in args_str, "OIDC client-id arg not found"
        assert "-oidc-client-secret" in args_str, "OIDC client-secret arg not found"
