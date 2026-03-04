# brevio-auth

OAuth and auth configuration service for Brevio.

## Endpoints

- `GET /health`
- `GET /health/deep`
- `GET /api/v1/providers`
- `GET /api/v1/providers/{service}`
- `POST /api/v1/oauth/{service}/authorize`
- `POST /api/v1/oauth/{service}/exchange`
- `POST /api/v1/oauth/{service}/refresh`
- `GET /callback/{service}`

## Behavior

- Loads OAuth/API-key/no-auth provider registry from `config/auth-service-map.yaml`.
- Enforces OAuth PKCE authorization state with TTL cleanup.
- Emits structured JSON logs with `trace_id`, `span_id`, and `correlation_id`.
- Supports graceful shutdown with configurable drain timeout.

## Configuration

- `PORT` (default `8080`)
- `NODE_ENV` (default `development`)
- `SERVICE_VERSION` (default `0.2.0`)
- `BREVIO_AUTH_MAP_PATH` (optional explicit path to `auth-service-map.yaml`)
- `BREVIO_AUTH_STATE_TTL_MS` (default `600000`)
- `BREVIO_AUTH_SHUTDOWN_TIMEOUT_MS` (default `30000`)
- `OAUTH_CLIENT_ID_<SERVICE>` (optional per-service local override)
