# trakt

Trakt adapter for watch history, trending items, and watched-state updates.

## Auth
- Trakt client credentials/session token in production.

## Input
- `action`: `history`, `trending`, `mark_watched`
- `media_id` required for `mark_watched`
- optional `media_type` (`movie` or `show`)

## Output
- `provider`: `trakt`
- action echo with optional `items` and `marked_watched`
