# ADR-001: Existing Codebase Audit

**Status:** Accepted
**Date:** 2026-03-06

## Context

Before implementing the Brevio Blueprint Addendum (42 skills / 6 categories), we audited the existing monorepo to understand conventions, schema, and infrastructure.

## Findings

### Repository Structure

- **Language:** Go 1.23 (primary), TypeScript (secondary services)
- **Monorepo layout:**
  - `cmd/` — Go service entry points: gateway, brain, control, executor, canvas, temporal-worker
  - `internal/` — Go shared packages (gateway, llm, brain, control, executor, canvas, workflows, runtime, contracts, etc.)
  - `services/` — TypeScript microservices: brevio-auth, brevio-profile, brevio-scheduler, brevio-metrics, brevio-edge-relay
  - `migrations/` — Active SQL migration system (001-011, up/down pairs)
  - `db/migrations/` — Legacy BREVIO schema (001-006, single files) — superseded by `migrations/`
  - `infra/` — Terraform modules, Helm charts, Kubernetes manifests, Docker service Dockerfiles
  - `scripts/` — Dev, security, database, deployment scripts
  - `.github/workflows/` — CI/CD: ci.yaml, ci.yml (ci-openclaw), security-scan.yml, deploy-*.yml, llm-evals.yml

### Go Service Conventions

- **HTTP:** `net/http` stdlib with `mux.HandleFunc("METHOD /path", handler)` (Go 1.22+ routing)
- **Config:** `os.Getenv` via `LoadEnvConfig` / `LoadServiceEnvConfig` helpers in `internal/runtime`
- **Server:** `runtimeserver.ServeWithGracefulShutdown(name, addr, handler)` shared utility
- **Logger:** `runtimeserver.NewJSONLogger(service, env)` with structured JSON output
- **Database:** `github.com/jackc/pgx/v5` (PostgreSQL)
- **WebSocket:** `github.com/gorilla/websocket`
- **Testing:** Go stdlib `testing` package, table-driven tests
- **Validation:** Manual validation in handler functions (no schema library)

### TypeScript Service Conventions

- **Runtime:** Node.js 20, TypeScript
- **Package manager:** pnpm (workspace)
- **Build:** `tsc -p tsconfig.json`
- **Docker base:** `gcr.io/distroless/nodejs20-debian12:nonroot`

### Database Schema (Active: `/migrations/`)

- **Migrations:** 011 total, `NNN_name.up.sql` / `NNN_name.down.sql` pairs
- **UUID strategy:** `public.uuid_v7_now()` (wraps `gen_random_uuid()`)
- **Type strategy:** VARCHAR with CHECK constraints (not custom ENUM types)
- **RLS:** Enabled on key tables, uses `current_setting('app.user_id', true)` and `current_setting('app.role', true)`
- **Schemas:** public, skills, auth, billing, temporal
- **Tables (19 total):**
  - public: users, messages (partitioned), message_channel_dedup, sessions, edge_agents
  - skills: registry, execution_log (partitioned), circuit_breaker_state
  - auth: oauth_tokens, api_keys, refresh_events, rbac_bindings
  - billing: usage_daily, monthly_budgets, cost_allocations
  - temporal: workflow_snapshots, compensation_logs
- **Key columns on users:** id (UUID PK), phone_number, email, channel, tier, timezone, enabled_skills, monthly_llm_budget_cents, preferences (JSONB)

### Infrastructure

- **Docker:** Multi-stage builds with `golang:1.23` builder + `gcr.io/distroless/static:nonroot` runtime
- **Compose:** 15+ services including postgres (pgvector:pg16), redis:7-alpine, temporalio/auto-setup:1.25.2
- **Terraform:** Modules for AWS VPC, EKS, RDS, ElastiCache, IAM
- **Helm:** Charts per service with deployment, service, HPA, servicemonitor templates
- **CI/CD:** GitHub Actions with lint, test, contract test, security scan, docker build, deploy gates

## Decisions

### Migration Numbering

The blueprint specifies migrations 100-166. Since the active schema ends at 011, we'll renumber to 012-050 to maintain sequential ordering. The mapping:
- Blueprint 100-105 (browser) → 012-017
- Blueprint 110-116 (marketing) → 018-024
- Blueprint 120-124 (agents) → 025-029
- Blueprint 130-134 (memory) → 030-034
- Blueprint 140-144 (routing) → 035-039
- Blueprint 150-154 (cron) → 040-044
- Blueprint 160-166 (individual skills) → 045-051

### Service Implementation

New Brain-plane services (agents, memory, router, cron) will be implemented in Go to match existing `cmd/` + `internal/` patterns. New Hands-plane services (browser, marketing) could be TypeScript (matching `services/` pattern) but will also use Go for consistency with the core architecture.

### Type Strategy

Following active schema convention: VARCHAR with CHECK constraints, NOT custom PostgreSQL ENUM types. The legacy schema uses enums but the active migrations do not.

### No `conversations` Table

The blueprint references `conversations` but the active schema uses `sessions` + `messages`. New tables will reference `users(id)` directly rather than a non-existent conversations table.
