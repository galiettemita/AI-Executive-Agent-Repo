# Disaster Recovery Plan

## RTO/RPO Targets
- RTO: 4 hours
- RPO: 1 hour

## Strategy
- Multi-region backups
- Infrastructure-as-code for environment rebuilds
- Runbook for failover
- Detailed operational scenarios: `docs/DR_OPERATIONAL_RUNBOOKS.md`
- Monthly restore drill automation: `.github/workflows/dr-restore-drill.yml`
