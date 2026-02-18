# Privacy Policy (v1 Draft)

Legal review is required before launch. This document is an operationally-specific draft to support early production readiness.

## 1) Data We Collect
- Account and identity data: internal user IDs, linked channel identifiers (e.g., phone number), authentication/verification events (e.g., OTP verification attempts), and basic account settings (timezone, preferences).
- Message content and conversation context: messages you send to the assistant and responses we generate, including message metadata (timestamps, channel, message IDs).
- Connected integrations: OAuth tokens and/or API keys for connected services (stored encrypted where applicable), plus integration metadata (scopes, provider identifiers, connection status).
- Files and media (if you send them): URLs and metadata required to retrieve and process files (e.g., voice notes, images, documents). Where enabled, we may store derived artifacts like transcripts and extracted entities.
- Usage and diagnostics: rate-limiting counters, feature usage metrics, error logs, latency metrics, and audit logs for administrative actions.
- Billing data: subscription status and Stripe customer/subscription/invoice identifiers. We do not store full payment card numbers; payment processing is handled by Stripe.

## 2) How We Use Data
- Provide the assistant’s core functionality (message handling, tool execution, reminders/notifications where enabled).
- Maintain product reliability and safety (monitoring, abuse prevention, rate limiting, incident investigation).
- Improve quality (evaluations, bug fixing, and product analytics).
- Comply with legal obligations (security logging, required recordkeeping).

## 3) AI/LLM Processing
To generate responses and run certain features, we may send message content and relevant context to AI model providers. Providers may change over time, but currently includes:
- OpenAI (e.g., GPT models)
- Anthropic (e.g., Claude models)
- Google (e.g., Gemini models), if configured
We minimize what is sent and restrict access by policy and technical controls.

## 4) Data Sharing
We share data with vendors only as needed to provide the service:
- Messaging: Meta/WhatsApp (for WhatsApp delivery), Apple Messages for Business / iMessage providers (if enabled), Slack (if enabled).
- Payments: Stripe (billing, subscription management).
- SMS/voice: Twilio (OTP and voice features, if enabled).
- Hosting/infra: cloud hosting, storage, and monitoring providers used to operate the service.
We do not sell personal information.

## 5) Data Retention
Retention depends on the data type and account status. Examples:
- Chat history: retained for product functionality and support, subject to the retention windows configured in `DATA_RETENTION.md`.
- OAuth tokens/API keys: retained while an integration is connected; revoked/deleted when you disconnect or delete your account.
- Canceled accounts: operational retention defaults to `DATA_RETENTION_DAYS_CANCELED=90` days for some records to support billing disputes, fraud prevention, and support, then deleted or anonymized as applicable.
For additional detail, see `DATA_RETENTION.md`.

## 6) Your Rights and Choices
You can request:
- Access/export: a copy of your data.
- Deletion: deletion of your account and associated data (subject to required retention of certain financial or security records).
To exercise rights, contact support (see `SUPPORT.md`) or use in-product endpoints where available.

## 7) Security
We use a combination of safeguards such as encryption in transit and at rest (where applicable), access controls, audit logging, and key rotation procedures.

## 8) Children
The service is not intended for use by children under 13.

## 9) Changes to This Policy
We may update this policy. If changes are material, we will provide notice via the product or other reasonable means.
