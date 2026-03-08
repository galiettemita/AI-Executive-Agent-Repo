# overseerr

Hands-plane Overseerr adapter for searching and requesting media.

## Supported actions

- `search_media`
- `request_media`
- `list_requests`

## Notes

- Uses deterministic request fixtures for CI-safe behavior.
- Enforces required query/media fields for action-specific flows.
