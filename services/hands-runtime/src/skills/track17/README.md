# track17

Multi-carrier package tracking adapter for 17TRACK style event feeds.

## Auth
- API key in production (deterministic stub output in local testing).

## Input
- `tracking_number` required
- `carrier_code` optional carrier hint
- `request_locale` optional localization

## Output
- `provider`: `17track`
- `tracking_number`, `carrier`, `status`
- `checkpoints[]` scan history timeline

## Notes
- Used in package disambiguation for 17TRACK-supported carriers.
