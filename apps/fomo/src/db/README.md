# `apps/fomo/src/db` — persistence substrate

Phase 2E ships the Drizzle ORM + Neon Postgres skeleton. This directory is
the **single source of truth** for the v0.1 schema; the legacy root
`migrations/012_*.sql` files are removed in this phase.

## Layout

| Path | Purpose |
|---|---|
| `schema.ts` | Drizzle schema for the 9 substrate tables |
| `client.ts` | Env-driven Drizzle client factory; fail-closed in production |
| `store-factory.ts` | Picks in-memory vs Postgres-backed stores per env |
| `migrations/` | Drizzle-generated SQL migrations (single source of truth) |
| `stores/*-postgres.ts` | Postgres-backed implementations of the in-memory store interfaces |

## Tables (Phase 2E)

The 9 substrate tables mirror the in-memory stores from Phases 2A / 2C / 2D
plus `tool_invocations` per [FOMO_PLAN §9.10](../../../../FOMO_PLAN.md):

- `users`
- `oauth_tokens`
- `consent`
- `audit_log`
- `feedback_events`
- `memory_signals`
- `alert_state_transitions`
- `cost_records`
- `tool_invocations`

**Deferred to Phase 3** (no caller yet, so no table): `alerts`,
`message_events`, `rank_results`, `gmail_cursors`, `replies`,
`sender_importance`, `suppressions`, `user_preferences`. These land with
their callers per "Real or absent. Never half-wired."

## Selection rules

The store factory at [`store-factory.ts`](store-factory.ts) chooses the
backend per environment:

| Env | Backend |
|---|---|
| `DATABASE_URL` set | Postgres (via `pg.Pool` + Drizzle) |
| `DATABASE_URL` missing + `NODE_ENV !== 'production'` | In-memory |
| `DATABASE_URL` missing + `NODE_ENV === 'production'` + no `BREVIO_DEV_MODE` | **throws** |
| `DATABASE_URL` missing + `BREVIO_DEV_MODE=true` | In-memory (dev escape hatch) |

The production fail-closed is asserted in
[`client.test.ts`](client.test.ts) and
[`store-factory.test.ts`](store-factory.test.ts).

## Migrations

Drizzle-kit generates migrations from `schema.ts`. Config lives at
[`drizzle.config.ts`](../../../../drizzle.config.ts) (repo root).

```bash
# Generate the next migration after editing schema.ts
pnpm exec drizzle-kit generate --name <descriptive_name>

# Apply pending migrations to the database pointed at by DATABASE_URL
pnpm exec drizzle-kit migrate

# Inspect the schema vs the live DB
pnpm exec drizzle-kit check
```

The initial migration is `migrations/0000_init.sql` (9 tables, all indexes,
no foreign keys — see schema.ts header for the FK rationale).

## Tests

Default `pnpm test` runs against in-memory stores only — no live DB
required. [`store-factory.test.ts`](store-factory.test.ts) verifies the
Postgres construction path returns Postgres instances (no actual query
is issued during construction; `pg.Pool` is lazy).

**Gated Postgres tests** live in
[`stores/gated-pg.test.ts`](stores/gated-pg.test.ts) and run end-to-end
against PGlite — PostgreSQL's C code compiled to WASM, same parser /
planner / executor as a server-based Postgres but running in-process.
Skipped by default; opt in via env:

```bash
BREVIO_RUN_PG_TESTS=true pnpm test
```

What gets verified end-to-end:

- The Drizzle migration `0000_init.sql` applies cleanly and creates all 9 tables
- `PostgresAuditStore` write + read + redact
- `PostgresFeedbackStore` write, countByKind, countBySender, redact
- `PostgresMemorySignalStore` upsert (ON CONFLICT replaces), null scope_key
  handling, delete, list scoping
- `PostgresCostStore` sumByModel + sumByPeriod aggregations
- `PostgresAlertStateTransitionStore` ordered transitions, currentState,
  state validation
- `PostgresToolInvocationStore` write/read, counts, unique invocation_id
  constraint, and the **privacy invariant** asserted twice: (a) metadata
  redacted on write, (b) the `tool_invocations` table has zero
  payload-shaped columns (`body_plain`, `body_html`, `reply_text`,
  `prompt`, `email_body`)
- `PostgresTokenStore` encrypted round-trip, upsert at (user, provider),
  markNeedsReauth

To run against a real server-based Postgres instead of PGlite (e.g. local
Docker, Neon test branch), the same `BREVIO_RUN_PG_TESTS=true` flag
works — extend `setup()` in `gated-pg.test.ts` to point at
`BREVIO_TEST_DATABASE_URL` instead of constructing a `new PGlite()`.

Local server-based Postgres setup:

1. `docker run --rm -e POSTGRES_PASSWORD=test -p 5432:5432 -d postgres:16`
2. `DATABASE_URL=postgres://postgres:test@localhost:5432/postgres pnpm exec drizzle-kit migrate`
3. `BREVIO_RUN_PG_TESTS=true BREVIO_TEST_DATABASE_URL=postgres://... pnpm test`
