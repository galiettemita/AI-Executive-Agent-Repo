# BREVIO Monorepo

BREVIO is an AI executive assistant platform implemented as a Go monorepo with deterministic workflow execution, policy-gated tool actions, and AWS-targeted infrastructure artifacts.

## Services
- `gateway`
- `brain`
- `control`
- `executor`
- `canvas`
- `temporal-worker`

## Quick Start
1. Run formatting/build/tests:
   - `go test ./... -count=1`
2. Review API contract:
   - `api/openapi/v9.yaml`
3. Review database schema migrations:
   - `db/migrations/001_BREVIO_v9_init.sql`
   - `db/migrations/002_BREVIO_v91_soft_intelligence.sql`
   - `db/migrations/003_BREVIO_v92_production_hardening.sql`

## Key Directories
- `cmd/` service entrypoints
- `internal/` domain packages
- `schemas/` JSON schema contracts
- `policies/` OPA policy bundles
- `terraform/` infrastructure modules/environments
- `helm/` Kubernetes chart manifests
- `runbooks/` incident/operational procedures
- `spec/traceability/` compliance matrices
