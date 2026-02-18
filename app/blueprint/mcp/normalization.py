from __future__ import annotations

from typing import Any

from app.blueprint.contracts import RiskLevel, ToolCall, ToolResult, ToolSpec
from app.blueprint.mcp.contracts import MCPServerConfig, MCPToolResult, MCPToolSchema


def classify_mcp_tool_risk(tool_name: str, annotations: dict[str, Any] | None = None) -> RiskLevel:
    name = (tool_name or "").lower()
    ann = annotations or {}
    if bool(ann.get("critical")):
        return RiskLevel.CRITICAL
    if any(k in name for k in ("delete", "remove", "revoke", "send", "pay", "book", "checkout", "create_order")):
        return RiskLevel.HIGH
    if any(k in name for k in ("create", "update", "write", "post", "publish")):
        return RiskLevel.MEDIUM
    if any(k in name for k in ("list", "search", "get", "read", "fetch")):
        return RiskLevel.LOW
    return RiskLevel.NONE


def normalize_mcp_tool(
    *,
    server_id: str,
    mcp_tool: MCPToolSchema,
    risk: RiskLevel,
    server_config: MCPServerConfig,
) -> ToolSpec:
    canonical_name = f"mcp.{server_id}.{mcp_tool.name}"
    return ToolSpec(
        name=canonical_name,
        description=mcp_tool.description or f"{mcp_tool.name} via {server_id}",
        input_schema=mcp_tool.inputSchema or {"type": "object", "properties": {}},
        output_schema={"type": "object"},
        risk_level=risk,
        is_reversible=False,
        requires_approval_above=risk if risk in {RiskLevel.MEDIUM, RiskLevel.HIGH, RiskLevel.CRITICAL} else RiskLevel.HIGH,
        timeout_ms=max(1000, int(server_config.transport.timeout_ms)),
        retry_policy={"max_retries": 1, "backoff": "exponential"},
        rate_limit_per_min=max(1, int(server_config.rate_limit_per_min)),
        tags=sorted(set([*(server_config.tags or []), "mcp", server_id])),
        is_mcp=True,
        mcp_server_id=server_id,
        capability_scope=[f"mcp:{server_id}:{mcp_tool.name}"],
    )


def normalize_mcp_result(mcp_result: MCPToolResult, tool_call: ToolCall) -> ToolResult:
    output: dict[str, Any] = {
        "blocks": [],
    }
    text_parts: list[str] = []

    for block in mcp_result.content:
        output["blocks"].append(block.model_dump())
        if block.type == "text":
            if block.text:
                text_parts.append(block.text)
        elif block.type == "image":
            output["image_data"] = block.data
            output["image_mime"] = block.mimeType
        elif block.type == "resource" and block.resource is not None:
            output.setdefault("resources", []).append(block.resource.model_dump())

    if text_parts:
        output["text"] = "\n".join(text_parts)

    return ToolResult(
        tool_name=tool_call.tool_name,
        tool=tool_call.tool,
        status="failed" if mcp_result.is_error else "success",
        ok=not mcp_result.is_error,
        output=output,
        result=output,
        error={"message": "MCP tool returned error"} if mcp_result.is_error else None,
        latency_ms=int(mcp_result.latency_ms or 0),
        cost_cents=float(mcp_result.cost_cents or 0),
    )
