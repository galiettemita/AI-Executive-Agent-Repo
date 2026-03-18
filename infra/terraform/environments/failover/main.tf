# Failover environment: promotes eu-west-1 to primary.
# Apply only during failover: terraform apply -var="failover_active=true"

terraform {
  required_version = ">= 1.6.0"
}

variable "failover_active" {
  type    = bool
  default = false
}

variable "hosted_zone_id" {
  type    = string
  default = ""
}

variable "eu_lb_dns_name" {
  type    = string
  default = ""
}

variable "eu_lb_zone_id" {
  type    = string
  default = ""
}

# Route53 failover: 100% to eu-west-1.
resource "aws_route53_record" "brevio_api_failover" {
  count   = var.failover_active ? 1 : 0
  zone_id = var.hosted_zone_id
  name    = "api.brevio.ai"
  type    = "A"

  weighted_routing_policy {
    weight = 100
  }

  set_identifier = "failover-eu-west-1"

  alias {
    name                   = var.eu_lb_dns_name
    zone_id                = var.eu_lb_zone_id
    evaluate_target_health = true
  }
}

output "failover_status" {
  value = {
    active     = var.failover_active
    target     = "eu-west-1"
    dns_record = var.failover_active ? "api.brevio.ai → eu-west-1 (100%)" : "inactive"
  }
}
