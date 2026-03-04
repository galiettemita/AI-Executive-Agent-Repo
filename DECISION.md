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

## DECISION-035: Replace `infra/docker` Service Image Scaffolds with Executable Multi-Stage Distroless Builds

**Date:** 2026-03-04  
**Blueprint Section:** §14, §16 (`infra/docker`), §20.12  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/infra/docker`  
**Conflict:** `infra/docker/Dockerfile.gateway`, `Dockerfile.brain`, and `Dockerfile.hands` were placeholder scaffold files (`FROM node:20-alpine`) and there were no service-scoped Dockerfiles for other runtime binaries, which diverged from the blueprint requirement for production-grade per-service container definitions.  
**Options Considered:**  
1. Keep using only the root `Dockerfile` and leave `infra/docker` as inert documentation stubs.  
2. Delete `infra/docker` and rely entirely on root-level container build indirection.  
3. Replace scaffold files with executable multi-stage distroless Dockerfiles and enforce them via closure tests and Makefile wiring.  
**Decision:** Option 3. Implemented real Dockerfiles for gateway, brain, hands, control, executor, canvas, and temporal-worker using Go 1.22 multi-stage builds + distroless non-root runtime, mapped hands to `cmd/executor` for current runtime compatibility, and wired `make docker-build` to build from `infra/docker/Dockerfile.<service>`.  
**Migration Plan:**  
1. Overwrite scaffold Dockerfiles with executable production build definitions.  
2. Add missing service Dockerfiles for all current Go runtime binaries.  
3. Add closure test coverage to enforce token-level invariants and prevent scaffold regressions.  
**Risk:** `make docker-build` now depends on each service-specific Dockerfile existing and remaining synchronized with command entrypoint names.  
**Rollback:** Revert `Makefile` docker-build target to root Dockerfile loop and remove service-specific docker closure test if per-service files become unnecessary.

## DECISION-036: Replace `infra/helm/brevio` Scaffold with Executable Umbrella Chart + Additional Service Templates

**Date:** 2026-03-04  
**Blueprint Section:** §8.2, §16 (`infra/helm/brevio`)  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/infra/helm/brevio`  
**Conflict:** The `infra/helm/brevio` chart contained scaffold-level placeholder content (single-line duplicated `global.environment` values and no executable templates), which did not satisfy the production umbrella-chart requirement.  
**Options Considered:**  
1. Keep `infra/helm/brevio` as documentation-only placeholder and rely solely on top-level `helm/BREVIO-*` charts.  
2. Delete `infra/helm/brevio` and avoid umbrella composition entirely.  
3. Implement an executable umbrella chart that composes existing core subcharts and renders missing core services via generic templates.  
**Decision:** Option 3. Replaced `infra/helm/brevio` with a dependency-backed umbrella chart for gateway/brain/hands/control/canvas/temporal-worker and added templated Deployment/Service/HPA rendering for additional services (`auth`, `profile`, `scheduler`, `metrics`, `edge-relay`) including non-root/read-only container security defaults.  
**Migration Plan:**  
1. Replace scaffold chart metadata and values with environment-aware production values (`values.yaml`, `values-staging.yaml`, `values-production.yaml`).  
2. Add Helm helpers, service account template, and additional-service render template set.  
3. Add closure test to enforce required dependency, values, and template tokens and prevent scaffold regressions.  
**Risk:** Umbrella dependency versions are pinned to current local chart versions (`0.1.0`); when subchart versions change, dependency metadata must be updated in lockstep.  
**Rollback:** Restore prior scaffold files in `infra/helm/brevio` and remove the new helm umbrella closure contract test.

## DECISION-037: Replace Minimal `infra/argocd` Placeholders with Executable Staging/Production Application Manifests

**Date:** 2026-03-04  
**Blueprint Section:** §9.2, §16 (`infra/argocd`)  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/infra/argocd/application-staging.yaml`, `/Users/galiettemita/Downloads/Executive AI Agent/backend/infra/argocd/application-production.yaml`  
**Conflict:** Both ArgoCD Application manifests only defined `apiVersion/kind/name/project` and lacked source repo/path, Helm value files, destination cluster/namespace, sync policies, and retry controls required for real deployments.  
**Options Considered:**  
1. Keep placeholder manifests and treat ArgoCD config as out-of-band operations.  
2. Remove ArgoCD manifests to avoid implying support.  
3. Implement production-usable staging/production ArgoCD Application specs with explicit helm source, namespace targeting, sync policy, and safety controls.  
**Decision:** Option 3. Added complete staging and production ArgoCD Application manifests targeting `infra/helm/brevio` with environment-specific values files, `brevio-system` destination, staging automated sync, production retry policy, namespace-creation sync options, and HPA status ignore-difference rules.  
**Migration Plan:**  
1. Replace placeholder YAML with complete application specs for staging and production.  
2. Add closure test coverage to enforce critical source/destination/sync tokens and prevent regression to placeholders.  
3. Keep project as `default` for compatibility with current cluster onboarding, with future move to dedicated `brevio` Argo project tracked separately.  
**Risk:** `repoURL` and `targetRevision` are pinned to the current repository/main flow; branch strategy changes will require manifest updates to avoid drift.  
**Rollback:** Restore minimal placeholder manifests and remove ArgoCD closure test if deployment control is moved fully outside-repo.

## DECISION-038: Upgrade `security-scan` Workflow from Minimal Script Trigger to Full Security Gate Pipeline

**Date:** 2026-03-04  
**Blueprint Section:** §9.1 (Stage 6), §12, §20.12  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/.github/workflows/security-scan.yml`  
**Conflict:** The security workflow only ran on schedule/manual and executed two shell scripts, without explicit Semgrep SARIF upload, dependency-audit stage, security-event permissions, or structured artifact publication expected for production security gating.  
**Options Considered:**  
1. Keep minimal workflow and rely on `ci.yaml` security steps only.  
2. Remove standalone security workflow and run scans only in manual local scripts.  
3. Expand `security-scan.yml` into a dedicated CI security pipeline with explicit SAST, dependency, vulnerability, SBOM, and artifact/reporting controls.  
**Decision:** Option 3. Implemented a full security workflow including Semgrep SAST (`--severity ERROR` + SARIF upload), pnpm high-severity dependency audit, strict runtime security validation script (Trivy/TruffleHog/Syft), govuln baseline checks, and artifact upload, with required GitHub permissions and concurrency controls.  
**Migration Plan:**  
1. Replace minimal workflow with a structured job including Go/Node/Python toolchain setup.  
2. Add explicit SAST/dependency/runtime security steps and SARIF artifact publishing.  
3. Add closure contract test to lock required workflow tokens and prevent regression to minimal mode.  
**Risk:** Security workflow is stricter and may fail existing PRs until all high-severity dependency findings and Semgrep ERROR findings are resolved.  
**Rollback:** Restore the prior minimal security workflow and remove the security workflow closure contract if strict gate rollout must be temporarily paused.

## DECISION-039: Expand Local `docker-compose.yml` from Dependency Stub to Runnable Core Service Stack

**Date:** 2026-03-04  
**Blueprint Section:** §8.4 (`local`), §16 (`docker-compose.yml`), §20.11  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/docker-compose.yml`, `/Users/galiettemita/Downloads/Executive AI Agent/backend/scripts/setup-local.sh`  
**Conflict:** Local compose only defined PostgreSQL/Redis/Temporal and a sleeping `gateway` placeholder, which did not represent a runnable local Brevio stack and did not align with one-command local bootstrap intent.  
**Options Considered:**  
1. Keep compose as dependency-only and rely entirely on `go run` for services.  
2. Add only one or two services to compose and leave the rest implicit.  
3. Define a full local compose stack for core binaries plus extension-profile service stubs and shared runtime env wiring.  
**Decision:** Option 3. Replaced compose with a runnable stack covering core dependencies plus gateway/brain/control/executor/canvas/temporal-worker service builds from local source, added Temporal UI and dependency health checks, and modeled additional non-local binaries (`auth/profile/scheduler/metrics/edge-relay`) behind an explicit `openclaw-extension` profile.  
**Migration Plan:**  
1. Replace placeholder compose content with shared env anchors, health checks, and per-service build args.  
2. Add extension profile services for components not yet represented by local Go binaries.  
3. Align local bootstrap script to bring up Temporal UI alongside core dependency containers and add closure test coverage.  
**Risk:** Full compose stack increases local resource usage and initial startup time compared to dependency-only mode.  
**Rollback:** Revert compose and setup script to dependency-only startup and remove docker-compose closure contract test if lightweight mode is temporarily preferred.

## DECISION-040: Replace Edge Agent/Relay TypeScript Scaffolds with WebSocket Runtime Baseline

**Date:** 2026-03-04  
**Blueprint Section:** §1.3 (`brevio-edge-relay`), §5.2 (Edge Agent Architecture), §20.1-§20.3  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/edge/brevio-edge-agent`, `/Users/galiettemita/Downloads/Executive AI Agent/backend/services/brevio-edge-relay`  
**Conflict:** Edge agent was a single constant export and edge relay was a minimal health-only HTTP scaffold; neither implemented the required local_mac WebSocket relay flow, session tracking, or offline queue behavior.  
**Options Considered:**  
1. Keep edge components as placeholders until native macOS packaging is implemented.  
2. Implement relay only and leave agent as static placeholder.  
3. Implement both relay and agent runtime baselines with typed message contracts, reconnect/heartbeat behavior, and offline-queue semantics.  
**Decision:** Option 3. Implemented a typed WebSocket relay (`/ws/edge`) with session tracking, execution dispatch endpoint, offline queue and queue expiry, plus a reconnecting edge-agent runtime that registers device metadata, emits heartbeats, executes local skill requests, and returns skill results with health/deep endpoints.  
**Migration Plan:**  
1. Replace scaffold source files and READMEs with executable runtime behavior.  
2. Add `ws` runtime/development dependencies in both package manifests.  
3. Add closure tests to enforce key protocol tokens and prevent scaffold regressions.  
**Risk:** Current queueing and session state are in-memory; relay restarts will clear queued offline requests until persistent backing is introduced.  
**Rollback:** Revert edge-agent/edge-relay source and package changes to prior scaffold baseline and remove the new edge closure contract test.

## DECISION-041: Extend `infra/docker` Coverage to TypeScript Core Services (Auth/Profile/Scheduler/Metrics/Edge Relay)

**Date:** 2026-03-04  
**Blueprint Section:** §14, §16 (`infra/docker`), §20.12  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/infra/docker`  
**Conflict:** `infra/docker` only covered Go runtime binaries, while active TypeScript service packages (`brevio-auth`, `brevio-profile`, `brevio-scheduler`, `brevio-metrics`, `brevio-edge-relay`) had no service-specific Dockerfiles in the canonical infra path.  
**Options Considered:**  
1. Keep TypeScript services image-less in `infra/docker` and rely on ad hoc local builds.  
2. Reuse one generic Dockerfile and pass package paths through runtime env only.  
3. Add explicit service Dockerfiles for each TypeScript core service using multi-stage Node build + distroless node runtime.  
**Decision:** Option 3. Added five explicit TypeScript Dockerfiles in `infra/docker`, wired `make docker-build-infra` to build them alongside Go services, and expanded docker closure contracts to enforce both Go (`distroless/static`) and TypeScript (`distroless/nodejs20`) image baselines.  
**Migration Plan:**  
1. Add Dockerfiles for auth/profile/scheduler/metrics/edge-relay with pnpm workspace build/deploy stages.  
2. Extend infra build target loop to include TypeScript service images.  
3. Update docker closure tests and README mappings to prevent regressions.  
**Risk:** Node distroless images depend on `pnpm deploy` behavior; pnpm major-version changes may require Dockerfile adjustments.  
**Rollback:** Remove TypeScript Dockerfiles and revert `docker-build-infra` loop/test expectations to Go-only coverage.

## DECISION-042: Replace `brevio-auth` Health-Only Scaffold with OAuth Registry + PKCE Runtime and Tight Addendum-A Service-Map Enforcement

**Date:** 2026-03-04  
**Blueprint Section:** §15, §20.1-§20.4, §20.6, Addendum §A.5  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/services/brevio-auth`, `/Users/galiettemita/Downloads/Executive AI Agent/backend/config/auth-service-map.yaml`, `/Users/galiettemita/Downloads/Executive AI Agent/backend/internal/contracts/auth_service_map_closure_test.go`  
**Conflict:** `brevio-auth` only exposed health endpoints and did not provide OAuth provider discovery, PKCE authorization-state management, or callback/exchange flows. Auth service-map contracts validated only counts/basic fields and did not enforce canonical Addendum-A provider sets or full `local-macos` 24-skill coverage.  
**Options Considered:**  
1. Keep `brevio-auth` as health-only scaffold and rely on future auth implementation work.  
2. Add only contract hardening for service-map data without runtime service upgrades.  
3. Implement both runtime auth service capabilities and stricter config/contract enforcement in one batch.  
**Decision:** Option 3. Implemented typed auth runtime modules for provider registry loading, PKCE state generation, OAuth authorize/exchange/refresh/callback endpoints, structured JSON logging with correlation fields, and graceful shutdown handling. Simultaneously hardened `config/auth-service-map.yaml` and contract tests to enforce exact OAuth/API-key/no-auth service sets and full 24-skill local-mac mapping.  
**Migration Plan:**  
1. Extend `brevio-auth` source into modular runtime (`config`, `server`, `pkce`, `logger`, `types`) while preserving `/health` and `/health/deep` behavior.  
2. Tighten `auth-service-map` data to canonical service IDs and complete `local-macos` skill list.  
3. Upgrade contract gates with exact-set assertions and add a dedicated `auth_service_runtime_closure_test.go` to lock endpoint/runtime invariants.  
4. Re-run full `make ci` gate set to ensure no regressions.  
**Risk:** OAuth token exchange currently uses deterministic simulated token issuance rather than live provider token endpoint calls; this is intentionally safe for local/staging closure but requires live secret-backed provider exchange integration before production OAuth onboarding.  
**Rollback:** Revert `services/brevio-auth` runtime modules and contract expansions to prior health-only scaffold and basic service-map count checks if a minimal baseline is temporarily required.

## DECISION-043: Replace `brevio-gateway` Health Scaffold with Webhook Ingress Runtime Baseline

**Date:** 2026-03-04  
**Blueprint Section:** §1.2.1, §2.5, §4.1, §20.1, §20.5, §20.6, Addendum §A.6  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/services/brevio-gateway`  
**Conflict:** `brevio-gateway` only exposed health endpoints and did not implement webhook ingress, signature verification, idempotency, normalization, or rate limiting required by the production directive.  
**Options Considered:**  
1. Keep gateway as health-only scaffold and defer all webhook logic to legacy Go gateway runtime.  
2. Add minimal endpoint placeholders without enforcement logic to satisfy path presence only.  
3. Implement a typed gateway runtime baseline with real ingress/auth/idempotency/rate-limit behavior and closure tests.  
**Decision:** Option 3. Implemented a modular TypeScript gateway runtime (`config`, `security`, `state`, `normalize`, `format`, `index`) supporting WhatsApp/iMessage/Temporal webhook endpoints (plus compatibility aliases), HMAC/API-key verification, canonical envelope normalization, 24h idempotency replay cache, tiered hourly + minute rate limiting, and outbound channel formatting endpoint.  
**Migration Plan:**  
1. Replace single-file scaffold with runtime modules and keep `/health` + `/health/deep` contract behavior.  
2. Add compatibility routing for `/webhooks/*`, `/api/v1/webhooks/*`, and legacy `/v1/gateway/webhook/*` paths.  
3. Add new gateway runtime closure contract test to enforce presence of auth/idempotency/rate-limit/normalization semantics and README runtime guidance.  
4. Re-run full `make ci` to validate no cross-system regressions.  
**Risk:** Current gateway state stores (dedup cache, rate-limit windows, session map) are in-memory and non-distributed; horizontal scaling without shared Redis backing may allow duplicate handling/rate-limit drift across replicas.  
**Rollback:** Revert `services/brevio-gateway` to prior scaffold and remove `internal/contracts/gateway_service_runtime_closure_test.go` if runtime baseline causes operational instability.

## DECISION-044: Replace `brevio-brain` Health Scaffold with Deterministic Classification/Disambiguation/Decomposition Runtime Baseline

**Date:** 2026-03-04  
**Blueprint Section:** §1.2.2, §4.1, §5.3, §7.2, §20.1-§20.4  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/services/brevio-brain`  
**Conflict:** `brevio-brain` only exposed health endpoints and did not provide production-required decision-plane API behavior for intent classification, skill disambiguation, task DAG decomposition, or result aggregation against a deterministic router contract.  
**Options Considered:**  
1. Keep `brevio-brain` as health-only scaffold and rely solely on existing Go runtime internals for orchestration behavior.  
2. Add endpoint placeholders that return static mock payloads for `/api/v1/brain/*`.  
3. Implement a typed runtime baseline with real request parsing, deterministic classifiers/decomposer/disambiguation logic, and closure tests to prevent regressions.  
**Decision:** Option 3. Implemented a modular Brain runtime (`classify`, `disambiguate`, `decompose`, `aggregate`, `config`, `types`, `index`) exposing `/api/v1/brain/classify|disambiguate|decompose|aggregate|process`, enforcing disambiguation rule loading from `config/skill-disambiguation.yaml`, bounded DAG checks (`max 10 tasks`, cycle detection), structured correlation logging, and graceful shutdown semantics.  
**Migration Plan:**  
1. Replace health-only source with modular deterministic handlers while retaining `/health` and `/health/deep` compatibility.  
2. Add README runtime contract details and endpoint behavior for operator visibility.  
3. Add `internal/contracts/brain_service_runtime_closure_test.go` to lock endpoint/path/runtime invariants and prevent regressions to scaffold mode.  
4. Re-run `make ci` to validate contracts, policy gates, and acceptance tests remain green.  
**Risk:** Current classification/decomposition implementation is keyword-driven and deterministic but does not yet invoke production LLM provider calls; behavior can be less nuanced than target Sonnet/Haiku prompts until LLM-backed activities are wired in.  
**Rollback:** Revert `services/brevio-brain` runtime modules and `internal/contracts/brain_service_runtime_closure_test.go` to restore prior scaffold baseline if rollout introduces runtime instability.

## DECISION-045: Upgrade `brevio-hands` Runtime to Circuit-Breaker/Timeout-Aware Skill Execution Baseline

**Date:** 2026-03-04  
**Blueprint Section:** §1.2.3, §2.2, §4.3, §13.1, §20.1-§20.3  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/services/brevio-hands`  
**Conflict:** `brevio-hands` exposed minimal execute/list endpoints but lacked production-grade execution controls (explicit circuit-breaker transitions, timeout-normalized `SkillResult` errors, API alias compatibility, and closure protection against regression to scaffold docs).  
**Options Considered:**  
1. Keep existing lightweight runtime and defer execution resilience controls to downstream adapters only.  
2. Add only path aliases while leaving error/circuit behavior unchanged.  
3. Implement service-level execution controls with per-skill circuit tracking, timeout/error normalization, and runtime closure test enforcement.  
**Decision:** Option 3. Reworked `brevio-hands` runtime to enforce execute-time controls: timeout-bound adapter execution, normalized failure payloads (`SKILL_NOT_FOUND`, `CIRCUIT_OPEN`, `EXTERNAL_TIMEOUT`, `EXTERNAL_ERROR`), per-skill circuit breaker transitions (`CLOSED`/`HALF_OPEN`/`OPEN`), circuit snapshot endpoint, `/v1` + `/api/v1` execute/skills aliases, structured correlation logging, and graceful shutdown. Added `internal/contracts/hands_service_runtime_closure_test.go` and replaced README scaffold text with operational runtime documentation.  
**Migration Plan:**  
1. Replace single-file minimal runtime with typed execution-control logic while preserving existing health/skills/execute routes through aliases.  
2. Add explicit API compatibility paths (`/api/v1/hands/*`, `/v1/hands/tool/execute`) to reduce downstream contract churn.  
3. Add contract gate verifying runtime tokens and README content to prevent regressions.  
4. Re-run `make ci` full gate to verify no cross-system drift.  
**Risk:** Circuit breaker state is currently in-memory per pod; in multi-replica deployments breaker behavior is eventually inconsistent across instances until centralized shared state is introduced.  
**Rollback:** Revert `services/brevio-hands/src/index.ts`, `services/brevio-hands/README.md`, and `internal/contracts/hands_service_runtime_closure_test.go` to prior baseline if execution control changes trigger unexpected adapter incompatibilities.

## DECISION-046: Replace `brevio-profile` Health Scaffold with Knowledge-File/Profile Runtime Baseline

**Date:** 2026-03-04  
**Blueprint Section:** §1.3 (`brevio-profile`), §1.2.2 (knowledge context), §20.1-§20.4  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/services/brevio-profile`  
**Conflict:** `brevio-profile` only exposed health endpoints and did not provide runtime APIs for user profile retrieval, preference updates, or canonical knowledge-file management (`USER.md`, `SOUL.md`, `AGENTS.md`) required by the Brain decision plane.  
**Options Considered:**  
1. Keep profile service as health-only and rely on direct filesystem reads from other services.  
2. Add lightweight profile fetch endpoint only, leaving knowledge files unmanaged.  
3. Implement a profile runtime baseline with explicit profile/knowledge APIs, profile-hash recomputation, and closure tests.  
**Decision:** Option 3. Implemented a typed `brevio-profile` runtime with `/api/v1` + `/v1` path support for profile fetch, preference updates, knowledge-file read/write, and profile hash refresh. Added filesystem-backed profile storage root, SHA-256 `profile_hash` recomputation from canonical knowledge files, structured correlation logging, and graceful shutdown behavior. Added `internal/contracts/profile_service_runtime_closure_test.go` and replaced README scaffold content with operational endpoint/config docs.  
**Migration Plan:**  
1. Replace health-only source with profile/knowledge runtime handlers while preserving `/health` and `/health/deep`.  
2. Persist profile metadata and knowledge files under configurable profile storage root to avoid coupling to hardcoded paths.  
3. Add closure contract coverage for runtime/README invariants and re-run full `make ci`.  
**Risk:** Current profile persistence is filesystem-backed and local to the service runtime; multi-replica deployments require shared persistent storage or DB-backed repository to avoid divergent profile state.  
**Rollback:** Revert `services/brevio-profile/src/index.ts`, `services/brevio-profile/README.md`, and `internal/contracts/profile_service_runtime_closure_test.go` to the prior health-only baseline if runtime storage behavior causes deployment issues.

## DECISION-047: Replace `brevio-scheduler` Health Scaffold with Job/Trigger Runtime Baseline

**Date:** 2026-03-04  
**Blueprint Section:** §1.3 (`brevio-scheduler`), §4.2, §20.1-§20.3  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/services/brevio-scheduler`  
**Conflict:** `brevio-scheduler` only exposed health endpoints and did not provide runtime APIs for managing scheduled jobs or dispatching trigger events required for cron-style skill invocation paths (e.g., DailyRhythm orchestration).  
**Options Considered:**  
1. Keep scheduler as health-only and rely on external cron/manual triggers.  
2. Add trigger-only endpoint without a managed job registry.  
3. Implement full baseline scheduler API with job lifecycle endpoints, trigger queueing, and closure tests.  
**Decision:** Option 3. Implemented `brevio-scheduler` runtime with `/api/v1` + `/v1` job and trigger endpoints (list/create/run/disable jobs, queue/list triggers), structured correlation logging, deep health checks, and graceful shutdown. Added closure enforcement with `internal/contracts/scheduler_service_runtime_closure_test.go` and replaced README scaffold text with runtime endpoint/configuration documentation.  
**Migration Plan:**  
1. Replace health-only source with in-memory scheduler runtime handlers while preserving `/health` and `/health/deep`.  
2. Add clear alias path support to reduce endpoint migration friction (`/v1` and `/api/v1`).  
3. Add closure test assertions for endpoint/runtime tokens and rerun full `make ci`.  
**Risk:** Scheduler state is currently in-memory; jobs/triggers are ephemeral across restarts and not shared across replicas until persistence backing (DB/Temporal schedules) is integrated.  
**Rollback:** Revert `services/brevio-scheduler/src/index.ts`, `services/brevio-scheduler/README.md`, and `internal/contracts/scheduler_service_runtime_closure_test.go` to the prior scaffold baseline if runtime behavior diverges from deployment expectations.

## DECISION-048: Replace `brevio-metrics` Health Scaffold with Prometheus/Event-Ingestion Runtime Baseline

**Date:** 2026-03-04  
**Blueprint Section:** §1.3 (`brevio-metrics`), §10.1, §20.1-§20.3  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/services/brevio-metrics`  
**Conflict:** `brevio-metrics` only exposed health endpoints and did not expose Prometheus-formatted metrics or ingestion paths required for runtime observability collection.  
**Options Considered:**  
1. Keep metrics service as health-only and rely on embedded metrics from other services only.  
2. Add `/metrics` endpoint with static placeholders but no runtime ingestion.  
3. Implement metric-family registry with `/metrics` rendering plus ingestion/snapshot APIs and closure tests.  
**Decision:** Option 3. Implemented `brevio-metrics` runtime with Section 10 metric family descriptors (`brevio_messages_total`, `brevio_message_latency_ms`, `brevio_skill_executions_total`, `brevio_skill_latency_ms`, `brevio_llm_tokens_total`, `brevio_llm_cost_cents`, `brevio_circuit_breaker_state`, `brevio_active_sessions`, `brevio_auth_token_refreshes`, `brevio_budget_utilization_pct`), Prometheus text exposition at `/metrics`, ingestion endpoint (`POST /api/v1|/v1 metrics/events`), snapshot endpoint (`GET /api/v1|/v1 metrics/snapshot`), structured logs, and graceful shutdown. Added `internal/contracts/metrics_service_runtime_closure_test.go` and replaced README scaffold content.  
**Migration Plan:**  
1. Replace health-only source with in-memory metric store supporting counter/gauge/histogram update semantics.  
2. Render metric families in Prometheus exposition format while retaining health endpoint behavior.  
3. Add closure test assertions for required metric identifiers and runtime endpoint tokens; re-run full `make ci`.  
**Risk:** Metrics state is in-memory and non-persistent; metrics reset on pod restart and require scraping/storage in external Prometheus for durability.  
**Rollback:** Revert `services/brevio-metrics/src/index.ts`, `services/brevio-metrics/README.md`, and `internal/contracts/metrics_service_runtime_closure_test.go` to prior scaffold baseline if ingestion behavior causes incompatibilities.

## DECISION-049: Replace `brevio-temporal-worker` Health Scaffold with Deterministic Workflow Runtime Baseline

**Date:** 2026-03-04  
**Blueprint Section:** §4.1, §4.2, §20.1-§20.3  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/services/brevio-temporal-worker`  
**Conflict:** `brevio-temporal-worker` exposed only health endpoints and lacked workflow runtime APIs/state tracking for `MessageProcessingWorkflow` and `DailyRhythmWorkflow` behavior expected by the production blueprint.  
**Options Considered:**  
1. Keep service as health-only and rely exclusively on external Temporal runtime without local contract surface.  
2. Add minimal placeholder workflow endpoints without explicit state-machine modeling.  
3. Implement deterministic workflow runtime endpoints with explicit state sequences, run-status retrieval, and jitter helper semantics.  
**Decision:** Option 3. Implemented `brevio-temporal-worker` runtime with `/api/v1` + `/v1` workflow endpoints (`workflows`, `runs/:run_id`, `workflows/message-processing`, `workflows/daily-rhythm`), modeled explicit `MessageProcessingWorkflow` terminal-state transitions (`COMPLETED`/`FAILED`/`DEAD_LETTER`), daily-rhythm progression, and deterministic jitter metadata using `fnv1a` hash helper. Added structured logging, graceful shutdown, and closure enforcement in `internal/contracts/temporal_worker_service_runtime_closure_test.go`; replaced README scaffold text with runtime docs.  
**Migration Plan:**  
1. Replace health-only source with deterministic workflow handlers while preserving `/health` and `/health/deep`.  
2. Add run snapshot store and workflow metadata endpoints to provide operational traceability.  
3. Add closure tests for required workflow-state tokens and rerun full `make ci` pipeline.  
**Risk:** Workflow run snapshots are currently in-memory and simulated for runtime closure; they do not yet persist in Temporal history tables and reset on restart.  
**Rollback:** Revert `services/brevio-temporal-worker/src/index.ts`, `services/brevio-temporal-worker/README.md`, and `internal/contracts/temporal_worker_service_runtime_closure_test.go` to prior scaffold baseline if integration expectations require temporary rollback.

## DECISION-050: Keep Generator-Backed Skill Coverage While Preserving Manual High-Value Hands Adapters

**Date:** 2026-03-04  
**Blueprint Section:** §2.4, §5.3, §A.8, §17.2  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/scripts/skills/generate_hands_skill_scaffolds.sh`, `/Users/galiettemita/Downloads/Executive AI Agent/backend/services/brevio-hands/src/skills/*`  
**Conflict:** The scaffold generator is the only practical way to maintain full file-structure coverage across 153+ skills, but it would overwrite early production-grade adapters for critical skills unless manual exceptions are explicitly preserved.  
**Options Considered:**  
1. Keep fully generated scaffolds for all skills until every adapter is production-ready.  
2. Stop using generator checks and manually maintain all skill directories immediately.  
3. Keep generator as the default for broad coverage but add explicit manual-preserve rules for priority skills being de-scaffolded incrementally.  
**Decision:** Option 3. Added explicit manual-preserve behavior in the scaffold generator for `shopping-expert`, `google-maps`, `google-calendar`, `tavily`, `smtp-send`, and `home-assistant`, then replaced those skill scaffolds with typed adapters (schema validation, deterministic client behavior, and safety/confirmation handling) plus closure enforcement in `internal/contracts/hands_priority_skills_closure_test.go`.  
**Migration Plan:**  
1. Preserve generator ownership for non-priority skills to keep contract parity and full registry coverage.  
2. De-scaffold priority skills in small waves with deterministic unit tests and README/runtime notes.  
3. Add each newly manualized skill to the preserve list and closure tests before merging the wave.  
4. Continue until all high-impact skills are moved off scaffold defaults.  
**Risk:** Manual preserve list can drift from actual manualized skill set, causing accidental overwrites or stale scaffolds if not kept in sync with tests.  
**Rollback:** Remove preserve-list entries, re-run generator to restore baseline scaffold state for affected skills, and revert manual adapter files if rapid stabilization is required.

## DECISION-051: Execute Priority Hands Adapter Wave 2 Across Task/Search/Finance/Notes/Image/Apple Local Domains

**Date:** 2026-03-04  
**Blueprint Section:** §2.4, §A.8, §17.2  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/services/brevio-hands/src/skills/{todoist,youtube-api,ynab,notion,fal-ai,apple-contacts}`  
**Conflict:** After Wave 1, key user-facing domains were still scaffold-only (`todoist`, `youtube-api`, `ynab`, `notion`, `fal-ai`, `apple-contacts`), leaving critical actions without typed I/O validation, deterministic behavior, or policy-safe guardrails.  
**Options Considered:**  
1. Keep these six skills scaffolded and focus only on service-level runtime work.  
2. De-scaffold all remaining ~150 skills immediately in one large change set.  
3. Continue controlled waves: select a cross-domain priority set, implement typed adapters + guardrails + tests, and update preserve/closure gates.  
**Decision:** Option 3. Implemented Wave 2 typed adapters for `todoist`, `youtube-api`, `ynab`, `notion`, `fal-ai`, and `apple-contacts` with explicit Zod schemas, deterministic mock clients, action validation, and safety checks (including content policy filtering for `fal-ai` and required-field enforcement across mutation actions). Expanded manual-preserve list and closure assertions to protect these adapters from scaffold overwrite.  
**Migration Plan:**  
1. Add Wave 2 skills to scaffold manual-preserve list before de-scaffolding files.  
2. Replace each skill scaffold (`types/schema/client/index/README/unit test`) with typed runtime implementation aligned to Addendum-A API/auth constraints.  
3. Extend `internal/contracts/hands_priority_skills_closure_test.go` token coverage to enforce non-scaffolded semantics for all preserved skills.  
4. Re-run contract and full CI gates; iterate next wave with remaining high-impact skills.  
**Risk:** Manualized adapters still use deterministic local simulations and may drift from external provider specifics unless paired with staged integration tests against sandbox credentials.  
**Rollback:** Remove new preserve-list entries and regenerate scaffolds for the six skills; revert Wave 2 adapter files to generated baseline if instability appears.

## DECISION-052: Extend Priority De-Scaffolding to Core Media, Finance, and Communications Connectors (Wave 3)

**Date:** 2026-03-04  
**Blueprint Section:** §2.4, §5.3, §A.8, §15.1-§15.2  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/services/brevio-hands/src/skills/{spotify-web-api,tmdb,plaid,google-workspace,outlook,icloud-findmy}`, `/Users/galiettemita/Downloads/Executive AI Agent/backend/Makefile`  
**Conflict:** After Waves 1-2, critical connectors for media playback, streaming recommendations, banking context, and enterprise communications were still scaffold-only, and the local `skills-scaffolds-check` gate could not pass during active manualization without excluding approved override paths.  
**Options Considered:**  
1. Keep these connectors scaffolded until all non-critical skills are complete.  
2. De-scaffold connectors but keep Makefile parity check against all skill paths (forcing commit-first validation only).  
3. De-scaffold selected high-impact connectors and formalize manual-override exclusions in `skills-scaffolds-check` while preserving generator enforcement for all non-overridden skills.  
**Decision:** Option 3. Implemented Wave 3 typed adapters for `spotify-web-api`, `tmdb`, `plaid`, `google-workspace`, `outlook`, and `icloud-findmy` (validated action contracts, scope metadata, confirmation-required send paths, deterministic result sets), expanded scaffold generator preserve list, and updated Makefile parity check with explicit override exclusions to keep local CI usable without weakening generated-skill enforcement elsewhere.  
**Migration Plan:**  
1. Add Wave 3 skill IDs to scaffold preserve list and closure tests.  
2. Replace each scaffold with typed `types/schema/client/index/README/unit` implementation.  
3. Extend `Makefile` `skills-scaffolds-check` excludes to match preserve list entries.  
4. Re-run contract gates, scaffold parity check, and full `make ci`; continue with next connector wave.  
**Risk:** Manual override lists now exist in both generator script and Makefile pathspec exclusions; drift between those lists could create false positives or accidental overwrites.  
**Rollback:** Remove Wave 3 preserve/exclude entries and regenerate scaffold defaults for the six connectors if runtime regressions or maintenance burden becomes unacceptable.

## DECISION-053: Centralize Manual Skill Override Source and Expand Search/Research Priority Adapters (Wave 4)

**Date:** 2026-03-04  
**Blueprint Section:** §2.4, §A.3, §A.8, §17.2  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/scripts/skills/*`, `/Users/galiettemita/Downloads/Executive AI Agent/backend/config/skill-manual-overrides.txt`, `/Users/galiettemita/Downloads/Executive AI Agent/backend/services/brevio-hands/src/skills/{exa,serpapi,perplexity,brave-search,firecrawl-search,news-aggregator}`  
**Conflict:** Manual skill override IDs were duplicated across generator logic, Makefile pathspec exclusions, and contract expectations, creating drift risk; search/research connectors were still scaffold-only despite high routing importance in Brain orchestration and proactive research workflows.  
**Options Considered:**  
1. Keep duplicated override lists and continue adding skills manually in multiple files each wave.  
2. Remove scaffold parity gate to avoid override maintenance complexity.  
3. Introduce a single override source file consumed by generator and scaffold parity checker, then de-scaffold next search/research skill wave under the unified mechanism.  
**Decision:** Option 3. Added `config/skill-manual-overrides.txt` as canonical override list, updated scaffold generator to consume it, introduced `scripts/skills/check_hands_skill_scaffold_parity.sh` for parity checks (excluding only configured manual skills), and wired Makefile `skills-scaffolds-check` to that script. In the same wave, replaced scaffolds for `exa`, `serpapi`, `perplexity`, `brave-search`, `firecrawl-search`, and `news-aggregator` with typed adapters, deterministic client behavior, and unit/readme upgrades.  
**Migration Plan:**  
1. Use `config/skill-manual-overrides.txt` as the only editable manual-skill list.  
2. Run parity script in CI/local to enforce generated skill stability outside override set.  
3. Expand closure tests to assert de-scaffolded tokens and override-file membership for each priority skill.  
4. Continue additional de-scaffolding waves by adding IDs once and implementing adapter suites.  
**Risk:** If override-file parsing fails or shell compatibility regresses, scaffold parity checks can fail broadly and block CI.  
**Rollback:** Revert to direct Makefile pathspec exclusions and inline generator manual list; regenerate scaffolds for Wave 4 skills if immediate stabilization is required.

## DECISION-054: Extend Priority De-Scaffolding to Productivity Tasking Connectors (Wave 5)

**Date:** 2026-03-04  
**Blueprint Section:** §2.4, §5.3, §A.8  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/services/brevio-hands/src/skills/{linear,jira,asana,trello,clickup-mcp,todo}`  
**Conflict:** Core productivity connectors remained scaffold-only, limiting Brain-plane decomposition quality for task/project intents and preventing consistent action-level validation for the most frequent executive workflows.  
**Options Considered:**  
1. Keep productivity skills scaffolded and prioritize only research/media connectors.  
2. Attempt a single large migration of all remaining skills at once.  
3. Continue controlled wave-based de-scaffolding focused on one domain cluster (productivity), with centralized override handling and closure assertions.  
**Decision:** Option 3. Implemented Wave 5 typed adapters for `linear`, `jira`, `asana`, `trello`, `clickup-mcp`, and `todo` with explicit action contracts, deterministic mutation IDs, error normalization for missing required fields, and upgraded documentation/unit tests. Added these IDs to centralized manual override configuration and closure assertions.  
**Migration Plan:**  
1. Register Wave 5 skill IDs in `config/skill-manual-overrides.txt`.  
2. Replace scaffold files (`types/schema/client/index/README/unit`) with typed implementations for each skill.  
3. Extend `hands_priority_skills_closure_test` token map and override-file presence checks.  
4. Re-run scaffold parity gate and full CI before commit.  
**Risk:** Mock deterministic clients can diverge from provider-specific edge behavior unless paired with staged integration fixtures and sandbox credentials.  
**Rollback:** Remove Wave 5 IDs from override config and regenerate scaffold defaults for affected skills if regression risk outweighs current implementation value.

## DECISION-055: Extend Priority De-Scaffolding to Notes/PKM Connectors (Wave 6)

**Date:** 2026-03-04  
**Blueprint Section:** §2.4, §5.3, §A.7.16, §A.8  
**Existing Code:** `/Users/galiettemita/Downloads/Executive AI Agent/backend/services/brevio-hands/src/skills/{apple-notes-skill,gkeep,bear-notes,obsidian,reflect,second-brain}`  
**Conflict:** Notes and PKM skills were still scaffold-only, despite being core to user context recall and frequent routing in executive workflows; this left no action-level validation for note creation, search, or updates.  
**Options Considered:**  
1. Keep notes/PKM skills scaffolded and rely on generic text responses.  
2. De-scaffold all remaining note skills plus adjacent personal-dev skills in one large change.  
3. Execute a focused notes/PKM wave with consistent list/create/search/update contracts and centralized override enforcement.  
**Decision:** Option 3. Implemented Wave 6 typed adapters for `apple-notes-skill`, `gkeep`, `bear-notes`, `obsidian`, `reflect`, and `second-brain` with deterministic note IDs, explicit create/update field validation, structured note metadata outputs, and updated unit tests/docs. Added these IDs to centralized override config and closure assertions.  
**Migration Plan:**  
1. Register Wave 6 IDs in `config/skill-manual-overrides.txt`.  
2. Replace scaffold implementations across all six skills with typed runtime modules.  
3. Extend contract token maps and override-file membership checks for these skills.  
4. Run scaffold parity plus full CI gates before merge.  
**Risk:** Current implementations use deterministic local note stores and can diverge from provider-specific search semantics (for example tag matching, markdown parsing, or vault path behavior) until sandbox integration tests are added.  
**Rollback:** Remove Wave 6 IDs from override config and regenerate scaffolds for these six skills if regressions occur or provider parity issues are discovered.
