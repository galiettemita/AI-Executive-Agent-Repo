from __future__ import annotations

from typing import Any


def list_resources() -> list[dict[str, Any]]:
    return [
        {
            "uri": "reminders://lists/default",
            "name": "Default Reminder List",
            "description": "Default list surfaced by the Executive OS Apple Reminders MCP server.",
            "mimeType": "application/json",
        }
    ]


def read_resource(uri: str) -> dict[str, Any]:
    if uri != "reminders://lists/default":
        return {"contents": []}
    return {
        "contents": [
            {
                "uri": uri,
                "mimeType": "application/json",
                "text": '{"list":"default","source":"apple-reminders-mcp"}',
            }
        ]
    }

