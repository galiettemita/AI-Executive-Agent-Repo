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
  module_name = "secrets"
  secrets_config = {
    managed_secrets = [
      "app_secret",
      "encryption_keys",
      "oauth_client_secrets",
    ]
    rotation_enabled                 = true
    dual_key_read_window_minutes_min = 10
  }
}

output "module_contract" {
  value = local.secrets_config
}
