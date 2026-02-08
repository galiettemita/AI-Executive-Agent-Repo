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
from app.services.calendar_intelligence import meeting_prep_brief, generate_followup
from app.services.email_router import send_email, search_emails
from app.services.email_semantic_search import semantic_search_emails, index_recent_emails
from app.services.email_intelligence import summarize_inbox, draft_reply
from app.services.email_draft_service import (
    create_email_draft,
    get_latest_pending_draft,
    send_email_draft,
    cancel_pending_draft,
)
from app.services.consent_service import require_consent
from app.db.models import TaskItem
from app.core.config import settings


client = OpenAI(api_key=settings.OPENAI_API_KEY)


def _normalize_text(value: str) -> str:
    return " ".join((value or "").strip().lower().split())


def _is_send_confirmation(message: str) -> bool:
    text = _normalize_text(message)
    if not text:
        return False
    if text in {"send", "send it", "yes send", "ok send", "approve", "approved", "confirm"}:
        return True
    last = text.split()[-1]
    return last in {"send", "approve", "confirm"}


def _is_cancel_confirmation(message: str) -> bool:
    text = _normalize_text(message)
    if not text:
        return False
    if text in {"cancel", "never mind", "nevermind", "discard", "stop"}:
        return True
    last = text.split()[-1]
    return last in {"cancel", "discard"}


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

    # Pending email draft confirmation
    pending_draft = get_latest_pending_draft(db, user_id)
    if pending_draft:
        if _is_send_confirmation(user_message):
            try:
                require_consent(db, user_id, "email")
            except Exception as e:
                return str(e)
            try:
                sent = send_email_draft(db, pending_draft)
                return f"✅ Email sent to {sent.to_email} with subject '{sent.subject}'."
            except Exception as exc:
                return f"Failed to send email: {exc}"
        if _is_cancel_confirmation(user_message):
            cancel_pending_draft(db, pending_draft)
            return "Draft canceled."

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
        "create_calendar_event | update_calendar_event | list_upcoming_events | meeting_prep_brief | meeting_followup | summarize_inbox | create_email_draft | draft_email_reply | send_email | search_email | semantic_search_email | create_task | list_tasks | need_clarification\n"
        "Rules:\n"
        "- Calendar: require title, start_time_iso, end_time_iso (UTC ISO like 2026-02-01T15:00:00Z).\n"
        "- Calendar: optional calendar_provider = google | microsoft | caldav.\n"
        "- Meeting prep: require event_id, optional calendar_provider.\n"
        "- Meeting follow-up: require event_id, optional notes, optional calendar_provider.\n"
        "- Update: require event_id and at least one field to change.\n"
        "- Email: require to_email, subject, body.\n"
        "- Summarize inbox: optional max_results, hours_back, email_provider.\n"
        "- Draft reply: require message_id or query, optional tone, instruction, email_provider.\n"
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

        if action == "meeting_prep_brief":
            try:
                require_consent(db, user_id, "calendar")
            except Exception as e:
                return str(e)
            event_id = data.get("event_id")
            if not event_id:
                return "Please provide the event_id for the meeting."
            result = meeting_prep_brief(
                db=db,
                user_id=user_id,
                event_id=str(event_id),
                provider=data.get("calendar_provider") or preferred_provider,
            )
            lines = [result.get("brief") or "Meeting brief ready."]
            checklist = result.get("checklist") or []
            if checklist:
                lines.append("Prep checklist:")
                for item in checklist[:6]:
                    lines.append(f"- {item}")
            questions = result.get("questions") or []
            if questions:
                lines.append("Questions to clarify:")
                for q in questions[:5]:
                    lines.append(f"- {q}")
            return "\n".join(lines)

        if action == "meeting_followup":
            try:
                require_consent(db, user_id, "calendar")
            except Exception as e:
                return str(e)
            event_id = data.get("event_id")
            if not event_id:
                return "Please provide the event_id for the meeting."
            result = generate_followup(
                db=db,
                user_id=user_id,
                event_id=str(event_id),
                notes=data.get("notes"),
                provider=data.get("calendar_provider") or preferred_provider,
            )
            tasks = result.get("tasks") or []
            task_lines = []
            for t in tasks:
                row = TaskItem(user_id=user_id, title=str(t))
                db.add(row)
                db.commit()
                db.refresh(row)
                task_lines.append(f"- {row.title}")

            draft_id = None
            if result.get("to_email") and result.get("email_body"):
                draft = create_email_draft(
                    db=db,
                    user_id=user_id,
                    to_email=result.get("to_email") or "",
                    subject=result.get("email_subject") or "Follow-up",
                    body_text=result.get("email_body") or "",
                    provider=result.get("event", {}).get("provider"),
                    metadata={"origin": "calendar_followup"},
                )
                draft_id = draft.id

            lines = ["Follow-up prepared."]
            if task_lines:
                lines.append("Tasks:")
                lines.extend(task_lines)
            if draft_id:
                lines.append("Draft email ready. Reply 'send' to send.")
            return "\n".join(lines)

        if action == "create_email_draft":
            try:
                require_consent(db, user_id, "email")
            except Exception as e:
                return str(e)
            draft = create_email_draft(
                db=db,
                user_id=user_id,
                to_email=str(data["to_email"]),
                subject=str(data["subject"]),
                body_text=str(data["body"]),
                cc=data.get("cc"),
                bcc=data.get("bcc"),
                provider=data.get("email_provider") or preferred_provider,
                metadata={"origin": "admin"},
            )
            preview = f"To: {draft.to_email}\nSubject: {draft.subject}\n\n{draft.body_text}"
            return f"Draft ready. Reply 'send' to send.\n\n{preview}"

        if action == "draft_email_reply":
            try:
                require_consent(db, user_id, "email")
            except Exception as e:
                return str(e)
            try:
                reply = draft_reply(
                    db=db,
                    user_id=user_id,
                    message_id=data.get("message_id"),
                    query=data.get("query"),
                    tone=data.get("tone"),
                    instruction=data.get("instruction"),
                    provider=data.get("email_provider") or preferred_provider,
                )
            except Exception as exc:
                return str(exc)
            draft = create_email_draft(
                db=db,
                user_id=user_id,
                to_email=reply.get("to_email") or "",
                subject=reply.get("subject") or "",
                body_text=reply.get("body") or "",
                provider=reply.get("provider") or (data.get("email_provider") or preferred_provider),
                source_message_id=reply.get("source_message_id"),
                metadata={"origin": "reply"},
            )
            preview = f"To: {draft.to_email}\nSubject: {draft.subject}\n\n{draft.body_text}"
            return f"Draft reply ready. Reply 'send' to send.\n\n{preview}"

        if action == "summarize_inbox":
            try:
                require_consent(db, user_id, "email")
            except Exception as e:
                return str(e)
            result = summarize_inbox(
                db=db,
                user_id=user_id,
                max_results=int(data.get("max_results", 10)),
                hours_back=int(data.get("hours_back", 24)),
                provider=data.get("email_provider") or preferred_provider,
            )
            lines = [result.get("summary") or "Inbox summary:"]
            priorities = result.get("priorities") or []
            if priorities:
                lines.append("Top priorities:")
                for item in priorities[:5]:
                    lines.append(
                        f"- ({item.get('priority')}) {item.get('from')} — {item.get('subject')}: {item.get('reason')}"
                    )
            return "\n".join(lines)

        if action == "send_email":
            try:
                require_consent(db, user_id, "email")
            except Exception as e:
                return str(e)
            if not _is_send_confirmation(user_message):
                draft = create_email_draft(
                    db=db,
                    user_id=user_id,
                    to_email=str(data["to_email"]),
                    subject=str(data["subject"]),
                    body_text=str(data["body"]),
                    cc=data.get("cc"),
                    bcc=data.get("bcc"),
                    provider=data.get("email_provider") or preferred_provider,
                    metadata={"origin": "send_request"},
                )
                preview = f"To: {draft.to_email}\nSubject: {draft.subject}\n\n{draft.body_text}"
                return f"Draft ready. Reply 'send' to send.\n\n{preview}"
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
