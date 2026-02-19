from __future__ import annotations

import asyncio

from app.api.internal import hands
from app.blueprint.brain import responder
from app.blueprint.contracts import LLMProvider, LLMResponse, TokenUsage, ToolCall, ToolResult
from app.db.database import SessionLocal
from app.services.provisioning_catalog import available_servers_for_user, render_available_servers_section


def test_available_servers_section_includes_catalog_entries():
    db = SessionLocal()
    try:
        entries = available_servers_for_user(db, user_id="layer1-user", connected_server_ids=set())
        section = render_available_servers_section(entries)
    finally:
        db.close()

    assert "## Available Servers (Not Connected)" in section
    assert "duffel-mcp" in section


def test_provision_server_stub_handler_returns_awaiting_auth(monkeypatch):
    monkeypatch.setattr(hands.settings, "FEATURE_PRIVILEGE_ISOLATION", False)

    result = asyncio.run(
        hands.execute(
            ToolCall(
                tool_name="provision_server",
                arguments={"server_id": "duffel-mcp", "reason": "Need flight booking capability"},
                user_id="layer1-user",
                run_id="run-layer1",
            )
        )
    )

    assert result.ok is True
    payload = result.result or {}
    assert payload.get("status") == "awaiting_auth"
    assert payload.get("server_id") == "duffel-mcp"


class _CapabilityGapRouter:
    def __init__(self) -> None:
        self.calls = 0

    def call(self, req):  # pragma: no cover - executed through generate_reply
        self.calls += 1
        if self.calls == 1:
            return LLMResponse(
                provider=LLMProvider.OPENAI,
                model="gpt-4o-mini",
                content="",
                tool_calls=[
                    {
                        "id": "tc-1",
                        "function": {
                            "name": "flight.search",
                            "arguments": "{}",
                        },
                    }
                ],
                usage=TokenUsage(input_tokens=10, output_tokens=6, total_tokens=16),
            )
        return LLMResponse(
            provider=LLMProvider.OPENAI,
            model="gpt-4o-mini",
            content="I found a matching server and started setup.",
            usage=TokenUsage(input_tokens=9, output_tokens=8, total_tokens=17),
        )

    def system_mode(self, force_refresh: bool = False) -> str:
        return "normal"


def test_generate_reply_capability_gap_calls_provision_server(monkeypatch):
    router = _CapabilityGapRouter()
    monkeypatch.setattr(responder, "get_llm_router", lambda: router)
    monkeypatch.setattr(
        responder,
        "compile_context_messages",
        lambda **kwargs: ([{"role": "system", "content": "x"}, {"role": "user", "content": kwargs.get("user_text") or ""}], [], []),
    )
    monkeypatch.setattr(
        responder,
        "compile_tool_schemas",
        lambda **kwargs: [
            {
                "type": "function",
                "function": {
                    "name": "web_search",
                    "description": "Search",
                    "parameters": {"type": "object", "properties": {"query": {"type": "string"}}},
                },
            }
        ],
    )
    monkeypatch.setattr(responder, "get_cached_response", lambda **kwargs: None)
    monkeypatch.setattr(responder, "put_cached_response", lambda **kwargs: None)
    monkeypatch.setattr(
        responder,
        "_load_tools_markdown",
        lambda **kwargs: (
            "# TOOLS.md\n"
            "## Available Servers (Not Connected)\n"
            "- duffel-mcp: Flight search and booking | auth: api_key | setup: ~30s\n"
        ),
    )
    monkeypatch.setattr(
        responder,
        "_maybe_reflect_response",
        lambda **kwargs: (kwargs.get("current_response") or "", {"applied": False}),
    )

    calls: list[dict[str, object]] = []

    def _fake_execute_tool(**kwargs):
        calls.append({"tool": kwargs.get("tool"), "args": kwargs.get("args")})
        return ToolResult(
            tool_name="provision_server",
            tool="provision_server",
            ok=True,
            result={"status": "awaiting_auth", "server_id": "duffel-mcp"},
        )

    monkeypatch.setattr(responder, "_execute_tool", _fake_execute_tool)

    text_value, meta = responder.generate_reply(
        user_text="Book me a flight to New York next week",
        tier=2,
        user_id="layer1-user",
        conversation_id="conv-layer1",
        run_id="run-layer1",
    )

    assert "started setup" in text_value
    assert calls
    assert calls[0]["tool"] == "provision_server"
    args = calls[0]["args"] if isinstance(calls[0]["args"], dict) else {}
    assert args.get("server_id") == "duffel-mcp"
    assert int(meta.get("iterations") or 0) >= 2
