terraform {
  required_version = ">= 1.6.0"
}

locals {
  module_name = "cloudfront"
  config = {
    enabled              = true
    origins              = ["gateway-api", "media-bucket"]
    webhook_edge_accel   = true
    tls_minimum_protocol = "TLSv1.2_2021"
    caching_profile      = "api-optimized"
    waf_attached         = true
  }
}

output "module_contract" {
  value = local.config
}
