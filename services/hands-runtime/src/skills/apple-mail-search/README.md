# apple-mail-search

Hands-plane low-latency Apple Mail indexed search adapter.

## Supported actions

- `search_all`
- `search_sender`
- `search_subject`

## Notes

- Contract encodes 50ms index-based latency profile.
- Deterministic result fixtures for stable regression tests.
