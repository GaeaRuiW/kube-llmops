# GCP GKE - kube-llmops Terraform Module

Deploy a production-ready GKE cluster with GPU support for running the kube-llmops stack.

## Architecture

- **VPC Network**: Custom subnet with secondary ranges for pods/services, Cloud NAT
- **GKE Cluster**: Standard cluster with REGULAR release channel, Workload Identity
- **System Node Pool**: `e2-standard-4` instances for control plane workloads
- **GPU Node Pool**: `n1-standard-4` + `nvidia-tesla-t4` with autoscaling (0-4) and GPU taints
- **Add-ons**: NVIDIA GPU Operator, KEDA autoscaler, kube-llmops-stack Helm chart

## Prerequisites

1. [Terraform](https://developer.hashicorp.com/terraform/install) >= 1.5
2. [Google Cloud SDK](https://cloud.google.com/sdk/docs/install) (`gcloud`) configured
3. A GCP project with billing enabled
4. GPU quota in your target region (request `NVIDIA_T4_GPUS` quota increase)
5. Enable required APIs:
   ```bash
   gcloud services enable container.googleapis.com compute.googleapis.com
   ```

## Quick Start

```bash
# Initialize Terraform
terraform init

# Review the plan (project_id is required)
terraform plan -var="project_id=my-gcp-project"

# Apply
terraform apply -var="project_id=my-gcp-project"

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
  -var="project_id=my-gcp-project" \
  -var="cluster_name=my-llmops" \
  -var="region=europe-west4" \
  -var="gpu_type=nvidia-tesla-a100" \
  -var="gpu_machine_type=a2-highgpu-1g"
```

### Use a custom values file

```bash
terraform apply \
  -var="project_id=my-gcp-project" \
  -var="kube_llmops_values_file=./my-values.yaml"
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

## Teardown

```bash
terraform destroy -var="project_id=my-gcp-project"
```
