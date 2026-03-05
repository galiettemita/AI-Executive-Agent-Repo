# home-assistant

Home automation control adapter.

- Plane: `hands`
- External API target: Home Assistant REST API
- Auth: long-lived access token (self-hosted)

## Safety constraints

- `unlock` and `disable_alarm` require `two_factor_code`.

## Brevio use case

"Turn off all the lights" or "Set thermostat to 72" with guardrails for sensitive actions.
