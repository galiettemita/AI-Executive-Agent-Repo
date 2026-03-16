# BREVIO Operations Runbook

## Quick Reference

### System Health Check
```bash
# Run doctor CLI
brevioctl doctor

# Manual health checks
curl http://localhost:18080/health/deep  # Gateway
curl http://localhost:18081/health/deep  # Brain
curl http://localhost:18082/health/deep  # Control
curl http://localhost:18083/health/deep  # Executor
curl http://localhost:18084/health/deep  # Temporal Worker
curl http://localhost:18793/health/deep  # Canvas
```

### Key Ports
| Service | Port | Description |
|---------|------|-------------|
| Gateway | 18080 | API ingress |
| Brain | 18081 | Planning/routing |
| Control | 18082 | Authorization |
| Executor | 18083 | Tool execution |
| Temporal Worker | 18084 | Workflow processing |
| Canvas | 18793 | WebSocket/UI |
| PostgreSQL | 5432 | Database |
| Redis | 6379 | Cache |
| Temporal | 7233 | Workflow engine |
| Temporal UI | 8088 | Workflow dashboard |

## Incident Procedures

### INC-001: Kill Switch Activation
**Severity:** P1
**Impact:** All workspace operations halted

1. Verify kill switch status: `SELECT * FROM kill_switch_state WHERE is_active = true;`
2. Identify activating admin: check `admin_audit_log` for `kill_switch_toggle` action
3. Assess impact: count affected workflows via Temporal UI
4. To deactivate (requires super_admin):
   ```sql
   UPDATE kill_switch_state SET is_active = false, deactivated_at = now() WHERE workspace_id = '<ws_id>';
   ```
5. Verify workflows resume via Temporal UI
6. Post-incident: review activation reason and add guardrails if needed

### INC-002: DLQ Backlog Growing
**Severity:** P2
**Impact:** Messages not being processed

1. Check DLQ depth: `brevioctl doctor` (dlq_backlog check)
2. Inspect failed events: `SELECT * FROM outbox_events WHERE status = 'failed' ORDER BY created_at DESC LIMIT 20;`
3. Common causes:
   - Temporal worker not polling → restart worker
   - Database connection pool exhausted → check pg_stat_activity
   - External API failures → check tool health scores
4. Reprocess failed events after fixing root cause

### INC-003: Temporal Worker Not Polling
**Severity:** P1
**Impact:** No workflows executing

1. Check worker health: `curl http://localhost:18084/health/deep`
2. Check Temporal connectivity: `curl http://localhost:7233/health`
3. Verify task queue registration in Temporal UI
4. Check worker logs for registration errors
5. Restart worker: `docker-compose restart temporal-worker`

### INC-004: Authorization Receipt Failures
**Severity:** P2
**Impact:** No side effects can be committed

1. Check Control service: `curl http://localhost:18082/health/deep`
2. Verify kill switch not active
3. Check policy bundle: `brevioctl doctor` (policy_bundle_load check)
4. Review `authorization_receipts` table for recent denials
5. Check `execution_gate_decisions` for gate failures

### INC-005: Database Migration Failure
**Severity:** P2
**Impact:** Schema out of sync

1. Do NOT attempt down migration in production
2. Identify failed migration version
3. Check migration error in logs
4. Create forward fix migration addressing the issue
5. Test fix on staging first
6. Apply fix migration: `make migrate`

### INC-006: OPA Sidecar Unavailable
**Severity:** P1
**Impact:** All policy decisions denied (deny-by-default posture D025)

1. Check OPA sidecar: `curl http://localhost:8181/health`
2. Check circuit breaker status in Control logs: search for `opa_circuit_open`
3. OPA down → all new tool executions are DENIED (no fallback in production)
4. To restore:
   ```bash
   kubectl rollout restart deployment/opa -n brevio
   curl http://localhost:8181/v1/data/brevio/control/allow
   ```
5. Circuit breaker auto-recovers after 30s cooldown once OPA responds
6. Post-incident: review why OPA crashed; check policy bundle syntax

### INC-007: Temporal Stuck Workflows
**Severity:** P2
**Impact:** Messages stuck in processing

1. Open Temporal UI: http://localhost:8088
2. Filter workflows by status: Running with age > 10 minutes
3. Check if worker is polling: `curl http://localhost:18084/health/deep`
4. Common causes:
   - Activity timeout too short → check `StartToCloseTimeout` in workflow code
   - Activity panicking → check worker logs for stack traces
   - Database lock contention → check `pg_stat_activity` for blocked queries
5. For truly stuck workflows (rare):
   ```bash
   tctl workflow terminate -w <workflow_id> -r <run_id> --reason "stuck_workflow_recovery"
   ```
6. After fix, affected messages will be reprocessed from ingress queue

### INC-008: Outbox DLQ Overflow
**Severity:** P2
**Impact:** Outbound messages not delivered

1. Check DLQ depth:
   ```sql
   SELECT COUNT(*) FROM outbox_items WHERE status = 'dlq';
   ```
2. Inspect recent failures:
   ```sql
   SELECT id, event_type, target, attempts, created_at
   FROM outbox_items WHERE status = 'dlq'
   ORDER BY created_at DESC LIMIT 20;
   ```
3. Common causes:
   - Target channel unreachable (Slack/WhatsApp API down)
   - Malformed payload → inspect `payload` column
   - Rate limited by provider → check tool health scores
4. To reprocess after fixing root cause:
   ```sql
   UPDATE outbox_items SET status = 'pending', attempts = 0
   WHERE status = 'dlq' AND created_at > now() - interval '1 hour';
   ```
5. `OutboxDispatchWorkflow` will pick up requeued items automatically

### INC-009: Load Shedding Tier Escalation
**Severity:** P1 (D4/D5)
**Impact:** System shedding work; some requests rejected

1. Check current tier:
   ```sql
   SELECT * FROM load_shedding_state;
   ```
2. D1-D3: Auto-recovery after 5+ minutes of healthy metrics
3. D4: Requires operator confirmation to recover
4. D5: Manual-only (activated by operator, not automated)
5. Investigate cause: CPU? Error rate? DB pool?
   ```sql
   SELECT count(*), state FROM pg_stat_activity GROUP BY state;
   ```
6. D4 recovery:
   ```sql
   UPDATE load_shedding_state SET current_tier = 'D0', reason = 'operator_recovery'
   WHERE workspace_id = '<ws_id>';
   ```

## Production Boundary

The repository enforces a strict production boundary. The following artifacts are **excluded** and must never reappear:

- **Demo UI** (`apps/web-demo/`): BP01 (4 Features Blueprint) is demo-only and excluded from production.
- **In-memory production wiring**: Forbidden outside `*_test.go` and `//go:build devtest` tagged files (D9).

**CI enforcement:**
- `TestDemoExclusionClosure` (`internal/contracts/demo_exclusion_closure_test.go`) — fails if demo paths exist or if Go source references demo artifacts.
- `TestBlueprintRequirementMapping/BP01_demo_excluded` (`internal/contracts/blueprint_coverage_test.go`) — fails if `apps/web-demo` directory exists.

**Operational doctrine:** This repository follows a **12-prompt staged implementation pipeline**. Each prompt is executed sequentially with acceptance gates, self-audit, and git checkpoints. Adding demo code or bypassing the production boundary violates D12.

## Required Environment Variables

### Core Infrastructure
| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `BREVIO_ENV` | Yes | — | Environment: `development`, `staging`, `production` |
| `DATABASE_URL` | Yes | — | PostgreSQL connection string (e.g. `postgres://user:pass@localhost:5432/brevio`) |
| `REDIS_URL` | Yes | — | Redis connection string (e.g. `redis://localhost:6379/0`) |
| `TEMPORAL_HOST` | Yes | `localhost:7233` | Temporal server gRPC address |
| `TEMPORAL_NAMESPACE` | Yes | `default` | Temporal namespace |

### LLM Provider Keys
| Variable | Required | Description |
|----------|----------|-------------|
| `ANTHROPIC_API_KEY` | Yes (prod) | Anthropic Messages API key (primary LLM provider) |
| `OPENAI_API_KEY` | Yes (prod) | OpenAI API key (failover provider) |

### OPA Policy Engine
| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `OPA_URL` | No | — | OPA sidecar HTTP URL (e.g. `http://localhost:8181`). When set, all /v1/ control routes are gated. When unset, embedded gate logic is used (devtest only). |
| `OPA_TIMEOUT_MS` | No | `2000` | OPA request timeout in milliseconds |
| `OPA_POLICIES_DIR` | No | `policies` | Directory containing .rego policy files |

### Hands Runtime
| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `HANDS_LISTEN_ADDR` | No | `:18090` | Hands runtime HTTP listen address |

### Messaging & Channels
| Variable | Required | Description |
|----------|----------|-------------|
| `SQS_INTERACTIVE_TURNS_URL` | Yes | SQS queue URL for ingress |
| `S3_ATTACHMENTS_BUCKET` | Yes | S3 bucket for attachments |
| `S3_SBOMS_BUCKET` | Yes | S3 bucket for SBOMs |
| `WHATSAPP_PHONE_NUMBER_ID` | Yes | WhatsApp phone number ID |
| `WHATSAPP_API_VERSION` | Yes | WhatsApp API version |
| `IMESSAGE_MSP_BASE_URL` | Yes | iMessage MSP base URL |
| `IMESSAGE_BUSINESS_ID` | Yes | iMessage business ID |

### Observability
| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Yes | — | OpenTelemetry collector endpoint |
| `LOG_LEVEL` | No | `info` | Log level: `debug`, `info`, `warn`, `error` |

## Local Development

### Prerequisites
- Go 1.23+
- Docker & Docker Compose
- PostgreSQL 16+ (via Docker or local install)
- Redis 7+ (via Docker or local install)

### Quick Start
```bash
# 1. Start infrastructure
docker compose up -d postgres redis temporal temporal-ui

# 2. Apply database migrations
make migrate

# 3. Seed the tool registry
go run ./cmd/brevioctl seed tools

# 4. Run the build + test suite
make local-verify

# 5. Start individual services (separate terminals)
go run ./cmd/gateway          # :18080
go run ./cmd/control          # :18082
go run ./cmd/hands            # :18090
go run ./cmd/temporal-worker  # :18084
```

### Minimal .env for Local Dev
```env
BREVIO_ENV=development
DATABASE_URL=postgres://brevio:brevio@localhost:5432/brevio?sslmode=disable
REDIS_URL=redis://localhost:6379/0
TEMPORAL_HOST=localhost:7233
TEMPORAL_NAMESPACE=default
ANTHROPIC_API_KEY=sk-ant-...   # Required for real LLM calls
OPENAI_API_KEY=sk-...          # Required for failover
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
LOG_LEVEL=debug
```

Note: Without `ANTHROPIC_API_KEY`/`OPENAI_API_KEY`, the LLM layer falls back to deterministic keyword-based classification and plan generation. This is sufficient for testing the execution pipeline but does not exercise real LLM inference.

## Troubleshooting

### Replay Cache Behavior
The LLM service uses a Redis-backed replay cache (`replay:{hash}`) with 24h TTL to deduplicate identical requests.

- **Cache hit:** Returns stored result instantly (no LLM call, no token cost).
- **Cache miss:** Calls LLM, stores result in both Redis and in-memory cache.
- **Redis down:** Falls back to in-memory cache (per-instance only, no cross-instance dedup).
- **Clearing cache:** `redis-cli DEL $(redis-cli KEYS 'replay:*')` or wait for TTL expiry.
- **Diagnosis:** Check `replay_hit_count` in service metrics or log lines with `replay_cache_hit`.

### Rate Limiting Behavior
The LLM service enforces per-workspace, per-provider rate limits via Redis fixed-window counters.

- **Keys:** `rl:llm:{provider}:{workspace}:req:{window}` (requests/min), `rl:llm:{provider}:{workspace}:tok:{window}` (tokens/min).
- **Default limits:** 60 requests/min, 100K tokens/min per workspace per provider.
- **When limited:** Returns `RateLimitError` which Temporal classifies as retryable (auto-retry with backoff).
- **Redis down:** Rate limiting is effectively disabled (fail-open) — requests proceed.
- **Window reset:** Counters expire automatically at window boundary (1 minute).

### OPA Policy Enforcement
When `OPA_URL` is set, the control service gates all `/v1/` API routes through OPA middleware.

- **OPA reachable:** Policy evaluated normally (allow/deny/require_approval).
- **OPA unreachable:** Deny-by-default (403 Forbidden with `OPA_UNAVAILABLE_DENY_BY_DEFAULT`).
- **Circuit breaker:** Opens after 3-5 consecutive failures; auto-recovers after 30s cooldown.
- **Health exempt:** `/health`, `/healthz/live`, `/healthz/ready`, `/docs` are never gated.
- **Devtest mode:** When `OPA_URL` is not set, embedded Go gate logic is used (no OPA dependency).

### pgvector / Embedding Layer
The memory and RAG subsystems use PostgreSQL's pgvector extension for vector similarity search.

- **Extension:** `CREATE EXTENSION vector` is in migration 001. Requires PostgreSQL compiled with pgvector.
- **Dimensions:** All embeddings are 1536-dimensional (`text-embedding-3-small`).
- **Indexes:** HNSW indexes on `rag_chunks.embedding` (migration 003) and `memory_items.embedding` (migration 019) using `vector_cosine_ops`.
- **Type registration:** pgvector types are registered via `AfterConnect` callback in `internal/database/pool.go`.
- **Embedding cache:** In-memory TTL cache (default 24h, max 10K entries) in `EmbeddingService`. No cross-instance sharing.
- **Degraded mode:** Without `OPENAI_API_KEY`, embeddings fall back to deterministic provider (fixed-dimension zero vectors). RAG search will return no results but pipeline does not error.
- **Schema alignment:** Migration 019 adds `user_id`, `embedding_version`, `expires_at` to `memory_items` and `metadata` to `rag_chunks`.

## Deployment Procedures

### Standard Deployment
1. CI passes all gates (A-H)
2. Deploy to staging: `make deploy-helm ENVIRONMENT=staging`
3. Run staging smoke tests: `make staging-smoke-tests`
4. Canary deployment to production (10% traffic)
5. Monitor SLOs for 15 minutes
6. Full rollout

### Rollback Procedure
1. **Application rollback:** Revert to previous Helm chart version
   ```bash
   helm rollback brevio <previous-revision> -n brevio
   ```
2. **Database rollback:** Restore from latest snapshot
   ```bash
   # Identify latest snapshot
   aws rds describe-db-snapshots --db-instance-identifier brevio-production
   # Restore (creates new instance)
   aws rds restore-db-instance-from-db-snapshot --db-instance-identifier brevio-production-rollback --db-snapshot-identifier <snapshot-id>
   ```
3. Create forward fix migration for any schema issues
4. Deploy fix through normal pipeline

### Migration Execution
```bash
# Verify migration files
make migrate

# Apply to database
psql $DATABASE_URL -f db/migrations/NNN_migration_name.sql

# Verify with doctor
brevioctl doctor
```

## Monitoring

### Key Metrics
- **p99 message processing latency:** < 5s
- **First byte streaming latency:** < 500ms
- **Authorization receipt issuance latency:** < 100ms
- **Tool health score:** > 0.8 for all active tools
- **DLQ depth:** < 100 events
- **Kill switch activations:** alert on any

### Dashboards
- Temporal UI: http://localhost:8088 (workflow execution status)
- Health endpoints: /health/deep on each service (dependency status)
- Doctor CLI: `brevioctl doctor` (comprehensive system check)

## Doctor CLI Usage

```bash
# Run full diagnostic
brevioctl doctor

# Expected output (healthy):
{
  "overall": "HEALTHY",
  "passed": 8,
  "failed": 0,
  "warned": 0,
  "checks": [
    {"name": "db_connectivity", "status": "pass", "...": ""},
    {"name": "migrations_applied", "status": "pass", "...": ""},
    {"name": "temporal_reachable", "status": "pass", "...": ""},
    {"name": "worker_polling", "status": "pass", "...": ""},
    {"name": "otel_exporter", "status": "pass", "...": ""},
    {"name": "policy_bundle_load", "status": "pass", "...": ""},
    {"name": "kill_switch_status", "status": "pass", "...": ""},
    {"name": "dlq_backlog", "status": "pass", "...": ""}
  ]
}
```

### Exit Codes
- `0`: All checks pass (HEALTHY)
- `1`: One or more checks failed (UNHEALTHY)

### Environment Variables
| Variable | Required | Description |
|----------|----------|-------------|
| DATABASE_URL | Yes | PostgreSQL connection string |
| TEMPORAL_HOST | No | Temporal server address (default: localhost:7233) |
| TEMPORAL_WORKER_LISTEN_ADDR | No | Worker health endpoint (default: :18084) |
| OTEL_EXPORTER_OTLP_ENDPOINT | No | OpenTelemetry collector endpoint |

## Verification Environment

### Running the deterministic verify environment
```bash
# Start infrastructure
docker compose -f docker-compose.verify.yml up -d postgres temporal otel-collector

# Run Go verification (one-shot)
docker compose -f docker-compose.verify.yml run --rm verify-go

# Run Node verification (one-shot)
docker compose -f docker-compose.verify.yml run --rm verify-node

# Teardown
docker compose -f docker-compose.verify.yml down -v
```

### Pinned Versions (D017)
| Component | Version |
|-----------|---------|
| Go | 1.23.x |
| Node.js | 24.x (Active LTS) |
| PostgreSQL | 16.x + pgvector |
| Temporal Server | 1.25.2 |
| OTel Collector | 0.96.0 |
| pnpm | 9.15.4 (via Corepack) |

### brevioctl verify commands
```bash
# Gate A — Blueprint Coverage
brevioctl verify blueprint-coverage
brevioctl verify requirements-graph
brevioctl verify traceability-matrix

# Gate B — Architecture Coherence
brevioctl verify no-inmemory-prod

# Gate C — Data and Contract Integrity
brevioctl verify schema-closure
brevioctl verify contract-closure

# Gate D — Security and Policy Integrity
brevioctl verify policy-closure
brevioctl verify receipt-enforcement
brevioctl verify workspace-rls
brevioctl verify uuidv7

# Gate E — Workflow/Runtime Integrity
brevioctl verify temporal-replay

# Gate F — Build/Test/Verification
brevioctl verify provider-contract-tests
brevioctl verify algorithm-fidelity
```

## LLM Configuration

All LLM and RAG behaviour is controlled via environment variables.

| Variable | Default | Purpose |
|---|---|---|
| `ANTHROPIC_API_KEY` | required | Anthropic provider authentication |
| `OPENAI_API_KEY` | optional | OpenAI fallback provider for all LLM tiers |
| `VOYAGE_API_KEY` | optional | Voyage AI embedding primary (falls back to OpenAI if unset) |
| `LLM_TIMEOUT_SECONDS` | `60` | HTTP timeout in seconds for all LLM provider calls |
| `FEATURE_STREAMING_ENABLED` | `false` | Set `true` to enable SSE streaming for synthesis |
| `MAX_PARALLEL_TOOL_CALLS` | `3` | Temporal tool fan-out concurrency cap (1-10) |
| `BREVIO_OPUS_ENABLED` | `false` | Set `true` to use Claude Opus 4 as orchestrator model |

---

## SLO Definitions

| SLO | Target | Measurement | Window |
|-----|--------|-------------|--------|
| Gateway availability | 99.9% | `job:brevio_gateway_availability:ratio5m` | 5-min rolling |
| Intent classify p99 | < 2s | `job:brevio_classify_latency_p99:5m` | 5-min rolling |
| Tool execution success | ≥ 99% | `job:brevio_tool_success_rate:5m` | 5-min rolling |
| E2E message p95 | < 15s | `job:brevio_e2e_latency_p95:5m` | 5-min rolling |
| Authorization denial rate | < 5% | `job:brevio_authz_denial_rate:5m` | 5-min rolling |
| LLM cost rate | < $50/hr | `job:brevio_llm_cost_rate_usd:1h` | 1-hour rolling |

---

## SLO Breach Response Playbook

### Gateway availability < 99.9% (alert: BrevioGatewayAvailabilityBreach)

1. Check pod health: `kubectl -n brevio get pods -l app.kubernetes.io/component=gateway`
2. If CrashLoopBackOff: `kubectl -n brevio logs -l app.kubernetes.io/component=gateway --previous`
3. If OOMKilled: increase memory limit in `values-production.yaml`, apply with `helm upgrade`
4. Emergency scale-up: `kubectl -n brevio scale deployment brevio-gateway --replicas=10`
5. If recent deploy is the cause: `helm -n brevio rollback brevio`

### Brain classify p99 > 2s (alert: BrevioBrainClassifyLatencyBreach)

1. Check LLM provider status pages: status.anthropic.com / status.openai.com
2. If provider incident: circuit breaker should auto-activate. Check logs for `circuit_state=open`
3. If circuit is not firing: verify `RateLimitedClient` and `CircuitBreaker` are wired in bootstrap
4. Emergency override — force deterministic keyword classification:
   ```
   kubectl -n brevio set env deployment/brevio-brain FORCE_DETERMINISTIC_CLASSIFICATION=true
   ```

### Tool execution failure > 1% (alert: BrevioToolExecutionSuccessRateBreach)

1. Identify failing tools: query `brevio_tool_executions_total{status="error"}` grouped by `tool_key`
2. If Google/Slack/Notion APIs → check provider status pages
3. If OAuth token errors → trigger re-auth for affected workspaces
4. If hands-runtime is down: `kubectl -n brevio rollout restart deployment/brevio-hands`

### High authorization denial rate > 5% (alert: BrevioHighAuthzDenialRate)

1. Query recent denials: `brevio_plan_authorizations_total{decision="deny"}` by reason label
2. If `KILL_SWITCH_ACTIVE`: check if kill switch was triggered accidentally
3. If `OPA_EVAL_ERROR`: OPA module failed to load — check `internal/policy/rego/` files
4. If `SKILL_ACL_DENIED`: workspace ACL may have been incorrectly tightened

### High LLM cost rate (alert: BrevioHighLLMCostRate)

1. Check if spike or sustained: compare 1h vs 24h average
2. Identify high-spend workspaces via `brevio_llm_tokens_total` labels
3. Apply emergency budget cap via admin API
4. Verify budget enforcement policy is active in OPA

---

## Pre-Deploy Checklist

- [ ] `make opa-verify` passes — OPA policies synced and tested
- [ ] `go test -race ./...` passes — zero test failures
- [ ] `go vet ./...` passes — no warnings
- [ ] `npm run validate-registry` passes in `services/hands-runtime/`
- [ ] `helm lint infra/helm/brevio` passes
- [ ] All pending DB migrations reviewed
- [ ] Feature flags for new capabilities set to disabled (default-off)
- [ ] RUNBOOK.md updated if new services or SLOs changed

## Post-Deploy Verification

- [ ] `make smoke BRAIN_URL=https://brain.production.brevio.com` passes
- [ ] Deep health green: `curl https://brain.production.brevio.com/health/deep | jq .status`
- [ ] Prometheus SLO dashboard shows no active alerts
- [ ] First 10 messages processed successfully (check Temporal UI)
- [ ] `/metrics` endpoint returning data
- [ ] No cost anomaly triggered (check 30 min after deploy)

## Incident Severity Levels

| Level | Criteria | Response Time | Examples |
|-------|----------|---------------|---------|
| P1 | Gateway down, >10% error rate | 15 min | Gateway crash, DB pool exhausted |
| P2 | SLO breach > 5 min, data loss risk | 30 min | LLM provider down, OPA eval errors |
| P3 | SLO at risk, degraded performance | 2 hours | Latency elevated, non-critical skills failing |
| P4 | Minor anomaly, no user impact | Next business day | Cost spike, log warnings |
