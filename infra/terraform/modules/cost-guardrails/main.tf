terraform {
  required_version = ">= 1.6.0"
}

variable "environment" {
  type    = string
  default = "production"
}

variable "monthly_budget_usd" {
  type    = number
  default = 5000
}

variable "alert_email" {
  type    = string
  default = "ops@brevio.ai"
}

variable "alert_threshold_80" {
  type    = number
  default = 80
}

variable "alert_threshold_100" {
  type    = number
  default = 100
}

locals {
  module_name = "cost-guardrails"
  config = {
    environment         = var.environment
    monthly_budget_usd  = var.monthly_budget_usd
    alert_email         = var.alert_email
    alert_threshold_80  = var.alert_threshold_80
    alert_threshold_100 = var.alert_threshold_100
    budgets = {
      monthly = {
        name        = "brevio-${var.environment}-monthly"
        budget_type = "COST"
        limit_usd   = var.monthly_budget_usd
        time_unit   = "MONTHLY"
        notifications = [
          {
            comparison_operator = "GREATER_THAN"
            threshold           = var.alert_threshold_80
            threshold_type      = "PERCENTAGE"
            notification_type   = "FORECASTED"
          },
          {
            comparison_operator = "GREATER_THAN"
            threshold           = var.alert_threshold_100
            threshold_type      = "PERCENTAGE"
            notification_type   = "ACTUAL"
          }
        ]
      }
    }
    cost_allocation_tags = ["brevio-service", "brevio-environment"]
    anomaly_detection = {
      monitor_name      = "brevio-${var.environment}-anomaly"
      monitor_type      = "DIMENSIONAL"
      monitor_dimension = "SERVICE"
      frequency         = "DAILY"
      threshold_pct     = 20
    }
  }
}

output "module_contract" {
  value = local.config
}
