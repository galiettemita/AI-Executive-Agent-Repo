# apple-mail

Apple Mail adapter for inbox read/search and confirmation-gated send/reply actions.

## Auth
- Local macOS Apple Mail permissions.

## Input
- `action`: `list_inbox`, `search`, `send`, `reply`
- `query` for search
- `to`, `subject`, `body` for send
- `reply_to_id`, `body` for reply
- `confirmed` required for send/reply mutations

## Output
- `provider`: `apple-mail-local`
- action echo plus optional `emails[]`, `sent`, and `message_id`

## Notes
- Send/reply are blocked unless explicit confirmation is present.
