# bluesky

Bluesky adapter for timeline retrieval, search, and confirmation-gated posting.

## Auth
- AT Protocol app-password/session credentials (stored encrypted via auth service).

## Input
- `action`: `timeline`, `search`, `post`
- `query` for search
- `text` + `confirmed` for post

## Output
- `provider`: `bluesky`
- action echo plus optional `posts[]`, `posted`, and `uri`

## Notes
- Posting requires explicit confirmation to prevent accidental social actions.
