variable "cluster_name" {
  description = "Name of the EKS cluster"
  type        = string
  default     = "kube-llmops"
}

variable "region" {
  description = "AWS region for the cluster"
  type        = string
  default     = "us-west-2"
}

variable "kubernetes_version" {
  description = "Kubernetes version for the EKS cluster"
  type        = string
  default     = "1.29"
}

variable "gpu_instance_type" {
  description = "EC2 instance type for GPU node group"
  type        = string
  default     = "g5.xlarge"
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
  description = "Desired number of GPU nodes"
  type        = number
  default     = 1
}

variable "system_instance_type" {
  description = "EC2 instance type for system node group"
  type        = string
  default     = "t3.large"
}

variable "system_node_count" {
  description = "Number of system nodes"
  type        = number
  default     = 2
}

variable "vpc_cidr" {
  description = "CIDR block for the VPC"
  type        = string
  default     = "10.0.0.0/16"
}

variable "enable_keda" {
  description = "Whether to install KEDA for autoscaling"
  type        = bool
  default     = true
}

variable "enable_gpu_operator" {
  description = "Whether to install NVIDIA GPU Operator"
  type        = bool
  default     = true
}

variable "kube_llmops_values_file" {
  description = "Path to a custom Helm values file for kube-llmops-stack"
  type        = string
  default     = ""
}

variable "tags" {
  description = "Additional tags for all resources"
  type        = map(string)
  default     = {}
}
