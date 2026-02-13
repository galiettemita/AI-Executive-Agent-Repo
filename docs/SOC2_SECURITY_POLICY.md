# SOC 2 Security Policy

## Purpose
Define baseline security requirements to protect customer data and maintain system integrity.

## Scope
All production systems, infrastructure, and integrations used by the Executive AI Agent.

## Policy
- Enforce MFA for all admin and infrastructure accounts.
- Apply least-privilege access to users, services, and tokens.
- Require code review and CI checks for all production changes.
- Log and monitor authentication, admin, and sensitive operations.
- Encrypt data in transit and at rest; use managed keys where possible.
- Rotate secrets at least every 90 days and immediately after incidents.
- Validate and sanitize all external inputs; encode outputs for HTML.
- Maintain incident response procedures and postmortems.
- Review this policy annually or after material changes.

## Ownership
Security Owner is responsible for policy upkeep and compliance.
