# Executive OS Infrastructure (SST)

This directory contains the AWS infrastructure for the Blueprint 2026 migration using SST + CDK.

## Prerequisites
- AWS account with admin access
- AWS CLI configured (`aws configure`)
- Node 18+

## Install
```
cd infra
npm install
```

## Deploy (staging)
```
cd infra
npx sst deploy --stage staging
```

## Deploy (prod)
```
cd infra
npx sst deploy --stage prod
```

If you want SST to manage the ALB HTTPS listener, set:
`ALB_CERTIFICATE_ARN=<acm-cert-arn-in-us-east-1>`

## Notes
- `NetworkStack` provisions the VPC.
- `DataStack` provisions RDS Postgres 16, Redis 7, and S3.
- `EcsStack` provisions the ECS cluster, ALB, and ECR repositories.
- ECS services (gateway/brain/hands/workers) are created with `desiredCount=1`.

## Secrets Setup (Required)
After deploy, update the Secrets Manager secret:
`executive-os/<stage>/app`

Set values for:
- `OPENAI_API_KEY`
- `WA_ACCESS_TOKEN`, `WA_VERIFY_TOKEN`, `WA_APP_SECRET`
- `CLERK_SECRET_KEY`
- `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`
- `MICROSOFT_CLIENT_ID`, `MICROSOFT_CLIENT_SECRET`
- `TAVILY_API_KEY`
- `STRIPE_SECRET_KEY`
- `SENTRY_DSN`
- `AXIOM_API_TOKEN`
- `DATABASE_URL` (Postgres DSN)
- `APP_BASE_URL` (public API base URL)
- `EMAIL_PROVIDER` (recommended: ses)
- `SES_REGION` (e.g., us-east-1)
- `SES_CONFIGURATION_SET` (optional)
- `FROM_EMAIL`
- `FROM_NAME`
- `OTEL_EXPORTER_OTLP_ENDPOINT` (or split traces/metrics keys below)
- `OTEL_EXPORTER_OTLP_HEADERS`
- `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` (optional)
- `OTEL_EXPORTER_OTLP_TRACES_HEADERS` (optional)
- `OTEL_EXPORTER_OTLP_METRICS_ENDPOINT` (optional)
- `OTEL_EXPORTER_OTLP_METRICS_HEADERS` (optional)
- `OTEL_METRICS_ENABLED` (`0` to disable metrics exporter when traces-only endpoint is used)
- `POSTHOG_API_KEY` (optional, to keep PostHog product analytics)
- `POSTHOG_HOST` (recommended: `https://app.posthog.com`)
- `ENABLE_SCHEDULER` (`0` to disable legacy background jobs until related tables are migrated)
