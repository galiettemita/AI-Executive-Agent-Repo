# Disaster Recovery Operational Runbooks

Owner: Infra + On-call engineer
Scope: Executive OS v4.0 staging and production

## 1) Database Failure (RDS unavailable)
1. Confirm impact:
- Check API `/health` and `/internal/health/deep`.
- Check CloudWatch alarms for RDS and app error rate.
2. Stabilize traffic:
- Pause non-critical workers.
- Keep gateway up for user-facing status messaging.
3. Restore path:
- If Multi-AZ failover already happened, verify DB endpoint recovered.
- If unrecoverable, restore latest automated snapshot to a temporary instance.
4. Validation:
- Run smoke checks: auth, billing, message ingest, message stream.
- Validate latest data freshness against expected RPO.
5. Closeout:
- Record incident timeline, RTO, and RPO.

## 2) Bad Deployment
1. Confirm regression:
- Identify failing build SHA and blast radius.
2. Roll back:
- Redeploy last known-good image tag to ECS services.
3. Verify:
- Run smoke checks (`/health`, `/api/v1/message`, billing endpoints).
4. Prevention:
- Add/adjust regression test and deployment guard.

## 3) Regional Outage
1. Confirm AWS regional degradation (status + alarms).
2. Activate failover plan:
- Restore latest DB snapshot in secondary region.
- Rehydrate required secrets and app config.
- Deploy gateway/brain/hands/workers in standby region.
3. Cut traffic:
- Update DNS/edge routing to standby region.
4. Validate:
- End-to-end message flow, auth, billing, and key connectors.
5. Post-incident:
- Reconcile data and fail back only after stability period.

## 4) Knowledge File Corruption
1. Confirm corruption source (bad write, bad deploy, bad consolidation run).
2. Stop further writes (disable consolidation job temporarily).
3. Recover:
- Restore from S3 object versioning snapshot.
- Validate checksum/version metadata in DB.
4. Verify user-visible behavior:
- Run knowledge retrieval APIs and targeted regression tests.
5. Re-enable jobs after validation.

## 5) Accidental Deletion (data or config)
1. Identify what was deleted and blast radius.
2. Restore from backup/versioned object:
- DB rows from snapshot/PITR.
- S3 objects from previous version.
3. Validate recovered entities with spot checks and smoke tests.
4. Add guardrail:
- Least-privilege IAM, confirmation prompts, or soft-delete gate where allowed.

## Drill Cadence
- Monthly restore drill (automated workflow + manual review).
- Required outputs per drill:
- Start/end time and effective RTO.
- Snapshot timestamp and effective RPO.
- Validation evidence (health checks + API smoke checks).
- Follow-up action items with owners.
