terraform {
  required_version = ">= 1.6.0"
}

locals {
  module_name = "monitoring"
  config = {
    cloudwatch = {
      enabled            = true
      log_retention_days = 30
    }
    grafana = {
      enabled         = true
      metrics_backend = "prometheus"
      traces_backend  = "tempo"
      logs_backend    = "loki"
    }
    alerting = {
      pagerduty            = true
      slo_burn_rate_alerts = true
    }
  }
}

output "module_contract" {
  value = local.config
}
