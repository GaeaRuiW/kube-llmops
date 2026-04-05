# kube-llmops Terraform Modules

Infrastructure-as-Code modules for deploying kube-llmops on major cloud providers. Each module provisions a managed Kubernetes cluster with GPU-enabled node pools and installs the kube-llmops Helm stack.

## Module Overview

| Module | Cloud | Cluster | GPU Default | Directory |
|--------|-------|---------|-------------|-----------|
| [aws-eks](./aws-eks/) | AWS | EKS | g5.xlarge (A10G) | `terraform/aws-eks/` |
| [gcp-gke](./gcp-gke/) | GCP | GKE Standard | n1-standard-4 + T4 | `terraform/gcp-gke/` |
| [azure-aks](./azure-aks/) | Azure | AKS | Standard_NC6s_v3 (V100) | `terraform/azure-aks/` |

## What Each Module Creates

All three modules follow the same architecture pattern:

1. **Networking**: VPC/VNet with subnets and NAT for private node communication
2. **Managed Kubernetes Cluster**: EKS / GKE / AKS with the latest stable Kubernetes
3. **System Node Pool**: CPU-only nodes for control plane workloads (monitoring, ingress, etc.)
4. **GPU Node Pool**: GPU-accelerated nodes with autoscaling (0-4) and taints to prevent non-GPU workloads
5. **NVIDIA GPU Operator**: Manages GPU drivers and device plugins via Helm
6. **KEDA**: Event-driven autoscaling for inference workloads (optional)
7. **kube-llmops-stack**: The full platform Helm chart with sensible cloud-specific defaults

## Prerequisites

### Common

- [Terraform](https://developer.hashicorp.com/terraform/install) >= 1.5
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [Helm](https://helm.sh/docs/intro/install/) >= 3.0 (for manual chart management)

### Cloud-Specific

| Cloud | CLI Required | GPU Quota Needed |
|-------|-------------|-----------------|
| AWS | [AWS CLI v2](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html) | `g5.xlarge` in target region |
| GCP | [gcloud CLI](https://cloud.google.com/sdk/docs/install) | `NVIDIA_T4_GPUS` in target region |
| Azure | [Azure CLI](https://learn.microsoft.com/en-us/cli/azure/install-azure-cli) | `Standard_NC6s_v3` in target region |

> **Important**: GPU instances require quota approval on all clouds. Request quota increases _before_ running `terraform apply`.

## Quick Start

### AWS EKS

```bash
cd terraform/aws-eks

terraform init
terraform plan
terraform apply

# Configure kubectl
aws eks update-kubeconfig --region us-west-2 --name kube-llmops
```

### GCP GKE

```bash
cd terraform/gcp-gke

# Enable required APIs
gcloud services enable container.googleapis.com compute.googleapis.com

terraform init
terraform plan -var="project_id=YOUR_PROJECT_ID"
terraform apply -var="project_id=YOUR_PROJECT_ID"

# Configure kubectl
gcloud container clusters get-credentials kube-llmops --region us-central1
```

### Azure AKS

```bash
cd terraform/azure-aks

# Register providers
az provider register --namespace Microsoft.ContainerService

terraform init
terraform plan
terraform apply

# Configure kubectl
az aks get-credentials --resource-group kube-llmops-rg --name kube-llmops
```

## Customization

### Custom Helm Values

All modules accept a `kube_llmops_values_file` variable pointing to your custom values file:

```bash
terraform apply -var="kube_llmops_values_file=./my-values.yaml"
```

Example values file for a production deployment:

```yaml
vllm:
  enabled: true
  replicas: 2
  resources:
    limits:
      nvidia.com/gpu: 1
    requests:
      cpu: "4"
      memory: "16Gi"
  model: "meta-llama/Llama-3.1-8B-Instruct"

observability:
  enabled: true

litellm:
  enabled: true

langfuse:
  enabled: true

keda:
  enabled: true
```

### Node Sizes

Each module exposes variables for customizing instance types:

```bash
# AWS - Use larger GPU instances
terraform apply -var="gpu_instance_type=g5.2xlarge" -var="gpu_max_size=8"

# GCP - Use A100 GPUs
terraform apply -var="project_id=my-project" \
  -var="gpu_type=nvidia-tesla-a100" \
  -var="gpu_machine_type=a2-highgpu-1g"

# Azure - Use T4 GPUs
terraform apply -var="gpu_vm_size=Standard_NC4as_T4_v3"
```

### GPU Autoscaling

All modules support scaling GPU nodes to zero when idle:

```hcl
gpu_min_size  = 0   # Scale to zero when no GPU pods are pending
gpu_max_size  = 8   # Maximum GPU nodes
```

The GPU node pools use `nvidia.com/gpu=present:NoSchedule` taints, ensuring only workloads with the appropriate toleration are scheduled on GPU nodes.

## Architecture Diagram

```
                    ┌─────────────────────────────────┐
                    │     Managed Kubernetes Cluster    │
                    │       (EKS / GKE / AKS)         │
                    │                                   │
                    │  ┌─────────────┐ ┌─────────────┐ │
                    │  │ System Pool │ │  GPU Pool    │ │
                    │  │ (CPU only)  │ │ (autoscale)  │ │
                    │  │             │ │              │ │
                    │  │ - Monitoring│ │ - vLLM       │ │
                    │  │ - LiteLLM   │ │ - TGI        │ │
                    │  │ - Langfuse  │ │ - Embeddings │ │
                    │  │ - KEDA      │ │              │ │
                    │  └─────────────┘ └─────────────┘ │
                    │                                   │
                    │  GPU Taint: nvidia.com/gpu=present│
                    └─────────────────────────────────┘
```

## State Management

For production use, configure a remote backend for Terraform state:

```hcl
# AWS S3 backend
terraform {
  backend "s3" {
    bucket = "my-terraform-state"
    key    = "kube-llmops/terraform.tfstate"
    region = "us-west-2"
  }
}

# GCP GCS backend
terraform {
  backend "gcs" {
    bucket = "my-terraform-state"
    prefix = "kube-llmops"
  }
}

# Azure Storage backend
terraform {
  backend "azurerm" {
    resource_group_name  = "terraform-state"
    storage_account_name = "tfstate"
    container_name       = "tfstate"
    key                  = "kube-llmops.tfstate"
  }
}
```

## Teardown

Each module cleans up all resources on destroy:

```bash
cd terraform/<cloud-module>
terraform destroy
```

> **Warning**: This will delete the cluster and all workloads. Ensure you have backed up any persistent data.
