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

variable "cluster_name" {
  type    = string
  default = "brevio-production"
}

# Karpenter IAM role for node provisioning.
locals {
  karpenter = {
    node_role_name       = "${var.cluster_name}-karpenter-node"
    controller_role_name = "${var.cluster_name}-karpenter-controller"
    interruption_queue   = "${var.cluster_name}-karpenter-interruption"
    node_iam_policies = [
      "arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy",
      "arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy",
      "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
    ]
    controller_permissions = [
      "ec2:CreateLaunchTemplate", "ec2:CreateFleet", "ec2:RunInstances",
      "ec2:CreateTags", "ec2:TerminateInstances", "ec2:DeleteLaunchTemplate",
      "ec2:DescribeLaunchTemplates", "ec2:DescribeInstances",
      "ec2:DescribeSecurityGroups", "ec2:DescribeSubnets",
      "ec2:DescribeInstanceTypes", "ec2:DescribeInstanceTypeOfferings",
      "ec2:DescribeAvailabilityZones", "ec2:DescribeSpotPriceHistory",
      "pricing:GetProducts", "ssm:GetParameter"
    ]
    interruption_queue_config = {
      message_retention_seconds = 300
      sse_enabled               = true
    }
  }
}

output "module_contract" {
  value = merge(local.config, {
    karpenter = local.karpenter
  })
}
