from __future__ import annotations

import asyncio

import pytest

from app.api.internal.hands import _assert_native_tool, execute as hands_execute
from app.api.internal import hands
from app.blueprint.context_compiler import compile_tool_schemas
from app.blueprint.contracts import ContentProvenance, RiskLevel, ToolCall, ToolResult, ToolSpec
from app.blueprint.security import PrivilegeViolation, validate_tool_privilege
from app.blueprint.tools import get_tool_registry
from app.core.config import settings
from app.db.database import SessionLocal
from app.db.models import Subscription
from app.services.content_safety import RateLimitDecision, SafetyVerdict


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


def _upsert_subscription(*, user_id: str, plan: str, status: str = "active") -> None:
    db = SessionLocal()
    try:
        sub = db.query(Subscription).filter(Subscription.user_id == user_id).first()
        if sub is None:
            sub = Subscription(user_id=user_id, plan=plan, status=status)
            db.add(sub)
        else:
            sub.plan = plan
            sub.status = status
        db.commit()
    finally:
        db.close()


def test_wave56_mcp_tool_execution_requires_professional_plan(monkeypatch) -> None:
    monkeypatch.setattr(settings, "FEATURE_MCP_CLIENT", True)
    spec = ToolSpec(
        name="mcp.wave5.flight.search",
        description="Wave 5 flight search tool.",
        input_schema={"type": "object", "properties": {"query": {"type": "string"}}},
        output_schema={"type": "object"},
        risk_level=RiskLevel.LOW,
        is_mcp=True,
        mcp_server_id="duffel-mcp",
    )
    get_tool_registry().register(spec, min_tier=2, tags=["test"], llm_name="mcp_wave5_flight_search")
    _upsert_subscription(user_id="wave56-free-user", plan="free")

    monkeypatch.setattr(
        hands,
        "invoke_mcp_tool",
        lambda *args, **kwargs: pytest.fail("invoke_mcp_tool should not be called when plan gate blocks"),
    )

    call = ToolCall(
        tool_name="mcp.wave5.flight.search",
        tool="mcp_wave5_flight_search",
        arguments={"query": "NYC to SFO"},
        args={"query": "NYC to SFO"},
        user_id="wave56-free-user",
        run_id="run-wave56-plan-gate",
        input_provenance=ContentProvenance.USER_DIRECT,
    )
    result = asyncio.run(hands_execute(call))
    assert result.ok is False
    assert "requires the professional plan" in str(result.error).lower()


def test_wave56_checkout_rate_limit_blocks_before_mcp_call(monkeypatch) -> None:
    monkeypatch.setattr(settings, "FEATURE_MCP_CLIENT", True)
    spec = ToolSpec(
        name="mcp.wave6.instacart.checkout",
        description="Wave 6 checkout tool.",
        input_schema={"type": "object", "properties": {"cart_id": {"type": "string"}}},
        output_schema={"type": "object"},
        risk_level=RiskLevel.HIGH,
        is_mcp=True,
        mcp_server_id="instacart-mcp",
    )
    get_tool_registry().register(spec, min_tier=2, tags=["test"], llm_name="mcp_wave6_instacart_checkout")
    _upsert_subscription(user_id="wave56-pro-user", plan="professional")

    monkeypatch.setattr(
        hands,
        "enforce_transaction_operation_rate_limit",
        lambda user_id, operation: RateLimitDecision(allowed=False, retry_after_seconds=30, reason="transaction_rate_limited"),
    )
    monkeypatch.setattr(
        hands,
        "invoke_mcp_tool",
        lambda *args, **kwargs: pytest.fail("invoke_mcp_tool should not be called when checkout rate limit blocks"),
    )

    call = ToolCall(
        tool_name="mcp.wave6.instacart.checkout",
        tool="mcp_wave6_instacart_checkout",
        arguments={"cart_id": "cart-1", "checkout": True, "approval_confirmed": True},
        args={"cart_id": "cart-1", "checkout": True, "approval_confirmed": True},
        user_id="wave56-pro-user",
        run_id="run-wave56-checkout-rate",
        input_provenance=ContentProvenance.USER_DIRECT,
    )
    result = asyncio.run(hands_execute(call))
    assert result.ok is False
    assert "transaction rate limit reached" in str(result.error).lower()


def test_wave56_checkout_abuse_detection_blocks_before_mcp_call(monkeypatch) -> None:
    monkeypatch.setattr(settings, "FEATURE_MCP_CLIENT", True)
    spec = ToolSpec(
        name="mcp.wave6.instacart.checkout.safe",
        description="Wave 6 checkout tool with abuse checks.",
        input_schema={"type": "object", "properties": {"cart_id": {"type": "string"}}},
        output_schema={"type": "object"},
        risk_level=RiskLevel.HIGH,
        is_mcp=True,
        mcp_server_id="instacart-mcp",
    )
    get_tool_registry().register(spec, min_tier=2, tags=["test"], llm_name="mcp_wave6_instacart_checkout_safe")
    _upsert_subscription(user_id="wave56-pro-user-2", plan="professional")

    monkeypatch.setattr(
        hands,
        "enforce_transaction_operation_rate_limit",
        lambda user_id, operation: RateLimitDecision(allowed=True),
    )
    monkeypatch.setattr(
        hands,
        "detect_transaction_abuse",
        lambda **kwargs: SafetyVerdict(
            flagged=True,
            risk_score=0.8,
            categories=["transaction_abuse", "cart_manipulation"],
            reason="test",
            classifier="transaction_rules",
        ),
    )
    monkeypatch.setattr(
        hands,
        "invoke_mcp_tool",
        lambda *args, **kwargs: pytest.fail("invoke_mcp_tool should not be called when abuse detection blocks"),
    )

    call = ToolCall(
        tool_name="mcp.wave6.instacart.checkout.safe",
        tool="mcp_wave6_instacart_checkout_safe",
        arguments={"cart_id": "cart-2", "checkout": True, "approval_confirmed": True},
        args={"cart_id": "cart-2", "checkout": True, "approval_confirmed": True},
        user_id="wave56-pro-user-2",
        run_id="run-wave56-checkout-abuse",
        input_provenance=ContentProvenance.USER_DIRECT,
    )
    result = asyncio.run(hands_execute(call))
    assert result.ok is False
    assert "blocked this operation for safety" in str(result.error).lower()


def test_wave56_checkout_requires_explicit_approval(monkeypatch) -> None:
    monkeypatch.setattr(settings, "FEATURE_MCP_CLIENT", True)
    spec = ToolSpec(
        name="mcp.wave6.instacart.checkout.confirm",
        description="Wave 6 checkout confirmation tool.",
        input_schema={"type": "object", "properties": {"cart_id": {"type": "string"}}},
        output_schema={"type": "object"},
        risk_level=RiskLevel.HIGH,
        is_mcp=True,
        mcp_server_id="instacart-mcp",
    )
    get_tool_registry().register(spec, min_tier=2, tags=["test"], llm_name="mcp_wave6_instacart_checkout_confirm")
    _upsert_subscription(user_id="wave56-pro-user-3", plan="professional")

    monkeypatch.setattr(
        hands,
        "invoke_mcp_tool",
        lambda *args, **kwargs: pytest.fail("invoke_mcp_tool should not run without explicit approval"),
    )

    call = ToolCall(
        tool_name="mcp.wave6.instacart.checkout.confirm",
        tool="mcp_wave6_instacart_checkout_confirm",
        arguments={"cart_id": "cart-3", "checkout": True},
        args={"cart_id": "cart-3", "checkout": True},
        user_id="wave56-pro-user-3",
        run_id="run-wave56-checkout-approval",
        input_provenance=ContentProvenance.USER_DIRECT,
    )
    result = asyncio.run(hands_execute(call))
    assert result.ok is False
    assert "requires explicit approval" in str(result.error).lower()


def test_mcp_invocation_records_tool_execution_with_server_binding(monkeypatch) -> None:
    monkeypatch.setattr(settings, "FEATURE_MCP_CLIENT", True)
    spec = ToolSpec(
        name="mcp.wave5.flight.read",
        description="Wave 5 read-only flight lookup.",
        input_schema={"type": "object", "properties": {"query": {"type": "string"}}},
        output_schema={"type": "object"},
        risk_level=RiskLevel.LOW,
        is_mcp=True,
        mcp_server_id="duffel-mcp",
    )
    get_tool_registry().register(spec, min_tier=2, tags=["test"], llm_name="mcp_wave5_flight_read")
    _upsert_subscription(user_id="wave56-log-user", plan="professional")

    async def _fake_invoke(db, *, spec_server_id: str, call: ToolCall):  # pragma: no cover - executed by hands
        return ToolResult(
            tool_name=call.tool_name,
            tool=call.tool,
            ok=True,
            result={"ok": True, "server_id": spec_server_id},
            output={"ok": True, "server_id": spec_server_id},
        )

    monkeypatch.setattr(hands, "invoke_mcp_tool", _fake_invoke)
    captured: dict[str, object] = {}

    def _fake_insert_tool_execution(db, **kwargs):  # pragma: no cover - called by hands
        captured.update(kwargs)
        return "tool-exec-1"

    monkeypatch.setattr(hands, "insert_tool_execution", _fake_insert_tool_execution)

    call = ToolCall(
        tool_name="mcp.wave5.flight.read",
        tool="mcp_wave5_flight_read",
        arguments={"query": "SFO to JFK"},
        args={"query": "SFO to JFK"},
        user_id="wave56-log-user",
        run_id="run-wave56-log",
        input_provenance=ContentProvenance.USER_DIRECT,
    )
    result = asyncio.run(hands_execute(call))
    assert result.ok is True
    assert captured
    input_payload = captured.get("input_payload")
    assert isinstance(input_payload, dict)
    assert bool(input_payload.get("is_mcp")) is True
    assert str(input_payload.get("mcp_server_id") or "") == "duffel-mcp"
