# API Documentation Standards

## Scope
These standards apply to all public and internal HTTP endpoints.

## Canonical Schema
- OpenAPI is the source of truth (`openapi.json`).
- All new endpoints must include schema updates and examples.

## Versioning
- Breaking changes require a new major API version.
- Use semantic versioning in `APP_VERSION`.
- Deprecations must be announced with a timeline.

## Authentication
- Production endpoints require either:
  - `Authorization: Bearer <JWT>` (HS256 signed with `JWT_SECRET`), or
  - `X-User-ID`, `X-User-Timestamp`, `X-User-Signature` (HMAC with `STATE_SIGNING_SECRET`).
- Webhooks must verify provider signatures when enabled.

## Error Format
All errors return JSON with:
```
{
  "error": "string",
  "message": "human readable message"
}
```
Use HTTP status codes appropriately (400, 401, 403, 404, 429, 500).

## Pagination
- Use `limit` and `cursor` (or `offset`) consistently.
- Default limit: 50 unless specified.

## Idempotency
- For create/charge endpoints, support idempotency keys.
- Persist idempotency state for at least 24 hours.

## Rate Limits
- Default per-user rate limit: 10/minute.
- Tighten for auth endpoints; loosen for webhooks.
- Return 429 with a `retry_after` value.

## Security Headers
- Enforce CORS allowlist.
- Return no sensitive data in error messages.
- Do not log secrets or full tokens.

## Observability
- All endpoints must emit request IDs and structured logs.
- Audit log required for sensitive actions.
