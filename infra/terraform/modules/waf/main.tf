terraform {
  required_version = ">= 1.6.0"
}

locals {
  module_name = "waf"
  config = {
    managed_rule_sets = [
      "AWSManagedRulesCommonRuleSet",
      "AWSManagedRulesKnownBadInputsRuleSet",
      "AWSManagedRulesAmazonIpReputationList",
    ]
    rate_limit = {
      requests_per_5m = 3000
    }
    geo_blocking = {
      enabled           = false
      country_allowlist = []
    }
    bot_control_enabled = true
  }
}

output "module_contract" {
  value = local.config
}
