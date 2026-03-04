# Brevio Service Dockerfiles

This directory contains service-specific production image definitions aligned to the
Brevio runtime binaries in `cmd/`.

## Mapping

- `Dockerfile.gateway` -> `cmd/gateway`
- `Dockerfile.brain` -> `cmd/brain`
- `Dockerfile.hands` -> `cmd/executor` (hands plane runtime)
- `Dockerfile.control` -> `cmd/control`
- `Dockerfile.executor` -> `cmd/executor`
- `Dockerfile.canvas` -> `cmd/canvas`
- `Dockerfile.temporal-worker` -> `cmd/temporal-worker`
- `Dockerfile.auth` -> `services/brevio-auth`
- `Dockerfile.profile` -> `services/brevio-profile`
- `Dockerfile.scheduler` -> `services/brevio-scheduler`
- `Dockerfile.metrics` -> `services/brevio-metrics`
- `Dockerfile.edge-relay` -> `services/brevio-edge-relay`

Go service Dockerfiles use `gcr.io/distroless/static:nonroot`.
TypeScript service Dockerfiles use `gcr.io/distroless/nodejs20-debian12:nonroot`.
