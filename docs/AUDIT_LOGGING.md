# Audit Logging

## What We Log
- Authentication attempts and failures
- Admin actions
- Execution approvals and actions
- Webhook deliveries (status + errors)

## Where It Lives
- `audit_logs` table in Postgres
- Render logs for infrastructure events

## Retention
- 12 months in Postgres (`RETENTION_AUDIT_LOGS_DAYS`)

## Access
- Restricted to operators with admin access
