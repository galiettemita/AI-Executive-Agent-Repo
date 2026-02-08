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
- `POST /admin/email/connect` connect iCloud or Yahoo (app-specific password)
- `GET /admin/email/status` check iCloud/Yahoo status
- `POST /admin/email/disconnect` disconnect iCloud/Yahoo

## Calendar
- `GET /calendar/intelligence/meeting-prep` briefing
- `POST /calendar/intelligence/followup` follow-up email/tasks

## Files & Photos
- `POST /files/upload`
- `GET /files/search?semantic=true`
- `POST /photos/upload`
- `GET /photos/search?semantic=true`

## Wardrobe
- `POST /wardrobe/items` create wardrobe item
- `GET /wardrobe/items` list wardrobe items
- `GET /wardrobe/items/{item_id}` get wardrobe item
- `PATCH /wardrobe/items/{item_id}` update wardrobe item
- `DELETE /wardrobe/items/{item_id}` delete wardrobe item
- `GET /wardrobe/items/{item_id}/photos` list item photos
- `POST /wardrobe/items/{item_id}/photos` attach existing photos
- `POST /wardrobe/items/{item_id}/photos/upload` upload and attach a photo
- `DELETE /wardrobe/items/{item_id}/photos/{photo_id}` detach a photo
- `POST /wardrobe/items/{item_id}/wear` log wear event
- `GET /wardrobe/items/{item_id}/wear` list wear events
- `GET /wardrobe/stats` wear frequency stats
- `GET /wardrobe/rotation` rotation recommendations (optional notify)
- `GET /wardrobe/suggestions` outfit suggestions (weather + calendar)
- `GET /wardrobe/recommendations` shopping recommendations

## Analytics
- `GET /analytics/events`
- `GET /analytics/summary`

## Beta Access
- `POST /admin/beta/testers` add or update a beta tester
- `GET /admin/beta/testers` list beta testers
- `DELETE /admin/beta/testers/{tester_id}` remove beta tester
- `POST /admin/beta/testers/bulk` bulk add/update testers
- `GET /admin/beta/summary` beta allowlist summary

## Billing
- `POST /billing/checkout`
- `POST /webhooks/stripe`

## Monitoring
- `POST /monitoring/trigger/price-check`
- `POST /monitoring/trigger/send-notifications`
- `POST /monitoring/trigger/email-monitoring`
