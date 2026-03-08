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
  module_name = "admin-frontend"
  admin_frontend_config = {
    hosting = {
      bucket      = "brevio-admin-frontend"
      cdn         = "cloudfront"
      waf_enabled = true
    }
    admin_ip_allowlist_enabled = true
  }
}

output "module_contract" {
  value = local.admin_frontend_config
}
