# Environments

We run three environments: dev, staging, production.

## dev
- Local development
- May use SQLite and in-memory services

## staging
- Mirrors production dependencies
- Used for pre-release validation

## production
- User-facing environment
- Strict validation of critical settings

## Required per environment
- ENV: dev | staging | production
- DATABASE_URL: PostgreSQL in staging/production
- REDIS_URL: required in staging/production
- OPENAI_API_KEY, JWT_SECRET, PII_ENCRYPTION_KEY
