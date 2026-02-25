# Secret Manager: OAuth credentials and YouTube token
#
# Resources:
# - Secret for OAuth client credentials (client_id, client_secret)
# - Secret for YouTube API token (refresh_token)
#
# Note: Secret versions (actual data) should be created manually via gcloud.

# ============================================
# LOCALS
# ============================================

locals {
  secrets_config         = lookup(local.config, "secrets", {})
  oauth_credentials_name = lookup(local.secrets_config, "oauth_credentials", "oauth-credentials")
  youtube_token_name     = lookup(local.secrets_config, "youtube_token", "youtube-token")
}

# ============================================
# OAUTH CREDENTIALS SECRET
# ============================================

resource "google_secret_manager_secret" "oauth_credentials" {
  secret_id = local.oauth_credentials_name

  replication {
    auto {}
  }

  labels = {
    environment = local.env
    managed_by  = "terraform"
    purpose     = "oauth-credentials"
  }
}

# ============================================
# YOUTUBE TOKEN SECRET
# ============================================

resource "google_secret_manager_secret" "youtube_token" {
  secret_id = local.youtube_token_name

  replication {
    auto {}
  }

  labels = {
    environment = local.env
    managed_by  = "terraform"
    purpose     = "youtube-token"
  }
}

# ============================================
# OUTPUTS
# ============================================

output "oauth_secret_name" {
  description = "Secret Manager secret name for OAuth credentials"
  value       = google_secret_manager_secret.oauth_credentials.secret_id
}

output "youtube_token_secret_name" {
  description = "Secret Manager secret name for YouTube token"
  value       = google_secret_manager_secret.youtube_token.secret_id
}

# ============================================
# MANUAL SECRET VERSION CREATION
# ============================================
#
# After Terraform creates the secrets, add versions with gcloud:
#
#   # OAuth credentials
#   gcloud secrets versions add scm-pwd-ytm-oauth-creds \
#     --data-file=$HOME/.credentials/scm-pwd-web.json \
#     --project=project-fb127223-bfef-43d1-94e
#
#   # YouTube token
#   gcloud secrets versions add scm-pwd-ytm-token \
#     --data-file=$HOME/.credentials/youtube-token.json \
#     --project=project-fb127223-bfef-43d1-94e
#
# Cloud Run reads secrets via volume mounts (configured in workload-mcp.tf).
