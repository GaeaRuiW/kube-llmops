# Azure AKS - kube-llmops Terraform Module

Deploy a production-ready AKS cluster with GPU support for running the kube-llmops stack.

## Architecture

- **Resource Group**: Dedicated Azure resource group
- **Virtual Network**: Custom VNet with AKS subnet, Azure CNI networking
- **AKS Cluster**: Managed Kubernetes 1.29 with system-assigned identity
- **System Node Pool**: `Standard_D4s_v3` instances for control plane workloads
- **GPU Node Pool**: `Standard_NC6s_v3` (V100 GPU) with autoscaling (0-4) and GPU taints
- **Monitoring**: Azure Log Analytics workspace integration
- **Add-ons**: NVIDIA GPU Operator, KEDA autoscaler, kube-llmops-stack Helm chart

## Prerequisites

1. [Terraform](https://developer.hashicorp.com/terraform/install) >= 1.5
2. [Azure CLI](https://learn.microsoft.com/en-us/cli/azure/install-azure-cli) (`az`) logged in
3. An Azure subscription with GPU quota (request `Standard_NC6s_v3` or your chosen GPU VM size)
4. Register required providers:
   ```bash
   az provider register --namespace Microsoft.ContainerService
   az provider register --namespace Microsoft.Compute
   ```

## Quick Start

```bash
# Initialize Terraform
terraform init

# Review the plan
terraform plan

# Apply (creates the cluster and installs kube-llmops)
terraform apply

# Configure kubectl
$(terraform output -raw kubeconfig_command)

# Verify
kubectl get nodes
kubectl get pods -n kube-llmops
```

## Customization

### Override variables

```bash
terraform apply \
  -var="cluster_name=my-llmops" \
  -var="location=westeurope" \
  -var="gpu_vm_size=Standard_NC4as_T4_v3" \
  -var="gpu_max_count=8"
```

### Use a custom values file

```bash
terraform apply -var="kube_llmops_values_file=./my-values.yaml"
```

## Teardown

```bash
terraform destroy
```
