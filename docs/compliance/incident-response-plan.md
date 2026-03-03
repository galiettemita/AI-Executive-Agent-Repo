# Incident Response Plan

## Objective
Detect, contain, eradicate, and recover from security or availability incidents with auditable communication.

## Incident Classes
- Security incident (credential compromise, exfiltration, abuse).
- Availability incident (outage, severe degradation).
- Data integrity incident (corruption, inconsistent state).

## Escalation Matrix
- P1: Page on-call + phone bridge, executive notification.
- P2: Page on-call + Slack incident channel.
- P3/P4: Slack + ticket workflow.

## Response Lifecycle
1. Detect and classify.
2. Contain impact.
3. Preserve evidence.
4. Eradicate root cause.
5. Recover service.
6. Post-incident review and corrective actions.

## Communication Requirements
- First update within 15 minutes for P1/P2.
- Update cadence every 15 minutes while active.
- Final resolution summary and customer communication if needed.

## Evidence Requirements
- Alerts and timeline.
- Query/log snapshots.
- Change history.
- Mitigation actions and approvals.
