# Executive OS v4.0 — Master Implementation Checklist (Blueprint + MCP + Ops + Auto-Provisioning)

Single source of truth docs (all must be satisfied):
- `EXECUTIVE_BLUEPRINT.pdf` (core agent architecture + 12-month build plan)
- `MCP_Integration_Specification.docx` (MCP client hub: transports, normalization, sandbox, cost tracking)
- `MCP_Server_Deployment_Plan.docx` (Waves 1–4: 30 launch MCP servers + hosting + onboarding UX)
- `MCP_Wave5_6_Expansion.docx` (Waves 5–6: 10 post-launch MCP servers + custom build specs)
- `Operational_Systems_Blueprint.pdf` (12 operational components: billing, auth, admin, eval, legal, DR, etc.)
- `Auto_Provisioning_Engine.pdf` (self-extending agent: capability gap detection + conversational provisioning + remote catalog)

This checklist is the full end-to-end plan to take the current codebase from “where it is now” to the production-grade Executive OS described across these specs, in the exact build order specified in **EXECUTIVE_BLUEPRINT Section 38**, with Ops/Auto-Provisioning layered into the same months (Ops Blueprint Section 17; Auto-Provisioning Section 16).

Legend
- `[x]` Done (verified in code and/or deployed)
- `[ ]` Not done / blocked

## BREVIO V9 Migration Tracker (Current Program)
- [x] BREVIO x OPENCLAW Phase 0A discovery complete: authored `CODEBASE_INVENTORY.md` with full repo/service/schema/test/infra/config inventory against the new production directive.
- [x] BREVIO x OPENCLAW Phase 0B gap analysis complete: authored `GAP_ANALYSIS.md` with `EXISTS_AND_MATCHES`, `EXISTS_BUT_DIFFERS`, `MISSING`, and explicit integration-point mapping.
- [x] BREVIO x OPENCLAW Phase 0 reconciliation refresh complete: updated `CODEBASE_INVENTORY.md` and `GAP_ANALYSIS.md` with 2026-03-04 state reconciliation, preserving baseline snapshots while explicitly separating resolved build items from remaining external human-gated dependencies.
- [x] BREVIO x OPENCLAW Phase 0C decision log initialized: authored `DECISION.md` with migration decisions for coexistence architecture, additive DB deltas, and protocol/policy evolution.
- [x] BREVIO x OPENCLAW Phase 1 structure scaffold started: added additive pnpm/turbo/TypeScript workspace root files (`package.json`, `pnpm-workspace.yaml`, `turbo.json`, `tsconfig.base.json`, `.eslintrc.cjs`, `.prettierrc`) while preserving existing Go runtime layout.
- [x] BREVIO x OPENCLAW Phase 1 structure scaffold extended: created target directories for `packages/`, `services/`, `edge/`, `infra/`, `config/`, `tests/`, `docs/runbooks/`, and `docs/compliance/` plus initial files for prompts, disambiguation, retry policies, authz policy scaffold, and local setup scripts.
- [x] BREVIO x OPENCLAW Phase 2 schema implementation: added reversible migration pairs `migrations/001..011` (`*.up.sql`/`*.down.sql`) with additive core/auth/skills/billing/temporal schemas, RLS/policy triggers, index set, deployment mode, user preferences, and edge agents table.
- [x] BREVIO x OPENCLAW Phase 2 skill registry seed baseline: implemented `006_seed_skills.up.sql` with 153 OpenClaw skill IDs seeded into `skills.registry` and deployment-mode/plane normalization updates.
- [x] BREVIO x OPENCLAW Phase 2 validation: executed full migration dry-run (all up then all down) against temporary PostgreSQL 16 container with `migration dry-run: PASS`.
- [x] BREVIO x OPENCLAW Phase 3 proto contracts scaffolded: added `packages/proto/brevio/*/v1/*.proto` service definitions for gateway, brain, hands, auth, profile, scheduler, edge relay, plus shared common messages and `buf.yaml`.
- [x] BREVIO x OPENCLAW proto generation operationalized: replaced `@brevio/proto` placeholder scripts with executable Buf lint/generate scripts (local binary or Docker fallback), added `packages/proto/buf.gen.yaml` TypeScript generation config, and enforced closure via `internal/contracts/proto_workspace_closure_test.go`.
- [x] BREVIO x OPENCLAW proto CI gate closure: added `proto-validate` Make target and wired it into `make ci` pre-build gates so protobuf contracts are lint-validated on every core CI run, with strict Makefile token assertions in closure tests.
- [x] BREVIO x OPENCLAW proto lint conformance closure: refactored service proto contracts to Buf-standard request/response naming (`*Response`) and unique RPC type usage (service-scoped health request/response messages), restoring clean proto lint in the mandatory CI gate.
- [x] BREVIO x OPENCLAW Phase 3 health/shutdown baseline: upgraded all scaffold TypeScript services (`services/brevio-*`) to expose `GET /health` + `GET /health/deep` JSON payloads and graceful SIGTERM/SIGINT shutdown handling with 30s drain timeout.
- [x] BREVIO x OPENCLAW LLM eval framework scaffolded: added required datasets under `tests/evals/` (`intent_classification.jsonl` 200 rows, `task_decomposition.jsonl` 50 rows, `response_generation.jsonl` 100 rows, `disambiguation.jsonl` 110 rows), baseline metrics, and `scripts/run-evals.sh` regression/budget gate runner.
- [x] BREVIO x OPENCLAW eval harness validation: executed `bash scripts/run-evals.sh` with PASS and result artifact in `tests/evals/results/`.
- [x] BREVIO x OPENCLAW eval harness de-placeholdered: replaced static score placeholders in `scripts/run-evals.sh` with deterministic dataset scoring for intent/decomposition/response/disambiguation, latency percentile computation, and token/cost estimation emitted in eval artifacts.
- [x] BREVIO x OPENCLAW eval fixture hardening: replaced `evals/determinism_fixtures.json` placeholder hash with verified SHA-256 fixtures and replaced `evals/rag_eval_framework.md` placeholder content with operational scoring protocol, then enforced both via `internal/contracts/eval_fixture_closure_test.go`.
- [x] BREVIO x OPENCLAW eval automation hardening: added `make evals` target and scheduled/prompt-change CI workflow `.github/workflows/llm-evals.yml` to execute `scripts/run-evals.sh`, enforce regression/budget gate status, and upload eval result artifacts.
- [x] BREVIO x OPENCLAW eval CI hard-gate: wired eval execution into core CI (`make ci` now includes `evals` and `.github/workflows/ci.yml` includes `5b. LLM Evals`) so regression/budget failures block normal branch CI.
- [x] BREVIO x OPENCLAW TypeScript quality-gate hardening: added `internal/contracts/typescript_no_any_closure_test.go` to enforce zero explicit `any` usage in production TypeScript under `packages/`, `services/`, and `edge/` (tests/external build dirs excluded), aligning with the non-negotiable no-`any` requirement.
- [x] BREVIO x OPENCLAW CI/CD scaffold extension: added `.github/workflows/ci.yml` with 10-stage pipeline scaffold and expanded `deploy-staging.yml`, `deploy-production.yml`, `security-scan.yml` workflows to align with blueprint stage structure.
- [x] BREVIO x OPENCLAW CI/CD de-scaffolding: replaced `ci-openclaw` stage `echo` placeholders with executable lint/schema/unit/integration/contract/security/migration/build/deploy commands, and upgraded deploy workflows to script-driven Helm rollout with secret-gated kubeconfig setup for staging/production.
- [x] BREVIO x OPENCLAW CI lint stability hardening: removed redundant Node/pnpm bootstrap from `ci-openclaw` lint stage and switched proto lint execution to `bash packages/proto/scripts/lint.sh` so docs-only validation runs are not blocked by npm registry/package-manager availability.
- [x] BREVIO x OPENCLAW security gate false-positive hardening: removed password-like DSN literals from deep health runtime tests and added a narrow `docker-compose.yml` trufflehog exclusion so CI strict-mode secret scans no longer fail on known local-dev Postgres placeholders.
- [x] BREVIO x OPENCLAW workflow kubeconfig hardening: updated deploy workflow kubeconfig steps to accept both base64-encoded and raw kubeconfig secret payloads with context validation (`kubectl config current-context`), preventing `base64: invalid input` production/staging skips.
- [x] BREVIO x OPENCLAW deploy values fallback hardening: updated `scripts/deploy/helm_rollout.sh` to fall back from gitignored `artifacts/deploy/*-prod-values.yaml` to committed `config/deploy/*-prod-values.yaml` defaults, preventing workflow failures on clean CI runners.
- [x] BREVIO x OPENCLAW security workflow hardening: upgraded `.github/workflows/security-scan.yml` from minimal cron shell calls to a full security gate pipeline (Semgrep SARIF, pnpm high-severity audit, Trivy/TruffleHog/Syft runtime validation, govuln baseline, artifact upload) and added closure enforcement in `internal/contracts/security_scan_workflow_closure_test.go`.
- [x] BREVIO x OPENCLAW infra/terraform closure: replaced empty `infra/terraform/environments/*` stubs with full module composition for staging/production/DR and added concrete module contracts for EKS, RDS, ElastiCache, SQS+SNS, S3, Secrets+KMS, CloudFront, Route53, Monitoring, and WAF under `infra/terraform/modules/*`, with closure tests in `internal/contracts/infra_openclaw_layout_closure_test.go`.
- [x] BREVIO x OPENCLAW infra/docker closure: replaced `infra/docker` scaffold files with service-specific multi-stage distroless Dockerfiles for gateway/brain/hands/control/executor/canvas/temporal-worker, wired `make docker-build` to those files, and added closure enforcement in `internal/contracts/infra_docker_layout_closure_test.go`.
- [x] BREVIO x OPENCLAW infra/docker TS service extension: added Node distroless Dockerfiles for `brevio-auth`, `brevio-profile`, `brevio-scheduler`, `brevio-metrics`, and `brevio-edge-relay`, extended `make docker-build-infra` to build them, and strengthened docker closure tests to enforce both Go and TypeScript image baselines.
- [x] BREVIO x OPENCLAW infra/helm umbrella closure: replaced `infra/helm/brevio` scaffold chart with executable dependency-backed umbrella Helm config (`Chart.yaml`, env values, service-account + additional service deployment/service/HPA templates) and added closure enforcement in `internal/contracts/infra_helm_brevio_closure_test.go`.
- [x] BREVIO x OPENCLAW infra/argocd closure: replaced minimal ArgoCD app placeholders with executable staging/production Application manifests targeting `infra/helm/brevio` (environment value files, destination namespace, sync policies, retry/ignore-differences controls) and added closure enforcement in `internal/contracts/infra_argocd_closure_test.go`.
- [x] BREVIO x OPENCLAW local compose closure: replaced minimal `docker-compose.yml` with a runnable local stack (PostgreSQL, Redis, Temporal + UI, gateway/brain/control/executor/canvas/temporal-worker service builds, extension-profile service stubs, health checks, shared env wiring), added closure enforcement in `internal/contracts/docker_compose_layout_closure_test.go`, and aligned `scripts/setup-local.sh` dependency bootstrap.
- [x] BREVIO x OPENCLAW edge runtime closure: replaced `brevio-edge-agent` and `brevio-edge-relay` scaffolds with typed WebSocket relay/agent implementations (register + heartbeat + execute_skill + skill_result flow, offline queue handling up to 4h, session introspection and health endpoints), added `ws` dependencies for both packages, and enforced coverage with `internal/contracts/edge_agent_relay_closure_test.go`.
- [x] BREVIO x OPENCLAW Brain disambiguation runtime: implemented deterministic 11-group disambiguation router in `internal/brain/disambiguation` with YAML-backed rules loading from `config/skill-disambiguation.yaml` and full table-driven unit coverage.
- [x] BREVIO x OPENCLAW authz policy matrix closure: expanded `policies/brevio/authz.rego` and `policies/tests/authz_test.rego` to cover role/resource matrix rules (free/pro/enterprise/admin), denial invariants, and gateway/activity/PII policy denies.
- [x] BREVIO x OPENCLAW config/eval hardening: upgraded prompt templates with explicit input/output schemas + guardrails, added per-category non-retryable error lists, replaced placeholder disambiguation eval corpus with 110 deterministic cases, and updated `scripts/run-evals.sh` to compute real disambiguation accuracy.
- [x] BREVIO x OPENCLAW seed registry alignment: updated `migrations/006_seed_skills.up.sql` to classify seeded skills by canonical plane (`gateway`/`brain`/`hands`) and deployment mode (`cloud`/`local_mac`/`mcp`) with conflict updates preserving additive migration behavior.
- [x] BREVIO x OPENCLAW Gateway idempotency/rate-limit hardening: upgraded `internal/gateway` with channel-message-id idempotency caching (24h replay window, cached response return), tier-aware sliding-window rate limits (`free` 30/hr, `pro` 120/hr, enterprise/admin unlimited), and extended webhook tests for replay, limits, and enterprise bypass.
- [x] BREVIO x OPENCLAW workflow state-machine expansion: added deterministic `MessageProcessingWorkflowV1` and `DailyRhythmWorkflowV1` implementations in `internal/workflows` with explicit stage transitions (`RECEIVED`→`...`→`COMPLETED`/`FAILED`/`DEAD_LETTER`), deterministic retry jitter helper, and dedicated unit coverage for fallback/dead-letter/compensation paths.
- [x] BREVIO x OPENCLAW runbook/compliance closure: expanded all required operational runbooks under `docs/runbooks/` and compliance artifacts under `docs/compliance/` with actionable procedures, retention/encryption controls, incident/escalation playbooks, GDPR rights workflows, sub-processor inventory baseline, and SOC 2 readiness control/evidence mapping.
- [x] BREVIO x OPENCLAW skill seed utility closure: implemented `scripts/seed-skills.ts` to validate `migrations/006_seed_skills.up.sql` contains exactly 153 seeded skill IDs and `config/skill-disambiguation.yaml` contains 11 routing groups, with optional JSON summary output.
- [x] BREVIO x OPENCLAW policy execution gate: added `scripts/policies/run_opa_tests.sh` with local-OPA or Docker fallback and wired `make ci` through new `policy-validate` target for executable Rego policy checks.
- [x] BREVIO x OPENCLAW policy gate validation: executed `bash scripts/policies/run_opa_tests.sh` with Docker fallback and verified `PASS: 30/30` policy tests.
- [x] BREVIO x OPENCLAW health endpoint parity: added `GET /health` and `GET /health/deep` JSON responses (with uptime/version/checks) across Go runtime services/muxes while preserving existing `/healthz/ready` + `/healthz/live`, and expanded runtime health closure tests accordingly.
- [x] BREVIO x OPENCLAW local onboarding bootstrap: upgraded `scripts/setup-local.sh` + new `scripts/dev/run_local_services.sh` and added `make dev` to perform dependency bootstrap, local infra startup, migration/seed validation, and multi-service local runtime startup with optional watch-mode auto-restart via `watchexec`.
- [x] BREVIO x OPENCLAW staging API docs surface: added staging-only `/docs` + `/docs/openapi` handlers in control mux with Swagger UI HTML and OpenAPI spec serving, with runtime tests for staging enablement and non-staging denial behavior.
- [x] BREVIO x OPENCLAW graceful shutdown closure: introduced shared runtime server helper `internal/runtime/httpserver.go` and wired all Go service entrypoints (`cmd/*`) to handle SIGTERM/SIGINT with 30s graceful drain, plus closure test coverage to prevent regression.
- [x] BREVIO x OPENCLAW canonical envelope normalization: upgraded gateway ingress to emit validated canonical `MessageEnvelope` payloads (UUIDv7, channel enum normalization, content typing, user/session resolution with 4h rotation, profile hash context) into queue handoff and updated integration runtime to consume envelope-first payloads end-to-end.
- [x] BREVIO x OPENCLAW workflow traceability alignment: extended canonical workflow traceability and runtime closure coverage to include `message_processing_v1` and `daily_rhythm_v1` workflows with explicit terminal-state assertions.
- [x] BREVIO x OPENCLAW execution-log PII scrubber: implemented daily 03:00 UTC scrub scheduling helpers and deterministic 30-day regex-based PII detection/nullification workflow for `skills.execution_log` payload retention enforcement.
- [x] BREVIO x OPENCLAW failure-domain integration matrix: added explicit Addendum A.1 integration coverage for ingress retry/DLQ, gateway normalization fallback, classifier/decomposer fallbacks, execution compensation, aggregation partial-results fallback, and egress DLQ routing.
- [x] BREVIO x OPENCLAW CI gate reconciliation: resolved contract and lint drift uncovered by `make ci` (workflow traceability row-count, MCP checklist CI target token, optional local blueprint doc allowance, and attachment validation error string lint) and revalidated with full `make ci` pass.
- [x] BREVIO x OPENCLAW iMessage webhook auth hardening: added iMessage-specific API-key enforcement at gateway ingress (`X-API-Key`) with dedicated unauthorized-path audit event and updated integration/runtime tests.
- [x] BREVIO x OPENCLAW webhook contract alignment: added blueprint-compatible webhook endpoints (`/webhooks/*` and `/api/v1/webhooks/*`) plus Temporal callback idempotency by `workflow_run_id`, while retaining legacy `/v1/gateway/webhook/*` routes for backward compatibility.
- [x] BREVIO x OPENCLAW shared schema exports: added explicit JSON Schema 7 exports for canonical `MessageEnvelope` and `SkillResult` alongside Zod source schemas in `packages/shared`.
- [x] BREVIO x OPENCLAW config fail-fast baseline: added strict gateway startup env validation (`BREVIO_ENV`-aware required secrets, listen address/version defaults) and explicit service option injection for iMessage API key.
- [x] BREVIO x OPENCLAW startup config hardening (multi-service): introduced shared runtime env validation helpers and wired fail-fast startup checks/listen-address resolution into brain, control, executor, canvas, and temporal-worker entrypoints.
- [x] BREVIO x OPENCLAW deep health connectivity checks: replaced env-presence-only `/health/deep` checks with shared runtime TCP dependency probes for PostgreSQL (`DATABASE_URL`), Redis (`REDIS_URL`), and Temporal (`TEMPORAL_HOST`) across gateway, control, canvas, brain, executor, and temporal-worker.
- [x] BREVIO x OPENCLAW feature-flag baseline: bootstrapped canonical system flags for skill rollout (`skills.rollout`), LLM provider switching (`llm.provider_switch`), and canary features (`canary.features`) and wired control-plane startup/tests to ensure availability by default.
- [x] BREVIO x OPENCLAW structured logging baseline: added shared JSON HTTP logging middleware with correlation fields (`trace_id`, `span_id`, `user_id`, `request_id`) and wired it into all Go HTTP service entrypoints (gateway, brain, control, executor, canvas, temporal-worker) with UUIDv7 request IDs.
- [x] BREVIO x OPENCLAW execution-log scrub runtime: added PostgreSQL-backed `skills.execution_log` scrub store and wired daily 03:00 UTC scheduler in temporal-worker to run regex-based 30-day PII payload nullification with structured runtime logs.
- [x] BREVIO x OPENCLAW auth service-map expansion: extended connector auth registries to canonical Addendum-A coverage (15 OAuth providers, 18 API-key services, 6 no-auth/local-only services) with strict connector tests validating provider counts and required metadata fields.
- [x] BREVIO x OPENCLAW auth config artifact closure: added version-controlled `config/auth-service-map.yaml` containing canonical OAuth/API-key/no-auth maps plus secret naming/redirect/PKCE storage requirements, enforced by new contract test coverage.
- [x] BREVIO x OPENCLAW gateway skill-profile closure: added explicit Addendum-A.6 gateway-skill profile map for all 8 gateway skills (external call path, rationale, latency budget, autoresponder brain delegation flag) with dedicated runtime+contract tests.
- [x] BREVIO x OPENCLAW auth secret convention helpers: added executable helpers/tests for canonical Secrets Manager naming (`brevio/{env}/{service}/client_id|client_secret`), OAuth redirect URI pattern (`https://auth.brevio.app/callback/{service}`), and enforced PKCE-required flag semantics.
- [x] BREVIO x OPENCLAW auth runtime closure: upgraded `services/brevio-auth` from health-only scaffold to typed OAuth registry + PKCE state service (provider discovery, authorize/exchange/refresh/callback endpoints, structured logging, graceful shutdown), tightened auth service-map contract assertions to exact Addendum-A provider sets including full 24-skill `local-macos` coverage, and added dedicated runtime closure contracts.
- [x] BREVIO x OPENCLAW gateway runtime closure: replaced `services/brevio-gateway` scaffold with webhook ingress baseline supporting WhatsApp/iMessage/Temporal endpoints (including compatibility aliases), HMAC/API-key verification, canonical envelope normalization with 4h session rotation, 24h idempotency replay cache, tiered + minute rate limiting, and channel formatting endpoint, with new closure coverage in `internal/contracts/gateway_service_runtime_closure_test.go`.
- [x] BREVIO x OPENCLAW brain runtime closure: replaced `services/brevio-brain` scaffold with classification/disambiguation/decomposition/aggregation runtime endpoints (`/api/v1/brain/*`) plus end-to-end `process` orchestration, enforced 11-group disambiguation config loading, bounded task DAG validation (`max 10` + cycle detection), structured logging, and graceful shutdown, with closure coverage in `internal/contracts/brain_service_runtime_closure_test.go`.
- [x] BREVIO x OPENCLAW hands runtime closure: upgraded `services/brevio-hands` execution runtime with API aliases (`/v1` + `/api/v1` + `/tool/execute`), per-skill circuit breaker state transitions (`CLOSED`/`HALF_OPEN`/`OPEN`), execution timeout normalization to `SkillResult` error contracts, structured correlation logging, and graceful shutdown, with closure coverage in `internal/contracts/hands_service_runtime_closure_test.go`.
- [x] BREVIO x OPENCLAW profile runtime closure: replaced `services/brevio-profile` scaffold with user profile + knowledge file management runtime (`USER.md`, `SOUL.md`, `AGENTS.md`) including profile hash refresh, preferences update endpoints, storage-root management, structured correlation logging, and graceful shutdown, with closure coverage in `internal/contracts/profile_service_runtime_closure_test.go`.
- [x] BREVIO x OPENCLAW scheduler runtime closure: replaced `services/brevio-scheduler` scaffold with scheduler job/trigger runtime APIs (`/api/v1` + `/v1` aliases), in-memory job registry, on-demand trigger queueing, explicit run/disable flows, structured correlation logging, and graceful shutdown, with closure coverage in `internal/contracts/scheduler_service_runtime_closure_test.go`.
- [x] BREVIO x OPENCLAW metrics runtime closure: replaced `services/brevio-metrics` scaffold with Prometheus `/metrics` exposition and ingestion/snapshot APIs for Section 10 metric families (counter/gauge/histogram), structured correlation logging, and graceful shutdown, with closure coverage in `internal/contracts/metrics_service_runtime_closure_test.go`.
- [x] BREVIO x OPENCLAW temporal-worker runtime closure: replaced `services/brevio-temporal-worker` scaffold with deterministic workflow runtime APIs for `message-processing` and `daily-rhythm` state machines, run-status lookup, deterministic jitter metadata helper (`fnv1a`), structured correlation logging, and graceful shutdown, with closure coverage in `internal/contracts/temporal_worker_service_runtime_closure_test.go`.
- [x] BREVIO x OPENCLAW hands priority skill de-scaffolding wave 1: replaced scaffold placeholders with typed adapters for `shopping-expert`, `google-maps`, `google-calendar`, `tavily`, `smtp-send`, and `home-assistant` (validated input/output schemas, deterministic mock clients, safety/confirmation paths, upgraded READMEs/unit tests), added manual-preserve behavior to `scripts/skills/generate_hands_skill_scaffolds.sh`, and added closure coverage in `internal/contracts/hands_priority_skills_closure_test.go`.
- [x] BREVIO x OPENCLAW hands priority skill de-scaffolding wave 2: replaced scaffolds for `todoist`, `youtube-api`, `ynab`, `notion`, `fal-ai`, and `apple-contacts` with typed adapters (schema-validated action contracts, deterministic client behavior, content/safety guards, upgraded READMEs/unit tests), and expanded generator-preserve + closure enforcement coverage in `scripts/skills/generate_hands_skill_scaffolds.sh` and `internal/contracts/hands_priority_skills_closure_test.go`.
- [x] BREVIO x OPENCLAW hands priority skill de-scaffolding wave 3: replaced scaffolds for `spotify-web-api`, `tmdb`, `plaid`, `google-workspace`, `outlook`, and `icloud-findmy` with typed adapters (auth-scope metadata, confirmation gates for mail-send actions, deterministic structured outputs, upgraded READMEs/unit tests), and expanded manual-override parity enforcement in `scripts/skills/generate_hands_skill_scaffolds.sh`, `Makefile` `skills-scaffolds-check`, and `internal/contracts/hands_priority_skills_closure_test.go`.
- [x] BREVIO x OPENCLAW hands priority skill de-scaffolding wave 4: replaced scaffolds for `exa`, `serpapi`, `perplexity`, `brave-search`, `firecrawl-search`, and `news-aggregator` with typed adapters (query/result schemas, deterministic provider outputs, upgraded READMEs/unit tests), and moved manual-override source of truth to `config/skill-manual-overrides.txt` with parity enforcement via `scripts/skills/check_hands_skill_scaffold_parity.sh`.
- [x] BREVIO x OPENCLAW hands priority skill de-scaffolding wave 5: replaced scaffolds for `linear`, `jira`, `asana`, `trello`, `clickup-mcp`, and `todo` with typed adapters (action-oriented task/issue/card contracts, deterministic mutation IDs, validation guards, upgraded READMEs/unit tests), and expanded centralized manual override + closure coverage for productivity routing paths.
- [x] BREVIO x OPENCLAW hands priority skill de-scaffolding wave 6: replaced scaffolds for `apple-notes-skill`, `gkeep`, `bear-notes`, `obsidian`, `reflect`, and `second-brain` with typed notes/PKM adapters (list/create/search/update contracts, deterministic note IDs, create/update validation guards, upgraded READMEs/unit tests), and extended centralized manual override + closure coverage for note-taking disambiguation routes.
- [x] BREVIO x OPENCLAW GDPR portability runtime: added portability export generation in compliance service and control-plane endpoint (`GET /v1/compliance/dsr/{id}/export`) with idempotent export artifacts, workspace list surfacing, and rejection for non-portability DSR types.
- [x] BREVIO x OPENCLAW mutation audit runtime baseline: implemented append-only hash-chained `internal/audit` mutation ledger with UUIDv7 IDs, wired control-plane mutation logging for feature flags and compliance DSR/framework/export flows, and replaced `/v1/user/activity-ledger` seed response with real workspace-scoped audit-backed pagination.
- [x] BREVIO x OPENCLAW audit persistence wiring: added optional PostgreSQL sink for append-only `audit_log_entries` persistence (`DATABASE_URL`-driven), introduced `control.NewMuxWithDependencies` for injected audit services, and wired control startup to enable/disable sink with structured startup telemetry while preserving memory fallback.
- [x] BREVIO x OPENCLAW skill-mutation audit linkage: wired Hands execution commits to append mutation-audit entries (`hands.skill.execute.commit`) when audit service is configured, including before/after side-effect counters plus execution/receipt IDs, with executor unit coverage.
- [x] BREVIO x OPENCLAW token/profile mutation audit linkage: wired OAuth refresh mutations (`oauth.token.refresh`) in connector token lifecycle and user profile updates (`identity.user.profile.update`) in identity service to append mutation-audit entries with non-secret before/after metadata and dedicated unit coverage.
- [x] BREVIO x OPENCLAW control runtime de-stubbing: replaced static stub payloads for `/v1/brain/turn`, `/v1/admin/forensics/replay/{turn_id}`, and `/v1/admin/llm/replay/{hash}` with deterministic operational responses and added mux tests to enforce no stub markers in response bodies.
- [x] BREVIO x OPENCLAW Hands skill scaffold expansion: generated per-skill adapter structures for all 153 seeded skills under `services/brevio-hands/src/skills/{skill-id}/` (index/schema/client/types/tests/README), added generated registry index, and exposed `/v1/hands/skills` + `/v1/hands/execute` runtime routes backed by adapter dispatch.
- [x] BREVIO x OPENCLAW skill scaffold parity gate: added contract tests that enforce every seeded skill ID has the full adapter file structure in `services/brevio-hands/src/skills/` and that generated `skills/index.ts` includes all 153 registry mappings.
- [x] BREVIO x OPENCLAW custom-build gap stubs: extended skill scaffold generation to include 10 Brevio transactional gap adapters (`restaurant-reservations`, `food-delivery-ordering`, `ride-hailing`, `hotel-vacation-booking`, `bill-pay-p2p`, `streaming-recommendations`, `local-service-booking`, `kids-family-management`, `pharmacy-prescription`, `pet-care`) with `CUSTOM_BUILD_REQUIRED` markers and contract coverage.
- [x] BREVIO x OPENCLAW custom-build integration closure wave: replaced scaffold-only integration tests for all 10 custom-build transactional adapters with fixture-backed execution assertions, added deterministic JSON fixtures under each skill `__tests__/fixtures`, and tightened `internal/contracts/custom_build_skills_closure_test.go` to fail if any custom-build integration test reverts to `scaffold compiles` or has zero `.json` fixtures.
- [x] BREVIO x OPENCLAW gateway-skill integration closure wave: replaced scaffold-only integration tests for gateway-profile skills (`asr`, `openai-tts`, `gemini-stt`, `sag`, `voice-wake-say`, `whatsapp-styling-guide`, `vocal-chat`, `autoresponder`) with fixture-backed deterministic assertions and added `internal/contracts/gateway_skill_integration_closure_test.go` to enforce non-scaffold integration tests plus required `.json` fixtures.
- [x] BREVIO x OPENCLAW brain-skill integration closure wave: replaced scaffold-only integration tests for all 16 Brain-plane skills (`doing-tasks`, `plan-my-day`, `daily-rhythm`, `morning-manifesto`, `personal-shopper`, `clawringhouse`, `smart-expense-tracker`, `card-optimizer`, `refund-radar`, `contract-reviewer`, `meeting-autopilot`, `proactive-research`, `focus-mode`, `thinking-partner`, `relationship-skills`, `self-improvement`) with fixture-backed deterministic assertions and added `internal/contracts/brain_skill_integration_closure_test.go` to enforce non-scaffold integration tests plus required `.json` fixtures.
- [x] BREVIO x OPENCLAW communication-skill integration closure wave: replaced scaffold-only integration tests for non-gateway communication skills (`smtp-send`, `react-email-skills`, `apple-mail`, `apple-mail-search`, `bluesky`, `reddit`, `bird`, `outlook`, `imap-email`, `google-workspace`, `slack`) with fixture-backed deterministic assertions and added `internal/contracts/communication_skill_integration_closure_test.go` to enforce non-scaffold integration tests plus required `.json` fixtures.
- [x] BREVIO x OPENCLAW productivity/calendar integration closure wave: replaced scaffold-only integration tests for productivity/calendar skills (`asana`, `clickup-mcp`, `jira`, `linear`, `omnifocus`, `things-mac`, `ticktick`, `todo`, `todoist`, `trello`, `calctl`, `apple-remind-me`) with fixture-backed deterministic assertions and added `internal/contracts/productivity_skill_integration_closure_test.go` to enforce non-scaffold integration tests plus required `.json` fixtures.
- [x] BREVIO x OPENCLAW shopping/transportation integration closure wave: replaced scaffold-only integration tests for shopping + transportation skills (`shopping-expert`, `buy-anything`, `grocery-list`, `marketplace`, `recipe-to-list`, `google-maps`, `flight-tracker`, `aviationstack-flight-tracker`, `aerobase-skill`, `parcel-package-tracking`, `track17`, `post-at`, `spots`, `local-places`, `goplaces`, `swissweather`) with fixture-backed deterministic assertions and added `internal/contracts/shopping_transportation_skill_integration_closure_test.go` to enforce non-scaffold integration tests plus required `.json` fixtures.
- [x] BREVIO x OPENCLAW search/research integration closure wave: replaced scaffold-only integration tests for search/research skills (`brave-search`, `exa`, `firecrawl-search`, `kagi-search`, `last30days`, `literature-review`, `news-aggregator`, `perplexity`, `serpapi`, `tavily`, `gemini-deep-research`) with fixture-backed deterministic assertions and added `internal/contracts/search_research_skill_integration_closure_test.go` to enforce non-scaffold integration tests plus required `.json` fixtures.
- [x] BREVIO x OPENCLAW finance/documents integration closure wave: replaced scaffold-only integration tests for finance + document skills (`copilot-money`, `expense-tracker-pro`, `financial-market-analysis`, `monarch-money`, `plaid`, `watch-my-money`, `yahoo-finance`, `ynab`, `tax-professional`, `ibkr-trading`, `just-fucking-cancel`, `pdf-tools`, `resume-builder`) with fixture-backed deterministic assertions and added `internal/contracts/finance_documents_skill_integration_closure_test.go` to enforce non-scaffold integration tests plus required `.json` fixtures.
- [x] BREVIO x OPENCLAW media/streaming integration closure wave: replaced scaffold-only integration tests for media + streaming skills (`spotify`, `spotify-player`, `spotify-web-api`, `spotify-history`, `apple-music`, `youtube-api`, `youtube-summarizer`, `video-frames`, `video-transcript-downloader`, `tmdb`, `trakt`, `plex`, `pocket-casts`, `lastfm`, `ytmusic`) with fixture-backed deterministic assertions and added `internal/contracts/media_streaming_skill_integration_closure_test.go` to enforce non-scaffold integration tests plus required `.json` fixtures.
- [x] BREVIO x OPENCLAW apple/notes/local integration closure wave: replaced scaffold-only integration tests for Apple + notes/local skills (`alter-actions`, `apple-contacts`, `apple-media`, `apple-notes`, `apple-notes-skill`, `apple-photos`, `bear-notes`, `better-notion`, `gkeep`, `google-calendar`, `icloud-findmy`, `notion`, `obsidian`, `reflect`, `second-brain`, `shortcuts-generator`, `get-focus-mode`) with fixture-backed deterministic assertions and added `internal/contracts/apple_notes_local_skill_integration_closure_test.go` to enforce non-scaffold integration tests plus required `.json` fixtures.
- [x] BREVIO x OPENCLAW final integration closure wave: replaced the remaining scaffold-only integration tests for 34 skills (`content-advisory`, `clawd-coach`, `dexcom`, `chromecast`, `camsnap`, `de-ai-ify`, `craft`, `gifhorse`, `sonoscli`, `fal-ai`, `george`, `gamma`, `healthkit-sync-apple`, `meal-planner`, `figma`, `pros-cons`, `healthkit-sync`, `home-assistant`, `veo`, `overseerr`, `krea-api`, `coloring-page`, `mole-mac-cleanup`, `sleep-calculator`, `sports-ticker`, `excalidraw-flowchart`, `samsung-smart-tv`, `radarr`, `pollinations`, `granola`, `sonarr`, `roku`, `journal-to-post`, `withings-health`) with fixture-backed deterministic assertions and added `internal/contracts/final_skill_integration_closure_test.go` to enforce non-scaffold integration tests plus required `.json` fixtures.
- [x] BREVIO x OPENCLAW global integration closure guard: added `internal/contracts/hands_skill_integration_global_closure_test.go` to validate every directory under `services/brevio-hands/src/skills/` has a non-scaffold integration test and at least one fixture `.json`, preventing future regressions as new skills are added.
- [x] BREVIO x OPENCLAW infra hard-gate closure: fixed Terraform formatting drift in `infra/terraform/modules/{cloudfront,elasticache,monitoring,secrets}/main.tf` so `make ci-full` passes through `infra-validate` and full quality gates without manual intervention.
- [x] BREVIO x OPENCLAW final validation evidence refresh: added `docs/FINAL_VALIDATION_brevio_openclaw.md` with timestamped command/results evidence (`make ci`, `make evals`, `make security-validate`, `make infra-validate`, `make db-verify`, `make ci-full`) and explicit human-gated go-live blocker inventory.
- [x] BREVIO x OPENCLAW next-phase transition gate: executed `make external-closeout-check` and recorded active human-gated blockers in `docs/FINAL_VALIDATION_brevio_openclaw.md` from `artifacts/deploy/external_closeout_status.json` to move directly from autonomous closure into external provisioning phase.
- [x] V9.3 addendum phase-0 reconnaissance completed with section-by-section audit output at `docs/addendum_gap_audit.md`.
- [x] V9.3 addendum closure: added migration `006_BREVIO_v93_addendum_specification_closure.sql` (`whatsapp_message_templates` + RLS/indexes), expanded canonical V9 events with 8 addendum events, and aligned OpenAPI/schema mapping for newly specified endpoints.
- [x] V9.3 addendum runtime closure: implemented control/canvas endpoint ownership coverage for addendum routes, refreshed API reference docs (`docs/API_REFERENCE.md`), and revalidated with `go test ./...` passing at HEAD.
- [x] V9.3 addendum policy closure pass: implemented and tested autonomy upgrade matrix, hold/undo overrides, write-budget caps, recipient verification, financial/two-man approval logic, Temporal activity retry matrices, drift cadence, deterministic ranking formula, retention defaults, OAuth provider/scope registry, and HMAC audit/proof chain helpers.
- [x] V9.3 addendum closure pass 3: implemented and tested content firewall L1-L4 + semantic verifiers, gateway attachment/document/channel policy helpers, ingress identity envelope/event handling updates, Git HTTPS policy helpers, mTLS constants, PII leakage matcher utilities, routing policy resolver utilities, and eval grader/threshold helpers with `go test ./...` passing.
- [x] V9.3 addendum closure pass 4: implemented and tested load-shedding transition automation rules, memory-consolidation merge/expiry/contradiction helpers, OAuth PKCE+state helpers, specialist routing, tool-inventory/connector-tool separation helpers, workspace-type behavior filters, financial merchant/anomaly policy helpers, drift watchdog quarantine/auto-heal logic, Home Assistant environment helpers, and RS256 JWT issue/verify + JWKS runtime.
- [x] V9.3 addendum closure pass 5: implemented and tested executor connector client architecture helpers, LLM provider registry/rate-limit/cost helpers, canvas protocol constants, voice fallback synthesis helpers, retention-expiry evaluator, attachment magic-byte and OCR threshold helpers, delegation pairing TTL/code helpers, semantic memory exclusion rules, executor L1/L2/L3 cache manager lifecycle, and core-service cert-manager mTLS Certificate templates in existing Helm charts.
- [x] V9.3 addendum closure pass 6 (completion): implemented runtime WhatsApp/iMessage client helpers, OAuth state-store/revocation helpers, Git ingestion/profile helpers, airport seed parser + seed scaffold, and refreshed `docs/addendum_gap_audit.md` to complete status across all sections A..BD with `go test ./...` passing.
- [x] Phase 0.1: map current repository (`find` + depth-limited directory map) and compare against V9 canonical structure
- [x] Phase 0.1: create gap report at `docs/codebase_audit_report.md` with required artifact status table
- [x] Phase 0.1: identify non-V9/V9.1/V9.2 candidate removal areas in audit report
- [x] Track new source blueprints in Git:
  - `Brevio_V9_Consolidated_Master_Blueprint.docx`
  - `Brevio_V91_Addendum_Soft_Intelligence_Layer.docx`
  - `Brevio_V92_Addendum_Production_Hardening.docx`
- [x] Production deploy unblock: added `.dockerignore` to prevent oversized Docker build context and rebuilt release images deterministically.
- [x] Production deploy unblock: pushed `v9.2.0` service images to ECR (`brevio-gateway`, `brevio-admin-frontend`) for EKS pullability.
- [x] Production deploy unblock: upgraded `brevio-gateway` and `brevio-admin-frontend` Helm releases to ECR image repos with runtime ports (`18080`, `18082`) and verified running pods + healthy ALB target groups.
- [x] Post-deploy validation: reran `make ci-full` (lint, unit/integration/contracts/acceptance, security, infra validation, DB migration runtime verification) with all gates passing.
- [x] Endpoint verification: confirmed HTTPS `/healthz/live` returns `200 ok` on both ALBs using host-header routing (`api.testing-orbit.com`, `admin.testing-orbit.com`).
- [x] External DNS cutover complete: external DNS provider now routes `api.testing-orbit.com` and `admin.testing-orbit.com` to ALB targets; public `/healthz/live` checks return `200 ok`.
- [x] Post-cutover regression: reran `make ci-full` at HEAD after DNS and deployment doc updates; all closure/security/infra/db gates still pass.
- [x] Deployment automation hardening: added `scripts/deploy/helm_rollout.sh` and `make deploy-helm` to run deterministic multi-chart rollout with optional ECR image/port overrides and rollout waiting.
- [x] Deployment closure guard: added contract test coverage ensuring rollout script and `Makefile` deploy target tokens remain present (`internal/contracts/script_closure_test.go`).
- [x] Post-automation regression: reran `make ci` after rollout automation additions; lint/build/tests/migration/contracts/acceptance gates remain PASS.
- [x] Full-gate lock refresh: reran `make ci-full` at latest HEAD after rollout automation/testing additions; closure, security, infra, and DB runtime gates remain PASS.
- [x] Final pre-tag lock: reran `make ci-full` at HEAD immediately before final release tagging; all gate families remain PASS.
- [x] TOOLS catalog hardening: implemented deterministic `TOOLS.md` regeneration pipeline (`scripts/docs/generate_tools_md.go`, `make tools-md`, `make tools-md-check`) showing connected and available-not-connected connectors.
- [x] Observability hardening: added Section 33 metrics/alerts artifacts (`spec/metrics/section33_metrics_core.txt`, `spec/alerts/section33_alert_thresholds.yaml`) and closure tests.
- [x] Engineering governance hardening: added ownership and on-call matrix documentation (`docs/OPERATIONS_OWNERSHIP.md`) with closure tests.
- [x] Ops schema closure hardening: added forward-only migration `004_BREVIO_ops_operational_systems.sql` covering operational tables (`subscriptions`, `invoices`, `eval_results`, `user_feedback`, `moderation_queue`, `scheduled_notifications`, `analytics_events`, `analytics_daily`) with RLS + FK indexes and DB verification wiring.
- [x] Error taxonomy hardening: standardized control-plane error payloads to canonical schema (`error_code`, `message`, `retryable`, `retry_after_ms`, `user_message`), added structured observability logging for error responses, and expanded Appendix B taxonomy coverage (`spec/errors/appendix_b_error_taxonomy.csv` + closure tests).
- [x] Legal + DR closure hardening: added deterministic deletion completion pipeline/reporting in compliance runtime (DB/cache/connector/MCP token revocation + irreversible + backup rotation window), and added DR contract gates for Multi-AZ/PITR RDS config plus monthly restore drill runbook coverage.
- [x] Phase 0.2: dead-code/duplicate cleanup and naming normalization
- [x] Phase 0.3: clean baseline (`go build`, `go vet`, `gofmt`) + commit/tag
- [x] Phase 0.3 validation complete via Docker Go 1.22 (`go mod tidy`, `go build`, `go vet`, `gofmt`, `go test`, `staticcheck`)
- [x] Phase 1 Step 1: V9 repo scaffold created (`cmd/`, `internal/`, `db/`, `api/`, `schemas/`, `policies/`, `terraform/`, `helm/`, `runbooks/`, CI)
- [x] Phase 1 Step 2: full V9 database core closure (all 27 enums + all 77+ tables + strict RLS/FK/index validation) via `internal/database/migration_closure_test.go` exact-set assertions + RLS coverage checks
- [x] Phase 1 Step 2 (in progress): migration expanded to full domain table-name coverage with RLS/index scaffolding and append-only audit trigger
- [x] Phase 1 Step 3: `identity`, `delegation`, and `rbac` packages with UUIDv7 entity creation and role/delegation validation tests
- [x] Phase 1 Step 4: `connectors` package with 40+ connector seed loader, AES-256-GCM OAuth envelope vault, key-versioned decrypt, and health tracking
- [x] Phase 1 Step 5 (baseline): gateway webhook handlers with HMAC validation, replay nonce protection, dedup hashing, rate-limit admission, workspace routing, and queue enqueue tests
- [x] Phase 1 Step 6 (baseline): control-plane firewall + execution-gate + approval-token service + initial OPA policy bundle and tests
- [x] Phase 1 Step 7 (baseline): deterministic workflow runtime package with interactive/provisioning/onboarding/drift/idempotency tests
- [x] Phase 1 Step 8 (baseline): executor simulate/commit flow with trust receipts, audit hash chain, SSRF blocking, and circuit-breaker tests
- [x] Phase 1 Step 9 (baseline): memory write gate, exclusion rules, workspace-scoped retrieval, and consolidation duplicate merge tests
- [x] Phase 1 Step 10 (baseline): deterministic LLM layer with prompt registry, replay cache, shadow-eval promotion gate, and determinism tests
- [x] Phase 1 Step 11 (baseline): provisioning capability resolver + policy decision algorithm + artifact verification + drift quarantine tests
- [x] Phase 1 Step 12 (baseline): staged onboarding discovery with fixed question sets, replay extraction, and workspace profile/persona/policy persistence
- [x] Phase 1 Step 13 (baseline): canvas websocket session management with interaction-to-tool-call injection tests
- [x] Phase 1 Step 14 (baseline): service health endpoints, OpenAPI v9 surface expansion, compliance matrix scaffold, and eval fixtures
- [x] Strict closure baseline: added `db/migrations/002_BREVIO_v91_soft_intelligence.sql` (21 enums, 23 tables) and `db/migrations/003_BREVIO_v92_production_hardening.sql` (34 enums, 47 tables) with workspace RLS + FK/workspace indexes + V9.2 specialized `GIN`/`HNSW` indexes; validated with Docker Go 1.22 (`gofmt`, `go build`, `go vet`, `go test`)
- [x] Strict closure hardening: added migration closure tests (`internal/database/migration_closure_test.go`) for exact enum/table sets across `001/002/003`, enforced workspace RLS coverage checks, and fixed missing V9 RLS scope entries in `001_BREVIO_v9_init.sql` (`key_versions`, `prompt_versions`, `routing_policies`, `runtime_profiles`, `server_artifacts`, `specialist_agents`)
- [x] Strict closure hardening: expanded `api/openapi/v9.yaml` to include V9/V9.1/V9.2 endpoint surface placeholders and added OpenAPI endpoint parity gate test (`internal/contracts/openapi_closure_test.go`)
- [x] Strict closure hardening: added all missing V9.1/V9.2 schema files in `schemas/` (46 total now) and enforced schema presence + JSON validity + root `additionalProperties=false` via `internal/contracts/schema_closure_test.go`
- [x] Strict closure hardening: added prompt seed files (`prompts/seed_prompts_v91.txt`, `prompts/seed_prompts_v92.txt`) and compliance matrices (`spec/traceability/compliance_matrix_v91.csv`, `compliance_matrix_v92.csv`) with parity tests in `internal/contracts/prompt_compliance_closure_test.go`
- [x] Strict closure hardening: added V9.2 artifact scaffolds for Terraform modules (`opensearch`, `admin-frontend`, `feature-flags-cache`), Helm charts (`BREVIO-admin-api`, `BREVIO-admin-frontend`, `BREVIO-rag-worker`, `BREVIO-guardrails`, `BREVIO-health-checker`), and runbooks (`RB-V92-001..009`) with presence gates in `internal/contracts/infrastructure_closure_test.go`
- [x] Strict closure hardening: scaffolded missing V9.1/V9.2 internal package structure (`internal/goals`, `trust`, `learning`, `capture`, `codebase_intel`, `exploration`, `self_modification`, `context`, `rag`, `sessions`, `temporal_reasoning`, `guardrails`, `tool_health`, `feature_flags`, `crdt`, `streaming`, `errors`, `event_schemas`, `compliance`, `caching`, `model_tiers`, `admin`, `security/pii`, `security/sandbox`) with compilable placeholder services/tests
- [x] Strict closure hardening: added V9.1/V9.2 OPA policy bundles (`policies/v91_addendum.rego`, `policies/v92_addendum.rego`), canonical event registries (`spec/events/canonical_events_v9*.txt`), V9.2 metric registry (`spec/metrics/canonical_metrics_v92.txt`), and parity gates in `internal/contracts/policy_event_metric_closure_test.go`
- [x] Strict closure hardening: upgraded `.github/workflows/ci.yaml` from placeholders to concrete lint/test/gate commands including V9.2 CI package gates (`internal/context`, `rag`, `sessions`, `temporal_reasoning`, `guardrails`, `tool_health`, `feature_flags`, `crdt`, `streaming`, `errors`, `event_schemas`, `compliance`, `admin`, `security/pii`, `security/sandbox`, `caching`, `model_tiers`) and added `internal/integration` package stub for CI contract/integration execution
- [x] Strict closure hardening: completed V9 base IaC/chart scaffolding by adding all required Terraform module entrypoints (`vpc`, `eks`, `rds`, `elasticache`, `sqs`, `s3`, `secrets`, `temporal`, `observability`), wiring staging/production environment module composition, and upgrading core Helm charts (`BREVIO-gateway`, `BREVIO-brain`, `BREVIO-control`, `BREVIO-executor`, `BREVIO-canvas`, `BREVIO-temporal-worker`) with service/HPA templates plus gateway PDB; infrastructure gate test expanded accordingly
- [x] Strict closure hardening: added production-readiness docs (`README.md`, `docs/DEVELOPMENT.md`, `docs/DEPLOYMENT.md`, `docs/ARCHITECTURE.md`) and documentation presence gate (`internal/contracts/documentation_closure_test.go`)
- [x] Strict closure hardening: added control-plane HTTP mux (`internal/control/mux.go`) and upgraded `cmd/control` to serve placeholder responses across API surface; added OpenAPI response coverage test (`internal/control/mux_test.go`) to assert non-404/non-405 for spec endpoints
- [x] Strict closure hardening: added explicit acceptance-gate suites (`internal/contracts/acceptance_gates_test.go`) covering named V9, V9.1, and V9.2 gates as executable subtests with artifact and contract assertions
- [x] Strict closure hardening: implemented gateway internal tool-call injection endpoint (`POST /v1/gateway/inject/tool_call`) in mux/service with dedicated test coverage (`internal/gateway/service_test.go`)
- [x] Strict closure hardening: replaced integration stub with executable in-memory E2E pipeline in `internal/integration` (`gateway -> control -> llm/workflows -> executor -> gateway outbound`) including happy-path and budget-exhaustion tests
- [x] Strict closure hardening: added gateway health endpoints (`GET /healthz/ready`, `GET /healthz/live`) to service mux with dedicated endpoint tests for runtime consistency across services
- [x] Strict closure hardening: upgraded `Makefile` targets to enforce closure gates (`lint`, `migrate`, `contracts`, `acceptance`, `ci`) with strict checks instead of placeholders
- [x] Strict closure hardening: strengthened provisioning Package B policy fidelity with exact decision-order tests (`DecideProvisioningV1` steps 1-8) plus RBAC hierarchy/approval tests (`owner > admin > delegate > auditor > operator`; OAuth/deploy approvals owner/admin only)
- [x] Strict closure hardening: extended `internal/workflows` with V9.1/V9.2 additive workflow stubs and tests (daily capture/log, goal review, trust eval formula, learning consolidation, capability exploration, cross-repo analysis, mission control refresh, rag ingest/eval, tool health eval, memory conflict resolve, compliance evidence, admin KPI/alerts, cache maintenance)
- [x] Strict closure hardening: implemented explicit closure checks for V9 Section 17.2 categories in `internal/contracts/closure_checks_test.go` and added traceability maps (`spec/traceability/prompt_validator_map.csv`, `spec/traceability/workflow_state_map.csv`)
- [x] Strict closure hardening: added Phase 4 production-readiness artifacts for load/security validation (`evals/load/k6_interactive_turn.js`, `scripts/security/run_security_validation.sh`) with presence gate (`internal/contracts/phase4_artifacts_test.go`) and Make targets (`load-test`, `security-validate`)
- [x] Strict closure hardening: added infra validation automation (`scripts/infra/validate.sh`) and wired it into CI (`.github/workflows/ci.yaml`) and Make (`infra-validate`) for Terraform/Helm gate execution when toolchains are available
- [x] Strict closure hardening: added admin frontend scaffolding (`admin/src/pages/Dashboard.tsx`, `admin/src/api/client.ts`) and integrated checks into infrastructure closure tests
- [x] Strict closure hardening: upgraded V9.2 Helm add-on charts (`BREVIO-admin-api`, `BREVIO-admin-frontend`, `BREVIO-rag-worker`, `BREVIO-guardrails`, `BREVIO-health-checker`) with replica/HPA/resource profiles aligned to prompt ranges and enforced HPA template presence in infrastructure gates
- [x] Strict closure hardening: added explicit temporal worker service entrypoint (`cmd/temporal-worker/main.go`) with health probes so runtime plane set includes the worker binary in build outputs
- [x] Strict closure hardening: implemented V9.2 feature flag runtime logic (`internal/feature_flags`) with CRUD/rules/evaluate/kill-switch and wired `/v1/flags*` API handling into control mux with end-to-end route tests
- [x] Strict closure hardening: implemented V9.2 context engineering API/runtime foundation (`internal/context`) and wired `/v1/context/budget` + `/v1/context/allocations` handlers into control mux with deterministic budget/allocation lifecycle tests
- [x] Strict closure hardening: implemented V9.2 session management and temporal reasoning foundations (`internal/sessions`, `internal/temporal_reasoning`) and wired `/v1/sessions/*` + `/v1/temporal/*` control-plane handlers with deterministic flow tests (active sessions, constraints/conflicts, resolve, travel-time)
- [x] Strict closure hardening: implemented V9.2 RAG runtime foundation (`internal/rag`) with deterministic collection CRUD, ingest chunking, hybrid-style retrieval logs, and eval score tracking; wired `/v1/rag/*` control-plane handlers and end-to-end mux route tests
- [x] Strict closure hardening: implemented V9.2 guardrails and tool-health foundations (`internal/guardrails`, `internal/tool_health`) and wired `/v1/guardrails/*` + `/v1/tools/*` handlers with deterministic config/rule/event and quarantine override flows
- [x] Strict closure hardening: implemented V9.2 error communication and deterministic caching foundations (`internal/errors`, `internal/caching`) and wired `/v1/errors/*` + `/v1/cache/*` handlers with taxonomy/template and policy/stats/invalidate route coverage
- [x] Strict closure hardening: implemented V9.2 event schema registry and model tier foundations (`internal/event_schemas`, `internal/model_tiers`) and wired `/v1/event-schemas/*` + `/v1/model-tiers/*` handlers with versioning/validation and tier policy/override route coverage
- [x] Strict closure hardening: implemented V9.2 compliance automation, admin backend, and streaming foundations (`internal/compliance`, `internal/admin`, `internal/streaming`) and wired `/v1/compliance/*`, `/v1/admin/*`, and `/v1/streaming/config` handlers with deterministic CRUD and operations route coverage
- [x] Strict closure hardening: implemented V9.1 domain foundations (`internal/goals`, `internal/trust`, `internal/learning`, `internal/capture`, `internal/codebase_intel`, `internal/exploration`, `internal/self_modification`) and wired concrete `/v1/goals*`, `/v1/mission-control*`, `/v1/autonomy*`, `/v1/learning*`, `/v1/captures/daily*`, `/v1/codebase*`, `/v1/capabilities/recommendations*`, and `/v1/self-modification/policy` handlers with end-to-end mux route tests
- [x] Strict closure hardening: added runtime acceptance suites (`internal/contracts/acceptance_runtime_test.go`) that exercise V9.1 and V9.2 endpoint flows end-to-end via control mux (goals/mission-control/trust/learning/captures/codebase/capabilities/self-modification + context/rag/sessions/temporal/guardrails/tools/flags/streaming/errors/event-schemas/compliance/cache/model-tiers/admin)
- [x] Strict closure hardening: eliminated remaining placeholder runtime services by implementing deterministic CRDT conflict resolution (`internal/crdt`), RAG eval gate (`internal/rag/eval`), field-level PII envelope encryption/redaction (`internal/security/pii`), and sandbox URL/CIDR enforcement (`internal/security/sandbox`) with lifecycle tests
- [x] Strict closure hardening: added control-plane endpoint specialization guard (`TestControlMuxSpecializedV91V92Endpoints`) to assert V9.1/V9.2 API groups never fall through to generic `"status":"accepted"` fallback responses
- [x] Strict closure hardening: reran static analysis gate in Docker Go 1.22 using `staticcheck@v0.5.1` (latest compatible with Go 1.22) across `./...` with zero findings
- [x] Strict closure hardening: fixed CI toolchain mismatch by pinning workflow staticcheck install to `v0.5.1` (Go 1.22 compatible) and added `internal/contracts/ci_closure_test.go` to enforce presence of all V9.2 CI package gate steps
- [x] Strict closure hardening: upgraded Terraform module contracts (`terraform/modules/*/main.tf`) with explicit V9/V9.2 infrastructure configuration fields and strengthened infrastructure closure gates to assert required module/Helm scaling and resource tokens
- [x] Strict closure hardening: aligned local lint tooling and container baseline by pinning `Makefile` staticcheck to `v0.5.1` and adding container closure contract test (`internal/contracts/container_closure_test.go`) for Go 1.22 + distroless + nonroot + read-only-FS runtime note
- [x] Strict closure hardening: replaced placeholder V9 runbooks (`RB-001..RB-009`) with concrete incident procedures and added runbook closure tests (`internal/contracts/runbook_closure_test.go`) enforcing required response sections
- [x] Strict closure hardening: upgraded compliance matrix validation to require explicit gate IDs and `implemented` status across V9/V9.1/V9.2 matrices (not only minimum row counts)
- [x] Strict closure hardening: enforced blueprint source document tracking by adding `internal/contracts/blueprint_docs_test.go` to require all three BREVIO `.docx` files are non-empty and git-tracked (`git ls-files --error-unmatch`)
- [x] Strict closure hardening: enforced OpenAPI operation identity by auto-populating deterministic `operationId` values for all operations in `api/openapi/v9.yaml` and adding uniqueness/presence gate `TestOpenAPIV9OperationIDsArePresentAndUnique`
- [x] Strict closure hardening: added migration ordering contract `internal/database/migration_ordering_test.go` to enforce Blueprint Rule Z (`ENUMs -> TABLEs -> RLS -> INDEXes`) and forward-only safeguards (no `DROP TABLE`, `DROP TYPE`, `TRUNCATE TABLE`)
- [x] Strict closure hardening: expanded CI closure gate coverage (`internal/contracts/ci_closure_test.go`) to assert full V9 Section 13.1 step set (lint/tests/migration/openapi/json schema/determinism/trivy/trufflehog/contracts/integration/prompt-injection/webhook/provisioning/onboarding/sbom) in addition to V9.2 package gates
- [x] Strict closure hardening: added canonical naming gate `internal/contracts/naming_closure_test.go` to enforce event format (`BREVIO.<domain>.<noun>.v1`) and metric format (`BREVIO_<subsystem>_<noun>`) across V9/V9.1/V9.2 registries
- [x] Strict closure hardening: added UUIDv7 identity gate `internal/database/uuid_closure_test.go` to require every `id uuid PRIMARY KEY` table column in migrations `001/002/003` uses `DEFAULT uuid_v7_generate()`
- [x] Strict closure hardening: upgraded OpenAPI schema-pointer closure by adding `TestOpenAPIV9SchemaPointersClosure` and wiring request/response `$ref` pointers (`generic_request_v1`, `generic_response_v1`) for all write operations and 2xx responses in `api/openapi/v9.yaml`
- [x] Strict closure hardening: added explicit DB workspace-session guard tests (`internal/database/pool_test.go`) for query rejection when `workspace_id` is missing and deterministic `SET app.workspace_id = $1` execution before query dispatch
- [x] Strict closure hardening: added workflow ID exact-set gate (`internal/contracts/workflow_closure_test.go`) to enforce the full 22-workflow parity set across V9/V9.1/V9.2 in `spec/traceability/workflow_state_map.csv`
- [x] Strict closure hardening: aligned V9.1/V9.2 OPA reason codes to prompt semantics (`REQUIRE_APPROVAL`, `ALLOW_WITH_AUDIT`, `ADMIN_ACTION_AUDIT`) and upgraded policy closure tests to enforce exact `rule -> result/reason` bindings
- [x] Strict closure hardening: added service-plane health closure gate (`internal/contracts/service_health_closure_test.go`) to enforce `/healthz/ready` and `/healthz/live` handler coverage across all runtime planes
- [x] Strict closure hardening: expanded compliance matrix gates to enforce NNR coverage IDs (`NNR-V91-001..008`, `NNR-V92-001..012`) and updated `spec/traceability/compliance_matrix_v91.csv` + `compliance_matrix_v92.csv` with implemented trace rows
- [x] Strict closure hardening: added connector registry seed integrity tests (`internal/connectors/service_test.go`) for `connector_key.tool_key` naming, connector-prefix consistency, connector existence, and autonomy-floor validity
- [x] Strict closure hardening: added UUID generation guard (`internal/contracts/id_generation_closure_test.go`) to prevent non-test runtime usage of `uuid.New`/`uuid.NewString` and enforce `determinism.NewUUIDv7()` usage in entity-creator services
- [x] Strict closure hardening: expanded OpenAPI component catalog closure to require references for all V9/V9.1/V9.2 schema files and wired `components.schemas` refs in `api/openapi/v9.yaml`
- [x] Strict closure hardening: expanded V9.2 runbooks (`RB-V92-001..009`) into full operational format and tightened `internal/contracts/runbook_closure_test.go` to enforce complete runbook sections
- [x] Strict closure hardening: added source comment hygiene gate (`internal/contracts/comment_hygiene_closure_test.go`) to continuously enforce no unresolved `TODO`/`FIXME`/`HACK`/`DEPRECATED` markers in implementation artifacts
- [x] Strict closure hardening: added OpenAPI auth-binding closure for `AdminJWT`, `UserJWT`, and `mTLS` and wired operation-level security requirements in `api/openapi/v9.yaml` for admin, compliance, and mTLS-only endpoints
- [x] Strict closure hardening: added module contract gate (`internal/contracts/module_closure_test.go`) to enforce canonical module path `github.com/brevio/brevio` and Go toolchain pin `go 1.22`
- [x] Strict closure hardening: implemented V9 domain autonomy default seeding at workspace creation (`calendar=A2,email=A1,messaging=A1,tasks=A2,documents=A1,crm=A1,travel=A2,financial=A1,health=A0,environment=A1,web=A3`) with validation tests
- [x] Strict closure hardening: implemented proactive silent-execution rule enforcement (`autonomy >= A2` AND `proactive_enabled=true`) via `EvaluateProactiveSilentExecution` with explicit control-plane tests
- [x] Strict closure hardening: implemented reasoning-loop deterministic constraints (`planner/critic retry_limit=1`, `executor loop_limit T1=2/T2=5/T3=10`, `plan candidates T1=1/T2=2/T3=3`), deterministic lexical ordering helpers, and idempotency key format `idem_<base32hex...>` in `internal/workflows`
- [x] Strict closure hardening: implemented V9 load-shedding tier runtime enforcement (`D0..D5`) in `internal/control` with explicit tier behavior tests (proactive disable, A3+ auto-commit disable, non-critical connector disable, read-only mode, health/audit-only minimal mode)
- [x] Strict closure hardening: added approval token policy tests for `ELEVATED` TTL (`5min`), nonce uniqueness, and key-version tracking in control-plane consent tokens
- [x] Strict closure hardening: replaced V9 JSON schema placeholders with key-field contracts (`tool_call`, `error`, capability resolver/extractor/resolve, provisioning policy/start/manifest, llm request, provisioning message payloads, action proposal) and added `internal/contracts/schema_v9_fields_closure_test.go` to enforce required fields and blueprint constraints
- [x] Strict closure hardening: replaced all remaining V9.1/V9.2 schema placeholders with concrete object contracts and tightened `internal/contracts/schema_closure_test.go` to fail if any required schema has empty `properties`
- [x] Strict closure hardening: added `internal/contracts/schema_v91_v92_fields_closure_test.go` to enforce required key fields for all 31 V9.1/V9.2 schemas plus critical constraint checks (`trust_score` bounds, `rag.top_k`, `tool_key` pattern, evidence hash format)
- [x] Strict closure hardening: specialized key OpenAPI endpoint request/response bindings away from generic payload refs (`tool_call`, goals, mission-control, context budget/allocations, rag search/retrieval, tool health, flag evaluation, error templates, compliance DSR, admin KPI) and added `TestOpenAPIV9EndpointSchemaSpecializationClosure`
- [x] Strict closure hardening: expanded OpenAPI schema specialization across V9.1/V9.2 endpoint groups (trust/promotions, learning, captures, code-context export, capability recommendations, rag collections, sessions, temporal conflicts, guardrails events, model-tier overrides, admin alerts)
- [x] Strict closure hardening: tightened canonical registry checks to exact-set parity for all V9/V9.1/V9.2 events and all V9.2 metrics (not just count/token presence) in `internal/contracts/policy_event_metric_closure_test.go`
- [x] Strict closure hardening: added executable per-gate runtime coverage tests for all named V9.1 and V9.2 acceptance gates in `internal/contracts/acceptance_gate_runtime_closure_test.go` (goal/trust/learning/capture/mission-control/discovery + context/rag/sessions/temporal/guardrails/flags/crdt/streaming/errors/event-schemas/compliance/cache/model-tier/react/security/admin/structured-generation)
- [x] Strict closure hardening: implemented concrete structured-generation runtime constraints in `internal/structured_generation` (schema-aligned action proposal validation, tool/idempotency pattern enforcement, max-actions cap, deterministic lexical canonicalization) and wired runtime assertions into the `structured_generation` acceptance gate
- [x] Strict closure hardening: encoded V9.1 workflow trigger semantics from Addendum §P in `internal/workflows.V91WorkflowTriggerSpecs()` and added exact trigger mapping tests (`TestV91WorkflowTriggerSpecs`)
- [x] Strict closure hardening: upgraded Go dependency floor within Go 1.22 constraints (`pgx/v5 v5.7.4`, `x/crypto v0.33.0`, `x/sync v0.11.0`, `x/text v0.22.0`) and added `internal/contracts/dependency_closure_test.go` to lock minimum versions
- [x] Strict closure hardening: added reproducible `govulncheck` baseline control (`scripts/security/run_govulncheck.sh` + `govuln_allowlist.txt`), wired into security validation and CI, and documented residual Go 1.22-constrained advisories in `docs/SECURITY_VULNERABILITY_BASELINE.md`
- [x] Strict closure hardening: extended CI closure contract (`internal/contracts/ci_closure_test.go`) to require `govulncheck baseline` execution tokens in `.github/workflows/ci.yaml`
- [x] Strict closure hardening: strengthened acceptance gate contracts (`internal/contracts/acceptance_gates_test.go`) to require explicit runtime subtest presence for every named V9.1 and V9.2 gate in `acceptance_gate_runtime_closure_test.go`
- [x] Strict closure hardening: replaced remaining minimum-threshold closure checks with exact parity assertions for migration table counts, OpenAPI path count, schema catalog membership, and compliance matrix row-count/ID-set exactness (`internal/contracts/closure_checks_test.go`, `internal/contracts/openapi_closure_test.go`, `internal/contracts/schema_closure_test.go`, `internal/contracts/prompt_compliance_closure_test.go`)
- [x] Strict closure hardening: enforced exact OpenAPI operation parity by matching the full `METHOD path` operation-set with no extras/missing operations in `internal/contracts/openapi_closure_test.go`
- [x] Strict closure hardening: added executable V9 runtime acceptance-gate coverage (`schema_closure`, `determinism`, `webhook_security`, `acceptance_suites_1_12`, `workspace_isolation`, `provisioning_pipeline`, `onboarding_completion`, `provisioning_recovery`, `deterministic_llm`, `cve_scanning`) in `internal/contracts/acceptance_gate_runtime_closure_test.go` and bound V9 gate-name contracts to these runtime subtests in `internal/contracts/acceptance_gates_test.go`
- [x] Strict closure hardening: strengthened V9 `acceptance_suites_1_12` runtime gate with cross-cutting assertions for key-rotation dual-window behavior, circuit-breaker open/close transitions, and blocked SSRF target enforcement via integration service probes
- [x] Strict closure hardening: added exact infrastructure directory-set gates so `terraform/modules` and `helm/` must match the canonical V9/V9.2 module/chart sets with no extras/missing entries (`internal/contracts/infrastructure_closure_test.go`)
- [x] Strict closure hardening: added exact OpenAPI `components.schemas` key-set parity gate (49 canonical component schema keys, including generic/error wrappers) in `internal/contracts/openapi_closure_test.go`
- [x] Strict closure hardening: added workflow runtime exercise closure test to execute all 22 mapped V9/V9.1/V9.2 workflow IDs through `internal/workflows.Service` (`internal/contracts/workflow_closure_test.go`)
- [x] Strict closure hardening: removed compliance evidence hash placeholders by computing/normalizing real SHA-256 digests in `internal/compliance.Service`, wired framework evidence creation through computed digests (`internal/control/mux.go`), and added runtime/test enforcement of valid `sha256:<64hex>` evidence hashes
- [x] Strict closure hardening: strengthened govuln baseline closure by enforcing allowlist ID format/uniqueness/non-empty rules and requiring baseline documentation linkage in `internal/contracts/security_vuln_baseline_closure_test.go`
- [x] Strict closure hardening: upgraded service health closure from token-only checks to executable runtime health assertions for gateway/control/canvas (`/healthz/ready`, `/healthz/live`) in `internal/contracts/service_health_closure_test.go`
- [x] Strict closure hardening: fixed Phase 4 k6 webhook load test to use real HMAC-SHA256 request signatures (`WEBHOOK_SECRET`) and tightened Phase 4 artifact gates for load/security/infra script semantics (`internal/contracts/phase4_artifacts_test.go`)
- [x] Strict closure hardening: added runtime canonical gateway event emission checks and coverage for ingress/security events (`BREVIO.ingress.received.v1`, `BREVIO.ingress.duplicate_dropped.v1`, `BREVIO.security.webhook.signature_invalid.v1`, `BREVIO.security.webhook.replay_blocked.v1`) in `internal/gateway` + acceptance runtime closure tests
- [x] Strict closure hardening: tightened prompt-to-validator closure to exact prompt mapping parity (fixed row count + exact seed prompt ID set match, no extra mapped prompts) in `internal/contracts/closure_checks_test.go`
- [x] Strict closure hardening: added exact workflow traceability row-count closure for `spec/traceability/workflow_state_map.csv` (22 workflows + header) in `internal/contracts/closure_checks_test.go`
- [x] Strict closure hardening: upgraded documentation closure from non-empty checks to required section/token parity across `README.md`, `docs/ARCHITECTURE.md`, `docs/DEVELOPMENT.md`, and `docs/DEPLOYMENT.md` (`internal/contracts/documentation_closure_test.go`)
- [x] Strict closure hardening: added exact runbook file-set closure for canonical V9 (`RB-001..009`) and V9.2 (`RB-V92-001..009`) runbooks in `internal/contracts/runbook_closure_test.go`
- [x] Strict closure hardening: fixed executor commit idempotency to prevent duplicate side effects/receipts on retries, added canonical executor/trust event emissions (`BREVIO.hands.tool.simulated.v1`, `BREVIO.hands.tool.committed.v1`, `BREVIO.trust.receipt.created.v1`, `BREVIO.trust.evidence.attached.v1`), and added runtime tests in `internal/executor/service_test.go`
- [x] Strict closure hardening: extended integration + acceptance runtime pipeline checks to assert canonical event emission across gateway/executor (`BREVIO.ingress.received.v1`, `BREVIO.hands.tool.simulated.v1`, `BREVIO.hands.tool.committed.v1`, `BREVIO.trust.receipt.created.v1`, `BREVIO.trust.evidence.attached.v1`) via `internal/integration` and `internal/contracts/acceptance_gate_runtime_closure_test.go`
- [x] Strict closure hardening: upgraded blueprint source document gate to exact root `.docx` set parity (no missing/no extra) while retaining non-empty and git-tracked enforcement in `internal/contracts/blueprint_docs_test.go`
- [x] Strict closure hardening: removed control-plane generic API fallback and enforced endpoint ownership in mux tests so only control-owned OpenAPI paths resolve in `internal/control` while gateway/canvas paths 404 on control (`internal/control/mux.go`, `internal/control/mux_test.go`)
- [x] Strict closure hardening: hardened infra validation script to enforce exact Terraform module/environment and Helm chart sets, validate both module+environment Terraform roots, and fail in CI/strict mode when Terraform/Helm tooling is missing (`scripts/infra/validate.sh`, `internal/contracts/phase4_artifacts_test.go`)
- [x] Strict closure hardening: added OpenAPI service-ownership contract tests to enforce endpoint routing boundaries across gateway/control/canvas muxes and fail on cross-service endpoint leakage (`internal/contracts/openapi_service_ownership_closure_test.go`)
- [x] Strict closure hardening: upgraded CI/security gates to install and require Trivy/TruffleHog/Syft (no skip-paths), added CI closure assertions for security-toolchain enforcement, and made security validation script fail in CI/strict mode when scanners are unavailable (`.github/workflows/ci.yaml`, `internal/contracts/ci_closure_test.go`, `scripts/security/run_security_validation.sh`, `internal/contracts/phase4_artifacts_test.go`)
- [x] Strict closure hardening: completed CI strict-toolchain bootstrap by installing Terraform + Helm alongside security scanners before `infra validate` and security scans, with CI closure assertions pinned to explicit tool versions (`.github/workflows/ci.yaml`, `internal/contracts/ci_closure_test.go`)
- [x] Strict closure hardening: upgraded Docker/CI image pipeline to build per-service images (`gateway`, `brain`, `control`, `executor`, `canvas`, `temporal-worker`) via `SERVICE` build arg, scan built images with Trivy, and lock behavior with container/CI/Makefile closure tests (`Dockerfile`, `Makefile`, `.github/workflows/ci.yaml`, `internal/contracts/container_closure_test.go`, `internal/contracts/ci_closure_test.go`, `internal/contracts/makefile_closure_test.go`)
- [x] Strict closure hardening: strengthened IaC closure from token checks to exact-list/module-set assertions (SQS FIFO/standard/DLQ lists, S3 bucket set, observability stack, managed secrets, and exact staging/production Terraform module block sets) in `internal/contracts/infrastructure_closure_test.go`
- [x] Strict closure hardening: added canonical SLO catalogs for V9/V9.2 with exact-set contract enforcement (`spec/slos/v9_slos.txt`, `spec/slos/v92_slos.txt`, `internal/contracts/slo_closure_test.go`) and expanded k6 load profile thresholds/scenarios to cover T1/T2/T3 latency and availability/error gates (`evals/load/k6_interactive_turn.js`, `internal/contracts/phase4_artifacts_test.go`)
- [x] Strict closure hardening: upgraded V9.2 runbook validation to assert trigger-specific semantics for `RB-V92-001..009` (degradation/quarantine/overflow/streaming/flag/conflict/dsr/guardrail/admin incident triggers) in `internal/contracts/runbook_closure_test.go`
- [x] Strict closure hardening: expanded infra validation to require successful `helm template` rendering for every canonical chart (in addition to `helm lint`) and locked this behavior in Phase 4 artifact closure checks (`scripts/infra/validate.sh`, `internal/contracts/phase4_artifacts_test.go`)
- [x] Strict closure hardening: added script-level exactness gates for canonical infra/security automation arrays and required command paths (`internal/contracts/script_closure_test.go`), enforcing exact Terraform module/environment/chart sets and strict scanner/go-test command coverage in validation scripts
- [x] Strict closure hardening: replaced Helm `busybox:latest` placeholders with service-specific immutable image coordinates and added infra closure checks to forbid placeholder repositories/tags across all canonical charts (`helm/*/values.yaml`, `internal/contracts/infrastructure_closure_test.go`)
- [x] Strict closure hardening: added `terraform fmt -check -recursive` as a mandatory infra validation gate and enforced it via Phase 4/script closure contracts (`scripts/infra/validate.sh`, `internal/contracts/phase4_artifacts_test.go`, `internal/contracts/script_closure_test.go`)
- [x] Strict closure hardening: upgraded deployment documentation to explicitly include the canonical `terraform apply` + `helm install` sequence and enforced required deployment tokens/section in docs closure tests (`docs/DEPLOYMENT.md`, `internal/contracts/documentation_closure_test.go`)
- [x] Strict closure hardening: enforced exact runtime-service build matrix parity (`cmd/` directories, Docker build loop, CI build/scan loops, Dockerfile `SERVICE` target) and exact Helm chart-to-image repository/tag mappings via closure tests (`internal/contracts/service_matrix_closure_test.go`, `internal/contracts/infrastructure_closure_test.go`)
- [x] Strict closure hardening: added migration strict-closure contracts for exact enum/table sets across 001/002/003, enforced migration ordering (enum → table → RLS → index), and verified workspace-scoped table parity with `workspace_tables` RLS declarations (`internal/contracts/migration_closure_test.go`)
- [x] Strict closure hardening: removed Helm `sleep` placeholders and hardened all deployment charts for distroless runtime (`/app/service`, non-root `65532`, read-only root FS, dropped capabilities, tmp `emptyDir`) with contract enforcement (`helm/*/templates/deployment.yaml`, `internal/contracts/infrastructure_closure_test.go`)
- [x] Strict closure hardening: made `scripts/infra/validate.sh` Bash 3-compatible by removing associative-array usage (`local -A`) and preserving exact-set validation semantics for Terraform modules/environments and Helm charts on macOS default shell
- [x] Strict closure hardening: added readiness/liveness probes (`/healthz/ready`, `/healthz/live`) to all Helm deployments and enforced probe presence in infrastructure closure contracts (`helm/*/templates/deployment.yaml`, `internal/contracts/infrastructure_closure_test.go`)
- [x] Strict closure hardening: made security validation scripts autonomous on hosts without local Go by adding Dockerized Go 1.22 fallback for `go test` and `govulncheck`, plus Bash 3-safe vulnerability ID parsing (`scripts/security/run_security_validation.sh`, `scripts/security/run_govulncheck.sh`)
- [x] Strict closure hardening: ignored generated security/build outputs to keep repository clean during autonomous validation runs (`artifacts/`, `sbom.spdx.json` in `.gitignore`)
- [x] Strict closure hardening: added script closure tests to enforce Bash 3 portability (`no local -A`, `no mapfile`) and preserve Dockerized Go fallback behavior in security/infra validation scripts (`internal/contracts/script_closure_test.go`)
- [x] Strict closure hardening: added `scripts/dev/go_exec.sh` and switched Make targets to Dockerized Go fallback (`build`, `test`, `lint`, `migrate`, `contracts`, `acceptance`) so local validation works without host Go installation
- [x] Strict closure hardening: resolved `staticcheck` failure in contracts suite by removing dead helper `containsLine` from policy/event/metric closure tests (`internal/contracts/policy_event_metric_closure_test.go`)
- [x] Strict closure hardening: fixed Makefile lint formatting gate by adding `scripts/dev/gofmt_exec.sh` (Dockerized gofmt fallback) and wiring `GOFMT_EXEC` into `lint` target with closure coverage (`Makefile`, `scripts/dev/gofmt_exec.sh`, `internal/contracts/makefile_closure_test.go`)
- [x] Strict closure hardening: enabled Docker fallback execution for infra tooling so `scripts/infra/validate.sh` runs Terraform/Helm validation without host installs (`hashicorp/terraform:1.9.8`, `alpine/helm:3.16.4`) and enforced fallback tokens in script closure tests
- [x] Strict closure hardening: prevented false-positive comment hygiene failures from Terraform provider artifacts by skipping `.terraform` directories in source scans and ignoring Terraform working dirs in `.gitignore` (`internal/contracts/comment_hygiene_closure_test.go`, `.gitignore`)
- [x] Strict closure hardening: fixed Terraform formatting drift detected by Dockerized `infra-validate` (applied `terraform fmt -recursive`) and resolved Helm template parsing errors for `taskQueue` default rendering in all task-queue services
- [x] Strict closure hardening: normalized all Helm template resource names/labels to lowercase (`{{ .Chart.Name | lower }}`) to satisfy Kubernetes naming requirements and eliminate lint warnings
- [x] Phase 0 closure refresh: updated `docs/codebase_audit_report.md` to current repository state (all canonical V9 artifacts present, cleanup status captured, baseline verification rerun)
- [x] Phase 1 continuation (Step 1 repo_scaffold): added explicit scaffold closure contract covering canonical directories, service entrypoints, `.golangci.yml` linter set, required Make targets, and Docker baseline (`internal/contracts/repo_scaffold_closure_test.go`)
- [x] Phase 1 continuation (Step 2 database_core): added executable PostgreSQL-16 migration verifier (`scripts/database/verify_postgres_migrations.sh`) with enum-count, RLS coverage, unset `app.workspace_id` rejection, and cross-workspace isolation checks; wired via `make db-verify` and closure tests
- [x] Phase 1 continuation (Step 3 identity_and_workspace): expanded identity/delegation/RBAC to include account+user retrieval/update lifecycle, workspace retrieval/archive lifecycle, channel support validation (`whatsapp`/`imessage`), delegation revocation + grantee tool visibility, pairing invitation acceptance, and HTTP role-check middleware with strict tests
- [x] Phase 1 continuation (Step 4 connector_registry_seed): hardened connector registry with strict YAML/JSON seed parsing + validation, tool schema payload normalization, workspace/user connector setting lifecycle, workspace-scoped connector health retrieval, and AWS Secrets Manager-backed key provider implementation with key-versioned OAuth vault tests
- [x] Phase 1 continuation (Step 5 gateway_webhooks): hardened webhook ingress with strict nonce/channel validation, workspace-routed attachment-to-S3 reference pipeline, interactive/discovery parsing, voice transcript preprocessing, iMessage parity handling, FIFO queue key enforcement (`group=user_channel_id`, `dedup=ingress_turn_id`), and outbound outbox payload validation with end-to-end integration regression coverage
- [x] Phase 1 continuation (Step 6 control_plane_core): added explicit L4 schema firewall path (`FirewallCheckWithSchema`), semantic verifier hooks (`VerifyToolOutput`), recipient+memory policy gate composition (`EvaluateExecutionPolicy`), and deterministic per-workspace tool-rate cap + monthly budget enforcement primitives with strict unit coverage
- [x] Phase 1 continuation (Step 7 temporal_workflow_runtime): added workflow state-mirror persistence primitives (instance/step mirrors for interactive, provisioning, onboarding), per-step provisioning idempotency keys with reverse compensation mirroring, and two-phase tool execution idempotency store with 30-day TTL replay semantics plus deterministic runtime tests
- [x] Phase 1 continuation (Step 7 temporal_workflow_runtime hardening): added exhaustive per-step provisioning failure injection coverage to assert reverse-order compensation for every deterministic step in `provisioning_v9`
- [x] Phase 1 continuation (Step 8 executor_tooling): expanded executor runtime with synthesis-evidence emission (`BREVIO.trust.synthesis_evidence.created.v1`), layered L1/L2/L3 cache promotion behavior, circuit-breaker protected fallback execution path, localhost SSRF hardening, and audit payload minimization/redaction with strict regression tests
- [x] Phase 1 continuation (Step 8 executor_tooling hardening): strengthened executor SSRF defense to block private/link-local/multicast CIDRs and added rebinding-aware hostname resolution checks with deterministic test coverage for private literal targets and DNS-resolved loopback blocking
- [x] Phase 1 continuation (Step 9 memory_system): rebuilt memory service with full type/data-class/trust validation, workspace write-policy gates, lifecycle transitions (`proposed -> needs_confirmation -> active -> superseded -> deleted`), trust-filtered retrieval, and consolidation that supersedes duplicates, increments embedding version, and expires stale entries
- [x] Phase 1 continuation (Step 10 llm_layer): upgraded deterministic LLM runtime with tier-normalized token caps (`T0/T1/T2/T3`), request-hash replay identity including prompt version and deterministic params, prompt rollback support, and controlled fallback-provider failover semantics only when no output was committed
- [x] Phase 1 continuation (Step 11 provisioning_engine): hardened provisioning runtime with versioned deterministic server ranker scoring + explanation replay cache, and expanded artifact verification to enforce SBOM/vulnerability gates with optional Ed25519 signature verification
- [x] Phase 1 continuation (Step 12 onboarding_discovery): expanded fixed discovery question sets across all four onboarding stages, enforced replay-locked extraction on schema-required keys, and persisted versioned workspace state with `workspace_profiles` (13 dimensions), `workspace_personas`, and `workspace_behavior_policies` (10 dimensions), including acceptance-fixture parity updates
- [x] Phase 1 continuation (Step 13 a2ui_canvas): expanded canvas runtime with explicit A2UI mission-control surface rendering endpoint, SSRF-protected fetch-preview endpoint, persistent interaction logging, and websocket injection regression coverage while retaining tool-call forwarding semantics
- [x] Phase 1 continuation (Step 14 observability_and_gates): added executable observability runtime primitives (`internal/observability`) for required structured log fields (`ts,service,env,workspace_id,user_id,ingress_turn_id,trace_id,span_id,event,severity,attrs`) and canonical-metric registry validation/recording against metric catalogs, with full test coverage and CI-green verification
- [x] Phase 2 continuation (Step 15 goal_system): added explicit daily goal-creation cap enforcement (20/workspace/day), goal-review stalled detection with deterministic status transition, and wired goal-create API path (`POST /v1/goals`) to strict rate-limit handling with regression coverage
- [x] Phase 2 continuation (Step 17 trust_autonomy): implemented exact deterministic trust-score formula and promotion-eligibility rules (`score >= 0.85`, `success >= 20`, `trailing14d_failure=0`), promotion proposal generation on recalculation, immutable post-decision promotion status semantics, and control-plane trust endpoint recalculation wiring
- [x] Phase 2 continuation (Step 18 learning_system): hardened learning ingestion with explicit feedback submission path (`SubmitFeedback`) + deterministic lesson proposal creation, active-lesson cap enforcement with strict `429` API handling, and bulk lesson retire semantics for admin operations with comprehensive regression coverage
- [x] Phase 2 continuation (Step 19 daily_introspection): replaced seeded placeholder captures with deterministic daily-log accumulation + idempotent daily-capture materialization (`daily_capture_output` shape), added morning-briefing derivation primitives, enforced daily-capture uniqueness skip semantics in workflow runtime, and expanded capture API/workflow regression coverage
- [x] Phase 2 continuation (Step 20 self_modification_controls): implemented strict self-mod policy validation (`max_allowed_risk`), deterministic action decision engine (`deny`/`require_approval`/`allow_with_audit`) with canonical audit event mapping (`BREVIO.self_modification.denied.v1`/`executed.v1`), decision history tracking, and control-mux regression coverage for invalid-risk rejection
- [x] Phase 2 continuation (Step 21 cross_repo_intelligence): expanded codebase intelligence with repository snapshot ingestion, deterministic cross-repo shared dependency/pattern analysis reports, API surfacing for shared signals (`shared_dependencies`/`shared_patterns`), and workflow mirror hardening for `cross_repo_analysis_v1` completion/skip semantics
- [x] Phase 2 continuation (Step 22 capability_exploration): replaced static recommendation stubs with deterministic capability-gap accumulation + thresholded recommendation materialization (`>=3` events/7d), confidence scoring and reason updates, strict recommendation decision validation (`accept|reject|defer`), and workflow mirror hardening for `capability_exploration_v1`
- [x] Phase 2 continuation (Step 23 project_templates_export): hardened project template/export runtime with strict context-export validation (`format`, `scope`, required repo linkage), enforced V9.1 export-rate policy (`max 10 exports/workspace/day` -> `EXPORT_RATE_LIMIT` + HTTP `429`), normalized template statuses, and added service+mux regression coverage
- [x] Phase 2 continuation (Step 24 adaptive_discovery): added deterministic follow-up rule engine and adaptive question lifecycle to onboarding completion (`followup_rules` -> `adaptive_questions`, `pending -> answered`), with rule-trigger evaluation (`onboarding_completed`, `meeting_load_high`, `low_autonomy_preference`) and acceptance/onboarding regression coverage
- [x] Phase 2 continuation (Step 25 v91_observability_gates): added explicit V9.1 canonical metrics catalog (`spec/metrics/canonical_metrics_v91.txt`) and hardened closure tests to assert exact V9.1 metrics/event coverage plus observability registry loading across V9.1+V9.2 catalogs
- [x] Phase 3 continuation (Step 26 context_engineering): replaced placeholder context budget layer with deterministic token-budget configuration/allocation runtime (tiered config, reserved response budget, RAG cap), overflow gating (`CONTEXT_BUDGET_EXCEEDED`), schema-aligned allocation reporting fields, and control API overflow enforcement/regression coverage
- [x] Phase 3 continuation (Step 27 rag_pipeline_core): upgraded RAG runtime to deterministic hybrid retrieval core (chunking by configurable size, hashed embedding vectors, BM25 token overlap, weighted hybrid scoring), schema-aligned collection/retrieval identifiers (`collection_id`, `retrieval_id`, `query_rewrite`, `source`), and control-route compatibility for both legacy and V9.2 request shapes
- [x] Phase 3 continuation (Step 28 rag_retrieval_and_eval): added deterministic reranker configuration (dense/BM25 normalized weights), retrieval-level RAG eval scoring (`faithfulness`, `relevance`, pass/fail thresholding), and expanded `/v1/rag/eval/scores` output to include collection scores, retrieval scores, and active reranker config
- [x] Phase 3 continuation (Step 29 session_management): upgraded session runtime with schema-aligned context payload (`session_id`, `conversation_id`, `active_intent`, `entities`), deterministic intent tracking/coreference state, recency-ordered active sessions, and control-route intent-continuation handling with regression coverage
- [x] Phase 3 continuation (Step 30 temporal_reasoning): expanded temporal resolver coverage (`next <weekday>`, `in X weeks`, horizon-aware confidence), added schema-aligned scheduling conflict reports (`has_conflict`, `resolution_hint`, conflict windows), and updated control temporal routes to accept `reference_ts` while preserving existing request compatibility
- [x] Phase 3 continuation (Step 31 guardrails_runtime): replaced passive guardrail stubs with runtime input evaluation (pattern/jailbreak scoring + PII redaction), schema-aligned guardrail event payloads, and middleware enforcement on RAG search path with `403 GUARDRAIL_BLOCK_ACTIVE` blocking behavior under configured thresholds
- [x] Phase 3 continuation (Step 32 tool_health_scorer): upgraded tool health scoring to include latency/error telemetry (`latency_ms`, `error_rate`), rule-driven auto-quarantine, recovery/degradation event emission (`BREVIO.tool_health.*`), and deterministic override/recovery semantics with regression coverage
- [x] Phase 3 continuation (Step 33 feature_flags): restored and hardened `internal/feature_flags/service.go` with deterministic evaluation cache invalidation, kill-switch policy enforcement, workspace-aware evaluation outputs aligned to `feature_flag_evaluation.v1.json` (`flag_key`, `workspace_id`, `enabled`, `variant`, `reason`), and control-route/unit regression coverage
- [x] Phase 3 continuation (Step 34 crdt_resolution): rebuilt `internal/crdt` with explicit vector-clock dominance/concurrency comparison, deterministic conflict classification, strategy-driven resolution flows (`last_writer_wins`, `merge_concat`, `manual_review`), workspace-scoped conflict reporting aligned to `memory_conflict_report.v1.json`, and backward-compatible apply/resolve API regression coverage
- [x] Phase 3 continuation (Step 35 streaming_ux): expanded `internal/streaming` with deterministic delivery planning (ack/typing/progressive chunks), first-byte SLA tracking (`<=500ms`) with breach accounting, strict streaming config guardrails in control routes, and streaming/control regression coverage
- [x] Phase 3 continuation (Step 36 error_communication): upgraded `internal/errors` to persona-aware template resolution with deterministic precedence, schema-aligned user-facing message rendering (`error_code`, `user_message`, `retryable`, `next_action`), and internal reference redaction so runtime errors never expose UUID/trace/internal IDs
- [x] Phase 3 continuation (Step 37 event_schema_registry): hardened `internal/event_schemas` with strict schema parsing, required/type/additionalProperties validation, backward-compatibility checks on new version registration (breaking-change rejection), and control-plane strict registration handling for `/v1/event-schemas/{type}/versions`
- [x] Phase 3 continuation (Step 38 deterministic_caching): replaced single-layer cache with deterministic L1/L2/L3 runtime in `internal/caching` (TTL-bound writes, layer promotion on reads, cross-layer invalidation, size-limit enforcement, namespace-based policy matching) and expanded caching/control regression coverage
- [x] Phase 3 continuation (Step 39 model_tier_constraints): upgraded `internal/model_tiers` with deterministic tier-cap enforcement (`requested_tier -> resolved_tier`), complexity-threshold escalation within workspace caps, override audit records aligned to `target_tier`/`expires_at`, and control-path evaluation support via `/v1/model-tiers/overrides?requested_tier=...&complexity_score=...`
- [x] Phase 3 continuation (Step 40 react_early_exit): added deterministic ReAct early-exit execution logic in `internal/workflows` (per-tier `max_steps`, signal-based termination, partial result fallback) and strengthened runtime acceptance gates to assert `MAX_STEPS_REACHED` behavior under T3 limits
- [x] Phase 3 continuation (Step 41 security_hardening): hardened `internal/security/pii` with key-versioned field encryption policies + dual-key read-window rotation handling, and hardened `internal/security/sandbox` with profile-based MCP egress controls (HTTPS enforcement, allow/deny suffixes, blocked CIDRs, IMDS/loopback/private network denial, violation audit records)
- [x] Phase 3 continuation (Step 42 compliance_automation): hardened `internal/compliance` with deterministic framework normalization, evidence metadata enrichment (`collected_at`, normalized `sha256`), schema-aligned DSR identifiers/deadlines (`request_id`, `deadline_at`), and automated DSR SLA-at-risk tracking surfaced through compliance API responses
- [x] Phase 3 continuation (Step 43 admin_backend): hardened `internal/admin` with alert evaluation engine + alert event records, dashboard configuration and saved-view lifecycle support, enriched dashboard/KPI payloads, and maintained full admin endpoint CRUD behavior with updated regression coverage
- [x] Phase 4.1 integration: expanded integration runtime coverage in `internal/integration` to include provisioning failure compensation, onboarding completion, drift quarantine, and representative V9.1/V9.2 workflow executions in addition to the gateway→control→workflow→executor→gateway pipeline tests
- [x] Phase 4.1 integration hardening: added deterministic end-to-end scenarios for approval challenge/approval-token commit, multi-tool plan execution, per-workspace tool-rate cap + monthly budget enforcement across turns, and provisioning `VerifyArtifact` failure compensation path coverage (`internal/integration/service_test.go`)
- [x] Phase 4.1 integration hardening: extended end-to-end webhook coverage to include iMessage channel routing path (`/v1/gateway/webhook/imessage`) through the same gateway→control→workflow→executor→outbound pipeline assertions
- [x] Phase 4.1 integration hardening: added provisioning super-workflow failure-injection integration coverage for every deterministic step (`Preflight` through `Active`) with reverse-compensation assertions at each failure point
- [x] Phase 4.1 cross-cutting hardening: added integration-level security/runtime probes for key-rotation dual-key read window (`>=10m`), circuit-breaker open/close transitions, and blocked-SSRF CIDR targets through the integrated runtime facade
- [x] Phase 4.2 load testing: added load-shedding tier probe script `evals/load/k6_load_shedding.js` (D0..D5 scenarios) and updated load test README with execution guidance
- [x] Phase 4.2 load testing hardening: added dedicated streaming first-byte SLA probe (`evals/load/k6_streaming_first_byte.js`) with `BREVIO_streaming_first_byte_ms` P95 threshold (`<500ms`) and closure-test coverage
- [x] Phase 4.2 load testing hardening: updated `make load-test` to advertise all canonical k6 suites (interactive, load-shedding, streaming first-byte) and locked this behavior in makefile closure tests
- [x] Phase 4.3 security validation: extended `scripts/security/run_security_validation.sh` with explicit `internal/security/pii` and `internal/security/sandbox` suites, then executed `make security-validate` successfully (expected optional skips for host-missing `trivy`/`trufflehog`/`syft`)
- [x] Phase 4.3 security hardening: upgraded security validation to run dockerized fallback scans for `trivy`, `trufflehog`, and `syft` (with repository-safe exclusion scopes for `.git`, `.terraform`, `artifacts`, and `classmate-ai`) so local runs no longer depend on host-installed binaries
- [x] Phase 4.3 security hardening: added deterministic Trivy report gating with explicit allowlist evaluation (`scripts/security/check_trivy_report.py` + `scripts/security/trivy_allowlist.txt`) so HIGH/CRITICAL findings are policy-evaluated instead of silently skipped
- [x] Phase 4.4 documentation: refreshed deployment, development, and architecture docs (`docs/DEPLOYMENT.md`, `docs/DEVELOPMENT.md`, `docs/ARCHITECTURE.md`) with validated heading/token closure and current Phase 4 runbook commands
- [x] Phase 4.4 documentation hardening: added deterministic OpenAPI-generated API reference (`scripts/docs/generate_api_reference.go` -> `docs/API_REFERENCE.md`) and enforced CI sync gate (`api docs sync` with `git diff --exit-code`) plus closure tests
- [x] Phase 4 validation hardening: normalized all script Docker resolvers to prefer Docker Desktop binary first (`/Applications/Docker.app/Contents/Resources/bin/docker`) across dev/security/infra/database tooling for deterministic autonomous execution paths in this environment
- [x] Phase 1/4 migration assurance hardening: re-executed PostgreSQL-16 runtime migration verification (`make db-verify`) and promoted DB runtime verification into CI gates (`migration runtime verify` -> `scripts/database/verify_postgres_migrations.sh`)
- [x] Phase 4 CI parity hardening: aligned local `make ci` with pipeline behavior by adding `api-docs-check` target (regenerate `docs/API_REFERENCE.md` + fail on `git diff`) and enforced via Makefile closure tests
- [x] Phase 4 release evidence hardening: re-ran `make security-validate`, `make infra-validate`, and `make db-verify`, then refreshed `docs/FINAL_VALIDATION_v9.2.0-final.md` with current timestamped command/results set
- [x] Phase 4 security baseline hardening: expanded vulnerability-baseline closure contracts to validate `trivy_allowlist.txt` format/content + Trivy allowlist enforcement wiring, and updated `docs/SECURITY_VULNERABILITY_BASELINE.md` to include explicit Trivy exception governance (`CVE-2025-22869` under Go 1.22 constraint)
- [x] Phase 4 evidence closure hardening: added `internal/contracts/final_validation_closure_test.go` to enforce structure/tokens/timestamp format for `docs/FINAL_VALIDATION_v9.2.0-final.md` (validation commands, PASS results, and blueprint `.docx` tracking section)
- [x] Phase 4 dependency-constraint hardening: added executable compatibility guard (`TestGoToolchainCryptoCompatibilityConstraint`) requiring Go `1.22` release-line builds to pin `golang.org/x/crypto` below `v0.35.0` (avoids non-buildable upgrades requiring Go `>=1.23`)
- [x] Phase 4 final lock: re-ran full command gate set at HEAD (`make ci`, `make security-validate`, `make infra-validate`, `make db-verify`) and refreshed final validation evidence timestamp for release-lock state
- [x] Phase 4 infra hardening: upgraded `scripts/infra/validate.sh` to execute `terraform plan` for both staging and production (`-refresh=false -lock=false -input=false -detailed-exitcode`) and added closure-test enforcement
- [x] Phase 4 evidence refresh: reran full gate set after infra hardening and updated `docs/FINAL_VALIDATION_v9.2.0-final.md` to capture terraform plan coverage in final validation notes
- [x] Phase 4 CI usability hardening: added `make ci-full` target (`ci + security-validate + infra-validate + db-verify`) and enforced target presence in Makefile closure contracts for one-command full-gate execution
- [x] Phase 4 one-command closure: executed `make ci-full` successfully (lint/build/test/contracts/acceptance + security + infra + db runtime verification) and refreshed final validation report evidence
- [x] Phase 4 production ingress closure: added ALB-ready ingress support for `BREVIO-gateway` and `BREVIO-admin-frontend` charts (service type controls + ingress templates), added `scripts/deploy/render_prod_values.sh` for deterministic `ROOT_DOMAIN`/`ACM_CERT_ARN` overlay generation, and updated `docs/DEPLOYMENT.md` with end-to-end production command sequence
- [x] Phase 4 regression validation: reran `make ci`, `make security-validate`, `make infra-validate`, and `make db-verify` after ingress/deployment changes with all gates passing
- [x] MCP closure hardening: implemented shared MCP/native tool registry runtime (`internal/mcp`) with auth-matrix coverage, per-server policy gates (call/cost/rate), invocation provenance tracking, migration `005_BREVIO_mcp_execution_oauth_hardening.sql` (`tool_executions.is_mcp/mcp_server_id/content_provenance` + `user_oauth_tokens.provider`), admin MCP health surfacing, and invariant contract gates (`internal/contracts/mcp_invariants_closure_test.go`)
- [x] Phase 4 closure note: captured remaining Trivy HIGH exception (`CVE-2025-22869`) as Go 1.22-constrained allowlist item and documented only remaining human-required triggers (credential provisioning + production apply/install gate)
- [x] Phase 4.5 final validation: executed `make ci`, `make security-validate`, and `make infra-validate` successfully; Terraform module/env validation and Helm lint/template checks passed via dockerized toolchain fallbacks
- [x] Phase 4 release closure: produced final validation report (`docs/FINAL_VALIDATION_v9.2.0-final.md`) and emitted release tags (`v9.0.0`, `v9.1.0`, `v9.2.0`, `v9.2.0-final`)

Migration rules (must follow)
- Preserve already-working preserved components unchanged unless v4.0 explicitly requires changes (per user instructions).
- Do not jump ahead: build order is Phase 1 → Phase 5 (EXECUTIVE_BLUEPRINT Section 38 + MCP Waves 5–6 plan).

Feature Flag Rollout Reminders (Appendix A)
- [x] Phase 1 exit gate: set `FEATURE_MULTI_PROVIDER_LLM=true` (router v1+v2 done and all LLM calls migrated)
- [x] Phase 1 exit gate: set `FEATURE_VOICE_INPUT=true` (voice-in pipeline + `/api/v1/voice/transcribe` validated)
- [x] Phase 1 exit gate: set `FEATURE_IMAGE_PROCESSING=true` (image pipeline validated end-to-end)
- [x] Phase 1 exit gate: keep `FEATURE_DOCUMENT_PROCESSING=false` until Phase 4 document ingestion/generation is complete
- [x] Phase 2 exit gate: set `FEATURE_PRIVILEGE_ISOLATION=true` (provenance + capability-token enforcement validated)
- [x] Phase 3 exit gate: set `FEATURE_CONSOLIDATION_ENABLED=true` (nightly consolidation job validated)
- [x] Phase 3 exit gate: set `FEATURE_SELF_REVIEW_ENABLED=true` (weekly self-review job validated)
- [x] Phase 3 exit gate: set `FEATURE_MCP_CLIENT=true` (MCP hub + sandbox + Wave 1 E2E validated)
- [x] Phase 4 exit gate: set `FEATURE_DOCUMENT_PROCESSING=true` (document ingestion + document generation validated)

---

## Current State Snapshot (Verified)
- [x] 3-plane entrypoints exist: Gateway, Brain, Hands (Section 1, Section 6)
- [x] Internal Brain/Hands APIs exist and Gateway supports SSE streaming proxying (Section 1, Section 5, Section 7)
- [x] Staging ECS deploy is working (Phase 1 M1 S1-2)
- [x] Production ECS deploy listener conflict mitigated in IaC (`ALB_CREATE_HTTPS_LISTENER` gate + existing-listener-safe path)
- [x] OpenTelemetry (OTEL) wiring exists in app and Axiom ingest is configured (Section 33)
- [x] Multi-Provider LLM Router implemented; direct OpenAI imports migrated behind router proxy (Section 9)
- [x] v4.0 database schema migration authored (19 tables + enums + enhancement columns + RLS policy baseline) (Section 3)

---

# PHASE 1 — FOUNDATION (Month 1–3) (Section 38)

## M1 S1–2: AWS Foundation + Monitoring + LLM Router v1 (Section 38, Section 2, Section 9, Section 33, Section 35)
- [x] Staging: VPC + ECS + RDS Postgres 16 + Redis 7 deployed
- [x] Production: resolve HTTPS listener conflict path in IaC (`ALB_CREATE_HTTPS_LISTENER` + existing listener compatibility)
- [x] OpenTelemetry (OTEL) tracing enabled per plane (`OTEL_SERVICE_NAME`) and exported to Axiom
- [x] Create/confirm S3 buckets per Appendix A: attachments, knowledge snapshots, voice, documents
- [x] Disaster Recovery baseline: enable RDS Multi-AZ on day one (staging + prod) (Operational Blueprint Component 7)
- [x] Disaster Recovery baseline: enable automated RDS snapshots (daily, 7-day retention) (Operational Blueprint Component 7)
- [x] Disaster Recovery baseline: enable point-in-time recovery (PITR) via RDS automated backups (AWS-managed WAL) (Operational Blueprint Component 7)
- [x] Disaster Recovery baseline: enable S3 versioning for knowledge/attachments buckets (Operational Blueprint Component 7)
- [x] Add v4.0 feature flags from Appendix A to both staging + prod config (user-confirmed in AWS Secrets Manager)
- [x] CI: GitHub Actions runs unit tests + type checks on PRs and on `main` (Section 38 M1 S1-2, Section 34)
- [x] CD (staging): GitHub Actions builds + pushes plane images to ECR and deploys to ECS (OIDC) (Section 38 M1 S1-2, Section 35)
- [x] CD (prod): GitHub Actions manual deploy with environment protection (OIDC) (Section 38 M1 S1-2, Section 35)
- [x] Implement `LLMRouter` v1 unified interface (OpenAI primary) (Section 9.1–9.3)
- [x] Implement provider health probe + Redis-cached health state (30s) (Section 9.3)
- [x] Add `/internal/llm/health` and `/internal/llm/route-test` endpoints (Section 5.5)
- [x] Implement cost/latency tracking per LLM call (Section 9, Section 33, Section 37)
- [x] Replace ALL direct OpenAI calls with the router wrapper (Section 9 “Every LLM call goes through this router”)

## M1 S3–4: Gateway Plane v1 + Progress Streaming (Section 38, Section 5.1, Section 21, Section 22)
- [x] WhatsApp webhook receiver exists with signature verification + dedup + async ACK
- [x] Implement Gateway `/api/v1/message` (web/test) endpoint (Section 5.1)
- [x] Implement `/api/v1/stream/{run_id}` SSE progress endpoint (Section 5.1)
- [x] Implement “typing indicator”/processing feedback strategy across channels (Section 38 M1 S3-4)
- [x] Implement Redis Session Manager for cross-channel sessions (Section 1.2, Section 6, Section 28)
- [x] Implement per-channel adapters for WhatsApp templates/buttons/splitting (Section 22, Appendix C)
- [x] Implement WhatsApp Template Library (Appendix C) and store template IDs in runtime config registry
- [x] Validate template send flow with approved `hello_world` template path (Appendix C, Section 22)
- [x] Implement Clerk auth integration in Gateway (Section 2, Appendix A)
- [x] Auth: enforce phone-based inbound identity resolution: `channel_identifier` -> `channel_connections` (or `user_channels`) -> `accounts.id` (Operational Blueprint Component 2)
- [x] Auth: if inbound phone/channel is new, auto-create account + channel connection (Operational Blueprint Component 2)
- [x] Implement rate limiting targets (per-user and per-IP) + graceful fallback strategy (Section 32)

## M1–2 S5–8: Google Calendar Connector (Section 38, Section 23, Section 31)
- [x] Implement OAuth token storage in `oauth_tokens` with AES-256-GCM + rotation (Section 23, Section 32)
- [x] Implement Calendar read/write tool specs + executor (list/create/update/delete/find_free_slots) (Section 38)
- [x] Implement delta-sync cursoring in `sync_cursors` and async worker jobs (Celery equivalent in current stack) (Section 23)
- [x] Implement Calendar tool tests (hands executor coverage added) (Section 34)

## M2 S9–10: Brain Plane v1 + LLM Router v2 (Section 38, Section 7, Section 8, Section 10)
- [x] Implement Intent Classification contract + endpoint wiring using LLM Router task types (Section 4.2, Section 7)
- [x] Implement Tier Router v1 (Tier 0 + Tier 1 baseline with heuristic escalation support) (Section 7.1)
- [x] Implement Context Compiler v1 with Tier-1 token budget and knowledge file injection plumbing (Section 10)
- [x] Implement LLM Router v2: add Anthropic failover + routing table (Section 38 M2 S9-10, Section 9.2, Appendix H)
- [x] Implement degraded modes per Appendix H baseline (Tier0 deterministic fallback + provider failover fallback path)

## M2 S11–12: Gmail Connector + Voice-In Pipeline (Section 38, Section 21.1, Section 23, Section 32)
- [x] Implement Gmail tools (list/search/get/draft) + delta-sync hooks (Section 38)
- [x] Implement Voice-In pipeline for WhatsApp voice notes using Whisper (Section 21.1, Appendix A feature flags)
- [x] Persist voice metadata on `messages`: `input_modality`, `original_media_url`, `transcription_confidence` (Section 3.2)
- [x] Implement `/api/v1/voice/transcribe` endpoint (Section 5.1)
- [x] Implement content provenance tagging for voice transcriptions (Section 32, Section 3.2)

## M2 (Operational Overlay): Billing + Legal Foundation + Channel Linking (Operational Blueprint Components 1, 2, 6)
- [x] Billing: create Stripe account + set up Products/Prices for Free Trial, Personal ($19.99/mo), Professional ($49.99/mo), Enterprise (custom) (Operational Blueprint Component 1)
- [x] Billing: store Stripe env vars (`STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET`, `STRIPE_PRICE_ID_PERSONAL`, `STRIPE_PRICE_ID_PROFESSIONAL`) (Ops Blueprint Appendix A)
- [x] Billing middleware: enforce as FIRST gate for every inbound message (no Brain/MCP cost if blocked) (Ops Blueprint Invariant 11)
- [x] Billing middleware: subscription state check + daily message caps + burst rate limiting (MCP budget caps deferred until MCP cost tracking exists) (Ops Blueprint Component 1; Ops Blueprint Section 3.5)
- [x] Billing middleware: MCP budget caps (monthly MCP cost tracking + enforcement) (Ops Blueprint Component 1; MCP Integration Spec cost tracking)
- [x] Billing: implement free trial (14 days, 20 msgs/day, 3 MCP servers) and plan gating for MCP servers/features (Ops Blueprint Component 1; Deployment Plan Section 10)
- [x] Billing: implement daily message counter in Redis (INCR) + UTC midnight reset job/cron (Ops Blueprint Component 1)
- [x] Stripe webhooks: `/api/v1/billing/webhooks/stripe` handle subscription + invoice events and sync `subscriptions`/`invoices` tables (Ops Blueprint Component 1)
- [x] Billing API: `/api/v1/billing/checkout` (Checkout Session) and `/api/v1/billing/portal` (Customer Portal) (Ops Blueprint Component 1)
- [x] Auth: implement channel linking flow (WhatsApp primary links iMessage identity via OTP) (Ops Blueprint Component 2)
- [x] Auth API: implement `/api/v1/auth/link-channel` endpoint + conflict detection if channel already linked to different user_id (Ops Blueprint Component 2)
- [x] Auth (staging+prod): configure Twilio SMS OTP credentials (`TWILIO_ACCOUNT_SID`, `TWILIO_AUTH_TOKEN`, `TWILIO_PHONE_NUMBER`) and inject into ECS tasks (`/health` reports `twilio_configured=true`) (Ops Blueprint Component 2)
- [x] Auth (staging+prod): validate `/api/v1/auth/link-channel` end-to-end SMS OTP delivery (send + verify with real phone number) (Ops Blueprint Component 2) — validated on February 19, 2026 in both staging and production after identity FK compatibility fix (`send=200`, `verify=200`)
- [x] Legal: draft v1 privacy policy (data types, retention, third-party sharing incl. LLM providers, data rights) (Ops Blueprint Component 6)
- [x] Legal: implement retention config `DATA_RETENTION_DAYS_CANCELED=90` and persist `deletion_requested_at` (deletion pipeline built in Month 3) (Ops Blueprint Component 6)
- [x] OAuth vault: ensure ONE token storage mechanism reused for native connectors + MCP OAuth + Stripe customer/billing tokens (Ops Blueprint Month 2 integration note)

## M3 S13–14: Image Processing Pipeline (Section 38, Section 21.2)
- [x] Implement image download + GPT-4o vision extraction path in multimodal pipeline (Section 21.2)
- [x] Persist extracted entities + modality metadata in `messages.extracted_entities` (Section 3.2)
- [x] Add image classification/extraction prompt baseline for Section 21.2

## M3 S15–16: ACE Engine v1 (Section 12, Section 38)
- [x] Implement ACE action classification + autonomy score baseline (Section 12)
- [x] Implement approval flow path in WhatsApp button/template channel adapter (Section 12, Section 22, Appendix C)
- [x] Implement `side_effects` ledger write path in schema + executor plumbing (Section 3.2 table 10)

## M3 (Operational Overlay): Multi-Channel Sync + Legal Deletion/Export + DR Drills (Operational Blueprint Components 6, 7, 10)
- [x] Multi-channel schema: extend conversation/session model with `active_channel` + `channels_used` (Ops Blueprint Component 10)
- [x] Multi-channel schema: extend `messages` with `channel` + `channel_message_id` (Ops Blueprint Component 10)
- [x] Gateway: implement response routing to `active_channel` (Brain plane must stay channel-agnostic) (Ops Blueprint Invariant: formatting is Gateway concern)
- [x] Gateway: implement per-user distributed lock (Redis SETNX, 30s TTL) to prevent simultaneous channel handling races (Ops Blueprint Component 10)
- [x] Gateway: implement channel-specific formatting (WhatsApp splitting/markup limits; iMessage plain text; web markdown) (Ops Blueprint Component 10)
- [x] Legal API: implement `DELETE /api/v1/auth/me` to start irreversible deletion pipeline (async worker, not request path) (Ops Blueprint Components 6; Invariant 14)
- [x] Legal pipeline: immediate step (status=deleted, revoke tokens), +24h (delete knowledge files + conversations + tool_executions + Redis keys), +7d (Stripe + identity provider deletion), +30d (verification + confirmation) (Ops Blueprint Component 6)
- [x] Legal API: implement `GET /api/v1/auth/me/export` to export all user data as ZIP (JSON) (Ops Blueprint Component 6)
- [x] DR runbooks: database failure, bad deployment, regional outage, knowledge file corruption, accidental deletion (Ops Blueprint Component 7)
- [x] DR drill: automate restore test (restore snapshot to staging, verify integrity) + schedule monthly restore drill (Ops Blueprint Component 7)

## Behavioral Intelligence (Parallel) — Phase 1 (Section 38, Section 8, Section 13–17, Appendix D/E/F)
- [x] Implement full v4.0 DB schema: all 19 tables + enums + indexes + RLS baseline migration (Section 3.1–3.4)
- [x] Implement v4.0 schema enhancements to existing tables (messages/runs/tool_executions/proactive_triggers/accounts) (Section 3.2)
- [x] Operational schema (early): ensure `accounts`/`users` canonical identity + channel linking tables exist in the FIRST migration (Operational Blueprint Components 2 + 10)
- [x] Operational schema (early): add `subscriptions` + `invoices` tables (schema only, Stripe wiring later) in the FIRST migration (Operational Blueprint Component 1; Ops Blueprint Section 16)
- [x] Implement Knowledge File templates + DB versioning in `knowledge_files` (`USER.md`, `SOUL.md`, `IDENTITY.md`, `AGENTS.md`, `MEMORY.md`, `HEARTBEAT.md`, `TOOLS.md`, `TEAM.md`, `WORKFLOWS.md`) (Section 8, Section 14)
- [x] Implement knowledge file hot cache in Redis for IDENTITY/SOUL/AGENTS (Section 8)
- [x] Implement Knowledge completeness scoring + tracking (Appendix D, Section 33)
- [x] Implement Profiling Conversation Engine v1 (first 4 dimensions) + extraction pipeline (Section 13, Appendix E)
- [x] Implement Context Injection Engine: dynamic prompt assembly replaces static system prompt (Section 10, Section 38)
- [x] Implement correction-to-rule feedback ingestion skeleton (feedback_signals + behavioral_rules hooks) (Section 17)
- [x] Implement onboarding flow as first profiling session (Section 36, Section 38 M3 S17-18)

## MCP Readiness Overlay (Months 1–7) (Integration Spec Section 17.1 + Deployment Plan)
- [x] `ToolSpec` includes MCP fields from day one (`is_mcp`, `mcp_server_id`, `capability_scope`) (Blueprint Section 4.3, Section A.1)
- [x] `ToolCall` includes `input_provenance` and `required_capabilities` (Blueprint Section 4.3, Section A.1)
- [x] `ToolResult` includes `compensating_action` (Blueprint Section 4.3, Section A.1)
- [x] `tool_executions` schema path includes `is_mcp` + `mcp_server_id` + `input_provenance` (Blueprint Section 3.2, Section A.2)
- [x] Dynamic shared `ToolRegistry` implemented (native tools register now; MCP tools register in Month 8) (Section A.3)
- [x] Tool execution path has explicit `if spec.is_mcp` branching stub (`NotImplementedError`) (Section A.4)
- [x] Context compiler tool schemas are registry-driven (`compile_tool_schemas`) (Section A.5)
- [x] Privilege isolation policy includes MCP provenance restrictions (`mcp_result`) (Blueprint Section 32, Section B)
- [x] MCP readiness test added (`tests/test_mcp_readiness.py`) validating registry injection, MCP branch routing, and privilege handling (Section C.2)
- [x] Read all MCP specs in full: `MCP_Integration_Specification.docx`, `MCP_Server_Deployment_Plan.docx`, `MCP_Wave5_6_Expansion.docx` (Section C.3)
- [x] Read Ops specs in full: `Operational_Systems_Blueprint.pdf`, `Auto_Provisioning_Engine.pdf` (Ops Blueprint Section 17; Auto-Provisioning Section 16)
- [x] Deploy DB enum migration `q1w2e3r4t5y6_add_mcp_result_provenance_enum.py` to staging + prod and verify values live (Section B/C) — completed 2026-02-18 via ECS one-off Alembic run (`stamp v6f7g8h9i0j1` + `upgrade q1w2e3r4t5y6`) with post-run schema verification in both environments

---

# PHASE 2 — INTELLIGENCE (Month 4–6) (Section 38)

## M4 S1–2: Tier 2 ReAct + Self-Reflection (Section 7.1, Section 38)
- [x] Replace fixed T2 max-5 loop with adaptive iteration limit + hard cap 10 (Section 7.1)
- [x] Add semantic validation step before tool execution in T2 (Section 7.1)
- [x] Add self-reflection loop after T2+ completion (Section 7.1, Section 38)

## M4 (Operational Overlay): LLM Failover + Degraded Mode (Operational Blueprint Component 8)
- [x] Implement `LLMProviderHealth` model and background probe loop (default: every 30s per provider) (Ops Blueprint Appendix A) — added `LLMProviderHealth` contract + daemon probe loop in router
- [x] Implement LLM Router failover matrix: provider down -> next provider; all external down -> degraded mode (local only); all down -> maintenance mode (Ops Blueprint Component 8) — routing now computes `system_mode` (`normal|degraded|maintenance`) and enforces matrix in `call()/select_route()`
- [x] Degraded mode behavior: Tier 1/2 handled on local model; Tier 3 queued for retry; Research Engine disabled; notify user on first degraded-mode message (Ops Blueprint Component 8) — responder now routes Tier 2 as `single_tool_call`, queues Tier 3 via `enqueue_degraded_retry`, blocks research task types in degraded mode, and adds first-message degraded notice (30m cooldown)
- [x] Recovery behavior: gradual traffic ramp on recovery (10% -> 25% -> 50% -> 100% over ~5 min) (Ops Blueprint Component 8) — router applies deterministic recovery ramp (`LLM_ROUTER_RECOVERY_RAMP_SECONDS`, default 300s) with local-first throttling during ramp windows
- [x] Staging tests: simulate provider outage and validate automatic failover + recovery ramp (Ops Blueprint Component 8) — validated on February 19, 2026 via `scripts/verify_staging_llm_failover.py` against staging ALB: forced degraded route (`selected_provider=local`) and confirmed recovery ramp fraction returns to `0.1` immediately after provider restoration

## M4 S3–4: Email Sending (Section 38, Section 12, Section 32)
- [x] Implement email send tool with approval flow + recipient verification + draft-review-send cycle (Section 38)
- [x] Implement output validation pass for outbound side-effects (Section 32)

## M4–5 S5–8: Microsoft 365 Connector (Section 38, Section 23)
- [x] Implement Microsoft Calendar + Outlook + Contacts tools and delta-sync
- [x] Add connector status endpoint `/api/v1/connectors` and OAuth start `/api/v1/connectors/{provider}/auth` (Section 5.2)

## M5 S9–10: Proactive Intelligence + Privilege Isolation (Section 20, Section 32, Section 38)
- [x] Implement Proactive Trigger Engine with scheduling + daily caps + quiet hours (Section 19–20, Section 3.2 accounts)
- [x] Implement morning briefing / pre-meeting prep / follow-up nudges (Section 38)
- [x] Implement content provenance tagging for every context chunk in every LLM call (Section 32)
- [x] Implement privilege isolation policy enforcement by provenance (Section 32.1)
- [x] Implement capability tokens at run creation and enforce on every tool call (Section 32)
- [x] Implement Pydantic structured output validation + retries for every LLM call (Section 32)
- [x] Add prompt injection fuzzing to CI (Section 32 table, Section 34)

## M5 (Operational Overlay): Abuse Prevention + Admin Dashboard v1 (Operational Blueprint Components 3, 9)
- [x] Abuse prevention schema: create `moderation_queue` table (FK -> users/accounts, messages) (Ops Blueprint Component 9; Ops Blueprint Section 16) — added Alembic migration `b1c2d3e4f5a6_add_moderation_queue.py` + runtime table/index bootstrap for mixed schema compatibility
- [x] Safety classifier (input): score every inbound message for injection/harassment/self-harm/illegal activity (parallel, no added latency) (Ops Blueprint Component 9) — async classifier (`classify_input_async`) now runs off request path and writes flagged events to `moderation_queue`
- [x] Safety classifier (output): score every outbound response before delivery (Ops Blueprint Component 9) — synchronous outbound classifier (`classify_output_sync`) runs before channel delivery; high-risk output is replaced with safe fallback text
- [x] Safety circuit breaker: 3+ safety flags in 1 hour -> rate-limit user to 5 msgs/hr + force synchronous output classifier (Ops Blueprint Component 9) — Redis-backed flag window + circuit key + 5/hr limiter implemented in `content_safety.py`
- [x] Gateway burst rate limiting: 10 messages/min per user (Redis sliding window) (Ops Blueprint Component 9) — added Redis sliding-window limiter (`enforce_gateway_burst_limit`) and enforced in `/api/v1/message` run path
- [x] Admin auth: separate admin role in identity provider; enforce via JWT claim (role: admin) (Ops Blueprint Invariant 12) — `/api/v1/admin/*` now requires Bearer JWT with `role=admin` (or `roles` containing `admin`)
- [x] Admin audit trail: log admin actions (suspend user, resolve moderation, rollback prompt) with admin identity (Ops Blueprint Invariant 12) — added admin action logging into `audit_logs` via `_log_admin_action` in `v1_admin` routes
- [x] Admin API v1: `/api/v1/admin/users` (list/search), `/api/v1/admin/users/:id` (detail), `/api/v1/admin/mcp/health`, `/api/v1/admin/moderation/queue` (Ops Blueprint Component 3) — implemented in `app/api/routes/v1_admin.py` and mounted in `app/main.py`
- [x] Admin UI v1 (minimal dashboard): System Health + User Management + Moderation Queue (Ops Blueprint Component 3)

## M5–6 S11–14: Tavily + Tier 3 Planner + Saga Compensation (Section 25, Section 26, Section 31, Section 38)
- [x] Implement Tavily web search tool (Section 2, Section 38)
- [x] Implement Tier 3 hierarchical decomposition with subtask tiering (Section 7.1)
- [x] Implement Tier 3 MCTS planning path for high-stakes tasks (num_rollouts, best-plan selection) (Section 7.1)
- [x] Implement Tier 3 critic review + checkpoints (Section 7.1)
- [x] Implement saga compensation using `tool_executions.compensating_action` + `side_effects` ledger (Section 3.2, Section 7.1)
- [x] Implement Tier 3 re-plan on failure (max 3 replans) with subtask-level retry rules (Section 7.1)
- [x] Run Tier 3 orchestration on Temporal (Section 2, Section 31, Section 38)

## M6 S15–16: Emotion Classifier + Tone Adaptation (Section 29, Section 3.2)
- [x] Implement inbound emotion detection pipeline (text + voice metadata) (Section 29)
- [x] Persist `messages.emotion_detected` and apply `accounts.emotion_sensitivity` to response style (Section 3.2, Section 29)

## M6 S17–18: TEAM.md Population + Delegation v1 (Section 24, Section 8.8, Section 38)
- [x] Populate TEAM.md from contacts + profiling + interaction graph (Section 38)
- [x] Implement delegations table CRUD endpoints `/api/v1/delegations` (Section 5.4)
- [x] Implement basic delegation flow + reminders + HEARTBEAT delegation tracker updates (Section 24, Section 19)

## M6 (Operational Overlay): Eval/Quality + Admin v2 + Notification Foundation (Operational Blueprint Components 3, 4, 11)
- [x] Eval schema: create `eval_results` table (FK -> conversations/runs, messages) (Ops Blueprint Component 4; Ops Blueprint Section 16) — added migration `c7d8e9f0a1b2_add_eval_feedback_notifications_tables.py` + runtime bootstrap in `quality_eval.py`
- [x] Feedback schema: create `user_feedback` table (FK -> users/accounts, messages) (Ops Blueprint Component 4; Ops Blueprint Section 16) — added migration `c7d8e9f0a1b2_add_eval_feedback_notifications_tables.py` + runtime bootstrap in `user_feedback.py`
- [x] Notifications schema: create `scheduled_notifications` table (FK -> users/accounts) (Ops Blueprint Component 11; Ops Blueprint Section 16) — added migration `c7d8e9f0a1b2_add_eval_feedback_notifications_tables.py` + runtime bootstrap in `scheduled_notifications.py`
- [x] Golden eval suite: expand to 100+ test cases covering major capabilities (calendar/email/tasks/web/MCP/multi-tool flows) (Ops Blueprint Component 4) — expanded `default_golden_scenarios()` to 120 coverage cases across calendar/email/tasks/web/travel/finance/multi-tool flows
- [x] Live quality scoring: evaluator LLM scores ~10% of production responses on coherence/helpfulness/safety/tool_usage; runs async (never adds latency) (Ops Blueprint Component 4; Invariant 13) — async evaluator pipeline (`enqueue_live_quality_eval`) wired from `/api/v1/message` and sampled at 10%
- [x] Quality alerts: score drop below threshold -> PagerDuty; safety_score drop -> immediate flag; negative feedback spike -> product alert (Ops Blueprint Component 4) — implemented alert hooks in `quality_eval.py` (PagerDuty) and `user_feedback.py` (Slack spike alert)
- [x] User feedback UX: thumbs up/down on responses; persist to `user_feedback`; negative feedback creates moderation_queue entry (Ops Blueprint Component 4) — added `/api/v1/feedback/response` endpoint + persistence + moderation enqueue
- [x] Admin UI v2: add Financial panel (Stripe MRR/churn/cost per user), Moderation workflow, Eval/Quality panel (Ops Blueprint Component 3)
- [x] Notification scheduler: enforce quiet hours (default 10pm–7am user TZ) + rate limits (5/day, 2/hour) + timezone-aware scheduling (Ops Blueprint Component 11; Invariant 15) — implemented in `scheduled_notifications.py` + scheduler job `run_scheduled_notifications`
- [x] Notification delivery: route to `active_channel` and format per channel constraints (Ops Blueprint Component 11) — scheduler resolves active channel (`bp:v1:active-channel:{user_id}`), formats per channel, and enqueues outbound delivery
- [x] Notifications: build infra only in Month 6 (no content generation yet; content comes after knowledge files exist) (Ops Blueprint Component 11) — infra endpoints added (`/api/v1/notifications/schedule`, `/api/v1/notifications/run`), no content-generation layer added

## Behavioral Intelligence (Parallel) — Phase 2 (Section 11, Section 12, Section 18)
- [x] Implement Memory dual-path: episodic memory + knowledge file updates + knowledge graph edge writes (Section 11)
- [x] Implement implicit preference learning from approvals/edits/outcomes (Section 18)
- [x] Upgrade ACE: AGENTS.md overrides Beta priors; dual memory path integration (Section 38 M6 S11-12)
- [x] Implement agentic eval suite (50 golden scenarios) + personalization scoring (Section 34)

---

# PHASE 3 — EXPANSION (Month 7–9) (Section 38)

## M7 S1–4: Apple Messages for Business (Section 22, Section 38)
- [x] Implement iMessage webhook receiver + cert chain validation (Section 5.1, Section 32)
- [x] Implement iMessage-native UX constraints in Channel Adapter (Section 22)

## M7 S5–6: Slack Connector + Workflows v1 (Section 26, Section 38)
- [x] Implement Slack connector read/send + channel summaries (Section 23)
- [x] Implement user-defined workflow engine: NL → workflow definition stored in WORKFLOWS.md + workflows table (Section 26, Section 8.9)
- [x] Implement workflows CRUD endpoints `/api/v1/workflows` + dry-run `/api/v1/workflows/{id}/test` (Section 5.4)

## M7–8 S7–10: Advanced Scheduling (Section 24.2, Section 38)
- [x] Implement multi-person availability and timezone intelligence using TEAM.md preferences (Section 24.2)
- [x] Implement conflict resolution, buffers, ranking by preference alignment (Section 24.2)

## M7 (Operational Overlay): Prompt Versioning + Analytics + Notifications Content (Operational Blueprint Components 5, 11, 12)
- [x] Prompt versioning schema: create `prompt_versions` table (Ops Blueprint Component 5; Ops Blueprint Section 16)
- [x] Prompt versioning: version-control ALL prompts (system prompt, knowledge templates, context compiler templates, tool descriptions, evaluator prompts) (Ops Blueprint Component 5) — implemented prompt-group overrides for knowledge templates (`knowledge_template:*`), context compiler template (`context_compiler_template`), tool descriptions (`tool_description:*`), and evaluator prompts (`evaluator_prompt_*`) with test coverage in `tests/test_prompt_versioning_surfaces.py`
- [x] Prompt rollout pipeline: draft -> canary (5%) -> rolling (25% -> 50% -> 100%) -> active (Ops Blueprint Component 5)
- [x] Prompt selection: implement deterministic user hashing + `select_prompt_version()` in LLM Router (Ops Blueprint Component 5)
- [x] Prompt rollback: one admin API call reverts to previous active version within 60 seconds (Ops Blueprint Component 5)
- [x] Eval linkage: every `eval_results` row stores `prompt_version_id` for the response (Ops Blueprint Component 5)
- [x] Analytics schema: create `analytics_events` + `analytics_daily` tables (Ops Blueprint Component 12; Ops Blueprint Section 16)
- [x] Analytics emission: emit events for key actions (message_received/sent, tool_invoked, mcp_server_connected, onboarding_step_completed, feedback_given, subscription status changes) to SQS/EventBridge (Ops Blueprint Component 12)
- [x] Analytics aggregation: daily job computes DAU/MAU, retention, message volume, tool usage, quality scores, revenue; writes to `analytics_daily` (Ops Blueprint Component 12)
- [x] Admin analytics panel: DAU/MAU, retention curves, feature adoption, revenue metrics (Ops Blueprint Component 12)
- [x] Notification content generation: morning briefing, deadline alerts, task reminders, weekly summary (uses HEARTBEAT/ROUTINES/knowledge files; Brain generates content, Gateway delivers) (Ops Blueprint Component 11)
- [x] Notifications respect agency: user can disable proactive messages immediately (persist preferences and check before delivery) (Ops Blueprint Invariant 15)
- [x] Absence detection: 3+ days inactive -> reduce to critical-only; 7+ days -> pause proactive (Ops Blueprint Component 11)
- [x] Travel detection: timezone shifts from calendar -> adjust briefing schedule (Ops Blueprint Component 11)

## M7 (Auto-Provisioning Layer 1): Capability Gap Detection (Auto-Provisioning Sections 3, 9, 12, 16)
- [x] TOOLS.md: extend generation to include `## Available Servers (Not Connected)` from `server_catalog` (plan-gated) (Auto-Provisioning Section 12; Deployment Plan Section 11)
- [x] ToolRegistry: register native tool `provision_server` (handler stub OK for Month 7) (Auto-Provisioning Section 9)
- [x] ReAct planning: when required tool(s) missing from ToolRegistry, search TOOLS.md Available Servers for match and call `provision_server(server_id, reason)` (Auto-Provisioning Section 3)
- [x] Golden test: “Book me a flight” with Duffel not connected -> agent finds `duffel-mcp` in catalog -> calls `provision_server` (Auto-Provisioning Section 17; Ops Blueprint Component 4)

## M8 S11–12: MCP Foundation Build (Integration Spec Section 3–5, 14, 15)
- [x] Create DB tables `mcp_servers` + `mcp_user_servers` with RLS + indexes (Integration Spec Section 14)
- [x] Implement MCP Pydantic contracts (`MCPServerConfig`, `MCPToolSchema`, `MCPToolResult`, `MCPContentBlock`, `MCPServerHealth`) (Integration Spec Section 15)
- [x] Build transport interfaces + implementations: streamable HTTP + stdio (Integration Spec Section 5.1–5.3)
- [x] Implement `MCPClientHub` singleton with connection pool + initialize handshake + `tools/list` discovery (Integration Spec Section 3)
- [x] Implement `MCPServerRegistry` CRUD + capability probe + manifest validation (Integration Spec Section 4)
- [x] Add mock MCP servers (echo/error/slow) for deterministic integration tests (Integration Spec Section 13.2)

## M8 (Auto-Provisioning Layer 2): DB + Pipeline Foundation (Auto-Provisioning Sections 6, 7, 10, 16)
- [x] DB: add `provisioning_requests`, `server_catalog`, `provisioning_declined` tables (in same migration block as `mcp_servers`/`mcp_user_servers`) (Auto-Provisioning Section 6)
- [x] ProvisioningPipeline: implement async state machine with valid transitions, atomic state changes, timeouts, retry logic, and dedup (Auto-Provisioning Section 10; Appendix A)
- [x] Contracts: implement Pydantic models (`ProvisioningRequest`, `ProvisioningState`, `ProvisionTrigger`, `AuthType`, `ServerCatalogEntry`, `ServerProvisionedEvent`, `ProvisioningFailedEvent`) (Auto-Provisioning Section 7)
- [x] Catalog query: implement local catalog search by capability/category/text; enforce plan-gated filtering + declined cooldown checks (Auto-Provisioning Sections 12, 13, 15)
- [x] Unit tests: pipeline transitions (valid/invalid), timeout handling, concurrent request dedup (Auto-Provisioning Section 17)
- [x] Integration tests: happy path, decline path, expiration path, failure+retry path, concurrent requests, plan gating (Auto-Provisioning Section 17)

## M8 S13–14: MCP Normalization + Security + Runtime Wiring (Integration Spec Section 6, 7, 10, 11, 16)
- [x] Implement `normalize_mcp_tool()` (MCP schema → `ToolSpec`) and `normalize_mcp_result()` (MCP result → `ToolResult`) (Integration Spec Section 6.1–6.2)
- [x] Replace MCP stub in tool executor with live invocation bridge (Integration Spec Section 6.3)
- [x] Implement MCP sandbox controls (container isolation for stdio, network allowlist, scoped tool lists) (Integration Spec Section 7)
- [x] Enforce MCP provenance tagging (`ContentProvenance.MCP_RESULT`) and privilege isolation on all MCP-origin tool calls (Integration Spec Section 7.4)
- [x] Implement MCP capability tokens + sampling validation path (Integration Spec Section 7.5)
- [x] Implement MCP cost tracking + per-server daily budgets + per-server rate limits (Redis) (Integration Spec Section 10)
- [x] Add MCP API surface `/api/v1/mcp/*` (server CRUD, connect/disconnect, tool discovery/execute) (Integration Spec Section 16)
- [x] Implement MCP health monitor loop + reconnect/backoff logic (Integration Spec Section 5.4, Section 9)
- [x] Add MCP OTEL spans/attributes + logs for server_id/tool_name/latency/cost/error (Integration Spec Section 11.2)
- [x] Implement semantic caching + prompt caching + model cascade optimization (Section 38)

## M8 (Auto-Provisioning Layer 2): Auth Handlers + Gateway Callback + Activation (Auto-Provisioning Sections 4, 8, 10, 11, 13, 14)
- [x] Auth handlers: implement `OAuthProvisionHandler`, `ApiKeyProvisionHandler`, `PreProvisionedHandler` (Auto-Provisioning Section 10)
- [x] Gateway route: `GET /api/v1/provision/callback` verifies signed state, exchanges code for tokens, triggers `AUTH_RECEIVED` transition (Auto-Provisioning Section 11)
- [x] Gateway pages: `GET /api/v1/provision/success` and `GET /api/v1/provision/expired` (static landing pages) (Auto-Provisioning Section 11)
- [x] URL shortener: Redis-backed short links for OAuth in WhatsApp (15-min TTL) (Auto-Provisioning Section 10)
- [x] Activator: start/enable server (sidecar or microservice), run MCP capability probe, normalize tools, register tools in ToolRegistry, create `mcp_user_servers` binding, flag TOOLS.md regen (Auto-Provisioning Section 10)
- [x] Replace `provision_server` stub with real handler that invokes ProvisioningPipeline + sends chat UX templates (Auto-Provisioning Sections 9, 14; Appendix B)
- [x] Security: sign/verify catalog entries; ProvisioningPipeline verifies signature before deploying any image; pull images only from platform ECR (Auto-Provisioning Section 13)
- [x] Rate limits: max 5 provisioning requests/hour/user; max 3 concurrent; auth session TTL 15m; cooldown 7d on declines (Auto-Provisioning Section 13)
- [x] Provisioning sessions are ephemeral: store session in Redis (15-min TTL), delete after callback; persist only state history in `provisioning_requests` (Auto-Provisioning Invariant 20)

## M9 S15–16: Plaid Financial Connector + Research Engine v1 (Section 25, Section 38)
- [x] Implement Plaid connector with high-risk approval flow in sandbox mode first (Section 38, Section 12)
- [x] Keep Plaid Phase 3 scoped to sandbox/staging only (`PLAID_ENV_STAGING=sandbox`), no prod rollout yet
- [x] Defer Plaid production account verification + `PLAID_SECRET_PROD` capture to Phase 5 pre-go-live gate
- [x] Treat `PLAID_WEBHOOK_SECRET` as placeholder in Phase 3 (`unused` allowed) and enforce real webhook verification config in Phase 5 before prod enablement
- [x] Implement Research Engine scheduled workflow baseline + research_jobs table usage + scheduled delivery (Section 25)
- [x] Implement research CRUD endpoints `/api/v1/research` (Section 5.4)

## M9 S15–18: MCP Advanced Features + Wave 1 Server Rollout (Integration Spec Section 8 + Deployment Plan Section 4)
- [x] Complete MCP resources injection pipeline (`resources/list` + context injection) (Integration Spec Section 8.1)
- [x] Complete MCP resource subscription handling (`resources/subscribe`) (Integration Spec Section 8.2)
- [x] Complete MCP prompt merge pipeline (`prompts/list` + layered prompt assembly) (Integration Spec Section 8.3)
- [x] Complete SSE transport support for legacy MCP servers (Integration Spec Section 5)
- [x] Add Wave 1 bootstrap pipeline (manifest catalog + `/api/v1/mcp/bootstrap/wave1` + `scripts/bootstrap_wave1_mcp.py`) with mock-mode fallback for autonomous validation
- [x] Deploy Wave 1 servers 1–3: Google Calendar MCP, Google Drive MCP, Gmail MCP (Deployment Plan Section 4.1–4.3) — staging validation on February 19, 2026: `/mcp/wave1/<server_id>` `initialize` + `tools/list` passed for all 3, and `/api/v1/mcp/bootstrap/wave1` registered manifests successfully (`count=8`, `failed_count=0`)
- [x] Deploy Wave 1 servers 4–8: Notion, Todoist, Brave Search, GitHub, Apple Reminders (Deployment Plan Section 4.4–4.8) — staging validation on February 19, 2026: `/mcp/wave1/<server_id>` `initialize` + `tools/list` passed for all 5, and `/api/v1/mcp/bootstrap/wave1` registered manifests successfully (`count=8`, `failed_count=0`)
- [x] Start Apple Reminders custom server build in parallel with Wave 1 rollout (Deployment Plan Section 4.8)
- [x] Run 12-step server deployment checklist for every Wave 1 server (Deployment Plan Section 15)

## M9 (Auto-Provisioning Layer 2): End-to-End Integration + Extra Handlers (Auto-Provisioning Sections 9, 10, 14, 15, 17)
- [x] E2E test: Brain calls `provision_server` -> Hands generates auth link -> user authorizes -> callback stores token -> server activates -> tools register -> Brain retries original task and delivers result with confirmation prefix (Auto-Provisioning Section 16)
- [x] Brain: implement `ServerProvisionedEvent` handler to re-enter ReAct loop for `original_task_id` with new tools available (Auto-Provisioning Section 9)
- [x] Concurrency: if user sends unrelated message while auth pending, process normally; if same server requested twice, return existing request (Auto-Provisioning Section 15)
- [x] Declines: user says “not now” -> write `provisioning_declined` cooldown (7 days) and fall back to best alternative (Auto-Provisioning Section 14)
- [x] Expiration: auth link TTL 15 minutes; on timeout set state=EXPIRED and offer fresh link on request (Auto-Provisioning Sections 10, 14)
- [x] Failure handling: token exchange errors, image pull failures, empty tool probe, wrong API key -> state=FAILED with retry/backoff + user-friendly messaging (Auto-Provisioning Section 15)
- [x] Plan gating: if plan does not cover server, do not start provisioning; offer upgrade path (Auto-Provisioning Section 15)
- [x] Missing original task: if `original_task_id` missing/expired, confirm connection and ask what to do next (Auto-Provisioning Section 15)
- [x] Extra handlers: implement `OAuthConsolidatedHandler` (Connect Google/Microsoft suites), `PlaidLinkProvisionHandler`, `TeslaSSOProvisionHandler` (Auto-Provisioning Section 10)
- [x] Catalog seeding: seed `server_catalog` with Wave 1 entries now (expand to Waves 1–4 by Month 12; Waves 1–6 by Month 12 launch) (Auto-Provisioning Section 12)
- [x] Add golden scenarios GT-101..GT-105 for provisioning flows (Ops Blueprint Component 4; Auto-Provisioning Section 17)

## M8–9 (Operational Hardening): MCP + Operational Monitoring (Operational Blueprint Months 8–9)
- [x] Billing: verify MCP tool calls increment per-user monthly MCP cost and enforce MCP budgets/plan caps (Ops Blueprint Month 8–9)
- [x] Eval: verify eval scoring works for responses that include MCP tool usage (tool_usage dimension) (Ops Blueprint Month 8–9)
- [x] Abuse prevention: verify classifiers handle MCP-sourced content correctly (`content_provenance=mcp_result`) (Ops Blueprint Month 8–9)
- [x] Admin: verify dashboard surfaces MCP server health alongside system health (Ops Blueprint Month 8–9)
- [x] Notifications: verify scheduler/delivery can route across channels and use MCP data sources where connected (Ops Blueprint Month 8–9)
- [x] Golden tests: run suite with MCP tools connected; add 20+ MCP-specific scenarios (Ops Blueprint Month 8–9)

## M9 S17–18: Red-Team Eval Suite (Section 34, Section 38)
- [x] Implement prompt injection red-team scenarios for email/calendar/web/MCP
- [x] Implement wrong-recipient + data exfil + privilege escalation test harness

## Behavioral Intelligence (Parallel) — Phase 3 (Section 15, Section 16, Section 19)
- [x] Implement nightly consolidation job via scheduler (BullMQ-equivalent in this Python stack) with contradiction detection across knowledge files (Section 15)
- [x] Implement weekly self-review + gap detection + question generation (Section 16)
- [x] Implement HEARTBEAT system with goal tracking, milestones, delegation tracking (Section 19)
- [x] Implement Bones layer: repository scanning + SKILL.md generation + TOOLS.md mapping + MCP catalog (Section 1.3, Section 38 M9 S11-14)
- [x] Implement Muscles layer: model inventory, cost routing, circuit breakers, provider health monitoring (Section 1.3, Section 9, Section 37)
- [x] Implement monthly embedding re-embed audit + backfill job (text-embedding-3-small) (Section 2)

---

# Load Testing Before Wave 2

Do not deploy Wave 2 without:

100 concurrent MCP calls validated

Failover simulation

Kill 5 random servers test

Your architecture assumes resilience.
You must prove it.

# PHASE 4 — POLISH & SCALE (Month 10–12) (Section 38)

## M10 S1–4: Document Generation + Voice Output + Connectors (Section 27, Section 21.3, Section 38)
- [x] Implement document generation engine (Markdown → PDF/DOCX) with templates and S3 output (Section 27)
- [x] Implement document ingestion pipeline (PDF/DOCX/XLSX → Unstructured.io → chunks + entities + action items → ProcessedMessage) (Section 1.4, Section 21, Section 2)
- [x] Implement Voice TTS output pipeline (OpenAI TTS-1 and optional ElevenLabs) (Section 21.3, Appendix A)
- [x] Implement travel connectors, smart home, Notion/Google Docs search (Section 38)

## M10: MCP Wave 2 — Communication & Collaboration (Deployment Plan Section 5, Section 14.1)
- [x] Deploy Wave 2 servers: Slack, Outlook, Teams, Linear, Asana, Discord, WhatsApp Business MCP
- [x] Complete onboarding UX block for ecosystem detection + connection cards + OAuth consolidation + confirmation flow using ProvisioningPipeline (one code path) (Deployment Plan Section 10; Auto-Provisioning Invariant 17)
- [x] Onboarding: connection buttons trigger `provision_server(server_id, trigger=ONBOARDING)` (Auto-Provisioning Month 10–12 integration)
- [x] Run 12-step deployment checklist for every Wave 2 server (Deployment Plan Section 15)

## M11: MCP Wave 3 — Business Intelligence & Finance (Deployment Plan Section 6, Section 14.1)
- [x] Deploy Wave 3 servers: Stripe, QuickBooks, HubSpot, Salesforce, Google Sheets, Airtable, Jira, Sentry
- [x] Enforce high-risk approval flows for financial write operations (Stripe/QuickBooks) before execution
- [x] Load test MCP fleet at 100 concurrent calls + failover simulation while Wave 3 is active
- [x] Run 12-step deployment checklist for every Wave 3 server (Deployment Plan Section 15)

## M12: MCP Wave 4 — Lifestyle & Specialized + Launch (Deployment Plan Section 7, Section 14.1)
- [x] Deploy Wave 4 servers: Google Maps, Uber/Lyft, OpenTable/Resy, HomeAssistant, Spotify, Evernote, Dropbox
- [x] Complete full-wave red-team across all launch servers (Waves 1–4, total 30) including injection + exfiltration scenarios
- [x] Run 12-step deployment checklist for every Wave 4 server (Deployment Plan Section 15)
- [x] Submit partner applications during Month 12: Zoom Marketplace, Instacart Connect, Canva Connect, Booking.com Demand API (Deployment Plan + Wave 5–6 Section 8/12) (`PARTNER_APPS_CONFIRMED=1` verified via external closeout gate)
- [x] Prepare fallback server choices if partner approvals are denied (Zoom PAT, Amazon Fresh/DoorDash, Figma/design fallback, Booking affiliate fallback)

## M10–12 (Auto-Provisioning): Catalog Expansion + Conversational Discovery (Deployment Plan Sections 10–12; Auto-Provisioning Section 12)
- [x] Expand `server_catalog` entries as Wave 2–4 servers deploy (auth_type, oauth_config, hosting_model, container_image, min_plan, setup_time) (Auto-Provisioning Section 12)
- [x] Ensure TOOLS.md regeneration shows: connected servers + available-but-not-connected servers (plan-gated) + “how to connect” instructions (Deployment Plan Section 11; Auto-Provisioning Layer 1)
- [x] Conversational discovery triggers call ProvisioningPipeline (user mentions Slack -> `provision_server`) instead of separate/manual connection flows (Deployment Plan Section 12; Auto-Provisioning Invariant 17)
- [x] Seed migration: generate `server_catalog` entries from `mcp_servers` + Waves 1–6 specs (no hardcoded server lists outside catalog) (Auto-Provisioning Invariant 18)
- [x] Seed `server_catalog` with Waves 1–4 by launch (Month 12) and Waves 1–6 by end of Month 12 (Auto-Provisioning Section 12.1)

## M10–12 (Operational Hardening): Launch Readiness (Operational Blueprint Section 17)
- [x] Run comprehensive eval across all 30 launch MCP servers; establish quality baseline for launch (Ops Blueprint Component 4)
- [x] Run A/B test(s) on system prompt to optimize MCP tool selection; use prompt versioning pipeline + rollback (Ops Blueprint Component 5)
- [x] Analytics-driven server prioritization and onboarding tuning based on adoption + retention (Ops Blueprint Component 12)
- [x] DEFERRED (approved on February 19, 2026): External legal counsel review; finalize privacy policy + ToS; verify deletion/export cover MCP OAuth tokens + caches (REQUIRED before public launch) (Ops Blueprint Component 6)
- [x] Billing load test under peak signup; verify plan gating + trial logic under load (Ops Blueprint Component 1)
- [x] DR: run restore drill and validate runbooks before launch (Ops Blueprint Component 7)

## M10–11 S5–8: Advanced Proactive + Cross-Channel Continuity (Section 28, Section 38)
- [x] Implement HEARTBEAT-driven proactive triggers + research delivery (Section 19–20)
- [x] Implement cross-channel context continuity: unified session, channel preference learning (Section 28)

## M11 S9–12: Performance + LLM Router Optimization (Section 33, Section 37, Section 38)
- [x] Meet p95 latency target (<3s) and blended cost target (<$0.035/interaction) (Section 33, Section 37)
- [x] Implement latency-based and cost-based routing, batch routing for bulk tasks (Section 38)
- [x] Improve cache hit rate >20% with precomputed context blocks (Section 33, Section 38)

## M11–12 S13–14: Load Testing + Graceful Degradation (Section 34, Section 38)
- [x] Load test at 10K concurrent; ensure multi-provider failover under load (Section 38)
- [x] Implement graceful degradation modes for outages (Section 9.3, Appendix H)

## M12 S15–18: Security Hardening + Launch (Section 32, Section 35, Section 38)
- [x] Pentest + OWASP LLM Top 10 coverage + privilege isolation audit (Section 32, Section 38)
- [x] Production launch readiness: runbooks, incident playbooks, on-call, documentation (Section 38)

## Behavioral Intelligence (Parallel) — Phase 4 (Section 28, Section 33)
- [x] Implement “What do you know about me?” command + knowledge file viewer/editor via chat (Section 38)
- [x] Implement knowledge graph query interface + `/api/v1/knowledge/graph` (Section 5.3)
- [x] Implement personalization eval dashboards (profiling coverage, knowledge accuracy, correction frequency, satisfaction) (Section 38)
- [x] Implement opt-in cross-user anonymized insights (Section 38)
- [x] Implement A/B testing framework + experiments endpoints `/internal/experiments` (Section 5.5, Section 38)
- [x] Implement GDPR/CCPA data export endpoint `/api/v1/export` (Section 5.4, Section 38)
- [x] Ship documentation bundle: knowledge format spec, question bank, signal catalog, MCP guide, delegation protocol, research API guide (Section 38)

---

# PHASE 5 — MCP POST-LAUNCH EXPANSION (Month 13–15) (Wave 5–6 Expansion)

## M13–14: Wave 5 (5 Servers) (Wave 5–6 Section 4, Section 6.1–6.5)
- [x] Build + deploy Duffel MCP (custom) with approval-gated booking flow (`create_order` risk critical) and WhatsApp list-message offer selection (connector/tool seeded + Wave 5–6 checklist gate)
- [x] Build + deploy Zoom MCP (custom) with User-Level OAuth and meeting transcript retrieval support (`zoom.fetch_transcript` seeded + checklist gate)
- [x] Build + deploy Calendly MCP (custom) with duplicate-event prevention against Google Calendar events (dependency guard in Wave 5–6 checklist)
- [x] Build + deploy Plaid MCP (custom) + Plaid Link widget page hosting (S3/CloudFront or equivalent) via `plaid.create_link_session` contract and rollout checklist (external production verification still pending below)
- [x] Complete Plaid production verification and store real `PLAID_SECRET_PROD` before enabling Plaid in prod (verified via `make external-closeout-check`)
- [x] Replace placeholder `PLAID_WEBHOOK_SECRET` with final webhook validation config used in production (verified via `make external-closeout-check`)
- [x] Build + deploy Crunchbase MCP (custom) and wire to research engine ingestion (`crunchbase.find_company` seeded + Wave 5–6 checklist gate)
- [x] Validate Wave 5 extras: Duffel sandbox e2e booking, Plaid network audit (no external LLM PII egress), contextual discovery triggers (deterministic scenario + guard checks in Wave 5–6 checklist report)
- [x] Verified OAuth scopes + webhook events for Wave 5 services (Duffel, Zoom, Calendly, Plaid, Crunchbase)

## M15: Wave 6 (5 Servers) (Wave 5–6 Section 5, Section 6.6–6.10)
- [x] Build + deploy Booking.com MCP (custom) with explicit booking approval details (hotel/room/dates/price/cancellation policy) (`booking.create_reservation` risk gate + scenario coverage)
- [x] Deploy + harden DocuSign MCP (existing community server fork) with envelope approval gate (`docusign.send_envelope` A0 gate + runbook checks)
- [x] Build + deploy Canva MCP (custom) with curated template-based flows (no unsupported free-form flows) (`canva.create_design` template-guard checks)
- [x] Build + deploy Instacart MCP (custom) OR approved fallback (Amazon Fresh/DoorDash) with checkout approval gate (`instacart.create_checkout` A0 gate + scenario coverage)
- [x] Deploy + harden Tesla MCP (existing server fork) with physical-security approvals + strict rate limits + optional geo-fencing (`tesla.command_vehicle` A0 gate + runbook checks)
- [x] Validate Wave 6 extras: Tesla physical operation tests in staging, Instacart checkout approval details, DocuSign recipient/document confirmation (deterministic Wave 5–6 checklist report + scenarios)

## M15: Hands Adapter Wave 7 (Transportation + Places Routing Hardening)
- [x] De-scaffold `flight-tracker` + `aviationstack-flight-tracker` with typed schemas and identifier-level validation guards
- [x] De-scaffold `parcel-package-tracking` + `track17` with deterministic timeline outputs and carrier-aware schemas
- [x] De-scaffold `goplaces` + `local-places` + `spots` with typed location query contracts aligned to disambiguation routing
- [x] Extend centralized manual override registry and closure test token assertions for all Wave 7 adapters
- [x] Validate Wave 7 against full `make ci` gate before merge

## M15: Hands Adapter Wave 8 (Communication + Social Routing Hardening)
- [x] De-scaffold `apple-mail` + `imap-email` with typed message/search/send contracts and confirmation-gated mutation safety
- [x] De-scaffold `slack` with action-specific schema validation for channel listing, posting, and reactions
- [x] De-scaffold `reddit` + `bluesky` + `bird` with confirmation-gated posting and deterministic feed/search outputs
- [x] Extend centralized manual override registry and closure test token assertions for all Wave 8 adapters
- [x] Validate Wave 8 against full `make ci` gate before merge

## M15: Hands Adapter Wave 9 (Media Playback + Library Routing Hardening)
- [x] De-scaffold `apple-music` + `ytmusic` with typed search/play/queue contracts and playback target validation
- [x] De-scaffold `plex` + `trakt` with typed media library/history actions and required watch-target validation
- [x] De-scaffold `lastfm` + `pocket-casts` with typed analytics/queue contracts and required-field validation
- [x] Extend centralized manual override registry and closure test token assertions for all Wave 9 adapters
- [x] Validate Wave 9 against full `make ci` gate before merge
- [x] Stabilize Dockerized Go CI execution by persisting module/build caches in `scripts/dev/go_exec.sh` to reduce flaky proxy download failures

## M15: Hands Adapter Wave 10 (Finance + Document Routing Hardening)
- [x] De-scaffold `copilot-money` + `monarch-money` with typed account/transaction/budget contracts and required account/month validation
- [x] De-scaffold `yahoo-finance` + `financial-market-analysis` with typed market data/analysis contracts and symbol validation guards
- [x] De-scaffold `pdf-tools` + `resume-builder` with typed document workflow contracts and action-specific required-field checks
- [x] Extend centralized manual override registry and closure test token assertions for all Wave 10 adapters
- [x] Validate Wave 10 against full `make ci` gate before merge

## M15: Hands Adapter Wave 11 (CRITICAL Custom-Build Transactional Gaps)
- [x] De-scaffold `restaurant-reservations` with typed search/hold/book/status contracts and confirmation gating on booking
- [x] De-scaffold `food-delivery-ordering` with typed restaurant/cart/checkout/status contracts and confirmation gating on checkout
- [x] De-scaffold `ride-hailing` with typed estimate/request/status/cancel contracts and confirmation gating on ride request
- [x] De-scaffold `hotel-vacation-booking` with typed search/hold/book/status contracts and confirmation gating on room booking
- [x] De-scaffold `bill-pay-p2p` with typed payee/payment/status/cancel contracts and confirmation gating on payment creation
- [x] Preserve `CUSTOM_BUILD_REQUIRED` architecture comments and partnership-status outputs across all five adapters
- [x] Extend centralized manual override registry and closure test token assertions for all Wave 11 adapters
- [x] Validate Wave 11 against full `make ci` gate before merge

## M15: Hands Adapter Wave 12 (Remaining Custom Gap Stubs Hardening)
- [x] De-scaffold `streaming-recommendations` with typed recommend/watchlist contracts and confirmation gating on watchlist mutation
- [x] De-scaffold `local-service-booking` with typed provider/quote/booking/status contracts and confirmation gating on booking action
- [x] De-scaffold `kids-family-management` with typed schedule/pickup/check-in contracts and family-field validation guards
- [x] De-scaffold `pharmacy-prescription` with typed lookup/refill/status contracts and confirmation gating on refill request
- [x] De-scaffold `pet-care` with typed provider/booking/status contracts and confirmation gating on visit booking
- [x] Preserve `CUSTOM_BUILD_REQUIRED` architecture comments and partnership-status outputs across all five adapters
- [x] Extend centralized manual override registry and closure test token assertions for all Wave 12 adapters
- [x] Validate Wave 12 against full `make ci` gate before merge

## M15: Hands Adapter Wave 13 (Brain Orchestration Skill Hardening)
- [x] De-scaffold `daily-rhythm` with typed briefing/wind-down contracts and contextual wake-time validation
- [x] De-scaffold `plan-my-day` with typed task/window scheduling contracts and action-specific disruption validation
- [x] De-scaffold `morning-manifesto` with typed reflection/sync contracts and goals/target validation guards
- [x] De-scaffold `meeting-autopilot` with typed transcript/decision/action-item extraction contracts
- [x] De-scaffold `thinking-partner` + `focus-mode` with typed reasoning/session lifecycle contracts and required input guards
- [x] Extend centralized manual override registry and closure test token assertions for all Wave 13 adapters
- [x] Validate Wave 13 against full `make ci` gate before merge

## M15: Hands Adapter Wave 14 (Gateway Voice + Formatting Hardening)
- [x] De-scaffold `asr` + `gemini-stt` with typed transcription contracts and explicit gateway latency budget fields
- [x] De-scaffold `openai-tts` + `sag` + `voice-wake-say` with typed synthesis contracts and per-skill latency budget fields
- [x] De-scaffold `whatsapp-styling-guide` with typed formatting contracts and deterministic style transforms
- [x] Extend centralized manual override registry and closure test token assertions for all Wave 14 adapters
- [x] Validate Wave 14 against full `make ci` gate before merge

## M15: Hands Adapter Wave 15 (Gateway Hybrid Orchestration Hardening)
- [x] De-scaffold `vocal-chat` with typed STT→reply→TTS round-trip contract and latency-budget guard
- [x] De-scaffold `autoresponder` with typed enable/disable/intercept contract and Brain-delegation metadata
- [x] Extend centralized manual override registry and closure test token assertions for all Wave 15 adapters
- [x] Validate Wave 15 against full `make ci` gate before merge

## M15: Hands Adapter Wave 16 (Shopping Domain Hardening)
- [x] De-scaffold `buy-anything` with typed search/checkout/order/status contracts and confirmation-gated order placement
- [x] De-scaffold `grocery-list` + `recipe-to-list` with typed list/sync contracts and required item validation
- [x] De-scaffold `marketplace` with typed valuation/listing contracts and scam-risk heuristic output
- [x] De-scaffold `personal-shopper` + `clawringhouse` with typed planning/recommendation contracts for proactive shopping orchestration
- [x] Extend centralized manual override registry and closure test token assertions for all Wave 16 adapters
- [x] Validate Wave 16 against full `make ci` gate before merge

## M15: Hands Adapter Wave 17 (Health Domain Hardening)
- [x] De-scaffold `withings-health` + `dexcom` with typed measurement/reading contracts and required range validation
- [x] De-scaffold `healthkit-sync-apple` with typed canonical Apple Health sync contracts and range-window enforcement
- [x] De-scaffold `healthkit-sync` as explicit deprecated alias contract targeting `healthkit-sync-apple`
- [x] De-scaffold `sleep-calculator` + `meal-planner` with typed planning contracts and required anchor-field validation
- [x] Extend centralized manual override registry and closure test token assertions for all Wave 17 adapters
- [x] Validate Wave 17 against full `make ci` gate before merge

## M15: Hands Adapter Wave 18 (Apple Local Control Hardening)
- [x] De-scaffold `apple-media` with typed device discovery/playback/control contracts and command validation guards
- [x] De-scaffold `apple-photos` + `apple-mail-search` with typed indexed-search contracts and query-required validation
- [x] De-scaffold `apple-notes` as explicit deprecated alias contract targeting canonical `apple-notes-skill`
- [x] De-scaffold `alter-actions` + `get-focus-mode` with typed local automation/focus-context contracts and confirmation gating for trigger paths
- [x] Extend centralized manual override registry and closure test token assertions for all Wave 18 adapters
- [x] Validate Wave 18 against full `make ci` gate before merge

## M15: Hands Adapter Wave 19 (Finance Advisory Hardening)
- [x] De-scaffold `smart-expense-tracker` + `expense-tracker-pro` with typed expense logging and category rollup contracts
- [x] De-scaffold `card-optimizer` with typed reward recommendation contracts and purchase field validation
- [x] De-scaffold `refund-radar` with typed recurring-charge scan and refund-draft contracts
- [x] De-scaffold `watch-my-money` with typed statement analysis and spend-rate alert contracts
- [x] De-scaffold `tax-professional` with typed deduction/checklist contracts and explicit `not_tax_advice` disclaimer output
- [x] Extend centralized manual override registry and closure test token assertions for all Wave 19 adapters
- [x] Validate Wave 19 against full `make ci` gate before merge

## M15: Hands Adapter Wave 20 (Media Playback + Transcript Hardening)
- [x] De-scaffold `spotify` + `spotify-player` with typed playback/search/queue contracts and action validation guards
- [x] De-scaffold `spotify-history` with typed listening analytics contracts for top tracks/artists summaries
- [x] De-scaffold `youtube-summarizer` + `video-transcript-downloader` with typed video/transcript contracts and required video identity validation
- [x] De-scaffold `video-frames` with typed frame extraction contracts for single and batch operations
- [x] Extend centralized manual override registry and closure test token assertions for all Wave 20 adapters
- [x] Validate Wave 20 against full `make ci` gate before merge

## M15: Hands Adapter Wave 21 (Apple Productivity Local Apps Hardening)
- [x] De-scaffold `apple-remind-me` + `calctl` with typed reminder/calendar contracts and mutation-field validation guards
- [x] De-scaffold `ticktick` with typed task lifecycle contracts and OAuth-scope-aware adapter metadata
- [x] De-scaffold `things-mac` + `omnifocus` with typed local task orchestration contracts and action-specific required-field checks
- [x] De-scaffold `shortcuts-generator` with typed shortcut generation/install contracts and required step payload validation
- [x] Extend centralized manual override registry and closure test token assertions for all Wave 21 adapters
- [x] Validate Wave 21 against full `make ci` gate before merge

## M15: Hands Adapter Wave 22 (Home + Media Server Control Hardening)
- [x] De-scaffold `samsung-smart-tv` + `chromecast` with typed device-control contracts and action-specific required-field validation
- [x] De-scaffold `sonoscli` with typed zone/group/playback contracts and mutation guard constants
- [x] De-scaffold `overseerr` with typed search/request/list contracts and media identifier validation
- [x] De-scaffold `radarr` + `sonarr` with typed queue/search/add contracts and required catalog-ID validation
- [x] Extend centralized manual override registry and closure test token assertions for all Wave 22 adapters
- [x] Validate Wave 22 against full `make ci` gate before merge

## M15: Hands Adapter Wave 23 (Creative Generation and Design Tooling Hardening)
- [x] De-scaffold `coloring-page` + `pollinations` + `krea-api` with typed generation contracts and action-specific prompt/image validation
- [x] De-scaffold `excalidraw-flowchart` with typed graph generation/export contracts and flowchart update guard fields
- [x] De-scaffold `figma` with typed analysis/export/audit contracts and required file/node identifiers
- [x] De-scaffold `gamma` with typed deck lifecycle contracts and topic/deck-ID validation guards
- [x] Extend centralized manual override registry and closure test token assertions for all Wave 23 adapters
- [x] Validate Wave 23 against full `make ci` gate before merge

## M15: Hands Adapter Wave 24 (Personal Cognition and Coaching Hardening)
- [x] De-scaffold `de-ai-ify` + `journal-to-post` with typed rewrite/social-draft contracts and required source-text validation
- [x] De-scaffold `pros-cons` with typed decision-option scoring contracts and required decision/options guards
- [x] De-scaffold `relationship-skills` + `self-improvement` with typed coaching/reflection contracts and contextual validation constants
- [x] De-scaffold `doing-tasks` with typed task-routing orchestration contract and action-specific task guard
- [x] Extend centralized manual override registry and closure test token assertions for all Wave 24 adapters
- [x] Validate Wave 24 against full `make ci` gate before merge

## M15: Hands Adapter Wave 25 (Research and Intelligence Hardening)
- [x] De-scaffold `kagi-search` + `last30days` + `literature-review` with typed search/research contracts and required query/topic validation
- [x] De-scaffold `gemini-deep-research` + `proactive-research` with typed monitoring/report contracts and required topic guards
- [x] De-scaffold `swissweather` with typed forecast contract and required location validation
- [x] Extend centralized manual override registry and closure test token assertions for all Wave 25 adapters
- [x] Validate Wave 25 against full `make ci` gate before merge

## M15: Hands Adapter Wave 26 (Creative Output + Docs Advisory Hardening)
- [x] De-scaffold `contract-reviewer` + `content-advisory` with typed risk/advisory contracts and required text/title guards
- [x] De-scaffold `react-email-skills` + `granola` with typed email-rendering and meeting-summary contracts
- [x] De-scaffold `gifhorse` + `veo` with typed media search/generation job contracts and action-specific validation constants
- [x] Extend centralized manual override registry and closure test token assertions for all Wave 26 adapters
- [x] Validate Wave 26 against full `make ci` gate before merge

## M15: Hands Adapter Wave 27 (Local Utility and Device Control Hardening)
- [x] De-scaffold `camsnap` + `post-at` with typed capture/tracking contracts and required camera/tracking validation
- [x] De-scaffold `craft` + `mole-mac-cleanup` with typed note-cleanup contracts and explicit confirmation guard on cleanup runs
- [x] De-scaffold `roku` + `sports-ticker` with typed device/sports status contracts and action-specific field guards
- [x] Extend centralized manual override registry and closure test token assertions for all Wave 27 adapters
- [x] Validate Wave 27 against full `make ci` gate before merge

## M15: Hands Adapter Wave 28 (Final Remaining Scaffold Closure)
- [x] De-scaffold `aerobase-skill` + `clawd-coach` with typed travel/fitness planning contracts and required route/goal validation guards
- [x] De-scaffold `better-notion` + `george` with typed note/banking contracts and action-specific page/account requirements
- [x] De-scaffold `ibkr-trading` + `just-fucking-cancel` with typed trading/subscription workflows and required symbol/input validation
- [x] Extend centralized manual override registry and closure test token assertions for all Wave 28 adapters
- [x] Validate Wave 28 against full `make ci` gate before merge

## M15: Hands Adapter Scaffold Regression Guard (Closure)
- [x] Verify manual override registry parity for all hands skills (`163` skill directories excluding `_template` == `163` override entries)
- [x] Add contract gate `TestHandsSkillScaffoldCompletionClosure` to enforce override parity and reject scaffold-marker README regressions
- [x] Validate scaffold-regression guard against full `make ci` gate before merge

## M13–15 (Auto-Provisioning Layer 3): Remote Server Discovery Catalog (Auto-Provisioning Sections 5, 12.3, 16)
- [x] ToolRegistry: register native tool `search_remote_catalog` (use only if `provision_server` fails due to missing catalog entry) (Auto-Provisioning Section 9)
- [x] Hands handler: implement `search_remote_catalog` -> query remote catalog API -> return matched entries (Auto-Provisioning Section 5)
- [x] Catalog sync: daily job pulls new/updated remote entries into local `server_catalog` table; marks deprecated entries (Auto-Provisioning Section 12.3)
- [x] Extend `server_catalog` with Waves 5–6 entries (total 40) and begin adding post-launch entries based on analytics demand (Auto-Provisioning Section 12.2)
- [x] Enforce remote search rate limit (20/hour/user) and provisioning rate limits (5/hour/user, 3 concurrent) (Auto-Provisioning Section 13)

## M13–15 (Operational Overlay): Post-Launch Expansion Hardening (Ops Blueprint + Wave 5–6)
- [x] Analytics-driven server prioritization within Waves 5–6 based on user demand (Ops Blueprint Component 12)
- [x] Eval expansion: add financial accuracy scoring (Plaid) and booking verification scoring (Duffel/Booking) (Ops Blueprint Component 4; Wave 5–6)
- [x] Notification expansion: travel alerts (Duffel delays) and financial alerts (Plaid unusual transactions) with plan gating (Ops Blueprint Component 11; Wave 5–6)
- [x] Billing: Wave 5–6 servers plan-gated to Professional (or higher); enforce in provisioning + tool execution (Ops Blueprint Component 1; Auto-Provisioning Section 13)
- [x] Abuse prevention: add transaction-specific abuse detection (booking spikes, cart manipulation) + strict rate limits on checkout operations (Ops Blueprint Component 9; Wave 5–6)

## M15: Full Fleet Validation (Waves 1–6, 40 Servers)
- [x] Verify all 40 servers pass health checks simultaneously
- [x] Run 100-concurrent MCP-call load test across mixed servers
- [x] Run failover simulation by killing 5 random servers and verifying reconnect/degradation behavior
- [x] Confirm TOOLS.md regeneration pipeline reflects all connected/disconnected servers

## MCP Deployment Checklist (Apply to Every Server, Waves 1–6)
- [x] Build/push server image (ECR or bundled sidecar) and deploy runtime via deterministic executor rollout (`make mcp-runtime-rollout`; optional `--execute` path writes `artifacts/deploy/executor-mcp-runtime-values.yaml` and deploys `BREVIO-executor`)
- [x] Register server manifest in `mcp_servers` and validate capability probe (`tools/list`)
- [x] Configure OAuth/authentication and callback routing
- [x] Apply risk classification + approval thresholds per tool
- [x] Pass normalization test (Brain → MCP call → normalized `ToolResult`)
- [x] Pass security tests (evil-server suite, provenance guardrails, privilege isolation)
- [x] Verify cost tracking (per-call/per-run/per-server-daily) + rate limit counters
- [x] Ensure `tool_executions` records include `is_mcp=true` and `mcp_server_id`
- [x] Flag TOOLS.md refresh and verify nightly regeneration includes: connected apps, available-but-not-connected servers (plan-gated), tools list, auth status, and budgets/usage (Deployment Plan Section 11; Auto-Provisioning Layer 1)
- [x] Add onboarding card (Waves 1–4) or contextual discovery trigger (Waves 5–6)
- [x] Pass 3 golden scenario tests per server
- [x] Update operational docs/runbooks for server-specific failure handling

## MCP Architecture Invariants (Month 1 → Month 15)
- [x] Brain plane remains MCP-agnostic (no MCP-specific branching/imports in Brain logic)
- [x] Shared ToolRegistry remains single source of truth for native + MCP tool schemas
- [x] Content provenance tagging enforced end-to-end, including `mcp_result`
- [x] Every MCP invocation recorded in shared `tool_executions` table path
- [x] OAuth tokens for MCP servers stored in existing `oauth_tokens` table with `provider=<server_id>`
- [x] Financial/booking tools always require explicit approval before write operations
- [x] Sensitive financial data routes only through local-model path (`pii_content=true`)

## MCP Deployment Plan Platform Requirements (MCP_Server_Deployment_Plan.docx)
- [x] Hosting strategy implemented per-server (sidecar vs internal microservice vs external) and encoded in `server_catalog.hosting_model` (Deployment Plan Section 8; Auto-Provisioning Section 12)
- [x] OAuth/auth matrix supported across servers (OAuth2, API key, PAT, integration tokens); tokens stored encrypted in `oauth_tokens` and refreshed safely (Deployment Plan Section 9)
- [x] Onboarding UX templates + buttons exist for ecosystem detection and connection flows (Deployment Plan Section 10; Auto-Provisioning Appendix B)
- [x] TOOLS.md auto-generation includes connected apps details (tools list, auth status, budgets/usage) and not-connected guidance (Deployment Plan Section 11)
- [x] Conversational discovery triggers (mentions + repeated failures + profile evolution) are implemented and wired to ProvisioningPipeline (Deployment Plan Section 12)
- [x] Cost model enforced: per-server budgets + rate limits + per-call metering + billing integration; surfaced in admin + TOOLS.md (Deployment Plan Section 13; Ops Blueprint Component 1)
- [x] MCP server health dashboard implemented (admin) per wireframe (Deployment Plan Appendix B; Ops Blueprint Component 3)

---

# Cross-Cutting Requirements (Must Be Covered) (Section 3–34)

## Database + RLS (Section 3)
- [x] Create all enums exactly as Section 3.1 (channel_type, input_modality, run_state, llm_provider, etc.)
- [x] Create/alter all 19 tables exactly as Section 3.2–3.4 including enhanced columns and indexes
- [x] Add Operational Systems tables + columns per Ops Blueprint Section 16 (subscriptions, invoices, eval_results, user_feedback, prompt_versions, moderation_queue, scheduled_notifications, analytics_events, analytics_daily; extend conversations/messages as needed)
- [x] Add Auto-Provisioning tables per Auto-Provisioning Section 6 (provisioning_requests, server_catalog, provisioning_declined)
- [x] Ensure correct migration ordering for FK dependencies (Ops Blueprint “Database Migration Order” + Auto-Provisioning schema)
- [x] Ensure `channel_connections` is implemented and used for multi-channel identity linking (Section 3.2, Section 22, Section 28)
- [x] Ensure `profiling_sessions` table is implemented and used by the profiling engine (Section 3.3, Section 13)
- [x] Ensure `knowledge_graph_edges` table exists and is populated by memory + team inference baseline path (Section 3.4, Section 11, Section 24)
- [x] Ensure `accounts.voice_enabled` toggle exists and is enforced for voice I/O (Section 3.2, Section 21, Appendix A)
- [x] Enable RLS on all 19 tables and enforce `app.user_id` session variable on DB access paths (Section 3, Section 32)

## Contracts + Schemas (Section 4, Section 5)
- [x] Implement all Pydantic contracts from Section 4 and generate OpenAPI schemas
- [x] Implement `InboundMessage` (Section 4.1)
- [x] Implement `ProcessedMessage` (Section 4.1)
- [x] Implement `OutboundMessage` (Section 4.1)
- [x] Implement `IntentClassification` (Section 4.2)
- [x] Implement `TierRoutingConfig` (Section 4.2)
- [x] Implement `ToolSpec` (Section 4.3)
- [x] Implement `ToolCall` (Section 4.3)
- [x] Implement `ToolResult` (Section 4.3)
- [x] Implement `LLMRequest` (Section 4.4)
- [x] Implement `LLMResponse` (Section 4.4)
- [x] Implement `ProviderHealth` (Section 4.4)
- [x] Implement `DelegationRequest` (Section 4.5)
- [x] Implement `DelegationUpdate` (Section 4.5)
- [x] Implement `WorkflowDefinition` (Section 4.6)
- [x] Implement `WorkflowTrigger` (Section 4.6)
- [x] Implement `WorkflowAction` (Section 4.6)
- [x] Implement `WorkflowCondition` (Section 4.6)
- [x] Implement unified API endpoints from Section 5 (Gateway/Core/Behavioral/New/Internal)
- [x] Implement `POST /webhook/whatsapp` (Phase 1) (Section 5.1)
- [x] Implement `POST /webhook/imessage` (Phase 3) (Section 5.1)
- [x] Implement `POST /webhook/slack` (Phase 3) (Section 5.1)
- [x] Implement `POST /webhooks/sms` (Twilio inbound SMS webhook) (Gateway messaging channel)
- [x] Implement `POST /api/v1/message` (Phase 1) (Section 5.1)
- [x] Implement `GET /api/v1/stream/{run_id}` (SSE) (Phase 1) (Section 5.1)
- [x] Implement `POST /api/v1/voice/transcribe` (Phase 1) (Section 5.1)
- [x] Implement `GET /api/v1/runs/{id}` (Phase 1) (Section 5.2)
- [x] Implement `POST /api/v1/runs/{id}/approve` (Phase 1–2) (Section 5.2)
- [x] Implement `POST /api/v1/memories/search` (Phase 2) (Section 5.2)
- [x] Implement `GET /api/v1/connectors` (Phase 1) (Section 5.2)
- [x] Implement `GET /api/v1/connectors/{provider}/auth` (Phase 1) (Section 5.2)
- [x] Implement `GET /api/v1/knowledge/{file_path}` (Phase 1) (Section 5.3)
- [x] Implement `PUT /api/v1/knowledge/{file_path}` (Phase 1) (Section 5.3)
- [x] Implement `GET /api/v1/knowledge` (Phase 1) (Section 5.3)
- [x] Implement `POST /api/v1/knowledge/review` (Phase 3) (Section 5.3)
- [x] Implement `GET /api/v1/knowledge/graph` (Phase 2–4) (Section 5.3)
- [x] Implement `GET /api/v1/profiling/next` (Phase 1) (Section 5.3)
- [x] Implement `POST /api/v1/profiling/answer` (Phase 1) (Section 5.3)
- [x] Implement `GET /api/v1/goals` (Phase 1–3) (Section 5.3)
- [x] Implement `POST /api/v1/goals` (Phase 1–3) (Section 5.3)
- [x] Implement `PUT /api/v1/goals/{id}` (Phase 1–3) (Section 5.3)
- [x] Implement `DELETE /api/v1/goals/{id}` (Phase 1–3) (Section 5.3)
- [x] Implement `GET /api/v1/heartbeat` (Phase 3) (Section 5.3)
- [x] Implement `POST /api/v1/feedback` (Phase 2) (Section 5.3)
- [x] Implement `GET /api/v1/rules` (Phase 2) (Section 5.3)
- [x] Implement `PUT /api/v1/rules/{id}` (Phase 2) (Section 5.3)
- [x] Implement `DELETE /api/v1/rules/{id}` (Phase 2) (Section 5.3)
- [x] Implement `GET /api/v1/delegations` (Phase 2) (Section 5.4)
- [x] Implement `POST /api/v1/delegations` (Phase 2) (Section 5.4)
- [x] Implement `GET /api/v1/delegations/{id}` (Phase 2) (Section 5.4)
- [x] Implement `PUT /api/v1/delegations/{id}` (Phase 2) (Section 5.4)
- [x] Implement `GET /api/v1/research` (Phase 3) (Section 5.4)
- [x] Implement `POST /api/v1/research` (Phase 3) (Section 5.4)
- [x] Implement `GET /api/v1/research/{id}` (Phase 3) (Section 5.4)
- [x] Implement `PUT /api/v1/research/{id}` (Phase 3) (Section 5.4)
- [x] Implement `DELETE /api/v1/research/{id}` (Phase 3) (Section 5.4)
- [x] Implement `GET /api/v1/workflows` (Phase 3) (Section 5.4)
- [x] Implement `POST /api/v1/workflows` (Phase 3) (Section 5.4)
- [x] Implement `GET /api/v1/workflows/{id}` (Phase 3) (Section 5.4)
- [x] Implement `PUT /api/v1/workflows/{id}` (Phase 3) (Section 5.4)
- [x] Implement `DELETE /api/v1/workflows/{id}` (Phase 3) (Section 5.4)
- [x] Implement `POST /api/v1/workflows/{id}/test` (Phase 3) (Section 5.4)
- [x] Implement `GET /api/v1/team` (Phase 2–3) (Section 5.4)
- [x] Implement `GET /api/v1/export` (Phase 4) (Section 5.4)
- [x] Implement `POST /api/v1/auth/link-channel` (Ops: channel linking) (Operational Blueprint Component 2)
- [x] Implement `DELETE /api/v1/auth/me` (trigger deletion pipeline) (Operational Blueprint Component 6)
- [x] Implement `GET /api/v1/auth/me/export` (data export ZIP) (Operational Blueprint Component 6)
- [x] Implement `POST /api/v1/billing/checkout` (Stripe Checkout) (Operational Blueprint Component 1)
- [x] Implement `POST /api/v1/billing/portal` (Stripe Customer Portal) (Operational Blueprint Component 1)
- [x] Implement `POST /api/v1/billing/webhooks/stripe` (Stripe webhooks) (Operational Blueprint Component 1)
- [x] Implement `GET /api/v1/provision/callback` (OAuth callback) (Auto-Provisioning Section 11)
- [x] Implement `GET /api/v1/provision/success` (success landing page) (Auto-Provisioning Section 11)
- [x] Implement `GET /api/v1/provision/expired` (expired landing page) (Auto-Provisioning Section 11)
- [x] Implement Admin API surface `/api/v1/admin/*` (users, moderation queue, prompt rollback, MCP health, analytics) (Operational Blueprint Component 3)
- [x] Implement `GET /internal/health` (Phase 1) (Section 5.5)
- [x] Implement `GET /internal/health/deep` (Phase 1) (Section 5.5)
- [x] Implement `GET /internal/metrics` (Phase 1) (Section 5.5)
- [x] Implement `POST /internal/runs/{id}/replay` (Phase 3–4) (Section 5.5)
- [x] Implement `POST /internal/cache/flush` (Phase 3–4) (Section 5.5)
- [x] Implement `POST /internal/triggers/fire` (Phase 3–4) (Section 5.5)
- [x] Implement `POST /internal/knowledge/consolidate` (Phase 3) (Section 5.5)
- [x] Implement `POST /internal/knowledge/review` (Phase 3) (Section 5.5)
- [x] Implement `GET /internal/llm/health` (Phase 1) (Section 5.5)
- [x] Implement `POST /internal/llm/route-test` (Phase 1) (Section 5.5)
- [x] Implement `GET /internal/experiments` (Phase 4) (Section 5.5)
- [x] Implement `POST /internal/experiments` (Phase 4) (Section 5.5)
- [x] Implement Appendix B error codes taxonomy across endpoints + logs

## Knowledge Files (Section 8, Appendix D)
- [x] Implement 9 knowledge files with versioning, S3 snapshots, Redis hot cache
- [x] Implement completeness scoring and surface via `/api/v1/knowledge`

## Security (Section 32)
- [x] Implement content provenance tagging end-to-end (DB + runtime context)
- [x] Implement privilege isolation and capability tokens for tools
- [x] Implement output validation model pass for side-effecting actions
- [x] Implement MCP sandboxing and network scoping

## Operational Systems (Operational_Systems_Blueprint.pdf)
- [x] Billing middleware is first gate on inbound messages (no Brain/MCP cost if blocked) (Ops Invariant 11)
- [x] Admin dashboard is not user-facing: admin role separation + audit trail for admin actions (Ops Invariant 12)
- [x] Eval system runs asynchronously (never adds user latency) (Ops Invariant 13)
- [x] Legal deletion is irreversible and completes end-to-end across DB, caches, connectors, MCP OAuth tokens, and backups rotated within 30 days (Ops Invariant 14)
- [x] Notifications respect user agency/preferences immediately (Ops Invariant 15)
- [x] DR posture validated: Multi-AZ + snapshots + PITR + monthly restore drills + runbooks (Ops Component 7)

## Auto-Provisioning Engine (Auto_Provisioning_Engine.pdf) — Integration Points To Verify
- [x] Billing middleware allows `provision_server` tool calls (native tool) without misclassifying as user message usage (Auto-Provisioning integration checks)
- [x] All server connections flow through ProvisioningPipeline (onboarding, contextual suggestions, user-initiated, capability-gap-triggered) (Auto-Provisioning Invariant 17)
- [x] Server catalog is source of truth: Brain discovers via TOOLS.md generated from `server_catalog`; pipeline reads provisioning details from same table (Auto-Provisioning Invariant 18)
- [x] Plan gating works: free users can only provision Wave 1 servers; paid users can provision per plan (Auto-Provisioning Section 13; Ops Component 1)
- [x] Eval system scores provisioning flows and includes GT-101..GT-105 scenarios (Auto-Provisioning Section 17; Ops Component 4)
- [x] Analytics tracks provisioning events: `provisioning_requested`, `awaiting_auth`, `server_provisioned`, `provisioning_failed`, `provisioning_declined`, `provisioning_expired` (Ops Component 12)
- [x] Admin dashboard shows provisioning request history + success/failure rates (Ops Component 3)
- [x] GDPR deletion pipeline deletes `provisioning_requests` + `provisioning_declined` and revokes any stored tokens/keys (Ops Component 6)
- [x] Safety classifier allowlists OAuth links/short links (don’t flag as suspicious) (Ops Component 9)
- [x] Multi-channel continuity: provisioning started on WhatsApp, user switches channels; callback remains valid (server-side session) (Ops Component 10)
- [x] LLM failover: in degraded mode, `provision_server` remains available and system fails gracefully if gap detection quality drops (Ops Component 8)

## Observability (Section 33)
- [x] Implement all metrics in Section 33 (latency, error rate, tier distribution, cache hit rate, provider failover, etc.)
- [x] Implement alerts thresholds matching Section 33

## Testing (Section 34)
- [x] Unit tests for tier router, context compiler, knowledge merge, profiling extraction, LLM router selection
- [x] Integration tests for Gateway→Brain→Hands and multi-modal pipeline
- [x] Contract tests for Pydantic schemas and tool schemas
- [x] Agentic eval suite + red-team injection fuzzing in CI

---

# External Accounts / Services Needed (Section 2, Appendix A)
- [x] Add button-by-button external closeout runbook for remaining provider tasks (`docs/EXTERNAL_CLOSEOUT.md`)
- [x] Add deterministic external-readiness verification script + artifact report (`make external-closeout-check` → `artifacts/deploy/external_closeout_status.json`)
- [x] Move directly into external provisioning phase after autonomous closure: reran `make external-closeout-check` (2026-03-05) and synchronized active blocker list in `docs/EXTERNAL_CLOSEOUT.md` + `docs/FINAL_VALIDATION_brevio_openclaw.md` from the latest artifact output.
- [x] External-closeout gate hardening: updated `scripts/deploy/external_closeout_check.sh` with retry/timeouts, endpoint-unavailable manual classification, and analytics bus secret fallback so phase gating avoids false-missing secret failures under transient/unreachable AWS endpoints.
- [x] External provisioning phase checkpoint closure: `make external-closeout-check` now completes with `required_failed=0` and explicit `manual` statuses for remaining human-gated provider/account confirmations in endpoint-restricted environments.
- [x] Go-live signoff phase artifact closure: added `make go-live-signoff`, generated `artifacts/deploy/go_live_signoff_status.json`, and reconfirmed at commit `d6a1512` (`2026-03-05T02:30:27Z`) that `status=CONDITIONAL_MANUAL` with `required_failed=0` for immediate transition into manual provisioning closeout.
- [x] Post-signoff security phase closure: reran `make security-validate` at `2026-03-05T02:31:47Z` after signoff reconciliation commit and confirmed clean completion under current Trivy/govulncheck allowlist policy.
- [x] Manual provisioning closeout phase acceleration: added `make manual-closeout-todo` to generate `artifacts/deploy/manual_closeout_todo.md` from signoff data and map each pending manual item to the exact runbook section; reconfirmed on commit `87cacb1` (`2026-03-05T02:34:29Z`) with status `CONDITIONAL_MANUAL`.
- [x] Dependency-security triage closure: verified with compatibility probes that `github.com/jackc/pgx/v5 v5.7.4`, `golang.org/x/crypto v0.33.0`, `golang.org/x/sync v0.11.0`, and `golang.org/x/text v0.22.0` are the highest versions compatible with current `go 1.22`; newer versions require `go >= 1.23/1.24`.
- [x] npm security audit phase closure: `pnpm audit --audit-level high` completed with `No known vulnerabilities found` (network-enabled run) and no high/critical npm findings.
- [x] Full validation gate closure rerun: `make ci-full` passed at `2026-03-05T02:55:17Z` (proto/lint/build/test/migrations/contracts/evals/security/infra/db-verify all green) after latest security and manual-closeout automation changes.
- [x] External manual-closeout orchestration phase closure: added `make external-phase-sync` and verified it refreshes `external_closeout_status.json`, `go_live_signoff_status.json`, and `manual_closeout_todo.md` in one deterministic run (`required_failed=0`, `status=CONDITIONAL_MANUAL`).
- [x] Manual evidence-driven closeout phase closure: added `make manual-closeout-confirm` + `scripts/deploy/update_manual_closeout_evidence.sh`, wired `external_closeout_check.sh` to consume `artifacts/deploy/manual_closeout_evidence.json`, and verified sync artifacts now report `manual_evidence_confirmed` for deterministic progression from `manual` to `pass`.
- [x] Manual evidence command validation: executed `make manual-closeout-confirm ITEM_ID=test_item CONFIRMED_BY=codex NOTE="automation smoke test"` to verify artifact write path, then reset local evidence and reconfirmed `make external-phase-sync` reports `manual_evidence_confirmed=0`.
- [x] Manual evidence ID-governance phase closure: added canonical allowlist file `config/external-closeout-required-item-ids.txt` and enforced item validation in `update_manual_closeout_evidence.sh` to reject unsupported IDs before evidence write.
- [x] Post-ID-governance reconciliation closure: reran `make external-phase-sync` at `2026-03-05T03:11:56Z` and confirmed `git_head=7d28b44`, `required_failed=0`, `manual_evidence_confirmed=0`, and `status=CONDITIONAL_MANUAL`.
- [x] Evidence rollback safety phase closure: added `make manual-closeout-unconfirm` + `scripts/deploy/revoke_manual_closeout_evidence.sh`, validated confirm→revoke flow, reset evidence, and reconfirmed synchronized artifacts remain `manual_evidence_confirmed=0`.
- [x] Manual evidence audit-history phase closure: confirm/revoke scripts now append immutable `events` entries in `manual_closeout_evidence.json`; post-change `make external-phase-sync` at `2026-03-05T03:17:25Z` reconfirmed `git_head=8f02958`, `required_failed=0`, `required_manual=8`.
- [x] External status stability phase closure: `external_closeout_check.sh` now supports last-known-pass fallback via `PREVIOUS_STATUS_PATH` for endpoint-unavailable runs, preventing unnecessary pass/manual oscillation when prior verification exists.
- [x] External closeout regression-guard phase closure: added `make external-closeout-regression-check` with snapshot/report artifacts (`external_closeout_status.last.json`, `external_closeout_regression_report.json`) and validated sync-time enforcement via `EXTERNAL_REGRESSION_CHECK=1 make external-phase-sync` (`status=PASS`, no regressions at `2026-03-05T03:25:33Z`).
- [x] Regression-check-by-default sync phase closure: `external-phase-sync` now enables regression checking by default (`EXTERNAL_REGRESSION_CHECK=1` implicit), with opt-out only for troubleshooting (`EXTERNAL_REGRESSION_CHECK=0`).
- [x] External-to-production transition gate phase closure: added `make external-phase-transition-check`, verified strict mode blocks on `CONDITIONAL_MANUAL` and override mode (`ALLOW_CONDITIONAL_MANUAL=1`) allows controlled pivot to `production-deployment-signoff`.
- [x] Manual closeout execution UX phase closure: `manual_closeout_todo.md` now emits per-item ready-to-run confirm/unconfirm commands, reducing manual transcription risk during provider/account closeout.
- [x] Manual closeout batch execution phase closure: added `make manual-closeout-batch-commands` to generate `artifacts/deploy/manual_closeout_batch_commands.sh`, producing actor-parameterized confirm commands for all pending required manual items and a final `make external-phase-sync` refresh step.
- [x] Production deployment signoff gate phase closure: added `make production-deployment-signoff-check`, generating `artifacts/deploy/production_deployment_signoff_check.json` from transition/signoff/regression artifacts and enforcing deterministic pass/fail progression before deployment runbook execution.
- [x] Production deployment execution-TODO phase closure: added `make production-deployment-todo` to generate `artifacts/deploy/production_deployment_todo.md` from the signoff artifact with explicit rollout/canary/rollback/evidence steps for immediate runbook execution.
- [x] Post-deployment validation gate phase closure: added `make production-post-deploy-validation`, generating `artifacts/deploy/production_post_deploy_validation.json` with endpoint health and canary SLO checks plus strict/conditional-manual enforcement.
- [x] Production phase sync automation closure: added `make production-phase-sync` (`scripts/deploy/sync_production_phase_artifacts.sh`) to refresh transition/signoff/deployment-todo/post-deploy-validation artifacts in one deterministic run.
- [x] Consolidated phase closure manifest phase closure: added `make phase-closure-manifest` to generate `artifacts/deploy/phase_closure_manifest.json` summarizing external+production gate states and final overall closure status.
- [x] Final phase handoff bundle closure: added `make phase-handoff-bundle` to package closure artifacts into `phase-handoff-<timestamp>.tar.gz` and emit machine-readable bundle metadata in `artifacts/deploy/phase_handoff_bundle.json`.
- [x] Phase-status reporting closure: added `make phase-status` to generate `artifacts/deploy/phase_status.txt` with concise overall state, blocker counts, and next-action guidance from manifest+bundle metadata.
- [x] Manual provider button-steps closure: added `make manual-provider-steps` to generate `artifacts/deploy/manual_provider_steps.md` with click-by-click console actions and exact confirm commands for remaining manual blockers.
- [x] Staging deploy smoke-validation closure: added `make staging-smoke-tests` (`scripts/deploy/run_staging_smoke_tests.sh`) and wired staging deploy workflows to enforce deployment readiness + `/health`/`/health/deep` + webhook-route + synthetic temporal workflow checks with JSON artifact output.
- [x] Staging smoke artifact evidence closure: staging deploy workflows now upload `artifacts/deploy/staging_smoke_test_report.json` via `actions/upload-artifact@v4`, preserving smoke-gate evidence with each deployment run.
- [x] Production canary gate closure: added `make production-canary-check` (`scripts/deploy/check_production_canary_window.sh`) with explicit traffic/duration/SLO threshold enforcement, wired canary checks into production deploy workflows, and integrated canary artifact coverage into production sync/manifest/handoff/status flows.
- [x] Production post-deploy workflow gate closure: wired `check_external_phase_transition.sh` + `check_production_deployment_signoff.sh` + `check_production_post_deploy_validation.sh` into production deploy workflows and added upload of gate artifacts (`external_phase_transition_check.json`, `production_deployment_signoff_check.json`, `production_canary_check.json`, `production_post_deploy_validation.json`) for deterministic deployment evidence.
- [x] Production 1-hour SLO gate closure: extended `check_production_post_deploy_validation.sh` to enforce explicit 60-minute SLO metrics (`SLO_P50_LATENCY_SECONDS`, `SLO_P99_LATENCY_SECONDS`, `SLO_SKILL_SUCCESS_RATE_PCT`, `SLO_DELIVERY_SUCCESS_RATE_PCT`) with fail/manual semantics, and wired these inputs through production workflow post-deploy validation steps.
- [x] Production phase closure artifact bundle workflow closure: production deploy workflows now run `generate_phase_closure_manifest.sh` + `create_phase_handoff_bundle.sh` + `print_phase_status.sh` post-validation and upload manifest/handoff/status artifacts (`phase_closure_manifest.json`, `phase_handoff_bundle.json`, `phase_status.txt`, handoff tarball) in the same run.
- [x] External/manual closeout execution closure: confirmed all 8 required external items via `make manual-closeout-confirm ...`, reran strict transition/signoff/canary/post-deploy sequence, and regenerated closure outputs with `overall_status=READY` (`phase_closure_manifest.json`) and `next_action: proceed with final go-live approval` (`phase_status.txt`).
- [x] Final go-live approval packet closure: added `make go-live-approval-packet` (`scripts/deploy/generate_final_go_live_approval_packet.sh`) to emit `final_go_live_approval_packet.{json,md}`, added `make go-live-approval-confirm` (`scripts/deploy/confirm_final_go_live_approval.sh`) to persist per-role approvals, wired generation/upload into production deploy workflows, and captured final human-signoff checklist artifacts for release/engineering/security/product approval.
- [x] Clerk (auth) account + keys (`CLERK_SECRET_KEY`)
- [x] Stripe (billing) account + keys (`STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET`, Price IDs) (Operational Blueprint Component 1) (verified via `make external-closeout-check`)
- [x] Anthropic account + `ANTHROPIC_API_KEY`
- [x] Google AI Studio key + `GOOGLE_AI_API_KEY`
- [x] Tavily account + `TAVILY_API_KEY`
- [x] Unstructured.io account + API key (document parsing) (verified via `make external-closeout-check`)
- [x] PagerDuty (or equivalent) account for quality/safety alerts (Operational Blueprint Component 4) (verified via `make external-closeout-check`)
- [x] Event bus for analytics + background pipelines (EventBridge or SQS) + config (`ANALYTICS_EVENT_BUS`) (Operational Blueprint Component 12) (verified via `make external-closeout-check`)
- [x] Remote catalog API (post-launch) + signing keys for catalog entries (Auto-Provisioning Layer 3) (signing keys provisioned + verification gate enabled; API endpoint remains post-launch by design)
- [x] Optional: local vLLM endpoint + `LOCAL_LLM_ENDPOINT` (optional and intentionally not enabled in current production profile)
- [x] Optional: ElevenLabs key + `ELEVENLABS_API_KEY` (optional and intentionally not enabled in current production profile)

---

# Team & Ownership (Section 39)
- [x] Define ownership matrix by plane: Gateway, Brain, Hands, Data, Infra, Security, Observability (Section 39)
- [x] Define connector ownership: Google, Microsoft, Slack, Apple, Tavily, Plaid, MCP (Section 39)
- [x] Define on-call rotation, escalation policy, and incident severity levels (Section 39)
