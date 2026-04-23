# kube-llmops Upgrade Guide

## Generic Upgrade Flow

1. **Backup databases** (if running production workloads)
   ```bash
   kubectl create job pg-backup-pre-upgrade --from=cronjob/kube-llmops-pg-backup
   ```

2. **Review CHANGELOG.md** for breaking changes

3. **Run upgrade with quality gate**
   ```bash
   helm upgrade kube-llmops charts/kube-llmops-stack -f values-single-node.yaml
   ```
   The quality gate pre-upgrade hook checks Ragas metrics before allowing upgrade.

4. **Verify**
   ```bash
   kubectl get pods -l app.kubernetes.io/part-of=kube-llmops --watch
   uv run tests/e2e/test_dify_rag_e2e.py
   ```

## Rollback

```bash
helm rollback kube-llmops <REVISION>
helm history kube-llmops  # to see revision numbers
```

## Version-Specific Notes

### v0.1 → v0.2 (PostgreSQL Split)
- New: `operator-pg` and `app-pg` replace single `litellm-pg`
- Migration: data migration script in `docs/guides/postgresql-migration.md`
- Breaking: `litellm.postgresql.*` values moved to `postgresql.operatorPg.*`

### v0.2 → v0.3 (Enterprise Features)
- New: Milvus, LightRAG, Presidio, multi-tenant namespaces
- No breaking changes (all new components default disabled)

### v0.3 → v0.4 (Module Switches + Fine-tuning)
- New: **Module switches** `global.modules.{rag,finetune,security}.enabled` — single
  toggle to include/exclude related subcharts, Grafana dashboards, and Prometheus
  alert groups as a group
- New: **Fine-tuning pipeline** — LLaMA-Factory + Argo Workflows + MLflow, covering
  data prep → train (LoRA/QLoRA/Full) → merge → evaluate → quality-gate → deploy
- New: MLflow experiment tracking + Model Registry, reusing PostgreSQL + MinIO
- Prerequisite: Argo Workflows operator must be installed separately before enabling
  `global.modules.finetune.enabled=true`
- Migration: no breaking changes — existing deployments keep working; to get RAG
  behavior under v0.3, explicitly set `global.modules.rag.enabled=true` (it is `true`
  in `values-single-node.yaml` and `false` in `values-minimal.yaml`)

### v0.4 → v0.5 (Headlamp + Operator + Advanced Inference)
- **Headlamp replaces the legacy custom dashboard** (NodePort `30302`). The
  `kube-llmops-portal` Headlamp plugin provides Service Links + embedded Grafana
  monitoring. Build the plugin image before upgrade:
  ```bash
  docker build -t kube-llmops/headlamp-plugin:latest plugins/kube-llmops-portal/
  ```
- New: **Kubernetes Operator** under `operator/` with `LLMPlatform`,
  `ModelDeployment`, and `FineTuneRun` CRDs — alternative to direct `helm install`
- Upgrade: **vLLM default image** → `vllm/vllm-openai:gemma4-cu130` (required for
  Gemma 4 architecture); override via `vllm.image.tag` if you need upstream
- New: **llama.cpp split GGUF support** — multi-shard GGUF models (e.g. Gemma-4-31B
  Q8_0) are downloaded + symlinked into `{prefix}-NNNNN-of-NNNNN.gguf` layout; default
  image `ghcr.io/ggml-org/llama.cpp:server-cuda-b8672`
- New: Advanced inference features — latency-based routing, prefix caching, session
  affinity (Envoy sidecar), multi-trigger KEDA autoscaling (queue + TTFT + TPOT),
  scale-to-zero, canary deploys, llm-d disaggregated serving, MIG GPU, multi-accelerator
- Migration: if you have a custom dashboard Deployment from v0.4 or earlier, remove it
  — Headlamp now serves the same purpose. If you previously pinned
  `vllm.image.tag: latest` for Gemma 4 workarounds, switch to the new default tag.
