# news-aggregator

Multi-source news briefing adapter.

- Plane: `hands`
- External API target: Aggregated feeds/APIs (HN, Product Hunt, GitHub Trending, etc.)
- Auth: mixed (provider-specific), simulated here

## Input

- `topic` (optional)
- `max_items` (optional)

## Output

- `provider`: `news-aggregator`
- `items[]` with source/title/url

## Brevio use case

"What happened in tech today?" -> concise cross-source briefing list.
