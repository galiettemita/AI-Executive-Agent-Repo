# Secrets Management

## Principles
- Never commit secrets to git
- Rotate secrets every 90 days
- Use environment variables or managed secrets
- Separate secrets per environment (dev/staging/prod)
- Use scoped tokens with minimal permissions

## Recommended Providers
- AWS Secrets Manager
- GCP Secret Manager
- Render encrypted environment variables

## Access
- Least privilege
- Audit access to secrets
- MFA required for all secret managers
