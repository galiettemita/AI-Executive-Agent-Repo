# linear

Linear issue management adapter.

- Plane: `hands`
- External API target: Linear GraphQL API
- Auth: API key or OAuth

## Input

- `action` (`issue_list`, `issue_create`, `issue_update`)
- issue metadata (`team_id`, `issue_id`, `title`, `description`, `status`)

## Output

- `provider`: `linear`
- `issue_id` and `issues[]` depending on action

## Brevio use case

"Create a Linear issue for this bug" or "Show open issues for ENG".
