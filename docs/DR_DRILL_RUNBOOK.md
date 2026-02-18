# Backup + Disaster Recovery Drill

## Goal
Validate that we can restore from snapshot and meet RTO/RPO targets.

## Automation
- Workflow: `.github/workflows/dr-restore-drill.yml`
- Script: `scripts/dr_restore_drill.py`
- Default schedule: monthly (day 1, 08:00 UTC)

## Required Variables (GitHub Environment: `staging`)
- `DR_REGION`
- `DR_SOURCE_DB_INSTANCE_ID`
- `DR_RESTORE_DB_SUBNET_GROUP`
- `DR_RESTORE_VPC_SECURITY_GROUP_IDS` (comma-separated)
- Optional: `DR_RESTORE_DB_INSTANCE_CLASS`, `DR_RESTORE_ID_PREFIX`, `DR_WAIT_TIMEOUT_SECONDS`

## Manual Steps
1. Trigger `DR Restore Drill` workflow from GitHub Actions (or let schedule run).
2. Use `execute=true` for real restore, `cleanup=true` to remove temporary restored instance afterward.
3. Confirm workflow output includes snapshot ID, restored instance ID, and availability confirmation.
4. Run smoke tests:
- `/health`
- `/internal/health/deep`
- `/api/v1/message` (basic request path)
5. Record incident template fields:
- Drill date/time
- RTO (restore start -> available + smoke complete)
- RPO (now - snapshot timestamp)
- Follow-up actions

## Related Runbooks
- `docs/DR_OPERATIONAL_RUNBOOKS.md`
