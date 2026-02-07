# Security Threat Model (High-Level)

## Assets
- User identity data
- Financial data and payment tokens
- Voice recordings and transcripts
- Private communications (email, messages)

## Threats
- Account takeover via weak auth
- Webhook spoofing and replay
- PII leakage via logs
- Prompt injection and tool abuse

## Controls
- JWT auth + signed headers
- Webhook signature verification
- PII encryption at rest
- Rate limiting and circuit breakers
- Audit logs for sensitive actions
