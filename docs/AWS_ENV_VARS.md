# AWS ECS Environment Variables (Blueprint Alignment)

This file mirrors the required env vars for the AWS migration. Use it to configure ECS task definitions and Secrets Manager.

## Gateway
- WA_VERIFY_TOKEN=
- WA_APP_SECRET=
- WA_ACCESS_TOKEN=
- WA_PHONE_NUMBER_ID=
- WA_BUSINESS_ACCOUNT_ID=
- APPLE_BUSINESS_CHAT_ID=
- CLERK_SECRET_KEY=

## Brain
- OPENAI_API_KEY=
- OPENAI_ORG_ID=

## Hands
- GOOGLE_CLIENT_ID=
- GOOGLE_CLIENT_SECRET=
- MICROSOFT_CLIENT_ID=
- MICROSOFT_CLIENT_SECRET=
- TAVILY_API_KEY=

## Data
- DATABASE_URL=postgresql+asyncpg://...
- REDIS_URL=redis://...
- VECTOR_DB_BACKEND=pgvector
- PGVECTOR_DSN=postgresql+asyncpg://... (optional; defaults to DATABASE_URL)

## Email
- EMAIL_PROVIDER=ses
- SES_REGION=us-east-1
- SES_CONFIGURATION_SET= (optional)

## Infrastructure
- AWS_REGION=us-east-1
- AWS_SECRETS_PREFIX=executive-os/prod/
- S3_BUCKET=executive-os-attachments
- AXIOM_API_TOKEN=
- AXIOM_DATASET=executive-os-prod

## Feature Flags
- FEATURE_TIER_3_ENABLED=true
- FEATURE_PROACTIVE_ENABLED=true
- FEATURE_IMESSAGE_ENABLED=false
