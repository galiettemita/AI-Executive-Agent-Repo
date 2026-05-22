# Salvage Decisions — Phase 1 Cleanup Verdicts

**Date:** 2026-05-22
**Branch:** april-may-improvements
**Baseline (archive-commit-sha):** `a288f3ed` — *"baseline: bring openclaw-phase0 salvage candidates into the tree"*

This file is the receipt that ties [SALVAGE_MAP.md](SALVAGE_MAP.md) (the candidate list), [docs/future-architecture-notes.md](docs/future-architecture-notes.md) (institutional memory for archived concepts), and the Phase 1 cleanup commit (the actual deletion) together.

**Verdict meanings:**
- **KEEP** — stays in the active tree with v0.1-compatible code
- **REWRITE_MINIMAL** — file replaced with a minimal v0.1 skeleton; original archived at baseline SHA
- **ARCHIVE** — deleted from the active tree; concept preserved in [docs/future-architecture-notes.md](docs/future-architecture-notes.md), code recoverable via `git show a288f3ed:<path>`
- **KILL** — deleted; no archive note needed (premature, dead, or scratch)

---

## Surviving services

Phase 1 left only the two workspaces SALVAGE_MAP flagged as KEEP: `services/brevio-brain` and `services/brevio-gateway`. Plus `packages/shared`. Everything else is gone.

### `services/brevio-brain/src/` — gutted to a `/health` skeleton

When the 48-hour gate from SALVAGE_MAP §"48-hour decision gate" was applied, the brain salvage candidates failed it: `index.ts`, `policy.ts`, `verify.ts`, `gating.ts`, `normalize.ts`, `catalog.ts`, `types.ts`, and `config.ts` were all so deeply coupled to `PlannerProposal` / `ProcessRequest` / `NormalizedReasoningRequest` / `IntentClassificationInput` / `DisambiguationRules` (all archived per future-architecture-notes.md) that "salvage" would have been a rewrite. Per the gate's own escape hatch — *"If by end of day 2 you've marked more than 6 things KILL — Codex was right and you should restart from a fresh repo"* — the brain is being rebuilt fresh in Phase 2/3 against v0.1's actual types.

| File | Verdict | Notes |
|---|---|---|
| `index.ts` | REWRITE_MINIMAL | Was 669 lines of multi-route brain server. Now ~95 lines: `/health` only. Phase 2/3 wires the FOMO workflow. |
| `config.ts` | REWRITE_MINIMAL | Was 249 lines of planner/disambiguation config. Now ~26 lines: just port + service metadata. |
| `types.ts` | REWRITE_MINIMAL | Was 348 lines of planner/decompose/aggregate types. Now ~15 lines: `BrainConfig`, `RequestContext`, `ExternalModelEgress`. |
| `catalog.ts` | ARCHIVE | Skill catalog for 50+ legacy skills. Concept partially in §Intent Classification of future-architecture-notes. |
| `gating.ts` | ARCHIVE | Tightly coupled to `PlannerProposal`. Concept in §Approval Gating. |
| `normalize.ts` | ARCHIVE | Operated on `IntentClassificationInput`/`ProcessRequest`. No v0.1 caller after planner is gone. |
| `policy.ts` | ARCHIVE | Built `PlanPolicySummary` from planned actions. PII regex helpers live on in concept — useful for Phase 3 egress redaction. |
| `verify.ts` | ARCHIVE | Verifier operated entirely on `PlannerProposal`. Concept "should I send this SMS?" is reborn fresh in Phase 3. |
| `aggregate.ts`, `aggregate.test.ts` | ARCHIVE | Per future-architecture-notes §Result Aggregation. |
| `classify.ts`, `classify.test.ts` | ARCHIVE | Per future-architecture-notes §Intent Classification. |
| `decompose.ts`, `decompose.test.ts` | ARCHIVE | Per future-architecture-notes §Multi-step Task Decomposition. |
| `disambiguate.ts`, `disambiguate.test.ts` | ARCHIVE | Per future-architecture-notes §Tool Router. |
| `planner.ts`, `planner.test.ts` | ARCHIVE | Per future-architecture-notes §Agent Orchestration. |
| `workflow-runtime.ts`, `workflow-runtime.test.ts` | ARCHIVE | Per future-architecture-notes §Workflow Runtime. |
| `server.test.ts`, `config.test.ts`, `verify.test.ts`, `gating.test.ts` | KILL | Tests for archived/rewritten surface. |

### `services/brevio-gateway/src/` — OAuth + crypto + audit + safe-logger preserved

Per Codex's prediction in SALVAGE_MAP — *"importing only the OAuth + crypto + safe-logger files"* — these are what survive cleanly. The chat-UI-specific gateway code archives.

| File | Verdict | Notes |
|---|---|---|
| `index.ts` | REWRITE_MINIMAL | Was 827 lines wiring WhatsApp/iMessage/Temporal webhooks. Now ~120 lines: `/health` only. Phase 2/3 wires the new OAuth flow. |
| `config.ts` | REWRITE_MINIMAL | Was 39 lines of webhook secrets and rate-limit envs. Now ~22 lines: just port + service metadata. |
| `types.ts` | REWRITE_MINIMAL | Was 88 lines of WhatsApp/iMessage envelopes and rate-limit state. Now ~12 lines: `GatewayConfig`, `RequestContext`. |
| `audit.ts` (+ test) | KEEP | Audit log helper. v0.1 critical. |
| `auth.ts` (+ test) | KEEP | Signed-session HMAC. v0.1 will reuse for the founder/admin session. |
| `auth-middleware.ts` | KEEP | Express-style auth middleware. |
| `crypto.ts` (+ test) | KEEP | AES-256-GCM for OAuth tokens. **Non-negotiable.** |
| `safe-logger.ts` (+ test) | KEEP | PII-aware redacting logger. **Non-negotiable.** |
| `oauth-exchange.ts` (+ test) | KEEP | Pure token-exchange functions, no skill coupling. |
| `oauth-state.ts` (+ test) | KEEP | OAuth CSRF state + PKCE + nonce store. |
| `oauth-providers/` | KEEP | Google provider config. |
| `token-store.ts` (+ test) | KEEP | OAuth token storage. |
| `consent-routes.ts` | ARCHIVE | Was 328 lines of chat-UI consent flow + pending-message handlers. Phase 3 builds a minimal `gmail_read` consent toggle fresh. |
| `consent-store.ts` | ARCHIVE | Depended on archived `skill-tiers`/`capability-inventory` overlay (email/money/health categories). v0.1 has only `gmail_read`. |
| `oauth-routes.ts` (+ test) | ARCHIVE | Multi-skill OAuth router (`getCategoryForSkill`, `getOAuthScopesForSkill`). v0.1 hardcodes one provider (Google) and one scope (`gmail.readonly`). Rebuilt fresh in Phase 3. |
| `format.ts` | ARCHIVE | WhatsApp/iMessage outbound text formatter. SendBlue does this in Phase 3. |
| `normalize.ts` (+ test) | ARCHIVE | Imported archived `capability-inventory` from `@brevio/shared`. |
| `pending-message-store.ts` (+ test) | ARCHIVE | Per future-architecture-notes §Resume-After-OAuth Pattern. |
| `rate-limit.ts` (+ test) | ARCHIVE | Only caller was archived `consent-routes`. |
| `security.ts` | ARCHIVE | WhatsApp HMAC verification — chat-UI specific. |
| `state.ts` | ARCHIVE | Session + rate-limit state for chat-UI webhooks. |
| `workflow-runtime.ts` (+ test) | ARCHIVE | Per future-architecture-notes §Workflow Runtime. |
| `server.test.ts` | KILL | Tested archived routes. |

### `packages/shared/src/` — interfaces + schemas, planner-era types archived

| File | Verdict | Notes |
|---|---|---|
| `index.ts` | REWRITE_MINIMAL | Re-exports trimmed to surviving modules only. |
| `errors/index.ts` | KEEP | `BrevioError` base class. |
| `constants/index.ts` | KEEP | `SERVICE_NAMES`. |
| `utils/index.ts` | KEEP | `sha256Hex`. |
| `interfaces/skill-adapter.ts` | KEEP | Skill plugin shape (`ISkillAdapter`, `SkillInput`, `SkillContext`). |
| `schemas/skill-result.ts` | KEEP | Zod schema for skill results. |
| `capability-inventory.ts` (+ test) | ARCHIVE | Per future-architecture-notes §Capability Discovery. |
| `skill-tiers.ts` (+ test) | ARCHIVE | Per future-architecture-notes §Safety Tier System. |
| `schemas/a2a-runtime.ts` | ARCHIVE | Per future-architecture-notes §Agent-to-Agent Protocol. |
| `schemas/message-envelope.ts` | ARCHIVE | Multi-process design. v0.1 collapses to single deployable. |

Also added `@types/json-schema` as devDep — `interfaces/skill-adapter.ts` and `schemas/skill-result.ts` use `JSONSchema7` types from there.

---

## Non-KEEP services / packages (all killed in Phase 1)

| Path | Verdict | Reason |
|---|---|---|
| `services/brevio-auth` | KILL | Already in `deprecated/`. Auth lives in gateway/auth.ts now. |
| `services/brevio-edge-relay` | KILL | Already in `deprecated/`. Recent hardening (b76de6d9) predated the salvage plan. |
| `services/brevio-hands`, `services/hands-runtime` | KILL | Action execution. Premature — no v0.1 caller. |
| `services/brevio-metrics` | KILL | Structured logs + audit log are enough for v0.1. |
| `services/brevio-profile` | KILL | Folded into brain/gateway later. |
| `services/brevio-scheduler` | KILL | v0.1 uses an in-app cron worker. |
| `services/brevio-temporal-worker` | KILL | No Temporal in v0.1. |
| `services/browser-mcp` | KILL | No browser/computer-use in v0.1. |
| `packages/proto` | KILL | One service, no inter-service messaging. |
| `packages/sdk` | KILL | Premature SDK surface. |
| `edge/` | KILL | Edge worker code. Folded into app later. |
| `deprecated/` | KILL | The pre-archived junk drawer. |

---

## Top-level legacy code (KILL)

### Compiled Go binaries (~300MB)

Removed from working tree: `agents`, `brain`, `brevioctl`, `browser`, `canvas`, `control`, `cron`, `executor`, `hands`, `marketing`, `memory`, `router`, `temporal-worker`. None were in git proper.

### Go source + build/runtime artifacts

`cmd/` (~24K), `internal/` (~2.1M), `vendor/`, `go.mod`, `go.sum`, `.golangci.yml`, `Dockerfile`, `.dockerignore`, `docker-compose.yml`, `Makefile`, `sbom.spdx.json` — all gone. v0.1 is pure TS.

### Premature infrastructure

`helm/` (252K Kubernetes), `terraform/` (small source + 8.5GB of local `.terraform/` plugin caches), `policies/` (14 OPA files), `infra/` (SST/legacy), and the entire pre-FOMO scaffolding: `admin/`, `api/`, `artifacts/`, `db/`, `evals/`, `prompts/`, `runbooks/`, `schemas/`, `scripts/`, `spec/`, `tests/`, `simulator/`, `config/`.

### Scratch docs

`CHECKLIST.md` (200KB), `DECISION.md` (254KB), `Brevio_V9*.docx` (3 files, ~350KB), `Deterministic*.pdf` (2 files, ~250KB), `CODEBASE_INVENTORY.md`, `GAP_ANALYSIS.md`, `FULL_FILE_INVENTORY.txt` (925KB), `P0B_FORENSIC_AUDIT_REPORT.md`, `TOOLS.md`, `brevio-openclaw-blueprint.docx`, `README.md` (was V9-era; rewritten fresh), `AI Agent (3).zip`, `AI Agent (5).zip`, `.simulator_gap_analysis.html`, `.repo_locks/`, `md files/`, `packages/proto/gen/`, `packages/shared/package-lock.json` (npm vs pnpm inconsistency).

### Docs and migrations

`docs/` pruned to just `future-architecture-notes.md`. Migrations `001` through `011` (v9-era schema) deleted; only `012_consent_audit_oauth` survives (the OAuth token table that Phase 3 will need).

---

## Phase 1 exit criteria

Per [FOMO_PLAN.md §19 Repo Cleanup Gate](FOMO_PLAN.md):

- [x] Build passes (`pnpm build` — 3 of 3 packages green)
- [x] Tests pass (`pnpm test` — 60 of 60 gateway tests green; brain has no tests; shared has no tests)
- [x] Salvage decisions documented (this file)
- [x] Future concepts archived ([docs/future-architecture-notes.md](docs/future-architecture-notes.md))
- [x] No fake active code

**Active surface after Phase 1:**

```
packages/shared/src/    6 active files (3 source + 3 utility/index files)
services/brevio-brain/  3 active source files (~135 lines)
services/brevio-gateway/  14 active source files + 7 tests (~1500 lines)
docs/future-architecture-notes.md  archive ledger
migrations/012_*  OAuth token + consent audit schema
```

The recoverability invariant — *every archived file accessible via `git show a288f3ed:<path>`* — is satisfied by the baseline commit.
