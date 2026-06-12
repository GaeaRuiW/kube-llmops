# ADR-0007: Use Kueue For Batch Admission

## Status

Proposed

## Context

kube-llmops includes online inference workloads and optional batch-style
workloads:

- fine-tuning workflows
- batch inference
- RAG evaluation jobs
- model preload jobs
- future offline evaluation and adapter promotion jobs

Argo Workflows is useful for DAG orchestration, but it does not provide
multi-tenant GPU admission, quota sharing, ResourceFlavor selection, pending
workload visibility, or fair sharing by itself.

The two main scheduling options considered are Kueue and Volcano.

Kueue provides Kubernetes-native admission and queueing. It focuses on when a
workload is admitted, how quota is consumed, and how resources are shared among
tenants. It works with the default Kubernetes scheduler and integrates with
Jobs, CronJobs, Ray, JobSet, Kubeflow jobs, Deployments, StatefulSets, and plain
Pods. For Argo Workflows, Kueue currently manages the pods created by a
Workflow rather than the Workflow object as a single atomic unit.

Kueue's public workload APIs are still beta-versioned (`v1beta1` and
`v1beta2` are both documented upstream). kube-llmops should therefore isolate
Kueue API-version choices in templates and tests, instead of spreading them
through operator or chart logic.

Volcano provides a richer batch scheduler for HPC and distributed training
clusters. It supports gang scheduling, DRF, binpack, queue resource management,
heterogeneous device scheduling, topology-aware scheduling, VolcanoJob, and
online/offline workload colocation.

## Decision

Use Kueue as the default queue/admission integration for kube-llmops batch
workloads.

Volcano should remain a possible future compatibility profile for users who
already operate Volcano or need large distributed training scheduling
semantics. It should not be the default dependency.

The base chart should not require either scheduler for simple single-node or
basic inference installs.

## Consequences

Positive:

- Keeps the default platform closer to standard Kubernetes scheduling.
- Adds GPU quota, ResourceFlavor, Kueue fair-sharing mechanisms, and pending
  workload visibility without making kube-llmops own a full batch scheduler.
- Fits the current Argo-based fine-tuning pipeline with minimal disruption.
- Gives the operator a clear API surface for queue selection.

Negative:

- Kueue does not treat a multi-step Argo Workflow as a single atomic workload.
  Earlier workflow steps can run before a later GPU step waits for quota.
- Some advanced distributed training features, such as gang scheduling and DRF,
  are stronger in Volcano.
- Kueue fair sharing is admission/preemption oriented; it is not equivalent to
  Volcano's scheduler-level DRF and gang scheduling semantics.
- Kueue beta APIs require compatibility tests around generated manifests during
  upgrades.

Neutral:

- A future `scheduling.backend: volcano` profile can be added if the project
  expands toward large-scale distributed training clusters.
- Implementation should start with optional Kueue templates and pod labels, not
  with installing Kueue by default.
- Kueue DRA, topology-aware scheduling, and MultiKueue are useful future
  production-profile options, but they should not enter the default single-node
  path before real GPU-cluster validation.

## References

- Roadmap research: [LLMOps Technology Roadmap Research](../llmops-technology-roadmap-2026.md)
- Kueue overview: <https://kueue.sigs.k8s.io/docs/overview/>
- Kueue Argo Workflow integration:
  <https://kueue.sigs.k8s.io/docs/tasks/run/external_workloads/argo_workflow/>
- Kueue fair sharing: <https://kueue.sigs.k8s.io/docs/concepts/fair_sharing/>
- Kueue Dynamic Resource Allocation:
  <https://kueue.sigs.k8s.io/docs/concepts/dynamic_resource_allocation/>
- Kueue v1beta API references:
  <https://kueue.sigs.k8s.io/docs/reference/kueue.v1beta1/> and
  <https://kueue.sigs.k8s.io/docs/reference/kueue.v1beta2/>
- Volcano introduction: <https://volcano.sh/docs/home/introduction/>
