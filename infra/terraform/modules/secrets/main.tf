terraform {
  required_version = ">= 1.6.0"
}

locals {
  module_name = "secrets"
  config = {
    primary_store           = "secrets-manager"
    kms_envelope_encryption = true
    rotation_days           = 90
    secret_name_pattern     = "brevio/{environment}/{service}/{key}"
    oauth_redirect_pattern  = "https://auth.brevio.app/callback/{service}"
  }
}

output "module_contract" {
  value = local.config
}
