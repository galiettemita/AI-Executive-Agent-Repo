# Incident Response Runbook

## Purpose
Standardize response for production incidents with clear severity, ownership, and communication.

## Severity Levels
- `P1`: outage, data loss, security breach. Response < 5 min.
- `P2`: major degradation, SLO burn. Response < 15 min.
- `P3`: partial degradation, single-skill impact. Response < 1 hr.
- `P4`: low impact/perf anomaly. Next business day.

## Immediate Actions
1. Acknowledge alert in PagerDuty.
2. Open incident channel: `#inc-<date>-<id>`.
3. Assign roles.
   - Incident Commander
   - Ops Lead
   - Communications Lead
4. Capture blast radius.
   - Affected channels, services, regions, user tiers.

## Triage Workflow
1. Check dashboards (latency, error rate, queue depth, DB/Redis health).
2. Inspect traces/logs for first failing span and error taxonomy.
3. Validate recent changes (deployments, config, migrations, key rotations).
4. Apply mitigation.
   - rollback
   - traffic reduction
   - feature flag disable
   - failover region activation

## Communication Templates
- Initial: "Investigating incident <id>. Impact: <scope>. Next update in 15 min."
- Mitigation: "Mitigation applied: <action>. Monitoring recovery."
- Resolution: "Incident resolved at <time>. RCA within 48 hours."

## Closure
- Confirm SLO recovery for 30 minutes.
- Close PagerDuty incident.
- Publish timeline and owner-tagged follow-ups.
