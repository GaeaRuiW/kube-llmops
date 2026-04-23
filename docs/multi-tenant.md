# Multi-Tenant Configuration Guide

> **Note:** This module requires `global.modules.security.enabled: true`. The security module includes LLM-Guard, NetworkPolicy, and the tenant-overview dashboard.

## Overview

kube-llmops supports namespace-based multi-tenancy for isolating teams sharing a GPU cluster. Each team gets its own namespace with GPU quotas, resource limits, and network isolation.

## Prerequisites

Enable the security module:

```yaml
global:
  modules:
    security:
      enabled: true
```

## Configuration

Define teams in `security.multiTenant`:

```yaml
security:
  multiTenant:
    enabled: true
    teams:
      - name: team-nlp
        namespace: llm-team-nlp
        gpuQuota: 4
        memoryQuota: 128Gi
        cpuQuota: 32
        maxPods: 20
      - name: team-cv
        namespace: llm-team-cv
        gpuQuota: 2
        memoryQuota: 64Gi
        cpuQuota: 16
        maxPods: 10
```

## What Gets Created

For each team, kube-llmops creates:

### 1. Namespace
```yaml
kind: Namespace
metadata:
  name: llm-team-nlp
  labels:
    kube-llmops/tenant: team-nlp
```

### 2. ResourceQuota
```yaml
kind: ResourceQuota
metadata:
  name: team-nlp-quota
  namespace: llm-team-nlp
spec:
  hard:
    requests.nvidia.com/gpu: "4"
    requests.memory: 128Gi
    requests.cpu: "32"
    pods: "20"
```

### 3. NetworkPolicy
```yaml
kind: NetworkPolicy
metadata:
  name: team-nlp-isolation
  namespace: llm-team-nlp
spec:
  podSelector: {}
  policyTypes: [Ingress, Egress]
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              kube-llmops/tenant: team-nlp
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: default
  egress:
    - {}
```

Teams can communicate within their namespace and access shared services in the default namespace (LiteLLM, Grafana, etc.), but cannot access other team namespaces.

## Deploying Models Per Team

Teams deploy models in their own namespace using separate Helm releases:

```bash
# Team NLP deploys their model
helm install team-nlp-model charts/kube-llmops-stack \
  -n llm-team-nlp \
  --set global.models[0].name=team-nlp-model \
  --set global.models[0].source=org/model
```

Or use the kube-llmops operator with ModelDeployment CRDs in each namespace.

## Monitoring Per-Team Usage

The **Tenant Overview** Grafana dashboard (`tenant-overview`) shows:
- GPU utilization per team namespace
- Request counts per team
- Cost attribution per team
- Quota utilization (used vs limit)

## Quota Management

### Viewing Current Usage

```bash
kubectl get resourcequota -n llm-team-nlp
```

### Updating Quotas

Modify the team's quotas in values and upgrade:

```bash
helm upgrade kube-llmops charts/kube-llmops-stack \
  --set 'security.multiTenant.teams[0].gpuQuota=8' --no-hooks
```

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Pod pending in team namespace | Quota exceeded | Check `kubectl describe resourcequota -n <ns>` |
| Cannot access LiteLLM from team namespace | NetworkPolicy blocking | Verify default namespace label selector |
| Team can access other team's pods | NetworkPolicy not applied | Check `security.networkPolicy.enabled: true` |
| GPU quota shows 0 available | All GPUs allocated | Increase team's `gpuQuota` or reduce other teams |
