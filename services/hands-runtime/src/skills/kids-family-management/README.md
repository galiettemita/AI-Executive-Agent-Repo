# kids-family-management

Custom-build adapter for family schedule, pickup planning, and check-in workflows.

## Auth
- Pending partner integrations for family/location composites.

## Input
- `action`: `family_schedule`, `pickup_plan`, `location_checkin`
- pickup plan requires `child_name` + `date`
- location check-in requires `child_name` + `location`

## Output
- `provider`: `kids-family-management`
- action echo with events/check-in status
- `partnership_status`: `awaiting_api_partnership`

## Notes
- `// CUSTOM_BUILD_REQUIRED: Awaiting API partnership` enforced in adapter runtime.
