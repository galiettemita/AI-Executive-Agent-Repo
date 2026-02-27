# BREVIO V9 Codebase Audit Report

## Phase 0.1 — Structure Gap Report

| V9 Required Artifact | Exists? | Status (complete/partial/missing/extra) |
|---|---|---|
| `cmd/gateway/main.go` | No | missing |
| `cmd/brain/main.go` | No | missing |
| `cmd/control/main.go` | No | missing |
| `cmd/executor/main.go` | No | missing |
| `cmd/canvas/main.go` | No | missing |
| `internal/determinism/` | No | missing |
| `internal/contracts/` | No | missing |
| `internal/integration/` | No | missing |
| `internal/security/` | No | missing |
| `internal/provisioning/` | No | missing |
| `internal/onboarding/` | No | missing |
| `db/migrations/001_BREVIO_v9_init.sql` | No | missing |
| `api/openapi/v9.yaml` | No | missing |
| `schemas/*.json` | No | missing |
| `prompts/seed_prompts_v9.txt` | No | missing |
| `policies/*.rego` | No | missing |
| `terraform/modules/vpc/` | No | missing |
| `terraform/modules/eks/` | No | missing |
| `terraform/modules/rds/` | No | missing |
| `terraform/modules/elasticache/` | No | missing |
| `terraform/modules/sqs/` | No | missing |
| `terraform/modules/s3/` | No | missing |
| `terraform/modules/secrets/` | No | missing |
| `terraform/modules/temporal/` | No | missing |
| `terraform/modules/observability/` | No | missing |
| `helm/BREVIO-gateway/` | No | missing |
| `helm/BREVIO-brain/` | No | missing |
| `helm/BREVIO-control/` | No | missing |
| `helm/BREVIO-executor/` | No | missing |
| `helm/BREVIO-canvas/` | No | missing |
| `helm/BREVIO-temporal-worker/` | No | missing |
| `.github/workflows/ci.yaml` | No (`ci.yml` exists) | partial |
| `spec/traceability/compliance_matrix_v9.csv` | No | missing |
| `runbooks/RB-001.md` through `runbooks/RB-009.md` | No | missing |
| `evals/` | No | missing |
| `Dockerfile` | Yes | partial |
| `go.mod` | No | missing |
| `go.sum` | No | missing |

## Current Repository Summary

- Primary implementation is Python (`app/`, `tests/`, `alembic/`) plus TypeScript infra and a nested frontend project (`classmate-ai/`).
- No Go module, Go packages, or V9 directory layout currently exist.
- Existing CI and infra files target current Python/TypeScript stack, not the required V9 Go monorepo.

## Candidate Removal Inventory (Non-V9/V9.1/V9.2)

The following tracked areas do not map to the V9/V9.1/V9.2 required repository structure and are candidates for removal/refactor during Phase 0.2:

- `app/` (entire Python application tree)
- `tests/` (Python test suite)
- `alembic/` and `alembic.ini` (Python migration stack)
- `scripts/*.py` and current SQL helper scripts
- `infra/` (current SST/CDK layout; to be replaced by Terraform+Helm V9 layout)
- `classmate-ai/` nested application
- `openapi.json` (replace with `api/openapi/v9.yaml`)
- `requirements.txt`, `.python-version`, runtime Python artifacts (`app.db`, `__pycache__`, etc.)
- Existing docs not in V9 traceability/runbook set (retain only required V9/V9.1/V9.2 docs and generated outputs)
- Existing GitHub workflows not aligned to V9 Section 13.1 CI gates

## Phase 0.1 Notes

- `tree -L 3 --dirsfirst` was requested; `tree` is not installed in this environment, so equivalent depth-limited directory mapping was collected via `find`.
- Three new source blueprints are now tracked in Git:
  - `Brevio_V9_Consolidated_Master_Blueprint.docx`
  - `Brevio_V91_Addendum_Soft_Intelligence_Layer.docx`
  - `Brevio_V92_Addendum_Production_Hardening.docx`

## Phase 0.2 — Audit and Cleanup Outputs

### Files deleted (grouped) and reason

- Legacy Python application and runtime model removed: `app/`, `tests/`, `alembic/`, `alembic.ini`, `app.db`, `test_api.py`, `test_travel_api.py`.
  Reason: non-V9 architecture and tooling stack.
- Legacy TypeScript/SST infra removed: `infra/`, legacy workflow files under `.github/workflows/*.yml`, `docker-compose.yml`.
  Reason: replaced by V9 Terraform + Helm + `ci.yaml`.
- Legacy MCP/ops spec files removed from prior program: `MCP_Integration_Specification.docx`, `MCP_Server_Deployment_Plan.docx`, `MCP_Wave5_6_Expansion.docx`, `Operational_Systems_Blueprint.pdf`, `Auto_Provisioning_Engine.pdf`.
  Reason: replaced by V9/V9.1/V9.2 source blueprints in this program.
- Legacy helper scripts and artifacts removed: `scripts/`, `requirements.txt`, `runtime.txt`, `openapi.json`, `package-lock.json`, `storage/` tracked artifacts.
  Reason: non-canonical for V9 Go monorepo.
- Legacy documentation set removed (`docs/*` old files and reports).
  Reason: replaced by V9 traceability/runbooks and current audit artifacts.

### Files renamed (old -> new)

- None.

### Duplicate detection results

- `jscpd` invocation attempted but was not usable in this environment (hung during `npx` execution).
- Equivalent practical result after cleanup: only fresh scaffold sources remain, and no duplicate Go code blocks were identified manually across `cmd/` and `internal/`.

### Naming violations fixed

- Legacy non-V9 naming conventions were removed with legacy codebase deletion.
- New scaffold uses V9-compatible naming baseline:
  - snake_case file naming in internal packages and migration path
  - connector/tool schema placeholders using `connector_key.tool_key`-compatible pattern file intents
  - event/metric naming deferred to policy/event implementation phases
  - UUIDv7 helper scaffolded in `internal/determinism/uuid_v7.go`

## Phase 0.3 — Clean Baseline Status

### Baseline scaffold established

- Canonical V9 repository skeleton created for:
  - `cmd/{gateway,brain,control,executor,canvas}`
  - `internal/{determinism,contracts,integration,security,provisioning,onboarding,...}`
  - `db/migrations`, `api/openapi`, `schemas`, `prompts`, `policies`
  - `terraform/modules/*`, `terraform/environments/{staging,production}`
  - `helm/BREVIO-*`, `runbooks/RB-001..009`, `spec/traceability`, `evals`

### Validation command results

- Local `go` toolchain is not installed on host (`go: command not found`).
- Validation executed via Docker `golang:1.22`:
  - `go mod tidy` -> success
  - `go build ./...` -> success
  - `go vet ./...` -> success
  - `gofmt -l .` -> empty (pass)
  - `go test ./... -count=1` -> success (no test files yet)
  - `staticcheck` (v0.5.1 for Go 1.22 compatibility) -> success
