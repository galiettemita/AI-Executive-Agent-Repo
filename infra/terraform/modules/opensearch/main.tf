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
  module_name = "opensearch"
  opensearch_config = {
    mode               = "hybrid_rag"
    data_nodes         = 3
    ultra_warm_enabled = true
    enforce_https      = true
    encryption_at_rest = true
  }
}

output "module_contract" {
  value = local.opensearch_config
}
