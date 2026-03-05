# financial-market-analysis

Market analysis adapter for sentiment, volatility, and correlation summaries.

## Auth
- Uses public market data/derived metrics in this deterministic implementation.

## Input
- `action`: `sentiment`, `volatility`, `correlation`
- `symbols` required (1-10)

## Output
- `provider`: `financial-market-analysis`
- action echo with either `metrics` or a `correlation` matrix
