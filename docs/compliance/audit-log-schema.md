# Audit Log Schema

## Purpose
Define immutable audit events for all sensitive state changes and operational control points.

## Canonical Fields
- `id` (UUID)
- `timestamp` (UTC ISO8601)
- `actor_type` (`user|service|admin|system`)
- `actor_id`
- `workspace_id`
- `action` (normalized verb, e.g., `skill.execute`, `oauth.refresh`)
- `resource_type`
- `resource_id`
- `before_state` (JSON, optional)
- `after_state` (JSON, optional)
- `trace_id`
- `outcome` (`success|failure|denied`)
- `error_code` (optional)

## Retention and Integrity
- Append-only writes; no in-place updates.
- Hot retention: 90 days.
- Warm archive: 12 months.
- Long-term archive: encrypted object storage.
- Hash-chain or signed batch digest for tamper detection.

## Access Control
- Read access limited to security/ops/admin roles.
- Query access is logged and reviewed.
- PII minimization applied in `before_state`/`after_state`.

## Query Patterns
- Incident reconstruction by `trace_id` and time range.
- Actor investigation by `actor_id`.
- Resource timeline by `resource_id`.
- Compliance evidence exports by control/domain.
