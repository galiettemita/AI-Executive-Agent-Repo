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
  module_name = "feature-flags-cache"
  feature_flags_cache_config = {
    engine                     = "redis"
    dedicated_cluster          = true
    sub_millisecond_target     = true
    transit_encryption_enabled = true
  }
}

output "module_contract" {
  value = local.feature_flags_cache_config
}
