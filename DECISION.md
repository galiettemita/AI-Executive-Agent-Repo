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
