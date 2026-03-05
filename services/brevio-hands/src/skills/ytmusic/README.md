# ytmusic

YouTube Music adapter for search, playback, and queueing actions.

## Auth
- Uses account/session auth in production; deterministic fixture output in repository.

## Input
- `action`: `search`, `play`, `queue`
- `query` for search
- `track_id` or `query` for play/queue targeting

## Output
- `provider`: `ytmusic`
- action echo plus optional `tracks`, `now_playing`, `queued`
