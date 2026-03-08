# pharmacy-prescription

Custom-build adapter for medication lookup and refill workflows.

## Auth
- Pending pharmacy API partnership integration.

## Input
- `action`: `medication_lookup`, `refill_request`, `refill_status`
- lookup requires `medication_name`
- refill request requires `prescription_id`, `pharmacy_name`, and `confirmed: true`

## Output
- `provider`: `pharmacy-prescription`
- action echo with medication/refill status fields
- `partnership_status`: `awaiting_api_partnership`

## Notes
- `// CUSTOM_BUILD_REQUIRED: Awaiting API partnership` enforced in adapter runtime.
