# pet-care

Custom-build adapter for pet provider discovery and visit booking workflows.

## Auth
- Pending partner API integrations (Rover/Wag style providers).

## Input
- `action`: `providers`, `book_visit`, `booking_status`
- provider search requires `pet_type` + `service_type`
- `book_visit` requires `provider_id` and `confirmed: true`

## Output
- `provider`: `pet-care`
- action echo with provider/booking fields
- `partnership_status`: `awaiting_api_partnership`

## Notes
- `// CUSTOM_BUILD_REQUIRED: Awaiting API partnership` enforced in adapter runtime.
