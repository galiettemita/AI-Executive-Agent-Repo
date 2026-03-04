# serpapi

Unified multi-engine search adapter.

- Plane: `hands`
- External API target: SerpAPI
- Auth: API key

## Input

- `query` (required)
- `engine` (`google`, `amazon`, `yelp`)
- `max_results`

## Output

- `provider`: `serpapi`
- `engine`
- `results[]`

## Brevio use case

"Search products and reviews across platforms" with one connector.
