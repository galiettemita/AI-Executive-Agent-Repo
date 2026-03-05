# flight-tracker

Tracks flight status and live movement summaries using OpenSky-compatible responses.

## Auth
- None for this deterministic adapter implementation.

## Input
- `callsign` optional flight callsign (e.g. `AAL100`)
- `icao24` optional transponder identifier (6-char hex)
- `origin_iata` optional airport filter
- `destination_iata` optional airport filter

At least one identifier is required.

## Output
- `provider`: `opensky`
- `flights[]`: matched flights with origin/destination, altitude, speed, and status
- `queried_at_utc`: response timestamp

## Notes
- Supports disambiguation routing for track-flight intents in the Brain plane.
