terraform {
  required_version = ">= 1.6.0"
  required_providers {
    helm = {
      source  = "hashicorp/helm"
      version = ">= 2.12"
    }
  }
}

locals {
  module_name = "observability"
  observability_config = {
    stack = [
      "prometheus",
      "grafana",
      "loki",
      "jaeger",
      "otel_collector",
    ]
    logs_retention_days    = 30
    metrics_retention_days = 30
  }
}

output "module_contract" {
  value = local.observability_config
}
