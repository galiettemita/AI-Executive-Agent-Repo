# MCP Guide

## Purpose
MCP servers provide controlled access to external tools and services. They are registered in `server_catalog` and exposed through the ToolRegistry with risk classification, approval gates, and plan gating.

## Provisioning Workflow
1. Register server metadata in `server_catalog`.
2. Verify `tools/list` and normalize tool schemas.
3. Bind tools in ToolRegistry with risk levels and capabilities.
4. Enable auth/OAuth setup and token storage in `oauth_tokens`.
5. Validate normalization and provenance tagging in `tool_executions`.

## Transport Modes
- `mock`: Use `mock://` tool lists for local testing.
- `stdio`: Connect to a local MCP process.
- `remote`: Use the configured MCP server URL over HTTP.

## Risk and Approval Gates
- Write actions (booking, checkout, financial) require explicit approval via `approval_confirmed=true`.
- High-risk tools are gated by `RiskLevel` and capability checks.
- Tool provenance is enforced to block unsafe cross-tool escalation.

## Plan Gating (Wave 5-6)
Wave 5-6 servers are plan-gated (Professional or higher). Gate checks occur before any MCP invocation.

## Troubleshooting
- Ensure the server is listed in `server_catalog` with a valid `hosting_model`.
- Confirm `FEATURE_MCP_CLIENT=true` and correct MCP URL env vars.
- Check `tool_executions` for errors and `audit_logs` for approval denials.
