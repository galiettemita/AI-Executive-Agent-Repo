# Security Protocols

## Authentication
- JWT (HS256) for API calls, signed by `JWT_SECRET`.
- Signed header auth for internal calls: `X-User-ID`, `X-User-Timestamp`, `X-User-Signature`.
- Reject stale signatures beyond 5 minutes.
- Approval links use signed tokens with explicit expiration.

## Webhook Security
- Enforce signature verification for WhatsApp, Stripe, and other webhooks.
- Replay protection on inbound webhooks (idempotency check).

## Security Headers
- HSTS enabled in staging/production.
- CSP on HTML responses.
- X-Content-Type-Options, X-Frame-Options, Referrer-Policy, Permissions-Policy.

## Secrets Management
- Never commit secrets to source control.
- Store secrets in Render encrypted env vars or a managed secret store.
- Rotate secrets every 90 days or immediately after incident.

## Encryption
- TLS for all traffic in transit.
- Encrypt sensitive fields at rest (PII/PHI) using managed keys.
- Rotate keys; keep previous keys for decryption until re-encrypted.

## Access Control
- Least privilege for all service accounts.
- Admin actions logged in audit logs.
- MFA required for admin and infra accounts.

## HIPAA Controls (in scope)
- Access controls and audit trails for PHI access.
- Minimum necessary data access.
- Encryption in transit and at rest.
- Incident response and breach notification processes.

## Logging and Monitoring
- Structured logs with PII masking.
- Security alerts on auth failures, webhook failures, and rate limit spikes.

## Secure SDLC
- Code review required for all changes.
- Dependency and container scanning in CI.
