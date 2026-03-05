# brevio-edge-agent

macOS local edge runtime for `local_mac` skills.

## Behavior

- Maintains a persistent WebSocket connection to `brevio-edge-relay`.
- Sends `register` and periodic `heartbeat` messages.
- Executes incoming `execute_skill` requests locally and returns `skill_result` payloads.
- Queues outbound results while disconnected and retries after reconnect.
- Exposes health endpoints:
  - `GET /health`
  - `GET /health/deep`

## Key Environment Variables

- `EDGE_RELAY_URL` (default `ws://127.0.0.1:8086/ws/edge`)
- `EDGE_USER_ID` (default `local-user`)
- `EDGE_DEVICE_ID` (default hostname)
- `EDGE_DEVICE_NAME` (default hostname)
- `EDGE_CLIENT_CERT_FINGERPRINT` (default `dev-client-cert-fingerprint`)
- `EDGE_SUPPORTED_SKILLS` (comma-separated)
- `EDGE_HEARTBEAT_MS` (default `15000`)
- `EDGE_MAX_QUEUE_AGE_MS` (default `14400000` / 4 hours)
- `EDGE_HEALTH_PORT` (default `18090`)
