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
  module_name = "rds"
  rds_config = {
    engine            = "postgres"
    engine_version    = "16"
    instance_class    = "db.r6g.large"
    allocated_storage = 100
    multi_az          = true
    storage_encrypted = true
    pgbouncer_sidecar = true
    backup = {
      retention_days = 14
      pitr_enabled   = true
    }
  }
}

output "module_contract" {
  value = local.rds_config
}
