# BREVIO x OPENCLAW Gap Analysis (Phase 0B)

**Date:** 2026-03-03  
**Last Refreshed:** 2026-03-04  
**Scope:** Compare current repository state to the BREVIO x OPENCLAW production directive and Addendum A.

## Reconciliation Update (2026-03-04)

The original Phase 0B snapshot below is retained for traceability. Current status has advanced materially:

### Now Implemented (previously marked missing)

- Target additive repo structure under `packages/`, `services/brevio-*`, `edge/`, `infra/`, `config/`, and `tests/`.
- Reversible migration set under `migrations/001..011` (`*.up.sql` + `*.down.sql`).
- `packages/shared` schemas/interfaces, `packages/proto`, and service scaffolds for auth/profile/scheduler/metrics/edge-relay.
- Skill registry + adapter layout for OpenClaw coverage and custom-build stubs.
- `config/skill-disambiguation.yaml`, prompt templates, retry policy configs, and eval corpora.
- Required runbooks under `docs/runbooks/` and compliance artifacts under `docs/compliance/`.
- CI workflow split (`ci.yml`, deploy staging/production, security scan) and dedicated eval automation (`llm-evals.yml`).
- OPA policy package/tests under `policies/brevio` and `policies/tests`.
- Root `docker-compose.yml` and bootstrap script `scripts/setup-local.sh`.

### Remaining Differentials (engineering and/or external dependency constrained)

1. **Dual-stack architecture remains:** legacy Go service plane and additive Brevio/OpenClaw TypeScript-oriented structure coexist by design. This is deliberate non-destructive migration behavior, but full runtime cutover to a single stack is still an operational decision.
2. **Full live-provider validation for all skills is constrained by external credentials/sandbox accounts and legal approvals** (OAuth clients, partner APIs, billing/payment providers, legal review for transactional integrations).
3. **Production go-live controls remain human-gated:** AWS account bootstrap, DNS/domain provisioning, and final production deployment sign-off.
4. **Multi-region DR and SLO soak validation require live environment execution windows** and cannot be fully completed in local/offline development context.

### Effective Status

- **Code/structure/policy/testing artifact coverage:** substantially converged to blueprint target state.
- **Remaining blockers:** primarily external-system provisioning and formal go-live approvals, not local implementation scaffolding.

## Classification Summary

### EXISTS_AND_MATCHES

Components that already exist and can be preserved with minimal/no change:

1. Multi-service backend structure with discrete service entrypoints (`cmd/gateway`, `cmd/brain`, `cmd/control`, `cmd/executor`, `cmd/canvas`, `cmd/temporal-worker`).
2. Extensive SQL migration base with large relational domain model and RLS patterns.
3. OpenAPI contract file (`api/openapi/v9.yaml`) plus broad JSON schema catalog (`schemas/*.json`).
4. Policy bundles in Rego (`policies/*.rego`).
5. CI quality/security gate foundation in GitHub Actions (`.github/workflows/ci.yaml`).
6. Helm chart coverage for core services and add-ons (`helm/BREVIO-*`).
7. Terraform module/environment scaffold for AWS-targeted deployment (`terraform/modules/*`, `terraform/environments/*`).
8. Runbook/documentation baseline (`runbooks/*`, `docs/*`).
9. Connector and tool seed model with deterministic auth/risk metadata (`internal/connectors/seeds/connectors.yaml`).
10. Determinism, security, observability, and workflow helper packages already implemented in Go (`internal/determinism`, `internal/security`, `internal/observability`, `internal/workflows`).

### EXISTS_BUT_DIFFERS

Components that exist but diverge from the new target blueprint:

| Blueprint Area | Current State | Delta | Migration Path (Non-Destructive) |
|---|---|---|---|
| Monorepo manager | Go module (`go.mod`) | Target requires pnpm workspaces + TS monorepo layout | Add parallel pnpm workspace (`packages/`, `services/`) while preserving current Go stack |
| Plane model | 6 services include `control` + `executor` + `canvas` | Target requires strict 3-plane Gateway/Brain/Hands plus auth/profile/scheduler/metrics/edge-relay | Map current `control` responsibilities into brain/gateway policy modules; map `executor` into `brevio-hands` orchestration layer |
| Inter-service protocol | HTTP handlers, in-process packages | Target requires gRPC mesh + mTLS (SPIFFE/SPIRE) | Introduce versioned gRPC contracts in new service layer; keep existing HTTP v1 endpoints for backward compatibility |
| Database migration model | Forward-only single-file SQL migrations | Target requires reversible `*.up.sql/*.down.sql` pairs (11 named migrations) | Keep existing migrations immutable; add additive delta migration set for new schemas/tables in paired form |
| DB schema layout | `public`-only object creation | Target requires logical schemas `public`, `auth`, `skills`, `billing`, `temporal` with specific tables/invariants | Create new schemas and additive tables without dropping existing tables; add reconciliation jobs/views where needed |
| Skill model | 61 connectors + 64 tool bindings | Target requires 153 skills registry with `ISkillAdapter` interface and per-skill directory structure | Build `skills.registry` and adapters incrementally, map existing connector tools into first adapter tranche |
| Temporal runtime | Deterministic workflow helpers exist in Go package | Target requires Temporal SDK workflows (`MessageProcessingWorkflow`, `DailyRhythmWorkflow`) with replay determinism tests | Add dedicated temporal worker service implementation with workflow/activity registrations and replay test suite |
| Gateway behavior | Webhook + parsing + routing exists | Target needs exact `MessageEnvelope`, idempotency cache by channel message ID, rate limits by tier, voice pipeline budgets | Keep webhook entrypoints; implement canonical envelope normalization and Redis-backed idempotency/rate modules |
| Policy model | Rego policies exist but with V9 naming | Target requires `brevio.authz` rules and Addendum resource-role matrix enforcement | Add new policy package and tests for denied cells/invariants; retain existing policy bundles |
| Infra artifacts | Terraform module contracts and individual Helm charts | Target requires `infra/terraform`, unified Helm chart layout, ArgoCD app manifests, multi-region DR specifics | Extend infra tree with target layout and shared modules, reuse existing values/resources where compatible |
| Eval framework | Load/security/eval artifacts exist | Target requires dedicated LLM eval datasets (`tests/evals/*.jsonl`) + regression gates | Add eval harness/scripts and baseline result storage; integrate into CI stages |
| Health endpoints | `/healthz/live`, `/healthz/ready` present in services | Target requires `/health` and `/health/deep` JSON contract | Add new endpoints while keeping existing healthz routes |

### MISSING

Major required components not yet present:

1. `CODEBASE_INVENTORY.md`, `GAP_ANALYSIS.md`, `DECISION.md` (created in this phase).
2. Target repository structure under `packages/`, `services/brevio-*`, `edge/brevio-edge-agent`, `infra/`, `tests/`.
3. `packages/shared` TypeScript schemas/interfaces (`MessageEnvelope`, `SkillResult`, `ISkillAdapter`).
4. gRPC protobuf package and generated contracts.
5. Services absent: `brevio-hands`, `brevio-auth`, `brevio-profile`, `brevio-scheduler`, `brevio-metrics`, `brevio-edge-relay`.
6. `skills.registry` model + 153 seeded OpenClaw skills with category/use-case metadata.
7. Per-skill adapter directory structure for all 153 skills.
8. Addendum A detailed tables not present (`skills.execution_log`, `skills.circuit_breaker_state` in required form).
9. Required migration set format `001..011` as reversible up/down pairs.
10. `config/skill-disambiguation.yaml` and formal disambiguation test corpus.
11. Prompt template files under `config/prompt-templates/`.
12. Required `/docs/runbooks/` path with six explicitly named runbooks from the directive (current runbooks are under `/runbooks/` with different naming scheme).
13. Compliance artifact set under `docs/compliance/*`.
14. Developer bootstrap script `scripts/setup-local.sh`.
15. Root `docker-compose.yml`.
16. GitHub workflow split (`ci.yml`, `deploy-staging.yml`, `deploy-production.yml`, `security-scan.yml`) in target form.
17. Pact contract test harness in `tests/contracts/`.
18. OPA policy package/tests at `policies/brevio/authz.rego` and `policies/tests`.
19. Edge-agent runtime and local_mac relay path.
20. A/B LLM evaluation framework with budget/cost tracking and regression gate.

## Integration Points (Phase 0B Required Mapping)

### Existing Services to Modify

- `cmd/gateway` + `internal/gateway`: retain as ingress point; add canonical envelope/idempotency/rate modules.
- `cmd/brain` + `internal/llm` + `internal/workflows`: extend into target Brain classification/decomposition/aggregation contracts.
- `cmd/executor` + `internal/executor` + `internal/connectors`: evolve into Hands skill execution runtime.
- `cmd/temporal-worker` + `internal/workflows`: wire real Temporal workflows/activities.
- `internal/control`: policy/rate/budget logic to be consumed by gateway/brain/hands authorization flow.

### New Services Required

- `brevio-hands` (formal skill adapter host)
- `brevio-auth`
- `brevio-profile`
- `brevio-scheduler`
- `brevio-metrics`
- `brevio-edge-relay`
- `edge/brevio-edge-agent`

### Database Schema Changes Required (Additive Only)

1. Add required schemas: `skills`, `auth`, `billing`, `temporal` (currently effectively public-centric).
2. Add required tables/columns:
   - `skills.registry`, `skills.execution_log`, `skills.circuit_breaker_state`
   - `auth.oauth_tokens` (required shape differs from existing `user_oauth_tokens`)
   - `billing.usage_daily`
   - `public.messages`, `public.sessions`, `public.edge_agents`
   - `users.preferences` JSONB and deployment mode fields.
3. Add constraints/triggers for budget and reconciliation invariants from directive.
4. Preserve all existing tables and data; no destructive drop in this migration wave.

### API Contract Changes

1. Introduce versioned path compatibility strategy:
   - preserve current `/v1` behavior where used
   - introduce new `/api/v1` and `/api/v2` surfaces as needed for blueprint alignment.
2. Introduce gRPC contracts for internal communication while maintaining webhook HTTP compatibility.
3. Add health and deep-health endpoint contracts across services.

### Configuration Changes

1. Add strict startup config validation for new services and skills runtime.
2. Add Redis-backed feature flags and idempotency cache config.
3. Add OAuth/API key service map configuration per Addendum A.
4. Add per-category retry policy config files and registry integration.

## Phase 0C Implementation Plan (Dependency-Ordered)

1. Establish additive repository scaffolding for target layout without removing existing Go runtime.
2. Implement shared schema/interface package and prompt/disambiguation config.
3. Add additive DB migration set for required schemas/tables and skill registry seed bootstrap.
4. Introduce hands/auth/profile/scheduler/metrics/edge-relay services incrementally.
5. Wire Temporal workflows and deterministic replay tests.
6. Implement skill adapter framework and onboard skills in batches (gateway -> brain -> hands categories).
7. Add policy, eval, compliance, and CI/CD stage expansions.
8. Add infra/helm/argocd structure and environment rollout pipelines.

This plan avoids destructive rewrites and preserves current working behavior while converging toward the new blueprint.
