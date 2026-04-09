"""Tests for unified module switches (modules.rag, modules.finetune, modules.security)."""
import subprocess, yaml, json, pytest

CHART = "charts/kube-llmops-stack"

def helm_template(set_values=None, show_only=None):
    cmd = ["helm", "template", "test", CHART]
    for k, v in (set_values or {}).items():
        cmd += ["--set", f"{k}={v}"]
    if show_only:
        cmd += ["-s", show_only]
    r = subprocess.run(cmd, capture_output=True, text=True)
    if r.returncode != 0:
        raise RuntimeError(f"helm template failed: {r.stderr}")
    docs = []
    for raw in r.stdout.split("---"):
        raw = raw.strip()
        if raw:
            try:
                docs.append(yaml.safe_load(raw))
            except yaml.YAMLError:
                pass
    return docs

def find_by_kind(docs, kind):
    return [d for d in docs if isinstance(d, dict) and d.get("kind") == kind]

def find_by_name(docs, name):
    return [d for d in docs if isinstance(d, dict) and d.get("metadata", {}).get("name") == name]

# Minimal base values to avoid GPU/model dependencies
# Module-controlled subcharts (dify, milvus, lightrag, rag-eval, finetune,
# jupyterhub, security) are NOT listed here — their lifecycle is controlled
# by global.modules.* via Chart.yaml dual-path conditions.
BASE = {
    "vllm.enabled": "false",
    "llamacpp.enabled": "false",
    "tei.enabled": "false",
    "litellm.enabled": "false",
    "observability.enabled": "false",
    "langfuse.enabled": "false",
    "logging.enabled": "false",
    "keycloak.enabled": "false",
    "fluid.enabled": "false",
    "postgresql.enabled": "false",
    "keda.enabled": "false",
    "harbor.enabled": "false",
    "headlamp.enabled": "false",
}


class TestRAGModuleSwitch:
    """global.modules.rag.enabled controls dify, milvus, lightrag, rag-eval."""

    def test_rag_module_off_by_default(self):
        """No RAG components rendered when modules.rag defaults to false."""
        docs = helm_template(set_values=BASE)
        names = [d["metadata"]["name"] for d in docs if isinstance(d, dict) and d.get("metadata")]
        for keyword in ["dify", "milvus", "lightrag", "rag-eval", "smoke-test", "ragas"]:
            assert not any(keyword in n for n in names), f"Found {keyword} resource when modules.rag not set"

    def test_rag_module_on_enables_all(self):
        """global.modules.rag.enabled=true brings up all RAG components."""
        vals = {**BASE, "global.modules.rag.enabled": "true"}
        docs = helm_template(set_values=vals)
        names = [d["metadata"]["name"] for d in docs if isinstance(d, dict) and d.get("metadata")]
        name_str = " ".join(names)
        assert "dify" in name_str, "dify not rendered"
        assert "milvus" in name_str, "milvus not rendered"
        assert "lightrag" in name_str, "lightrag not rendered"

    def test_explicit_override_disables_component(self):
        """global.modules.rag=true + milvus.enabled=false -> milvus off."""
        vals = {**BASE, "global.modules.rag.enabled": "true", "milvus.enabled": "false"}
        docs = helm_template(set_values=vals)
        names = [d["metadata"]["name"] for d in docs if isinstance(d, dict) and d.get("metadata")]
        name_str = " ".join(names)
        assert "milvus" not in name_str, "milvus should be disabled by explicit override"
        assert "dify" in name_str, "dify should still be on"

    def test_explicit_override_enables_component(self):
        """modules.rag=false + dify.enabled=true -> dify on."""
        vals = {**BASE, "dify.enabled": "true"}
        docs = helm_template(set_values=vals)
        names = [d["metadata"]["name"] for d in docs if isinstance(d, dict) and d.get("metadata")]
        name_str = " ".join(names)
        assert "dify" in name_str, "dify should be on via explicit override"
        assert "milvus" not in name_str, "milvus should be off (module off, no override)"


class TestFinetuneModuleSwitch:
    """global.modules.finetune.enabled controls finetune, jupyterhub."""

    def test_finetune_module_off_by_default(self):
        docs = helm_template(set_values=BASE)
        names = [d["metadata"]["name"] for d in docs if isinstance(d, dict) and d.get("metadata")]
        for keyword in ["finetune", "mlflow", "jupyterhub"]:
            assert not any(keyword in n for n in names), f"Found {keyword} when modules.finetune not set"

    def test_finetune_module_on(self):
        vals = {
            **BASE,
            "global.modules.finetune.enabled": "true",
            "finetune.baseModel": "Qwen/Qwen2.5-0.5B",
            "finetune.outputName": "test-ft",
            "finetune.dataSource.path": "s3://data/train.json",
        }
        docs = helm_template(set_values=vals)
        names = [d["metadata"]["name"] for d in docs if isinstance(d, dict) and d.get("metadata")]
        name_str = " ".join(names)
        assert "finetune" in name_str or "mlflow" in name_str, "finetune not rendered"
        assert "jupyterhub" in name_str, "jupyterhub not rendered"

    def test_finetune_explicit_override(self):
        vals = {
            **BASE,
            "global.modules.finetune.enabled": "true",
            "jupyterhub.enabled": "false",
            "finetune.baseModel": "Qwen/Qwen2.5-0.5B",
            "finetune.outputName": "test-ft",
            "finetune.dataSource.path": "s3://data/train.json",
        }
        docs = helm_template(set_values=vals)
        names = [d["metadata"]["name"] for d in docs if isinstance(d, dict) and d.get("metadata")]
        name_str = " ".join(names)
        assert "jupyterhub" not in name_str, "jupyterhub should be off by override"


class TestSecurityModuleSwitch:
    """global.modules.security.enabled controls security subchart."""

    def test_security_module_off_by_default(self):
        docs = helm_template(set_values=BASE)
        names = [d["metadata"]["name"] for d in docs if isinstance(d, dict) and d.get("metadata")]
        for keyword in ["llm-guard", "network-polic", "multi-tenant"]:
            assert not any(keyword in n for n in names), f"Found {keyword} when modules.security not set"

    def test_security_module_on(self):
        vals = {**BASE, "global.modules.security.enabled": "true"}
        docs = helm_template(set_values=vals)
        names = [d["metadata"]["name"] for d in docs if isinstance(d, dict) and d.get("metadata")]
        # Default security subchart renders NetworkPolicies (networkPolicy.enabled defaults to true)
        np = [d for d in docs if isinstance(d, dict) and d.get("kind") == "NetworkPolicy"]
        assert len(np) > 0, f"No NetworkPolicy resources found. Names: {names}"


class TestDashboardConditional:
    """Dashboard ConfigMap only includes module-relevant dashboards."""

    def _get_dashboard_keys(self, set_values):
        vals = {**BASE, "observability.enabled": "true", **set_values}
        docs = helm_template(set_values=vals)
        cms = find_by_kind(docs, "ConfigMap")
        dash_cm = [c for c in cms if "grafana-dashboards" in c["metadata"]["name"]
                   and "provision" not in c["metadata"]["name"]]
        assert len(dash_cm) == 1, f"Expected 1 dashboard ConfigMap, got {len(dash_cm)}"
        return set(dash_cm[0].get("data", {}).keys())

    def test_core_dashboards_always_present(self):
        keys = self._get_dashboard_keys({})
        for name in ["vllm-overview.json", "litellm-gateway.json", "system-overview.json",
                      "gpu-overview.json", "slo-overview.json", "cost-usage.json",
                      "infrastructure-roi.json"]:
            assert name in keys, f"Core dashboard {name} missing"

    def test_rag_dashboards_absent_when_off(self):
        keys = self._get_dashboard_keys({})
        assert "rag-quality.json" not in keys
        assert "milvus-overview.json" not in keys

    def test_rag_dashboards_present_when_on(self):
        keys = self._get_dashboard_keys({"global.modules.rag.enabled": "true"})
        assert "rag-quality.json" in keys
        assert "milvus-overview.json" in keys

    def test_finetune_dashboard_absent_when_off(self):
        keys = self._get_dashboard_keys({})
        assert "finetune-overview.json" not in keys

    def test_finetune_dashboard_present_when_on(self):
        keys = self._get_dashboard_keys({
            "global.modules.finetune.enabled": "true",
            "finetune.baseModel": "Qwen/Qwen2.5-0.5B",
            "finetune.outputName": "test-ft",
            "finetune.dataSource.path": "s3://data/train.json",
        })
        assert "finetune-overview.json" in keys

    def test_security_dashboard_absent_when_off(self):
        keys = self._get_dashboard_keys({})
        assert "tenant-overview.json" not in keys

    def test_security_dashboard_present_when_on(self):
        keys = self._get_dashboard_keys({"global.modules.security.enabled": "true"})
        assert "tenant-overview.json" in keys


class TestAlertConditional:
    """RAG alert rules only present when modules.rag is on."""

    def test_rag_alerts_absent_when_off(self):
        vals = {**BASE, "observability.enabled": "true",
                "global.modules.rag.enabled": "false"}
        docs = helm_template(set_values=vals)
        cms = find_by_kind(docs, "ConfigMap")
        all_data = ""
        for cm in cms:
            for v in cm.get("data", {}).values():
                if isinstance(v, str):
                    all_data += v
        assert "RAGFaithfulnessLow" not in all_data

    def test_rag_alerts_present_when_on(self):
        vals = {**BASE, "observability.enabled": "true",
                "global.modules.rag.enabled": "true"}
        docs = helm_template(set_values=vals)
        cms = find_by_kind(docs, "ConfigMap")
        all_data = ""
        for cm in cms:
            for v in cm.get("data", {}).values():
                if isinstance(v, str):
                    all_data += v
        assert "RAGFaithfulnessLow" in all_data

    def test_slo_alerts_always_present(self):
        """SLO alerts (TTFT/TPOT) are core, not module-gated."""
        vals = {**BASE, "observability.enabled": "true"}
        docs = helm_template(set_values=vals)
        cms = find_by_kind(docs, "ConfigMap")
        all_data = ""
        for cm in cms:
            for v in cm.get("data", {}).values():
                if isinstance(v, str):
                    all_data += v
        assert "TTFTSLOBreach" in all_data
