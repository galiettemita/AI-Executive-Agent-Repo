# Repository Discovery — Segment 1

## Service Entrypoints
13 service binaries under `cmd/`:
- **gateway** — HTTP/WebSocket API gateway
- **temporal-worker** — Temporal workflow worker
- **executor** — tool/action execution runtime
- **brain** — cognitive intelligence / LLM orchestration
- **memory** — memory & RAG subsystem
- **router** — request routing / delegation
- **agents** — agent lifecycle management
- **canvas** — collaborative canvas service
- **browser** — browser automation service
- **marketing** — marketing automation
- **cron** — scheduled job runner
- **control** — control plane / admin
- **brevioctl** — CLI management tool

## Worker Processes
- Temporal worker (`cmd/temporal-worker/`) — executes workflows and activities defined in `internal/temporal/`, `internal/workflows/`

## Database Migrations
18 forward-only migrations in `db/migrations/` (001 → 018):
- 001: v9 init schema
- 002–003: v9.1–v9.2 soft intelligence + production hardening
- 004: operational systems
- 005: MCP execution + OAuth hardening
- 006: v9.3 addendum specification closure
- 007: UUIDv7 reconciliation
- 008: v10 gap closure
- 009: v10 authorization receipts
- 010: v10.1 admin intelligence
- 011: v10.2/v10.3 intelligence
- 012: v10.4 voice calls
- 013: OpenClaw adoption
- 014: gateway production hardening
- 015: v10.1 cost/revenue intelligence
- 016: v10.3 cognitive architecture
- 017: v10.2 intelligence gap closure
- 018: v10.2 memory/context/RAG/latency

## Runtime Subsystems
| Subsystem | Package |
|-----------|---------|
| LLM orchestration | `internal/llm/` |
| Temporal workflows | `internal/temporal/`, `internal/workflows/` |
| MCP server fleet | `internal/mcp/` |
| Memory / RAG | `internal/memory/`, `internal/rag/` |
| Tool execution | `internal/runtime/` |
| RBAC / Auth | `internal/rbac/` |
| Guardrails | `internal/guardrails/` |
| Trust scoring | `internal/trust/` |
| Sessions | `internal/sessions/` |
| Streaming | `internal/streaming/` |
| Feature flags | `internal/feature_flags/` |
| Admin | `internal/admin/` |
| Voice | `internal/voice/` |
| Wallet / billing | `internal/wallet/` |
| Observability | `internal/observability/` |
| Security (PII, sandbox) | `internal/security/` |

## Build Verification
- `go mod tidy` — clean
- `go build ./...` — **PASS** (all packages compile)
- `go test ./... -count=1` — **2 failures** (see Baseline Status)
