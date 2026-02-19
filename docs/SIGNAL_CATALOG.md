# Signal Catalog

## Feedback Signals (`feedback_signals`)
Used to capture explicit corrections and approval behavior.
- `correction`: user corrected an output or fact.
- `edit`: user edited a suggested output.
- `override`: user rejected a suggestion and provided a replacement.
- `complaint`: negative feedback about quality or tone.
- `outcome_failed`: task failed or result was unusable.
- `approved`: user explicitly approved an action.
- `rejected`: user explicitly rejected an action.

Recommended fields:
- `signal_type`: one of the types above.
- `original_output` / `corrected_output`.
- `context`: dict with `run_id`, `tool`, `category`, `source`.

## User Feedback (`user_feedback`)
Lightweight satisfaction input.
- `up` / `positive`.
- `down` / `negative`.

## Analytics Events (`analytics_events`)
Instrumented behavioral signals for system health and personalization.
Common event names:
- `message_received`
- `tool_invocation`
- `mcp_tool_invocation`
- `notification_sent`
- `research_run_completed`
- `proactive_trigger_fired`

Recommended payload fields:
- `server_id` / `mcp_server_id`
- `channel`
- `duration_ms`
- `status`

## Usage Notes
- Use `feedback_signals` for high-signal corrections that impact personalization.
- Use `user_feedback` for quick satisfaction metrics.
- Use `analytics_events` for aggregated dashboards and anonymized insights.
