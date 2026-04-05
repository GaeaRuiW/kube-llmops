output "cluster_endpoint" {
  description = "AKS cluster API server FQDN"
  value       = azurerm_kubernetes_cluster.main.fqdn
}

output "cluster_name" {
  description = "AKS cluster name"
  value       = azurerm_kubernetes_cluster.main.name
}

output "kube_config_raw" {
  description = "Raw kubeconfig for the AKS cluster"
  value       = azurerm_kubernetes_cluster.main.kube_config_raw
  sensitive   = true
}

output "kubeconfig_command" {
  description = "Command to configure kubectl"
  value       = "az aks get-credentials --resource-group ${azurerm_resource_group.main.name} --name ${azurerm_kubernetes_cluster.main.name}"
}

output "resource_group_name" {
  description = "Azure resource group name"
  value       = azurerm_resource_group.main.name
}

output "gpu_node_pool_name" {
  description = "Name of the GPU node pool"
  value       = azurerm_kubernetes_cluster_node_pool.gpu.name
}

output "cluster_identity" {
  description = "AKS cluster managed identity"
  value       = azurerm_kubernetes_cluster.main.identity[0].principal_id
}
