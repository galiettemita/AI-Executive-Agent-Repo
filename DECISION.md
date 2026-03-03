## DECISION-001: Coexistence Migration Instead of Rewrite

**Date:** 2026-03-03  
**Blueprint Section:** §0.4, §16, §18  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/cmd`, `/Users/galiettemita/Downloads/Executive AI Agent/backend/internal`  
**Conflict:** Existing platform is a production-hardened Go runtime with extensive tests and contracts, while the target blueprint requires a pnpm/TypeScript monorepo and different service boundaries.  
**Options Considered:**
1. Preserve existing Go architecture and ignore TS blueprint requirements.
2. Full destructive rewrite to TypeScript-only architecture.
3. Hybrid coexistence: preserve current runtime and add target architecture incrementally in parallel.
**Decision:** Option 3. Use a non-destructive coexistence migration so current functionality, tests, and deployability remain intact while target services are introduced incrementally.  
**Migration Plan:**  
1. Complete discovery/gap artifacts first (`CODEBASE_INVENTORY.md`, `GAP_ANALYSIS.md`).  
2. Add target directory structure (`packages/`, `services/`, `edge/`, `infra/`, `tests/`) without removing current directories.  
3. Implement new services/contracts behind versioned interfaces and cut traffic by feature flag.  
4. Decommission old paths only after parity and rollback readiness are verified.  
**Risk:** Operational complexity during dual-stack period; possible contract drift between old/new service paths.  
**Rollback:** Keep old service entrypoints and routing as primary while disabling new paths via feature flags and deployment values.

## DECISION-002: Additive Database Delta Strategy with Legacy Preservation

**Date:** 2026-03-03  
**Blueprint Section:** §0.4, §3, §A.11  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/db/migrations`  
**Conflict:** Current DB model uses forward-only migrations and public-schema-centric tables; blueprint requires named schema groups and reversible up/down migration pairs with specific table shapes.  
**Options Considered:**
1. Replace existing migration chain with new 001-011 set.
2. Fork a new database and ignore legacy schema.
3. Keep legacy migration history immutable; add additive blueprint-aligned delta migrations and compatibility mapping.
**Decision:** Option 3. Preserve existing migrations and apply additive schema deltas only.  
**Migration Plan:**  
1. Leave `001..006` legacy migrations unchanged.  
2. Introduce new migration set adding required schemas/tables/constraints for OpenClaw blueprint.  
3. Backfill/bridge data from legacy tables where needed (for example legacy connector/tool execution to new skills execution ledger).  
4. Keep both models readable during transition; schedule future cleanup in later release gates only.  
**Risk:** Temporary dual-write/dual-read complexity and reconciliation bugs if mappings are incorrect.  
**Rollback:** Disable new write paths and continue using legacy tables; additive migrations remain harmless if unused.

## DECISION-003: Policy and Protocol Evolution via Versioned Internal Contracts

**Date:** 2026-03-03  
**Blueprint Section:** §1.3, §2.6, §6  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/internal/control`, `/Users/galiettemita/Downloads/Executive AI Agent/backend/api/openapi/v9.yaml`, `/Users/galiettemita/Downloads/Executive AI Agent/backend/policies`  
**Conflict:** Existing system uses HTTP-centric internal routing and V9 policy bundles; blueprint mandates gRPC mesh and specific `brevio.authz` policy semantics.  
**Options Considered:**
1. Keep HTTP-only internals and map gRPC requirement to docs only.
2. Replace all internal calls with gRPC immediately.
3. Add gRPC contracts incrementally while preserving HTTP paths; introduce new policy package with explicit matrix tests.
**Decision:** Option 3. Introduce new contracts in parallel and migrate safely.  
**Migration Plan:**  
1. Define shared proto contracts and stand up gRPC services for new components first.  
2. Keep HTTP ingress contracts stable; add adapter bridges where needed.  
3. Implement `policies/brevio/authz.rego` and explicit deny-cell tests for Access Control Matrix before enforcing in production.  
4. Shift traffic service-by-service with observability checks and rollback toggles.  
**Risk:** Mixed protocol paths may introduce inconsistent authorization behavior during rollout.  
**Rollback:** Route all calls through existing HTTP/control policy path until gRPC/policy parity is verified.

## DECISION-004: Global Message Dedup on Partitioned Messages Table

**Date:** 2026-03-03  
**Blueprint Section:** §2.5, §20.6, §3.3  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/migrations/001_core_schema.up.sql`, `/Users/galiettemita/Downloads/Executive AI Agent/backend/migrations/008_indexes.up.sql`  
**Conflict:** The blueprint requires unique `channel_message_id` idempotency, but PostgreSQL partitioned tables cannot enforce uniqueness on `channel_message_id` alone unless the partition key (`created_at`) is included, which weakens true global dedup semantics.  
**Options Considered:**
1. Use partitioned-table unique index on `(channel_message_id, created_at)` (not globally strict).
2. Drop partitioning to keep single-table unique constraint.
3. Keep partitioning and enforce global dedup via side-table + trigger.
**Decision:** Option 3. Added `public.message_channel_dedup` with primary key on `channel_message_id` and insert trigger on `public.messages` for strict global dedup.  
**Migration Plan:**  
1. Keep `public.messages` partitioned by month for retention/performance.  
2. Insert every inbound/outbound message id into `message_channel_dedup` before write.  
3. Raise duplicate exception on PK conflict to enforce idempotency invariant.  
**Risk:** Trigger path adds one extra write per message and can become a hot key path under extreme throughput.  
**Rollback:** Disable dedup trigger and rely on Redis idempotency cache as temporary fallback until a revised dedup strategy is deployed.

## DECISION-005: Implement Disambiguation and Authz Matrix in Existing Go Runtime First

**Date:** 2026-03-03  
**Blueprint Section:** §5.3, §6.2, §A.2  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/internal`, `/Users/galiettemita/Downloads/Executive AI Agent/backend/policies`  
**Conflict:** The blueprint specifies Brain-plane disambiguation and OPA matrix enforcement, while current production runtime is Go-first and TypeScript service scaffolds are not yet traffic-bearing.  
**Options Considered:**  
1. Delay implementation until TypeScript Brain service is production-ready.  
2. Duplicate logic in both Go and TypeScript immediately.  
3. Implement deterministic routing/policy in existing Go runtime now and keep TypeScript parity as a later migration step.  
**Decision:** Option 3. Added deterministic 11-group disambiguation in Go (`internal/brain/disambiguation`) and expanded `policies/brevio/authz.rego` + tests for the full access matrix.  
**Migration Plan:**  
1. Enforce routing/policy behavior in current runtime with unit and contract coverage.  
2. Keep config source in version-controlled YAML (`config/skill-disambiguation.yaml`) and policy bundle (`policies/brevio/authz.rego`).  
3. Port the same behavior to TypeScript Brain/Auth services when they become primary, validated by shared eval datasets and policy tests.  
**Risk:** Temporary implementation split between Go runtime and TypeScript scaffolds can drift if not continuously validated.  
**Rollback:** Disable new routing/policy paths via existing control-plane feature toggles and revert to previous generic routing behavior.
