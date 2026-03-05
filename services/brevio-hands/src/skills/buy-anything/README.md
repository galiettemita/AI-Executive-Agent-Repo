# buy-anything

Hands-plane shopping execution adapter for Amazon/Rye style checkout flows.

## Supported actions

- `search_product`
- `prepare_checkout`
- `place_order` (confirmation required)
- `order_status`

## Notes

- Mutating order placement is blocked unless `confirmed=true`.
- Uses deterministic fixture payloads pending live partner credentials.
