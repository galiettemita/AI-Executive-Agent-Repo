# brevio-scheduler

Cron and trigger orchestration service for Brevio.

## Endpoints

- `GET /health`
- `GET /health/deep`
- `GET /api/v1/scheduler/jobs`
- `GET /v1/scheduler/jobs`
- `POST /api/v1/scheduler/jobs`
- `POST /v1/scheduler/jobs`
- `POST /api/v1/scheduler/jobs/:job_id/run`
- `POST /v1/scheduler/jobs/:job_id/run`
- `DELETE /api/v1/scheduler/jobs/:job_id`
- `DELETE /v1/scheduler/jobs/:job_id`
- `POST /api/v1/scheduler/trigger`
- `POST /v1/scheduler/trigger`
- `GET /api/v1/scheduler/triggers`
- `GET /v1/scheduler/triggers`

## Runtime behavior

- Manages in-memory scheduled jobs keyed by `job_id`.
- Supports ad-hoc trigger queueing for skill execution requests.
- Tracks last/next run metadata per job.
- Emits structured JSON logs with correlation IDs.
- Handles graceful shutdown with configurable timeout.

## Configuration

- `PORT` (default `8085`)
- `BREVIO_SCHEDULER_SHUTDOWN_TIMEOUT_MS` (default `30000`)
- `BREVIO_SCHEDULER_MAX_BODY_BYTES` (default `262144`)
- `BREVIO_SCHEDULER_MAX_JOBS` (default `5000`)
