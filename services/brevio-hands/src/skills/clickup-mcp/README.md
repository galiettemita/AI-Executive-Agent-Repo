# clickup-mcp

ClickUp MCP-backed task/doc/time adapter.

- Plane: `hands`
- External API target: ClickUp MCP server (`mcp.clickup.com`)
- Auth: API key / MCP auth setup

## Input

- `action` (`task_list`, `task_create`, `doc_create`, `time_start`, `time_stop`)
- `title`/`content` for create actions
- `task_id` for time actions

## Output

- `provider`: `clickup-mcp`
- action-specific fields (`task_id`, `doc_id`, `timer_started`, `tasks[]`)

## Brevio use case

"Create a ClickUp task" and "Start time tracking" through MCP routing.
