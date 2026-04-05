output "cluster_endpoint" {
  description = "GKE cluster API server endpoint"
  value       = google_container_cluster.main.endpoint
}

output "cluster_name" {
  description = "GKE cluster name"
  value       = google_container_cluster.main.name
}

output "cluster_ca_certificate" {
  description = "GKE cluster CA certificate (base64-encoded)"
  value       = google_container_cluster.main.master_auth[0].cluster_ca_certificate
  sensitive   = true
}

output "kubeconfig_command" {
  description = "Command to configure kubectl"
  value       = "gcloud container clusters get-credentials ${google_container_cluster.main.name} --region ${var.region} --project ${var.project_id}"
}

output "network_name" {
  description = "VPC network name"
  value       = google_compute_network.main.name
}

output "gpu_node_pool_name" {
  description = "Name of the GPU node pool"
  value       = google_container_node_pool.gpu.name
}
