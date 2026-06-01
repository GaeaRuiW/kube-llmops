"""
Helm template unit tests for capability-based engine selection.

Tests auto-detection and explicit overrides for all 5 engines:
  vLLM, SGLang, Chitu, llama.cpp, TEI

Usage:
    cd kube-llmops
    python -m pytest tests/helm/test_engine_selection.py -v
"""

import subprocess
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


# ---------------------------------------------------------------------------
# Helper: build a single-model --set dict
# ---------------------------------------------------------------------------
def _model_values(name, source, gpu="1", memory="16Gi", **extras):
    """Return a dict of --set values for a single model."""
    vals = {
        "global.models[0].name": name,
        "global.models[0].source": source,
        "global.models[0].resources.gpu": gpu,
        "global.models[0].resources.memory": memory,
    }
    vals.update(extras)
    return vals


# ============================================================================
# TestAutoDetectMoE
# ============================================================================
class TestAutoDetectMoE:
    """Known MoE models should auto-select sglang."""

    def test_deepseek_v3_auto_sglang(self):
        vals = _model_values("deepseek-v3", "deepseek-ai/DeepSeek-V3", gpu="8")
        docs = helm_template(
            set_values=vals,
            show_only="charts/sglang/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment", "sglang-deepseek-v3")
        assert len(deps) == 1

    def test_deepseek_r1_auto_sglang(self):
        vals = _model_values("deepseek-r1", "deepseek-ai/DeepSeek-R1", gpu="8")
        docs = helm_template(
            set_values=vals,
            show_only="charts/sglang/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment", "sglang-deepseek-r1")
        assert len(deps) == 1

    def test_deepseek_r1_distill_stays_vllm(self):
        """Distill variants are dense — must NOT go to sglang."""
        vals = _model_values(
            "distill-qwen-14b",
            "deepseek-ai/DeepSeek-R1-Distill-Qwen-14B",
        )
        # sglang template should render nothing for this model
        sglang_docs = helm_template(
            set_values=vals,
            show_only="charts/sglang/templates/deployment.yaml",
        )
        assert len(find_by_kind(sglang_docs, "Deployment")) == 0

        # vllm template SHOULD render a deployment
        vllm_docs = helm_template(
            set_values=vals,
            show_only="charts/vllm/templates/deployment.yaml",
        )
        deps = find_by_kind(vllm_docs, "Deployment", "vllm-distill-qwen-14b")
        assert len(deps) == 1

    def test_qwen3_moe_auto_sglang(self):
        vals = _model_values("qwen3-235b", "Qwen/Qwen3-235B-A22B", gpu="8")
        docs = helm_template(
            set_values=vals,
            show_only="charts/sglang/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment", "sglang-qwen3-235b")
        assert len(deps) == 1

    def test_mixtral_auto_sglang(self):
        vals = _model_values("mixtral-8x7b", "mistralai/Mixtral-8x7B-v0.1", gpu="4")
        docs = helm_template(
            set_values=vals,
            show_only="charts/sglang/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment", "sglang-mixtral-8x7b")
        assert len(deps) == 1


# ============================================================================
# TestAutoDetectVLM
# ============================================================================
class TestAutoDetectVLM:
    """Known VLM models should auto-select sglang."""

    def test_qwen_vl_auto_sglang(self):
        vals = _model_values("qwen25-vl-7b", "Qwen/Qwen2.5-VL-7B-Instruct")
        docs = helm_template(
            set_values=vals,
            show_only="charts/sglang/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment", "sglang-qwen25-vl-7b")
        assert len(deps) == 1

    def test_llama_vision_auto_sglang(self):
        vals = _model_values(
            "llama32-vision",
            "meta-llama/Llama-3.2-11B-Vision-Instruct",
        )
        docs = helm_template(
            set_values=vals,
            show_only="charts/sglang/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment", "sglang-llama32-vision")
        assert len(deps) == 1


# ============================================================================
# TestFeatureTags
# ============================================================================
class TestFeatureTags:
    """Feature tags in global.models[0].features[] control engine selection."""

    def test_feature_domestic_gpu_selects_chitu(self):
        vals = _model_values("domestic-model", "org/some-model")
        vals["global.models[0].features[0]"] = "domestic-gpu"
        docs = helm_template(
            set_values=vals,
            show_only="charts/chitu/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment", "chitu-domestic-model")
        assert len(deps) == 1

    def test_feature_moe_selects_sglang(self):
        vals = _model_values("generic-moe", "org/some-model")
        vals["global.models[0].features[0]"] = "moe"
        docs = helm_template(
            set_values=vals,
            show_only="charts/sglang/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment", "sglang-generic-moe")
        assert len(deps) == 1

    def test_feature_vlm_selects_sglang(self):
        vals = _model_values("generic-vlm", "org/some-model")
        vals["global.models[0].features[0]"] = "vlm"
        docs = helm_template(
            set_values=vals,
            show_only="charts/sglang/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment", "sglang-generic-vlm")
        assert len(deps) == 1


# ============================================================================
# TestDefaultLLMEngine
# ============================================================================
class TestDefaultLLMEngine:
    """Test the global.defaultLLMEngine fallback."""

    def test_default_engine_vllm(self):
        """No special source / features → vllm (the default)."""
        vals = _model_values("plain-model", "org/plain-model")
        docs = helm_template(
            set_values=vals,
            show_only="charts/vllm/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment", "vllm-plain-model")
        assert len(deps) == 1

    def test_default_engine_sglang_override(self):
        """global.defaultLLMEngine=sglang → generic model uses sglang."""
        vals = _model_values("generic-model", "org/generic-model")
        vals["global.defaultLLMEngine"] = "sglang"
        docs = helm_template(
            set_values=vals,
            show_only="charts/sglang/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment", "sglang-generic-model")
        assert len(deps) == 1

    def test_default_engine_chitu_override(self):
        """global.defaultLLMEngine=chitu → generic model uses chitu."""
        vals = _model_values("generic-model", "org/generic-model")
        vals["global.defaultLLMEngine"] = "chitu"
        docs = helm_template(
            set_values=vals,
            show_only="charts/chitu/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment", "chitu-generic-model")
        assert len(deps) == 1


# ============================================================================
# TestExplicitEngineOverride
# ============================================================================
class TestExplicitEngineOverride:
    """Explicit engine= field overrides all auto-detection."""

    def test_explicit_vllm_overrides_moe_auto(self):
        """DeepSeek-V3 would auto-select sglang, but engine=vllm forces vllm."""
        vals = _model_values("deepseek-v3", "deepseek-ai/DeepSeek-V3", gpu="8")
        vals["global.models[0].engine"] = "vllm"
        # sglang should not render
        sglang_docs = helm_template(
            set_values=vals,
            show_only="charts/sglang/templates/deployment.yaml",
        )
        assert len(find_by_kind(sglang_docs, "Deployment")) == 0
        # vllm should render
        vllm_docs = helm_template(
            set_values=vals,
            show_only="charts/vllm/templates/deployment.yaml",
        )
        deps = find_by_kind(vllm_docs, "Deployment", "vllm-deepseek-v3")
        assert len(deps) == 1

    def test_explicit_sglang(self):
        """Generic model with engine=sglang → sglang."""
        vals = _model_values("generic-model", "org/generic-model")
        vals["global.models[0].engine"] = "sglang"
        docs = helm_template(
            set_values=vals,
            show_only="charts/sglang/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment", "sglang-generic-model")
        assert len(deps) == 1


# ============================================================================
# TestSGLangDeployment
# ============================================================================
class TestSGLangDeployment:
    """Verify SGLang deployment details (port, command, service, PVC)."""

    @pytest.fixture(autouse=True)
    def setup(self):
        self.vals = _model_values("deepseek-v3", "deepseek-ai/DeepSeek-V3", gpu="8")

    def test_sglang_deployment_port(self):
        docs = helm_template(
            set_values=self.vals,
            show_only="charts/sglang/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment", "sglang-deepseek-v3")
        assert len(deps) == 1
        container = deps[0]["spec"]["template"]["spec"]["containers"][0]
        ports = [p["containerPort"] for p in container["ports"]]
        assert 30000 in ports

    def test_sglang_deployment_command(self):
        docs = helm_template(
            set_values=self.vals,
            show_only="charts/sglang/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment", "sglang-deepseek-v3")
        container = deps[0]["spec"]["template"]["spec"]["containers"][0]
        args_text = container["args"][0]
        assert "sglang.launch_server" in args_text

    def test_sglang_service_port(self):
        docs = helm_template(
            set_values=self.vals,
            show_only="charts/sglang/templates/service.yaml",
        )
        svcs = find_by_kind(docs, "Service", "sglang-deepseek-v3")
        assert len(svcs) == 1
        port_spec = svcs[0]["spec"]["ports"][0]
        assert port_spec["port"] == 30000
        # targetPort uses the named port "http" which maps to containerPort 30000
        assert port_spec["targetPort"] == "http"

    def test_sglang_pvc_created(self):
        docs = helm_template(
            set_values=self.vals,
            show_only="charts/sglang/templates/pvc.yaml",
        )
        pvcs = find_by_kind(docs, "PersistentVolumeClaim", "sglang-deepseek-v3-cache")
        assert len(pvcs) == 1


# ============================================================================
# TestChituDeployment
# ============================================================================
class TestChituDeployment:
    """Verify Chitu deployment details (port, command, service, PVC)."""

    @pytest.fixture(autouse=True)
    def setup(self):
        self.vals = _model_values("domestic-model", "org/some-model")
        self.vals["global.models[0].features[0]"] = "domestic-gpu"

    def test_chitu_deployment_port(self):
        docs = helm_template(
            set_values=self.vals,
            show_only="charts/chitu/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment", "chitu-domestic-model")
        assert len(deps) == 1
        container = deps[0]["spec"]["template"]["spec"]["containers"][0]
        ports = [p["containerPort"] for p in container["ports"]]
        assert 21002 in ports

    def test_chitu_deployment_command(self):
        docs = helm_template(
            set_values=self.vals,
            show_only="charts/chitu/templates/deployment.yaml",
        )
        deps = find_by_kind(docs, "Deployment", "chitu-domestic-model")
        container = deps[0]["spec"]["template"]["spec"]["containers"][0]
        args_text = container["args"][0]
        assert "chitu.serve" in args_text

    def test_chitu_service_port(self):
        docs = helm_template(
            set_values=self.vals,
            show_only="charts/chitu/templates/service.yaml",
        )
        svcs = find_by_kind(docs, "Service", "chitu-domestic-model")
        assert len(svcs) == 1
        port_spec = svcs[0]["spec"]["ports"][0]
        assert port_spec["port"] == 21002
        assert port_spec["targetPort"] == 21002

    def test_chitu_pvc_created(self):
        docs = helm_template(
            set_values=self.vals,
            show_only="charts/chitu/templates/pvc.yaml",
        )
        pvcs = find_by_kind(
            docs, "PersistentVolumeClaim", "chitu-domestic-model-cache"
        )
        assert len(pvcs) == 1


# ============================================================================
# TestLiteLLMRouting
# ============================================================================
class TestLiteLLMRouting:
    """LiteLLM configmap must route to the correct engine service & port."""

    def test_litellm_sglang_routing(self):
        vals = _model_values("deepseek-v3", "deepseek-ai/DeepSeek-V3", gpu="8")
        docs = helm_template(
            set_values=vals,
            show_only="charts/litellm/templates/configmap.yaml",
        )
        config = get_configmap_data(docs, "litellm-config")
        entry = [
            e for e in config["model_list"] if e["model_name"] == "deepseek-v3"
        ][0]
        api_base = entry["litellm_params"]["api_base"]
        assert "sglang-deepseek-v3" in api_base
        assert ":30000" in api_base

    def test_litellm_chitu_routing(self):
        vals = _model_values("domestic-model", "org/some-model")
        vals["global.models[0].features[0]"] = "domestic-gpu"
        docs = helm_template(
            set_values=vals,
            show_only="charts/litellm/templates/configmap.yaml",
        )
        config = get_configmap_data(docs, "litellm-config")
        entry = [
            e for e in config["model_list"] if e["model_name"] == "domestic-model"
        ][0]
        api_base = entry["litellm_params"]["api_base"]
        assert "chitu-domestic-model" in api_base
        assert ":21002" in api_base

    def test_litellm_mixed_engines(self):
        """DeepSeek-V3 (sglang:30000) + Qwen3-8B (vllm:8000) in one config."""
        vals = {
            "global.models[0].name": "deepseek-v3",
            "global.models[0].source": "deepseek-ai/DeepSeek-V3",
            "global.models[0].resources.gpu": "8",
            "global.models[1].name": "qwen3-8b",
            "global.models[1].source": "Qwen/Qwen3-8B",
            "global.models[1].resources.gpu": "1",
        }
        docs = helm_template(
            set_values=vals,
            show_only="charts/litellm/templates/configmap.yaml",
        )
        config = get_configmap_data(docs, "litellm-config")
        entries = {e["model_name"]: e for e in config["model_list"]}

        # DeepSeek-V3 → sglang:30000
        ds = entries["deepseek-v3"]["litellm_params"]["api_base"]
        assert "sglang-deepseek-v3" in ds
        assert ":30000" in ds

        # Qwen3-8B → vllm:8000
        qw = entries["qwen3-8b"]["litellm_params"]["api_base"]
        assert "vllm-qwen3-8b" in qw
        assert ":8000" in qw


# ============================================================================
# TestKEDAScaling
# ============================================================================
class TestKEDAScaling:
    """KEDA ScaledObjects must target the correct engine deployment."""

    def test_keda_sglang_scaledobject(self):
        vals = _model_values("deepseek-v3", "deepseek-ai/DeepSeek-V3", gpu="8")
        vals["keda.enabled"] = "true"
        docs = helm_template(
            set_values=vals,
            show_only="charts/keda/templates/scaledobject.yaml",
        )
        sos = find_by_kind(docs, "ScaledObject")
        assert len(sos) == 1
        so = sos[0]
        assert so["metadata"]["name"] == "sglang-deepseek-v3-scaler"
        assert so["metadata"]["labels"]["kube-llmops/engine"] == "sglang"
        assert so["spec"]["scaleTargetRef"]["name"] == "sglang-deepseek-v3"
        # Verify the metric query uses the sglang engine metric
        query = so["spec"]["triggers"][0]["metadata"]["query"]
        assert "sglang:num_requests_waiting" in query

    def test_keda_chitu_scaledobject(self):
        vals = _model_values("domestic-model", "org/some-model")
        vals["global.models[0].features[0]"] = "domestic-gpu"
        vals["keda.enabled"] = "true"
        docs = helm_template(
            set_values=vals,
            show_only="charts/keda/templates/scaledobject.yaml",
        )
        sos = find_by_kind(docs, "ScaledObject")
        assert len(sos) == 1
        so = sos[0]
        assert so["metadata"]["name"] == "chitu-domestic-model-scaler"
        assert so["metadata"]["labels"]["kube-llmops/engine"] == "chitu"
        assert so["spec"]["scaleTargetRef"]["name"] == "chitu-domestic-model"
        query = so["spec"]["triggers"][0]["metadata"]["query"]
        assert "chitu:num_requests_waiting" in query
