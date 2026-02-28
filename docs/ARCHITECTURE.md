# ARCHITECTURE

## Runtime Planes
- Gateway: webhook ingress, signature verification, queueing, outbound dispatch
- Brain: deterministic planning and LLM orchestration
- Control: policy gates, firewall, rate/budget enforcement
- Executor: tool simulate/commit, trust receipts, audit emissions
- Canvas: A2UI websocket interaction surface
- Temporal Worker: workflow runtime for interactive/provisioning/onboarding/drift and V9.1/V9.2 additive workflows

## Data and Workflows
- PostgreSQL migrations: `db/migrations/001_BREVIO_v9_init.sql`, `002_BREVIO_v91_soft_intelligence.sql`, `003_BREVIO_v92_production_hardening.sql`
- Workflow engine code: `internal/workflows/`
- Policy bundles: `policies/`
- API and schema contracts: `api/openapi/v9.yaml`, `schemas/`

## Infrastructure
- Terraform modules in `terraform/modules/`
- Environment compositions in `terraform/environments/staging|production`
- Helm charts in `helm/` for runtime services and V9.2 add-ons
