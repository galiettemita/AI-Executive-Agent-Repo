# autoresponder

Gateway-plane hybrid interception skill with Brain delegation metadata.

## Supported actions

- `enable`: enables autoresponder rule mode.
- `disable`: disables autoresponder routing.
- `intercept`: creates immediate response payload and marks Brain delegation behavior.

## Notes

- Encodes hybrid gateway/brain behavior with `delegated_to_brain` flag.
- Enforces gateway latency budget contract (`latency_budget_ms = 8000`).
