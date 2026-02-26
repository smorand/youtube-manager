# Secret Manager: OAuth client credentials
#
# Resources:
# - Secret for OAuth client credentials (client_id, client_secret)
#
# The OAuth2 server loads credentials from Secret Manager via API at runtime.
# No volume mounts needed - the service account has secretAccessor role.
#
# Note: Secret versions (actual data) should be created manually via gcloud.

# ============================================
# LOCALS
# ============================================

locals {
  secrets_config         = lookup(local.config, "secrets", {})
  oauth_credentials_name = lookup(local.secrets_config, "oauth_credentials", "oauth-credentials")
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
# OUTPUTS
# ============================================

output "oauth_secret_name" {
  description = "Secret Manager secret name for OAuth credentials"
  value       = google_secret_manager_secret.oauth_credentials.secret_id
}

# ============================================
# MANUAL SECRET VERSION CREATION
# ============================================
#
# After Terraform creates the secret, add a version with gcloud:
#
#   gcloud secrets versions add scm-pwd-ytm-oauth-creds \
#     --data-file=$HOME/.credentials/scm-pwd-web.json \
#     --project=project-fb127223-bfef-43d1-94e
#
# The OAuth2 server accesses this via Secret Manager API (not volume mounts).
