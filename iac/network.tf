# Networking: VPC, Cloud NAT with static IP, VPC Connector
#
# Gives Cloud Run a fixed egress IP via:
# Cloud Run → VPC Connector → VPC → Cloud Router → Cloud NAT (static IP) → Internet

# ============================================
# VPC NETWORK
# ============================================

resource "google_compute_network" "main" {
  name                    = "${local.prefix}-vpc-${local.env}"
  auto_create_subnetworks = false
}

resource "google_compute_subnetwork" "connector" {
  name          = "${local.prefix}-connector-${local.env}"
  network       = google_compute_network.main.id
  region        = local.cloud_run_region
  ip_cidr_range = "10.8.0.0/28"
}

# ============================================
# STATIC IP
# ============================================

resource "google_compute_address" "nat" {
  name   = "${local.prefix}-nat-ip-${local.env}"
  region = local.cloud_run_region
}

# ============================================
# CLOUD ROUTER + NAT
# ============================================

resource "google_compute_router" "main" {
  name    = "${local.prefix}-router-${local.env}"
  network = google_compute_network.main.id
  region  = local.cloud_run_region
}

resource "google_compute_router_nat" "main" {
  name   = "${local.prefix}-nat-${local.env}"
  router = google_compute_router.main.name
  region = local.cloud_run_region

  nat_ip_allocate_option = "MANUAL_ONLY"
  nat_ips                = [google_compute_address.nat.self_link]

  source_subnetwork_ip_ranges_to_nat = "ALL_SUBNETWORKS_ALL_IP_RANGES"

  log_config {
    enable = false
    filter = "ERRORS_ONLY"
  }
}

# ============================================
# VPC CONNECTOR (Serverless VPC Access)
# ============================================

resource "google_vpc_access_connector" "main" {
  name   = "${local.prefix}-conn-${local.env}"
  region = local.cloud_run_region

  subnet {
    name = google_compute_subnetwork.connector.name
  }

  min_instances = 2
  max_instances = 3

  machine_type = "e2-micro"
}

# ============================================
# OUTPUTS
# ============================================

output "nat_ip" {
  description = "Static egress IP for Cloud Run"
  value       = google_compute_address.nat.address
}
