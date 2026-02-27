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
  module_name = "temporal"
  temporal_config = {
    namespace       = "BREVIO"
    retention_days  = 30
    helm_chart_name = "temporal"
    workers = {
      task_queue = "BREVIO-tasks"
      min_replicas = 2
      max_replicas = 8
    }
  }
}

output "module_contract" {
  value = local.temporal_config
}
