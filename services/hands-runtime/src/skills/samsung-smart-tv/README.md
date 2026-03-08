# samsung-smart-tv

Hands-plane SmartThings-backed Samsung TV control adapter.

## Supported actions

- `power_on`
- `power_off`
- `launch_app`
- `set_volume`
- `status`

## Notes

- Uses SmartThings OAuth scopes `r:devices:*` and `x:devices:*`.
- Enforces required app and volume fields for mutation actions.
