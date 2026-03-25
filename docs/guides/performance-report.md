# Performance Report

> Generated on: YYYY-MM-DD | Profile: `values-<profile>.yaml`

## Test Environment

| Parameter       | Value                  |
|-----------------|------------------------|
| Kubernetes      | v1.XX (e.g., k3s)     |
| Node count      | 1                      |
| GPU             | NVIDIA XXXX (XX GB)    |
| vLLM model      | model-name             |
| LiteLLM version | X.Y.Z                 |
| Dify version    | X.Y.Z                 |

## Baseline Results

| Test           | Concurrency | Requests | p50 (s) | p95 (s) | p99 (s) | RPS  | Errors |
|----------------|-------------|----------|---------|---------|---------|------|--------|
| llm-inference  | 4           | 50       | —       | —       | —       | —    | 0      |
| embedding      | 4           | 50       | —       | —       | —       | —    | 0      |
| rag-e2e        | 2           | 20       | —       | —       | —       | —    | 0      |

## Sizing Recommendations

| Profile       | GPU  | CPU   | RAM   | Intended Use           |
|---------------|------|-------|-------|------------------------|
| single-node   | 1    | 8 c   | 32 Gi | Dev / demo             |
| multi-gpu     | 2-4  | 16 c  | 64 Gi | Team staging           |
| ha            | 4+   | 32 c  | 128 Gi| Production workloads   |

## Overhead Analysis

| Component     | Idle CPU (m) | Idle RAM (Mi) | Notes                        |
|---------------|-------------|---------------|------------------------------|
| LiteLLM       | —           | —             | Proxy overhead per request   |
| Keycloak      | —           | —             | Auth token validation        |
| PostgreSQL    | —           | —             | Shared by LiteLLM + Dify     |
| Prometheus    | —           | —             | Scrape interval: 15 s        |
| Grafana       | —           | —             | Dashboard rendering          |

Fill dashes with measured values after running `make bench`.
