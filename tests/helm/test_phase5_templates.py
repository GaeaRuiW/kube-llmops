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
