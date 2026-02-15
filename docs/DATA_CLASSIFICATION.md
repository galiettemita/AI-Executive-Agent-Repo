# Data Classification

## Levels
- Restricted: credentials, tokens, encryption keys, PHI/PII
- Confidential: user content, conversations, email metadata, voice transcripts
- Internal: operational metrics, logs without PII
- Public: marketing site content

## Handling Rules
- Restricted: encrypt at rest, restrict access, audit all access, no third‑party sharing
- Confidential: encrypt at rest, minimize retention, audit access
- Internal: limit access to operators, retain per policy
- Public: no restrictions

## Examples
- OAuth tokens, phone verifications: Restricted
- Chat messages, drafts, alerts: Confidential
- Audit logs (masked): Internal
- Landing page copy: Public
