from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timezone
import threading
import uuid
from typing import Any


@dataclass
class Reminder:
    reminder_id: str
    title: str
    notes: str | None = None
    due_at: str | None = None
    completed: bool = False
    created_at: str = ""
    updated_at: str = ""

    def to_dict(self) -> dict[str, Any]:
        return {
            "id": self.reminder_id,
            "title": self.title,
            "notes": self.notes,
            "due_at": self.due_at,
            "completed": self.completed,
            "created_at": self.created_at,
            "updated_at": self.updated_at,
        }


class ReminderStore:
    def __init__(self) -> None:
        self._lock = threading.Lock()
        self._items: dict[str, Reminder] = {}

    def list(self, *, completed: bool | None = None, limit: int = 50) -> list[dict[str, Any]]:
        with self._lock:
            values = list(self._items.values())
        if completed is not None:
            values = [item for item in values if bool(item.completed) is bool(completed)]
        values.sort(key=lambda item: item.updated_at or item.created_at, reverse=True)
        return [item.to_dict() for item in values[: max(1, min(limit, 200))]]

    def create(self, *, title: str, notes: str | None, due_at: str | None) -> dict[str, Any]:
        now = datetime.now(timezone.utc).isoformat()
        reminder = Reminder(
            reminder_id=f"rem_{uuid.uuid4().hex[:12]}",
            title=str(title).strip(),
            notes=(str(notes).strip() or None) if notes is not None else None,
            due_at=(str(due_at).strip() or None) if due_at is not None else None,
            completed=False,
            created_at=now,
            updated_at=now,
        )
        with self._lock:
            self._items[reminder.reminder_id] = reminder
        return reminder.to_dict()

    def complete(self, *, reminder_id: str) -> dict[str, Any]:
        with self._lock:
            item = self._items.get(reminder_id)
            if not item:
                raise KeyError(f"Reminder not found: {reminder_id}")
            item.completed = True
            item.updated_at = datetime.now(timezone.utc).isoformat()
            return item.to_dict()

    def delete(self, *, reminder_id: str) -> bool:
        with self._lock:
            return self._items.pop(reminder_id, None) is not None


_STORE = ReminderStore()


def _text_result(payload: dict[str, Any]) -> dict[str, Any]:
    return {"content": [{"type": "text", "text": str(payload)}]}


def handle_tool_call(name: str, arguments: dict[str, Any] | None = None) -> dict[str, Any]:
    args = arguments or {}
    if name == "reminders.list":
        completed = args.get("completed")
        limit_raw = args.get("limit") or 50
        limit = int(limit_raw) if str(limit_raw).strip() else 50
        items = _STORE.list(completed=completed if isinstance(completed, bool) else None, limit=limit)
        return {"content": [{"type": "text", "text": str({"items": items, "count": len(items)})}]}

    if name == "reminders.create":
        title = str(args.get("title") or "").strip()
        if not title:
            raise ValueError("title is required")
        reminder = _STORE.create(
            title=title,
            notes=args.get("notes"),
            due_at=args.get("due_at"),
        )
        return _text_result({"created": reminder})

    if name == "reminders.complete":
        reminder_id = str(args.get("reminder_id") or "").strip()
        if not reminder_id:
            raise ValueError("reminder_id is required")
        reminder = _STORE.complete(reminder_id=reminder_id)
        return _text_result({"completed": reminder})

    if name == "reminders.delete":
        reminder_id = str(args.get("reminder_id") or "").strip()
        if not reminder_id:
            raise ValueError("reminder_id is required")
        deleted = _STORE.delete(reminder_id=reminder_id)
        return _text_result({"deleted": deleted, "id": reminder_id})

    raise ValueError(f"Unknown tool: {name}")

