# pocket-casts

Pocket Casts adapter for queue listing, YouTube ingestion, and episode removal.

## Auth
- Pocket Casts account/session in production.

## Input
- `action`: `queue_from_youtube`, `list_queue`, `remove_episode`
- `youtube_url` required for queueing from YouTube
- `episode_id` required for remove action

## Output
- `provider`: `pocket-casts`
- action echo with optional `queue`, `queued`, `removed`
