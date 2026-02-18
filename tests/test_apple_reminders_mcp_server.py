from __future__ import annotations

import ast

from app.blueprint.mcp.custom.apple_reminders.server import handle_rpc_payload


def _call(method: str, params: dict | None = None, rpc_id: str = "1") -> dict:
    payload = {
        "jsonrpc": "2.0",
        "id": rpc_id,
        "method": method,
        "params": params or {},
    }
    response = handle_rpc_payload(payload)
    assert response is not None
    assert "error" not in response
    return response


def test_apple_reminders_server_initialize_and_tools() -> None:
    init = _call("initialize")
    assert init["result"]["serverInfo"]["name"] == "executive-os-mcp-apple-reminders"

    tools = _call("tools/list")
    names = [item["name"] for item in tools["result"]["tools"]]
    assert "reminders.create" in names
    assert "reminders.complete" in names


def test_apple_reminders_create_list_complete_flow() -> None:
    create = _call(
        "tools/call",
        {
            "name": "reminders.create",
            "arguments": {"title": "Review Phase 3 rollout", "notes": "Include MCP wave-1"},
        },
    )
    created_payload = ast.literal_eval(create["result"]["content"][0]["text"])
    reminder_id = created_payload["created"]["id"]
    assert reminder_id.startswith("rem_")

    listed = _call("tools/call", {"name": "reminders.list", "arguments": {"completed": False}})
    listed_payload = ast.literal_eval(listed["result"]["content"][0]["text"])
    assert listed_payload["count"] >= 1

    completed = _call(
        "tools/call",
        {
            "name": "reminders.complete",
            "arguments": {"reminder_id": reminder_id},
        },
    )
    completed_payload = ast.literal_eval(completed["result"]["content"][0]["text"])
    assert completed_payload["completed"]["completed"] is True

