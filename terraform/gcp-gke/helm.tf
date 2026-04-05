################################################################################
# Helm Releases - kube-llmops stack and dependencies
################################################################################

# Wait for the cluster and node pools to be fully ready
resource "time_sleep" "wait_for_cluster" {
  depends_on = [
    google_container_node_pool.system,
    google_container_node_pool.gpu,
  ]
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

  # GKE auto-installs GPU drivers via the guest_accelerator config
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

  values = var.kube_llmops_values_file != "" ? [file(var.kube_llmops_values_file)] : [
    yamlencode({
      vllm = {
        enabled = true
        resources = {
          limits = {
            "nvidia.com/gpu" = 1
          }
        }
        nodeSelector = {
          "cloud.google.com/gke-accelerator" = var.gpu_type
          role                                = "gpu"
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

      global = {
        storageClass = "premium-rwo"
      }
    })
  ]

  depends_on = [
    time_sleep.wait_for_cluster,
    helm_release.nvidia_gpu_operator,
  ]
}
