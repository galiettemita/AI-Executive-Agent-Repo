terraform {
  required_version = ">= 1.6.0"
}

locals {
  module_name = "elasticache"
  config = {
    engine                     = "redis"
    engine_version             = "7"
    node_type                  = "cache.r6g.large"
    cluster_size               = 3
    automatic_failover_enabled = true
    transit_encryption_enabled = true
    at_rest_encryption_enabled = true
    maintenance_window_utc     = "sun:03:00-sun:04:00"
  }
}

output "module_contract" {
  value = local.config
}
