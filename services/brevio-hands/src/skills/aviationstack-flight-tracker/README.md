# aviationstack-flight-tracker

Premium flight status tracking adapter with gate, terminal, and delay metadata.

## Auth
- API key (mocked locally in this deterministic implementation).

## Input
- `flight_iata` optional flight number filter (e.g. `AA100`)
- `flight_icao` optional ICAO flight identifier
- `airline_iata` optional airline filter
- `date` optional `YYYY-MM-DD`

At least one flight or airline identifier is required.

## Output
- `provider`: `aviationstack`
- `flights[]`: status, airport, gate/terminal, and delay details
- `queried_at_utc`: response timestamp

## Notes
- Used for premium "track flight" intents in disambiguation group routing.
