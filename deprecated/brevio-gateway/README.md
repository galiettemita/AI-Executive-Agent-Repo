# brevio-gateway

Webhook ingress and normalization service for Brevio channels.

## Endpoints

- `GET /health`
- `GET /health/deep`
- `GET /webhooks/whatsapp` (verification challenge)
- `POST /webhooks/whatsapp`
- `POST /webhooks/imessage`
- `POST /webhooks/temporal`
- `POST /api/v1/gateway/format`

Compatibility aliases are also exposed for:

- `/api/v1/webhooks/*`
- `/v1/gateway/webhook/*`

## Runtime behavior

- Verifies WhatsApp webhook signatures via `X-Hub-Signature-256` (`HMAC-SHA256`).
- Verifies iMessage and Temporal callbacks with API key headers (`X-API-Key`).
- Deduplicates webhook events by `channel_message_id`/`message_id` for 24h TTL.
- Applies per-user token-bucket limits:
  - `free`: 30 messages/hour
  - `pro`: 120 messages/hour
  - `enterprise`: unlimited
- Enforces minute guardrail (`60` messages/minute by default).
- Normalizes inbound payloads into canonical `MessageEnvelope` shape with session rotation after 4h inactivity.
- Supports channel-aware outbound text formatting via `/api/v1/gateway/format`.
- Emits structured JSON logs with `trace_id`, `span_id`, `request_id`, and optional `user_id`.
- Handles graceful shutdown on `SIGTERM`/`SIGINT`.

## Required environment variables (production)

- `WHATSAPP_WEBHOOK_SECRET`
- `WHATSAPP_VERIFY_TOKEN`
- `IMESSAGE_API_KEY`
- `TEMPORAL_WEBHOOK_API_KEY`

## Optional tuning variables

- `BREVIO_GATEWAY_IDEMPOTENCY_TTL_MS`
- `BREVIO_GATEWAY_SESSION_IDLE_MS`
- `BREVIO_GATEWAY_RATE_LIMIT_PER_MINUTE`
- `BREVIO_GATEWAY_RATE_LIMIT_FREE_PER_HOUR`
- `BREVIO_GATEWAY_RATE_LIMIT_PRO_PER_HOUR`
- `BREVIO_GATEWAY_SHUTDOWN_TIMEOUT_MS`
