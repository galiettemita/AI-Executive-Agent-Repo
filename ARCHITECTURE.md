# BREVIO Architecture

## Overview

BREVIO is an Executive AI Agent platform built as a Go monorepo with five authoritative cloud planes, Temporal-based workflow orchestration, PostgreSQL as system of record, and OPA policy enforcement.

## Planes

### 1. Gateway Plane
- **Binary:** `cmd/gateway/`
- **Package:** `internal/gateway/`
- **Responsibility:** Ingress normalization, idempotency at channel boundary, validation, authn/authz boundary
- **Channels:** WhatsApp, iMessage, Web, Email, Voice
- **Key behaviors:**
  - HMAC signature verification for webhook channels
  - Replay detection via idempotency keys
  - workspace_id extraction and fail-closed enforcement
  - Rate limiting per workspace/channel
  - Message envelope normalization before forwarding to Brain

### 2. Brain Plane
- **Binary:** `cmd/brain/`
- **Package:** `internal/brain/`
- **Responsibility:** Planning, routing, prompt assembly; does NOT commit side effects
- **Key behaviors:**
  - Intent classification (LLM-backed with keyword fallback)
  - Plan generation with deterministic scoring (U(plan) formula)
  - Tool schema ordering (lexicographic for determinism)
  - Context budget enforcement
  - Temperature=0, top_p=1 for deterministic reasoning

### 3. Control Plane
- **Binary:** `cmd/control/`
- **Package:** `internal/control/`
- **Responsibility:** Sole authorizer + commit orchestrator; produces durable authorization receipts
- **Key behaviors:**
  - Gate evaluation chain: kill_switch → sandbox → skills → dm_pairing → call_approval → budget → rate_limit
  - Authorization receipt issuance with TTL (5min default, 30s for CRITICAL)
  - Non-bypassable enforcement: all side effects require valid receipt
  - Execution ledger persistence
  - Kill switch management

### 4. Executor/Hands Plane
- **Binary:** `cmd/executor/`
- **Package:** `internal/executor/`
- **Skills Runtime:** `services/brevio-hands/` (Node.js sidecar for TS OpenClaw skills)
- **Responsibility:** Executes approved activities/tools/skills
- **Key behaviors:**
  - Two-phase execution: simulate → commit
  - Receipt validation before any side effect
  - Idempotency enforcement via composite keys
  - Circuit breaker pattern for external tool calls
  - Compensation for failed multi-tool plans

### 5. Canvas Plane
- **Binary:** `cmd/canvas/`
- **Package:** `internal/canvas/`
- **Responsibility:** Interactive A2UI surface (WebSocket/UI integration)
- **Key behaviors:**
  - WebSocket connection management
  - Real-time workflow status streaming
  - Demo app surface

## Workflow Runtime

**Temporal is the ONLY workflow runtime.** All orchestration runs as Temporal workflows/activities.

### Core Workflows
| Workflow | Task Queue | Description |
|----------|------------|-------------|
| MessageProcessingWorkflow | brevio-core | Full message lifecycle: validate → classify → plan → authorize → execute → respond |
| OutboxDispatchWorkflow | brevio-core | Processes pending transactional outbox entries |
| ToolHealthEvaluationWorkflow | brevio-core | Periodic tool health scoring |
| OnboardingWorkflow | brevio-core | New workspace setup flow |
| CostRollupWorkflow | brevio-admin | Aggregates cost events into rollups |
| KillSwitchWorkflow | brevio-admin | Halts all workspace workflows on kill switch activation |

### Temporal Worker
- **Binary:** `cmd/temporal-worker/`
- **Package:** `internal/temporal/`
- Registers all workflows and activities
- Runs PII execution log scrubber

## Data Architecture

### PostgreSQL (System of Record)
- **pgvector** extension for vector storage
- **UUIDv7** (RFC 9562) for all primary keys — time-ordered, monotonic
- **RLS** (Row Level Security) on all tenant-scoped tables via `workspace_id`
- **Forward-only migrations** in `db/migrations/`
- No "default workspace" fallback; Gateway fails closed on missing/invalid workspace

### Migration Versions
| Migration | Version | Description |
|-----------|---------|-------------|
| 001 | V9 | Core schema: accounts, workspaces, users, channels, tools, policies |
| 002 | V9.1 | Soft intelligence: goals, trust, learning, capabilities |
| 003 | V9.2 | Production hardening: RAG, context budgets, guardrails, streaming |
| 004 | V9.3 | Operational systems: browser, marketing, agents, memory, routing, cron |
| 005 | V9.3+ | MCP execution & OAuth hardening |
| 006 | V9.3+ | Addendum specification closure |
| 007 | V10 | UUIDv7 reconciliation (RFC 9562 correct implementation) |
| 008 | V10 | Federation, wallet, cost tracking, capability registry |
| 009 | V10 | Authorization receipts, execution ledger, kill switch state |
| 010 | V10.1 | Admin intelligence: 18 tables for admin users, billing, usage |
| 011 | V10.2/V10.3 | EQ strategies, confidence calibration, metacognitive monitoring |
| 012 | V10.4 | Voice/call: 9 tables for call providers, approvals, transcripts |
| 013 | OpenClaw | Adoption features: hooks, A2A, queue lanes, sandbox, auth profiles |

### Rollback Strategy
Rollback is achieved via database snapshot + forward fix migration. Down migrations are not used in production. See RUNBOOK.md for rollback procedures.

## Security Architecture

### Authentication
- **User JWT:** Issued by identity service for workspace users
- **AdminJWT:** Separate issuance for admin users with MFA support
- **Service Identity:** Internal service-to-service authentication
- **Webhook HMAC:** SHA-256 signature verification for channel webhooks

### Authorization
- **OPA policies** in `policies/` directory
- **Gate chain:** kill_switch → sandbox → skills → dm_pairing → call_approval → budget → rate_limit
- **Authorization receipts:** Durable, auditable gate decisions
- **Deny-by-default:** No side effects without explicit policy allow + receipt

### Data Protection
- PII encryption at rest
- Execution log PII scrubber (scheduled background task)
- Transcript-only persistence for voice calls (no raw audio)
- KMS key rotation support

## Infrastructure

### Canonical Sources
- **Terraform:** `infra/terraform/` (3 environments: staging, production, dr)
- **Helm:** `infra/helm/` (umbrella chart) + `helm/` (per-service charts)
- **Docker:** `infra/docker/` (per-service Dockerfiles)
- **CI/CD:** `.github/workflows/ci.yml` (canonical)

### Cloud Resources (AWS)
- EKS for container orchestration
- RDS PostgreSQL with pgvector
- ElastiCache Redis
- S3 for object storage
- CloudFront CDN
- WAF for API protection
- SQS/SNS for async messaging
- Route53 DNS

## Observability
- OpenTelemetry (OTel) instrumentation
- Structured JSON logging
- Health check endpoints on all services (`/health`, `/health/deep`, `/healthz/ready`, `/healthz/live`)
- `brevioctl doctor` CLI for system diagnostics

## TypeScript Constraints
TypeScript is allowed ONLY for:
1. **Hands Skill Runtime** (`services/brevio-hands/`) — Node.js sidecar running OpenClaw skill corpus
2. **Edge Agent** (`edge/brevio-edge-agent/`) — Client-side agent
3. **Web Demo Frontend** (`apps/web-demo/`) — Demo application

All other TS services under `services/` are quarantined as NON_PRODUCTION. See `services/NON_PRODUCTION.md`.
