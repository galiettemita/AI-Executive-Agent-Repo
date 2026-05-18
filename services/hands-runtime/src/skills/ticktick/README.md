# ticktick

Hands-plane TickTick task adapter with typed task CRUD-like actions.

## Supported actions

- `add_task`
- `list_tasks`
- `complete_task`
- `delete_task`

## Notes

- Uses OAuth scopes `tasks:write` and `tasks:read`.
- Enforces required task content or task ID for mutations.
