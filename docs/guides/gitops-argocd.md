# GitOps with ArgoCD

## Prerequisites

Install ArgoCD into your cluster:

```bash
kubectl create namespace argocd
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
```

## Deploying the Application

Apply the manifest that points ArgoCD at this repository:

```bash
kubectl apply -f manifests/argocd/application.yaml
```

ArgoCD will clone the repo, run `helm template`, and apply the rendered
manifests automatically. The `syncPolicy.automated` block enables **prune**
(delete removed resources) and **selfHeal** (revert manual drift).

## Sync Waves

Sub-chart templates carry `argocd.argoproj.io/sync-wave` annotations so that
resources deploy in dependency order:

| Wave | Component     |
|------|---------------|
| 1    | PostgreSQL    |
| 2    | Keycloak      |
| 3    | LiteLLM       |
| 4    | vLLM          |
| 5    | Dify          |
| 6    | Observability |

## Multi-Environment Setup

Use the `ApplicationSet` manifest for staging + production:

```bash
kubectl apply -f manifests/argocd/applicationset.yaml
```

Each environment gets its own namespace and values file. Staging tracks `main`;
production pins to a Git tag for controlled rollouts.
