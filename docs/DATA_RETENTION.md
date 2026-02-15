# Data Retention Policy

This policy is aligned to US-only launch with HIPAA in scope and enterprise customers.

## Principles
- Minimize retention: store only what is needed for product functionality.
- Separate raw content from metadata when possible.
- Default to metadata-only for external integrations.
- Honor user deletion requests and legal hold requirements.

## Data Classes and Retention
- Identity/auth (users, tokens, credentials): retain while account active; delete within 30 days of closure (unless legal hold).
- Profiles/consent/prefs/memory summaries: retain while account active; delete within 30 days of closure.
- Relationship profiles and interactions (metadata only): profiles retained while account active; interactions retained 24 months.
- Chat/WhatsApp messages: retain 180 days; memory summaries retained until user deletes account.
- Email metadata (subjects/senders/snippets, alerts, drafts): alerts 180 days, drafts 90 days after send.
- Calendar metadata (events, times): metadata only; retained 180 days.
- Files/photos: full bytes in object storage; metadata in Postgres. Retained until user deletes; hard delete within 30 days of account deletion.
- Voice: transcripts/summaries retained 30 days; recordings stored via URL and purged at 30 days (VOICE_RECORDING_RETENTION_DAYS).
- Notification queue: retained 30 days after sent.
- Inbound webhooks and deliveries: retained 30 days.
- Audit logs: retained 12 months.
- Usage events/analytics: retained 24 months.
- Watch offers: retained 90 days.
- Billing/transactions: retained 7 years (anonymize on user deletion when legally required).
- Smart home energy readings: retained 90 days unless user opts in for longer history.
- Fitness and nutrition logs (including steps): retained while account active; delete within 30 days of closure.
- Entertainment content and consumption events: retained while account active; delete within 30 days of closure.
- Events and ticket booking metadata: retained while account active; delete within 30 days of closure.
- Gift orders, retailer allowlist, and order events: retained 24 months or until user deletes; refunds retained 7 years when tied to transactions.
- Language learning goals and practice sessions: retained while account active; delete within 30 days of closure.
- Learning resources and schedules: retained while account active; delete within 30 days of closure.
- Vector embeddings: derived data only; delete on request or account deletion.

## Automated Enforcement
- A daily retention job runs at `DATA_RETENTION_SCHEDULE` and purges records beyond their retention window.
- Voice recordings/transcripts are purged by a separate daily job using `VOICE_RECORDING_RETENTION_DAYS`.
- File/photo bytes remain in object storage until user deletion; metadata remains in Postgres.

## Deletion and Requests
- User-initiated deletion triggers removal or anonymization across all tables and object storage.
- HIPAA: follow minimum necessary standard; PHI deleted or anonymized per request unless legally required to retain.
- GDPR/CCPA: data export and deletion workflows logged and tracked.
