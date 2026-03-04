# local-service-booking

Custom-build adapter for local provider discovery, quote requests, and service booking.

## Auth
- Pending partner API integration (TaskRabbit/Thumbtack style APIs).

## Input
- `action`: `search_providers`, `request_quote`, `book_service`, `booking_status`
- search requires `service_type` + `zip_code`
- `book_service` requires `provider_id` and `confirmed: true`

## Output
- `provider`: `local-service-booking`
- action echo with provider and booking fields
- `partnership_status`: `awaiting_api_partnership`

## Notes
- `// CUSTOM_BUILD_REQUIRED: Awaiting API partnership` enforced in adapter runtime.
