# brevio-edge-relay

Cloud relay service for edge-agent WebSocket sessions.

## Behavior

- Accepts edge WebSocket connections at `/ws/edge`.
- Tracks active sessions by `user_id:device_id`.
- Accepts execution requests via `POST /v1/edge/execute` and dispatches to connected agents.
- Queues requests for offline agents (max queue age configurable, default 4 hours).
- Returns user-safe offline response messaging when device is unavailable.
- Exposes operational endpoints:
  - `GET /health`
  - `GET /health/deep`
  - `GET /v1/edge/sessions`

## Key Environment Variables

- `PORT` (default `8086`)
- `EDGE_RELAY_PATH` (default `/ws/edge`)
- `EDGE_MAX_QUEUE_AGE_MS` (default `14400000` / 4 hours)
- `EDGE_MAX_QUEUE_PER_DEVICE` (default `100`)
- `BREVIO_ENV` (default `local`)
