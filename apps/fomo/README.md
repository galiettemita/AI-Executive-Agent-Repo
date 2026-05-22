# @brevio/fomo

The FOMO modular monolith — single deployable app for the Brevio v0.1 minimal MCP OS kernel and the FOMO trust workflow.

Per [FOMO_PLAN.md §12](../../FOMO_PLAN.md), `apps/fomo` is the only application workspace. There are no microservices.

## Phase 2A scope (this commit)

Phase 2A is the scaffold + the migration of proven primitives from the old `services/brevio-gateway`. No active product behavior beyond `GET /health`.

Active surface:

- `GET /health` — liveness, returns service metadata

Proven primitives present but not yet wired to any user-facing route:

- `src/core/safe-logger.ts` — PII-aware redacting logger
- `src/core/audit.ts` — append-only audit log with in-memory ring buffer for dev
- `src/security/token-crypto.ts` — AES-256-GCM at-rest token encryption
- `src/security/session.ts` — signed-session HMAC for founder/admin auth
- `src/security/session-middleware.ts` — request authentication
- `src/security/oauth/state.ts` — OAuth state HMAC + PKCE
- `src/security/oauth/exchange.ts` — code exchange / refresh / revoke
- `src/security/oauth/token-store.ts` — encrypted token persistence
- `src/security/oauth/providers/index.ts` — provider registry (Google active in v0.1)

## Out of scope until Phase 2B/2C

Per [FOMO_PLAN.md §9](../../FOMO_PLAN.md), the rest of the MCP OS kernel will land in later phases:

- Tool Registry
- Permission Manager / Policy Gate
- Egress Policy
- Alert State Machine
- Feedback Events / Memory Signals
- Model Router
- Kill Switches
- Gmail / SendBlue / Slack adapters

Real or absent. Never half-wired.

## Environment

Phase 2A reads only the variables consumed by the primitives. Dev mode (`BREVIO_DEV_MODE=true`) relaxes secret requirements; production must set them.

| Variable | Purpose | Production required? |
|---|---|---|
| `PORT` | HTTP listen port (default `8080`) | no |
| `SERVICE_VERSION` | reported on `/health` | no |
| `NODE_ENV` | reported on `/health` | no |
| `BREVIO_DEV_MODE` | `true` to use per-process random keys + header-based auth fallback | no |
| `BREVIO_TOKEN_KEK` | 32-byte KEK for token encryption (base64 or `hex:`-prefixed) | yes |
| `BREVIO_SESSION_SIGNING_KEY` | session token HMAC key (≥32 bytes) | yes |
| `BREVIO_OAUTH_STATE_KEY` | OAuth state HMAC key (≥32 bytes) | yes |
| `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `BREVIO_OAUTH_REDIRECT_URI_GOOGLE` | only used once OAuth routes go live in Phase 2B | no (Phase 2A) |

## Scripts

- `pnpm build` — `tsc -p tsconfig.json`
- `pnpm test` — `node --experimental-strip-types --loader ./test-loader.mjs --test src/**/*.test.ts` (Node 22+)
- `pnpm dev` — runs the compiled `dist/index.js`
- `pnpm lint` — ESLint on `src`
