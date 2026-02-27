terraform {
  required_version = ">= 1.6.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
  }
}

locals {
  module_name = "elasticache"
  redis_config = {
    engine                     = "redis"
    engine_version             = "7"
    node_type                  = "cache.r7g.large"
    num_cache_clusters         = 3
    automatic_failover_enabled = true
    transit_encryption_enabled = true
    at_rest_encryption_enabled = true
  }
}

output "module_contract" {
  value = local.redis_config
}
