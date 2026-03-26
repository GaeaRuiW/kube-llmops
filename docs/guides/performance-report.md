# Performance Report Template

> **This is a template.** Fill in the values below after running load tests against your deployment.
>
> ```bash
> # Run load tests
> make bench
>
> # Or run individually
> uv run tests/load/llm-inference.py
> uv run tests/load/embedding.py
> uv run tests/load/rag-e2e.py
> ```

## Test Environment

| Parameter       | Value                             |
|-----------------|-----------------------------------|
| Date            | _YYYY-MM-DD_                      |
| Profile         | _e.g., values-single-node.yaml_   |
| Kubernetes      | _e.g., v1.31 (k3s)_              |
| Node count      | _e.g., 1_                         |
| GPU             | _e.g., NVIDIA RTX 4090 (24 GB)_  |
| vLLM model      | _e.g., Qwen2.5-0.5B-Instruct_    |
| LiteLLM version | _e.g., 1.63.2_                   |
| Dify version    | _e.g., 1.13.2_                   |

## Baseline Results

| Test           | Concurrency | Requests | p50 (s) | p95 (s) | p99 (s) | RPS  | Errors |
|----------------|-------------|----------|---------|---------|---------|------|--------|
| llm-inference  | 4           | 50       | —       | —       | —       | —    | 0      |
| embedding      | 4           | 50       | —       | —       | —       | —    | 0      |
| rag-e2e        | 2           | 20       | —       | —       | —       | —    | 0      |

## Sizing Recommendations

| Profile       | GPU  | CPU   | RAM    | Intended Use           |
|---------------|------|-------|--------|------------------------|
| single-node   | 1    | 8 c   | 32 Gi  | Dev / demo             |
| multi-gpu     | 2-4  | 16 c  | 64 Gi  | Team staging           |
| ha            | 4+   | 32 c  | 128 Gi | Production workloads   |

## Overhead Analysis

| Component     | Idle CPU (m) | Idle RAM (Mi) | Notes                        |
|---------------|-------------|---------------|------------------------------|
| LiteLLM       | —           | —             | Proxy overhead per request   |
| Keycloak      | —           | —             | Auth token validation        |
| PostgreSQL    | —           | —             | Shared by LiteLLM + Dify     |
| Prometheus    | —           | —             | Scrape interval: 15 s        |
| Grafana       | —           | —             | Dashboard rendering          |

> Replace dashes (—) with measured values after running `make bench`.
