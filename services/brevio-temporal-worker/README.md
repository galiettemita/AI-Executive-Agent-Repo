# brevio-temporal-worker

Temporal workflow runtime service for Brevio workflow orchestration.

## Endpoints

- `GET /health`
- `GET /health/deep`
- `GET /api/v1/temporal-worker/workflows`
- `GET /v1/temporal-worker/workflows`
- `GET /api/v1/temporal-worker/runs`
- `GET /v1/temporal-worker/runs`
- `GET /api/v1/temporal-worker/runs/:run_id`
- `GET /v1/temporal-worker/runs/:run_id`
- `GET /api/v1/temporal-worker/runs/:run_id/tasks`
- `GET /v1/temporal-worker/runs/:run_id/tasks`
- `GET /api/v1/temporal-worker/runs/:run_id/steps`
- `GET /v1/temporal-worker/runs/:run_id/steps`
- `POST /api/v1/temporal-worker/workflows/message-processing`
- `POST /v1/temporal-worker/workflows/message-processing`
- `POST /api/v1/temporal-worker/workflows/daily-rhythm`
- `POST /v1/temporal-worker/workflows/daily-rhythm`
- `POST /api/v1/temporal-worker/runs/:run_id/resume`
- `POST /v1/temporal-worker/runs/:run_id/resume`
- `POST /api/v1/temporal-worker/runs/:run_id/steps/:step_id/transition`
- `POST /v1/temporal-worker/runs/:run_id/steps/:step_id/transition`

## Runtime behavior

- Simulates deterministic `MessageProcessingWorkflow` states:
  - `RECEIVED -> CLASSIFYING -> DECOMPOSING -> EXECUTING -> AGGREGATING -> FORMATTING -> DELIVERING -> COMPLETED`
  - terminal fallback states: `FAILED` and `DEAD_LETTER`
- Simulates `DailyRhythmWorkflow` states: `INIT -> COMPOSING -> DELIVERING -> COMPLETED`.
- Uses deterministic jitter helper (`fnv1a`) for retry/jitter metadata.
- Persists run, task, and step snapshots to a local JSON state file for restart-safe debug/status inspection.
- Supports paused runs (`pause_after_state`) plus explicit step transition and resume endpoints.
- Emits structured JSON logs with correlation IDs.
- Handles graceful shutdown with configurable timeout.

## Configuration

- `PORT` (default `8087`)
- `BREVIO_TEMPORAL_WORKER_SHUTDOWN_TIMEOUT_MS` (default `30000`)
- `BREVIO_TEMPORAL_WORKER_MAX_BODY_BYTES` (default `262144`)
- `BREVIO_TEMPORAL_WORKER_STATE_FILE` (default `./.runtime/temporal-worker-state.json`)
