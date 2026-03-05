# restaurant-reservations

Custom-build adapter for restaurant discovery, holds, and reservation booking.

## Auth
- Pending partner API integration (OpenTable/Resy). No live external calls yet.

## Input
- `action`: `search`, `hold`, `book`, `reservation_status`
- `search`: requires `city`, `date`, `party_size`
- `hold`: requires `restaurant_id`
- `book`: requires `hold_id` and `confirmed: true`

## Output
- `provider`: `restaurant-reservations`
- action echo with options/hold/reservation fields
- `partnership_status`: `awaiting_api_partnership`

## Notes
- `// CUSTOM_BUILD_REQUIRED: Awaiting API partnership` enforced in adapter runtime.
