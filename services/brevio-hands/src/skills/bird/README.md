# bird

X/Twitter-style adapter for timeline/search and confirmation-gated posting.

## Auth
- API key/app auth in production (deterministic local adapter in repository).

## Input
- `action`: `timeline`, `search`, `post`
- `query` for search
- `text` + `confirmed` for post

## Output
- `provider`: `bird`
- action echo plus optional `posts[]`, `posted`, `post_id`

## Notes
- Posting requires explicit confirmation.
