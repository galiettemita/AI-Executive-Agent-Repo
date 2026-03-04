# goplaces

Google Places style search adapter with optional location and open-now filters.

## Auth
- OAuth / API key in production (`google.places.read` scope represented in adapter metadata).

## Input
- `query` required search text
- `location` optional `{ lat, lng, radius_m }`
- `open_now` optional open-state filter
- `max_results` optional cap (1-25)

## Output
- `provider`: `goplaces`
- `results[]`: place IDs, names, addresses, and optional rating/open state

## Notes
- Primary resolver for "find near me" routing in location disambiguation.
