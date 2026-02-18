# Executive OS Apple Reminders MCP (Custom Server Scaffold)

This package is the Phase 3 starter scaffold for the Apple Reminders MCP server.

## Entry point

```bash
python3 -m app.blueprint.mcp.custom.apple_reminders.server
```

## Implemented methods

- `initialize`
- `tools/list`
- `tools/call`
- `resources/list`
- `resources/read`
- `prompts/list`
- `prompts/get`
- `ping`

## Current behavior

- Uses an in-memory reminder store.
- Supports `reminders.list`, `reminders.create`, `reminders.complete`, `reminders.delete`.
- Optional auth token check via `APPLE_REMINDERS_MCP_TOKEN`.

This is intentionally a safe scaffold so deployment wiring can proceed before iCloud-specific integrations are finalized.

