# lastfm

Last.fm adapter for recent tracks, top tracks, and artist summary insights.

## Auth
- API key/session in production.

## Input
- `action`: `recent_tracks`, `top_tracks`, `artist_summary`
- `username` required for recent/top actions
- `artist` required for artist summary

## Output
- `provider`: `lastfm`
- action echo with optional `tracks` or `artist_summary`
