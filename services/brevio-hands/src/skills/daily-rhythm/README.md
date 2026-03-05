# daily-rhythm

Brain-plane planning skill for structured daily briefings and wind-down prompts.

## Supported actions

- `compose_briefing`: generates morning briefing text, priorities, and schedule blocks.
- `wind_down_prompt`: generates end-of-day wrap-up guidance and reset nudges.

## Notes

- No external API calls; deterministic internal planner logic.
- Requires contextual inputs (`timezone`, `date`, and `wake_time_local` for morning briefings).
