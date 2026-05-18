# plex

Plex adapter for media search, recent listings, and playback control.

## Auth
- Plex auth/session token in production.

## Input
- `action`: `search`, `play`, `recent`
- search: `query`
- play: `media_id` or `query`

## Output
- `provider`: `plex`
- action echo with optional `results` and `now_playing`
