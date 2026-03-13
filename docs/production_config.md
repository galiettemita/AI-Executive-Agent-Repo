# Production Configuration

Required environment variables for the Brevio backend services in non-local
environments. Local/test environments may omit most of these — services degrade
gracefully with warnings.

## Temporal Worker (`cmd/temporal-worker`)

| Variable | Required | Description |
|---|---|---|
| `BREVIO_ENV` | Yes | Environment name: `local`, `test`, `staging`, `production` |
| `DATABASE_URL` | Yes (non-local) | PostgreSQL connection string for pgxpool |
| `REDIS_URL` | Yes (non-local) | Redis connection string |
| `TEMPORAL_HOST` | Yes (non-local) | Temporal server address (e.g., `temporal:7233`) |
| `TEMPORAL_NAMESPACE` | Yes (non-local) | Temporal namespace |
| `CONNECTORS_MASTER_KEY_B64` | Yes (non-local) | Base64-encoded master key for connector encryption. Zero key only in local/test. |
| `CONNECTORS_SEED_FILE` | No | Path to connectors YAML seed file. Default: `internal/connectors/seeds/connectors.yaml` |
| `OPENAI_API_KEY` | No | OpenAI API key for embedding provider. Optional if RAG embeddings not needed. |
| `GOOGLE_PLACES_API_KEY` | No | Google Places API key for phone verification. Optional. |

### HandsExecutor

Wired in-process via the hands service. Requires `CONNECTORS_MASTER_KEY_B64` in
non-local environments. Startup fails fast if this is missing.

### OutboxDispatcher

Uses `HTTPOutboxDispatcher` with 30s timeout. SSRF protections enforced:
- Only `http://` and `https://` schemes allowed
- Localhost, loopback (127.x), private (10.x, 172.16-31.x, 192.168.x),
  link-local, and metadata (169.254.169.254) IPs blocked
- Redirects re-validated against SSRF rules (max 3 hops)
- Response body capped at 64KB

## Executor (`cmd/executor`)

| Variable | Required | Description |
|---|---|---|
| `HMAC_KEY` | Yes (non-local) | HMAC signing key for request authentication. Fails fast in production. |

## Hands (`cmd/hands`)

| Variable | Required | Description |
|---|---|---|
| `BREVIO_ENV` | Yes | Environment name |
| `CONNECTORS_MASTER_KEY_B64` | Yes (non-local) | Master key for connector encryption |
| `SEED_FILE` | No | Path to connectors seed file. Default: `internal/connectors/seeds/connectors.yaml` |
| `HANDS_PORT` | No | Listen port. Default: `18090` |

Placeholder MCP URLs (`unconfigured.local`) are rejected in non-local environments. Set `MCP_BASE_URL` to override seed placeholders at load time.

## Canvas (`cmd/canvas`)

| Variable | Required | Description |
|---|---|---|
| `CANVAS_ALLOWED_ORIGINS` | Recommended | Comma-separated WebSocket origin allowlist. When empty, all origins accepted (local/test only). |

## Fail-fast behavior

Non-local environments enforce:
1. `CONNECTORS_MASTER_KEY_B64` must be set (temporal-worker, hands)
2. `HMAC_KEY` must be set (executor)
3. Placeholder MCP URLs rejected (hands)
4. All required env vars from `RequiredNonLocalEnv` validated at startup
