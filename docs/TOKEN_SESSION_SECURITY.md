# Token and Session Security

## Principles
- Short‑lived tokens for access
- Rotation for long‑lived refresh tokens
- Explicit revocation on logout or compromise
- Least privilege scopes

## Current Controls
- Signed requests via `X-User-*` headers with 5‑minute replay window
- JWT verification for Bearer tokens (HS256)
- Redis TTLs for session and preference caches

## Planned Controls
- Issue short‑lived JWTs with refresh rotation for user sessions
- Maintain a token revocation list
- Rotate encryption keys per `KEY_ROTATION_RUNBOOK.md`
