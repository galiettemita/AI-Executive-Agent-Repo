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

# --- PagerDuty Integration (P3-08) ---

variable "pagerduty_token" {
  description = "PagerDuty API token"
  type        = string
  sensitive   = true
  default     = ""
}

variable "alert_provider" {
  description = "Alert provider: pagerduty, opsgenie, slack, webhook"
  type        = string
  default     = "webhook"
}

variable "oncall_user_ids" {
  description = "PagerDuty user IDs for on-call rotation"
  type        = list(string)
  default     = []
}

resource "pagerduty_service" "brevio_alerts" {
  count                   = var.alert_provider == "pagerduty" ? 1 : 0
  name                    = "Brevio AI Agent Alerts"
  auto_resolve_timeout    = 14400
  acknowledgement_timeout = 600
  escalation_policy       = pagerduty_escalation_policy.brevio[0].id
  alert_creation          = "create_alerts_and_incidents"
}

resource "pagerduty_escalation_policy" "brevio" {
  count = var.alert_provider == "pagerduty" ? 1 : 0
  name  = "Brevio On-Call"
  rule {
    escalation_delay_in_minutes = 30
    target {
      type = "schedule_reference"
      id   = pagerduty_schedule.brevio_oncall[0].id
    }
  }
}

resource "pagerduty_schedule" "brevio_oncall" {
  count     = var.alert_provider == "pagerduty" ? 1 : 0
  name      = "Brevio 24/7 On-Call"
  time_zone = "UTC"
  layer {
    name                         = "Primary"
    start                        = "2025-01-01T00:00:00Z"
    rotation_virtual_start       = "2025-01-01T00:00:00Z"
    rotation_turn_length_seconds = 604800
    users                        = var.oncall_user_ids
  }
}
