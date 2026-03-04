# local-places

Nearby place search adapter optimized for quick local lookups.

## Auth
- Google Places credentials in production (`google.places.read` scope).

## Input
- `query` required search text
- `near` optional free-text area hint
- `radius_km` optional numeric radius filter
- `max_results` optional cap

## Output
- `provider`: `local-places`
- `results[]`: name, address, category, distance in km

## Notes
- Supports low-latency "find near me" requests when exhaustive search is unnecessary.
