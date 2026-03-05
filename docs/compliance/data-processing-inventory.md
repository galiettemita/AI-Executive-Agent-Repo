# Data Processing Inventory

## Data Domains
- Identity data: phone number, email, display name.
- Communication data: inbound/outbound message content and metadata.
- OAuth credential material: encrypted access/refresh tokens.
- Financial data: budget usage, transaction summaries (provider-scoped).
- Health data: metrics from integrated health providers.
- Operational telemetry: logs, metrics, traces, audit entries.

## Processing Matrix
| Data Type | Purpose | Retention | Encryption | Deletion Path |
|---|---|---|---|---|
| User profile fields | Account and personalization | Active account lifecycle | AES-256 at rest, TLS in transit | GDPR erasure workflow |
| Messages | Assistant context and history | 24 months hot + archive | AES-256 + TLS | TTL archive + deletion request |
| OAuth tokens | API access to user tools | Until revoked/expired | AES-256-GCM envelope | Token revocation + secure wipe |
| Skill execution logs | Reliability, billing, audit | 90 days hot + archive | AES-256 + TLS | Scheduled retention job |
| Billing usage | Cost controls and invoicing | 7 years where required | AES-256 + TLS | Accounting retention policy |
| Security audit events | Compliance evidence | 12+ months archive | AES-256 + integrity chain | Policy-driven retention expiry |

## Data Owners
- Engineering: system operation and data integrity.
- Security: audit access and policy enforcement.
- Compliance: retention/legal process oversight.
