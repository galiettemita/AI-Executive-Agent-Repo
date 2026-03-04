# withings-health

Hands-plane health metrics adapter for Withings device measurements.

## Supported actions

- `get_measurements`
- `trend_summary`

## Notes

- Requires Withings metric scopes (`user.metrics`, `user.activity`).
- Returns deterministic fixture measurements for contract-safe validation.
