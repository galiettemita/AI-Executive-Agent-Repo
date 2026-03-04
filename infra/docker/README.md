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

All Dockerfiles use multi-stage builds with a distroless non-root runtime image.
