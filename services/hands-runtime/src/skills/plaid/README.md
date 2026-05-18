# plaid

Plaid account/transaction/balance adapter.

- Plane: `hands`
- External API target: Plaid API
- Auth: Plaid Link + server credentials

## Input

- `action` (`accounts`, `transactions`, `balance`)
- `account_id` (optional)

## Output

- `provider`: `plaid`
- `accounts[]`, `transactions[]`, `balances[]` depending on action

## Brevio use case

"Show my latest transactions" and "What is my account balance?" with structured finance outputs.
