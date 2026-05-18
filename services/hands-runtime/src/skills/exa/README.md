# exa

Neural web-search adapter for high-relevance research queries.

- Plane: `hands`
- External API target: Exa API
- Auth: API key

## Input

- `query` (required)
- `max_results` (optional)
- `include_domains` (optional)

## Output

- `provider`: `exa`
- `results[]` with title, URL, snippet, score

## Brevio use case

"Research executive planning frameworks" -> semantic results prioritized by relevance.
