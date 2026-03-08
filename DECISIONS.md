# Binding Architectural Decisions

This document records all binding design decisions for the Brevio Executive AI Agent platform.

## D1 — Plane Runtime Boundaries

Cloud production planes are Go services: Gateway, Brain, Control, Executor/Hands, Canvas, Temporal Worker, and brevioctl CLI.

TypeScript is permitted only for:
- Hands skill runtime (to run existing TS OpenClaw skills)
- Edge agent
- Web/demo frontend

TS services duplicating cloud planes have been moved to `deprecated/` and excluded from CI release artifacts and production manifests.

## D2 — Temporal-Only Orchestration

All orchestration runs as Temporal workflows/activities using the Go Temporal SDK (`go.temporal.io/sdk`).
No in-process workflow simulators in production builds. Task queues:
- `brevio-core` — primary worker queue
- `brevio-gateway`, `brevio-brain`, `brevio-control`, `brevio-executor`, `brevio-canvas`, `brevio-admin` — per-plane queues

## D3 — Control-Plane Non-Bypassability

Control is the sole authorizer and commit orchestrator of side effects:
1. Control persists execution gate decisions in `authorization_receipts` table.
2. Control issues durable authorization receipts (signed, time-limited).
3. Activities verify receipts and refuse to execute without them.
4. Deny-by-default policy posture everywhere.

Receipt format: `{receipt_id, workspace_id, plan_id, tool_keys[], decision, issued_at, expires_at, signature_sha256}`.

## D4 — Tenancy and RLS

- `workspace_id` (UUIDv7) is the universal tenant isolation key.
- Every request must resolve workspace_id or fail (no "default workspace").
- Every DB session calls `SET app.workspace_id = $1` before queries.
- All production tables have RLS policies filtering on `workspace_id`.
- Implemented in `internal/database/pool.go` via `setWorkspaceIDOnSession`.

## D5 — IDs

- UUIDv7 (RFC 9562) required for all new primary keys.
- Generated via `uuid.Must(uuid.NewV7())` from `github.com/google/uuid`.
- Existing UUIDs accepted on reads only.
- Forward-only reconciliation migration at `db/migrations/007_BREVIO_uuidv7_reconciliation.sql`.

## D6 — Forward-Only Migrations

- `db/migrations/` is the only production migration chain.
- Legacy `migrations/` directory is quarantined as pre-v9 schema.
- Rollback strategy: snapshot + forward fix (no down migrations in production).

## D7 — Deterministic Temporal Retry Jitter

All retry logic uses this deterministic formula:
```
seed = workflow_id | activity | attempt
jitter_ms = fnv1a64(seed) % jitter_window_ms
backoff_ms = base_backoff_ms * 2^(attempt-1) + jitter_ms
clamped to max_backoff_ms
```
Implemented in `internal/temporal/jitter.go`.

## D8 — OpenClaw Skill Runtime

- TS skill corpus kept in `services/hands-runtime/`.
- Node Hands runtime exposes strict versioned contract: `list_skills`, `get_schema`, `execute_skill`, `health`, `metrics`.
- Go Executor activities call Hands runtime only after receipt verification.

## D9 — Persistence Strategy

All production domain state persists in PostgreSQL via pgx connection pool.
In-memory repositories exist only:
- In `*_test.go` files
- Behind `//go:build devtest` build tags under `internal/testing/`

Repository interfaces defined per domain package; pgx implementations injected at service startup.

## D10 — Similarity & Inference

- Semantic similarity uses OpenAI embeddings (`text-embedding-3-small`, 1536 dims) persisted in pgvector.
- Vector search uses `<=>` cosine distance operator with IVFFlat indexes.
- Lexical Jaccard similarity forbidden in production paths — only used in test fallbacks.
- Inference endpoints use real model calls (OpenAI, Anthropic) — hardcoded thresholds forbidden.

## D11 — Observability

- Structured JSON logging via `internal/runtime/logger.go`.
- Health checks: `/health`, `/health/deep`, `/healthz/ready`, `/healthz/live`.
- Metrics exposed at `/metrics` (Prometheus format when enabled).

## D001 — Federation Data Model

Federation between workspaces uses the following binding types:

- `negotiation_state`: Tracks the lifecycle of federation agreements between workspace pairs.
  States: `pending` → `accepted` | `rejected` | `revoked`. Stored in `federation_peers` table.
- `federation_permission_type`: Defines what data categories can be shared across federated workspaces.
  Types: `memory_read`, `memory_write`, `tool_delegate`, `knowledge_sync`, `lesson_share`.
- Federation sync is orchestrated via `FederationSyncWorkflow` (Temporal).
- All cross-workspace queries enforce RLS on both source and target workspace_id.

## D012 — Voice & Calling Pipeline

- Outbound calls use VAPI (primary) with Retell (fallback) provider pattern.
- Real-time voice uses LiveKit rooms with JWT token signing (HS256).
- STT/TTS use primary/fallback failover with 3-error threshold for circuit breaking.
- Post-session task extraction runs as a Temporal activity (`ExtractVoiceTasksActivity`).
- Voice sessions orchestrated via `VoiceSessionWorkflow` with heartbeat monitoring.

## D013 — Learning Consolidation Pipeline

- User corrections clustered by embedding similarity into lesson candidates.
- Conflict detection: redundant (>60% word overlap), superseded (same workspace, mixed status), contradictory (same workspace, both confirmed).
- Learning consolidation runs as `LearningConsolidationWorkflow` (Temporal).
- Rule proposals generated from confirmed lesson clusters.

## D014 — Runtime Deployment

- Target runtime: Kubernetes
- Ingress: nginx ingress controller
- Secrets: Kubernetes Secrets (baseline); optional Vault integration for production
- Probes: liveness/readiness/startup probes on every service
- Rate limiting: ingress + per-service limiter
- Service-to-service auth: mTLS in production; dev uses self-signed certs
- Audit logging required for privileged actions, policy decisions, receipts, and side effects

## D015 — Supply-Chain Security

- SBOM generation: CycloneDX for build artifacts
- Dependency vulnerability scanning: Go + Node as blocking CI checks
- Secret scanning in CI
- Container hardening: non-root, minimal base images (distroless), read-only filesystem, dropped capabilities, explicit healthchecks
- Signing/provenance: SBOM + scan attestations as minimum

## D016 — Dependency Lock Enforcement

- Go: `go mod tidy` in CI; fail if git diff is non-empty
- Node: `pnpm install --frozen-lockfile` enforced
- Corepack pins pnpm version: `9.15.4`
- No floating dependency ranges in production packages

## D017 — Verification Environment

- Go: 1.23.x (pinned in docker-compose.verify.yml)
- Node.js: 24.x Active LTS
- PostgreSQL: 16.x with pgvector
- Temporal Server: 1.25.2
- OpenTelemetry Collector (contrib): 0.96.0
- Kubernetes target: 1.29.x
- All images pinned with digests in docker-compose.verify.yml

## D018 — sync.Mutex Classification (S1 Refinement)

sync.Mutex usage is classified into two categories:
1. **Authoritative domain state** — FORBIDDEN in production. Must use pgx repositories.
2. **Transient operational state** (caching, rate limiters, circuit breakers, connection pools) — PERMITTED as in-process concurrency primitives.

The S1 verifier checks that no repository interface is bound to an in-memory implementation in production DI wiring. Transient mutex usage for thread safety is acceptable.

## D019 — internal/workflows/ Classification (D2 Refinement)

The 23 pure-Go state machines in `internal/workflows/` are classified as **domain state transition helpers**, not workflow orchestration. They:
- Track pipeline stages within Temporal activity execution context
- Are synchronous state transitions, not asynchronous workflow orchestration
- Do not start/signal/update external processes or bypass Temporal

This classification satisfies D2 (Temporal-only orchestration) because these are not orchestration bypasses.
