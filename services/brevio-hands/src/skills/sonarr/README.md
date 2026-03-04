# sonarr

Hands-plane Sonarr adapter for series search, add, and queue introspection.

## Supported actions

- `search_series`
- `add_series`
- `list_queue`

## Notes

- Uses deterministic series queue fixtures for CI stability.
- Enforces required query/TVDB fields for relevant actions.
