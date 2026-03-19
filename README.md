# kube-llmops

**Kubernetes-native LLMOps Platform** -- Deploy, manage, monitor, and optimize your entire LLM infrastructure with one command.

> [!NOTE]
> This project is under active development. Star and watch for updates!

## What is kube-llmops?

`kube-llmops` is an opinionated, batteries-included Helm chart that deploys a complete LLM operations stack on Kubernetes:

- **Model Serving** -- vLLM, llama.cpp, or TEI, auto-selected based on model format
- **AI Gateway** -- LiteLLM for unified OpenAI-compatible API, API key management, cost tracking
- **Observability** -- OpenTelemetry + Prometheus + Grafana dashboards + Langfuse for LLM tracing
- **Infrastructure** -- GPU scheduling, distributed model caching, autoscaling

```bash
helm install kube-llmops kube-llmops/kube-llmops-stack -f values-minimal.yaml
```

## Use Cases

- **"I want to deploy Qwen3.5-122B and let 5 teams share it with token budget limits"**
- **"I want to see which team burned the most GPU hours this month"**
- **"I want a GGUF model on llama.cpp and a full-precision model on vLLM behind the same API"**
- **"I want every LLM request traced with full prompt, tokens, cost, and latency"**

## Architecture

<!-- TODO: Replace with polished SVG diagram -->

```
Client -> LiteLLM (AI Gateway) -> Model Resolver -> vLLM / llama.cpp / TEI
                                                         |
          Langfuse (Traces) <- OTel Collector <- Prometheus + DCGM (Metrics)
                                                         |
                                                    Grafana (6 Dashboards)
```

See [ARCHITECTURE.md](ARCHITECTURE.md) for the full technical design.

## Quick Start

### Prerequisites

- Kubernetes cluster (1.28+) with GPU node, or `kind` for CPU-only demo
- Helm 3.x
- kubectl

### Install

```bash
# Add Helm repo (available after v0.1.0 release)
helm repo add kube-llmops https://GaeaRuiW.github.io/kube-llmops
helm repo update

# Install with minimal profile (1 GPU, 1 model, basic monitoring)
helm install kube-llmops kube-llmops/kube-llmops-stack -f values-minimal.yaml

# Or: CPU-only demo (no GPU required)
helm install kube-llmops kube-llmops/kube-llmops-stack -f values-ci.yaml
```

### Chat with your model

```bash
kubectl port-forward svc/kube-llmops-litellm 4000:4000 &

curl http://localhost:4000/v1/chat/completions \
  -H "Authorization: Bearer sk-kube-llmops-dev" \
  -H "Content-Type: application/json" \
  -d '{"model":"qwen2-5-0-5b","messages":[{"role":"user","content":"Hello!"}]}'
```

### Access the UIs

```bash
kubectl port-forward svc/kube-llmops-litellm 4000:4000 &    # AI Gateway
kubectl port-forward svc/kube-llmops-grafana 3000:3000 &     # Metrics
kubectl port-forward svc/kube-llmops-langfuse 3001:3000 &    # LLM Tracing
```

| Service | URL | Default Credentials |
|---|---|---|
| **LiteLLM** (AI Gateway + Admin UI) | `http://localhost:4000/ui` | any username / `sk-kube-llmops-dev` |
| **Grafana** (Dashboards) | `http://localhost:3000` | `admin` / `admin` |
| **Langfuse** (LLM Tracing) | `http://localhost:3001` | `admin@kube-llmops.local` / `admin123!` |

> [!WARNING]
> These are development defaults. For production, override via `--set`:
> ```bash
> helm install kube-llmops kube-llmops/kube-llmops-stack \
>   --set litellm.masterKey=sk-your-secret-key \
>   --set observability.grafana.adminPassword=your-grafana-pw \
>   --set langfuse.init.userPassword=your-langfuse-pw \
>   --set langfuse.externalUrl=https://langfuse.your-domain.com
> ```

## Features

| Feature | kube-llmops | Raw vLLM | KAITO | KServe |
|---|---|---|---|---|
| Engine auto-selection (GPTQ->vLLM, GGUF->llama.cpp) | Yes | N/A | No | No |
| AI Gateway (key mgmt, cost tracking, rate limit) | Yes | No | No | No |
| LLM tracing (prompt, tokens, cost per request) | Yes | No | No | No |
| Pre-built Grafana dashboards (6) | Yes | No | No | No |
| GPU monitoring (DCGM) | Yes | DIY | No | No |
| One-click full stack | Yes | N/A | No | No |
| Cloud-agnostic | Yes | Yes | Azure only | Yes |

## Deployment Profiles

| Profile | GPU | Models | Monitoring | Tracing | Use Case |
|---|---|---|---|---|---|
| `values-ci.yaml` | None | Tiny (CPU) | Basic | Off | CI / Demo |
| `values-minimal.yaml` | 1x | 1 small | Prometheus + Grafana | Off | Development |
| `values-standard.yaml` | 4-8x | 2-3 | Full OTel stack | Langfuse | Team |
| `values-production.yaml` | 16+x | N | Full + HA | Full | Enterprise |

## Documentation

- [Architecture](ARCHITECTURE.md) -- Full technical design and technology choices
- [Implementation Plan](PLAN.md) -- Milestones, CI/CD strategy, and backlog
- [Contributing](CONTRIBUTING.md) -- How to contribute

## Roadmap

- [x] **v0.1.0 (MVP)** -- Model serving + Gateway + Metrics + Tracing
- [ ] **v0.2.0** -- Logging + Autoscaling + Model cache + Security
- [ ] **v0.3.0** -- RAG + Vector DB + Inference Gateway (IGW)
- [ ] **v0.4.0** -- Fine-tuning + ML platform
- [ ] **v0.5.0** -- Disaggregated serving (llm-d)
- [ ] **v1.0.0** -- Operator + CLI + Dashboard

## License

[Apache License 2.0](LICENSE)

### License Notice

This project is Apache 2.0 licensed. However, some optional dependencies have different licenses:

| Component | License | Required? |
|---|---|---|
| Grafana | AGPL-3.0 | Optional (can bring your own) |
| Loki | AGPL-3.0 | Optional (can use OpenSearch) |
| All other components | Apache 2.0 / MIT / BSD | Yes |

If AGPL is a concern for your organization, Grafana and Loki can be disabled and replaced with your own visualization and log storage solutions.

## Contributing

Contributions welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Star History

If you find this project useful, please give it a star!
