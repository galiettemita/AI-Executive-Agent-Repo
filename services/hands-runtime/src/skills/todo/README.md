# todo

Generic task list adapter for local task operations.

- Plane: `hands`
- External API target: local task store / task bridges (provider-specific)
- Auth: none in local simulation

## Input

- `action` (`list`, `add`, `complete`, `delete`)
- `content`/`due` for add
- `item_id` for complete/delete

## Output

- `provider`: `todo`
- action-specific `items[]` and `item_id`

## Brevio use case

"Add this to my tasks" and quick task mutations when no provider preference is set.
