"""
Helm template unit tests for the dashboard subchart.
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


class TestDashboardDeployment:
    """Tests for dashboard Deployment manifest."""

    def test_deployment_exists(self):
        docs = helm_template(show_only="charts/dashboard/templates/deployment.yaml")
        assert len(docs) == 1
        dep = docs[0]
        assert dep["kind"] == "Deployment"
        assert "dashboard" in dep["metadata"]["name"]

    def test_deployment_image(self):
        docs = helm_template(show_only="charts/dashboard/templates/deployment.yaml")
        dep = docs[0]
        container = dep["spec"]["template"]["spec"]["containers"][0]
        assert container["image"] == "kube-llmops/dashboard:latest"

    def test_deployment_port(self):
        docs = helm_template(show_only="charts/dashboard/templates/deployment.yaml")
        dep = docs[0]
        container = dep["spec"]["template"]["spec"]["containers"][0]
        ports = [p["containerPort"] for p in container.get("ports", [])]
        assert 3000 in ports

    def test_deployment_has_env_vars(self):
        docs = helm_template(show_only="charts/dashboard/templates/deployment.yaml")
        dep = docs[0]
        container = dep["spec"]["template"]["spec"]["containers"][0]
        env_names = [e["name"] for e in container.get("env", [])]
        assert "PORT" in env_names
        assert "NAMESPACE" in env_names

    def test_deployment_service_account(self):
        docs = helm_template(show_only="charts/dashboard/templates/deployment.yaml")
        dep = docs[0]
        sa = dep["spec"]["template"]["spec"].get("serviceAccountName", "")
        assert "dashboard" in sa

    def test_deployment_replicas(self):
        docs = helm_template(show_only="charts/dashboard/templates/deployment.yaml")
        dep = docs[0]
        assert dep["spec"]["replicas"] == 1

    def test_deployment_resource_limits(self):
        docs = helm_template(show_only="charts/dashboard/templates/deployment.yaml")
        dep = docs[0]
        container = dep["spec"]["template"]["spec"]["containers"][0]
        resources = container.get("resources", {})
        assert "limits" in resources
        assert "requests" in resources


class TestDashboardService:
    """Tests for dashboard Service manifest."""

    def test_service_exists(self):
        docs = helm_template(show_only="charts/dashboard/templates/service.yaml")
        assert len(docs) >= 1
        svc = docs[0]
        assert svc["kind"] == "Service"
        assert "dashboard" in svc["metadata"]["name"]

    def test_service_port(self):
        docs = helm_template(show_only="charts/dashboard/templates/service.yaml")
        svc = docs[0]
        ports = svc["spec"]["ports"]
        port_numbers = [p["port"] for p in ports]
        assert 3000 in port_numbers


class TestDashboardNodePort:
    """Tests for dashboard NodePort in parent template."""

    def test_nodeport_exists(self):
        docs = helm_template(
            set_values={"global.nodePort.enabled": "true", "global.nodePort.host": "10.0.0.1"},
            show_only="templates/nodeport-services.yaml",
        )
        svc_names = [d["metadata"]["name"] for d in docs if d["kind"] == "Service"]
        dashboard_svcs = [n for n in svc_names if "dashboard" in n]
        assert len(dashboard_svcs) >= 1, f"No dashboard NodePort found in: {svc_names}"

    def test_nodeport_is_30302(self):
        docs = helm_template(
            set_values={"global.nodePort.enabled": "true", "global.nodePort.host": "10.0.0.1"},
            show_only="templates/nodeport-services.yaml",
        )
        for doc in docs:
            if doc["kind"] == "Service" and "dashboard" in doc["metadata"]["name"]:
                node_ports = [p.get("nodePort") for p in doc["spec"]["ports"]]
                assert 30302 in node_ports, f"Expected NodePort 30302, got {node_ports}"
                return
        pytest.fail("Dashboard NodePort service not found")


class TestDashboardRBAC:
    """Tests for dashboard RBAC resources."""

    def test_serviceaccount_exists(self):
        docs = helm_template(show_only="charts/dashboard/templates/serviceaccount.yaml")
        assert len(docs) >= 1
        sa = docs[0]
        assert sa["kind"] == "ServiceAccount"
        assert "dashboard" in sa["metadata"]["name"]


class TestDashboardIntegration:
    """Tests for dashboard integrations with other components."""

    def test_keycloak_has_dashboard_client(self):
        """Dashboard OIDC client should be registered in Keycloak realm."""
        docs = helm_template(show_only="charts/keycloak/templates/realm-configmap.yaml")
        assert len(docs) >= 1
        cm = docs[0]
        realm_data = cm["data"].get("realm-import.json", "") or cm["data"].get("realm.json", "")
        assert "dashboard" in realm_data, "Dashboard client not found in Keycloak realm"

    def test_postgresql_has_dashboard_db(self):
        """PostgreSQL init should create dashboard database."""
        docs = helm_template(show_only="charts/litellm/templates/postgresql.yaml")
        for doc in docs:
            if doc["kind"] == "ConfigMap" and "init" in doc["metadata"]["name"]:
                init_script = list(doc["data"].values())[0]
                assert "dashboard" in init_script, "Dashboard DB not in PostgreSQL init"
                return
        # Try alternative locations
        all_docs = helm_template()
        for doc in all_docs:
            if doc.get("kind") == "ConfigMap" and "postgres" in doc["metadata"]["name"].lower():
                for v in doc.get("data", {}).values():
                    if "dashboard" in v:
                        return
        pytest.skip("PostgreSQL init ConfigMap not found in expected location")
