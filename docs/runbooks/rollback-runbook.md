# Rollback Runbook

## Purpose
Restore stable production state within 30 seconds when canary or full rollout degrades service.

## Rollback Triggers
- Error rate > 1% for 5 minutes.
- P99 latency > 2x baseline for 5 minutes.
- Any P1 incident during deployment.
- Health checks failing on core services.

## Procedure
1. Declare rollback in incident channel.
2. Stop further promotions and freeze new deploys.
3. Execute rollback.
   - ArgoCD: `argocd app rollback <app> <revision>`
   - Helm fallback (if needed): `helm rollback <release> <revision>`
4. Route traffic back to blue.
5. Verify service recovery.
   - Check `/health` and `/health/deep` on all core services.
   - Confirm webhook ingress and message delivery path.
6. Assess database impact.
   - If migrations were applied and compatible, keep schema.
   - If required, execute tested down migration in maintenance window.
7. Publish status update.
   - Incident timeline, rollback revision, current status.

## Post-Rollback
- Open corrective action ticket.
- Capture root cause hypotheses within 2 hours.
- Schedule blameless RCA within 48 hours.
