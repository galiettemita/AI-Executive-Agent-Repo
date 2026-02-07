# Backup Strategy

## Databases
- PostgreSQL: daily full backups + hourly WAL archiving
- MongoDB: daily snapshots

## Object Storage
- Versioned buckets with lifecycle policies
- Critical artifacts retained for 365 days

## Verification
- Monthly restore drill in staging
