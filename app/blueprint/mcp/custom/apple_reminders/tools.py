from __future__ import annotations

from typing import Any


def get_tools() -> list[dict[str, Any]]:
    return [
        {
            "name": "reminders.list",
            "description": "List reminders, optionally filtering by completion state.",
            "inputSchema": {
                "type": "object",
                "properties": {
                    "completed": {"type": "boolean"},
                    "limit": {"type": "integer", "minimum": 1, "maximum": 200},
                },
            },
        },
        {
            "name": "reminders.create",
            "description": "Create a reminder with title, optional notes, and optional due date.",
            "inputSchema": {
                "type": "object",
                "properties": {
                    "title": {"type": "string"},
                    "notes": {"type": "string"},
                    "due_at": {"type": "string", "description": "ISO8601 datetime"},
                },
                "required": ["title"],
            },
        },
        {
            "name": "reminders.complete",
            "description": "Mark a reminder as complete by id.",
            "inputSchema": {
                "type": "object",
                "properties": {
                    "reminder_id": {"type": "string"},
                },
                "required": ["reminder_id"],
            },
        },
        {
            "name": "reminders.delete",
            "description": "Delete a reminder by id.",
            "inputSchema": {
                "type": "object",
                "properties": {
                    "reminder_id": {"type": "string"},
                },
                "required": ["reminder_id"],
            },
        },
    ]

