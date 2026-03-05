# google-calendar

Calendar CRUD skill adapter.

- Plane: `hands`
- External API target: Google Calendar API v3 / MCP bridge
- Auth: OAuth2 (`calendar` scope)

## Safety constraints

- `create` and `delete` actions require explicit `confirmed=true`.

## Brevio use case

"Schedule dinner with Sarah Friday at 7" -> creates event after confirmation.
