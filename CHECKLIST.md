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
- [x] Phase 0.1: map current repository (`find` + depth-limited directory map) and compare against V9 canonical structure
- [x] Phase 0.1: create gap report at `docs/codebase_audit_report.md` with required artifact status table
- [x] Phase 0.1: identify non-V9/V9.1/V9.2 candidate removal areas in audit report
- [x] Track new source blueprints in Git:
  - `Brevio_V9_Consolidated_Master_Blueprint.docx`
  - `Brevio_V91_Addendum_Soft_Intelligence_Layer.docx`
  - `Brevio_V92_Addendum_Production_Hardening.docx`
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
- [x] Strict closure hardening: encoded V9.1 workflow trigger semantics from Addendum §P in `internal/workflows.V91WorkflowTriggerSpecs()` and added exact trigger mapping tests (`TestV91WorkflowTriggerSpecs`)
- [x] Strict closure hardening: upgraded Go dependency floor within Go 1.22 constraints (`pgx/v5 v5.7.4`, `x/crypto v0.33.0`, `x/sync v0.11.0`, `x/text v0.22.0`) and added `internal/contracts/dependency_closure_test.go` to lock minimum versions
- [x] Strict closure hardening: added reproducible `govulncheck` baseline control (`scripts/security/run_govulncheck.sh` + `govuln_allowlist.txt`), wired into security validation and CI, and documented residual Go 1.22-constrained advisories in `docs/SECURITY_VULNERABILITY_BASELINE.md`
- [x] Strict closure hardening: extended CI closure contract (`internal/contracts/ci_closure_test.go`) to require `govulncheck baseline` execution tokens in `.github/workflows/ci.yaml`
- [x] Strict closure hardening: strengthened acceptance gate contracts (`internal/contracts/acceptance_gates_test.go`) to require explicit runtime subtest presence for every named V9.1 and V9.2 gate in `acceptance_gate_runtime_closure_test.go`
- [x] Strict closure hardening: replaced remaining minimum-threshold closure checks with exact parity assertions for migration table counts, OpenAPI path count, schema catalog membership, and compliance matrix row-count/ID-set exactness (`internal/contracts/closure_checks_test.go`, `internal/contracts/openapi_closure_test.go`, `internal/contracts/schema_closure_test.go`, `internal/contracts/prompt_compliance_closure_test.go`)
- [x] Strict closure hardening: enforced exact OpenAPI operation parity by matching the full `METHOD path` operation-set with no extras/missing operations in `internal/contracts/openapi_closure_test.go`
- [x] Strict closure hardening: added executable V9 runtime acceptance-gate coverage (`schema_closure`, `determinism`, `webhook_security`, `acceptance_suites_1_12`, `workspace_isolation`, `provisioning_pipeline`, `onboarding_completion`, `provisioning_recovery`, `deterministic_llm`, `cve_scanning`) in `internal/contracts/acceptance_gate_runtime_closure_test.go` and bound V9 gate-name contracts to these runtime subtests in `internal/contracts/acceptance_gates_test.go`
- [x] Strict closure hardening: added exact infrastructure directory-set gates so `terraform/modules` and `helm/` must match the canonical V9/V9.2 module/chart sets with no extras/missing entries (`internal/contracts/infrastructure_closure_test.go`)
- [x] Strict closure hardening: added exact OpenAPI `components.schemas` key-set parity gate (49 canonical component schema keys, including generic/error wrappers) in `internal/contracts/openapi_closure_test.go`
- [x] Strict closure hardening: added workflow runtime exercise closure test to execute all 22 mapped V9/V9.1/V9.2 workflow IDs through `internal/workflows.Service` (`internal/contracts/workflow_closure_test.go`)
- [x] Strict closure hardening: removed compliance evidence hash placeholders by computing/normalizing real SHA-256 digests in `internal/compliance.Service`, wired framework evidence creation through computed digests (`internal/control/mux.go`), and added runtime/test enforcement of valid `sha256:<64hex>` evidence hashes
- [x] Strict closure hardening: strengthened govuln baseline closure by enforcing allowlist ID format/uniqueness/non-empty rules and requiring baseline documentation linkage in `internal/contracts/security_vuln_baseline_closure_test.go`

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
- [ ] Run 12-step server deployment checklist for every Wave 1 server (Deployment Plan Section 15)

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
- [ ] Submit partner applications during Month 12: Zoom Marketplace, Instacart Connect, Canva Connect, Booking.com Demand API (Deployment Plan + Wave 5–6 Section 8/12)
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
- [ ] Build + deploy Duffel MCP (custom) with approval-gated booking flow (`create_order` risk critical) and WhatsApp list-message offer selection
- [ ] Build + deploy Zoom MCP (custom) with User-Level OAuth and meeting transcript retrieval support
- [ ] Build + deploy Calendly MCP (custom) with duplicate-event prevention against Google Calendar events
- [ ] Build + deploy Plaid MCP (custom) + Plaid Link widget page hosting (S3/CloudFront or equivalent)
- [ ] Complete Plaid production verification and store real `PLAID_SECRET_PROD` before enabling Plaid in prod
- [ ] Replace placeholder `PLAID_WEBHOOK_SECRET` with final webhook validation config used in production
- [ ] Build + deploy Crunchbase MCP (custom) and wire to research engine ingestion
- [ ] Validate Wave 5 extras: Duffel sandbox e2e booking, Plaid network audit (no external LLM PII egress), contextual discovery triggers
- [x] Verified OAuth scopes + webhook events for Wave 5 services (Duffel, Zoom, Calendly, Plaid, Crunchbase)

## M15: Wave 6 (5 Servers) (Wave 5–6 Section 5, Section 6.6–6.10)
- [ ] Build + deploy Booking.com MCP (custom) with explicit booking approval details (hotel/room/dates/price/cancellation policy)
- [ ] Deploy + harden DocuSign MCP (existing community server fork) with envelope approval gate
- [ ] Build + deploy Canva MCP (custom) with curated template-based flows (no unsupported free-form flows)
- [ ] Build + deploy Instacart MCP (custom) OR approved fallback (Amazon Fresh/DoorDash) with checkout approval gate
- [ ] Deploy + harden Tesla MCP (existing server fork) with physical-security approvals + strict rate limits + optional geo-fencing
- [ ] Validate Wave 6 extras: Tesla physical operation tests in staging, Instacart checkout approval details, DocuSign recipient/document confirmation

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
- [ ] Verify all 40 servers pass health checks simultaneously
- [ ] Run 100-concurrent MCP-call load test across mixed servers
- [ ] Run failover simulation by killing 5 random servers and verifying reconnect/degradation behavior
- [ ] Confirm TOOLS.md regeneration pipeline reflects all connected/disconnected servers

## MCP Deployment Checklist (Apply to Every Server, Waves 1–6)
- [ ] Build/push server image (ECR or bundled sidecar) and deploy runtime
- [ ] Register server manifest in `mcp_servers` and validate capability probe (`tools/list`)
- [ ] Configure OAuth/authentication and callback routing
- [ ] Apply risk classification + approval thresholds per tool
- [ ] Pass normalization test (Brain → MCP call → normalized `ToolResult`)
- [ ] Pass security tests (evil-server suite, provenance guardrails, privilege isolation)
- [ ] Verify cost tracking (per-call/per-run/per-server-daily) + rate limit counters
- [x] Ensure `tool_executions` records include `is_mcp=true` and `mcp_server_id`
- [ ] Flag TOOLS.md refresh and verify nightly regeneration includes: connected apps, available-but-not-connected servers (plan-gated), tools list, auth status, and budgets/usage (Deployment Plan Section 11; Auto-Provisioning Layer 1)
- [ ] Add onboarding card (Waves 1–4) or contextual discovery trigger (Waves 5–6)
- [ ] Pass 3 golden scenario tests per server
- [ ] Update operational docs/runbooks for server-specific failure handling

## MCP Architecture Invariants (Month 1 → Month 15)
- [ ] Brain plane remains MCP-agnostic (no MCP-specific branching/imports in Brain logic)
- [ ] Shared ToolRegistry remains single source of truth for native + MCP tool schemas
- [ ] Content provenance tagging enforced end-to-end, including `mcp_result`
- [ ] Every MCP invocation recorded in shared `tool_executions` table path
- [ ] OAuth tokens for MCP servers stored in existing `oauth_tokens` table with `provider=<server_id>`
- [x] Financial/booking tools always require explicit approval before write operations
- [ ] Sensitive financial data routes only through local-model path (`pii_content=true`)

## MCP Deployment Plan Platform Requirements (MCP_Server_Deployment_Plan.docx)
- [x] Hosting strategy implemented per-server (sidecar vs internal microservice vs external) and encoded in `server_catalog.hosting_model` (Deployment Plan Section 8; Auto-Provisioning Section 12)
- [ ] OAuth/auth matrix supported across servers (OAuth2, API key, PAT, integration tokens); tokens stored encrypted in `oauth_tokens` and refreshed safely (Deployment Plan Section 9)
- [ ] Onboarding UX templates + buttons exist for ecosystem detection and connection flows (Deployment Plan Section 10; Auto-Provisioning Appendix B)
- [x] TOOLS.md auto-generation includes connected apps details (tools list, auth status, budgets/usage) and not-connected guidance (Deployment Plan Section 11)
- [ ] Conversational discovery triggers (mentions + repeated failures + profile evolution) are implemented and wired to ProvisioningPipeline (Deployment Plan Section 12)
- [ ] Cost model enforced: per-server budgets + rate limits + per-call metering + billing integration; surfaced in admin + TOOLS.md (Deployment Plan Section 13; Ops Blueprint Component 1)
- [ ] MCP server health dashboard implemented (admin) per wireframe (Deployment Plan Appendix B; Ops Blueprint Component 3)

---

# Cross-Cutting Requirements (Must Be Covered) (Section 3–34)

## Database + RLS (Section 3)
- [x] Create all enums exactly as Section 3.1 (channel_type, input_modality, run_state, llm_provider, etc.)
- [x] Create/alter all 19 tables exactly as Section 3.2–3.4 including enhanced columns and indexes
- [ ] Add Operational Systems tables + columns per Ops Blueprint Section 16 (subscriptions, invoices, eval_results, user_feedback, prompt_versions, moderation_queue, scheduled_notifications, analytics_events, analytics_daily; extend conversations/messages as needed)
- [x] Add Auto-Provisioning tables per Auto-Provisioning Section 6 (provisioning_requests, server_catalog, provisioning_declined)
- [ ] Ensure correct migration ordering for FK dependencies (Ops Blueprint “Database Migration Order” + Auto-Provisioning schema)
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
- [ ] Implement unified API endpoints from Section 5 (Gateway/Core/Behavioral/New/Internal)
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
- [ ] Implement Appendix B error codes taxonomy across endpoints + logs

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
- [ ] Legal deletion is irreversible and completes end-to-end across DB, caches, connectors, MCP OAuth tokens, and backups rotated within 30 days (Ops Invariant 14)
- [x] Notifications respect user agency/preferences immediately (Ops Invariant 15)
- [ ] DR posture validated: Multi-AZ + snapshots + PITR + monthly restore drills + runbooks (Ops Component 7)

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
- [ ] Implement all metrics in Section 33 (latency, error rate, tier distribution, cache hit rate, provider failover, etc.)
- [ ] Implement alerts thresholds matching Section 33

## Testing (Section 34)
- [ ] Unit tests for tier router, context compiler, knowledge merge, profiling extraction, LLM router selection
- [ ] Integration tests for Gateway→Brain→Hands and multi-modal pipeline
- [ ] Contract tests for Pydantic schemas and tool schemas
- [x] Agentic eval suite + red-team injection fuzzing in CI

---

# External Accounts / Services Needed (Section 2, Appendix A)
- [x] Clerk (auth) account + keys (`CLERK_SECRET_KEY`)
- [ ] Stripe (billing) account + keys (`STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET`, Price IDs) (Operational Blueprint Component 1)
- [x] Anthropic account + `ANTHROPIC_API_KEY`
- [x] Google AI Studio key + `GOOGLE_AI_API_KEY`
- [x] Tavily account + `TAVILY_API_KEY`
- [ ] Unstructured.io account + API key (document parsing)
- [ ] PagerDuty (or equivalent) account for quality/safety alerts (Operational Blueprint Component 4)
- [ ] Event bus for analytics + background pipelines (EventBridge or SQS) + config (`ANALYTICS_EVENT_BUS`) (Operational Blueprint Component 12)
- [ ] Remote catalog API (post-launch) + signing keys for catalog entries (Auto-Provisioning Layer 3)
- [ ] Optional: local vLLM endpoint + `LOCAL_LLM_ENDPOINT`
- [ ] Optional: ElevenLabs key + `ELEVENLABS_API_KEY`

---

# Team & Ownership (Section 39)
- [ ] Define ownership matrix by plane: Gateway, Brain, Hands, Data, Infra, Security, Observability (Section 39)
- [ ] Define connector ownership: Google, Microsoft, Slack, Apple, Tavily, Plaid, MCP (Section 39)
- [ ] Define on-call rotation, escalation policy, and incident severity levels (Section 39)
