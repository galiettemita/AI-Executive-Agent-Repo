# copilot-money

Copilot Money adapter for accounts, transactions, and net-worth summaries.

## Auth
- OAuth/session token in production with account and transaction read scopes.

## Input
- `action`: `accounts`, `transactions`, `net_worth`
- `account_id` required for `transactions`
- optional date filters `from_date`, `to_date`

## Output
- `provider`: `copilot-money`
- action echo plus `accounts`, `transactions`, or `net_worth_cents`
