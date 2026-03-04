# tmdb

TMDB recommendation/search adapter.

- Plane: `hands`
- External API target: TMDB API v3
- Auth: API key (server-side)

## Input

- `query` (optional)
- `genre` (optional)
- `type` (`movie` | `tv`, optional)

## Output

- `provider`: `tmdb`
- `results[]` with title/year/rating/overview/streaming

## Brevio use case

"What should I watch tonight?" -> ranked recommendations with streaming availability.
