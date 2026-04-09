# LLaMA-Factory Fine-tuning Pipeline — Implementation Plan

> **STATUS: COMPLETED** — Implemented in v0.4.0. All tasks executed successfully.
> See: `charts/kube-llmops-stack/charts/finetune/`, `tests/helm/test_finetune_templates.py`, `tests/e2e/test_finetune_e2e.py`

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a production-grade fine-tuning pipeline (LLaMA-Factory + Argo Workflows + MLflow) to kube-llmops as a new Helm subchart.

**Architecture:** New `finetune/` subchart containing MLflow Deployment, Argo WorkflowTemplate (6-step DAG), and supporting RBAC/ConfigMap/PDB. Reuses existing PostgreSQL (+mlflow DB), MinIO (+mlflow,datasets buckets), and Pushgateway. Canary deployment via LiteLLM weight routing.

**Tech Stack:** Helm templates (Go), Argo Workflows CRD, MLflow 2.21.3, LLaMA-Factory, Python scripts for pipeline steps.

**Spec:** `docs/superpowers/specs/2026-03-30-llamafactory-finetune-design.md`

---

## File Structure

```
charts/kube-llmops-stack/
  Chart.yaml                                    # MODIFY: add finetune dependency
  values-single-node.yaml                       # MODIFY: add finetune + mlflow sections, mlflow DB, buckets
  templates/
    nodeport-services.yaml                      # MODIFY: add MLflow NodePort :30505
  charts/finetune/
    Chart.yaml                                  # CREATE
    values.yaml                                 # CREATE
    templates/
      mlflow.yaml                               # CREATE: MLflow Deployment + Service
      workflow.yaml                             # CREATE: Argo WorkflowTemplate (6-step DAG)
      cronworkflow.yaml                         # CREATE: CronWorkflow (if schedule set)
      configmap-train.yaml                      # CREATE: LLaMA-Factory training config
      rbac.yaml                                 # CREATE: ServiceAccount + ClusterRole for kubectl
      pdb.yaml                                  # CREATE: PDB for MLflow
  charts/observability/dashboards/
    finetune-overview.json                      # CREATE: Grafana dashboard
```

---

### Task 1: Create finetune subchart scaffold

**Files:**
- Create: `charts/kube-llmops-stack/charts/finetune/Chart.yaml`
- Create: `charts/kube-llmops-stack/charts/finetune/values.yaml`
- Modify: `charts/kube-llmops-stack/Chart.yaml`

- [ ] **Step 1: Create Chart.yaml**

```yaml
# charts/kube-llmops-stack/charts/finetune/Chart.yaml
apiVersion: v2
name: finetune
description: LLaMA-Factory fine-tuning pipeline with Argo Workflows and MLflow
version: 0.1.0
type: application
```

- [ ] **Step 2: Create values.yaml with full schema**

Create `charts/kube-llmops-stack/charts/finetune/values.yaml` with the complete default values from the spec (Section 2). Include all fields: `enabled`, `baseModel`, `outputName`, `method`, `loraRank`, `loraAlpha`, `epochs`, `batchSize`, `learningRate`, `dataSource`, `resources`, `nodeSelector`, `tolerations`, `evaluation`, `deploy`, `schedule`, and the `mlflow` section.

- [ ] **Step 3: Register subchart in parent Chart.yaml**

Add to `charts/kube-llmops-stack/Chart.yaml` dependencies:

```yaml
  - name: finetune
    version: 0.1.0
    repository: "file://charts/finetune"
    condition: finetune.enabled
    tags:
      - ml-platform
```

- [ ] **Step 4: Rebuild Helm dependencies**

```bash
cd charts/kube-llmops-stack && rm -f charts/*.tgz Chart.lock && helm dependency update .
```

- [ ] **Step 5: Verify template renders without error**

```bash
helm template kube-llmops charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml 2>&1 | head -5
```

Expected: no errors (finetune.enabled=false, so no finetune resources rendered).

- [ ] **Step 6: Commit**

```bash
git add charts/kube-llmops-stack/charts/finetune/ charts/kube-llmops-stack/Chart.yaml
git commit -m "feat(finetune): scaffold subchart with values schema"
```

---

### Task 2: MLflow Deployment + Service + PDB

**Files:**
- Create: `charts/kube-llmops-stack/charts/finetune/templates/mlflow.yaml`
- Create: `charts/kube-llmops-stack/charts/finetune/templates/pdb.yaml`

- [ ] **Step 1: Create mlflow.yaml**

Create the MLflow Deployment + Service template. The Deployment uses `ghcr.io/mlflow/mlflow` image, connects to PostgreSQL (`mlflow` database) for metadata and MinIO (`s3://mlflow/`) for artifacts. Key env vars: `MLFLOW_S3_ENDPOINT_URL`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`. Service exposes port 5000. Wrap in `{{- if .Values.enabled }}` and `{{- if .Values.mlflow.enabled }}`.

Follow the exact label pattern from langfuse/deployment.yaml:
- `app.kubernetes.io/name: mlflow`
- `app.kubernetes.io/instance: {{ .Release.Name }}`
- `app.kubernetes.io/component: ml-platform`
- `app.kubernetes.io/part-of: kube-llmops`

- [ ] **Step 2: Create pdb.yaml**

Follow the exact pattern from `charts/langfuse/templates/pdb.yaml`:

```yaml
{{- if .Values.enabled }}
{{- if .Values.pdb.enabled }}
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: {{ .Release.Name }}-mlflow
  labels:
    app.kubernetes.io/name: mlflow
    app.kubernetes.io/instance: {{ .Release.Name }}
    app.kubernetes.io/part-of: kube-llmops
spec:
  maxUnavailable: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: mlflow
      app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
{{- end }}
```

- [ ] **Step 3: Rebuild and verify MLflow renders**

```bash
cd charts/kube-llmops-stack && rm -f charts/*.tgz Chart.lock && helm dependency update .
helm template kube-llmops charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  --set finetune.enabled=true --set finetune.mlflow.enabled=true 2>&1 | grep "name: kube-llmops-mlflow"
```

Expected: see `kube-llmops-mlflow` Deployment and Service.

- [ ] **Step 4: Commit**

```bash
git add charts/kube-llmops-stack/charts/finetune/templates/
git commit -m "feat(finetune): MLflow Deployment + Service + PDB"
```

---

### Task 3: RBAC for workflow steps

**Files:**
- Create: `charts/kube-llmops-stack/charts/finetune/templates/rbac.yaml`

- [ ] **Step 1: Create rbac.yaml**

Create ServiceAccount + ClusterRole + ClusterRoleBinding. The workflow steps need to:
- Create/delete Deployments (for temp vLLM eval instance and canary deploy)
- Patch ConfigMaps (for LiteLLM canary routing)
- Create Jobs (for promote/rollback)
- Read Pods (for readiness checks)

```yaml
{{- if .Values.enabled }}
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .Release.Name }}-finetune
  labels:
    app.kubernetes.io/name: finetune
    app.kubernetes.io/instance: {{ .Release.Name }}
    app.kubernetes.io/part-of: kube-llmops
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: {{ .Release.Name }}-finetune
  labels:
    app.kubernetes.io/name: finetune
    app.kubernetes.io/instance: {{ .Release.Name }}
    app.kubernetes.io/part-of: kube-llmops
rules:
  - apiGroups: [""]
    resources: ["pods", "configmaps", "services"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["apps"]
    resources: ["deployments"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["batch"]
    resources: ["jobs"]
    verbs: ["get", "list", "watch", "create"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: {{ .Release.Name }}-finetune
  labels:
    app.kubernetes.io/name: finetune
    app.kubernetes.io/instance: {{ .Release.Name }}
    app.kubernetes.io/part-of: kube-llmops
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: {{ .Release.Name }}-finetune
subjects:
  - kind: ServiceAccount
    name: {{ .Release.Name }}-finetune
    namespace: {{ .Release.Namespace }}
{{- end }}
```

- [ ] **Step 2: Rebuild and verify**

```bash
cd charts/kube-llmops-stack && rm -f charts/*.tgz Chart.lock && helm dependency update .
helm template kube-llmops charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  --set finetune.enabled=true 2>&1 | grep "kind: ServiceAccount" | head -5
```

- [ ] **Step 3: Commit**

```bash
git add charts/kube-llmops-stack/charts/finetune/templates/rbac.yaml
git commit -m "feat(finetune): RBAC for workflow steps"
```

---

### Task 4: LLaMA-Factory training ConfigMap

**Files:**
- Create: `charts/kube-llmops-stack/charts/finetune/templates/configmap-train.yaml`

- [ ] **Step 1: Create ConfigMap template**

Generate LLaMA-Factory `train_config.yaml` from Helm values. The ConfigMap contains a YAML file that `llamafactory-cli train` reads. Key fields: `model_name_or_path`, `dataset`, `finetuning_type` (lora/full), `lora_rank`, `lora_alpha`, `num_train_epochs`, `per_device_train_batch_size`, `learning_rate`, `output_dir`, `report_to` (mlflow).

Template the config from `.Values.finetune.*` fields. Use `{{- if eq .Values.method "qlora" }}` to add quantization fields (`quantization_bit: 4`).

- [ ] **Step 2: Verify ConfigMap renders correctly**

```bash
helm template kube-llmops charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  --set finetune.enabled=true \
  --set finetune.baseModel=Qwen/Qwen2.5-0.5B-Instruct \
  --set finetune.method=lora 2>&1 | grep -A30 "train_config.yaml"
```

Expected: valid YAML with `finetuning_type: lora`, `lora_rank: 8`, etc.

- [ ] **Step 3: Commit**

```bash
git add charts/kube-llmops-stack/charts/finetune/templates/configmap-train.yaml
git commit -m "feat(finetune): LLaMA-Factory training ConfigMap"
```

---

### Task 5: Argo WorkflowTemplate — 6-step DAG

**Files:**
- Create: `charts/kube-llmops-stack/charts/finetune/templates/workflow.yaml`

This is the largest task. The WorkflowTemplate defines a DAG with 6 steps. Each step is an inline template with its container spec, env vars, volume mounts, and script.

- [ ] **Step 1: Create workflow.yaml header**

Create the WorkflowTemplate resource header with:
- `{{- if .Values.enabled }}` guard
- Argo CRD check: `{{- if not (lookup ...) }}{{- fail ... }}{{- end }}`
- `serviceAccountName: {{ .Release.Name }}-finetune`
- `volumeClaimTemplates`: workspace PVC (100Gi)
- DAG definition referencing 6 templates

- [ ] **Step 2: Add prepare-data template**

Image: `kube-llmops/model-loader:latest`. Script that branches on `dataSource.type`:
- `minio`: use `mc cp` to download from MinIO path to `/workspace/data/`
- `huggingface`: `pip install datasets && python -c "from datasets import load_dataset; ..."`
- `pvc`: data already at mount point, just validate format
Also downloads the base model using the shared model-loader logic.

- [ ] **Step 3: Add finetune template**

Image: `llamafactory/llamafactory:latest`. GPU required (`resources.limits.nvidia.com/gpu`). Mounts ConfigMap as `/workspace/train_config.yaml`. Env: `MLFLOW_TRACKING_URI`, `MLFLOW_EXPERIMENT_NAME`, `HF_HUB_ENABLE_HF_TRANSFER=1`. Command: `llamafactory-cli train /workspace/train_config.yaml`.

- [ ] **Step 4: Add merge-upload template**

Image: `kube-llmops/model-loader:latest`. Script:
1. If method=lora/qlora: merge adapter with base model (`llamafactory-cli export`)
2. Upload merged weights to `s3://models/<outputName>/` via MinIO client
3. Register in MLflow Model Registry: `mlflow.register_model()`, state=Staging

- [ ] **Step 5: Add evaluate template**

Image: `python:3.13-slim`. Script:
1. `pip install ragas mlflow` (or use pre-built image)
2. Create temp vLLM Deployment via `kubectl apply` pointing to new model
3. Wait for readiness
4. Run Ragas evaluation against temp instance
5. Log metrics to MLflow and Pushgateway
6. Delete temp vLLM Deployment
7. Write results to `/workspace/eval/results.json`

- [ ] **Step 6: Add quality-gate template**

Image: `python:3.13-slim`. Script:
1. Read `/workspace/eval/results.json`
2. Compare each metric against `evaluation.thresholds`
3. If all pass: mark MLflow version stage as ready, exit 0
4. If any fail: mark MLflow version as Archived, send webhook notification, exit 1

- [ ] **Step 7: Add deploy template**

Image: `bitnami/kubectl:latest`. Script:
1. If `deploy.auto=false`: send webhook with metrics + approve command, exit 0
2. If `deploy.auto=true`:
   a. Create vLLM Deployment for fine-tuned model (`vllm-<outputName>`)
   b. Wait for readiness
   c. Patch LiteLLM ConfigMap with canary weight routing
   d. Restart LiteLLM pod
   e. Tag MLflow: `deployment_stage: canary`

- [ ] **Step 8: Rebuild and verify full DAG renders**

```bash
cd charts/kube-llmops-stack && rm -f charts/*.tgz Chart.lock && helm dependency update .
helm template kube-llmops charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  --set finetune.enabled=true \
  --set finetune.baseModel=Qwen/Qwen2.5-0.5B-Instruct 2>&1 | grep "kind: WorkflowTemplate" 
```

Expected: one WorkflowTemplate with 6 DAG tasks.

- [ ] **Step 9: Commit**

```bash
git add charts/kube-llmops-stack/charts/finetune/templates/workflow.yaml
git commit -m "feat(finetune): Argo WorkflowTemplate — 6-step DAG"
```

---

### Task 6: CronWorkflow template

**Files:**
- Create: `charts/kube-llmops-stack/charts/finetune/templates/cronworkflow.yaml`

- [ ] **Step 1: Create cronworkflow.yaml**

Only renders when `finetune.schedule` is non-empty. Creates an Argo CronWorkflow that references the WorkflowTemplate from Task 5. Uses `.Values.schedule` as the cron expression.

```yaml
{{- if and .Values.enabled .Values.schedule }}
apiVersion: argoproj.io/v1alpha1
kind: CronWorkflow
metadata:
  name: {{ .Release.Name }}-finetune-cron
  ...
spec:
  schedule: {{ .Values.schedule | quote }}
  workflowSpec:
    workflowTemplateRef:
      name: {{ .Release.Name }}-finetune
{{- end }}
```

- [ ] **Step 2: Verify with and without schedule**

```bash
# Without schedule — no CronWorkflow
helm template ... --set finetune.enabled=true 2>&1 | grep -c "CronWorkflow"
# Expected: 0

# With schedule — CronWorkflow rendered
helm template ... --set finetune.enabled=true --set finetune.schedule="0 2 * * 0" 2>&1 | grep -c "CronWorkflow"
# Expected: 1
```

- [ ] **Step 3: Commit**

```bash
git add charts/kube-llmops-stack/charts/finetune/templates/cronworkflow.yaml
git commit -m "feat(finetune): CronWorkflow for scheduled training"
```

---

### Task 7: Update values-single-node.yaml + PostgreSQL + MinIO

**Files:**
- Modify: `charts/kube-llmops-stack/values-single-node.yaml`

- [ ] **Step 1: Add finetune section to values**

Add the `finetune:` section with defaults (`enabled: false`) after the keycloak section. Add the `mlflow:` section inside it.

- [ ] **Step 2: Add mlflow database to PostgreSQL extraDatabases**

```yaml
    extraDatabases:
      - name: langfuse
        password: langfuse-default-pw
      - name: dify
        password: dify-default-pw
      - name: dify_plugin
        password: dify-default-pw
      - name: mlflow                   # NEW
        password: mlflow-default-pw    # NEW
```

- [ ] **Step 3: Add mlflow + datasets buckets to MinIO**

```yaml
    defaultBuckets:
      - langfuse
      - models
      - mlflow       # NEW
      - datasets     # NEW
```

- [ ] **Step 4: Verify Helm template renders without errors**

```bash
cd charts/kube-llmops-stack && rm -f charts/*.tgz Chart.lock && helm dependency update .
helm template kube-llmops charts/kube-llmops-stack -f values-single-node.yaml 2>&1 | head -5
```

- [ ] **Step 5: Commit**

```bash
git add charts/kube-llmops-stack/values-single-node.yaml
git commit -m "feat(finetune): add finetune + mlflow config to values"
```

---

### Task 8: MLflow NodePort + NOTES.txt update

**Files:**
- Modify: `charts/kube-llmops-stack/templates/nodeport-services.yaml`
- Modify: `charts/kube-llmops-stack/templates/NOTES.txt`

- [ ] **Step 1: Add MLflow NodePort service**

Add to `nodeport-services.yaml` (inside the `{{- if ... nodePort.enabled }}` block):

```yaml
---
apiVersion: v1
kind: Service
metadata:
  name: {{ .Release.Name }}-mlflow-np
  labels:
    app.kubernetes.io/name: mlflow-nodeport
    app.kubernetes.io/instance: {{ .Release.Name }}
    app.kubernetes.io/part-of: kube-llmops
spec:
  type: NodePort
  selector:
    app.kubernetes.io/name: mlflow
    app.kubernetes.io/instance: {{ .Release.Name }}
  ports:
    - name: http
      port: 5000
      targetPort: 5000
      nodePort: {{ $np.mlflow | default 30505 }}
```

Wrap in `{{- if (index .Values "finetune" | default dict).enabled }}` so it only renders when finetune is enabled.

- [ ] **Step 2: Update NOTES.txt**

Add MLflow URL to the NodePort access section:

```
{{- if (index .Values "finetune" | default dict).enabled }}
   MLflow:      http://$NODE_IP:{{ $np.mlflow | default 30505 }}
{{- end }}
```

- [ ] **Step 3: Commit**

```bash
git add charts/kube-llmops-stack/templates/
git commit -m "feat(finetune): MLflow NodePort :30505 + NOTES.txt"
```

---

### Task 9: Finetune Overview Grafana dashboard

**Files:**
- Create: `charts/kube-llmops-stack/charts/observability/dashboards/finetune-overview.json`

- [ ] **Step 1: Create dashboard JSON**

Create `finetune-overview.json` following the exact panel schema from the working `rag-quality.json` dashboard (use `datasource.uid: "prometheus"`, `editorMode: "code"`, proper `fieldConfig.defaults.custom` for timeseries). 4 panels:

1. **Training Loss** (timeseries): query MLflow metrics via Prometheus (or use stat panel with latest MLflow run data)
2. **Fine-tune Job Status** (stat): `kube_job_status_succeeded{job_name=~".*finetune.*"}`, `kube_job_status_failed`
3. **Model Versions** (table): from kube-state-metrics, listing fine-tune related deployments
4. **Canary Traffic Split** (bargauge): LiteLLM routing weights if available

UID: `finetune-overview`, title: `Fine-tuning Pipeline Overview`.

- [ ] **Step 2: Rebuild and verify dashboard loads**

```bash
cd charts/kube-llmops-stack && rm -f charts/*.tgz Chart.lock && helm dependency update .
```

- [ ] **Step 3: Commit**

```bash
git add charts/kube-llmops-stack/charts/observability/dashboards/finetune-overview.json
git commit -m "feat(finetune): Grafana dashboard — Fine-tuning Pipeline Overview"
```

---

### Task 10: Deploy to cluster and validate

**Files:** None (validation only)

- [ ] **Step 1: Install Argo Workflows operator**

```bash
kubectl create ns argo 2>/dev/null || true
kubectl apply -n argo -f https://github.com/argoproj/argo-workflows/releases/download/v3.6.5/install.yaml
kubectl wait -n argo --for=condition=available deployment/workflow-controller --timeout=120s
```

Note: If images can't be pulled (Docker Hub blocked), use rancher mirrors or skip this step.

- [ ] **Step 2: Build model-loader image (if not done)**

```bash
docker build -t kube-llmops/model-loader:latest images/model-loader/
```

- [ ] **Step 3: Deploy with finetune enabled**

```bash
NODE_IP=$(kubectl get node -o jsonpath='{.items[0].status.addresses[0].address}')
cd charts/kube-llmops-stack && rm -f charts/*.tgz Chart.lock && helm dependency update .
helm upgrade kube-llmops charts/kube-llmops-stack \
  -f values-single-node.yaml \
  --set global.nodePort.enabled=true \
  --set global.nodePort.host=$NODE_IP \
  --set finetune.enabled=true \
  --set finetune.baseModel=Qwen/Qwen2.5-0.5B-Instruct \
  --set finetune.outputName=qwen-ft-test \
  --timeout 5m --no-hooks
```

- [ ] **Step 4: Verify MLflow is running**

```bash
kubectl get pods -l app.kubernetes.io/name=mlflow
# Expected: kube-llmops-mlflow Running

curl -s http://$NODE_IP:30505/api/2.0/mlflow/experiments/search | python3 -m json.tool
# Expected: {"experiments": [...]}
```

- [ ] **Step 5: Verify WorkflowTemplate exists**

```bash
kubectl get workflowtemplates
# Expected: kube-llmops-finetune
```

- [ ] **Step 6: Verify Grafana dashboard loaded**

```bash
kubectl exec deploy/kube-llmops-litellm -- python3 -c "
import urllib.request, json, base64
creds = base64.b64encode(b'admin:admin123!').decode()
req = urllib.request.Request('http://kube-llmops-grafana:3000/api/dashboards/uid/finetune-overview',
    headers={'Authorization': f'Basic {creds}'})
data = json.loads(urllib.request.urlopen(req).read())
print(f'Dashboard: {data[\"dashboard\"][\"title\"]} — {len(data[\"dashboard\"][\"panels\"])} panels')
"
```

- [ ] **Step 7: Commit final state**

```bash
git add -A
git commit -m "feat(finetune): v0.4.0 fine-tuning pipeline — MLflow + Argo Workflows + LLaMA-Factory"
git push origin main
```

---

### Task 11: Update documentation

**Files:**
- Modify: `AGENTS.md`
- Modify: `README.md`
- Modify: `README.zh-CN.md`
- Modify: `CHANGELOG.md`
- Modify: `CHANGELOG.zh-CN.md`
- Modify: `.devin/skills/kube-llmops/SKILL.md`

- [ ] **Step 1: Update AGENTS.md**

Add finetune section to Architecture diagram, Key Features, Grafana Dashboards (11), File Layout. Add Argo Workflows prerequisite to Key Commands.

- [ ] **Step 2: Update README / README.zh-CN**

Update feature list (add "Fine-tuning Pipeline"), dashboard count (10→11), roadmap (v0.4.0 partial).

- [ ] **Step 3: Update CHANGELOG / CHANGELOG.zh-CN**

Add v0.4.0-alpha section with fine-tuning pipeline features.

- [ ] **Step 4: Update Devin skill**

Add `finetune` subcommand to `/kube-llmops` skill, add MLflow dashboard to dashboard list (11 total), update install command with Argo prerequisite.

- [ ] **Step 5: Commit**

```bash
git add AGENTS.md README.md README.zh-CN.md CHANGELOG.md CHANGELOG.zh-CN.md .devin/skills/
git commit -m "docs: update all docs for v0.4.0 fine-tuning pipeline"
git push origin main
```
