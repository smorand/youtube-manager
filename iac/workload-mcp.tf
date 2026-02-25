# MCP Server Workload: Cloud Run service with Artifact Registry
#
# Resources:
# - Artifact Registry repository for container images
# - Cloud Run service for MCP server
# - Service account IAM permissions (Secret Manager)
# - IAM binding for unauthenticated access
# - DNS CNAME record for custom domain
# - Cloud Run domain mapping

# ============================================
# LOCALS
# ============================================

locals {
  # MCP service configuration from config.yaml
  mcp_name = lookup(local.cloud_run_config, "name", "youtube-manager-mcp")

  # Artifact Registry configuration
  artifact_registry_config = lookup(local.gcp_resources, "artifact_registry", {})
  artifact_registry_name   = lookup(local.artifact_registry_config, "name", "youtube-manager")
  artifact_registry_format = lookup(local.artifact_registry_config, "format", "DOCKER")

  # Docker image (uses Artifact Registry)
  mcp_image = "${local.cloud_run_region}-docker.pkg.dev/${local.project_id}/${google_artifact_registry_repository.mcp.name}/${local.mcp_name}:latest"

  # Resource limits from cloud_run config
  mcp_cpu           = lookup(local.cloud_run_config, "cpu", "1")
  mcp_memory        = lookup(local.cloud_run_config, "memory", "512Mi")
  mcp_min_instances = lookup(local.cloud_run_config, "min_instances", 0)
  mcp_max_instances = lookup(local.cloud_run_config, "max_instances", 3)

  # Access configuration
  allow_unauthenticated = lookup(local.cloud_run_config, "allow_unauthenticated", true)

  # Custom domain
  custom_domain = lookup(local.cloud_run_config, "custom_domain", "")

  # Service account email from init module (referenced by name)
  mcp_service_account = "${local.prefix}-cloudrun-${local.env}@${local.project_id}.iam.gserviceaccount.com"

  # Secret names
  oauth_secret_id = google_secret_manager_secret.oauth_credentials.secret_id
  token_secret_id = google_secret_manager_secret.youtube_token.secret_id
}

# ============================================
# DATA SOURCES
# ============================================

data "google_project" "current" {
  project_id = local.project_id
}

# ============================================
# ARTIFACT REGISTRY
# ============================================

resource "google_artifact_registry_repository" "mcp" {
  repository_id = local.artifact_registry_name
  location      = local.cloud_run_region
  format        = local.artifact_registry_format
  description   = "Docker repository for YouTube Manager MCP server"

  labels = {
    environment = local.env
    managed_by  = "terraform"
  }
}

# ============================================
# CLOUD RUN SERVICE
# ============================================

resource "google_cloud_run_v2_service" "mcp" {
  name                = local.mcp_name
  location            = local.cloud_run_region
  ingress             = "INGRESS_TRAFFIC_ALL"
  deletion_protection = false

  template {
    service_account = local.mcp_service_account

    scaling {
      min_instance_count = local.mcp_min_instances
      max_instance_count = local.mcp_max_instances
    }

    containers {
      image = local.mcp_image

      resources {
        limits = {
          cpu    = local.mcp_cpu
          memory = local.mcp_memory
        }
        cpu_idle = true
      }

      ports {
        container_port = 8080
      }

      # Environment variables
      env {
        name  = "OAUTH_CREDENTIALS_FILE"
        value = "/secrets/creds/scm-pwd-web.json"
      }

      env {
        name  = "YOUTUBE_TOKEN_FILE"
        value = "/secrets/token/youtube-token.json"
      }

      env {
        name  = "ENVIRONMENT"
        value = local.env
      }

      # Secret volume mounts
      volume_mounts {
        name       = "oauth-creds"
        mount_path = "/secrets/creds"
      }

      volume_mounts {
        name       = "youtube-token"
        mount_path = "/secrets/token"
      }
    }

    # Secret volumes
    volumes {
      name = "oauth-creds"
      secret {
        secret = local.oauth_secret_id
        items {
          version = "latest"
          path    = "scm-pwd-web.json"
        }
      }
    }

    volumes {
      name = "youtube-token"
      secret {
        secret = local.token_secret_id
        items {
          version = "latest"
          path    = "youtube-token.json"
        }
      }
    }
  }

  traffic {
    type    = "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST"
    percent = 100
  }

  depends_on = [
    google_artifact_registry_repository.mcp,
    docker_registry_image.mcp,
  ]
}

# ============================================
# SERVICE ACCOUNT PERMISSIONS
# ============================================

# Grant Secret Manager access to Cloud Run service account
resource "google_project_iam_member" "mcp_secretmanager" {
  project = local.project_id
  role    = "roles/secretmanager.secretAccessor"
  member  = "serviceAccount:${local.mcp_service_account}"
}

# ============================================
# IAM BINDING FOR PUBLIC ACCESS
# ============================================

resource "google_cloud_run_v2_service_iam_member" "mcp_public" {
  count    = local.allow_unauthenticated ? 1 : 0
  project  = local.project_id
  location = google_cloud_run_v2_service.mcp.location
  name     = google_cloud_run_v2_service.mcp.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# ============================================
# CUSTOM DOMAIN
# ============================================

# DNS CNAME record for custom domain
resource "google_dns_record_set" "mcp_cname" {
  count        = local.custom_domain != "" ? 1 : 0
  name         = "${local.custom_domain}."
  managed_zone = "scm-platform-org"
  type         = "CNAME"
  ttl          = 300
  rrdatas      = ["ghs.googlehosted.com."]
}

# Cloud Run domain mapping
resource "google_cloud_run_domain_mapping" "mcp" {
  count    = local.custom_domain != "" ? 1 : 0
  location = local.cloud_run_region
  name     = local.custom_domain

  metadata {
    namespace = local.project_id
  }

  spec {
    route_name = google_cloud_run_v2_service.mcp.name
  }

  depends_on = [google_dns_record_set.mcp_cname]
}

# ============================================
# OUTPUTS
# ============================================

output "mcp_url" {
  description = "MCP server URL"
  value       = google_cloud_run_v2_service.mcp.uri
}

output "mcp_custom_url" {
  description = "MCP server custom domain URL"
  value       = local.custom_domain != "" ? "https://${local.custom_domain}" : ""
}

output "mcp_service_account" {
  description = "MCP service account email"
  value       = local.mcp_service_account
}

output "artifact_registry_url" {
  description = "Artifact Registry repository URL"
  value       = "${local.cloud_run_region}-docker.pkg.dev/${local.project_id}/${google_artifact_registry_repository.mcp.name}"
}
