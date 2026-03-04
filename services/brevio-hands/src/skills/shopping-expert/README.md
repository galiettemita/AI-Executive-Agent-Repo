# shopping-expert

Shopping skill adapter for ranked product discovery.

- Plane: `hands`
- External API target: Google Custom Search JSON API (production), deterministic local catalog fallback (current)
- Auth: API key (server-side)

## Input

- `query` (required)
- `max_price` (optional)
- `category` (optional)
- `limit` (optional, max `20`)

## Output

- `results[]` containing `title`, `price`, `url`, `rating`, `store`

## Brevio use case

"Find me running shoes under $100" -> ranked options with links and scores.
