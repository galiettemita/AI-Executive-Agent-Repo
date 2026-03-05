# chromecast

Hands-plane Chromecast control adapter for discovery and media playback.

## Supported actions

- `discover_devices`
- `cast_media`
- `pause`
- `resume`
- `stop`
- `status`

## Notes

- Validates required device/media fields for cast and control operations.
- Returns deterministic local device fixtures for CI stability.
