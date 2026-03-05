# firecrawl-search

Web search + crawl adapter for richer content extraction.

- Plane: `hands`
- External API target: Firecrawl API
- Auth: API key

## Input

- `query` (required)
- `max_results` (optional)

## Output

- `provider`: `firecrawl`
- `results[]` including crawled content snippets

## Brevio use case

"Find and summarize reviews" with crawl-ready content for downstream synthesis.
