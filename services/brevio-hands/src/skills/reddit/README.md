# reddit

Reddit adapter for search, hot-post listing, and confirmation-gated posting.

## Auth
- OAuth scopes: `read`, `submit`, `identity`.

## Input
- `action`: `search`, `list_hot`, `post`
- `query` required for search
- `subreddit`, `title`, `text`, `confirmed` required for post

## Output
- `provider`: `reddit`
- action echo plus optional `posts[]`, `submitted`, `post_id`

## Notes
- Posting is denied unless explicit confirmation is provided.
