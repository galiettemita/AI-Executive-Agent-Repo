# Executive AI Agent API Docs (Summary)

This is a high-level API reference. The canonical schema is the generated OpenAPI document in `openapi.json`.

## Authentication
- All endpoints expect `user_id` for now (WhatsApp phone number or internal user id).
- Webhooks are verified when `ENFORCE_WEBHOOK_SIGNATURES=1`.

## Core Endpoints
- `GET /health/ready` readiness probe
- `POST /chat` conversational entry point
- `POST /assist` structured task execution

## Messaging
- `POST /messages/send` queue outbound messages (WhatsApp/SMS)
- `POST /webhooks/whatsapp` inbound WhatsApp messages
- `POST /webhooks/sms/status` delivery receipts

## Email
- `GET /email/intelligence/summary` inbox summarization
- `POST /email/intelligence/reply/draft` create draft
- `POST /email/intelligence/reply/send` send draft
- `GET /email/intelligence/monitoring/configs` list monitoring configs
- `POST /email/intelligence/monitoring/configs` upsert monitoring config
- `POST /email/intelligence/monitoring/run` trigger monitoring
- `GET /email/intelligence/monitoring/alerts` list alerts
- `POST /email/intelligence/monitoring/test` create a test alert (non-prod unless enabled)

## Calendar
- `GET /calendar/intelligence/meeting-prep` briefing
- `POST /calendar/intelligence/followup` follow-up email/tasks

## Files & Photos
- `POST /files/upload`
- `GET /files/search?semantic=true`
- `POST /photos/upload`
- `GET /photos/search?semantic=true`

## Analytics
- `GET /analytics/events`
- `GET /analytics/summary`

## Auth
- `POST /auth/pair` exchange pairing code for JWT
- `POST /admin/pairing/code` create pairing code (admin)

## Billing
- `POST /billing/checkout`
- `POST /webhooks/stripe`

## Monitoring
- `POST /monitoring/trigger/price-check`
- `POST /monitoring/trigger/send-notifications`
- `POST /monitoring/trigger/email-monitoring`
