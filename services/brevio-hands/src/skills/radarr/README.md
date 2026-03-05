# radarr

Hands-plane Radarr adapter for movie search, add, and queue introspection.

## Supported actions

- `search_movie`
- `add_movie`
- `list_queue`

## Notes

- Uses deterministic movie queue fixtures for CI stability.
- Enforces required query/TMDB fields for relevant actions.
