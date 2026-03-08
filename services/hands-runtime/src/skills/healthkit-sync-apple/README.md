# healthkit-sync-apple

Hands-plane canonical Apple HealthKit sync adapter.

## Supported actions

- `sync_steps`
- `sync_sleep`
- `sync_heart_rate`
- `sync_all`

## Notes

- Canonical skill for HealthKit routing (preferred over `healthkit-sync`).
- Requires `healthkit.read` and enforces explicit time-range/window input.
