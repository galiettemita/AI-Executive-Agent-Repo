# Brevio FOMO — backend

Backend for **FOMO v0.1**, the first user-facing wedge of Brevio: an iMessage assistant that watches your Gmail and texts you only when there is something you'd be sad to miss.

This repository is mid-cleanup. The Phase 1 salvage left a minimal kernel that compiles green; Phase 2 onwards rebuilds the active code. The product direction is in [FOMO_DESIGN.md](FOMO_DESIGN.md); the implementation path is in [FOMO_PLAN.md](FOMO_PLAN.md); the salvage receipts are in [SALVAGE_MAP.md](SALVAGE_MAP.md) and [SALVAGE_DECISIONS.md](SALVAGE_DECISIONS.md); the long-term-assistant institutional memory for archived modules is in [docs/future-architecture-notes.md](docs/future-architecture-notes.md).

## Layout

```
packages/shared/          shared types, schemas, errors, utilities
services/brevio-brain/    /health server today; ranking + reply parser in Phase 3
services/brevio-gateway/  /health server + OAuth/audit/crypto primitives; routes wired in Phase 2-3
migrations/               only 012_consent_audit_oauth survives Phase 1
docs/                     future-architecture-notes (Phase 1 archive ledger)
```

## Develop

```bash
pnpm install
pnpm build
pnpm test
```

Each service runs its own `tsc` build into `dist/`. Tests run via `node --experimental-strip-types`.

## Status

Phase 1 ✅ — salvage cleanup, build green, tests green.
Phase 2 — minimal MCP OS kernel (tool registry, OAuth manager, permission gate, audit log, etc.).
Phase 3 — FOMO workflow (Gmail poll, ranker, Slack review, SendBlue alert, reply parser).
Phase 4 — founder demo.

See [FOMO_PLAN.md §18](FOMO_PLAN.md) for the full phase plan.
