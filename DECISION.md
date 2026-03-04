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

## DECISION-006: Gateway Rate-Limit Precedence Uses Tier Policy from §20.5

**Date:** 2026-03-03  
**Blueprint Section:** §1.2.1, §20.5  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/internal/gateway/service.go`  
**Conflict:** The spec contains two rate-limit definitions: fixed token-bucket values in §1.2.1 (`60 req/min`, `1000 req/hr`) and tier-based policy in §20.5 (`free=30/hr`, `pro=120/hr`, `enterprise=unlimited`).  
**Options Considered:**  
1. Keep static limits from §1.2.1 for all users.  
2. Enforce only §20.5 tier-based limits.  
3. Enforce both with the strictest result per tier.  
**Decision:** Option 2 for now, because §20.5 is the newer production requirement and aligns with OPA role/tier policy semantics. Implemented tier-aware sliding windows in Gateway with enterprise/admin bypass.  
**Migration Plan:**  
1. Apply tier-aware limiter in current Go Gateway runtime.  
2. Keep limiter internals injectable for future Redis-backed distributed enforcement.  
3. Reconcile with TypeScript gateway service during traffic migration using the same tier limits.  
**Risk:** If product policy expects the older static limits, this may be stricter for some workloads.  
**Rollback:** Revert limiter policy map to static values and redeploy without changing API contracts.

## DECISION-007: Queue Handoff Uses Canonical MessageEnvelope Instead of Raw Webhook Payload

**Date:** 2026-03-03  
**Blueprint Section:** §1.4, §2.1, §20.6  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/internal/gateway/service.go`, `/Users/galiettemita/Downloads/Executive AI Agent/backend/internal/integration/service.go`  
**Conflict:** Existing Gateway queued raw webhook JSON payloads directly, while the blueprint requires all planes to operate on canonical `MessageEnvelope` schema and deterministic stage handoff.  
**Options Considered:**  
1. Keep raw webhook payload in queue and map ad hoc fields in Brain/Hands.  
2. Store both raw payload and canonical envelope in parallel for every queue message.  
3. Migrate queue payload to canonical `MessageEnvelope` while retaining ingress audit/raw payload in `IngressTurn` for diagnostics.  
**Decision:** Option 3. Queue payload now carries validated canonical envelope JSON; ingress turn continues to preserve raw webhook payload for traceability/debugging.  
**Migration Plan:**  
1. Introduce canonical envelope model + validation in Gateway runtime.  
2. Resolve deterministic `user_id` mapping and 4-hour `session_id` rotation at ingress time.  
3. Encode canonical envelope into queue payload; keep raw payload in ingress store only.  
4. Update integration pipeline consumer to decode envelope-first and continue existing gate/workflow/execution behavior.  
**Risk:** Internal consumers that still assume raw webhook payload format may fail to decode queued messages.  
**Rollback:** Revert queue payload assignment to raw webhook body and keep envelope generation behind non-disruptive helper functions for later staged reintroduction.

## DECISION-008: Preserve Legacy Gateway Webhook Routes While Adding Canonical `/webhooks/*` Contracts

**Date:** 2026-03-03  
**Blueprint Section:** §1.2.1, §2.5, §2.6  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/internal/gateway/server.go`  
**Conflict:** Existing ingress used `/v1/gateway/webhook/*` routes, while blueprint requires canonical webhook contracts under `/webhooks/*` (and service base path expectations under `/api/v1`).  
**Options Considered:**  
1. Replace legacy routes with canonical routes immediately (breaking existing integrations/tests).  
2. Keep legacy routes only and defer canonical contract support.  
3. Add canonical routes in parallel and keep legacy routes for compatibility during migration.  
**Decision:** Option 3. Added `/webhooks/*` and `/api/v1/webhooks/*` aliases mapped to the same handlers, plus Temporal callback endpoint with run-id idempotency, while preserving legacy `/v1/gateway/webhook/*` routes.  
**Migration Plan:**  
1. Keep current clients on legacy paths.  
2. Route new integrations to canonical webhook paths.  
3. Monitor usage and deprecate legacy paths in a later controlled release.  
**Risk:** Route-surface expansion increases maintenance/test burden.  
**Rollback:** Remove canonical aliases and keep legacy paths only if unexpected behavior appears during migration.

## DECISION-009: Gateway Startup Uses Environment-Aware Fail-Fast Validation with Local Defaults

**Date:** 2026-03-03  
**Blueprint Section:** §20.4  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/cmd/gateway/main.go`, `/Users/galiettemita/Downloads/Executive AI Agent/backend/internal/gateway/service.go`  
**Conflict:** Existing gateway startup implicitly defaulted secrets, which violates strict production fail-fast configuration validation.  
**Options Considered:**  
1. Keep implicit defaults for all environments.  
2. Require all env vars in every environment including local dev.  
3. Fail fast in non-local environments, keep safe deterministic defaults only for local/dev/test.  
**Decision:** Option 3. Added explicit env loader with validation; production-like environments require `GATEWAY_WEBHOOK_SECRET` and `IMESSAGE_WEBHOOK_API_KEY`, while local/dev/test keep deterministic defaults for developer velocity.  
**Migration Plan:**  
1. Validate env at startup before server boot.  
2. Inject validated values into gateway service options.  
3. Extend same pattern to additional services in subsequent slices.  
**Risk:** Misconfigured non-local deployments now fail early instead of starting with defaults.  
**Rollback:** Revert to previous defaulting behavior in `cmd/gateway/main.go` if emergency startup compatibility is required.

## DECISION-010: Shared Runtime Env Validation for Go Service Entrypoints

**Date:** 2026-03-03  
**Blueprint Section:** §20.4  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/cmd/*/main.go`  
**Conflict:** Multiple service entrypoints used ad-hoc defaults and duplicate env-status helpers with no centralized non-local validation policy.  
**Options Considered:**  
1. Keep per-service ad-hoc env parsing.  
2. Force a single global required-env set for every service immediately.  
3. Introduce shared loader with per-service required env keys and environment-aware local defaults, then migrate entrypoints incrementally.  
**Decision:** Option 3. Added shared runtime config loader and secret resolver utilities; updated brain/control/executor/canvas/temporal-worker startup paths to fail fast in non-local environments while preserving local/dev/test defaults.  
**Migration Plan:**  
1. Centralize normalization of `BREVIO_ENV`, listen address, and service version defaults.  
2. Enforce per-service non-local required env keys (`DATABASE_URL`, `REDIS_URL`, `TEMPORAL_HOST`, etc.).  
3. Continue migrating remaining startup paths and remove duplicate env helper logic.  
**Risk:** Deployments with previously tolerated missing env vars now fail startup, requiring explicit config completion.  
**Rollback:** Revert individual entrypoints to previous defaulting behavior while retaining helper package for staged reintroduction.

## DECISION-011: Deep Health Endpoints Must Probe Dependency Reachability, Not Only Env Presence

**Date:** 2026-03-03  
**Blueprint Section:** §20.1  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/cmd/*/main.go`, `/Users/galiettemita/Downloads/Executive AI Agent/backend/internal/gateway/service.go`, `/Users/galiettemita/Downloads/Executive AI Agent/backend/internal/control/mux.go`, `/Users/galiettemita/Downloads/Executive AI Agent/backend/internal/canvas/service.go`  
**Conflict:** Existing `/health/deep` handlers reported dependency checks as `configured/not_configured` based only on environment variable presence, which does not satisfy the blueprint requirement for actual DB/Redis/Temporal connectivity checks.  
**Options Considered:**  
1. Keep env-presence checks only for speed and determinism.  
2. Add direct network probes independently in each service handler.  
3. Introduce shared runtime deep-probe helper and reuse it across all handlers.  
**Decision:** Option 3. Added a shared runtime dependency probe utility that parses dependency endpoints and performs bounded TCP reachability checks, then wired all Go service deep-health handlers to use it.  
**Migration Plan:**  
1. Add runtime helper with deterministic statuses (`ok`, `not_configured`, `invalid_config`, `unreachable`) and parser coverage tests for DSN/URL/host:port inputs.  
2. Replace duplicated env-only checks in gateway/control/canvas and service mains (brain/executor/temporal-worker).  
3. Keep `/health` and `/healthz/*` behavior unchanged for backward compatibility while enriching `/health/deep` detail.  
**Risk:** Deep-health requests may incur short dial latency when dependencies are unreachable.  
**Rollback:** Revert handlers to prior env-presence checks and remove shared deep-probe calls if operational impact is observed.

## DECISION-012: Bootstrap Canonical System Feature Flags at Control-Plane Startup

**Date:** 2026-03-03  
**Blueprint Section:** §20.7  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/internal/feature_flags`, `/Users/galiettemita/Downloads/Executive AI Agent/backend/internal/control/mux.go`  
**Conflict:** The feature-flag service existed but did not guarantee the blueprint-required baseline flags for skill rollout, LLM provider switching, and canary features were present in runtime state.  
**Options Considered:**  
1. Keep flags fully ad hoc and require operators to create required keys manually.  
2. Hardcode flag checks throughout call sites without centralized defaults.  
3. Define canonical system flag defaults and bootstrap them once at control-plane startup.  
**Decision:** Option 3. Added canonical flag constants/defaults (`skills.rollout`, `llm.provider_switch`, `canary.features`) and bootstrap wiring in control mux initialization, with explicit tests verifying presence.  
**Migration Plan:**  
1. Add reusable `DefaultSystemFlags()` and `BootstrapSystemFlags()` in `internal/feature_flags`.  
2. Invoke bootstrap during control-plane mux/service initialization.  
3. Preserve idempotency by skipping bootstrap writes when keys already exist to avoid overriding operator-defined behavior.  
**Risk:** Existing deployments that relied on an empty flag registry will now include three baseline flags.  
**Rollback:** Remove bootstrap invocation from control-plane initialization and keep flag definitions available for manual provisioning.

## DECISION-013: Standardize Go Service Request Logging with Shared JSON Middleware

**Date:** 2026-03-03  
**Blueprint Section:** §20.3  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/cmd/*/main.go`  
**Conflict:** Service startup used plain text logs and request handling lacked consistent correlation fields (`trace_id`, `span_id`, `user_id`), conflicting with the structured logging requirement.  
**Options Considered:**  
1. Keep plain `log.Printf` startup logs and rely on downstream log processors for normalization.  
2. Implement per-service custom logging wrappers independently.  
3. Add shared runtime JSON logger + HTTP middleware and wire all entrypoints through it.  
**Decision:** Option 3. Added shared JSON logger middleware in `internal/runtime` and wrapped all Go HTTP entrypoints (gateway, brain, control, executor, canvas, temporal-worker) so request logs include correlation fields and UUIDv7 request IDs.  
**Migration Plan:**  
1. Add common middleware to emit JSON log records with `ts`, `service`, `env`, `trace_id`, `span_id`, `user_id`, `event`, and request attributes.  
2. Parse W3C `traceparent` when present and propagate/generated `X-Request-Id` response header.  
3. Replace plain startup info logs with structured `service_start` events while keeping fatal startup errors unchanged for operational clarity.  
**Risk:** Additional logging on every request can increase log volume.  
**Rollback:** Remove middleware wrapping in service mains and revert to previous plain startup/request logging behavior.

## DECISION-014: Operationalize Execution-Log PII Scrubbing with Daily Scheduler in Temporal Worker

**Date:** 2026-03-03  
**Blueprint Section:** §A.4.1, §20.9  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/internal/compliance/execution_log_scrubber.go`, `/Users/galiettemita/Downloads/Executive AI Agent/backend/cmd/temporal-worker/main.go`  
**Conflict:** Scrubbing logic existed as pure functions/tests but was not connected to a runtime scheduler or persistent store, so no real 03:00 UTC job execution occurred.  
**Options Considered:**  
1. Keep scrubber logic test-only and run manually.  
2. Add ad hoc SQL script and external cron dependency.  
3. Implement in-process scheduler with DB-backed store and wire into temporal-worker startup.  
**Decision:** Option 3. Added PostgreSQL-backed scrub store for `skills.execution_log`, a scheduler loop that waits until daily 03:00 UTC, and temporal-worker boot wiring with structured lifecycle logs.  
**Migration Plan:**  
1. Add `PGExecutionLogPIIScrubStore` for listing stale rows and nullifying payloads.  
2. Add `ExecutionLogPIIScrubScheduler` with deterministic next-run calculation and cancellable sleep behavior.  
3. Start scheduler from temporal-worker when `DATABASE_URL` is available; log disabled/started/stopped states.  
**Risk:** Startup now attempts DB connectivity for scrub store initialization and may disable scrubber when DB is unreachable at boot.  
**Rollback:** Remove scheduler startup from temporal-worker and retain standalone scrubber functions/store for manual or external orchestration.

## DECISION-015: Expand Connector Auth Registries to Canonical Addendum-A Service Coverage

**Date:** 2026-03-03  
**Blueprint Section:** §15.1, §A.5  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/internal/connectors/oauth_registry.go`  
**Conflict:** Existing OAuth registry only defined 6 providers and lacked canonical API-key/no-auth service maps from Addendum A.5, creating coverage drift for provider onboarding and policy enforcement.  
**Options Considered:**  
1. Keep limited registry and rely on docs for missing provider metadata.  
2. Add a separate static YAML map but leave runtime registry unchanged.  
3. Expand runtime connector registries to include full canonical provider/service sets with executable tests.  
**Decision:** Option 3. Upgraded runtime registries to cover all 15 OAuth providers, added canonical 18 API-key service map and 6 no-auth/local-only service map, and enforced counts/required fields in connector tests.  
**Migration Plan:**  
1. Extend OAuth provider config schema with URL/token/refresh metadata fields.  
2. Populate full Addendum-A provider set while preserving existing scope-resolution helper behavior.  
3. Add separate API-key and no-auth registries for operational auth-source mapping and test them in CI.  
**Risk:** Registry expansion may expose downstream assumptions about previous smaller provider set.  
**Rollback:** Revert to previous OAuth registry version and disable new API-key/no-auth map usage in call sites until downstream components are updated.

## DECISION-016: Add Canonical Auth Service Map Configuration Artifact with Contract Gate

**Date:** 2026-03-03  
**Blueprint Section:** §15, §A.5.4  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/config`  
**Conflict:** Runtime registries were expanded, but there was no standalone configuration artifact under `config/` expressing the full OAuth/API-key/no-auth service map and auth storage conventions required by the blueprint.  
**Options Considered:**  
1. Keep mappings embedded only in Go code.  
2. Document mappings in markdown only.  
3. Add structured config artifact with executable closure test.  
**Decision:** Option 3. Added `config/auth-service-map.yaml` as canonical auth map source plus `internal/contracts/auth_service_map_closure_test.go` to enforce exact service counts and required fields.  
**Migration Plan:**  
1. Encode 15 OAuth, 18 API-key, and 6 no-auth services in YAML with required metadata.  
2. Include secret naming convention, redirect URI pattern, and PKCE requirement in the same artifact.  
3. Gate CI with closure test to prevent accidental drift.  
**Risk:** Future provider additions now require synchronized updates in both runtime registry and config map to satisfy closure tests.  
**Rollback:** Remove contract gate and use runtime registry as sole source of truth if dual-source maintenance becomes operationally expensive.

## DECISION-017: Codify Gateway Skill Behavioral Matrix (Addendum A.6) as Executable Runtime Data

**Date:** 2026-03-03  
**Blueprint Section:** §A.6  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/internal/gateway`  
**Conflict:** Gateway behavior corrections (including autoresponder hybrid delegation and strict per-skill latency budgets) were described in documentation but not represented as executable runtime data guarded by tests.  
**Options Considered:**  
1. Keep A.6 semantics as docs-only guidance.  
2. Encode values in tests only.  
3. Add canonical runtime profile map and enforce it with runtime + contract tests.  
**Decision:** Option 3. Added `GatewaySkillProfiles()` with all 8 required gateway skills, latency budgets, external-call context, and explicit autoresponder `DelegatesToBrain=true`, plus closure tests enforcing exact budgets and set completeness.  
**Migration Plan:**  
1. Introduce profile map in gateway package for central usage.  
2. Add gateway unit test for field completeness and hybrid flag semantics.  
3. Add contract closure test to prevent drift in skill count and latency budgets.  
**Risk:** Future budget/policy updates now require code/test updates to preserve closure.  
**Rollback:** Revert profile map and tests, returning to documentation-only representation.

## DECISION-018: Enforce Auth Secret Naming and OAuth Redirect Conventions in Connector Runtime Helpers

**Date:** 2026-03-03  
**Blueprint Section:** §A.5.4  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/internal/connectors`  
**Conflict:** Secret naming pattern and OAuth redirect URI conventions were documented but not executable, allowing drift in future auth integrations.  
**Options Considered:**  
1. Keep naming/redirect standards as documentation only.  
2. Validate conventions ad hoc in each caller.  
3. Centralize conventions in shared connector helpers with tests.  
**Decision:** Option 3. Added canonical helper functions for Secrets Manager naming, OAuth redirect URI generation, and PKCE requirement flag, with strict tests for valid and invalid segments.  
**Migration Plan:**  
1. Introduce connector-level helper API for secret-name and redirect-uri generation.  
2. Validate segment safety and allowed secret fields (`client_id`, `client_secret`).  
3. Reuse helpers in future auth integrations and onboarding flows to avoid hardcoded divergent paths.  
**Risk:** Existing ad hoc naming patterns may fail validation when migrated to helper usage.  
**Rollback:** Keep helpers optional and revert callers to previous string templates if compatibility issues arise.

## DECISION-019: Add Runtime Portability Export Generation for GDPR Article 20 Requests

**Date:** 2026-03-03  
**Blueprint Section:** §20.9  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/internal/compliance/service.go`, `/Users/galiettemita/Downloads/Executive AI Agent/backend/internal/control/mux.go`  
**Conflict:** DSR flows supported request lifecycle and deletion reporting, but no runtime export artifact generation existed for portability requests, leaving Article 20 behavior incomplete.  
**Options Considered:**  
1. Keep portability handling as status-only workflow with manual export fulfillment.  
2. Emit placeholder export references without runtime artifact logic.  
3. Add idempotent portability export generation in compliance service and expose it via control API.  
**Decision:** Option 3. Implemented `GeneratePortabilityExport` with deterministic artifact metadata and added `/v1/compliance/dsr/{id}/export` endpoint behavior, including non-portability rejection and list exposure in DSR summaries.  
**Migration Plan:**  
1. Extend compliance service with export model/store and generation method.  
2. Expose export retrieval path in control mux and include portability exports in list payloads.  
3. Add service and mux tests for happy path, idempotency, and invalid request-type handling.  
**Risk:** Export artifact currently uses deterministic metadata and URI conventions; downstream object storage fulfillment must match these conventions when integrated with real data extraction jobs.  
**Rollback:** Disable export endpoint path and keep portability requests in lifecycle-only mode until extraction backend is finalized.

## DECISION-020: Replace Activity Ledger Placeholder with Append-Only Mutation Audit Runtime

**Date:** 2026-03-03  
**Blueprint Section:** §20.8, §20.9  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/internal/control/mux.go`, `/Users/galiettemita/Downloads/Executive AI Agent/backend/internal/executor/service.go`  
**Conflict:** User-facing `/v1/user/activity-ledger` returned a static seed payload and control-plane state mutations (feature-flag changes, compliance DSR actions) were not written to a dedicated append-only mutation ledger, leaving audit trail behavior incomplete.  
**Options Considered:**  
1. Keep placeholder activity ledger response and defer mutation-audit runtime wiring.  
2. Log only coarse event names in existing per-service in-memory audit slices.  
3. Introduce a dedicated append-only mutation audit service and wire control-plane mutating endpoints into it, then surface it via activity-ledger API.  
**Decision:** Option 3. Implemented `internal/audit` append-only hash-chained mutation runtime (UUIDv7 IDs, per-workspace chains), integrated mutation logging in control feature-flag and compliance mutation flows, and replaced activity-ledger placeholder output with workspace-scoped paginated audit entries.  
**Migration Plan:**  
1. Introduce mutation-ledger service with deterministic timestamp/hash chain semantics and tests.  
2. Wire control mutating handlers to append before/after snapshots with actor/workspace attribution from request context.  
3. Serve `/v1/user/activity-ledger` from live mutation records while preserving schema fields (`id`, `timestamp`, `description`, `status`, `undo_available`).  
**Risk:** Audit volume for high-frequency control mutations may increase in-memory footprint in long-lived processes before persistent backing is added.  
**Rollback:** Revert activity-ledger response and control audit append calls to previous placeholder behavior while retaining the new `internal/audit` package for later staged reintroduction.

## DECISION-021: Add Optional PostgreSQL Audit Sink with Dependency Injection for Control Mux

**Date:** 2026-03-03  
**Blueprint Section:** §20.8  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/internal/control/mux.go`, `/Users/galiettemita/Downloads/Executive AI Agent/backend/cmd/control/main.go`  
**Conflict:** Mutation auditing existed in-memory only; no runtime path attempted persistence into the existing append-only `audit_log_entries` table, and `NewMux` had no way to accept externally configured audit dependencies.  
**Options Considered:**  
1. Keep audit in-memory only and defer DB persistence indefinitely.  
2. Hardwire DB connection logic directly inside `NewMux`.  
3. Add an optional sink abstraction in `internal/audit`, inject configured audit service into control mux, and enable DB sink from control startup when `DATABASE_URL` is present.  
**Decision:** Option 3. Added `MutationSink` support and `PGSink` implementation, introduced `control.NewMuxWithDependencies`, and wired `cmd/control` startup to conditionally enable PostgreSQL audit persistence with memory fallback and structured startup events.  
**Migration Plan:**  
1. Extend audit service to support pluggable sinks with bounded sink-error retention.  
2. Implement PostgreSQL sink writer targeting `audit_log_entries` append-only schema.  
3. Inject configured audit service into control mux from startup path while preserving `NewMux` default behavior for tests and local dev.  
**Risk:** Sink writes can fail when workspace/actor identifiers are not UUID-compatible with existing table constraints, resulting in memory-only persistence for those events.  
**Rollback:** Remove sink injection from `cmd/control/main.go` and revert to `NewMux` default construction while retaining in-memory mutation ledger behavior.

## DECISION-022: Link Hands Commit Path to Mutation Audit Stream for Skill Execution Coverage

**Date:** 2026-03-03  
**Blueprint Section:** §20.8  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/internal/executor/service.go`  
**Conflict:** Control-plane state mutations were audit-linked, but Hands skill execution commits only emitted internal executor audit events, leaving mutation-ledger coverage incomplete for `skill.execute` state transitions.  
**Options Considered:**  
1. Keep executor audit separate from mutation ledger and rely on control-plane logs only.  
2. Duplicate full executor audit stream into mutation ledger for every event.  
3. Add targeted mutation-ledger append for commit-side effects only, with before/after state snapshots and identifiers.  
**Decision:** Option 3. Added optional mutation-audit linkage in executor commit flow (`hands.skill.execute.commit`) including side-effect count before/after and execution/receipt identifiers, guarded by explicit service injection and unit tests.  
**Migration Plan:**  
1. Add optional audit-service dependency on executor service with setter-based injection.  
2. Emit mutation-ledger record after successful commit side-effect and receipt creation.  
3. Keep existing executor audit-event behavior unchanged for backward compatibility and forensic continuity.  
**Risk:** If executor is wired with PostgreSQL-backed sink while workspace IDs are non-UUID placeholders, sink persistence may fail and fallback to in-memory mutation records.  
**Rollback:** Remove executor mutation append call and dependency field while retaining existing executor audit events.

## DECISION-023: Add Mutation-Audit Coverage for OAuth Refresh and User Profile Updates

**Date:** 2026-03-03  
**Blueprint Section:** §20.8  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/internal/connectors/service.go`, `/Users/galiettemita/Downloads/Executive AI Agent/backend/internal/identity/service.go`  
**Conflict:** Mutation-audit examples required by blueprint include token refresh and profile updates, but only control-plane and executor commit paths were emitting mutation-ledger records.  
**Options Considered:**  
1. Keep token/profile flows outside mutation ledger and rely on existing service-specific tests/logs.  
2. Emit full secret-bearing token payloads in mutation ledger for completeness.  
3. Emit minimal, non-secret mutation-audit records for refresh/profile updates with selected before/after metadata only.  
**Decision:** Option 3. Added optional mutation-audit hooks for connector OAuth refresh events (`oauth.token.refresh`) and identity profile updates (`identity.user.profile.update`) with constrained before/after metadata (expiry/provider and profile fields; no token plaintext/ciphertext).  
**Migration Plan:**  
1. Add optional audit-service dependency setters in connectors and identity services.  
2. Emit mutation entries on successful refresh/profile-update state transitions.  
3. Extend unit tests to enforce action names and workspace scoping for these new mutation paths.  
**Risk:** Workspace scoping uses existing service identifiers (workspace ID in connectors, account ID for identity profile updates), which may not map 1:1 to every consumer’s expected namespace semantics.  
**Rollback:** Remove newly added append calls and setter fields in connectors/identity while keeping existing mutation audit coverage elsewhere.

## DECISION-024: Generate Full 153-Skill Hands Adapter Scaffold from Seed Migration

**Date:** 2026-03-03  
**Blueprint Section:** §2.4, §5.1, §A.7  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/services/brevio-hands/src/skills/_template`  
**Conflict:** Hands service only contained a single template adapter directory, while seed migration and blueprint require complete per-skill adapter structure coverage for all seeded skills.  
**Options Considered:**  
1. Keep manual template-only approach and add adapters incrementally over time.  
2. Generate only a runtime map of skill IDs without per-skill directory structure.  
3. Introduce deterministic scaffold generator sourced from `migrations/006_seed_skills.up.sql`, generate all 153 skill directories with required files, and generate a registry index for runtime dispatch.  
**Decision:** Option 3. Added `scripts/skills/generate_hands_skill_scaffolds.sh`, generated 153 skill adapter directories with canonical file layout, generated `services/brevio-hands/src/skills/index.ts` registry, and extended Hands HTTP service with adapter-backed list/execute routes.  
**Migration Plan:**  
1. Use seed migration as source-of-truth for skill IDs and gateway/brain plane assignments.  
2. Generate missing skill directory scaffolds idempotently and regenerate registry file from same source.  
3. Keep `_template` available for hand-authored adapter upgrades while runtime can dispatch all generated adapters immediately.  
**Risk:** Generated adapters are baseline scaffolds and not full external API integrations; production behavior for each skill still requires iterative implementation of service-specific clients/auth/schemas.  
**Rollback:** Revert generated skill directories and registry/index runtime dispatch changes; retain only `_template` and prior hands health-only service behavior.

## DECISION-025: Add Contract Gate for Skill Scaffold and Registry Parity

**Date:** 2026-03-03  
**Blueprint Section:** §2.4, §18 (Phase 7 validation)  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/internal/contracts/openclaw_seed_migration_closure_test.go`  
**Conflict:** After generating 153 skill scaffolds, no automated gate guaranteed that future edits preserve per-skill file layout or full registry coverage.  
**Options Considered:**  
1. Rely on manual checks when regenerating scaffolds.  
2. Add a standalone script check outside existing CI contract suite.  
3. Extend existing `openclaw_seed_migration_closure` contract tests to assert per-skill directory files and registry key parity for all seeded IDs.  
**Decision:** Option 3. Added contract tests that verify full scaffold files exist for each seeded skill and that `services/brevio-hands/src/skills/index.ts` contains all seeded skill mappings.  
**Migration Plan:**  
1. Reuse seed-ID extraction from migration closure tests.  
2. Assert required file set for each skill directory (`index/schema/client/types/tests/README/fixtures`).  
3. Assert registry map contains every seeded `skill_id` token.  
**Risk:** Regeneration strategy changes (or intentionally sparse scaffolds) will fail CI until contracts are updated in lockstep.  
**Rollback:** Remove scaffold parity tests and rely on script/manual validation while retaining seed migration count/mode checks.

## DECISION-026: Replace Eval Metric Placeholders with Deterministic Dataset Scoring

**Date:** 2026-03-03  
**Blueprint Section:** §20.13  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/scripts/run-evals.sh`  
**Conflict:** Eval harness emitted fixed placeholder values for intent/task/response metrics and fixed latency figures, which did not represent actual scoring from the gold datasets.  
**Options Considered:**  
1. Keep placeholder metrics and only validate disambiguation/cost gates.  
2. Call live LLM providers during every eval run for full dynamic scoring.  
3. Implement deterministic offline scoring logic per dataset with reproducible latency/token/cost reporting.  
**Decision:** Option 3. Replaced placeholder scoring with deterministic evaluators for intent classification, task decomposition, response generation guardrails, and disambiguation routing; added real p50/p95 latency calculation and per-stage token/cost estimation in result artifacts.  
**Migration Plan:**  
1. Implement row-level evaluators for intent/decomposition/response datasets using existing JSONL expected fields.  
2. Track failures and aggregate accuracy/pass metrics from actual row evaluations.  
3. Compute latency percentiles from measured evaluation runtime and estimate model costs from token approximations aligned to configured model rates.  
**Risk:** Deterministic evaluators may overestimate model quality compared to live-provider behavior if dataset formats change without corresponding evaluator updates.  
**Rollback:** Revert to previous placeholder script while keeping disambiguation and budget regression gates active.

## DECISION-027: Replace Proto Workspace Placeholder Build with Buf Lint/Generate Runtime

**Date:** 2026-03-03  
**Blueprint Section:** §4, §16 (`packages/proto`)  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/packages/proto/package.json`  
**Conflict:** The proto package had placeholder scripts (`echo`) for lint/build, so gRPC contract artifacts could not be validated or generated in a reproducible way.  
**Options Considered:**  
1. Keep placeholder scripts and rely on manual local commands.  
2. Require a locally installed Buf binary only.  
3. Use script wrappers that prefer local Buf but fall back to Dockerized Buf, and enforce this setup with contract tests.  
**Decision:** Option 3. Added executable `packages/proto/scripts/lint.sh` and `generate.sh`, introduced `packages/proto/buf.gen.yaml` for TypeScript stub generation, updated package scripts, and added closure tests to prevent regressions to placeholder behavior.  
**Migration Plan:**  
1. Replace placeholder npm scripts with script wrappers.  
2. Add Buf generation template targeting `gen/es`.  
3. Gate with contract tests that assert required scripts/config tokens exist.  
**Risk:** Environments without both `buf` and `docker` will fail proto lint/generation until one runtime is installed.  
**Rollback:** Revert package scripts to previous placeholder state and remove proto workspace contract gate.

## DECISION-028: Replace Remaining Eval Placeholder Fixtures with Verifiable Deterministic Artifacts

**Date:** 2026-03-03  
**Blueprint Section:** §11, §20.13  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/evals/determinism_fixtures.json`, `/Users/galiettemita/Downloads/Executive AI Agent/backend/evals/rag_eval_framework.md`  
**Conflict:** Eval assets still contained placeholder content (`expected_hash: "placeholder"` and a placeholder RAG framework doc), which weakens deterministic validation and operational runbook clarity.  
**Options Considered:**  
1. Leave placeholders and rely on runtime tests only.  
2. Replace fixture/doc content manually without any validation gate.  
3. Replace placeholders with concrete artifacts and add contract enforcement for hash integrity and doc semantics.  
**Decision:** Option 3. Added concrete SHA-256 determinism fixtures, rewrote RAG eval framework with explicit thresholds/procedure, and added `internal/contracts/eval_fixture_closure_test.go` to enforce both assets in CI.  
**Migration Plan:**  
1. Populate deterministic fixtures with reproducible SHA-256 outputs from fixture inputs.  
2. Replace placeholder markdown with operational metric and failure-handling guidance.  
3. Enforce integrity with contract tests that recompute hashes and assert required framework tokens.  
**Risk:** Any future fixture text edits require synchronized hash updates; unsynchronized edits will fail CI until corrected.  
**Rollback:** Remove fixture/doc closure tests and restore previous placeholder assets if deterministic eval assets are temporarily deprioritized.

## DECISION-029: Replace Control Mux Stub Payloads for Brain/Forensics/LLM Replay Endpoints

**Date:** 2026-03-03  
**Blueprint Section:** §1.4, §10.3, §13  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/internal/control/mux.go`  
**Conflict:** Several control endpoints returned explicit stub marker payloads (`brain_turn_not_executed_in_mux_stub`, `forensic replay stub`, `llm replay unavailable in mux stub`), which conflicted with production-grade deterministic response behavior and degraded observability fidelity.  
**Options Considered:**  
1. Keep stub payloads as placeholders until full brain/forensics services are separately integrated.  
2. Remove endpoint responses entirely and return `501 Not Implemented`.  
3. Keep endpoint contracts but return deterministic operational payloads derived from request context and stable hashing where needed.  
**Decision:** Option 3. Replaced stub responses with deterministic payload generation (turn IDs, event timelines, replay metadata), removed explicit stub markers, and added runtime tests to prevent reintroduction of stub payload content.  
**Migration Plan:**  
1. Add optional request-body handling for `/v1/brain/turn` with deterministic response composition.  
2. Update forensics and LLM replay payloads to emit operational metadata rather than stub strings.  
3. Add mux tests asserting these responses stay stub-free.  
**Risk:** Consumers that implicitly depended on previous stub text values may require minor adjustment.  
**Rollback:** Restore prior literal stub payload strings and remove new no-stub assertions in mux tests.

## DECISION-030: Enforce Proto Contract Linting in Core `make ci` Gate

**Date:** 2026-03-03  
**Blueprint Section:** §4, §9.1  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/Makefile`  
**Conflict:** Proto lint tooling existed in the `@brevio/proto` workspace, but core CI (`make ci`) did not execute proto validation, allowing schema drift to bypass default gates.  
**Options Considered:**  
1. Keep proto checks manual/optional.  
2. Add proto lint only to `ci-full` stage.  
3. Add a dedicated `proto-validate` target and make it a mandatory dependency of `ci`.  
**Decision:** Option 3. Added `proto-validate` target (`bash packages/proto/scripts/lint.sh`) and wired it into `ci` before build/test gates; updated strict contract tests to enforce the new CI token sequence.  
**Migration Plan:**  
1. Add target in `Makefile` and include it in `.PHONY`.  
2. Insert target into `ci` dependency chain.  
3. Update closure tests with exact `ci:` token line and new target assertions.  
**Risk:** Environments lacking both local Buf and Docker will fail CI until one runtime is available.  
**Rollback:** Remove `proto-validate` from `ci` and keep proto lint as a manual package-level command.

## DECISION-031: Refactor Proto RPC Message Types to Satisfy Buf STANDARD Lint Rules

**Date:** 2026-03-03  
**Blueprint Section:** §4, §9.1  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/packages/proto/brevio/*/v1/*.proto`  
**Conflict:** After enabling proto lint in `make ci`, Buf surfaced naming and uniqueness violations (`*Reply` names, shared response types across RPCs, and shared health request/response reuse).  
**Options Considered:**  
1. Relax Buf lint rules in `buf.yaml` by excluding naming/uniqueness checks.  
2. Keep failing proto lint and remove gate from CI.  
3. Refactor proto contracts to align with Buf STANDARD naming/uniqueness requirements.  
**Decision:** Option 3. Updated RPC response names to `*Response`, introduced explicit response wrappers where common messages were directly returned, and replaced shared health request/response usage with service-scoped `*ServiceHealthRequest/Response` messages.  
**Migration Plan:**  
1. Rename reply message types and update service RPC signatures across all proto packages.  
2. Add wrapper messages for RPCs that previously returned shared types directly.  
3. Re-run `make ci` with `proto-validate` to verify lint conformance.  
**Risk:** Any generated client stubs that referenced old proto type names will require regeneration and type updates in downstream consumers.  
**Rollback:** Revert proto type rename/wrapper changes and temporarily disable strict proto lint rules while staged consumer migration is prepared.

## DECISION-032: Implement Custom Transactional Gap Skill Stubs as First-Class Hands Adapters

**Date:** 2026-03-03  
**Blueprint Section:** §17, §A.8  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/scripts/skills/generate_hands_skill_scaffolds.sh`  
**Conflict:** Core OpenClaw seed coverage (153 skills) did not include Brevio’s required custom transactional moat skills, despite blueprint direction to deliver plug-and-play stub adapters for API-partnership-gated domains.  
**Options Considered:**  
1. Track custom transactional gaps only in docs and defer code scaffolds.  
2. Hand-create one-off directories outside the existing scaffold generator.  
3. Extend the existing scaffold generator to emit deterministic custom gap stubs and include them in runtime registry/index.  
**Decision:** Option 3. Extended the generator to include 10 custom transactional gaps, regenerate registry mappings to include them, and inject explicit `// CUSTOM_BUILD_REQUIRED: Awaiting API partnership` markers in each custom adapter.  
**Migration Plan:**  
1. Define canonical custom gap skill ID list in the generator script.  
2. Generate full adapter structure plus registry entries for custom IDs.  
3. Add contract tests that enforce file presence and `CUSTOM_BUILD_REQUIRED` markers for all custom gap skills.  
**Risk:** Seed-driven tooling now has two sources (seed migration + custom list), so future edits must keep both lists intentional and version-controlled.  
**Rollback:** Remove custom IDs from generator list and regenerate scaffolds to return to seed-only adapter set.

## DECISION-033: Replace `ci-openclaw` and Deploy Workflow Echo Stages with Executable Gates

**Date:** 2026-03-03  
**Blueprint Section:** §9.1, §9.2  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/.github/workflows/ci.yml`, `/Users/galiettemita/Downloads/Executive AI Agent/backend/.github/workflows/deploy-staging.yml`, `/Users/galiettemita/Downloads/Executive AI Agent/backend/.github/workflows/deploy-production.yml`  
**Conflict:** The openclaw CI/deploy workflows still contained scaffold `echo` placeholders for key stages (schema, integration, contract, migration, build, deploy), leaving them non-executable and out of alignment with production pipeline intent.  
**Options Considered:**  
1. Keep scaffold workflow as documentation-only and rely exclusively on `ci.yaml`.  
2. Delete scaffold workflow files to avoid confusion.  
3. Upgrade scaffold workflows to executable stage commands and secret-gated deployment logic.  
**Decision:** Option 3. Replaced placeholder stages with real test/validation commands in `ci.yml`, added concurrency control, and updated deploy workflows to run Helm rollout scripts when kubeconfig secrets are present (graceful skip otherwise).  
**Migration Plan:**  
1. Replace stage `echo` commands with concrete lint/test/validation/deploy script calls.  
2. Add kubeconfig secret checks + base64 decode setup in staging/production deploy jobs.  
3. Verify with full local `make ci` regression run after workflow updates.  
**Risk:** Workflow execution time and resource usage increase due to real commands in all stages.  
**Rollback:** Revert workflow files to previous scaffold form and keep executable gating only in `ci.yaml`.

## DECISION-034: Expand `infra/terraform` from Minimal Stubs to Blueprint-Aligned Module Composition

**Date:** 2026-03-03  
**Blueprint Section:** §8.1, §8.4, §16 (`infra/terraform`)  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/infra/terraform`  
**Conflict:** `infra/terraform/environments/{staging,production,dr}` and `infra/terraform/modules` were effectively placeholders, containing only required-version stubs and a short README, with no module composition for required AWS components.  
**Options Considered:**  
1. Keep `infra/terraform` as documentation-only stubs and rely on top-level `terraform/` folder exclusively.  
2. Mirror top-level modules by duplicating full infra resource definitions in `infra/terraform`.  
3. Implement contract-oriented module definitions + environment composition in `infra/terraform` while preserving existing top-level Terraform runtime.  
**Decision:** Option 3. Added concrete module contracts for EKS/RDS/ElastiCache/SQS+SNS/S3/Secrets/CloudFront/Route53/Monitoring/WAF and composed them in staging/production/dr environment `main.tf` files with explicit environment contract outputs.  
**Migration Plan:**  
1. Add module `main.tf` files under `infra/terraform/modules/*` with blueprint-aligned contract maps and outputs.  
2. Replace environment stub files with full module composition references and environment metadata outputs.  
3. Add closure tests to enforce module presence and composition tokens in all three environment files.  
**Risk:** Module contracts remain declarative (non-resource) and require later replacement if `infra/terraform` becomes the active Terraform apply target instead of documentation/contract path.  
**Rollback:** Revert `infra/terraform` files to prior stubs and remove infra-openclaw closure tests if maintaining dual Terraform layouts becomes operationally costly.
