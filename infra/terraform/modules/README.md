# Infra Terraform Modules

Contract-oriented module definitions for the OpenClaw target layout.

## Module Map

| Module | AWS Service | Purpose |
| --- | --- | --- |
| `eks` | EKS | Core Kubernetes compute plane |
| `rds` | RDS PostgreSQL | Primary transactional database |
| `elasticache` | ElastiCache Redis | Cache and rate-limit backend |
| `sqs-sns` | SQS + SNS | Async queues, fanout, and DLQ routing |
| `s3` | S3 | Attachments, archives, backups, and media |
| `secrets` | Secrets Manager + KMS | Secret lifecycle and envelope encryption |
| `cloudfront` | CloudFront | CDN and webhook edge acceleration |
| `route53` | Route 53 | DNS routing and health-check failover |
| `monitoring` | CloudWatch + Grafana | Metrics/log/trace aggregation |
| `waf` | AWS WAF | L7 protections and rate limiting |

All modules expose a `module_contract` output used by environment compositions in
`infra/terraform/environments/*`.
