# ride-hailing

Custom-build adapter for ride estimates, dispatch requests, and ride status tracking.

## Auth
- Pending partner API integration (Uber/Lyft). No live dispatch API yet.

## Input
- `action`: `estimate`, `request_ride`, `ride_status`, `cancel_ride`
- route fields (`origin`, `destination`) required for estimate/request
- `request_ride` requires `confirmed: true`
- `ride_status` and `cancel_ride` require `ride_id`

## Output
- `provider`: `ride-hailing`
- action echo with estimate/ride fields
- `partnership_status`: `awaiting_api_partnership`

## Notes
- `// CUSTOM_BUILD_REQUIRED: Awaiting API partnership` enforced in adapter runtime.
