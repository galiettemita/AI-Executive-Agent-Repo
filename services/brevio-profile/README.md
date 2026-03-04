# brevio-profile

Profile and knowledge-file service for Brevio users.

## Endpoints

- `GET /health`
- `GET /health/deep`
- `GET /api/v1/profile/:user_id`
- `GET /v1/profile/:user_id`
- `PUT /api/v1/profile/:user_id/preferences`
- `PUT /v1/profile/:user_id/preferences`
- `GET /api/v1/profile/:user_id/knowledge/:file`
- `GET /v1/profile/:user_id/knowledge/:file`
- `PUT /api/v1/profile/:user_id/knowledge/:file`
- `PUT /v1/profile/:user_id/knowledge/:file`
- `POST /api/v1/profile/:user_id/hash/refresh`
- `POST /v1/profile/:user_id/hash/refresh`

## Runtime behavior

- Manages canonical knowledge files: `USER.md`, `SOUL.md`, `AGENTS.md`.
- Maintains `profile_hash` as SHA-256 of merged knowledge-file content.
- Supports profile preference updates (timezone, locale, preferences).
- Persists profile and knowledge data under local profile storage root.
- Emits structured JSON logs with correlation IDs.
- Handles graceful shutdown with configurable timeout.

## Configuration

- `PORT` (default `8084`)
- `BREVIO_PROFILE_DATA_DIR` (default `<repo>/data/profiles`)
- `BREVIO_PROFILE_MAX_KNOWLEDGE_BYTES` (default `524288`)
- `BREVIO_PROFILE_SHUTDOWN_TIMEOUT_MS` (default `30000`)
