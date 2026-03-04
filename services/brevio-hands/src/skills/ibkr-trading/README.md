# ibkr-trading

Hands-plane adapter for IBKR quote/order/status workflows.

## Supported actions

- `quote_symbol`
- `place_order`
- `order_status`

## Notes

- Enforces action-specific symbol/order/confirmation requirements.
- Returns deterministic quote/order payloads in CI/local runs.
