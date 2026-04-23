# AGENTS.md — Project Knowledge for AI Assistants

## Project Overview
kube-llmops is a Kubernetes-native LLMOps platform using Umbrella Helm Charts.
Deploy, manage, monitor, and optimize LLM infrastructure with a single `helm install`
(or via the LLMPlatform CR managed by the included Kubernetes Operator).

## Key Commands

```bash
# Deploy (single-node with GPU + NodePort access) — direct Helm
NODE_IP=$(kubectl get node -o jsonpath='{.items[0].status.addresses[0].address}')
helm install kube-llmops charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  --set global.nodePort.enabled=true \
  --set global.nodePort.host=$NODE_IP \
  --set global.hfToken=$HF_TOKEN

# Upgrade
helm upgrade kube-llmops charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  --set global.hfToken=$HF_TOKEN --no-hooks

# Alternative: Operator-managed deployment via LLMPlatform CR
# 1) Build + push operator image (one-time; see operator/build.sh)
bash operator/build.sh
docker tag kube-llmops/operator:latest localhost:5000/kube-llmops/operator:latest
docker push localhost:5000/kube-llmops/operator:latest
# 2) Install operator chart (embeds the umbrella chart)
helm install kube-llmops-operator operator/charts/kube-llmops-operator
# 3) Apply an LLMPlatform CR (see operator/config/samples/llmplatform_full.yaml)
kubectl apply -f operator/config/samples/llmplatform_full.yaml

# IMPORTANT: After changing any subchart template, rebuild archives:
cd charts/kube-llmops-stack && rm -f charts/*.tgz Chart.lock && helm dependency update .

# Build model-loader image (first time only)
docker build -t kube-llmops/model-loader:latest images/model-loader/

# Build Headlamp plugin image (first time only)
docker build -t kube-llmops/headlamp-plugin:latest plugins/kube-llmops-portal/

# Run Playwright E2E tests
uv run tests/e2e/test_dify_model_provider.py
uv run tests/e2e/test_dify_rag_e2e.py

# Trigger Ragas evaluation
kubectl create job ragas-manual --from=cronjob/kube-llmops-ragas-eval

# Check smoke test
kubectl logs -l app.kubernetes.io/name=rag-smoke-test --tail=30

# Check quality gate
kubectl logs job/kube-llmops-quality-gate

# Run all Helm template tests (Phase 5 + finetune + module switches)
python -m pytest tests/helm/ -v

# Run finetune E2E tests (requires GPU cluster + Argo Workflows)
uv run tests/e2e/test_finetune_e2e.py

# Run Phase 5 Helm template tests
python -m pytest tests/helm/test_phase5_templates.py -v
```

## Architecture

```
┌─ Ingress (Traefik) / NodePort ───────────────────┐
│  *.llmops.local → litellm/grafana/langfuse/dify  │
│  or NODE_IP:304xx                                 │
├──────────────────────────────────────────────────┤
│ LiteLLM (Gateway:4000) → vLLM (LLM:8000)        │
│                        → TEI (Embed:8080)         │
│                        → TEI (Rerank:8080)        │
│                        → llama.cpp (GGUF:8080)    │
│ Dify (RAG:5001/3000) → LiteLLM → pgvector        │
│ Langfuse (Trace:3000) ← LiteLLM callbacks         │
│ Prometheus:9090 + Pushgateway:9091 → Grafana:3000 │
│ Node Exporter:9100 + Kube State Metrics:8080      │
│ LLM-Guard (Security:8000)                         │
│ Keycloak (SSO:8080 / HTTPS:8443)                  │
│ Argo Workflows + LLaMA-Factory (Fine-tune)          │
│ MLflow (Experiment Tracking:5000)                    │
│ Headlamp (K8s UI:4466) + Portal Plugin             │
│ MinIO (S3:9000) + PostgreSQL:5432 + Harbor:80      │
│ kube-llmops-operator (LLMPlatform/ModelDeployment/ │
│                       FineTuneRun CRDs)             │
└──────────────────────────────────────────────────┘
```

## Key Features (v1.0.0)

### kubectl-llmops CLI
Kubectl plugin for imperative shortcuts — complement to the declarative Operator:
- Binary: `kubectl-llmops` (invoke as `kubectl llmops <command>` when on PATH)
- Source: `operator/cmd/kubectl-llmops/main.go` (impl: `operator/internal/cli/cmd/`)
- Build: `cd operator && make build-cli` (produces `bin/kubectl-llmops`) or `make install-cli` (to `$GOPATH/bin`)
- Global flags: `-n/--namespace`, `-o table|json|yaml|wide`, `--kubeconfig`, `--context`
- Top-level commands (model lifecycle):
  - `deploy <source>` — create ModelDeployment from HF source (engine auto-detected)
  - `list` / `status <name>` / `scale <name> --replicas N` / `delete <name>`
  - `canary <name> --target <new-source> --weight <percent>` / `promote` / `rollback`
- Developer UX:
  - `logs <name> [-f]` / `test <name> --prompt "..."` / `endpoint <name>`
  - `port-forward --service=gateway|grafana|langfuse|dify|minio`
  - `dashboard` (opens Grafana)
- Subcommand groups:
  - `finetune {create,list,status,logs,delete}` — drives FineTuneRun CR
  - `platform {init,status,update}` — drives LLMPlatform CR (incl. module toggles)
  - `rag {list-kb,create-kb,upload,delete-kb,query,eval}` — Dify API operations
- `migrate <helm-release>` — one-way conversion: existing Helm release → LLMPlatform + ModelDeployment CRs
- Integration:
  - CR operations (deploy/scale/canary/finetune/platform) go through K8s API → operator reconciles
  - RAG commands call Dify Console API directly (auto-discovers NodePort/ClusterIP)
  - `test` calls LiteLLM gateway directly
  - `logs` for finetune jobs queries Argo Workflow pod logs (read-only)

### Kubernetes Operator (LLMPlatform CR)
Declarative platform management via CRDs — alternative to direct `helm install`:
- **LLMPlatform** CR: full platform spec (gateway, observability, models, modules, nodePort)
- **ModelDeployment** CR: per-model deployment (advanced; vLLM/TEI/llamacpp)
- **FineTuneRun** CR: fine-tuning pipeline runs
- Operator chart: `operator/charts/kube-llmops-operator/` (embeds the umbrella chart)
- Image: `localhost:5000/kube-llmops/operator:latest` (push to your registry)
- SA uses `cluster-admin` (needed for Helm chart's wildcard RBAC, e.g. Headlamp)
- Built-in stuck-release recovery (pending-install/upgrade rollback)
- Generation-based reconcile loop prevention (skip if ObservedGeneration == Generation)
- Sample CR: `operator/config/samples/llmplatform_full.yaml`
- User guide: `operator/docs/user-guide/operator-guide-en.md` (+ zh)

### Module Switches
One-toggle enable/disable for feature groups via `global.modules`:
```yaml
global:
  modules:
    rag:
      enabled: true       # dify, milvus, lightrag, rag-eval + dashboards + alerts
    finetune:
      enabled: false      # finetune (LLaMA-Factory + Argo + MLflow), jupyterhub
    security:
      enabled: false      # LLM-Guard, NetworkPolicy + tenant dashboard
```
- Chart.yaml dual-path conditions: `dify.enabled,global.modules.rag.enabled` — explicit override wins
- Dashboards and Prometheus alert groups auto-included/excluded per module
- Defaults: all modules off; values-single-node.yaml enables `rag: true`

### Engine Auto-Detection
Models are defined in a single `global.models` list. Engine is auto-detected from source name:
- `*GGUF*`, `*gguf*`, `*GUFF*` (case-insensitive; `guff` typo-tolerant since v0.5.0) → llamacpp
- `*rerank*`, `bge-*`, `e5-*`, `*embedding*` → tei
- everything else → vllm
- Explicit `engine:` field overrides auto-detection

### Default Engine Images
- **vLLM**: `vllm/vllm-openai:gemma4-cu130` (custom build — needed for Gemma 4 architecture)
- **llama.cpp**: `ghcr.io/ggml-org/llama.cpp:server-cuda-b8672`
- **TEI**: `ghcr.io/huggingface/text-embeddings-inference:cpu-1.8` (CPU default; GPU tag `1.8` for CUDA)

Override per deployment via `vllm.image.tag` / `llamacpp.image.tag`.

### Unified Model Distribution
```
helm install → model-preload Job → HF download → MinIO cache
                                                    ↓
pod start   → model-loader init  → MinIO hit → local PVC (<1s)
```
- Pre-built `model-loader` image (no runtime pip install)
- hf-transfer multi-threaded downloads (3-5x faster)
- `global.hfToken` for gated models (Llama, Gemma, etc.)
- Supports `allowPatterns` for selective downloads (e.g. `"*q4_k_m*"` to fetch just one GGUF quant)

### Split GGUF Support (llama.cpp)
llama.cpp requires `{prefix}-NNNNN-of-NNNNN.gguf` naming for multi-shard GGUF:
- Model-loader downloads matching shards from HuggingFace
- Pod startup hook creates symlinks for consistent naming
- Shell wrapper dynamically resolves `--model` path to first shard
- Tested: Gemma-4-31B Q8_0 GGUF (9 splits, ~31GB) on NVIDIA GB10 (Blackwell ARM64)
- Deployment strategy: `Recreate` (prevents GPU deadlock during rolling updates)

### NodePort Access
```bash
--set global.nodePort.enabled=true --set global.nodePort.host=$NODE_IP
```
Ports: LiteLLM :30400, Grafana :30300, Langfuse :30301, Dify :30500,
       Keycloak :30808 (HTTP) / :30809 (HTTPS), Prometheus :30909,
       MinIO :30900/:30901, Headlamp :30302, MLflow :30505

SSO works automatically — OIDC URLs auto-computed from nodePort.host.

### Keycloak HTTPS + K8s OIDC (Headlamp SSO)
Full SSO chain: Headlamp → Keycloak OIDC → K8s API Server.
Requires HTTPS for Keycloak (K8s API Server mandates HTTPS OIDC issuer).

**Quick Setup (k3s + self-signed cert using k3s CA):**
```bash
# 1. Enable Keycloak TLS in values-single-node.yaml (already enabled by default)
#    keycloak.tls.enabled: true

# 2. Generate cert signed by k3s CA (reuses existing CA — no new CA to manage)
K3S_TLS=/var/lib/rancher/k3s/server/tls
NODE_IP=$(kubectl get node -o jsonpath='{.items[0].status.addresses[0].address}')

# Create CSR config
cat > /tmp/keycloak.cnf << EOF
[req]
distinguished_name = req_dn
req_extensions = v3_req
prompt = no
[req_dn]
CN = keycloak
[v3_req]
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = IP:${NODE_IP},IP:127.0.0.1,DNS:kube-llmops-keycloak,DNS:kube-llmops-keycloak.default.svc.cluster.local
EOF

# Sign with k3s CA
openssl genrsa -out /tmp/kc.key 2048
openssl req -new -key /tmp/kc.key -out /tmp/kc.csr -config /tmp/keycloak.cnf
sudo openssl x509 -req -in /tmp/kc.csr \
  -CA $K3S_TLS/server-ca.crt -CAkey $K3S_TLS/server-ca.key -CAcreateserial \
  -out /tmp/kc.crt -days 3650 -extensions v3_req -extfile /tmp/keycloak.cnf

# Create K8s secret
kubectl create secret generic keycloak-tls-k3sca \
  --from-file=tls.crt=/tmp/kc.crt --from-file=tls.key=/tmp/kc.key \
  --from-file=ca.crt=$K3S_TLS/server-ca.crt --type=kubernetes.io/tls

# 3. Configure k3s API Server for OIDC
sudo tee /etc/rancher/k3s/config.yaml << EOF
kube-apiserver-arg:
  - "oidc-issuer-url=https://${NODE_IP}:30809/realms/kube-llmops"
  - "oidc-client-id=headlamp"
  - "oidc-username-claim=preferred_username"
  - "oidc-username-prefix=-"
  - "oidc-groups-claim=groups"
  - "oidc-ca-file=/var/lib/rancher/k3s/server/tls/server-ca.crt"
EOF
sudo systemctl restart k3s

# 4. Deploy with OIDC
helm upgrade kube-llmops charts/kube-llmops-stack \
  -f charts/kube-llmops-stack/values-single-node.yaml \
  --set global.nodePort.enabled=true --set global.nodePort.host=$NODE_IP \
  --set headlamp.headlamp.config.oidc.issuerURL=https://$NODE_IP:30809/realms/kube-llmops \
  --no-hooks
```

**User-provided cert (production):**
```yaml
keycloak:
  tls:
    enabled: true
    selfSigned: false
    existingSecret: "my-keycloak-tls"   # must have tls.crt, tls.key, ca.crt
```
Then configure k3s `--oidc-ca-file` with your CA.

**OIDC RBAC** — maps Keycloak users to K8s ClusterRoles:
```yaml
oidcRBAC:
  enabled: true
  bindings:
    - username: admin           # must match Keycloak preferred_username
      clusterRole: cluster-admin
    - username: dev@company.com
      clusterRole: edit
```

### Fine-tuning Pipeline (v0.4.0)
- Argo Workflows DAG: prepare-data → finetune → merge-upload → evaluate → quality-gate → deploy
- LLaMA-Factory: LoRA / QLoRA / Full fine-tuning for all model types
- MLflow: Experiment tracking + Model Registry (reuses PostgreSQL + MinIO)
- Data sources: MinIO (s3://), HuggingFace datasets, PVC
- Quality gate with configurable thresholds
- Canary deployment via LiteLLM weight routing
- Human approval via webhook notifications
- Prerequisite: Argo Workflows operator must be installed separately

### HPA Autoscaling (KEDA)
- Auto-creates HPA for vLLM and llama.cpp Deployments via KEDA ScaledObjects
- Trigger: Prometheus `pending_requests` metric per engine:
  - vLLM: `vllm:num_requests_waiting{model_name="<name>"}`
  - llama.cpp: `llamacpp_requests_processing{model="<name>"}`
- Models auto-detected from `global.models` (no separate list needed)
- Per-model overrides: `keda.models.<name>.{minReplicas,maxReplicas,threshold}`
- Prerequisite: KEDA operator must be installed separately
  ```bash
  helm repo add kedacore https://kedacore.github.io/charts
  helm install keda kedacore/keda -n keda-system --create-namespace
  ```

### Advanced Inference (v0.5.0)
- Latency-based routing (default strategy, configurable per deployment)
- Prefix caching for repeated system prompts
- Session affinity via Envoy sidecar (`litellm.sessionAffinity.enabled`)
- Multi-trigger KEDA autoscaling (queue + TTFT P95 + TPOT P95)
- Scale-to-zero with LiteLLM fallback for cold start
- Spot/preemptible GPU tolerations (AWS, GCP, Azure, Karpenter)
- MIG GPU device support for model co-location
- Canary deployment with weight-based traffic splitting
- llm-d disaggregated serving (experimental, prefill/decode split)
- Multi-accelerator support (nvidia, amd, gaudi)
- SLO alerts (TTFT/TPOT breach thresholds)
- Envoy AI Gateway with InferencePool + InferenceModel CRDs (IGW extension)

## Critical Gotchas

### Helm .tgz Cache
Helm uses `.tgz` archives in `charts/` over directory sources. After editing any subchart template:
```bash
rm -f charts/kube-llmops-stack/charts/*.tgz charts/kube-llmops-stack/Chart.lock
helm dependency update charts/kube-llmops-stack/
```

### Operator: Chart Re-Bake After Subchart Changes
The operator embeds the umbrella chart at build time (`_build_charts/` staging dir).
After editing any subchart, rebuild + push + restart the operator:
```bash
bash operator/build.sh   # calls helm dep update, then docker build
docker tag kube-llmops/operator:latest localhost:5000/kube-llmops/operator:latest
docker push localhost:5000/kube-llmops/operator:latest
kubectl rollout restart deployment/kube-llmops-operator-operator
# Trigger reconcile (spec didn't change, so bump observedGeneration to 0):
kubectl patch llmplatform <name> --type=merge --subresource=status \
  -p '{"status":{"observedGeneration":0}}'
```

### Umbrella Chart Values Override Subchart Defaults
If the umbrella `charts/kube-llmops-stack/values.yaml` sets a key like `vllm.image.tag`,
it takes precedence over the subchart default in `charts/vllm/values.yaml`. Always
check the umbrella values.yaml FIRST when debugging "why is this using the old image".

### Module Switch Condition Order
Chart.yaml uses dual-path conditions: `dify.enabled,global.modules.rag.enabled`.
Module-controlled subcharts (dify, milvus, lightrag, rag-eval, finetune, jupyterhub)
must NOT have `enabled: true` in their own values.yaml — otherwise the subchart default
takes precedence and the module switch never gets checked. The security subchart uses
nested enabled fields (e.g. `llmGuard.enabled`) which don't conflict.

### LiteLLM Embedding Config
- Use `huggingface/` prefix (NOT `openai/`) for TEI embedding models
- Set `drop_params: true` in `litellm_settings` (Dify sends `encoding_format` which huggingface provider rejects)
- `api_base` for huggingface provider: NO `/v1` suffix

### LiteLLM Model Name DNS Constraints
Model names become K8s Service names (`vllm-<name>`) and MUST be DNS-1035 compliant:
lowercase alphanumeric + `-`, start with letter, no dots. e.g. `qwen25-7b` ✅, `qwen2.5-7b` ❌.

### LiteLLM routingStrategy Empty String
If `litellm.routingStrategy` is passed as `""` (empty string, not unset) in the values,
it overrides the subchart's `latency-based-routing` default with an invalid value. The
operator's TranslateValues only sets it when non-empty; direct helm installs should
either omit the key or set it explicitly.

### Dify 1.x Architecture
- Uses HttpOnly cookies with `SameSite=Lax` — must use single-domain path-based routing
- Requires Plugin Daemon (`dify-plugin-daemon`) for model providers
- Plugin Daemon needs its own DB (`dify_plugin`) + Redis + PVC for persistence
- Frontend sends base64-encoded passwords in login requests

### LLM-Guard
- Needs ~6GB RAM for PromptInjection scanner model
- Config file at `/home/user/app/config/scanners.yml` — mount via ConfigMap
- Default config enables ALL scanners (Sentiment scanner has ZeroDivisionError bug)

### PostgreSQL
- Image: `pgvector/pgvector:pg16` or `pg17` (NOT plain `postgres:16-alpine` — needs pgvector)
- Init script auto-creates databases: litellm, langfuse, dify, dify_plugin, mlflow
- Auto-enables `vector` extension in each DB
- `charts/postgresql/` is now a standalone subchart (v0.5.0)

### Model Loader
- Pre-built image: `kube-llmops/model-loader:latest` (must `docker build` before first deploy)
- Flow: check MinIO → fallback HuggingFace → upload back to MinIO
- Uses hf-transfer for multi-threaded downloads
- `HF_HUB_ENABLE_HF_TRANSFER=1` set automatically
- `allowPatterns` supports glob (e.g. `"*q4_k_m*"`) for selective downloads

### Gemma 4 Model Requirements
- Requires `vllm/vllm-openai:gemma4-cu130` image (architecture not in upstream vLLM)
- OR use llama.cpp with a GGUF quant (e.g. `nohurry/gemma-4-26B-A4B-it-heretic-GUFF` q4_k_m,
  ~16.87 GB fits RTX 3090's 24GB VRAM)
- With vLLM: no `--tool-call-parser gemma4` (removed); use default or hermes
- With vLLM: no `--disable-access-log-for-endpoints` (removed in v0.9+)

## Test Coverage

| Test | Tool | Count | What it validates |
|------|------|-------|-------------------|
| Model Provider | Playwright | 5/5 | Login + add LLM + embedding + verify |
| RAG E2E | Playwright | 9/9 | KB → upload → index → chat → verify answer |
| Smoke Test | K8s Job | 5/5 | Embedding + LLM + Langfuse + trace + reranker |
| Ragas Eval | K8s CronJob | 4 metrics | faithfulness, relevancy, precision, recall |
| Quality Gate | Helm Hook | pass/block | Pre-upgrade check on Ragas thresholds |
| LLM-Guard | Manual | 4/4 | Normal + direct injection + subtle + benign |
| Finetune Helm Templates | pytest | 35+ | ConfigMap, RBAC, MLflow, PDB, LoRA/QLoRA/Full, profiles, validation |
| Finetune E2E | Python+kubectl | ~26 | MLflow health, WorkflowTemplate, Argo run, Registry, QG, Grafana |
| Finetune Sample Data | CI | 1 | Alpaca-format validation (>=10 samples) |
| Phase 5 Templates | pytest | 39 | routing, KEDA multi-trigger, scale-to-zero, canary, llm-d, MIG, accelerator |
| Module Switches | pytest | 19 | RAG/finetune/security toggles, dashboard/alert conditionals, explicit overrides |
| Headlamp Templates | pytest | 23 | deployment, service, NodePort, plugin, RBAC, Keycloak OIDC integration |

## Grafana Dashboards (14)

| UID | Title |
|-----|-------|
| `gpu-cluster` | GPU · L1 Cluster Overview *(v1.0+)* |
| `gpu-node` | GPU · L2 Node View *(v1.0+)* |
| `gpu-gpu` | GPU · L3 Single GPU View *(v1.0+)* |
| `gpu-pod` | GPU · L4 Pod / Workload View *(v1.0+)* |
| `vllm-overview` | vLLM Model Serving |
| `litellm-gateway` | LiteLLM AI Gateway |
| `gpu-overview` | GPU & Infrastructure (flat, legacy — superseded by L1-L4 above) |
| `rag-quality` | RAG Quality (Ragas) |
| `cost-usage` | Cost & Usage |
| `slo-overview` | SLO Overview |
| `infra-roi` | Infrastructure ROI |
| `tenant-overview` | Tenant Overview |
| `milvus-overview` | Milvus Vector DB |
| `system-overview` | System CPU/Memory/Disk/Network |
| `finetune-overview` | Fine-tuning Pipeline |

The four GPU dashboards (`gpu-cluster` → `gpu-node` → `gpu-gpu` → `gpu-pod`)
form a drill-down hierarchy — click a row in the Node/GPU/Pod table to
navigate to the next tier with variables pre-populated. See
[docs/gpu-monitoring.md](docs/gpu-monitoring.md) for the architecture.

## File Layout

```
charts/kube-llmops-stack/
  values.yaml                 # Umbrella defaults (overrides subchart defaults!)
  values-single-node.yaml     # Main config for single GPU node
  values-production.yaml      # Production HA config (external DB, 2+ replicas)
  templates/
    _helpers.tpl              # resolveEngine + resolveModelType + modelLoader helpers
    nodeport-services.yaml    # NodePort toggle
    model-preload-job.yaml    # Helm hook: batch HF→MinIO
    secret-hf-token.yaml      # global.hfToken Secret
  charts/                     # 20 subcharts
    vllm/                     # Model serving (PagedAttention)
    llamacpp/                 # GGUF model serving (supports split GGUF)
    tei/                      # Embedding + reranking
    litellm/                  # AI Gateway
    langfuse/                 # Tracing
    dify/                     # RAG platform + setup Job
    observability/            # Prometheus + Grafana + Pushgateway + node-exporter + kube-state-metrics
    rag-eval/                 # Smoke test + Ragas CronJob + Quality gate
    security/                 # LLM-Guard + NetworkPolicy
    keycloak/                 # SSO (HTTP + HTTPS)
    logging/                  # Loki + Fluent Bit
    finetune/                 # LLaMA-Factory + Argo Workflows + MLflow
    jupyterhub/               # Interactive notebooks (finetune module)
    fluid/                    # MinIO
    harbor/                   # Private container + model registry (v0.5.0)
    postgresql/               # Standalone PostgreSQL (v0.5.0)
    milvus/                   # Vector DB (rag module)
    lightrag/                 # Lightweight RAG (rag module)
    headlamp/                 # Headlamp K8s UI (wraps upstream chart)
    keda/                     # KEDA autoscaling (multi-trigger)
operator/                     # Kubernetes Operator (Go, controller-runtime)
  api/v1alpha1/               # CRD types: LLMPlatform, ModelDeployment, FineTuneRun
  internal/
    controller/               # Reconcilers
    helmbridge/               # Helm SDK wrapper + values translation
    builder/                  # Deployment/Service builders (ModelDeployment)
  charts/kube-llmops-operator/  # Helm chart to deploy the operator itself
  config/samples/             # Example CRs (llmplatform_full.yaml)
  docs/                       # Operator architecture + user guides (en + zh)
  build.sh                    # Build script: helm dep update → docker build
images/
  model-loader/               # Pre-built model downloader (minio + huggingface_hub + hf-transfer)
  model-resolver/             # Engine auto-detection logic (optional runtime fallback)
plugins/
  kube-llmops-portal/         # Headlamp plugin (Service Links + Grafana Monitoring)
tests/e2e/                    # Playwright E2E tests
tests/e2e/test_finetune_e2e.py  # Finetune pipeline E2E (MLflow + Argo + QG)
tests/helm/                   # Helm template unit tests (pytest)
tests/load/                   # Load testing scripts
examples/
  curl/                       # API curl examples
  python/                     # Python SDK examples
  eval/                       # Evaluation datasets
  finetune/                   # Sample training data + example values
docs/
  getting-started.md          # Getting started guide (en + zh)
  routing.md                  # Routing strategies + prefix caching
  large-model-deployment.md   # Multi-GPU, quantization, MIG
  speculative-decoding.md     # Draft model configuration
  kserve-integration.md       # KServe coexistence guide
  disaggregated-serving.md    # llm-d architecture + configuration
  model-updates.md            # Canary deployment flow
  multi-tenant.md             # Multi-tenant RBAC/quota guide
  presidio-pii.md             # Presidio PII integration
  adr/                        # Architecture Decision Records
  guides/                     # SLO, performance, GitOps, upgrade guides
```

### Headlamp Dashboard
- CNCF Kubernetes web UI with custom `kube-llmops-portal` plugin (replaces the legacy custom dashboard in v0.5.0)
- NodePort 30302, Keycloak OIDC integration
- Plugin pages: Service Links (card grid with NodePort URLs) + Monitoring (Grafana iframe embeds)
- Build plugin image: `docker build -t kube-llmops/headlamp-plugin:latest plugins/kube-llmops-portal/`
- Grafana configured with `allow_embedding=true` + anonymous Viewer access for iframes

### Default Models (values-single-node.yaml)
- **LLM**: `nohurry/gemma-4-26B-A4B-it-heretic-GUFF` (llama.cpp, q4_k_m, ~16.87GB, 160K ctx)
- **Embedding**: `BAAI/bge-small-en-v1.5` (TEI, CPU, 384 dim)
- **Reranker**: `BAAI/bge-reranker-base` (TEI, CPU)

The LLM source name uses `GUFF` (typo in the upstream HF repo). Auto-detection handles
this transparently since v0.5.0 (both `gguf` and `guff` patterns resolve to llamacpp),
so the explicit `engine: llamacpp` is optional. Use `allowPatterns: "*q4_k_m*"` to
selectively download only the q4_k_m shards (saves ~50GB of other quants).
