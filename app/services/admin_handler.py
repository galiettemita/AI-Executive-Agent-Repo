# backend/app/services/admin_handler.py

from __future__ import annotations

import json
from datetime import datetime, timezone
from typing import Any, Dict, List

from openai import OpenAI
from sqlalchemy.orm import Session

from app.services.google_oauth import get_google_connection_status, build_google_auth_url
from app.services.google_calendar import create_calendar_event, list_upcoming_events
from app.services.google_gmail import create_draft, send_email
from app.db.models import TaskItem
from app.core.config import settings


client = OpenAI(api_key=settings.OPENAI_API_KEY)


def _iso_to_dt_utc(iso_str: str) -> datetime:
    dt = datetime.fromisoformat(iso_str.replace("Z", "+00:00"))
    return dt.astimezone(timezone.utc)


def _format_events(events: List[Dict[str, Any]]) -> str:
    if not events:
        return "No upcoming events found."
    lines = ["Upcoming events:"]
    for e in events:
        start = (e.get("start") or {}).get("dateTime") or (e.get("start") or {}).get("date")
        lines.append(f"- {e.get('summary','(no title)')} — {start}")
    return "\n".join(lines)


def _format_tasks(tasks: List[TaskItem]) -> str:
    if not tasks:
        return "No open tasks."
    out = ["Open tasks:"]
    for t in tasks:
        due = t.due_at.isoformat() if t.due_at else "no due date"
        out.append(f"- [{t.id}] {t.title} (due: {due})")
    return "\n".join(out)


def handle_admin(
    db: Session,
    user_id: str,
    history: List[Dict[str, str]],
    user_message: str,
) -> str:
    status = get_google_connection_status(db=db, user_id=user_id)
    connected = bool(status.get("connected"))

    msg_lower = user_message.lower()

    # Explicit connect request
    if ("connect" in msg_lower or "link" in msg_lower) and any(k in msg_lower for k in ["google", "gmail", "calendar"]):
        url = build_google_auth_url(user_id=user_id)
        return (
            "To connect Google (Gmail + Calendar), open this link in a browser:\n"
            f"{url}\n\n"
            "After you approve, come back here and tell me what you want to do."
        )

    # If user asks for calendar/email but not connected, provide connect link
    wants_google = any(k in msg_lower for k in ["calendar", "gmail", "email", "invite", "meeting"])
    if wants_google and not connected:
        url = build_google_auth_url(user_id=user_id)
        return (
            "I can do that, but you need to connect Google first.\n"
            "Open this link in a browser to connect Gmail + Calendar:\n"
            f"{url}"
        )

    # Simple task commands without LLM (works even without Google)
    if msg_lower.startswith("task ") or msg_lower.startswith("todo "):
        title = user_message.split(" ", 1)[1].strip()
        t = TaskItem(user_id=user_id, title=title)
        db.add(t)
        db.commit()
        db.refresh(t)
        return f"Added task [{t.id}]: {t.title}"

    if "list tasks" in msg_lower or "show tasks" in msg_lower:
        tasks = (
            db.query(TaskItem)
            .filter(TaskItem.user_id == user_id, TaskItem.completed == False)  # noqa: E712
            .order_by(TaskItem.due_at.is_(None), TaskItem.due_at.asc(), TaskItem.id.desc())
            .all()
        )
        return _format_tasks(tasks)

    # Connected Google actions: ask the model for structured JSON
    system = (
        "You are an admin assistant. Output ONLY valid JSON.\n"
        "Pick one action:\n"
        "create_calendar_event | list_upcoming_events | create_email_draft | send_email | create_task | list_tasks | need_clarification\n"
        "Rules:\n"
        "- Calendar: require title, start_time_iso, end_time_iso (UTC ISO like 2026-02-01T15:00:00Z).\n"
        "- Email: require to_email, subject, body.\n"
        "- Tasks: require title, optional due_time_iso (UTC ISO).\n"
        "- If missing fields, action=need_clarification with a single short question.\n"
        "Example:\n"
        '{"action":"create_calendar_event","title":"Lunch","start_time_iso":"2026-02-01T17:00:00Z","end_time_iso":"2026-02-01T18:00:00Z","description":null,"location":null,"reminders_minutes":[10]}\n'
    )

    resp = client.chat.completions.create(
        model=settings.OPENAI_MODEL,
        messages=[
            {"role": "system", "content": system},
            *history[-10:],
            {"role": "user", "content": user_message},
        ],
        temperature=0.1,
    )

    raw = resp.choices[0].message.content or "{}"
    try:
        data = json.loads(raw)
    except Exception:
        return "I had trouble parsing that. Please rephrase with the key details (who/what/when)."

    action = data.get("action")
    if action == "need_clarification":
        return str(data.get("question") or "What’s the key detail you want me to use?")

    try:
        if action == "list_upcoming_events":
            events = list_upcoming_events(db=db, user_id=user_id, max_results=int(data.get("max_results", 10)))
            return _format_events(events)

        if action == "create_calendar_event":
            title = str(data["title"])
            start = _iso_to_dt_utc(str(data["start_time_iso"]))
            end = _iso_to_dt_utc(str(data["end_time_iso"]))
            ev = create_calendar_event(
                db=db,
                user_id=user_id,
                title=title,
                start_utc=start,
                end_utc=end,
                description=data.get("description"),
                location=data.get("location"),
                reminders_minutes=data.get("reminders_minutes") or None,
            )
            # link = ev.get("htmlLink") --- editted this out so that i don't get a confimation link
            return f"✅ Your calendar event '{ev.get('summary')}' has been scheduled."

        if action == "create_email_draft":
            d = create_draft(
                db=db,
                user_id=user_id,
                to_email=str(data["to_email"]),
                subject=str(data["subject"]),
                body_text=str(data["body"]),
                cc=data.get("cc"),
                bcc=data.get("bcc"),
            )
            return f"Draft created in Gmail. Draft ID: {d.get('id')}"

        if action == "send_email":
            s = send_email(
                db=db,
                user_id=user_id,
                to_email=str(data["to_email"]),
                subject=str(data["subject"]),
                body_text=str(data["body"]),
                cc=data.get("cc"),
                bcc=data.get("bcc"),
            )
            return f"Sent. Message ID: {s.get('id')}"

        if action == "create_task":
            title = str(data["title"])
            due_iso = data.get("due_time_iso")
            due = _iso_to_dt_utc(str(due_iso)) if due_iso else None
            t = TaskItem(user_id=user_id, title=title, due_at=due)
            db.add(t)
            db.commit()
            db.refresh(t)
            return f"Added task [{t.id}]: {t.title}"

        if action == "list_tasks":
            tasks = (
                db.query(TaskItem)
                .filter(TaskItem.user_id == user_id, TaskItem.completed == False)  # noqa: E712
                .order_by(TaskItem.due_at.is_(None), TaskItem.due_at.asc(), TaskItem.id.desc())
                .all()
            )
            return _format_tasks(tasks)

        return "I can help with calendar, email, or tasks. Tell me what you want to do."
    except Exception as e:
        return f"Admin tool error: {str(e)}"