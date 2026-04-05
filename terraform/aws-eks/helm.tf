################################################################################
# Helm Releases - kube-llmops stack and dependencies
################################################################################

# Wait for the cluster to be fully ready before installing Helm charts
resource "time_sleep" "wait_for_cluster" {
  depends_on      = [module.eks]
  create_duration = "30s"
}

################################################################################
# NVIDIA GPU Operator
################################################################################

resource "helm_release" "nvidia_gpu_operator" {
  count = var.enable_gpu_operator ? 1 : 0

  name             = "gpu-operator"
  repository       = "https://helm.ngc.nvidia.com/nvidia"
  chart            = "gpu-operator"
  namespace        = "gpu-operator"
  create_namespace = true
  version          = "v24.3.0"
  timeout          = 600

  # On EKS with GPU AMI, the driver is pre-installed
  set {
    name  = "driver.enabled"
    value = "false"
  }

  set {
    name  = "toolkit.enabled"
    value = "true"
  }

  set {
    name  = "devicePlugin.enabled"
    value = "true"
  }

  # Tolerate GPU taints so the operator can run on GPU nodes
  set {
    name  = "operator.defaultRuntime"
    value = "containerd"
  }

  depends_on = [time_sleep.wait_for_cluster]
}

################################################################################
# KEDA (Kubernetes Event-Driven Autoscaling)
################################################################################

resource "helm_release" "keda" {
  count = var.enable_keda ? 1 : 0

  name             = "keda"
  repository       = "https://kedacore.github.io/charts"
  chart            = "keda"
  namespace        = "keda"
  create_namespace = true
  version          = "2.14.0"
  timeout          = 300

  set {
    name  = "resources.operator.requests.cpu"
    value = "100m"
  }

  set {
    name  = "resources.operator.requests.memory"
    value = "128Mi"
  }

  depends_on = [time_sleep.wait_for_cluster]
}

################################################################################
# kube-llmops-stack
################################################################################

resource "helm_release" "kube_llmops" {
  name             = "kube-llmops"
  chart            = "${path.module}/../../charts/kube-llmops-stack"
  namespace        = "kube-llmops"
  create_namespace = true
  timeout          = 900

  # Use a custom values file if provided
  values = var.kube_llmops_values_file != "" ? [file(var.kube_llmops_values_file)] : [
    yamlencode({
      # Default values for AWS EKS deployment
      vllm = {
        enabled = true
        resources = {
          limits = {
            "nvidia.com/gpu" = 1
          }
        }
        nodeSelector = {
          role = "gpu"
        }
        tolerations = [{
          key      = "nvidia.com/gpu"
          operator = "Equal"
          value    = "present"
          effect   = "NoSchedule"
        }]
      }

      observability = {
        enabled = true
      }

      litellm = {
        enabled = true
      }

      # Storage class for PVCs
      global = {
        storageClass = "gp3"
      }
    })
  ]

  depends_on = [
    time_sleep.wait_for_cluster,
    helm_release.nvidia_gpu_operator,
    kubernetes_storage_class.gp3,
  ]
}
