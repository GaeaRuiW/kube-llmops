"""
Helm template unit tests for the finetune subchart.
Validates that helm template renders correct K8s manifests for all configurations.

Usage:
    cd kube-llmops
    pip install pytest pyyaml
    python -m pytest tests/helm/test_finetune_templates.py -v
"""

import subprocess
import json
import yaml
import pytest
from pathlib import Path

CHART_DIR = Path(__file__).parent.parent.parent / "charts" / "kube-llmops-stack"


def helm_template(set_values=None, values_files=None, show_only=None):
    """Run helm template and return parsed YAML documents."""
    cmd = ["helm", "template", "test", str(CHART_DIR)]
    for vf in (values_files or []):
        cmd += ["-f", str(vf)]
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


# ---------------------------------------------------------------------------
# Shared values for finetune-enabled rendering
# ---------------------------------------------------------------------------
FINETUNE_BASE = {
    "finetune.enabled": "true",
    "finetune.baseModel": "Qwen/Qwen2.5-0.5B-Instruct",
    "finetune.outputName": "qwen-ft-test",
    "finetune.method": "lora",
    "finetune.mlflow.enabled": "true",
}


class TestFinetuneDisabled:
    """When finetune.enabled=false, no finetune resources should be rendered."""

    def test_no_resources_when_disabled(self):
        docs = helm_template(set_values={"finetune.enabled": "false"})
        finetune_docs = [d for d in docs if "finetune" in d.get("metadata", {}).get("name", "")]
        assert len(finetune_docs) == 0, f"Expected no finetune resources, got {len(finetune_docs)}"

    def test_no_mlflow_when_disabled(self):
        docs = helm_template(set_values={"finetune.enabled": "false"})
        mlflow_docs = find_by_kind(docs, "Deployment", "mlflow")
        assert len(mlflow_docs) == 0


class TestFinetuneEnabled:
    """When finetune.enabled=true, all finetune resources should be rendered."""

    def test_configmap_rendered(self):
        docs = helm_template(set_values=FINETUNE_BASE)
        cms = find_by_kind(docs, "ConfigMap", "finetune-config")
        assert len(cms) == 1
        cm = cms[0]
        config_yaml = cm["data"]["train_config.yaml"]
        assert "model_name_or_path: /workspace/base-model" in config_yaml
        assert "finetuning_type: lora" in config_yaml

    def test_serviceaccount_rendered(self):
        docs = helm_template(set_values=FINETUNE_BASE)
        sas = find_by_kind(docs, "ServiceAccount", "finetune")
        assert len(sas) == 1
        assert sas[0]["metadata"]["labels"]["app.kubernetes.io/name"] == "finetune"

    def test_clusterrole_rendered(self):
        docs = helm_template(set_values=FINETUNE_BASE)
        roles = find_by_kind(docs, "ClusterRole", "finetune")
        assert len(roles) == 1
        rules = roles[0]["rules"]
        # Verify key permissions
        api_groups = {r["apiGroups"][0] for r in rules}
        assert "" in api_groups  # core resources
        assert "apps" in api_groups  # deployments
        assert "batch" in api_groups  # jobs
        assert "argoproj.io" in api_groups  # workflowtaskresults

    def test_clusterrolebinding_rendered(self):
        docs = helm_template(set_values=FINETUNE_BASE)
        crbs = find_by_kind(docs, "ClusterRoleBinding", "finetune")
        assert len(crbs) == 1
        assert crbs[0]["roleRef"]["name"] == "test-finetune"
        assert crbs[0]["subjects"][0]["name"] == "test-finetune"

    def test_pdb_rendered(self):
        docs = helm_template(set_values={**FINETUNE_BASE, "finetune.pdb.enabled": "true"})
        pdbs = find_by_kind(docs, "PodDisruptionBudget", "mlflow")
        assert len(pdbs) == 1
        assert pdbs[0]["spec"]["maxUnavailable"] == 1

    def test_pdb_not_rendered_when_disabled(self):
        docs = helm_template(set_values={**FINETUNE_BASE, "finetune.pdb.enabled": "false"})
        pdbs = find_by_kind(docs, "PodDisruptionBudget", "mlflow")
        assert len(pdbs) == 0


class TestMLflowDeployment:
    """Validate MLflow Deployment + Service rendering."""

    def test_mlflow_deployment_rendered(self):
        docs = helm_template(set_values=FINETUNE_BASE)
        deps = find_by_kind(docs, "Deployment", "mlflow")
        assert len(deps) == 1
        dep = deps[0]
        containers = dep["spec"]["template"]["spec"]["containers"]
        assert len(containers) == 1
        c = containers[0]
        assert c["name"] == "mlflow"
        assert "5000" in str(c["command"])  # port in command args

    def test_mlflow_service_rendered(self):
        docs = helm_template(set_values=FINETUNE_BASE)
        svcs = find_by_kind(docs, "Service", "mlflow")
        # Filter to only the ClusterIP service (not NodePort)
        cluster_svcs = [s for s in svcs if s["spec"].get("type", "ClusterIP") == "ClusterIP"]
        assert len(cluster_svcs) == 1
        assert cluster_svcs[0]["spec"]["ports"][0]["port"] == 5000

    def test_mlflow_not_rendered_when_disabled(self):
        vals = {**FINETUNE_BASE, "finetune.mlflow.enabled": "false"}
        docs = helm_template(set_values=vals)
        deps = find_by_kind(docs, "Deployment", "mlflow")
        assert len(deps) == 0

    def test_mlflow_postgres_connection(self):
        docs = helm_template(set_values=FINETUNE_BASE)
        deps = find_by_kind(docs, "Deployment", "mlflow")
        assert len(deps) == 1
        cmd = " ".join(deps[0]["spec"]["template"]["spec"]["containers"][0]["command"])
        args = deps[0]["spec"]["template"]["spec"]["containers"][0].get("args", [])
        full_cmd = cmd + " " + " ".join(str(a) for a in args)
        assert "postgresql://" in full_cmd
        assert "litellm-pg:5432" in full_cmd

    def test_mlflow_s3_env_vars(self):
        docs = helm_template(set_values=FINETUNE_BASE)
        deps = find_by_kind(docs, "Deployment", "mlflow")
        assert len(deps) == 1
        envs = {e["name"]: e["value"] for e in deps[0]["spec"]["template"]["spec"]["containers"][0]["env"]}
        assert "MLFLOW_S3_ENDPOINT_URL" in envs
        assert "AWS_ACCESS_KEY_ID" in envs
        assert "AWS_SECRET_ACCESS_KEY" in envs

    def test_mlflow_probes(self):
        docs = helm_template(set_values=FINETUNE_BASE)
        deps = find_by_kind(docs, "Deployment", "mlflow")
        c = deps[0]["spec"]["template"]["spec"]["containers"][0]
        assert "readinessProbe" in c
        assert "livenessProbe" in c
        assert c["readinessProbe"]["tcpSocket"]["port"] == 5000

    def test_mlflow_resource_limits(self):
        docs = helm_template(set_values=FINETUNE_BASE)
        deps = find_by_kind(docs, "Deployment", "mlflow")
        resources = deps[0]["spec"]["template"]["spec"]["containers"][0]["resources"]
        assert "requests" in resources
        assert "limits" in resources


class TestConfigMapTrainingConfig:
    """Validate training ConfigMap for different methods."""

    def test_lora_config(self):
        vals = {**FINETUNE_BASE, "finetune.method": "lora", "finetune.loraRank": "16", "finetune.loraAlpha": "32"}
        docs = helm_template(set_values=vals)
        cms = find_by_kind(docs, "ConfigMap", "finetune-config")
        config = cms[0]["data"]["train_config.yaml"]
        assert "finetuning_type: lora" in config
        assert "lora_rank: 16" in config
        assert "lora_alpha: 32" in config
        assert "lora_target: all" in config
        # No quantization for lora
        assert "quantization_bit" not in config

    def test_qlora_config(self):
        vals = {**FINETUNE_BASE, "finetune.method": "qlora"}
        docs = helm_template(set_values=vals)
        cms = find_by_kind(docs, "ConfigMap", "finetune-config")
        config = cms[0]["data"]["train_config.yaml"]
        assert "finetuning_type: qlora" in config
        assert "quantization_bit: 4" in config
        assert "quantization_method: bitsandbytes" in config
        assert "lora_rank:" in config

    def test_full_config(self):
        vals = {**FINETUNE_BASE, "finetune.method": "full"}
        docs = helm_template(set_values=vals)
        cms = find_by_kind(docs, "ConfigMap", "finetune-config")
        config = cms[0]["data"]["train_config.yaml"]
        assert "finetuning_type: full" in config
        # No LoRA or quantization settings for full
        assert "lora_rank" not in config
        assert "quantization_bit" not in config

    def test_training_hyperparams(self):
        vals = {
            **FINETUNE_BASE,
            "finetune.epochs": "5",
            "finetune.batchSize": "8",
            "finetune.learningRate": "1e-5",
        }
        docs = helm_template(set_values=vals)
        cms = find_by_kind(docs, "ConfigMap", "finetune-config")
        config = cms[0]["data"]["train_config.yaml"]
        assert "num_train_epochs: 5" in config
        assert "per_device_train_batch_size: 8" in config
        assert "learning_rate: 1e-5" in config

    def test_gpu_enables_bf16(self):
        vals = {**FINETUNE_BASE, "finetune.resources.gpu": "1"}
        docs = helm_template(set_values=vals)
        cms = find_by_kind(docs, "ConfigMap", "finetune-config")
        config = cms[0]["data"]["train_config.yaml"]
        assert "bf16: true" in config

    def test_cpu_disables_bf16(self):
        vals = {**FINETUNE_BASE, "finetune.resources.gpu": "0"}
        docs = helm_template(set_values=vals)
        cms = find_by_kind(docs, "ConfigMap", "finetune-config")
        config = cms[0]["data"]["train_config.yaml"]
        assert "bf16: false" in config

    def test_mlflow_reporting(self):
        docs = helm_template(set_values=FINETUNE_BASE)
        cms = find_by_kind(docs, "ConfigMap", "finetune-config")
        config = cms[0]["data"]["train_config.yaml"]
        assert "report_to: mlflow" in config


class TestCronWorkflow:
    """Validate CronWorkflow conditional rendering."""

    def test_no_cronworkflow_without_schedule(self):
        """CronWorkflow should NOT render when schedule is empty."""
        vals = {**FINETUNE_BASE, "finetune.schedule": ""}
        docs = helm_template(set_values=vals)
        crons = find_by_kind(docs, "CronWorkflow")
        assert len(crons) == 0

    # NOTE: CronWorkflow with schedule set requires Argo CRDs to be installed
    # (the template uses `lookup` which returns empty when CRDs are absent).
    # This is tested in the E2E suite instead.


class TestNodePortService:
    """Validate NodePort service for MLflow."""

    def test_mlflow_nodeport_when_enabled(self):
        vals = {
            **FINETUNE_BASE,
            "global.nodePort.enabled": "true",
            "global.nodePort.host": "192.168.1.100",
        }
        docs = helm_template(set_values=vals)
        np_svcs = [d for d in find_by_kind(docs, "Service", "mlflow") if d["spec"].get("type") == "NodePort"]
        assert len(np_svcs) == 1
        assert np_svcs[0]["spec"]["ports"][0]["nodePort"] == 30505


class TestAllProfiles:
    """Ensure helm template renders without errors for all value profiles."""

    @pytest.mark.parametrize("profile", [
        "values-ci.yaml",
        "values-single-node.yaml",
    ])
    def test_profile_renders(self, profile):
        """Template rendering should succeed (even if finetune is disabled)."""
        vf = CHART_DIR / profile
        if not vf.exists():
            pytest.skip(f"{profile} not found")
        docs = helm_template(values_files=[vf])
        assert len(docs) > 0, f"No documents rendered for {profile}"

    def test_finetune_enabled_renders(self):
        """Rendering with finetune.enabled=true should produce finetune resources."""
        vf = CHART_DIR / "values-single-node.yaml"
        docs = helm_template(
            values_files=[vf],
            set_values={
                "finetune.enabled": "true",
                "finetune.baseModel": "test/model",
                "finetune.outputName": "test-output",
            }
        )
        finetune_docs = [d for d in docs if "finetune" in d.get("metadata", {}).get("name", "")]
        assert len(finetune_docs) >= 3, (
            f"Expected at least SA+ClusterRole+ConfigMap, got {len(finetune_docs)}"
        )
