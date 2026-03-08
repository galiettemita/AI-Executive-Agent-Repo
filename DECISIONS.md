# BREVIO Architecture Decisions

This document records binding architectural decisions made during blueprint reconciliation.

## D001: V10 Enum Completion — negotiation_state

**Context:** BP06 includes ellipses in the negotiation_state enum definition.
**Decision:** Implement minimal complete deterministic set:
```
negotiation_state = ('proposed','evaluating','accepted','rejected','expired','executing','executed','failed','compensating')
```
**Rationale:** These states cover the full lifecycle of a federation negotiation including error and compensation paths. The `compensating` state is required for Temporal saga-pattern rollback. Locked by schema migration 008 and contract tests.

## D002: V10 Enum Completion — federation_permission_type

**Context:** BP06 includes ellipses in the federation_permission_type enum.
**Decision:** Implement minimal complete deterministic set:
```
federation_permission_type = ('calendar_query','calendar_write','routing_negotiate','task_delegate','knowledge_share','status_query')
```
**Rationale:** These cover the core cross-workspace interactions defined in the federation blueprint. Additional permissions can be added via forward migration. Locked by schema migration 008 and contract tests.

## D003: V10.1 Table Count Reconciliation

**Context:** BP02 states "16 new tables" but the body defines 18 CREATE TABLE statements.
**Decision:** Implement ALL 18 tables. Treat "16" as outdated count.
**Rationale:** The body content is authoritative per blueprint interpretation rules. All 18 tables are required for complete admin functionality.

## D004: Temporal as ONLY Workflow Runtime

**Context:** Existing codebase has in-memory workflow state machines in `internal/workflows/service.go`.
**Decision:** Preserve existing in-memory implementations as business logic reference, wrap with real Temporal SDK workflow/activity definitions in `internal/temporal/`.
**Rationale:** The in-memory implementations contain tested business logic (plan scoring, trust evaluation, etc.) that should not be discarded. The Temporal wrapper provides durability, replay safety, and distributed execution.

## D005: UUIDv7 Reconciliation Strategy

**Context:** `uuid_v7_generate()` in migration 001 returns `gen_random_uuid()` (random UUIDs, not UUIDv7).
**Decision:** Forward-only reconciliation migration (007) redefines the function to produce real RFC 9562 UUIDv7. Existing PKs remain valid; new PKs are time-ordered.
**Rationale:** Changing existing PKs would require cascading foreign key updates across all tables. Forward-only approach is safe and preserves data integrity.

## D006: Go for Cloud Plane Services

**Context:** Repo contains both Go (cmd/*, internal/*) and TypeScript (services/*) implementations of plane services.
**Decision:** Go is authoritative for all five planes. TypeScript plane duplicates are quarantined as NON_PRODUCTION.
**Rationale:** Blueprint mandates Go for cloud plane services. TS is allowed only for Hands Skill Runtime sidecar, Edge Agent, and Web Demo Frontend.

## D007: Canonical Infrastructure Sources

**Context:** Duplicate infrastructure exists (terraform/ vs infra/terraform/, helm/ vs infra/helm/, ci.yml vs ci.yaml).
**Decision:**
- Canonical infra: `infra/` (infra/terraform + infra/helm)
- Canonical CI: `.github/workflows/ci.yml`
- Removed: `.github/workflows/ci.yaml` (duplicate)
**Rationale:** Single source of truth prevents deployment drift and CI redundancy.

## D008: Authorization Receipt Chain

**Context:** Blueprints mandate non-bypassable authorization but existing code has in-memory approval tokens.
**Decision:** Implement durable authorization receipts with DB persistence. Gate evaluation order: kill_switch (highest precedence) → sandbox → skills → dm_pairing → call_approval → budget → rate_limit.
**Rationale:** Durable receipts provide audit evidence and replay protection. Gate ordering ensures highest-impact blocks are checked first.

## D009: Forward-Only Migration Strategy

**Context:** db/migrations/ contains numbered .sql files. Some earlier versions had up/down pairs.
**Decision:** Forward-only migrations are the production toolchain. Rollback is via database snapshot + forward fix. Down migrations are not executed in production.
**Rationale:** Down migrations are inherently risky in production. Snapshot + forward fix is the safest rollback strategy.

## D010: Transcript-Only Voice Persistence

**Context:** BP05 defines voice/call capabilities with transcript persistence.
**Decision:** Never persist raw audio. Only transcripts (text segments with timing) are stored.
**Rationale:** Raw audio storage introduces significant compliance burden (GDPR, CCPA). Transcripts provide the necessary audit trail.

## D011: Cost Values as NUMERIC(18,8)

**Context:** BP06 mandates cost values as NUMERIC(18,8) USD.
**Decision:** All cost/financial columns use `numeric(18,8)` with USD denomination.
**Rationale:** Sufficient precision for sub-cent calculations while avoiding floating-point rounding issues.

## D012: workspace_id Universal Tenant Scope

**Context:** Blueprints mandate workspace_id as universal tenant boundary.
**Decision:** No "default workspace" fallback in production. Gateway fails closed when workspace scope is missing/invalid. All DB sessions SET app.workspace_id for RLS enforcement.
**Rationale:** Fail-closed prevents accidental cross-tenant data access. RLS enforcement at the database level provides defense-in-depth.

## D013: Kill Switch Non-Bypassable

**Context:** Kill switch must halt all workspace operations.
**Decision:** Kill switch check is the FIRST gate evaluated (highest precedence). When active, no receipts can be issued, no workflows can execute, no tools can be invoked.
**Rationale:** Emergency shutdown capability requires absolute precedence over all other gates.
