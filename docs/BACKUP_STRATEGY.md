# Backup Strategy

## Databases
- PostgreSQL: daily full backups + hourly WAL archiving

## Object Storage
- Versioned buckets with lifecycle policies
- Critical artifacts retained for 365 days

## Verification
- Monthly restore drill in staging via `.github/workflows/dr-restore-drill.yml`
- Operational playbooks: `docs/DR_OPERATIONAL_RUNBOOKS.md`
