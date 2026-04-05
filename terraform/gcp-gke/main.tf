locals {
  network_name = var.network_name != "" ? var.network_name : "${var.cluster_name}-vpc"
  common_labels = merge(var.labels, {
    project    = "kube-llmops"
    managed-by = "terraform"
  })
}

################################################################################
# VPC Network
################################################################################

resource "google_compute_network" "main" {
  name                    = local.network_name
  project                 = var.project_id
  auto_create_subnetworks = false
}

resource "google_compute_subnetwork" "main" {
  name          = "${var.cluster_name}-subnet"
  project       = var.project_id
  region        = var.region
  network       = google_compute_network.main.id
  ip_cidr_range = "10.0.0.0/20"

  secondary_ip_range {
    range_name    = "pods"
    ip_cidr_range = "10.4.0.0/14"
  }

  secondary_ip_range {
    range_name    = "services"
    ip_cidr_range = "10.8.0.0/20"
  }

  private_ip_google_access = true
}

################################################################################
# Cloud Router + NAT (for private nodes)
################################################################################

resource "google_compute_router" "main" {
  name    = "${var.cluster_name}-router"
  project = var.project_id
  region  = var.region
  network = google_compute_network.main.id
}

resource "google_compute_router_nat" "main" {
  name                               = "${var.cluster_name}-nat"
  project                            = var.project_id
  router                             = google_compute_router.main.name
  region                             = var.region
  nat_ip_allocate_option             = "AUTO_ONLY"
  source_subnetwork_ip_ranges_to_nat = "ALL_SUBNETWORKS_ALL_IP_RANGES"

  log_config {
    enable = false
    filter = "ERRORS_ONLY"
  }
}

################################################################################
# GKE Cluster
################################################################################

resource "google_container_cluster" "main" {
  name     = var.cluster_name
  project  = var.project_id
  location = var.region

  # Use REGULAR release channel for stable, auto-updating Kubernetes
  release_channel {
    channel = "REGULAR"
  }

  min_master_version = var.kubernetes_version != "" ? var.kubernetes_version : null

  network    = google_compute_network.main.id
  subnetwork = google_compute_subnetwork.main.id

  # Use VPC-native (alias IP) networking
  ip_allocation_policy {
    cluster_secondary_range_name  = "pods"
    services_secondary_range_name = "services"
  }

  # Remove the default node pool; we manage our own
  remove_default_node_pool = true
  initial_node_count       = 1

  # Enable workload identity for secure service account mapping
  workload_identity_config {
    workload_pool = "${var.project_id}.svc.id.goog"
  }

  # Enable network policy
  network_policy {
    enabled = true
  }

  # Logging and monitoring
  logging_config {
    enable_components = ["SYSTEM_COMPONENTS", "WORKLOADS"]
  }

  monitoring_config {
    enable_components = ["SYSTEM_COMPONENTS"]
    managed_prometheus {
      enabled = true
    }
  }

  resource_labels = local.common_labels
}

################################################################################
# System Node Pool
################################################################################

resource "google_container_node_pool" "system" {
  name     = "system"
  project  = var.project_id
  location = var.region
  cluster  = google_container_cluster.main.name

  node_count = var.system_node_count

  node_config {
    machine_type = var.system_machine_type
    disk_size_gb = 100
    disk_type    = "pd-ssd"

    oauth_scopes = [
      "https://www.googleapis.com/auth/cloud-platform",
    ]

    labels = {
      role = "system"
    }

    workload_metadata_config {
      mode = "GKE_METADATA"
    }

    shielded_instance_config {
      enable_secure_boot          = true
      enable_integrity_monitoring = true
    }
  }

  management {
    auto_repair  = true
    auto_upgrade = true
  }
}

################################################################################
# GPU Node Pool
################################################################################

resource "google_container_node_pool" "gpu" {
  name     = "gpu"
  project  = var.project_id
  location = var.region
  cluster  = google_container_cluster.main.name

  initial_node_count = var.gpu_desired_size

  autoscaling {
    min_node_count = var.gpu_min_size
    max_node_count = var.gpu_max_size
  }

  node_config {
    machine_type = var.gpu_machine_type
    disk_size_gb = 200
    disk_type    = "pd-ssd"

    # Attach GPU accelerator
    guest_accelerator {
      type  = var.gpu_type
      count = var.gpu_count

      gpu_driver_installation_config {
        gpu_driver_version = "DEFAULT"
      }
    }

    oauth_scopes = [
      "https://www.googleapis.com/auth/cloud-platform",
    ]

    labels = {
      role                         = "gpu"
      "nvidia.com/gpu.present"     = "true"
      "cloud.google.com/gke-accelerator" = var.gpu_type
    }

    # GPU taint - only pods that tolerate this will be scheduled
    taint {
      key    = "nvidia.com/gpu"
      value  = "present"
      effect = "NO_SCHEDULE"
    }

    workload_metadata_config {
      mode = "GKE_METADATA"
    }
  }

  management {
    auto_repair  = true
    auto_upgrade = true
  }
}
