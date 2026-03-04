# brevio-metrics

Metrics aggregation service for Brevio observability.

## Endpoints

- `GET /health`
- `GET /health/deep`
- `GET /metrics` (Prometheus text format)
- `POST /api/v1/metrics/events`
- `POST /v1/metrics/events`
- `GET /api/v1/metrics/snapshot`
- `GET /v1/metrics/snapshot`

## Runtime behavior

- Exposes Prometheus metric families required by blueprint Section 10.
- Supports counter/gauge/histogram ingestion via event API.
- Maintains in-memory metric series for local/staging runtime.
- Returns JSON metric snapshots for debugging.
- Emits structured JSON logs with correlation IDs.
- Handles graceful shutdown with configurable timeout.

## Configuration

- `PORT` (default `9090`)
- `BREVIO_METRICS_SHUTDOWN_TIMEOUT_MS` (default `30000`)
- `BREVIO_METRICS_MAX_BODY_BYTES` (default `131072`)
