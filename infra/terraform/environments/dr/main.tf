terraform {
  required_version = ">= 1.6.0"
}

locals {
  environment      = "dr"
  active_region    = "eu-west-1"
  failover_source  = "us-east-1"
  failover_enabled = true
}

module "eks" {
  source = "../../modules/eks"
}

module "rds" {
  source = "../../modules/rds"
}

module "elasticache" {
  source = "../../modules/elasticache"
}

module "sqs_sns" {
  source = "../../modules/sqs-sns"
}

module "s3" {
  source = "../../modules/s3"
}

module "secrets" {
  source = "../../modules/secrets"
}

module "cloudfront" {
  source = "../../modules/cloudfront"
}

module "route53" {
  source = "../../modules/route53"
}

module "monitoring" {
  source = "../../modules/monitoring"
}

module "waf" {
  source = "../../modules/waf"
}

output "environment_contract" {
  value = {
    environment      = local.environment
    active_region    = local.active_region
    failover_source  = local.failover_source
    failover_enabled = local.failover_enabled
    modules = {
      eks         = module.eks.module_contract
      rds         = module.rds.module_contract
      elasticache = module.elasticache.module_contract
      sqs_sns     = module.sqs_sns.module_contract
      s3          = module.s3.module_contract
      secrets     = module.secrets.module_contract
      cloudfront  = module.cloudfront.module_contract
      route53     = module.route53.module_contract
      monitoring  = module.monitoring.module_contract
      waf         = module.waf.module_contract
    }
  }
}
