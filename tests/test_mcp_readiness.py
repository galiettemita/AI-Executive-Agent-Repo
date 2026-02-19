from __future__ import annotations

import asyncio

import pytest

from app.api.internal.hands import _assert_native_tool, execute as hands_execute
from app.blueprint.context_compiler import compile_tool_schemas
from app.blueprint.contracts import ContentProvenance, RiskLevel, ToolCall, ToolSpec
from app.blueprint.security import PrivilegeViolation, validate_tool_privilege
from app.blueprint.tools import get_tool_registry
from app.core.config import settings


def _register_test_mcp_tool() -> ToolSpec:
    registry = get_tool_registry()
    spec = ToolSpec(
        name="mcp.test.echo",
        description="Echo test tool for MCP readiness checks.",
        input_schema={
            "type": "object",
            "properties": {"text": {"type": "string"}},
            "required": ["text"],
        },
        output_schema={"type": "object"},
        risk_level=RiskLevel.NONE,
        is_mcp=True,
        mcp_server_id="mcp-test-server",
        capability_scope=["echo:read"],
    )
    registry.register(spec, min_tier=2, tags=["test"], llm_name="mcp_test_echo")
    return spec


def test_mcp_tool_registered_in_context_compiler_tool_schemas() -> None:
    _register_test_mcp_tool()
    schemas = compile_tool_schemas(tier=2)
    names = [((item.get("function") or {}).get("name")) for item in schemas]
    assert "mcp_test_echo" in names


def test_mcp_branch_stub_raises_not_implemented(monkeypatch) -> None:
    monkeypatch.setattr(settings, "FEATURE_MCP_CLIENT", False)
    with pytest.raises(NotImplementedError):
        _assert_native_tool(is_mcp=True)


def test_privilege_isolation_for_mcp_result() -> None:
    # Allowed path from untrusted MCP content.
    validate_tool_privilege(tool_name="web.search", provenance=ContentProvenance.MCP_RESULT)

    # Blocked side-effecting path from MCP content.
    with pytest.raises(PrivilegeViolation):
        validate_tool_privilege(tool_name="calendar.create", provenance=ContentProvenance.MCP_RESULT)


def test_hands_routes_fake_mcp_tool_to_stub_branch(monkeypatch) -> None:
    monkeypatch.setattr(settings, "FEATURE_MCP_CLIENT", False)
    _register_test_mcp_tool()
    call = ToolCall(
        tool_name="mcp.test.echo",
        tool="mcp_test_echo",
        arguments={"text": "hello"},
        args={"text": "hello"},
        input_provenance=ContentProvenance.USER_DIRECT,
    )
    result = asyncio.run(hands_execute(call))
    assert result.ok is False
    assert "MCP client is disabled" in str(result.error)
