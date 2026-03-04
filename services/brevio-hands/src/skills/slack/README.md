# slack

Slack collaboration adapter for channel listing, posting, and reactions.

## Auth
- OAuth bot scopes: `channels:read`, `chat:write`, `reactions:write`, `users:read`.

## Input
- `action`: `list_channels`, `post_message`, `add_reaction`
- post: `channel_id`, `text`
- reaction: `channel_id`, `message_ts`, `emoji`

## Output
- `provider`: `slack`
- action echo plus optional `channels[]`, `post`, or `reacted`

## Notes
- Action schemas enforce required field sets per Slack operation.
