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
  module_name = "s3"
  s3_config = {
    buckets = [
      "attachments",
      "sboms",
      "exports",
      "schemas",
    ]
    versioning_enabled = true
    encryption = {
      algorithm = "aws:kms"
    }
    lifecycle_expiration_days = 365
  }
}

output "module_contract" {
  value = local.s3_config
}
