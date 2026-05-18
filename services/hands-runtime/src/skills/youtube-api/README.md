# youtube-api

YouTube search/transcript/channel adapter.

- Plane: `hands`
- External API target: YouTube Data API v3 + transcript extraction path (production), deterministic simulation (current)
- Auth: API key (server-side)

## Input

- `mode` (`search`, `transcript`, `channel`)
- `query` (optional)
- `video_id` (required for transcript mode)
- `channel_id` (optional for channel mode)

## Output

- `provider`: `youtube`
- `mode`
- `results[]` for search/channel
- `transcript` for transcript mode

## Brevio use case

"Summarize this YouTube video" -> transcript summary routed to the response generator.
