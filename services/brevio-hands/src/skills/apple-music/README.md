# apple-music

Apple Music adapter for search, playback, and playlist update actions.

## Auth
- Apple Music auth/token scopes for read + modify operations.

## Input
- `action`: `search`, `play`, `add_to_playlist`
- search: `query`
- play: `track_id` or `query`
- add_to_playlist: `track_id` + `playlist_id`

## Output
- `provider`: `apple-music`
- action echo with optional `tracks`, `now_playing`, `playlist_updated`
