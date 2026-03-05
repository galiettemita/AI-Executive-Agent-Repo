# food-delivery-ordering

Custom-build adapter for food delivery search, cart assembly, and checkout.

## Auth
- Pending partner API integration (DoorDash/Uber Eats). No live checkout API yet.

## Input
- `action`: `search_restaurants`, `build_cart`, `checkout`, `order_status`
- `search_restaurants`: requires `address`
- `build_cart`: requires `restaurant_id` and `items`
- `checkout`: requires `cart_id` and `confirmed: true`

## Output
- `provider`: `food-delivery-ordering`
- action echo with restaurants/cart/order fields
- `partnership_status`: `awaiting_api_partnership`

## Notes
- `// CUSTOM_BUILD_REQUIRED: Awaiting API partnership` enforced in adapter runtime.
