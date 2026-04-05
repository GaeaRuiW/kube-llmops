"""
Helm template unit tests for Phase 5: Advanced Inference features.

Usage:
    cd kube-llmops
    python -m pytest tests/helm/test_phase5_templates.py -v
"""

import subprocess
import json
import yaml
import pytest
from pathlib import Path

CHART_DIR = Path(__file__).parent.parent.parent / "charts" / "kube-llmops-stack"

# Minimal model set for template rendering
SINGLE_MODEL = {
    "global.models[0].name": "test-model",
    "global.models[0].source": "org/test-model",
    "global.models[0].resources.gpu": "1",
    "global.models[0].resources.memory": "16Gi",
}


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
        # Templates that render nothing cause "could not find template" – treat as empty
        if "could not find template" in result.stderr:
            return []
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


def get_configmap_data(docs, name_contains):
    """Extract parsed YAML from a ConfigMap's data field."""
    cms = find_by_kind(docs, "ConfigMap", name_contains)
    assert len(cms) >= 1, f"No ConfigMap found containing '{name_contains}'"
    raw = cms[0]["data"].get("config.yaml", "")
    return yaml.safe_load(raw)


class TestRoutingStrategy:
    """Test LiteLLM routing strategy configuration."""

    def test_default_routing_is_latency_based(self):
        docs = helm_template(set_values=SINGLE_MODEL)
        config = get_configmap_data(docs, "litellm-config")
        assert config["router_settings"]["routing_strategy"] == "latency-based-routing"

    def test_custom_routing_strategy(self):
        vals = {**SINGLE_MODEL, "litellm.routingStrategy": "simple-shuffle"}
        docs = helm_template(set_values=vals)
        config = get_configmap_data(docs, "litellm-config")
        assert config["router_settings"]["routing_strategy"] == "simple-shuffle"

    def test_routing_strategy_args_rendered(self):
        vals = {
            **SINGLE_MODEL,
            "litellm.routingStrategyArgs.ttl": "120",
            "litellm.routingStrategyArgs.lowest_latency_buffer": "0.3",
        }
        docs = helm_template(set_values=vals)
        config = get_configmap_data(docs, "litellm-config")
        rs = config["router_settings"]
        assert rs["routing_strategy_args"]["ttl"] == 120
        assert rs["routing_strategy_args"]["lowest_latency_buffer"] == 0.3


class TestPrefixCaching:
    """Test vLLM prefix caching flag."""

    def test_prefix_caching_disabled_by_default(self):
        docs = helm_template(
            set_values=SINGLE_MODEL,
            show_only="charts/vllm/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment")
        assert len(deps) == 1
        container_args = deps[0]["spec"]["template"]["spec"]["containers"][0]["args"][0]
        assert "--enable-prefix-caching" not in container_args

    def test_prefix_caching_enabled(self):
        vals = {**SINGLE_MODEL, "global.models[0].prefixCaching": "true"}
        docs = helm_template(
            set_values=vals,
            show_only="charts/vllm/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment")
        assert len(deps) == 1
        container_args = deps[0]["spec"]["template"]["spec"]["containers"][0]["args"][0]
        assert "--enable-prefix-caching" in container_args


class TestSLOAlerts:
    """Test SLO alert rules in Prometheus config."""

    def test_ttft_slo_alerts_exist(self):
        docs = helm_template(
            set_values=SINGLE_MODEL,
            show_only="charts/observability/templates/prometheus.yaml",
        )
        cms = find_by_kind(docs, "ConfigMap", "prometheus-config")
        assert len(cms) >= 1
        rules_raw = cms[0]["data"]["rules.yml"]
        rules = yaml.safe_load(rules_raw)
        all_alert_names = []
        for group in rules["groups"]:
            for rule in group.get("rules", []):
                if "alert" in rule:
                    all_alert_names.append(rule["alert"])
        assert "TTFTSLOBreach" in all_alert_names
        assert "TTFTSLOCritical" in all_alert_names


KEDA_BASE = {
    **SINGLE_MODEL,
    "keda.enabled": "true",
}


class TestKedaMultiTrigger:
    """Test KEDA ScaledObject multi-trigger configuration."""

    def test_single_trigger_default(self):
        """Default: only requestsWaiting trigger."""
        docs = helm_template(
            set_values=KEDA_BASE,
            show_only="charts/keda/templates/scaledobject.yaml",
        )
        sos = find_by_kind(docs, "ScaledObject")
        assert len(sos) == 1
        triggers = sos[0]["spec"]["triggers"]
        assert len(triggers) == 1
        assert "num_requests_waiting" in triggers[0]["metadata"]["query"]

    def test_ttft_trigger_added(self):
        """Enable TTFT P95 trigger — should produce 2 triggers."""
        vals = {**KEDA_BASE, "keda.triggers.ttftP95.enabled": "true"}
        docs = helm_template(
            set_values=vals,
            show_only="charts/keda/templates/scaledobject.yaml",
        )
        sos = find_by_kind(docs, "ScaledObject")
        assert len(sos) == 1
        triggers = sos[0]["spec"]["triggers"]
        assert len(triggers) == 2
        queries = [t["metadata"]["query"] for t in triggers]
        assert any("num_requests_waiting" in q for q in queries)
        assert any("time_to_first_token" in q for q in queries)

    def test_all_three_triggers(self):
        """Enable all 3 triggers."""
        vals = {
            **KEDA_BASE,
            "keda.triggers.ttftP95.enabled": "true",
            "keda.triggers.tpotP95.enabled": "true",
        }
        docs = helm_template(
            set_values=vals,
            show_only="charts/keda/templates/scaledobject.yaml",
        )
        sos = find_by_kind(docs, "ScaledObject")
        triggers = sos[0]["spec"]["triggers"]
        assert len(triggers) == 3
        queries = [t["metadata"]["query"] for t in triggers]
        assert any("time_per_output_token" in q for q in queries)

    def test_per_model_ttft_threshold_override(self):
        """Per-model override for TTFT threshold."""
        vals = {
            **KEDA_BASE,
            "keda.triggers.ttftP95.enabled": "true",
            "keda.triggers.ttftP95.threshold": "3",
            "keda.models.test-model.triggers.ttftP95.threshold": "1.5",
        }
        docs = helm_template(
            set_values=vals,
            show_only="charts/keda/templates/scaledobject.yaml",
        )
        sos = find_by_kind(docs, "ScaledObject")
        triggers = sos[0]["spec"]["triggers"]
        ttft_trigger = [t for t in triggers if "time_to_first_token" in t["metadata"]["query"]][0]
        assert ttft_trigger["metadata"]["threshold"] == "1.5"


class TestScaleToZero:
    """Test KEDA scale-to-zero configuration."""

    def test_scale_to_zero_disabled_by_default(self):
        docs = helm_template(
            set_values=KEDA_BASE,
            show_only="charts/keda/templates/scaledobject.yaml",
        )
        sos = find_by_kind(docs, "ScaledObject")
        assert sos[0]["spec"]["minReplicaCount"] == 1
        assert "idleReplicaCount" not in sos[0]["spec"]

    def test_scale_to_zero_enabled(self):
        vals = {
            **KEDA_BASE,
            "keda.models.test-model.scaleToZero.enabled": "true",
            "keda.models.test-model.scaleToZero.idleTimeout": "600",
        }
        docs = helm_template(
            set_values=vals,
            show_only="charts/keda/templates/scaledobject.yaml",
        )
        sos = find_by_kind(docs, "ScaledObject")
        assert sos[0]["spec"]["minReplicaCount"] == 0
        assert sos[0]["spec"]["idleReplicaCount"] == 0
        assert sos[0]["spec"]["advanced"]["horizontalPodAutoscalerConfig"]["behavior"]["scaleDown"]["stabilizationWindowSeconds"] == 600

    def test_scale_to_zero_activation_threshold(self):
        vals = {
            **KEDA_BASE,
            "keda.models.test-model.scaleToZero.enabled": "true",
        }
        docs = helm_template(
            set_values=vals,
            show_only="charts/keda/templates/scaledobject.yaml",
        )
        sos = find_by_kind(docs, "ScaledObject")
        triggers = sos[0]["spec"]["triggers"]
        queue_trigger = triggers[0]
        assert queue_trigger["metadata"]["activationThreshold"] == "1"


TWO_MODELS = {
    "global.models[0].name": "small-model",
    "global.models[0].source": "org/small-model",
    "global.models[0].resources.gpu": "1",
    "global.models[1].name": "big-model",
    "global.models[1].source": "org/big-model",
    "global.models[1].resources.gpu": "2",
}


class TestScaleToZeroFallback:
    """Test LiteLLM fallback config for scale-to-zero models."""

    def test_no_fallback_by_default(self):
        docs = helm_template(set_values=TWO_MODELS)
        config = get_configmap_data(docs, "litellm-config")
        for entry in config["model_list"]:
            assert "model_info" not in entry

    def test_fallback_rendered_when_set(self):
        vals = {
            **TWO_MODELS,
            "global.models[0].scaleToZero.fallbackModel": "big-model",
        }
        docs = helm_template(set_values=vals)
        config = get_configmap_data(docs, "litellm-config")
        small_entry = [e for e in config["model_list"] if e["model_name"] == "small-model"][0]
        assert small_entry["model_info"]["metadata"]["fallbacks"] == ["big-model"]


class TestSpotToleration:
    """Test spot/preemptible GPU tolerations."""

    def test_no_spot_tolerations_by_default(self):
        docs = helm_template(
            set_values=SINGLE_MODEL,
            show_only="charts/vllm/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment")
        tolerations = deps[0]["spec"]["template"]["spec"].get("tolerations", [])
        toleration_keys = [t["key"] for t in tolerations]
        assert "karpenter.sh/capacity-type" not in toleration_keys

    def test_spot_tolerations_added(self):
        vals = {**SINGLE_MODEL, "global.models[0].spotToleration": "true"}
        docs = helm_template(
            set_values=vals,
            show_only="charts/vllm/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment")
        tolerations = deps[0]["spec"]["template"]["spec"]["tolerations"]
        toleration_keys = [t["key"] for t in tolerations]
        assert "kubernetes.azure.com/scalesetpriority" in toleration_keys
        assert "cloud.google.com/gke-spot" in toleration_keys
        assert "karpenter.sh/capacity-type" in toleration_keys


class TestGracefulDrain:
    """Test terminationGracePeriodSeconds and preStop hook."""

    def test_termination_grace_period_set(self):
        docs = helm_template(
            set_values=SINGLE_MODEL,
            show_only="charts/vllm/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment")
        spec = deps[0]["spec"]["template"]["spec"]
        assert spec["terminationGracePeriodSeconds"] == 90

    def test_prestop_hook_exists(self):
        docs = helm_template(
            set_values=SINGLE_MODEL,
            show_only="charts/vllm/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment")
        container = deps[0]["spec"]["template"]["spec"]["containers"][0]
        prestop = container["lifecycle"]["preStop"]["exec"]["command"]
        assert "sleep" in " ".join(prestop)


DISAGG_MODEL = {
    "global.models[0].name": "big-model",
    "global.models[0].source": "org/big-model",
    "global.models[0].resources.gpu": "4",
    "global.models[0].resources.memory": "64Gi",
    "global.models[0].disaggregated.enabled": "true",
    "global.models[0].disaggregated.prefill.replicas": "2",
    "global.models[0].disaggregated.prefill.resources.gpu": "4",
    "global.models[0].disaggregated.prefill.resources.memory": "64Gi",
    "global.models[0].disaggregated.decode.replicas": "4",
    "global.models[0].disaggregated.decode.resources.gpu": "2",
    "global.models[0].disaggregated.decode.resources.memory": "32Gi",
}


class TestDisaggregatedServing:
    """Test llm-d disaggregated serving templates."""

    def test_no_disaggregated_resources_by_default(self):
        docs = helm_template(
            set_values=SINGLE_MODEL,
            show_only="charts/vllm/templates/disaggregated.yaml",
        )
        # Should render nothing when disaggregated is not enabled
        assert len(docs) == 0

    def test_disaggregated_creates_prefill_and_decode(self):
        docs = helm_template(
            set_values=DISAGG_MODEL,
            show_only="charts/vllm/templates/disaggregated.yaml",
        )
        deps = find_by_kind(docs, "Deployment")
        assert len(deps) == 2
        names = [d["metadata"]["name"] for d in deps]
        assert "big-model-prefill" in names
        assert "big-model-decode" in names

    def test_disaggregated_creates_inference_pool(self):
        docs = helm_template(
            set_values=DISAGG_MODEL,
            show_only="charts/vllm/templates/disaggregated.yaml",
        )
        pools = find_by_kind(docs, "InferencePool")
        assert len(pools) == 1
        assert pools[0]["metadata"]["name"] == "big-model-pool"

    def test_disaggregated_creates_inference_model(self):
        docs = helm_template(
            set_values=DISAGG_MODEL,
            show_only="charts/vllm/templates/disaggregated.yaml",
        )
        models = find_by_kind(docs, "InferenceModel")
        assert len(models) == 1
        assert models[0]["spec"]["modelName"] == "big-model"

    def test_epp_deployment_created(self):
        docs = helm_template(
            set_values=DISAGG_MODEL,
            show_only="charts/vllm/templates/epp.yaml",
        )
        deps = find_by_kind(docs, "Deployment")
        assert len(deps) == 1
        assert "epp" in deps[0]["metadata"]["name"]

    def test_no_epp_when_disabled(self):
        docs = helm_template(
            set_values=SINGLE_MODEL,
            show_only="charts/vllm/templates/epp.yaml",
        )
        assert len(docs) == 0


class TestMIGDevice:
    """Test MIG GPU device support."""

    def test_default_gpu_resource(self):
        docs = helm_template(
            set_values=SINGLE_MODEL,
            show_only="charts/vllm/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment")
        resources = deps[0]["spec"]["template"]["spec"]["containers"][0]["resources"]
        assert "nvidia.com/gpu" in resources["requests"]

    def test_mig_device_replaces_gpu(self):
        vals = {
            **SINGLE_MODEL,
            "global.models[0].resources.gpu": "0",
            "global.models[0].resources.migDevice": "nvidia.com/mig-1g.5gb",
        }
        docs = helm_template(
            set_values=vals,
            show_only="charts/vllm/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment")
        resources = deps[0]["spec"]["template"]["spec"]["containers"][0]["resources"]
        assert "nvidia.com/gpu" not in resources["requests"]
        assert resources["requests"]["nvidia.com/mig-1g.5gb"] == "1"


CANARY_MODEL = {
    "global.models[0].name": "test-model",
    "global.models[0].source": "org/test-model-v1",
    "global.models[0].resources.gpu": "1",
    "global.models[0].resources.memory": "16Gi",
    "global.models[0].canary.enabled": "true",
    "global.models[0].canary.source": "org/test-model-v2",
    "global.models[0].canary.weight": "10",
    "global.models[0].canary.replicas": "1",
}


class TestCanaryDeployment:
    """Test canary model deployment."""

    def test_no_canary_by_default(self):
        docs = helm_template(
            set_values=SINGLE_MODEL,
            show_only="charts/vllm/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment")
        assert len(deps) == 1
        assert "canary" not in deps[0]["metadata"]["name"]

    def test_canary_deployment_rendered(self):
        docs = helm_template(
            set_values=CANARY_MODEL,
            show_only="charts/vllm/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment")
        assert len(deps) == 2
        names = [d["metadata"]["name"] for d in deps]
        assert "vllm-test-model" in names
        assert "vllm-test-model-canary" in names

    def test_canary_service_rendered(self):
        docs = helm_template(
            set_values=CANARY_MODEL,
            show_only="charts/vllm/templates/service.yaml",
        )
        svcs = find_by_kind(docs, "Service")
        assert len(svcs) == 2
        names = [s["metadata"]["name"] for s in svcs]
        assert "vllm-test-model" in names
        assert "vllm-test-model-canary" in names

    def test_canary_litellm_weight_routing(self):
        docs = helm_template(set_values=CANARY_MODEL)
        config = get_configmap_data(docs, "litellm-config")
        entries = [e for e in config["model_list"] if e["model_name"] == "test-model"]
        assert len(entries) == 2
        weights = sorted([e["litellm_params"].get("weight", 100) for e in entries])
        assert weights == [10, 90]

    def test_canary_uses_different_source(self):
        docs = helm_template(
            set_values=CANARY_MODEL,
            show_only="charts/vllm/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment", "canary")
        assert len(deps) == 1
        args = deps[0]["spec"]["template"]["spec"]["containers"][0]["args"][0]
        assert "org--test-model-v2" in args
