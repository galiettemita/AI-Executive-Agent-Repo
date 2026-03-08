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
