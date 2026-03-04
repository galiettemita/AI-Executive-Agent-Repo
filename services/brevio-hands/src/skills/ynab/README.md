# ynab

Budget data adapter for YNAB account and transaction workflows.

- Plane: `hands`
- External API target: YNAB API v1 (production), deterministic simulation (current)
- Auth: OAuth2 (`read-only`/`read-write` in production)

## Input

- `action` (`summary`, `accounts`, `transactions`)
- `budget_id` (optional)
- `account_id` (optional, for transaction filtering)

## Output

- `provider`: `ynab`
- `budget_id`
- `total_budget_cents` (summary)
- `accounts[]` (accounts action)
- `transactions[]` (transactions action)

## Brevio use case

"How much is left in my budget this month?" -> summary/accounts/transactions context for response generation.
