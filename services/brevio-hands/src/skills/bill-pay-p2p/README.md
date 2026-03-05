# bill-pay-p2p

Custom-build adapter for payee management and payment scheduling workflows.

## Auth
- Pending partner API integration (Plaid + payment rails). No live money movement API yet.

## Input
- `action`: `list_payees`, `create_payment`, `payment_status`, `cancel_payment`
- `create_payment` requires `payee_id`, `amount_cents`, and `confirmed: true`
- `payment_status`/`cancel_payment` require `payment_id`

## Output
- `provider`: `bill-pay-p2p`
- action echo with payee/payment fields
- `partnership_status`: `awaiting_api_partnership`

## Notes
- `// CUSTOM_BUILD_REQUIRED: Awaiting API partnership` enforced in adapter runtime.
