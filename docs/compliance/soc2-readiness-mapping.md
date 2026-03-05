# SOC 2 Readiness Mapping

## Trust Service Criteria Coverage
- Security (CC series): access controls, encryption, secure SDLC, vulnerability management.
- Availability (A series): SLOs, incident response, DR testing, capacity management.
- Confidentiality (C series): data classification, retention, restricted access, encryption.
- Processing Integrity (PI series): deterministic workflows, contract tests, idempotency controls.
- Privacy (P series): GDPR rights handling, data minimization, deletion/export procedures.

## Control-to-Evidence Mapping
| Control Area | Primary Evidence | Collection Frequency |
|---|---|---|
| Access control enforcement | OPA policy tests + access review records | Per release + quarterly |
| Change management | PR approvals, CI logs, deployment records | Per change |
| Vulnerability management | Trivy/SAST results, remediation tickets | Per PR + weekly |
| Incident response | PagerDuty timelines, RCA docs | Per incident |
| Backup/restore and DR | Drill reports, restore logs | Quarterly |
| Data subject rights | Request tickets + completion artifacts | Per request |

## Evidence Collection Procedure
1. Export CI/security artifacts for each release.
2. Snapshot policy test and migration validation reports.
3. Archive deployment/change tickets with approvals.
4. Store incident and DR drill documentation in immutable evidence storage.
5. Perform monthly completeness audit of evidence inventory.
