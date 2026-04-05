variable "resource_group_name" {
  description = "Name of the Azure resource group"
  type        = string
  default     = "kube-llmops-rg"
}

variable "location" {
  description = "Azure region for the cluster"
  type        = string
  default     = "eastus"
}

variable "cluster_name" {
  description = "Name of the AKS cluster"
  type        = string
  default     = "kube-llmops"
}

variable "kubernetes_version" {
  description = "Kubernetes version for the AKS cluster"
  type        = string
  default     = "1.29"
}

variable "system_vm_size" {
  description = "VM size for the system node pool"
  type        = string
  default     = "Standard_D4s_v3"
}

variable "system_node_count" {
  description = "Number of system nodes"
  type        = number
  default     = 2
}

variable "gpu_vm_size" {
  description = "VM size for the GPU node pool"
  type        = string
  default     = "Standard_NC6s_v3"
}

variable "gpu_min_count" {
  description = "Minimum number of GPU nodes"
  type        = number
  default     = 0
}

variable "gpu_max_count" {
  description = "Maximum number of GPU nodes"
  type        = number
  default     = 4
}

variable "gpu_desired_count" {
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
  description = "Additional tags for Azure resources"
  type        = map(string)
  default     = {}
}
