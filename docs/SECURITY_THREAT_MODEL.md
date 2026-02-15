# Security Threat Model (STRIDE + Abuse Cases)

## Scope & Assets
- User identity data (accounts, OAuth tokens, phone numbers)
- Financial data and payment tokens
- Private communications (WhatsApp, email, calendar)
- Voice recordings and transcripts
- Files/photos stored in object storage
- Execution engine actions (purchases, bookings, refunds)

## Trust Boundaries
- Public internet → API gateway (external clients/webhooks)
- Third‑party providers → webhook endpoints (WhatsApp, Stripe, Twilio)
- App → data stores (Postgres, Redis, object storage, vector DB)
- Operator access (admin tools, dashboards, logs)

## STRIDE Analysis
### Spoofing
- Forged webhook requests
- Stolen OAuth tokens / session replay
- Impersonation via weak verification
Controls: webhook signatures + replay protection, signed header auth, token TTLs, MFA for admins.

### Tampering
- Payload manipulation in webhooks or proposals
- Database writes via compromised credentials
Controls: request validation, audit logs, least privilege IAM, immutable logs for sensitive actions.

### Repudiation
- Users or operators deny actions
Controls: audit logs, proposal approval logs, execution logs, signed actions.

### Information Disclosure
- PII leakage in logs or error responses
- Over‑broad data returned by endpoints
Controls: PII masking, output encoding, strict schemas, access control checks.

### Denial of Service
- Request floods or expensive queries
- Abuse of discovery/search providers
Controls: rate limits, queueing, circuit breakers, timeouts.

### Elevation of Privilege
- Bypass user_id checks
- Admin endpoints exposed
Controls: auth middleware, signed headers, allowlists, strict RBAC for admin tools.

## Abuse Cases (AI‑Specific)
- Prompt injection to trigger unauthorized actions
- Indirect prompt injection via email/web content
- Tool misuse (e.g., executing purchases without approvals)
Controls: approvals required for execution, tool allowlists, scoped permissions.

## Mitigations Implemented
- JWT auth + signed headers
- Webhook signature verification + dedupe
- PII encryption at rest
- Rate limiting
- Audit logging on write actions
- Output encoding for HTML responses
- Security headers (HSTS, CSP, nosniff, etc.)
