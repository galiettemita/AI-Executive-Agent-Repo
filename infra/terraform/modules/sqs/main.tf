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
  module_name = "sqs"
  sqs_config = {
    fifo_queues = [
      "interactive_turns.fifo",
    ]
    standard_queues = [
      "workflow_tasks",
      "ledger_writes",
      "trajectory_writes",
      "rate_limit_ledger_writes",
    ]
    dead_letter_queues = [
      "interactive_turns_dlq",
      "workflow_tasks_dlq",
      "ledger_writes_dlq",
      "trajectory_writes_dlq",
      "rate_limit_ledger_writes_dlq",
    ]
    alarms_enabled = true
    redrive_policy = true
  }
}

output "module_contract" {
  value = local.sqs_config
}
