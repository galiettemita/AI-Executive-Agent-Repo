# Business Continuity

## Objectives
Maintain critical service operation during outages and recover within target RTO/RPO.

## Targets
- PostgreSQL: RPO < 1 min, RTO < 5 min.
- Redis: RPO < 5 min, RTO < 2 min.
- Application tier: RTO < 2 min.
- Regional failover: RTO < 30 min.

## Continuity Strategy
- Multi-AZ primary region deployment.
- Cross-region backups and replication.
- DNS health-check failover to DR region.
- Runbook-driven failover and failback steps.

## Annual Test Program
- Quarterly restore drills.
- Semi-annual regional failover simulation.
- Evidence capture for each exercise.

## Crisis Roles
- Incident Commander
- Platform Lead
- Data Lead
- Communications Lead
