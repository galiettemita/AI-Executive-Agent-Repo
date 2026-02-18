from __future__ import annotations

from fastapi.testclient import TestClient

from app.core.config import settings
from app.main import app


def _post(client: TestClient, server_id: str, payload: dict):
    return client.post(f"/mcp/wave1/{server_id}", json=payload)


def test_wave1_host_initialize_and_tools_list(monkeypatch):
    monkeypatch.setattr(settings, "MCP_HOST_TOKEN", None)
    client = TestClient(app)

    init = _post(
        client,
        "google-calendar-mcp",
        {
            "jsonrpc": "2.0",
            "id": "1",
            "method": "initialize",
            "params": {},
        },
    )
    assert init.status_code == 200
    body = init.json()
    assert body["result"]["protocolVersion"] == "2025-03-26"

    tools = _post(
        client,
        "google-calendar-mcp",
        {
            "jsonrpc": "2.0",
            "id": "2",
            "method": "tools/list",
            "params": {},
        },
    )
    assert tools.status_code == 200
    names = [item["name"] for item in tools.json()["result"]["tools"]]
    assert "calendar.list" in names


def test_wave1_host_requires_mcp_token_when_configured(monkeypatch):
    monkeypatch.setattr(settings, "MCP_HOST_TOKEN", "test-token")
    client = TestClient(app)

    response = _post(
        client,
        "gmail-mcp",
        {
            "jsonrpc": "2.0",
            "id": "3",
            "method": "tools/list",
            "params": {},
        },
    )
    assert response.status_code == 401

    response = client.post(
        "/mcp/wave1/gmail-mcp",
        headers={"X-MCP-Host-Token": "test-token"},
        json={"jsonrpc": "2.0", "id": "4", "method": "tools/list", "params": {}},
    )
    assert response.status_code == 200


def test_wave1_host_unknown_tool_returns_jsonrpc_error(monkeypatch):
    monkeypatch.setattr(settings, "MCP_HOST_TOKEN", None)
    client = TestClient(app)

    response = _post(
        client,
        "apple-reminders-mcp",
        {
            "jsonrpc": "2.0",
            "id": "5",
            "method": "tools/call",
            "params": {"name": "reminders.unknown", "arguments": {}},
        },
    )
    assert response.status_code == 200
    body = response.json()
    assert "error" in body
    assert body["error"]["code"] == -32000
