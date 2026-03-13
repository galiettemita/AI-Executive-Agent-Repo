# todoist

Task management adapter for Todoist action workflows.

- Plane: `hands`
- External API target: Todoist REST API v2 (production), deterministic local simulation (current)
- Auth: OAuth2 scopes `task:add`, `data:read`

## Input

- `action` (`list`, `create`, `complete`, `delete`)
- `project_id` (optional)
- `task` (optional object with `content`, `due_string`, `priority`, `task_id`)

## Output

- `provider`: `todoist_deterministic`
- `action`
- `task_id` (for mutation actions)
- `tasks[]` for list/create/complete responses

## Brevio use case

"Add finish board memo to my work tasks" -> creates structured Todoist task metadata.
