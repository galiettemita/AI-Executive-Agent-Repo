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
  module_name = "eks"
  eks_config = {
    kubernetes_version = "1.29"
    endpoint_private   = true
    endpoint_public    = true
    irsa_enabled       = true
    managed_node_groups = {
      general = {
        desired_size = 3
        min_size     = 2
        max_size     = 10
        instance_types = ["m6i.large"]
      }
      workers = {
        desired_size   = 2
        min_size       = 2
        max_size       = 8
        instance_types = ["m6i.xlarge"]
      }
    }
  }
}

output "module_contract" {
  value = local.eks_config
}
