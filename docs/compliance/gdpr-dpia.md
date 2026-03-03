# GDPR DPIA

## Scope
Assessment covers high-risk processing domains:
- Health integrations (Dexcom, Withings, HealthKit).
- Financial integrations (Plaid, YNAB, Monarch, Copilot).
- Communication content (email and messaging channels).

## Processing Purpose
- Execute user-requested assistant actions.
- Provide contextual recommendations.
- Maintain operational reliability and compliance records.

## Risk Assessment
- Unauthorized access to sensitive domains.
- Excessive retention of personal/sensitive data.
- Cross-tenant data leakage.
- Inference risk from LLM prompt context.

## Controls
- Data minimization at prompt construction.
- Encryption at rest/in transit and least-privilege access.
- Role/policy checks before skill execution.
- PII scrubbing and retention enforcement jobs.
- Explicit user consent and revocation for high-risk connectors.

## Residual Risk
Residual risk is reduced to medium/low with implemented controls and periodic control testing.

## Review Cadence
- DPIA review every 12 months.
- Immediate reassessment on major architecture change or new high-risk processor.
