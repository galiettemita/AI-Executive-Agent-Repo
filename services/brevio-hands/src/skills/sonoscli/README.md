# sonoscli

Hands-plane Sonos multi-speaker control adapter.

## Supported actions

- `discover`
- `play`
- `pause`
- `set_volume`
- `group`
- `status`

## Notes

- Enforces required speaker/query fields for active playback actions.
- Returns deterministic zone topology for CI-safe testing.
