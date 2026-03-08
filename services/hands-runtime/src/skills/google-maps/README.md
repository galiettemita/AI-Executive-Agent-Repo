# google-maps

Route and travel estimation skill adapter.

- Plane: `hands`
- External API target: Google Maps Routes API
- Auth: API key (server-side)

## Input

- `origin` (required)
- `destination` (required)
- `mode` (`driving|walking|bicycling|transit`, optional)

## Output

- `distance_m`
- `duration_s`
- `steps[]`

## Brevio use case

"How long to the airport?" -> deterministic route estimate and steps.
