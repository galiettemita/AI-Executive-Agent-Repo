terraform {
  required_version = ">= 1.6.0"
}

locals {
  module_name = "eks"
  config = {
    kubernetes_version = "1.29"
    node_groups = {
      system = {
        instance_type = "t3.medium"
        min_size      = 3
        max_size      = 3
      }
      workers = {
        instance_type = "c6i.xlarge"
        min_size      = 3
        max_size      = 12
      }
      gpu = {
        instance_type = "g5.xlarge"
        min_size      = 1
        max_size      = 3
      }
    }
    service_mesh = {
      mtls                    = "spiffe-spire"
      inter_service_protocol  = "grpc"
      deterministic_tracepath = true
    }
  }
}

output "module_contract" {
  value = local.config
}
