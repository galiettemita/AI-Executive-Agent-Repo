# OPERATIONS OWNERSHIP

## Plane Ownership Matrix
| Plane | Primary Owner | Secondary Owner | Scope |
|---|---|---|---|
| Gateway | Platform Runtime | Messaging Integrations | Webhooks, ingress auth, outbound dispatch |
| Brain | AI Runtime | Applied AI | Planning, routing, tool selection, response synthesis |
| Hands/Executor | Integrations Runtime | Security Engineering | Tool execution, idempotency, side-effect controls |
| Data | Data Platform | Security Engineering | Postgres/Redis/S3 schemas, retention, RLS posture |
| Infra | Platform Infrastructure | SRE | Terraform/EKS/Helm, cluster/network capacity |
| Security | Security Engineering | Platform Runtime | Guardrails, IAM, key management, incident triage |
| Observability | SRE | Platform Runtime | Telemetry, SLOs, dashboards, alerts and paging |

## Connector Ownership Matrix
| Connector Group | Primary Owner | Backup Owner |
|---|---|---|
| Google (Calendar/Drive/Gmail/Sheets) | Integrations Runtime | Platform Runtime |
| Microsoft (Outlook/Teams) | Integrations Runtime | Platform Runtime |
| Slack | Integrations Runtime | Messaging Integrations |
| Apple (iMessage/Reminders/Health) | Messaging Integrations | Integrations Runtime |
| Tavily/Web Search | Applied AI | Integrations Runtime |
| Plaid/Financial | Financial Integrations | Security Engineering |
| MCP Platform | Integrations Runtime | Platform Infrastructure |

## On-Call Rotation And Escalation Policy

### Rotations
- Primary on-call: weekly rotation, Monday 00:00 UTC handoff.
- Secondary on-call: weekly rotation, staggered one week from primary.
- Incident commander: assigned by primary for `P0` and `P1` events.

### Severity Levels
- `P0`: active security breach, data integrity corruption, or complete platform outage.
- `P1`: major customer impact, core workflow unavailable, SLA breach in progress.
- `P2`: degraded functionality with workaround, no active data-loss signal.
- `P3`: non-critical defect, low-impact operational issue.

### Escalation Timers
- `P0`: page immediately, incident bridge within 5 minutes.
- `P1`: page immediately, incident bridge within 15 minutes.
- `P2`: triage within 60 minutes, escalate if unresolved after 4 hours.
- `P3`: business-hours triage, include in next sprint planning.

### Escalation Chain
1. Primary on-call.
2. Secondary on-call.
3. Domain owner (plane or connector owner).
4. Engineering leadership and security lead (for `P0`/`P1`).
