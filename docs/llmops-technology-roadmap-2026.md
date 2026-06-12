# LLMOps Technology Roadmap Research (2026-06)

Status: Proposed

This document records the June 2026 technology research and recommended
direction for kube-llmops. It is intended to guide near-term implementation
work, not to serve as a full product specification.

## Executive Recommendation

kube-llmops should remain focused on Kubernetes-native LLM deployment and
inference operations. RAG and fine-tuning should stay in the project, but as
optional, infrastructure-oriented modules rather than primary product surfaces.

The main engineering priority should shift from "many components can be
installed" to "the production inference path is correct, observable, scalable,
and secure by default."

Recommended split:

- 70%: inference deployment, routing, autoscaling, model lifecycle, GPU
  operations, observability, security
- 20%: minimal stable RAG infrastructure and evaluation
- 10%: fine-tuning pipeline, focused on adapter-to-deployment handoff

## Repo-Local Baseline

The project is already directionally aligned with current LLMOps practice:

- Unified model definitions and capability-based engine selection exist in
  `charts/kube-llmops-stack/values.yaml`.
- LiteLLM is used as the top-level AI gateway in
  `charts/kube-llmops-stack/charts/litellm/templates/configmap.yaml`.
- vLLM, SGLang, llama.cpp, Chitu, and TEI are supported as serving engines.
- KEDA HTTP add-on based scale-to-zero is present in
  `charts/kube-llmops-stack/charts/keda/templates/scaledobject.yaml`.
- vLLM prefix caching is exposed through model values in
  `charts/kube-llmops-stack/charts/vllm/templates/deployment.yaml`.
- RAG and fine-tuning are optional modules.
- llm-d and Gateway API Inference Extension concepts are already documented in
  `docs/disaggregated-serving.md`.

The gap is not a lack of feature ideas. The gap is that some production-grade
capabilities are still direct service wiring, templates, or documentation
rather than stable, tested operational paths.

## External Trends

### 1. Inference Routing Is Moving Beyond Basic Load Balancing

The industry direction is a two-layer gateway:

```text
Client
  -> LiteLLM or another AI gateway
  -> Envoy / Gateway API Inference Extension
  -> model server endpoints
```

The first layer handles API keys, user/team policy, provider abstraction,
budgets, rate limits, and cost tracking. The second layer handles inference
scheduling: model-aware routing, prefix/KV cache locality, LoRA adapter
availability, priority, and rollout.

Recommendation:

- Keep LiteLLM as the default first-layer AI gateway.
- Add Envoy AI Gateway / Gateway API Inference Extension as an optional second
  layer.
- Do not replace LiteLLM with Envoy AI Gateway. Their responsibilities are
  different.

Implementation direction:

- Add optional templates for `InferencePool` and model-aware routes, targeting
  the current `inference.networking.k8s.io/v1` API when the installed gateway
  implementation supports it.
- Start with one vLLM model pool.
- Route `LiteLLM -> Envoy/Inference Gateway -> vLLM`.
- Keep alpha model/objective routing resources out of the production path until
  the API and gateway implementations mature.
- Later add LoRA adapter routing and prefix-cache-aware endpoint picking.

### 2. KV Cache And Prefix Cache Are Becoming First-Class Scheduling Inputs

vLLM and SGLang expose increasingly rich runtime capabilities: prefix caching,
structured outputs, LoRA serving, KV events/offload, disaggregated prefill and
decode, and advanced distributed serving. The operational value is realized
only when routing decisions can use those signals.

Recommendation:

- Keep the existing `prefixCaching` model flag.
- Treat it as incomplete until requests can be routed to endpoints with useful
  cache locality.
- Use Gateway API Inference Extension / llm-d style routing rather than trying
  to implement cache-aware routing inside Helm templates or LiteLLM config.

Implementation direction:

- Add prefix-cache-aware routing as a gateway-gated profile, using the Gateway
  API Inference Extension Endpoint Picker prefix-cache plugin where compatible.
- Treat vLLM KV events and external KV indexers as advanced integrations until
  they have been tested against the selected gateway path.
- Add dashboards for prefix cache hit rate and per-endpoint request placement.
- Keep direct LiteLLM-to-service routing as the simple default path until the
  inference gateway path is tested.

### 3. LoRA Adapter Lifecycle Is The Right Fine-Tuning Integration Point

Fine-tuning should not become a full training platform in this project. The
highest-value integration is the lifecycle after training:

```text
FineTuneRun
  -> adapter artifact
  -> registry/object store
  -> ModelDeployment.adapterRefs
  -> vLLM LoRA serving
  -> LiteLLM / inference gateway canary
```

Recommendation:

- Keep fine-tuning optional.
- Focus on making LoRA/QLoRA output deployable as a first-class adapter.
- Do not default to vLLM runtime LoRA loading from arbitrary user-provided
  paths in production; it has security implications.

Implementation direction:

- Add `adapterRefs` to the operator model deployment API.
- Store adapters in MinIO or another configured artifact backend.
- Prefer vLLM resolver / registry integration for trusted adapter backends.
  Render `--enable-lora` and static `--lora-modules` as the simple fallback.
- Keep runtime adapter loading disabled by default; enable
  `VLLM_ALLOW_RUNTIME_LORA_UPDATING` only for explicitly trusted resolver
  sources.
- Add canary promotion for adapter-backed model names.

### 4. Batch GPU Workloads Need Admission, Not Just Workflow Orchestration

Argo Workflows is a DAG orchestrator. It does not solve multi-tenant GPU quota,
fair sharing, ResourceFlavor selection, topology-aware scheduling, or admission
control by itself.

Recommendation:

- Add Kueue as the default queue/admission controller for fine-tuning,
  batch inference, RAG eval, and model preload workloads.
- Keep Argo for workflow execution.
- Use Kueue to decide when GPU-consuming pods are admitted.
- Treat Kueue API versions as part of the chart compatibility contract because
  the project currently exposes beta APIs.

See [Scheduler Decision: Kueue vs Volcano](#scheduler-decision-kueue-vs-volcano).

### 5. GenAI Observability Should Follow OpenTelemetry Semantics

The current direction of LLM observability is OpenTelemetry-compatible tracing
with GenAI semantic attributes, plus LLM-native analysis in systems such as
Langfuse or MLflow.

Recommendation:

- Keep Langfuse as the LLM trace backend.
- Keep OpenTelemetry Collector as the transport and fan-out layer.
- Align emitted spans and metrics with OpenTelemetry GenAI semantic
  conventions where possible, while treating those conventions as evolving.
- Propagate `user`, `session`, `tenant`, `model`, `adapter`, and `request_id`
  across LiteLLM, Envoy, model server, RAG, and evaluation spans.

Implementation direction:

- Add a tracing contract document for required span attributes.
- Add OTel Collector configuration for Langfuse OTLP HTTP ingestion.
- Pin or opt in to a known GenAI semantic convention version where the
  instrumentation supports `OTEL_SEMCONV_STABILITY_OPT_IN`.
- Add dashboards that correlate gateway, model-server, GPU, and quality
  metrics.

### 6. RAG Is Becoming Evaluation-Driven Infrastructure

The useful role for kube-llmops is not to implement RAG algorithms. The useful
role is to make RAG infrastructure deployable and measurable:

- embedding service
- reranking service
- vector database or database extension
- gateway routing
- tracing
- quality evaluation
- dashboards and gates

Recommendation:

- Keep Dify and LightRAG as optional application-level integrations.
- Do not build a custom RAG framework.
- Strengthen RAG smoke tests, Ragas eval, quality gates, and trace coverage.

### 7. Agent Tooling And MCP Are Important, But Not A Core Module Yet

MCP is becoming a standard way to connect agents to tools and external data.
However, adding an agent platform would significantly expand project scope.

Recommendation:

- Track MCP as a gateway/security integration topic.
- Do not add a full agent runtime module yet.
- If added later, start with documentation and optional Envoy AI Gateway MCP
  routing, not a custom agent framework.

## Scheduler Decision: Kueue vs Volcano

Recommendation: use Kueue as the default queue/admission integration.
Treat Volcano as an optional future compatibility profile for large training
clusters.

### Why Kueue By Default

Kueue matches kube-llmops' current needs:

- Kubernetes-native admission and queueing without replacing core scheduling
  responsibilities.
- ResourceFlavor, ClusterQueue, LocalQueue, quota, fair sharing, borrowing,
  preemption, and pending workload visibility.
- Good fit for Argo-created pods, Kubernetes Jobs, CronJobs, Ray, JobSet,
  Kubeflow jobs, Deployments, and StatefulSets.
- Fits a platform that wants to manage both serving and batch workloads while
  keeping the operational surface small.
- The Kueue APIs exposed today are beta APIs (`v1beta1` / `v1beta2`), so chart
  templates should isolate API-version assumptions and keep upgrade tests close
  to the generated manifests.

Tradeoff:

- Kueue does not understand an Argo Workflow as a single atomic unit. With the
  Argo pod integration, each pod is admitted as a Kueue workload. A multi-step
  workflow can run earlier steps and later wait for quota on a GPU step.
- Kueue has fair-sharing mechanisms, but they are admission/preemption oriented.
  If strict DRF or gang semantics across running distributed jobs become hard
  requirements, Volcano is the stronger fit.

### Why Not Volcano As The Main Path

Volcano is stronger when kube-llmops becomes a full HPC / distributed training
scheduler:

- gang scheduling
- DRF
- binpack
- VolcanoJob
- vGPU and MIG-oriented scheduling
- task topology and NUMA policies
- online/offline workload colocation
- richer scheduler plugins

Those are valuable, but making Volcano the default would push kube-llmops toward
"ship and operate a batch scheduler" rather than "deploy and operate LLM
inference infrastructure." It also increases the default operational footprint.

### Decision

- Default: Kueue.
- Optional later: Volcano profile for users who already run Volcano or need
  large distributed training semantics.
- Do not require either scheduler for basic single-node or simple inference
  installs.

Suggested values shape:

```yaml
scheduling:
  enabled: false
  backend: kueue
  kueue:
    install: false
    defaultLocalQueue: llmops-default
    resourceFlavors:
      - name: nvidia-gpu
        nodeLabels:
          accelerator: nvidia
    clusterQueues:
      - name: llmops-gpu
        cohort: llmops
        nominalQuota:
          cpu: "64"
          memory: 512Gi
          nvidia.com/gpu: "8"
  volcano:
    install: false
```

For MIG or Kubernetes Dynamic Resource Allocation scenarios, add separate
ResourceFlavors and examples only after they are tested on a real GPU cluster.
Do not make DRA part of the default single-node path.

## Recommended Roadmap

### P0: Secure And Stabilize Defaults

Do first.

- Remove remaining fixed default credentials from charts.
- Generate Kubernetes Secrets with `lookup` reuse where possible.
- Support `existingSecret` for production credentials.
- Avoid `latest` and `nightly` image tags as production defaults.
- Add secret scanning to CI.
- Keep Helm template tests for every security-sensitive chart change.

Rationale:

Public repositories and copy-paste installs make unsafe defaults expensive. The
project should be safe to inspect and safe to install in a development cluster.

### P1: Inference Gateway MVP

Do next.

- Add optional Envoy AI Gateway / Gateway API Inference Extension templates.
- Start with vLLM only.
- Render one `InferencePool` per model or model group, using the current
  `inference.networking.k8s.io/v1` API where the target gateway supports it.
- Route LiteLLM to the inference gateway rather than directly to the service
  when enabled.
- Add basic prefix-cache-aware Endpoint Picker configuration behind the
  inference gateway feature flag.
- Add Helm tests that verify direct and gateway routing modes.

Non-goals:

- Do not implement custom endpoint picking.
- Do not put alpha model/objective routing resources on the production path.
- Do not make llm-d the default.

### P1: Adapter Deployment Loop

Do in parallel with the inference gateway work if time allows.

- Add `adapterRefs` to model deployment values and operator APIs.
- Support LoRA adapter artifacts from MinIO or a configured object store.
- Prefer vLLM LoRA resolver / registry integration for trusted adapter stores
  when available; render static `--lora-modules` only as the simple fallback.
- Keep runtime adapter loading disabled by default. Enable
  `VLLM_ALLOW_RUNTIME_LORA_UPDATING` only for explicitly trusted resolver
  sources.
- Allow canary routing between base and adapter model names.
- Add quality gate integration before promotion.

Non-goals:

- Do not expose arbitrary runtime LoRA loading by default.
- Do not build a fine-tuning UI.

### P1: Kueue Queueing Module

Do before expanding fine-tuning or batch evaluation.

- Add optional Kueue resource templates:
  - `ResourceFlavor`
  - `ClusterQueue`
  - `LocalQueue`
  - `WorkloadPriorityClass`
- Add labels to fine-tuning workflow pods and eval jobs.
- Document Argo Workflow limitations.
- Start by labeling GPU-consuming Argo templates, not every workflow step.
- Add Grafana panels for pending workloads and admitted workloads.
- Track Kueue DRA, topology-aware scheduling, and MultiKueue as future
  production-profile options after real-cluster validation.

Non-goals:

- Do not install Kueue by default in the base chart.
- Do not add Volcano until Kueue is tested.

### P2: GenAI Observability Contract

- Define required OTel attributes for gateway, model server, RAG, and eval
  spans.
- Add Langfuse OTLP HTTP exporter configuration examples.
- Treat OpenTelemetry GenAI semantic conventions as evolving. Pin or opt in to
  the emitted convention version where instrumentation supports it.
- Add dashboards that correlate:
  - request rate
  - TTFT / TPOT
  - queue depth
  - GPU utilization
  - cache hit rate
  - RAG quality
  - cost / token usage, using Langfuse or custom attributes until a stable
    cross-tool cost convention exists

### P2: RAG Infrastructure Quality

- Keep Dify / LightRAG optional.
- Add RAG e2e as an explicit CI/e2e tier, not a basic Helm lint dependency.
- Improve Ragas eval dataset and thresholds.
- Emit Prometheus metrics for quality and regression gates.
- Trace `embed -> retrieve -> rerank -> generate` as separate observations.

### P3: Large-Model Profiles

- Add llm-d profile for disaggregated serving.
- Add NVIDIA Dynamo documentation/profile only after the gateway path is stable.
- Treat P/D disaggregation, KV offload, and wide expert parallelism as advanced
  profiles, not defaults.

## Things Not To Do Now

- Do not replace LiteLLM with Envoy AI Gateway.
- Do not write a custom RAG framework.
- Do not write a custom batch scheduler.
- Do not make Volcano the default scheduler.
- Do not make llm-d or Dynamo default paths before the basic inference gateway
  path is reliable.
- Do not expand fine-tuning into a full training platform before the LoRA
  adapter deployment loop is complete.

## Source Notes

Research date: 2026-06-12.

Official or upstream sources used:

- Gateway API Inference Extension:
  <https://gateway-api-inference-extension.sigs.k8s.io/>
- Gateway API Inference Extension v1 API reference:
  <https://gateway-api-inference-extension.sigs.k8s.io/reference/spec/>
- Gateway API Inference Extension prefix-cache-aware plugin:
  <https://gateway-api-inference-extension.sigs.k8s.io/guides/epp-configuration/prefix-aware/>
- Envoy AI Gateway:
  <https://aigateway.envoyproxy.io/>
- llm-d:
  <https://github.com/llm-d/llm-d>
- NVIDIA Dynamo:
  <https://docs.nvidia.com/dynamo/latest/>
- vLLM OpenAI-compatible server:
  <https://docs.vllm.ai/en/latest/serving/openai_compatible_server/>
- vLLM Automatic Prefix Caching:
  <https://docs.vllm.ai/en/latest/features/automatic_prefix_caching/>
- vLLM KV events:
  <https://docs.vllm.ai/en/latest/examples/features/kv_events/>
- vLLM LoRA adapters:
  <https://docs.vllm.ai/en/latest/features/lora/>
- vLLM Structured Outputs:
  <https://docs.vllm.ai/en/latest/features/structured_outputs/>
- SGLang:
  <https://docs.sglang.io/>
- Kueue overview:
  <https://kueue.sigs.k8s.io/docs/overview/>
- Kueue Argo Workflow integration:
  <https://kueue.sigs.k8s.io/docs/tasks/run/external_workloads/argo_workflow/>
- Kueue fair sharing:
  <https://kueue.sigs.k8s.io/docs/concepts/fair_sharing/>
- Kueue Dynamic Resource Allocation:
  <https://kueue.sigs.k8s.io/docs/concepts/dynamic_resource_allocation/>
- Kueue v1beta API references:
  <https://kueue.sigs.k8s.io/docs/reference/kueue.v1beta1/> and
  <https://kueue.sigs.k8s.io/docs/reference/kueue.v1beta2/>
- Volcano:
  <https://volcano.sh/docs/home/introduction/>
- OpenTelemetry GenAI semantic conventions:
  <https://opentelemetry.io/docs/specs/semconv/gen-ai/>
- Langfuse OpenTelemetry integration:
  <https://langfuse.com/integrations/native/opentelemetry>
- Ragas:
  <https://docs.ragas.io/en/stable/>
- MLflow GenAI:
  <https://mlflow.org/docs/latest/genai/>
- Model Context Protocol:
  <https://modelcontextprotocol.io/specification/2025-06-18>
