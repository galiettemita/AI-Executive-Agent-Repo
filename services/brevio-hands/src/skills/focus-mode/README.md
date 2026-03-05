# focus-mode

Brain-plane focus execution skill for session lifecycle management.

## Supported actions

- `start_session`: starts a deterministic focus interval plan.
- `check_in`: returns corrective prompts during an active session.
- `end_session`: closes the session and summarizes outcomes.

## Notes

- No external dependencies.
- Uses deterministic session IDs and check-in schedules for replay safety.
