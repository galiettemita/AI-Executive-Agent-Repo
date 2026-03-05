terraform {
  required_version = ">= 1.6.0"
}

locals {
  module_name = "route53"
  config = {
    weighted_routing = true
    health_checks = {
      primary_region = "us-east-1"
      dr_region      = "eu-west-1"
    }
    failover_strategy = "blue-green-canary"
    hosted_zone_mode  = "public"
  }
}

output "module_contract" {
  value = local.config
}
