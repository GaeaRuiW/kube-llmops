variable "project_id" {
  description = "GCP project ID (required)"
  type        = string
}

variable "region" {
  description = "GCP region for the cluster"
  type        = string
  default     = "us-central1"
}

variable "cluster_name" {
  description = "Name of the GKE cluster"
  type        = string
  default     = "kube-llmops"
}

variable "kubernetes_version" {
  description = "Kubernetes version prefix (uses REGULAR release channel if unset)"
  type        = string
  default     = ""
}

variable "system_machine_type" {
  description = "Machine type for system node pool"
  type        = string
  default     = "e2-standard-4"
}

variable "system_node_count" {
  description = "Number of system nodes"
  type        = number
  default     = 2
}

variable "gpu_machine_type" {
  description = "Machine type for GPU node pool"
  type        = string
  default     = "n1-standard-4"
}

variable "gpu_type" {
  description = "GPU accelerator type"
  type        = string
  default     = "nvidia-tesla-t4"
}

variable "gpu_count" {
  description = "Number of GPUs per node"
  type        = number
  default     = 1
}

variable "gpu_min_size" {
  description = "Minimum number of GPU nodes"
  type        = number
  default     = 0
}

variable "gpu_max_size" {
  description = "Maximum number of GPU nodes"
  type        = number
  default     = 4
}

variable "gpu_desired_size" {
  description = "Initial number of GPU nodes"
  type        = number
  default     = 1
}

variable "enable_keda" {
  description = "Whether to install KEDA for autoscaling"
  type        = bool
  default     = true
}

variable "enable_gpu_operator" {
  description = "Whether to install NVIDIA GPU Operator (GKE auto-installs drivers, but operator adds device plugin features)"
  type        = bool
  default     = true
}

variable "kube_llmops_values_file" {
  description = "Path to a custom Helm values file for kube-llmops-stack"
  type        = string
  default     = ""
}

variable "network_name" {
  description = "Name of the VPC network"
  type        = string
  default     = ""
}

variable "labels" {
  description = "Additional labels for GCP resources"
  type        = map(string)
  default     = {}
}
