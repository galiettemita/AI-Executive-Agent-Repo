# jira

Jira issue management adapter.

- Plane: `hands`
- External API target: Jira REST API
- Auth: API token/OAuth

## Input

- `action` (`issue_list`, `issue_create`, `issue_transition`)
- issue fields (`project_key`, `issue_key`, `summary`, `description`, `transition_to`)

## Output

- `provider`: `jira`
- action-specific `issue_key` and `issues[]`

## Brevio use case

"Open a Jira ticket for this incident" and transition workflow states.
