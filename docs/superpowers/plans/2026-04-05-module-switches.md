# Module Switches Implementation Plan

> **STATUS: COMPLETED** — Implemented in v0.5.0. All tasks executed successfully.
> See: `charts/kube-llmops-stack/Chart.yaml` (dual-path conditions), `tests/helm/test_module_switches.py` (19 tests passing)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `modules.rag`, `modules.finetune`, `modules.security` switches that control entire feature groups (services + dashboards + alerts) with one flag, while preserving per-component override capability.

**Architecture:** Uses Helm's native dual-path `condition` in Chart.yaml (`component.enabled,modules.<module>.enabled`). Removes `enabled` from subchart defaults so the fallback path triggers. Dashboard ConfigMap moves to parent chart for direct module-state access. Prometheus RAG alerts gated behind `global.modules.rag`.

**Tech Stack:** Helm 3 templates, YAML values, pytest + subprocess for tests.

---

### Task 1: Write tests for module switches

**Files:**
- Create: `tests/helm/test_module_switches.py`

- [ ] **Step 1: Create test file with helpers and first test class**

```python
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
    return [d for d in docs if d and d.get("kind") == kind]

def find_by_name(docs, name):
    return [d for d in docs if d and d.get("metadata", {}).get("name") == name]

# Minimal base values to avoid GPU/model dependencies
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
}


class TestRAGModuleSwitch:
    """modules.rag.enabled controls dify, milvus, lightrag, rag-eval."""

    def test_rag_module_off_by_default(self):
        """No RAG components rendered when modules.rag not set."""
        docs = helm_template(set_values=BASE)
        names = [d["metadata"]["name"] for d in docs if d and d.get("metadata")]
        for keyword in ["dify", "milvus", "lightrag", "rag-eval", "smoke-test", "ragas"]:
            assert not any(keyword in n for n in names), f"Found {keyword} resource when modules.rag not set"

    def test_rag_module_on_enables_all(self):
        """modules.rag.enabled=true brings up all RAG components."""
        vals = {**BASE, "modules.rag.enabled": "true"}
        docs = helm_template(set_values=vals)
        names = [d["metadata"]["name"] for d in docs if d and d.get("metadata")]
        name_str = " ".join(names)
        assert "dify" in name_str, "dify not rendered"
        assert "milvus" in name_str, "milvus not rendered"
        assert "lightrag" in name_str, "lightrag not rendered"

    def test_explicit_override_disables_component(self):
        """modules.rag=true + milvus.enabled=false → milvus off."""
        vals = {**BASE, "modules.rag.enabled": "true", "milvus.enabled": "false"}
        docs = helm_template(set_values=vals)
        names = [d["metadata"]["name"] for d in docs if d and d.get("metadata")]
        name_str = " ".join(names)
        assert "milvus" not in name_str, "milvus should be disabled by explicit override"
        assert "dify" in name_str, "dify should still be on"

    def test_explicit_override_enables_component(self):
        """modules.rag=false + dify.enabled=true → dify on."""
        vals = {**BASE, "dify.enabled": "true"}
        docs = helm_template(set_values=vals)
        names = [d["metadata"]["name"] for d in docs if d and d.get("metadata")]
        name_str = " ".join(names)
        assert "dify" in name_str, "dify should be on via explicit override"
        assert "milvus" not in name_str, "milvus should be off (module off, no override)"


class TestFinetuneModuleSwitch:
    """modules.finetune.enabled controls finetune, jupyterhub."""

    def test_finetune_module_off_by_default(self):
        docs = helm_template(set_values=BASE)
        names = [d["metadata"]["name"] for d in docs if d and d.get("metadata")]
        for keyword in ["finetune", "mlflow", "jupyterhub"]:
            assert not any(keyword in n for n in names), f"Found {keyword} when modules.finetune not set"

    def test_finetune_module_on(self):
        vals = {**BASE, "modules.finetune.enabled": "true"}
        docs = helm_template(set_values=vals)
        names = [d["metadata"]["name"] for d in docs if d and d.get("metadata")]
        name_str = " ".join(names)
        assert "finetune" in name_str or "mlflow" in name_str, "finetune not rendered"
        assert "jupyterhub" in name_str, "jupyterhub not rendered"

    def test_finetune_explicit_override(self):
        vals = {**BASE, "modules.finetune.enabled": "true", "jupyterhub.enabled": "false"}
        docs = helm_template(set_values=vals)
        names = [d["metadata"]["name"] for d in docs if d and d.get("metadata")]
        name_str = " ".join(names)
        assert "jupyterhub" not in name_str, "jupyterhub should be off by override"


class TestSecurityModuleSwitch:
    """modules.security.enabled controls security subchart."""

    def test_security_module_off_by_default(self):
        docs = helm_template(set_values=BASE)
        names = [d["metadata"]["name"] for d in docs if d and d.get("metadata")]
        for keyword in ["llm-guard", "network-policy", "multi-tenant"]:
            assert not any(keyword in n for n in names), f"Found {keyword} when modules.security not set"

    def test_security_module_on(self):
        vals = {**BASE, "modules.security.enabled": "true"}
        docs = helm_template(set_values=vals)
        names = [d["metadata"]["name"] for d in docs if d and d.get("metadata")]
        name_str = " ".join(names)
        assert any("security" in n or "llm-guard" in n or "network-polic" in n for n in names), \
            "No security resources found"


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
        keys = self._get_dashboard_keys({"modules.rag.enabled": "true"})
        assert "rag-quality.json" in keys
        assert "milvus-overview.json" in keys

    def test_finetune_dashboard_absent_when_off(self):
        keys = self._get_dashboard_keys({})
        assert "finetune-overview.json" not in keys

    def test_finetune_dashboard_present_when_on(self):
        keys = self._get_dashboard_keys({"modules.finetune.enabled": "true"})
        assert "finetune-overview.json" in keys

    def test_security_dashboard_absent_when_off(self):
        keys = self._get_dashboard_keys({})
        assert "tenant-overview.json" not in keys

    def test_security_dashboard_present_when_on(self):
        keys = self._get_dashboard_keys({"modules.security.enabled": "true"})
        assert "tenant-overview.json" in keys


class TestAlertConditional:
    """RAG alert rules only present when modules.rag is on."""

    def _get_prom_config(self, set_values):
        vals = {**BASE, "observability.enabled": "true", **set_values}
        docs = helm_template(set_values=vals)
        cms = find_by_kind(docs, "ConfigMap")
        prom_cm = [c for c in cms if "prometheus-config" in c["metadata"]["name"]
                   or c["metadata"]["name"].endswith("-prometheus")]
        # Prometheus config is in the ConfigMap data
        for cm in prom_cm:
            for key, val in cm.get("data", {}).items():
                if "rules" in key.lower() or "rule" in key.lower():
                    return val
                if "prometheus.yml" in key:
                    continue
        return ""

    def test_rag_alerts_absent_when_off(self):
        vals = {"observability.enabled": "true",
                "global.modules.rag": "false"}
        docs = helm_template(set_values={**BASE, **vals})
        cms = find_by_kind(docs, "ConfigMap")
        all_data = ""
        for cm in cms:
            for v in cm.get("data", {}).values():
                if isinstance(v, str):
                    all_data += v
        assert "RAGFaithfulnessLow" not in all_data

    def test_rag_alerts_present_when_on(self):
        vals = {"observability.enabled": "true",
                "global.modules.rag": "true"}
        docs = helm_template(set_values={**BASE, **vals})
        cms = find_by_kind(docs, "ConfigMap")
        all_data = ""
        for cm in cms:
            for v in cm.get("data", {}).values():
                if isinstance(v, str):
                    all_data += v
        assert "RAGFaithfulnessLow" in all_data
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
python3 -m pytest tests/helm/test_module_switches.py -v 2>&1 | tail -30
```

Expected: All tests FAIL (module switches not yet implemented).

- [ ] **Step 3: Commit test file**

```bash
git add tests/helm/test_module_switches.py
git commit -m "test: add module switch tests (TDD, all failing)"
```

---

### Task 2: Update Chart.yaml with dual-path conditions

**Files:**
- Modify: `charts/kube-llmops-stack/Chart.yaml`

- [ ] **Step 1: Change conditions for module-grouped subcharts**

In `Chart.yaml`, change the `condition` field for these 7 subcharts:

```
security:   condition: security.enabled        → condition: security.enabled,modules.security.enabled
milvus:     condition: milvus.enabled          → condition: milvus.enabled,modules.rag.enabled
dify:       condition: dify.enabled            → condition: dify.enabled,modules.rag.enabled
rag-eval:   condition: rag-eval.enabled        → condition: rag-eval.enabled,modules.rag.enabled
lightrag:   condition: lightrag.enabled        → condition: lightrag.enabled,modules.rag.enabled
finetune:   condition: finetune.enabled        → condition: finetune.enabled,modules.finetune.enabled
jupyterhub: condition: jupyterhub.enabled      → condition: jupyterhub.enabled,modules.finetune.enabled
```

Keep all other subcharts unchanged.

- [ ] **Step 2: Rebuild chart archives**

```bash
cd charts/kube-llmops-stack && rm -f charts/*.tgz Chart.lock && helm dependency update .
```

- [ ] **Step 3: Commit**

```bash
git add charts/kube-llmops-stack/Chart.yaml charts/kube-llmops-stack/Chart.lock
git commit -m "feat: dual-path conditions in Chart.yaml for module switches"
```

---

### Task 3: Remove `enabled` from subchart defaults and template guards

**Files:**
- Modify: `charts/kube-llmops-stack/charts/dify/values.yaml` — remove line `enabled: false`
- Modify: `charts/kube-llmops-stack/charts/milvus/values.yaml` — remove line `enabled: false`
- Modify: `charts/kube-llmops-stack/charts/lightrag/values.yaml` — remove line `enabled: false`
- Modify: `charts/kube-llmops-stack/charts/rag-eval/values.yaml` — remove line `enabled: false`
- Modify: `charts/kube-llmops-stack/charts/finetune/values.yaml` — remove line `enabled: false`
- Modify: `charts/kube-llmops-stack/charts/security/values.yaml` — remove line `enabled: false`
- Modify: `charts/kube-llmops-stack/charts/jupyterhub/values.yaml` — remove line `enabled: false`
- Modify: `charts/kube-llmops-stack/values.yaml` — remove `enabled` for module-grouped components
- Modify: subchart templates — remove `{{- if .Values.enabled }}` / `{{- end }}` guards

- [ ] **Step 1: Remove `enabled: false` from 7 subchart values.yaml files**

For each file, delete the first line `enabled: false` (and the blank line after it if present):
- `charts/kube-llmops-stack/charts/dify/values.yaml`
- `charts/kube-llmops-stack/charts/milvus/values.yaml`
- `charts/kube-llmops-stack/charts/lightrag/values.yaml`
- `charts/kube-llmops-stack/charts/rag-eval/values.yaml`
- `charts/kube-llmops-stack/charts/finetune/values.yaml`
- `charts/kube-llmops-stack/charts/security/values.yaml`
- `charts/kube-llmops-stack/charts/jupyterhub/values.yaml`

- [ ] **Step 2: Remove `enabled` from parent values.yaml for module-grouped components**

In `charts/kube-llmops-stack/values.yaml`, remove the `enabled: false` line from these sections (keep the section header and other config):
- `milvus:` section (line ~269-270)
- `lightrag:` section (line ~272-274)
- `dify:` section (line ~276-278)
- `security:` section (line ~284-286)
- `jupyterhub:` section (line ~301-303)

Note: `finetune` and `rag-eval` don't appear in parent values.yaml — they only have subchart defaults.

- [ ] **Step 3: Add `modules` block to parent values.yaml**

Add after the `global:` section (after line 7):

```yaml
# -- Module switches (feature group toggles)
# Each module controls a group of related subcharts.
# Individual component overrides (e.g. dify.enabled=false) always take priority.
modules:
  rag:
    enabled: false              # dify, milvus, lightrag, rag-eval
  finetune:
    enabled: false              # finetune, jupyterhub
  security:
    enabled: false              # llm-guard, presidio, networkPolicy, multiTenant

# -- Global values passed to all subcharts
```

Also add `global.modules` for cross-chart access (observability needs it):

```yaml
global:
  gpu: true
  imagePullPolicy: IfNotPresent
  modules:
    rag: false
    finetune: false
    security: false
```

- [ ] **Step 4: Remove `{{- if .Values.enabled }}` guards from subchart templates**

These files have `{{- if .Values.enabled }}` on line 1 and `{{- end }}` at the end. Remove both lines:

- `charts/kube-llmops-stack/charts/dify/templates/dify.yaml` — remove line 1 (`{{- if .Values.enabled }}`) and last line (`{{- end }}`)
- `charts/kube-llmops-stack/charts/dify/templates/pdb.yaml` — same
- `charts/kube-llmops-stack/charts/milvus/templates/milvus.yaml` — same
- `charts/kube-llmops-stack/charts/milvus/templates/pdb.yaml` — same
- `charts/kube-llmops-stack/charts/lightrag/templates/lightrag.yaml` — same
- `charts/kube-llmops-stack/charts/lightrag/templates/pdb.yaml` — same
- `charts/kube-llmops-stack/charts/finetune/templates/configmap-train.yaml` — same
- `charts/kube-llmops-stack/charts/finetune/templates/mlflow.yaml` — same
- `charts/kube-llmops-stack/charts/finetune/templates/pdb.yaml` — same
- `charts/kube-llmops-stack/charts/finetune/templates/rbac.yaml` — same
- `charts/kube-llmops-stack/charts/jupyterhub/templates/jupyterhub.yaml` — same

**Important:** Only remove the OUTER `{{- if .Values.enabled }}` / `{{- end }}` pair. Don't touch inner conditionals like `{{- if .Values.setup.enabled }}`.

- [ ] **Step 5: Rebuild chart archives**

```bash
cd charts/kube-llmops-stack && rm -f charts/*.tgz Chart.lock && helm dependency update .
```

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat: remove enabled from subchart defaults for module fallback"
```

---

### Task 4: Conditional dashboard ConfigMap

**Files:**
- Create: `charts/kube-llmops-stack/templates/grafana-dashboards.yaml`
- Modify: `charts/kube-llmops-stack/charts/observability/templates/grafana.yaml` — remove old dashboard ConfigMap

- [ ] **Step 1: Create conditional dashboard ConfigMap in parent chart**

Create `charts/kube-llmops-stack/templates/grafana-dashboards.yaml`:

```yaml
{{- if ((.Values.observability).grafana).enabled }}
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-grafana-dashboards
  labels:
    app.kubernetes.io/name: grafana
    app.kubernetes.io/instance: {{ .Release.Name }}
    app.kubernetes.io/part-of: kube-llmops
data:
  # Core dashboards (always included)
  {{- $dashDir := "charts/observability/dashboards" }}
  vllm-overview.json: |
    {{- .Files.Get (printf "%s/%s" $dashDir "vllm-overview.json") | nindent 4 }}
  litellm-gateway.json: |
    {{- .Files.Get (printf "%s/%s" $dashDir "litellm-gateway.json") | nindent 4 }}
  slo-overview.json: |
    {{- .Files.Get (printf "%s/%s" $dashDir "slo-overview.json") | nindent 4 }}
  system-overview.json: |
    {{- .Files.Get (printf "%s/%s" $dashDir "system-overview.json") | nindent 4 }}
  gpu-overview.json: |
    {{- .Files.Get (printf "%s/%s" $dashDir "gpu-overview.json") | nindent 4 }}
  cost-usage.json: |
    {{- .Files.Get (printf "%s/%s" $dashDir "cost-usage.json") | nindent 4 }}
  infrastructure-roi.json: |
    {{- .Files.Get (printf "%s/%s" $dashDir "infrastructure-roi.json") | nindent 4 }}
  # RAG module dashboards
  {{- if ((.Values.modules).rag).enabled }}
  rag-quality.json: |
    {{- .Files.Get (printf "%s/%s" $dashDir "rag-quality.json") | nindent 4 }}
  milvus-overview.json: |
    {{- .Files.Get (printf "%s/%s" $dashDir "milvus-overview.json") | nindent 4 }}
  {{- end }}
  # Finetune module dashboard
  {{- if ((.Values.modules).finetune).enabled }}
  finetune-overview.json: |
    {{- .Files.Get (printf "%s/%s" $dashDir "finetune-overview.json") | nindent 4 }}
  {{- end }}
  # Security module dashboard
  {{- if ((.Values.modules).security).enabled }}
  tenant-overview.json: |
    {{- .Files.Get (printf "%s/%s" $dashDir "tenant-overview.json") | nindent 4 }}
  {{- end }}
{{- end }}
```

- [ ] **Step 2: Remove old dashboard ConfigMap from observability grafana.yaml**

In `charts/kube-llmops-stack/charts/observability/templates/grafana.yaml`, delete lines 195-206 (the `---` separator and the entire `ConfigMap` block with `.Files.Glob`):

```yaml
# DELETE THIS BLOCK (lines 195-206):
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ $.Release.Name }}-grafana-dashboards
  ...
data:
  {{- (.Files.Glob "dashboards/*.json").AsConfig | nindent 2 }}
{{- end }}
```

Replace with just the closing `{{- end }}` for the outer `{{- if .Values.grafana.enabled }}` block:

```yaml
{{- end }}
```

- [ ] **Step 3: Rebuild chart archives**

```bash
cd charts/kube-llmops-stack && rm -f charts/*.tgz Chart.lock && helm dependency update .
```

- [ ] **Step 4: Quick validation — render and check dashboards**

```bash
helm template test charts/kube-llmops-stack \
  --set observability.enabled=true \
  --set modules.rag.enabled=true \
  2>&1 | grep -c 'rag-quality'
# Should be >= 1

helm template test charts/kube-llmops-stack \
  --set observability.enabled=true \
  2>&1 | grep -c 'rag-quality'
# Should be 0
```

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: conditional dashboard ConfigMap based on module switches"
```

---

### Task 5: Conditional Prometheus alert rules

**Files:**
- Modify: `charts/kube-llmops-stack/charts/observability/templates/prometheus.yaml`

- [ ] **Step 1: Wrap RAG alert groups with global.modules.rag conditional**

In `prometheus.yaml`, wrap the `rag-quality-alerts` group (lines ~147-188) and the two RAG alerts in `vllm-alerts` group (RAGHighRetrievalLatency, RAGEmbeddingServiceDown) with a conditional.

For the `rag-quality-alerts` group (lines 147-188), wrap the entire group:

```yaml
      {{- if .Values.global.modules.rag }}
      - name: rag-quality-alerts
        rules:
          ... (keep all existing RAG alerts)
      {{- end }}
```

For the two RAG alerts in `vllm-alerts` group (lines 133-146: RAGHighRetrievalLatency + RAGEmbeddingServiceDown), wrap them:

```yaml
          {{- if .Values.global.modules.rag }}
          - alert: RAGHighRetrievalLatency
            ...
          - alert: RAGEmbeddingServiceDown
            ...
          {{- end }}
```

- [ ] **Step 2: Rebuild chart archives**

```bash
cd charts/kube-llmops-stack && rm -f charts/*.tgz Chart.lock && helm dependency update .
```

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "feat: conditional Prometheus alerts based on module switches"
```

---

### Task 6: Update values-single-node.yaml and values-ci.yaml

**Files:**
- Modify: `charts/kube-llmops-stack/values-single-node.yaml`
- Modify: `charts/kube-llmops-stack/values-ci.yaml`

- [ ] **Step 1: Update values-single-node.yaml**

Add `modules` block and `global.modules` near the top, and remove individual `enabled:` lines for module-grouped components.

Add after the `global:` section (around line 6, after `imagePullPolicy: IfNotPresent`):

```yaml
  modules:
    rag: true
    finetune: false
    security: true

modules:
  rag:
    enabled: true
  finetune:
    enabled: false
  security:
    enabled: true
```

Remove `enabled: true` from these sections (keep all other config):
- `dify:` section — remove `enabled: true` line
- `milvus:` section — remove `enabled: true` line
- `lightrag:` section — remove `enabled: true` line
- `rag-eval:` section — remove `enabled: true` line
- `security:` section — remove `enabled: true` line
- `finetune:` section — remove `enabled: false` line

- [ ] **Step 2: Update values-ci.yaml**

Add `modules` block (all off) near the top and remove individual enables for module-grouped components:

```yaml
modules:
  rag:
    enabled: false
  finetune:
    enabled: false
  security:
    enabled: false
```

Remove these lines from values-ci.yaml:
- `security: enabled: false`
- `milvus: enabled: false`
- `dify: enabled: false`

(finetune, jupyterhub, rag-eval, lightrag are not in values-ci.yaml currently — they inherit defaults of disabled.)

- [ ] **Step 3: Rebuild chart archives**

```bash
cd charts/kube-llmops-stack && rm -f charts/*.tgz Chart.lock && helm dependency update .
```

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "feat: update value profiles with module switches"
```

---

### Task 7: Run tests and fix failures

**Files:**
- Possible fixes in any of the above files

- [ ] **Step 1: Run module switch tests**

```bash
python3 -m pytest tests/helm/test_module_switches.py -v 2>&1
```

- [ ] **Step 2: Fix any failures**

Debug and fix test failures. Common issues:
- Dashboard ConfigMap name mismatch (test expects `grafana-dashboards`, template uses different name)
- `.Files.Get` path wrong for parent chart accessing subchart files
- Alert conditional syntax issues
- Values merge order issues

- [ ] **Step 3: Run ALL tests (regression check)**

```bash
python3 -m pytest tests/helm/ -v 2>&1
```

All 74 existing tests + new module switch tests must pass.

- [ ] **Step 4: Commit fixes**

```bash
git add -A
git commit -m "fix: module switch test fixes"
```

---

### Task 8: Verify with helm template renders

- [ ] **Step 1: Render with values-single-node.yaml and verify**

```bash
# Full render should succeed
helm template test charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  --set global.nodePort.enabled=true --set global.nodePort.host=10.0.0.1 \
  2>&1 | grep -c '^kind:'
# Should be ~156 resources

# RAG dashboards should be present (modules.rag.enabled=true)
helm template test charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  --set global.nodePort.enabled=true --set global.nodePort.host=10.0.0.1 \
  2>&1 | grep 'rag-quality'
# Should find matches

# Finetune dashboard should NOT be present (modules.finetune.enabled=false)
helm template test charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  --set global.nodePort.enabled=true --set global.nodePort.host=10.0.0.1 \
  2>&1 | grep 'finetune-overview'
# Should find 0 matches
```

- [ ] **Step 2: Render with values-ci.yaml**

```bash
helm template test charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-ci.yaml \
  2>&1 | grep -c '^kind:'
# Should succeed with fewer resources (no RAG, no finetune, no security)
```

- [ ] **Step 3: Test module + override combo**

```bash
# Module on + component off
helm template test charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  --set milvus.enabled=false \
  2>&1 | grep milvus
# Should find 0 matches
```

---

### Task 9: Update docs and commit

**Files:**
- Modify: `AGENTS.md`
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Add module switches to AGENTS.md**

Add a new section after "### Advanced Inference (v0.5.0)":

```markdown
### Module Switches
Enable/disable entire feature groups with one flag:
```bash
# Enable RAG (dify + milvus + lightrag + rag-eval + dashboards + alerts)
--set modules.rag.enabled=true

# Enable Fine-tuning (finetune + jupyterhub + dashboard)
--set modules.finetune.enabled=true

# Enable Security (llm-guard + presidio + networkPolicy + dashboard)
--set modules.security.enabled=true
```
Individual component overrides always take priority:
```bash
# RAG on but skip Milvus
--set modules.rag.enabled=true --set milvus.enabled=false

# Only Dify, no other RAG components
--set dify.enabled=true
```
```

- [ ] **Step 2: Add CHANGELOG entry**

Add to CHANGELOG.md under `## [0.5.0]` → `### Added`:

```markdown
- Unified module switches (`modules.rag`, `modules.finetune`, `modules.security`) — one flag controls all services, dashboards, and alert rules for each feature group
- Conditional dashboard provisioning (disabled modules' dashboards no longer appear in Grafana)
- Conditional Prometheus alert rules (RAG alerts only present when `modules.rag.enabled`)
```

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "feat: unified module switches (rag, finetune, security)

Adds modules.rag.enabled, modules.finetune.enabled, modules.security.enabled
as top-level feature group toggles. Sub-component overrides always take priority.
Dashboards and Prometheus alerts are conditionally included per module state."
```
