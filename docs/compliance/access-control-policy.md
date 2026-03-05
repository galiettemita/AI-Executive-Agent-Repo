# Access Control Policy

## Role Definitions
- `free`, `pro`, `enterprise`: end-user service tiers.
- `admin`: platform administration.
- `service`: inter-service machine identity.

## Principles
- Least privilege by default.
- Explicit deny for unsupported role/resource operations.
- Separation of duties for high-risk changes.

## Enforcement
- Runtime authorization via OPA (`policies/brevio/authz.rego`).
- Scoped OAuth token access controls.
- Tier-based rate/budget controls.
- Audit logging for all privileged operations.

## Access Lifecycle
1. Provision access based on role and need.
2. Time-bound elevated permissions.
3. Quarterly access reviews.
4. Immediate revocation on offboarding/security events.

## Credential Rotation Cadence
- API secrets: 90 days.
- OAuth client secrets: 90 days.
- mTLS certificates: daily automated rotation.
