terraform {
  required_version = ">= 1.6.0"
}

locals {
  module_name = "s3"
  config = {
    buckets = {
      media     = true
      archive   = true
      backups   = true
      artifacts = true
    }
    lifecycle_policy = {
      to_infrequent_access_days = 30
      to_glacier_days           = 90
    }
    encryption = "SSE-KMS"
    versioning = true
  }
}

output "module_contract" {
  value = local.config
}
