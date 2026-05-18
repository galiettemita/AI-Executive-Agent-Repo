# Schema Drift Report — Segment 6

## Migration File Coverage
- 18 migrations in `db/migrations/` (001–018)
- Migration verification script references migrations 001–013
- **Drift**: Script hardcodes `"applying all migrations from db/migrations/ (001 -> 013)"` in log message but actually globs `*.sql`, so all 18 are applied.

## Enum Count Expectation
- Script asserts `enum_count == 82`
- If migrations 014–018 add new enums, this check may fail at runtime
- **Action needed**: verify enum count after full migration run; update script if necessary

## Column Checks
- Script validates: `tool_executions.is_mcp`, `tool_executions.mcp_server_id`, `tool_executions.content_provenance`, `user_oauth_tokens.provider`
- These are introduced in migration 005

## RLS Coverage
- All tables with `workspace_id` column must have RLS enabled
- Verified by script at runtime

## Untracked Code Expectations
- New LLM files (`internal/llm/anthropic.go`, `openai.go`, `bootstrap.go`, `client.go`, `intelligence.go`) may reference tables or columns not yet in migrations
- **Assessment**: These files are untracked/uncommitted — schema alignment will be verified during prompt execution

## Summary
- No critical schema drift detected in committed code
- Migration verification script needs enum count update if new enums were added in migrations 014–018
- Untracked LLM files require schema review during prompt implementation
