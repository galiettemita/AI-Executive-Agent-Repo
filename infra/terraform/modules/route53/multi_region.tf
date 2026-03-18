# Multi-region Route53 routing — weighted + geolocation (P3-09).

variable "hosted_zone_id" {
  type    = string
  default = ""
}

variable "primary_lb_dns_name" {
  type    = string
  default = ""
}

variable "primary_lb_zone_id" {
  type    = string
  default = ""
}

variable "secondary_lb_dns_name" {
  type    = string
  default = ""
}

variable "secondary_lb_zone_id" {
  type    = string
  default = ""
}

# Weighted routing: 80% us-east-1, 20% eu-west-1.
resource "aws_route53_record" "brevio_api_primary" {
  count   = var.hosted_zone_id != "" ? 1 : 0
  zone_id = var.hosted_zone_id
  name    = "api.brevio.ai"
  type    = "A"

  weighted_routing_policy {
    weight = 80
  }

  set_identifier = "primary-us-east-1"

  alias {
    name                   = var.primary_lb_dns_name
    zone_id                = var.primary_lb_zone_id
    evaluate_target_health = true
  }
}

resource "aws_route53_record" "brevio_api_secondary" {
  count   = var.hosted_zone_id != "" ? 1 : 0
  zone_id = var.hosted_zone_id
  name    = "api.brevio.ai"
  type    = "A"

  weighted_routing_policy {
    weight = 20
  }

  set_identifier = "secondary-eu-west-1"

  alias {
    name                   = var.secondary_lb_dns_name
    zone_id                = var.secondary_lb_zone_id
    evaluate_target_health = true
  }
}

# Geolocation routing: EU workspaces → eu-west-1.
resource "aws_route53_record" "brevio_api_eu" {
  count   = var.hosted_zone_id != "" ? 1 : 0
  zone_id = var.hosted_zone_id
  name    = "api.brevio.ai"
  type    = "A"

  geolocation_routing_policy {
    continent = "EU"
  }

  set_identifier = "eu-geolocation"

  alias {
    name                   = var.secondary_lb_dns_name
    zone_id                = var.secondary_lb_zone_id
    evaluate_target_health = true
  }
}
