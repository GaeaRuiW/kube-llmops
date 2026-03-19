# Contributing to kube-llmops

Thanks for your interest in contributing! This guide will help you get started.

## Development Setup

### Prerequisites

- [Helm 3.x](https://helm.sh/docs/intro/install/)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [kind](https://kind.sigs.k8s.io/) (for local testing)
- [yamllint](https://github.com/adrienverber/yamllint) (`pip install yamllint`)
- [chart-testing](https://github.com/helm/chart-testing) (optional but recommended)
- Python 3.11+ (for model-resolver development)

### Quick Start

```bash
git clone https://github.com/wangr30/kube-llmops.git
cd kube-llmops

# Verify setup
make lint
make test
```

## Making Changes

### Helm Charts

All Helm charts live under `charts/kube-llmops-stack/`. Sub-charts for individual components are in `charts/kube-llmops-stack/charts/`.

```bash
# Lint your changes
make lint

# Render templates to check output
make template

# Test with specific values profile
helm template test charts/kube-llmops-stack/ -f charts/kube-llmops-stack/values-minimal.yaml
```

### values.yaml Contract

The top-level keys in `values.yaml` are the user-facing API. Treat them as stable:

- **Adding** new keys is fine (non-breaking)
- **Renaming/removing** keys is a breaking change -- requires deprecation notice in the previous release
- Document every key with a comment

### Docker Images

Images are in `images/`. Each has its own Dockerfile.

```bash
# Build all images
make build

# Build one image
docker build -t kube-llmops/model-resolver:dev images/model-resolver/
```

### Grafana Dashboards

Dashboard JSON files go in `dashboards/`. When editing:

- Use a descriptive `title`
- Set datasource to `${DS_PROMETHEUS}` (variable, not hardcoded)
- Test by importing into a running Grafana instance

## Pull Request Process

1. Fork the repo and create a branch from `main`
2. Make your changes
3. Ensure `make lint` and `make test` pass
4. Write a clear commit message (we follow [Conventional Commits](https://www.conventionalcommits.org/))
5. Open a PR against `main`

### Commit Message Format

```
feat(vllm): add support for tensor parallelism configuration
fix(litellm): correct PostgreSQL connection string template
docs: update quick start guide for v0.2.0
chore(ci): add Python lint step to lint workflow
```

### PR Labels

| Label | Meaning |
|---|---|
| `bug` | Something isn't working |
| `feature` | New feature request or implementation |
| `docs` | Documentation only changes |
| `good-first-issue` | Good for newcomers |
| `help-wanted` | Extra attention needed |
| `breaking-change` | Changes that break backward compatibility |

## Project Structure

```
charts/kube-llmops-stack/   # Umbrella Helm chart (core deliverable)
  charts/                   # Sub-charts per component
  templates/                # Shared templates
  values.yaml               # Default values
  values-*.yaml             # Profile overrides
dashboards/                 # Grafana dashboard JSON files
alerting/                   # Prometheus alert rules
otel/                       # OpenTelemetry Collector configs
images/                     # Docker image source
  model-resolver/           # Engine auto-selection logic
  model-loader/             # Model weight downloader
scripts/                    # Automation scripts
docs/                       # Documentation
examples/                   # Usage examples
```

## Questions?

- Open a [GitHub Discussion](https://github.com/wangr30/kube-llmops/discussions) for questions
- Open an [Issue](https://github.com/wangr30/kube-llmops/issues) for bugs or feature requests
