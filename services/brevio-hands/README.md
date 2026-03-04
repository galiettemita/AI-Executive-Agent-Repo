# brevio-hands

Execution-plane service for Brevio skill adapters.

## Endpoints

- `GET /health`
- `GET /health/deep`
- `GET /v1/hands/skills`
- `GET /api/v1/hands/skills`
- `GET /api/v1/hands/circuit-breakers`
- `POST /v1/hands/execute`
- `POST /api/v1/hands/execute`
- `POST /v1/hands/tool/execute`
- `POST /api/v1/hands/tool/execute`

## Runtime behavior

- Executes exactly one skill adapter per request.
- Enforces execution timeout (`BREVIO_HANDS_EXECUTION_TIMEOUT_MS`, default `60000`).
- Applies per-skill circuit breaker state (`CLOSED`/`HALF_OPEN`/`OPEN`) with defaults: threshold `5`, recovery timeout `60000ms`, half-open probes `3`.
- Returns normalized `SkillResult` payloads for success, timeout, and failure outcomes.
- Emits structured JSON logs with correlation IDs (`trace_id`, `span_id`, `request_id`, `user_id`).
- Handles graceful shutdown with configurable timeout.

## Configuration

- `PORT` (default `8082`)
- `BREVIO_HANDS_SHUTDOWN_TIMEOUT_MS` (default `30000`)
- `BREVIO_HANDS_EXECUTION_TIMEOUT_MS` (default `60000`)
- `BREVIO_HANDS_MAX_BODY_BYTES` (default `2097152`)
- `BREVIO_HANDS_CB_FAILURE_THRESHOLD` (default `5`)
- `BREVIO_HANDS_CB_RECOVERY_TIMEOUT_MS` (default `60000`)
- `BREVIO_HANDS_CB_HALF_OPEN_MAX_CALLS` (default `3`)
