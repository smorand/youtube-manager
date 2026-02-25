# Docker Image Build - Uses kreuzwerker/docker provider
# Builds locally with docker_image, pushes with docker_registry_image

# ============================================
# DOCKER PROVIDER CONFIGURATION
# ============================================

# Configure Docker provider with Artifact Registry authentication
provider "docker" {
  registry_auth {
    address     = "${local.cloud_run_region}-docker.pkg.dev"
    config_file = pathexpand("~/.docker/config.json")
  }
}

# ============================================
# DOCKER IMAGE BUILD (LOCAL)
# ============================================

resource "docker_image" "mcp" {
  name = local.mcp_image

  build {
    context    = "${path.root}/.."
    dockerfile = "Dockerfile"

    label = {
      "org.opencontainers.image.source" = "https://github.com/smorand/youtube-manager"
      "org.opencontainers.image.title"  = "youtube-manager-mcp"
      "environment"                     = local.env
      "managed_by"                      = "terraform"
    }
  }

  # Triggers rebuild when source files change
  triggers = {
    dockerfile_hash = filesha256("${path.root}/../Dockerfile")
    go_mod_hash     = filesha256("${path.root}/../go.mod")
    go_sum_hash     = filesha256("${path.root}/../go.sum")
    main_hash       = filesha256("${path.root}/../cmd/youtube-manager-mcp/main.go")
    auth_hash       = filesha256("${path.root}/../internal/auth/auth.go")
    server_hash     = filesha256("${path.root}/../internal/mcpserver/server.go")
    playlist_hash   = filesha256("${path.root}/../internal/mcpserver/playlist_tools.go")
    video_hash      = filesha256("${path.root}/../internal/mcpserver/video_tools.go")
    download_hash   = filesha256("${path.root}/../internal/mcpserver/download_tools.go")
  }
}

# ============================================
# DOCKER IMAGE PUSH (TO ARTIFACT REGISTRY)
# ============================================

resource "docker_registry_image" "mcp" {
  name = docker_image.mcp.name

  keep_remotely = true

  triggers = {
    image_id = docker_image.mcp.image_id
  }

  depends_on = [google_artifact_registry_repository.mcp]
}

# ============================================
# OUTPUTS
# ============================================

output "docker_image" {
  description = "Full Docker image URL"
  value       = docker_registry_image.mcp.name
}

output "docker_image_digest" {
  description = "Docker image SHA256 digest"
  value       = docker_registry_image.mcp.sha256_digest
}
