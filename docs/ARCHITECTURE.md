# ARCHITECTURE

## Runtime Planes
- Gateway: ingress/outbound channel handling and webhook security
- Brain: planning, reasoning, and deterministic LLM interaction
- Control: policy/firewall/execution gating
- Executor: tool execution with simulate/commit semantics
- Canvas: realtime A2UI interaction surface

## Data and Workflows
- PostgreSQL schema is defined via forward-only migrations in `db/migrations/`
- Workflows are implemented in `internal/workflows/`
- Policy bundles are in `policies/`
- Contracts are in `schemas/` and `api/openapi/v9.yaml`

## Infrastructure
- Terraform modules define platform components (network, compute, data, messaging, observability)
- Helm charts define service deployments and autoscaling manifests
