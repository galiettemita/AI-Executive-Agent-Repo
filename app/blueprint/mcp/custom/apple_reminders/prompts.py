from __future__ import annotations

from typing import Any


def list_prompts() -> list[dict[str, Any]]:
    return [
        {
            "name": "reminders_daily_plan",
            "description": "Create a short prioritized daily reminder plan.",
            "arguments": [
                {"name": "date", "required": False, "description": "ISO date (defaults to today)."},
            ],
        }
    ]


def get_prompt(name: str, arguments: dict[str, Any] | None = None) -> dict[str, Any]:
    if name != "reminders_daily_plan":
        return {"messages": []}
    args = arguments or {}
    target_date = str(args.get("date") or "today")
    return {
        "messages": [
            {
                "role": "system",
                "content": f"Build a concise reminders plan for {target_date}.",
            }
        ]
    }

