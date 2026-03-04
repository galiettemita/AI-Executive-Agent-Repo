# things-mac

Hands-plane local Things 3 adapter for task lifecycle operations.

## Supported actions

- `create_todo`
- `list_today`
- `complete_todo`
- `move_to_project`

## Notes

- Models deterministic Things-style task payloads for CI stability.
- Enforces required task/project IDs for mutation actions.
