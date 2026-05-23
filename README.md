# Brevio FOMO — backend

Backend for **FOMO v0.1**, the first user-facing wedge of Brevio: an iMessage assistant that watches your Gmail and texts you only when there is something you'd be sad to miss.

This repository is mid-cleanup. The Phase 1 salvage left a minimal kernel that compiles green; Phase 2 onwards rebuilds the active code. The product direction is in [FOMO_DESIGN.md](FOMO_DESIGN.md); the implementation path is in [FOMO_PLAN.md](FOMO_PLAN.md); the salvage receipts are in [SALVAGE_MAP.md](SALVAGE_MAP.md) and [SALVAGE_DECISIONS.md](SALVAGE_DECISIONS.md); the long-term-assistant institutional memory for archived modules is in [docs/future-architecture-notes.md](docs/future-architecture-notes.md).

## Layout

```
apps/fomo/                modular monolith — the v0.1 deployable
  src/index.ts            /health server (only active HTTP surface in Phase 2)
  src/core/               tool registry, kill switches, permission gate,
                          egress policy, state machine, model router,
                          cost tracking, audit log, safe logger,
                          tool invocations, alert state transitions
  src/memory/             feedback events, memory signals
  src/security/           token crypto, session HMAC, OAuth (Google only in v0.1)
  src/db/                 Drizzle schema + migrations + Postgres-backed stores
  src/eval/               eval harness for model bake-offs (no real fixtures yet)
packages/shared/          shared types, schemas, errors, utilities
docs/                     future-architecture-notes (Phase 1 archive ledger)
drizzle.config.ts         drizzle-kit config (schema at apps/fomo/src/db/schema.ts)
```

## Develop

```bash
pnpm install
pnpm build
pnpm test
pnpm lint
```

The `apps/fomo` workspace runs its own `tsc` build into `dist/`. Tests run via `node --experimental-strip-types`.

## Status

Phase 1 ✅ — salvage cleanup, build green, tests green.
Phase 2A ✅ — apps/fomo shell + migrated proven primitives.
Phase 2B ✅ — tool registry, kill switches, permission gate (+ 2B.1 explicit executor_status).
Phase 2C ✅ — egress policy, state machine, feedback + memory substrate (+ 2C.1 honest declared semantics).
Phase 2D ✅ — model router, cost tracking, eval harness substrate (mock backend only).
Phase 2E ✅ — Drizzle/Neon persistence skeleton (9 substrate tables + 7 Postgres-backed stores).
Phase 2F — Kernel Integration Gate (proves the substrate is complete before Phase 3).
Phase 3 — FOMO workflow (Gmail poll, ranker, Slack review, SendBlue alert, reply parser).
Phase 4 — founder demo.

See [FOMO_PLAN.md §18](FOMO_PLAN.md) for the full phase plan.
