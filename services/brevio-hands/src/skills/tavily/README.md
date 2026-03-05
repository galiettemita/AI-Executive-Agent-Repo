# tavily

Web search adapter for concise research retrieval.

- Plane: `hands`
- External API target: Tavily Search API
- Auth: API key

## Input

- `query` (required)
- `max_results` (optional)
- `include_domains` (optional)

## Output

- `results[]` containing `title`, `url`, `content`, `score`

## Brevio use case

Research question -> ranked web-grounded sources for Brain aggregation.
