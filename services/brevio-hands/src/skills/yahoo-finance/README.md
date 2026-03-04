# yahoo-finance

Yahoo Finance adapter for quotes, fundamentals, and market news.

## Auth
- Public market data access (no OAuth required in this adapter).

## Input
- `action`: `quotes`, `fundamentals`, `news`
- `symbols` required for `quotes` and `fundamentals`

## Output
- `provider`: `yahoo-finance`
- action echo with optional `quotes`, `fundamentals`, `news`
- `disclaimer`: always includes not-financial-advice notice
