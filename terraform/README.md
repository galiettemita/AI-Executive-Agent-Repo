# Terraform — Moved

All Terraform configuration is now in `infra/terraform/`.

This directory is a stale partial copy missing 5 modules (cloudfront, monitoring,
route53, sqs-sns, waf) and the DR environment. It must not be used for deployment.

See `infra/terraform/` for the authoritative infrastructure definitions.
