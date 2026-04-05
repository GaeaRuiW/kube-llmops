locals {
  common_tags = merge(var.tags, {
    Project   = "kube-llmops"
    ManagedBy = "terraform"
  })
}

################################################################################
# Resource Group
################################################################################

resource "azurerm_resource_group" "main" {
  name     = var.resource_group_name
  location = var.location
  tags     = local.common_tags
}

################################################################################
# Virtual Network
################################################################################

resource "azurerm_virtual_network" "main" {
  name                = "${var.cluster_name}-vnet"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  address_space       = ["10.0.0.0/16"]
  tags                = local.common_tags
}

resource "azurerm_subnet" "aks" {
  name                 = "${var.cluster_name}-aks-subnet"
  resource_group_name  = azurerm_resource_group.main.name
  virtual_network_name = azurerm_virtual_network.main.name
  address_prefixes     = ["10.0.0.0/20"]
}

################################################################################
# AKS Cluster
################################################################################

resource "azurerm_kubernetes_cluster" "main" {
  name                = var.cluster_name
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  dns_prefix          = var.cluster_name
  kubernetes_version  = var.kubernetes_version

  # System node pool (default)
  default_node_pool {
    name                = "system"
    node_count          = var.system_node_count
    vm_size             = var.system_vm_size
    vnet_subnet_id      = azurerm_subnet.aks.id
    os_disk_size_gb     = 128
    os_disk_type        = "Managed"
    type                = "VirtualMachineScaleSets"
    enable_auto_scaling = false

    node_labels = {
      role = "system"
    }
  }

  identity {
    type = "SystemAssigned"
  }

  # Azure CNI for better networking performance
  network_profile {
    network_plugin    = "azure"
    network_policy    = "calico"
    load_balancer_sku = "standard"
    service_cidr      = "10.1.0.0/16"
    dns_service_ip    = "10.1.0.10"
  }

  # Enable Azure Monitor (optional)
  oms_agent {
    log_analytics_workspace_id = azurerm_log_analytics_workspace.main.id
  }

  tags = local.common_tags
}

################################################################################
# Log Analytics Workspace (for Azure Monitor)
################################################################################

resource "azurerm_log_analytics_workspace" "main" {
  name                = "${var.cluster_name}-logs"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  sku                 = "PerGB2018"
  retention_in_days   = 30
  tags                = local.common_tags
}

################################################################################
# GPU Node Pool
################################################################################

resource "azurerm_kubernetes_cluster_node_pool" "gpu" {
  name                  = "gpu"
  kubernetes_cluster_id = azurerm_kubernetes_cluster.main.id
  vm_size               = var.gpu_vm_size
  node_count            = var.gpu_desired_count
  vnet_subnet_id        = azurerm_subnet.aks.id
  os_disk_size_gb       = 200
  os_disk_type          = "Managed"

  enable_auto_scaling = true
  min_count           = var.gpu_min_count
  max_count           = var.gpu_max_count

  node_labels = {
    role                     = "gpu"
    "nvidia.com/gpu.present" = "true"
  }

  node_taints = [
    "nvidia.com/gpu=present:NoSchedule"
  ]

  tags = local.common_tags
}

################################################################################
# Storage Class for Azure Managed Disks (Premium SSD)
################################################################################

resource "kubernetes_storage_class" "azure_premium" {
  metadata {
    name = "managed-premium-retain"
  }

  storage_provisioner = "disk.csi.azure.com"
  reclaim_policy      = "Retain"
  volume_binding_mode = "WaitForFirstConsumer"

  parameters = {
    skuName = "Premium_LRS"
  }

  depends_on = [azurerm_kubernetes_cluster.main]
}
