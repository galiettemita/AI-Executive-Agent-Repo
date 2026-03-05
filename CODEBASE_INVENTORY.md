# BREVIO x OPENCLAW Codebase Inventory (Phase 0A)

**Date:** 2026-03-03  
**Last Refreshed:** 2026-03-04  
**Repository:** `/Users/galiettemita/Downloads/Executive AI Agent/backend`  
**Commit Baseline:** `codex/brevio-openclaw-phase0` branch created from current `main`

## 0) Refresh Snapshot (2026-03-04)

This inventory was originally captured at the start of Phase 0A and is retained for baseline traceability. The repository has since been extended substantially. Current high-signal state:

- Monorepo now includes both legacy Go runtime and target additive layout:
  - `packages/{shared,proto,sdk}`
  - `services/{brevio-gateway,brevio-brain,brevio-hands,brevio-auth,brevio-profile,brevio-temporal-worker,brevio-scheduler,brevio-metrics,brevio-edge-relay}`
  - `edge/brevio-edge-agent`
  - `infra/{terraform,helm,docker,argocd}`
  - `config/{skill-disambiguation.yaml,prompt-templates,retry-policies}`
  - `tests/{contracts,integration,e2e,load,chaos,security,evals}`
- Reversible migration set `migrations/001..011` exists in paired `*.up.sql`/`*.down.sql` format.
- Hands skill adapter tree contains 163 generated skill directories (153 OpenClaw + 10 custom-build) plus `_template`; scaffold integration placeholders are fully eliminated from skill integration tests.
- CI workflow split required by the directive is present:
  - `.github/workflows/ci.yml`
  - `.github/workflows/deploy-staging.yml`
  - `.github/workflows/deploy-production.yml`
  - `.github/workflows/security-scan.yml`
  - `.github/workflows/llm-evals.yml` (weekly + prompt/eval change triggers)
- Directive-specific docs now exist under target paths:
  - `docs/runbooks/*` (6 required runbooks)
  - `docs/compliance/*` (security, GDPR, SOC2 readiness set)

The remaining sections below document the original Phase 0A baseline state used to drive the migration plan.

## 1) Repository Structure and Organization Pattern

- Pattern: **single Go monorepo** (not pnpm workspace).
- Root contains service entrypoints in `cmd/`, domain logic in `internal/`, SQL migrations in `db/migrations/`, OpenAPI + JSON schemas in `api/` + `schemas/`, infra in `terraform/` + `helm/`, ops docs in `docs/` + `runbooks/`.
- File volume: **497 tracked files**.

Top-level directories:
- `.github/workflows`
- `admin/`
- `api/`
- `artifacts/`
- `cmd/`
- `db/`
- `docs/`
- `evals/`
- `helm/`
- `internal/`
- `policies/`
- `prompts/`
- `runbooks/`
- `schemas/`
- `scripts/`
- `spec/`
- `terraform/`

## 2) Package Manager and Workspace Configuration

- Current language/toolchain: **Go 1.22**.
- Dependency management: `go.mod` + `go.sum`.
- No current pnpm workspace artifacts found:
  - missing `package.json` (workspace root)
  - missing `pnpm-workspace.yaml`
  - missing `turbo.json`
  - missing `tsconfig.base.json`

## 3) Existing Services and Entry Points

Current executables:

| Service Binary | Entrypoint | Runtime Type | Default Port |
|---|---|---|---|
| gateway | `cmd/gateway/main.go` | HTTP | `:18080` |
| brain | `cmd/brain/main.go` | HTTP | `:18081` |
| control | `cmd/control/main.go` | HTTP | `:18082` |
| executor | `cmd/executor/main.go` | HTTP | `:18083` |
| temporal-worker | `cmd/temporal-worker/main.go` | HTTP health shell | `:18084` |
| canvas | `cmd/canvas/main.go` | HTTP/WebSocket support in package | `:18793` |

Related internal service packages:
- `internal/gateway`
- `internal/control`
- `internal/executor`
- `internal/workflows`
- `internal/llm`
- `internal/connectors`
- `internal/mcp`
- `internal/security`
- `internal/observability`
- additional v9.1/v9.2 packages under `internal/*` (40+ domains).

## 4) Database Schemas Already in Place

Migration files present:
- `db/migrations/001_BREVIO_v9_init.sql`
- `db/migrations/002_BREVIO_v91_soft_intelligence.sql`
- `db/migrations/003_BREVIO_v92_production_hardening.sql`
- `db/migrations/004_BREVIO_ops_operational_systems.sql`
- `db/migrations/005_BREVIO_mcp_execution_oauth_hardening.sql`
- `db/migrations/006_BREVIO_v93_addendum_specification_closure.sql`

Current migration model:
- **Forward-only** SQL migrations (no `.down.sql` pairs).
- `internal/database/migrations.go` rejects down migrations.
- SQL objects are currently in **public schema only** (no `skills`, `auth`, `billing`, `temporal` schema creation blocks).

Current schema scale (from migration parsing):
- 170 `CREATE TABLE` statements total.
- 82 `CREATE TYPE` statements total.
- Includes user/workspace/channel/control/executor/workflow/connectors/memory/rag/compliance/admin tables.

Notable existing tables relevant to new blueprint mapping:
- `users`, `ingress_turns`, `workflow_instances`, `workflow_steps`
- `connectors`, `connector_tools`, `user_oauth_tokens`
- `tool_executions`, `audit_log_entries`
- no `skills.registry` / `skills.execution_log` / `skills.circuit_breaker_state` tables yet.

## 5) Existing Tests and Coverage

- Test files: **137** (`*_test.go`).
- Major package test concentration:
  - `internal/contracts` (40 test files)
  - `internal/gateway` (8)
  - `internal/executor` (8)
  - `internal/database` (8)
  - `internal/control` (6)

Coverage run executed (`./scripts/dev/go_exec.sh test ./... -cover -count=1`):
- Many runtime packages are ~70-90% covered.
- Example package coverage:
  - `internal/executor`: 85.1%
  - `internal/gateway`: 76.2%
  - `internal/control`: 75.0%
  - `internal/llm`: 86.1%
- Global run currently **fails** due contract test guard expecting a strict blueprint file set (`brevio-openclaw-blueprint.docx` appears as extra).

## 6) CI/CD Configuration Already Present

- Workflow file: `.github/workflows/ci.yaml`.
- Current CI stages include:
  - `gofmt`, `go mod tidy`, `go vet`, `staticcheck`
  - `go test ./...`
  - migration checks + runtime migration verification
  - OpenAPI/schema closure tests
  - infra validation (`scripts/infra/validate.sh`)
  - security scans (`trivy`, `trufflehog`, `govulncheck`)
  - SBOM generation (`syft`)

Current deployment automation:
- `scripts/deploy/helm_rollout.sh`
- `scripts/deploy/external_closeout_check.sh`
- `Makefile` targets: `ci`, `ci-full`, `deploy-helm`, `security-validate`, `infra-validate`, `db-verify`.

## 7) Infrastructure Files

Terraform:
- Modules: `vpc`, `eks`, `rds`, `elasticache`, `sqs`, `s3`, `secrets`, `temporal`, `observability`, `opensearch`, `admin-frontend`, `feature-flags-cache`.
- Environments: `terraform/environments/staging/main.tf`, `terraform/environments/production/main.tf`.
- Current module files are largely **module contract stubs** (locals + outputs), not full resource implementations.

Helm charts:
- Core: `BREVIO-gateway`, `BREVIO-brain`, `BREVIO-control`, `BREVIO-executor`, `BREVIO-canvas`, `BREVIO-temporal-worker`
- Add-ons: `BREVIO-admin-api`, `BREVIO-admin-frontend`, `BREVIO-rag-worker`, `BREVIO-guardrails`, `BREVIO-health-checker`

Containers:
- Single root `Dockerfile` with `ARG SERVICE=...` multi-service build strategy.
- Distroless runtime image usage is already present.

Missing infra artifacts vs target blueprint:
- No `infra/` root with `terraform/modules` + `helm/brevio` chart layout.
- No ArgoCD manifests under `infra/argocd`.
- No `docker-compose.yml`.

## 8) Environment Configuration Patterns

Config patterns currently found:
- Runtime env variables centralized in `internal/config/registry.go` (`RequiredEnvVars`, `RequiredSecretKeys`).
- Secrets expected from external secret management pattern, not hardcoded.
- Service env usage via `os.Getenv` in service entrypoints and scripts.
- No `.env.example` or dedicated env schema package in TypeScript format.

## 9) Dependency Manifest and Lock Files

Present:
- `go.mod`
- `go.sum`
- Terraform lock files (`.terraform.lock.hcl`) per module/environment.

Absent:
- `pnpm-lock.yaml`
- JS/TS manifest lockfiles (`package-lock.json`, `yarn.lock`, etc.)
- Python dependency lockfiles for skill runtime.

## 10) Documentation and READMEs

Core docs present:
- `README.md`
- `docs/ARCHITECTURE.md`
- `docs/DEPLOYMENT.md`
- `docs/DEVELOPMENT.md`
- `docs/API_REFERENCE.md`
- `docs/codebase_audit_report.md`
- `docs/addendum_gap_audit.md`
- `runbooks/RB-001.md` ... `RB-009.md`
- `runbooks/RB-V92-001.md` ... `RB-V92-009.md`

## 11) Existing Connector/Tool Registry State

- Current seed source: `internal/connectors/seeds/connectors.yaml`.
- Seed counts:
  - connectors: **61**
  - tools: **64**
- This is connector/tool oriented and not yet the required **153 OpenClaw skill adapter** registry model.

## 12) Discovery Summary

The existing codebase is production-hardened for a **Go-based V9/V9.1/V9.2 architecture** with extensive contracts/tests/ops scaffolding. It is not yet aligned to the new **Brevio x OpenClaw TypeScript/pnpm 3-plane + 153-skill adapter** target model and requires additive coexistence migration rather than destructive rewrite.
