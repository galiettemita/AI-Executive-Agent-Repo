# Change Management

## Goals
Deliver changes safely with traceability, approval controls, and rollback readiness.

## Change Classes
- Standard: low-risk repeatable changes.
- Normal: reviewed changes requiring explicit approval.
- Emergency: incident-driven changes with post-hoc review.

## Required Gates
1. Peer review.
2. CI pass (lint/tests/contracts/security).
3. Migration safety checks for DB changes.
4. Deployment approval for production.

## Deployment Controls
- Blue/green with canary progression.
- Feature flags for risky paths.
- Automated rollback criteria tied to SLOs.

## Communication Plan
- Pre-deploy notice in engineering and ops channels.
- Live updates during canary.
- Post-deploy summary with metrics and incident notes.
