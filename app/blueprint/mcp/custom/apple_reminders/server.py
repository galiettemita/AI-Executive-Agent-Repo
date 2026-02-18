from __future__ import annotations

import json
import sys
from typing import Any

from app.blueprint.mcp.custom.apple_reminders.auth import is_request_authorized
from app.blueprint.mcp.custom.apple_reminders.handlers import handle_tool_call
from app.blueprint.mcp.custom.apple_reminders.prompts import get_prompt, list_prompts
from app.blueprint.mcp.custom.apple_reminders.resources import list_resources, read_resource
from app.blueprint.mcp.custom.apple_reminders.tools import get_tools


def _ok(payload_id: str | int | None, result: dict[str, Any]) -> dict[str, Any]:
    return {"jsonrpc": "2.0", "id": payload_id, "result": result}


def _error(payload_id: str | int | None, message: str, code: int = -32000) -> dict[str, Any]:
    return {
        "jsonrpc": "2.0",
        "id": payload_id,
        "error": {"code": code, "message": message},
    }


def handle_rpc_payload(payload: dict[str, Any]) -> dict[str, Any] | None:
    method = str(payload.get("method") or "").strip()
    payload_id = payload.get("id")
    params = payload.get("params") if isinstance(payload.get("params"), dict) else {}

    # Notification (no id) does not require a response.
    if payload_id is None:
        return None

    if method == "initialize":
        return _ok(
            payload_id,
            {
                "protocolVersion": "2025-03-26",
                "serverInfo": {"name": "executive-os-mcp-apple-reminders", "version": "0.1.0"},
                "capabilities": {"tools": {}, "resources": {"subscribe": False}, "prompts": {}},
            },
        )

    if method == "tools/list":
        return _ok(payload_id, {"tools": get_tools()})

    if method == "tools/call":
        name = str(params.get("name") or "").strip()
        arguments = params.get("arguments") if isinstance(params.get("arguments"), dict) else {}
        auth_token = str(arguments.pop("_auth_token", "") or "")
        if not is_request_authorized(auth_token):
            return _error(payload_id, "Unauthorized", code=-32001)
        try:
            result = handle_tool_call(name=name, arguments=arguments)
            return _ok(payload_id, result)
        except Exception as exc:
            return _error(payload_id, str(exc))

    if method == "resources/list":
        return _ok(payload_id, {"resources": list_resources()})

    if method == "resources/read":
        uri = str(params.get("uri") or "").strip()
        return _ok(payload_id, read_resource(uri))

    if method == "prompts/list":
        return _ok(payload_id, {"prompts": list_prompts()})

    if method == "prompts/get":
        name = str(params.get("name") or "").strip()
        arguments = params.get("arguments") if isinstance(params.get("arguments"), dict) else {}
        return _ok(payload_id, get_prompt(name, arguments))

    if method == "ping":
        return _ok(payload_id, {})

    return _error(payload_id, f"Unsupported method: {method}", code=-32601)


def main() -> None:
    for line in sys.stdin:
        raw = line.strip()
        if not raw:
            continue
        try:
            payload = json.loads(raw)
        except Exception:
            continue
        if not isinstance(payload, dict):
            continue
        response = handle_rpc_payload(payload)
        if response is None:
            continue
        sys.stdout.write(json.dumps(response, ensure_ascii=False) + "\n")
        sys.stdout.flush()


if __name__ == "__main__":
    main()

