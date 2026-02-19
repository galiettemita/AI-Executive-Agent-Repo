# Delegation Protocol

## Lifecycle
Delegations move through a simple status lifecycle:
- `pending`: created, not yet started
- `in_progress`: actively being worked
- `blocked`: needs user input or external dependency
- `completed`: finished successfully
- `cancelled`: stopped or superseded

## Rules of Engagement
- Always capture a clear task description and expected outcome.
- Set a due date if there is any time sensitivity.
- Escalate to the user if blocked for more than 24 hours.
- Update `HEARTBEAT.md` to keep priorities visible.

## API Endpoints
- `GET /api/v1/delegations?user_id=...`
- `POST /api/v1/delegations`
- `GET /api/v1/delegations/{delegation_id}?user_id=...`
- `PUT /api/v1/delegations/{delegation_id}?user_id=...`

## Data Recorded
- Delegate name and contact
- Task description and priority
- Status, due date, and reminder timestamps
- Result summary on completion
