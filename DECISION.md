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
