# backend/app/services/admin_handler.py

from __future__ import annotations

import json
from datetime import datetime, timedelta, timezone
from typing import Any, Dict, List

from openai import OpenAI
from sqlalchemy.orm import Session

from app.services.google_oauth import get_google_connection_status, build_google_auth_url
from app.services.microsoft_oauth import get_microsoft_connection_status, build_microsoft_auth_url
from app.services.integration_credentials import get_connection_status as get_integration_status
from app.services.calendar_router import create_event as create_calendar_event, update_event as update_calendar_event, list_events_in_range
from app.services.email_router import create_draft, send_email, search_emails
from app.services.email_semantic_search import semantic_search_emails, index_recent_emails
from app.services.consent_service import require_consent
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
        event_id = e.get("id")
        if event_id:
            lines.append(f"- [{event_id}] {e.get('summary','(no title)')} — {start}")
        else:
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
    google_status = get_google_connection_status(db=db, user_id=user_id)
    ms_status = get_microsoft_connection_status(db=db, user_id=user_id)
    caldav_status = get_integration_status(db=db, user_id=user_id, provider="caldav")

    connected_google = bool(google_status.get("connected"))
    connected_ms = bool(ms_status.get("connected"))
    connected_caldav = bool(caldav_status.get("connected"))
    connected_any_calendar = connected_google or connected_ms or connected_caldav

    msg_lower = user_message.lower()
    preferred_provider = None
    if any(k in msg_lower for k in ["outlook", "microsoft", "office365", "office 365"]):
        preferred_provider = "microsoft"
    elif any(k in msg_lower for k in ["google", "gmail"]):
        preferred_provider = "google"
    elif any(k in msg_lower for k in ["apple", "icloud", "caldav"]):
        preferred_provider = "caldav"

    # Explicit connect request
    if ("connect" in msg_lower or "link" in msg_lower) and any(k in msg_lower for k in ["google", "gmail", "calendar"]):
        try:
            url = build_google_auth_url(user_id=user_id)
        except Exception as e:
            return f"Google connect failed: {str(e)}"
        return (
            "To connect Google (Gmail + Calendar), open this link in a browser:\n"
            f"{url}\n\n"
            "After you approve, come back here and tell me what you want to do."
        )

    if ("connect" in msg_lower or "link" in msg_lower) and any(k in msg_lower for k in ["outlook", "microsoft", "office365", "office 365"]):
        try:
            url = build_microsoft_auth_url(user_id=user_id)
        except Exception as e:
            return f"Microsoft connect failed: {str(e)}"
        return (
            "To connect Microsoft (Outlook + Calendar), open this link in a browser:\n"
            f"{url}\n\n"
            "After you approve, come back here and tell me what you want to do."
        )

    if ("connect" in msg_lower or "link" in msg_lower) and any(k in msg_lower for k in ["apple", "icloud", "caldav"]):
        return (
            "To connect Apple/iCloud Calendar (CalDAV), you’ll need to add your CalDAV credentials.\n"
            "Use the CalDAV connect endpoint with your server URL, username, and app-specific password.\n"
            "If you want step-by-step setup instructions, just say: “How do I connect CalDAV?”"
        )

    # If user asks for calendar/email but not connected, provide connect link
    wants_calendar_or_email = any(k in msg_lower for k in ["calendar", "gmail", "email", "invite", "meeting"])
    if wants_calendar_or_email and not connected_any_calendar:
        google_url = None
        ms_url = None
        try:
            google_url = build_google_auth_url(user_id=user_id)
        except Exception:
            pass
        try:
            ms_url = build_microsoft_auth_url(user_id=user_id)
        except Exception:
            pass

        lines = ["I can do that, but you need to connect a calendar first."]
        if google_url:
            lines.append("Open this link to connect Gmail + Google Calendar:")
            lines.append(google_url)
        if ms_url:
            lines.append("Open this link to connect Outlook (Microsoft):")
            lines.append(ms_url)
        lines.append("For Apple/iCloud (CalDAV), ask me for the CalDAV connection steps.")
        return "\n".join(lines)

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

    # Connected calendar actions: ask the model for structured JSON
    system = (
        "You are an admin assistant. Output ONLY valid JSON.\n"
        "Pick one action:\n"
        "create_calendar_event | update_calendar_event | list_upcoming_events | create_email_draft | send_email | search_email | semantic_search_email | create_task | list_tasks | need_clarification\n"
        "Rules:\n"
        "- Calendar: require title, start_time_iso, end_time_iso (UTC ISO like 2026-02-01T15:00:00Z).\n"
        "- Calendar: optional calendar_provider = google | microsoft | caldav.\n"
        "- Update: require event_id and at least one field to change.\n"
        "- Email: require to_email, subject, body.\n"
        "- Search: require query (string), optional max_results, optional email_provider.\n"
        "- Semantic search: require query (string), optional max_results, optional email_provider.\n"
        "- Tasks: require title, optional due_time_iso (UTC ISO).\n"
        "- If missing fields, action=need_clarification with a single short question.\n"
        "Example:\n"
        '{"action":"create_calendar_event","title":"Lunch","start_time_iso":"2026-02-01T17:00:00Z","end_time_iso":"2026-02-01T18:00:00Z","description":null,"location":null,"reminders_minutes":[10],"calendar_provider":"google"}\n'
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
            try:
                require_consent(db, user_id, "calendar")
            except Exception as e:
                return str(e)
            now = datetime.now(timezone.utc)
            window_end = now + timedelta(days=30)
            provider = data.get("calendar_provider") or preferred_provider
            events = list_events_in_range(
                db=db,
                user_id=user_id,
                start_utc=now,
                end_utc=window_end,
                provider=provider,
                max_results=int(data.get("max_results", 10)),
            )
            return _format_events(events)

        if action == "update_calendar_event":
            try:
                require_consent(db, user_id, "calendar")
            except Exception as e:
                return str(e)
            event_id = str(data["event_id"])
            start = _iso_to_dt_utc(str(data["start_time_iso"])) if data.get("start_time_iso") else None
            end = _iso_to_dt_utc(str(data["end_time_iso"])) if data.get("end_time_iso") else None
            ev = update_calendar_event(
                db=db,
                user_id=user_id,
                event_id=event_id,
                title=data.get("title"),
                start_utc=start,
                end_utc=end,
                description=data.get("description"),
                location=data.get("location"),
                reminders_minutes=data.get("reminders_minutes"),
                attendees=data.get("attendees"),
                provider=data.get("calendar_provider") or preferred_provider,
            )
            return f"✅ Updated calendar event '{ev.get('summary') or event_id}'."

        if action == "create_calendar_event":
            try:
                require_consent(db, user_id, "calendar")
            except Exception as e:
                return str(e)
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
                attendees=data.get("attendees"),
                provider=data.get("calendar_provider") or preferred_provider,
            )
            # link = ev.get("htmlLink") --- editted this out so that i don't get a confimation link
            return f"✅ Your calendar event '{ev.get('summary')}' has been scheduled."

        if action == "create_email_draft":
            try:
                require_consent(db, user_id, "email")
            except Exception as e:
                return str(e)
            d = create_draft(
                db=db,
                user_id=user_id,
                to_email=str(data["to_email"]),
                subject=str(data["subject"]),
                body_text=str(data["body"]),
                cc=data.get("cc"),
                bcc=data.get("bcc"),
                provider=data.get("email_provider") or preferred_provider,
            )
            draft_id = d.get("id")
            return f"Draft created. Draft ID: {draft_id}" if draft_id else "Draft created."

        if action == "send_email":
            try:
                require_consent(db, user_id, "email")
            except Exception as e:
                return str(e)
            s = send_email(
                db=db,
                user_id=user_id,
                to_email=str(data["to_email"]),
                subject=str(data["subject"]),
                body_text=str(data["body"]),
                cc=data.get("cc"),
                bcc=data.get("bcc"),
                provider=data.get("email_provider") or preferred_provider,
            )
            msg_id = s.get("id") if isinstance(s, dict) else None
            return f"Sent. Message ID: {msg_id}" if msg_id else "Sent."

        if action == "search_email":
            try:
                require_consent(db, user_id, "email")
            except Exception as e:
                return str(e)
            results = search_emails(
                db=db,
                user_id=user_id,
                query=str(data["query"]),
                max_results=int(data.get("max_results", 5)),
                provider=data.get("email_provider") or preferred_provider,
            )
            if not results:
                return "No matching emails found."
            lines = ["Top email matches:"]
            for e in results[:5]:
                lines.append(f"- {e.get('from')} — {e.get('subject')} ({e.get('date')})")
            return "\n".join(lines)

        if action == "semantic_search_email":
            try:
                require_consent(db, user_id, "email")
            except Exception as e:
                return str(e)
            provider = data.get("email_provider") or preferred_provider
            try:
                index_recent_emails(db=db, user_id=user_id, provider=provider, max_results=50, hours_back=168)
            except Exception:
                pass
            results = semantic_search_emails(
                db=db,
                user_id=user_id,
                query=str(data["query"]),
                top_k=int(data.get("max_results", 5)),
                provider=provider,
            )
            if not results:
                return "No matching emails found."
            lines = ["Top semantic email matches:"]
            for r in results[:5]:
                meta = r.get("metadata") or {}
                lines.append(f"- {meta.get('from')} — {meta.get('subject')} ({meta.get('date')})")
            return "\n".join(lines)

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
