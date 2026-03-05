# asana

Asana task management adapter.

- Plane: `hands`
- External API target: Asana REST API
- Auth: OAuth/API token

## Input

- `action` (`task_list`, `task_create`, `task_update`)
- task metadata fields (`project_id`, `task_id`, `name`, `notes`, `status`)

## Output

- `provider`: `asana`
- action-specific `task_id` and `tasks[]`

## Brevio use case

"Create an Asana task for this follow-up" with structured project routing.
