terraform {
  required_version = ">= 1.6.0"
}

locals {
  module_name = "rds"
  config = {
    engine               = "postgres"
    engine_version       = "16"
    instance_class       = "db.r6g.xlarge"
    multi_az             = true
    allocated_storage_gb = 500
    storage_type         = "gp3"
    backup_retention     = 14
    encryption           = "AES-256 (KMS)"
    deletion_protection  = true
  }
}

output "module_contract" {
  value = local.config
}
