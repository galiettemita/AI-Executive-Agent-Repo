# spots

Exhaustive area scanning adapter for dense places discovery.

## Auth
- Google Places credentials in production (`google.places.read`).

## Input
- `query` required search text
- `area` optional area descriptor
- `grid_density` optional (`low`, `medium`, `high`)
- `max_results` optional cap (1-200)

## Output
- `provider`: `spots`
- `grid_density` applied scan profile
- `results[]` with geocoded place entries

## Notes
- Used for disambiguation cases where users ask to "find all" matches in an area.
