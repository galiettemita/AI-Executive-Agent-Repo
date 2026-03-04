# alter-actions

Hands-plane local app x-callback action orchestrator.

## Supported actions

- `list_actions`
- `trigger_action` (confirmation required)

## Notes

- Trigger path is confirmation-gated to avoid accidental local app mutations.
- Emits callback URLs derived from configured templates.
