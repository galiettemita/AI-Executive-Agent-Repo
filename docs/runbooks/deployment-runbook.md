# Deployment Runbook

## Purpose
Deploy Brevio services with blue/green + canary while preserving SLOs and fast rollback.

## Preconditions
- Main branch CI passed (lint, tests, contracts, security, migration checks).
- Staging deployment healthy for at least 15 minutes.
- Release tag created and images signed.
- PagerDuty on-call acknowledged deployment window.

## Inputs
- `RELEASE_SHA`
- Target environment (`staging` or `production`)
- Helm values file and image tags

## Procedure
1. Validate release artifacts.
   - Verify image signatures and SBOM presence.
   - Confirm migration plan is additive and reversible.
2. Deploy to staging.
   - Run `helm upgrade --install` for all Brevio charts.
   - Wait for rollouts: `kubectl rollout status` for each deployment.
   - Execute smoke tests (`/health`, `/health/deep`, webhook path, one synthetic workflow).
   - Run `bash scripts/deploy/run_staging_smoke_tests.sh` and archive `artifacts/deploy/staging_smoke_test_report.json`.
3. Request production approval.
   - Confirm no P1/P2 alerts in last 30 minutes.
   - Post deployment summary in `#deployments`.
4. Deploy green in production.
   - Install/upgrade green release with no traffic cut.
   - Verify green pods healthy and ready.
5. Start canary.
   - Route 10% traffic to green.
   - Observe for 15 minutes.
   - Watch error rate, P99 latency, queue backlog, circuit breaker opens.
   - Run `CANARY_ERROR_RATE_PCT=<value> CANARY_P99_RATIO=<value> make production-canary-check`.
6. Promote to 100%.
   - Shift all traffic to green.
   - Drain blue and keep rollback window open for 30 minutes.
7. Close deployment.
   - Record deployment metadata in change ticket.
   - Archive dashboards/screenshots and alert timeline.

## Success Criteria
- All service health checks return 200.
- Error rate <= 1% and P99 <= 2x baseline.
- No unresolved P1/P2 incidents.

## Failure Handling
- Trigger rollback immediately if SLO breach threshold is crossed.
- Follow `docs/runbooks/rollback-runbook.md`.
