# spotify-web-api

Spotify playback/search/history adapter.

- Plane: `hands`
- External API target: Spotify Web API
- Auth: OAuth2 playback/history scopes

## Input

- `action` (`playback`, `search`, `history`, `top_tracks`)
- `query` (optional for `search`)

## Output

- `provider`: `spotify-web-api`
- `playing` snapshot for playback action
- `results[]` for search/history/top tracks

## Brevio use case

"Play my focus playlist" or "What have I listened to this month?" routed by disambiguation rules.
