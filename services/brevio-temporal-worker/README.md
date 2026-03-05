# brevio-temporal-worker

Temporal workflow runtime service for Brevio workflow orchestration.

## Endpoints

- `GET /health`
- `GET /health/deep`
- `GET /api/v1/temporal-worker/workflows`
- `GET /v1/temporal-worker/workflows`
- `GET /api/v1/temporal-worker/runs/:run_id`
- `GET /v1/temporal-worker/runs/:run_id`
- `POST /api/v1/temporal-worker/workflows/message-processing`
- `POST /v1/temporal-worker/workflows/message-processing`
- `POST /api/v1/temporal-worker/workflows/daily-rhythm`
- `POST /v1/temporal-worker/workflows/daily-rhythm`

## Runtime behavior

- Simulates deterministic `MessageProcessingWorkflow` states:
  - `RECEIVED -> CLASSIFYING -> DECOMPOSING -> EXECUTING -> AGGREGATING -> FORMATTING -> DELIVERING -> COMPLETED`
  - terminal fallback states: `FAILED` and `DEAD_LETTER`
- Simulates `DailyRhythmWorkflow` states: `INIT -> COMPOSING -> DELIVERING -> COMPLETED`.
- Uses deterministic jitter helper (`fnv1a`) for retry/jitter metadata.
- Persists in-memory run snapshots for debug/status inspection.
- Emits structured JSON logs with correlation IDs.
- Handles graceful shutdown with configurable timeout.

## Configuration

- `PORT` (default `8087`)
- `BREVIO_TEMPORAL_WORKER_SHUTDOWN_TIMEOUT_MS` (default `30000`)
- `BREVIO_TEMPORAL_WORKER_MAX_BODY_BYTES` (default `262144`)
