# GPU Monitoring — 4-Tier Dashboard Architecture

kube-llmops ships four progressively-drilled Grafana dashboards for GPU telemetry.
Each tier targets a specific role and zoom level, with click-through links that carry
variables (cluster / node / gpu / pod) down to the next tier.

## Tier Map

| Tier | Dashboard UID | Title | Target User | Typical Questions |
|------|---------------|-------|-------------|-------------------|
| **L1** | `gpu-cluster` | GPU · L1 Cluster Overview | CTO / Director | How many GPUs do we have? Any failing? Overall utilization? |
| **L2** | `gpu-node` | GPU · L2 Node View | SRE | Which node is hot / under-utilized / has XID errors? |
| **L3** | `gpu-gpu` | GPU · L3 Single GPU View | Hardware / Kernel engineer | Why is this specific card throttling? Temp / clock / power trace. |
| **L4** | `gpu-pod` | GPU · L4 Pod / Workload View | ML engineer | Is my training job using the GPU effectively? Tensor-Core active? |

```
 L1 Cluster  ─────┐
   │              │
   │ (click node) │
   ▼              │
 L2 Node  ─────┐  │
   │           │  │
   │ (click    │  │
   │   GPU)    │  │
   ▼           ▼  ▼
 L3 GPU  ──► L4 Pod
```

All four dashboards share the Prometheus datasource, the `cluster` variable
(populated from Prometheus `external_labels`), and consistent naming so that
variable-substitution in drill-down URLs (`${cluster}`, `${__data.fields.Node}`, etc.)
works out-of-the-box.

## Data Sources

### DCGM Exporter (`--kubernetes`)

DCGM Exporter runs as a DaemonSet and scrapes NVIDIA GPU metrics via NVML.
The chart enables `--kubernetes=true` by default (`observability.dcgmExporter.kubernetes.enabled`),
which makes the exporter mount the kubelet `pod-resources` socket and attach
`namespace`, `pod`, `container` labels to every metric. Without this, L4
wouldn't work (no way to correlate GPUs to workloads).

Requirements when `--kubernetes` is enabled:

- `hostPID: true` (required for kubelet socket access)
- `privileged: true` on the container security context
- Host mount `/var/lib/kubelet/pod-resources` (read-only)

To disable on hardened clusters (e.g. no-privileged-containers policy), set:

```yaml
observability:
  dcgmExporter:
    kubernetes:
      enabled: false
```

With `kubernetes.enabled: false`, the L4 dashboard will show no data — fall back
to joining `kube_pod_info` with `DCGM_FI_DEV_GPU_UTIL` by node, at the cost of
pod-level precision.

### Prometheus `external_labels`

Prometheus is configured with:

```yaml
global:
  external_labels:
    cluster: kube-llmops   # or whatever observability.prometheus.clusterName is set to
```

This attaches `cluster=<name>` to every scraped series, enabling multi-cluster
dashboards in the future without changing the PromQL. Today only one cluster is
scraped, but adding Thanos / Mimir / Prometheus federation is a drop-in change:
additional clusters will show up under the `$cluster` variable and get filtered
by the dashboards automatically.

Override via:

```yaml
observability:
  prometheus:
    clusterName: prod-us-west-2
```

## Metric Catalog

### GPU Hardware Metrics (from DCGM Exporter)

| Metric | Unit | Used In |
|--------|------|---------|
| `DCGM_FI_DEV_GPU_TEMP` | °C | L1 stats, L2 heatmap, L3 trace |
| `DCGM_FI_DEV_MEMORY_TEMP` | °C | L3 memory temp trace |
| `DCGM_FI_DEV_GPU_UTIL` | % | All tiers |
| `DCGM_FI_DEV_MEM_COPY_UTIL` | % | L3 memory util trace |
| `DCGM_FI_DEV_FB_USED` / `FB_FREE` | MiB | All tiers (VRAM) |
| `DCGM_FI_DEV_POWER_USAGE` | W | All tiers |
| `DCGM_FI_DEV_POWER_MGMT_LIMIT` | W | L3 (power-cap line) |
| `DCGM_FI_DEV_SM_CLOCK` / `MEM_CLOCK` | MHz | L3 frequency |
| `DCGM_FI_DEV_XID_ERRORS` | count | L1, L2, L3, L4 (fault tables) |
| `DCGM_FI_PROF_SM_ACTIVE` | ratio | L3 / L4 (effective SM utilization) |
| `DCGM_FI_PROF_PIPE_TENSOR_ACTIVE` | ratio | L4 (Tensor-Core utilization — key training health signal) |

Labels on every DCGM series: `cluster`, `Hostname` (node), `gpu` (index), `UUID`,
`modelName` (e.g. `"NVIDIA GeForce RTX 3090"`), and (with `--kubernetes`)
`namespace`, `pod`, `container`.

### Inference Metrics (from vLLM, used in L4)

| Metric | Unit | Purpose |
|--------|------|---------|
| `vllm:num_requests_waiting{model_name}` | count | Queue depth per model |
| `vllm:prompt_tokens_total` | tokens | Prompt throughput |
| `vllm:generation_tokens_total` | tokens | Generation throughput |

## Variable Cascade

Each dashboard defines only the variables it needs. All variables are defined
with `label_values()` queries that are chained — selecting a `node` in L2
narrows the `gpu` options in L3 automatically.

| Tier | Variables | Query |
|------|-----------|-------|
| L1 | `cluster` | `label_values(DCGM_FI_DEV_GPU_TEMP, cluster)` |
| L2 | `cluster`, `node` (multi) | `label_values(DCGM_FI_DEV_GPU_TEMP{cluster=~"$cluster"}, Hostname)` |
| L3 | `cluster`, `node` (single), `gpu` (multi, by UUID) | `label_values(DCGM_FI_DEV_GPU_TEMP{cluster=~"$cluster",Hostname=~"$node"}, UUID)` |
| L4 | `cluster`, `namespace` (multi), `pod` (multi) | `label_values(DCGM_FI_DEV_GPU_UTIL{cluster=~"$cluster",namespace=~"$namespace"}, pod)` |

## Drill-Down Links

Links are defined on table columns via `fieldConfig.overrides[].properties.links`.
Grafana substitutes `${__data.fields.<Column>}` at click-time.

Example (L1 → L2, on the "Node" column):

```json
{
  "matcher": {"id": "byName", "options": "Node"},
  "properties": [{
    "id": "links",
    "value": [{
      "title": "Drill down → L2 Node View",
      "url": "/d/gpu-node/gpu-node-view?var-cluster=${cluster}&var-node=${__data.fields.Node}&${__url_time_range}"
    }]
  }]
}
```

The `${__url_time_range}` token passes the current time range through, so the
drill-down opens at the same moment you're looking at.

## Header Links ("Back" Navigation)

Each dashboard's `links[]` at top-level contains a "← Previous Tier" link plus
a "dashboards by tag" dropdown so you can jump laterally:

```json
"links": [
  {"type": "link", "title": "← L1 Cluster", "url": "/d/gpu-cluster/gpu-l1-cluster-overview?var-cluster=${cluster}&${__url_time_range}"},
  {"type": "dashboards", "tags": ["kube-llmops", "gpu"], "title": "GPU Dashboards"}
]
```

## What's Not Covered Here

- **KV-cache / model-level memory pressure**: `vllm-overview` dashboard has dedicated panels.
- **SLO / TTFT / TPOT**: `slo-overview` dashboard.
- **Cost per token / infrastructure ROI**: `cost-usage` and `infra-roi` dashboards.

The 4-tier GPU dashboards focus on **hardware health + workload efficiency**;
service-level and quality signals live in their own dashboards linked from
the Grafana sidebar and the Headlamp `kube-llmops-portal` plugin.
