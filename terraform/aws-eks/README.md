# AWS EKS - kube-llmops Terraform Module

Deploy a production-ready EKS cluster with GPU support for running the kube-llmops stack.

## Architecture

- **VPC**: 2 AZs with public and private subnets, NAT gateway
- **EKS Cluster**: Managed Kubernetes 1.29 with IRSA enabled
- **System Node Group**: `t3.large` instances for control plane workloads
- **GPU Node Group**: `g5.xlarge` instances with autoscaling (0-4) and GPU taints
- **Storage**: GP3 EBS volumes via the EBS CSI driver
- **Add-ons**: NVIDIA GPU Operator, KEDA autoscaler, kube-llmops-stack Helm chart

## Prerequisites

1. [Terraform](https://developer.hashicorp.com/terraform/install) >= 1.5
2. [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html) configured with appropriate credentials
3. Sufficient GPU quota in your target region (request `g5.xlarge` or your chosen GPU instance type)

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

# Verify the cluster
kubectl get nodes
kubectl get pods -n kube-llmops
```

## Customization

### Override variables

```bash
terraform apply \
  -var="cluster_name=my-llmops" \
  -var="region=us-east-1" \
  -var="gpu_instance_type=g5.2xlarge" \
  -var="gpu_max_size=8"
```

### Use a custom values file for kube-llmops

```bash
terraform apply -var="kube_llmops_values_file=./my-values.yaml"
```

### Operator-Based Deployment (v0.5.0+)

Instead of letting Terraform run `helm install` for `kube-llmops-stack` directly, you
can install the Kubernetes Operator and manage the platform through `LLMPlatform` CRs:

```bash
bash operator/build.sh
docker tag kube-llmops/operator:latest <your-registry>/kube-llmops/operator:latest
docker push <your-registry>/kube-llmops/operator:latest

helm install kube-llmops-operator operator/charts/kube-llmops-operator \
  --set image.repository=<your-registry>/kube-llmops/operator
kubectl apply -f operator/config/samples/llmplatform_full.yaml
```

See [../../operator/README.md](../../operator/README.md) for details.

### Use a tfvars file

Create `terraform.tfvars`:

```hcl
cluster_name       = "my-llmops"
region             = "us-east-1"
gpu_instance_type  = "g5.2xlarge"
gpu_max_size       = 8
gpu_desired_size   = 2
enable_keda        = true
enable_gpu_operator = true
```

## Costs

Approximate monthly costs (us-west-2):

| Resource | Estimate |
|----------|----------|
| EKS control plane | ~$73 |
| 2x t3.large (system) | ~$120 |
| 1x g5.xlarge (GPU) | ~$726 |
| NAT Gateway | ~$32 |
| EBS volumes | Variable |

GPU nodes scale to zero when idle if `gpu_min_size = 0`.

## Teardown

```bash
terraform destroy
```
