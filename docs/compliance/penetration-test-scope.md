# Penetration Test Scope

## In-Scope Assets
- Public ingress endpoints (webhooks, auth callbacks, docs surface).
- Core APIs (Gateway/Brain/Hands/Auth/Profile).
- Edge relay channel and agent onboarding flow.
- OAuth/token lifecycle and secret handling paths.
- RBAC/OPA policy enforcement boundaries.

## In-Scope Test Categories
- Authentication/authorization bypass.
- SSRF, injection, deserialization, and request smuggling.
- Secrets exposure and token leakage.
- Tenant/workspace isolation failures.
- Prompt injection and model abuse routes.

## Out-of-Scope
- Third-party vendor internals.
- Physical security.
- Social engineering unless explicitly contracted.

## Rules of Engagement
- Testing windows and contact escalation list defined pre-engagement.
- No destructive payloads in production without explicit approval.
- Immediate notification on critical findings.

## Deliverables
- Executive summary.
- Technical findings with CVSS severity.
- Proof-of-concept details.
- Remediation recommendations and retest criteria.
