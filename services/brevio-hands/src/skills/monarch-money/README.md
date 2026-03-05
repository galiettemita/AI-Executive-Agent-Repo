# monarch-money

Monarch Money adapter for account, transaction, and budget summaries.

## Auth
- Monarch session/API token in production.

## Input
- `action`: `accounts`, `transactions`, `budgets`
- `account_id` required for transaction lookup
- `month` (`YYYY-MM`) required for budget lookup

## Output
- `provider`: `monarch-money`
- action echo with optional `accounts`, `transactions`, `budgets`
