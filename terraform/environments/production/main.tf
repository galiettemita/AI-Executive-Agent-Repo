terraform {
  required_version = ">= 1.6.0"
}

module "vpc" {
  source = "../../modules/vpc"
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

module "sqs" {
  source = "../../modules/sqs"
}

module "s3" {
  source = "../../modules/s3"
}

module "secrets" {
  source = "../../modules/secrets"
}

module "temporal" {
  source = "../../modules/temporal"
}

module "observability" {
  source = "../../modules/observability"
}

module "opensearch" {
  source = "../../modules/opensearch"
}

module "admin_frontend" {
  source = "../../modules/admin-frontend"
}

module "feature_flags_cache" {
  source = "../../modules/feature-flags-cache"
}
