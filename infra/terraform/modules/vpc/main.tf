terraform {
  required_version = ">= 1.6.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
  }
}

variable "environment" {
  type    = string
  default = "staging"
}

locals {
  module_name = "vpc"
  vpc_config = {
    cidr_block           = "10.42.0.0/16"
    availability_zones   = ["us-east-1a", "us-east-1b", "us-east-1c"]
    public_subnet_cidrs  = ["10.42.0.0/20", "10.42.16.0/20", "10.42.32.0/20"]
    private_subnet_cidrs = ["10.42.128.0/20", "10.42.144.0/20", "10.42.160.0/20"]
    nat_gateway_enabled  = true
    security_groups = {
      gateway       = "ingress from alb to gateway only"
      control       = "allow service mesh control-plane traffic"
      executor      = "allow controlled egress for connectors"
      data_plane    = "rds/redis restricted access"
      observability = "metrics/log scraping"
    }
  }
}

output "module_contract" {
  value = local.vpc_config
}
