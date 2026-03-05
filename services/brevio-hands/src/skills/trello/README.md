# trello

Trello card management adapter.

- Plane: `hands`
- External API target: Trello REST API
- Auth: API key + token

## Input

- `action` (`card_list`, `card_create`, `card_move`)
- board/list/card fields and destination list for moves

## Output

- `provider`: `trello`
- action-specific `card_id` and `cards[]`

## Brevio use case

"Move this card to Done" and board task automation flows.
