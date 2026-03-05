# hotel-vacation-booking

Custom-build adapter for hotel search, room holds, and booking confirmations.

## Auth
- Pending partner API integration (Booking/Amadeus). No live reservation API yet.

## Input
- `action`: `search_hotels`, `hold_room`, `book_room`, `reservation_status`
- search fields required for `search_hotels`: `city`, `check_in`, `check_out`, `guests`
- `hold_room` requires `hotel_id`
- `book_room` requires `hold_id` and `confirmed: true`

## Output
- `provider`: `hotel-vacation-booking`
- action echo with hotels/hold/reservation fields
- `partnership_status`: `awaiting_api_partnership`

## Notes
- `// CUSTOM_BUILD_REQUIRED: Awaiting API partnership` enforced in adapter runtime.
