# Pipeline Environment Detection — Segment 0

## Language & Runtime
- **Language**: Go 1.23
- **Module**: `github.com/brevio/brevio`

## Dependency Manager
- **Go modules**: `go.mod` / `go.sum`
- **Node (auxiliary)**: pnpm (`pnpm-lock.yaml`, `pnpm-workspace.yaml`)

## Build System
- **Primary**: `go build ./...` (via `Makefile` → `scripts/dev/go_exec.sh`)
- **Container**: `Dockerfile`, `docker-compose.yml`
- **Orchestration**: Helm charts (`helm/`), Terraform (`terraform/`)

## Test Framework
- **Unit/Integration**: `go test ./... -count=1`
- **Contract tests**: `internal/contracts/`, `tests/contract/`
- **Database verification**: `scripts/database/verify_postgres_migrations.sh` (Docker + pgvector/pg16)
- **Evals**: `evals/`, `scripts/run-evals.sh`

## Key Dependencies
- **Temporal**: `go.temporal.io/sdk v1.31.0` — workflow orchestration
- **PostgreSQL**: `pgx/v5 v5.7.4` — database driver
- **WebSocket**: `gorilla/websocket v1.5.3` — real-time streaming
- **UUID**: `google/uuid v1.6.0` — deterministic ID generation

## Service Entrypoints (cmd/)
| Service | Path |
|---------|------|
| gateway | `cmd/gateway/main.go` |
| temporal-worker | `cmd/temporal-worker/main.go` |
| executor | `cmd/executor/main.go` |
| brain | `cmd/brain/main.go` |
| memory | `cmd/memory/main.go` |
| router | `cmd/router/main.go` |
| agents | `cmd/agents/main.go` |
| canvas | `cmd/canvas/main.go` |
| browser | `cmd/browser/main.go` |
| marketing | `cmd/marketing/main.go` |
| cron | `cmd/cron/main.go` |
| control | `cmd/control/main.go` |
| brevioctl | `cmd/brevioctl/main.go` |
