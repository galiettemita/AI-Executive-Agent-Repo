# brevio-brain

Decision and orchestration service for Brevio.

## Endpoints

- `GET /health`
- `GET /health/deep`
- `POST /api/v1/brain/classify`
- `POST /api/v1/brain/disambiguate`
- `POST /api/v1/brain/decompose`
- `POST /api/v1/brain/aggregate`
- `POST /api/v1/brain/process`

## Runtime behavior

- Performs deterministic-first intent classification with explicit clarification and blocked-skill handling.
- Applies 11 disambiguation groups from `config/skill-disambiguation.yaml`.
- Builds bounded task DAG outputs (max 10 tasks) with validation and action-level reasoning.
- Aggregates only real skill results into channel-safe response payloads; `/process` no longer fabricates synthetic success.
- Exposes a planner/verifier pipeline that combines classify → decompose → disambiguate → plan → verify and optionally aggregate when real skill results are present.
- Supports deterministic planning by default with optional OpenAI Responses model augmentation.
- Emits structured JSON logs with correlation IDs.
- Handles graceful shutdown with configurable timeout.

## Configuration

- `PORT` (default `8081`)
- `BREVIO_DISAMBIGUATION_CONFIG_PATH` (optional path override)
- `BREVIO_BRAIN_SHUTDOWN_TIMEOUT_MS` (default `30000`)
- `BREVIO_BRAIN_PLANNER_PROVIDER` (`deterministic` or `openai_responses`)
- `BREVIO_BRAIN_PLANNER_MODEL` (default `gpt-5.2`)
- `BREVIO_BRAIN_PLANNER_FALLBACK_MODEL` (default `gpt-5-mini`)
- `BREVIO_BRAIN_PLANNER_TIMEOUT_MS` (default `30000`)
