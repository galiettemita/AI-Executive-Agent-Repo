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

- Performs deterministic keyword-first intent classification with confidence output.
- Applies 11 disambiguation groups from `config/skill-disambiguation.yaml`.
- Builds bounded task DAG outputs (max 10 tasks) with cycle validation.
- Aggregates skill results into channel-safe response payloads.
- Exposes a single end-to-end process endpoint combining classify → disambiguate → decompose → aggregate.
- Emits structured JSON logs with correlation IDs.
- Handles graceful shutdown with configurable timeout.

## Configuration

- `PORT` (default `8081`)
- `BREVIO_DISAMBIGUATION_CONFIG_PATH` (optional path override)
- `BREVIO_BRAIN_SHUTDOWN_TIMEOUT_MS` (default `30000`)
