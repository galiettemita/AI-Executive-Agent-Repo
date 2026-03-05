terraform {
  required_version = ">= 1.6.0"
}

locals {
  module_name = "sqs-sns"
  config = {
    queues = {
      primary = {
        max_receive_count = 3
        retention_days    = 14
      }
      dead_letter = {
        retention_days = 14
      }
    }
    topics = {
      events   = true
      alerts   = true
      workflow = true
    }
    delivery_guarantee = "at-least-once"
  }
}

output "module_contract" {
  value = local.config
}
