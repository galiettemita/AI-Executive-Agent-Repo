# Deprecated Services

These TypeScript services are quarantined duplicates of the authoritative Go services
under `cmd/` and `internal/`. They MUST NOT be deployed to production, included in
CI release artifacts, or referenced from production Kubernetes manifests.

Per binding decision D1, cloud production planes are Go services only.

## Quarantined Services

| Service | Replaced By |
|---------|-------------|
| brevio-gateway | cmd/gateway + internal/gateway |
| brevio-brain | cmd/brain + internal/brain |
| brevio-temporal-worker | cmd/temporal-worker + internal/temporal |
| brevio-metrics | internal/observability |
| brevio-scheduler | cmd/cron |
| brevio-edge-relay | edge/ |
| brevio-profile | internal/identity |
| brevio-auth | internal/identity + internal/rbac |

## Allowed TypeScript

Only these TypeScript codebases are permitted in production:

- `services/hands-runtime/` — OpenClaw skill runtime (D8)
- `edge/` — Edge agent
- `apps/web-demo/` — Demo UI frontend
