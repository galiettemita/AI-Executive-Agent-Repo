# Backup + Disaster Recovery Drill

## Goal
Validate RPO/RTO and recovery procedure.

## Steps
1. Trigger a backup snapshot (Postgres + object storage).
2. Restore to a new staging instance.
3. Run smoke tests (`/health/ready`, key API endpoints).
4. Record restore time (RTO) and data loss window (RPO).

## Evidence
- Screenshot of backup and restore
- RPO/RTO measurement notes
