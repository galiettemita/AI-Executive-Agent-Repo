terraform {
  required_version = ">= 1.6.0"
}

locals {
  environment    = "production"
  primary_region = "us-east-1"
  dr_region      = "eu-west-1"
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

module "cost_guardrails" {
  source             = "../../modules/cost-guardrails"
  environment        = local.environment
  monthly_budget_usd = 5000
  alert_email        = var.ops_alert_email
}

variable "ops_alert_email" {
  type    = string
  default = "ops@brevio.ai"
}

output "environment_contract" {
  value = {
    environment    = local.environment
    primary_region = local.primary_region
    dr_region      = local.dr_region
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
      waf              = module.waf.module_contract
      cost_guardrails  = module.cost_guardrails.module_contract
    }
  }
}
