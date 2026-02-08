from __future__ import annotations

import json
import logging
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional, Tuple

from openai import OpenAI
from sqlalchemy.orm import Session

from app.core.config import settings
from app.services.calendar_router import list_events_in_range, get_event_by_id
from app.services.calendar_scheduling import find_conflicts
from app.services.email_router import search_emails
from app.services.profile_service import get_profile

logger = logging.getLogger(__name__)

client = OpenAI(api_key=settings.OPENAI_API_KEY)


VIRTUAL_KEYWORDS = [
    "zoom",
    "google meet",
    "meet.google.com",
    "microsoft teams",
    "teams.microsoft.com",
    "webex",
    "skype",
    "hangouts",
    "virtual",
    "online",
]


def _is_virtual_location(location: Optional[str]) -> bool:
    if not location:
        return False
    loc = location.lower()
    return any(k in loc for k in VIRTUAL_KEYWORDS)


def _default_buffer_minutes(prefs: Dict[str, Any], location: Optional[str]) -> int:
    if _is_virtual_location(location):
        return 0
    try:
        return int(prefs.get("calendar_buffer_minutes") or 15)
    except Exception:
        return 15


def _extract_attendees(event: Dict[str, Any]) -> List[str]:
    attendees = event.get("attendees") or []
    emails = []
    for a in attendees:
        if isinstance(a, dict):
            addr = a.get("email") or a.get("emailAddress")
            if isinstance(addr, dict):
                addr = addr.get("address")
            if addr:
                emails.append(str(addr))
        elif isinstance(a, str):
            emails.append(a)
    return emails


def _extract_organizer(event: Dict[str, Any]) -> Optional[str]:
    organizer = event.get("organizer")
    if isinstance(organizer, dict):
        addr = organizer.get("email") or organizer.get("emailAddress")
        if isinstance(addr, dict):
            addr = addr.get("address")
        if addr:
            return str(addr)
    return None


def _extract_times(event: Dict[str, Any]) -> Tuple[Optional[str], Optional[str]]:
    start = event.get("start")
    end = event.get("end")
    if isinstance(start, dict):
        start = start.get("dateTime") or start.get("date")
    if isinstance(end, dict):
        end = end.get("dateTime") or end.get("date")
    return (str(start) if start else None, str(end) if end else None)


def meeting_prep_brief(
    db: Session,
    user_id: str,
    event_id: str,
    provider: Optional[str] = None,
) -> Dict[str, Any]:
    event = get_event_by_id(db=db, user_id=user_id, event_id=event_id, provider=provider)
    if not event:
        raise RuntimeError("Event not found")

    attendees = _extract_attendees(event)
    organizer = _extract_organizer(event)
    start, end = _extract_times(event)
    location = event.get("location")

    related_emails = []
    subject = event.get("summary") or ""
    if subject:
        try:
            related_emails = search_emails(
                db=db,
                user_id=user_id,
                query=subject,
                max_results=5,
                provider=provider,
                include_body=False,
            )
        except Exception:
            related_emails = []

    prompt = {
        "event": {
            "title": event.get("summary") or "",
            "start": start,
            "end": end,
            "location": location,
            "organizer": organizer,
            "attendees": attendees,
            "description": event.get("description"),
        },
        "related_emails": [
            {
                "from": e.get("from"),
                "subject": e.get("subject"),
                "snippet": e.get("snippet"),
            }
            for e in related_emails[:5]
        ],
    }

    system = (
        "You are an executive assistant preparing a meeting brief. "
        "Return ONLY valid JSON with keys: brief, checklist, questions. "
        "brief is a concise summary. checklist and questions are lists of strings."
    )

    try:
        resp = client.chat.completions.create(
            model=settings.OPENAI_MODEL,
            messages=[
                {"role": "system", "content": system},
                {"role": "user", "content": json.dumps(prompt)},
            ],
            temperature=0.2,
        )
        raw = resp.choices[0].message.content or "{}"
        data = json.loads(raw)
        return {
            "event": event,
            "brief": data.get("brief") or "Meeting brief ready.",
            "checklist": data.get("checklist") or [],
            "questions": data.get("questions") or [],
            "related_emails": related_emails,
        }
    except Exception as exc:
        logger.warning("Meeting prep brief failed: %s", exc)
        return {
            "event": event,
            "brief": "Meeting brief unavailable. Please try again.",
            "checklist": [],
            "questions": [],
            "related_emails": related_emails,
        }


def suggest_buffer(
    db: Session,
    user_id: str,
    start_utc: datetime,
    end_utc: datetime,
    location: Optional[str] = None,
    provider: Optional[str] = None,
) -> Dict[str, Any]:
    prefs = get_profile(db, user_id)
    buffer_minutes = _default_buffer_minutes(prefs, location)

    window_start = start_utc.replace(tzinfo=timezone.utc)
    window_end = end_utc.replace(tzinfo=timezone.utc)

    events = list_events_in_range(
        db=db,
        user_id=user_id,
        start_utc=window_start,
        end_utc=window_end,
        provider=provider,
        max_results=50,
    )
    conflicts = find_conflicts(events, window_start, window_end)

    return {
        "buffer_minutes": buffer_minutes,
        "virtual": _is_virtual_location(location),
        "conflicts": [
            {
                "id": e.get("id"),
                "summary": e.get("summary"),
                "start": e.get("start"),
                "end": e.get("end"),
            }
            for e in conflicts
        ],
        "suggested_start": window_start.isoformat(),
        "suggested_end": window_end.isoformat(),
    }


def generate_followup(
    db: Session,
    user_id: str,
    event_id: str,
    notes: Optional[str] = None,
    provider: Optional[str] = None,
) -> Dict[str, Any]:
    event = get_event_by_id(db=db, user_id=user_id, event_id=event_id, provider=provider)
    if not event:
        raise RuntimeError("Event not found")

    attendees = _extract_attendees(event)
    organizer = _extract_organizer(event)
    subject = event.get("summary") or "Meeting follow-up"

    profile = get_profile(db, user_id)
    user_email = profile.get("email")

    to_email = organizer or next((a for a in attendees if a and a != user_email), None) or ""

    prompt = {
        "event": {
            "title": subject,
            "start": _extract_times(event)[0],
            "end": _extract_times(event)[1],
            "attendees": attendees,
            "organizer": organizer,
            "location": event.get("location"),
        },
        "notes": notes or "",
    }

    system = (
        "You are an executive assistant creating a meeting follow-up. "
        "Return ONLY valid JSON with keys: tasks (list of strings), email_subject, email_body."
    )

    resp = client.chat.completions.create(
        model=settings.OPENAI_MODEL,
        messages=[
            {"role": "system", "content": system},
            {"role": "user", "content": json.dumps(prompt)},
        ],
        temperature=0.3,
    )
    raw = resp.choices[0].message.content or "{}"
    data = json.loads(raw)

    return {
        "event": event,
        "to_email": to_email,
        "tasks": data.get("tasks") or [],
        "email_subject": data.get("email_subject") or f"Re: {subject}",
        "email_body": data.get("email_body") or "",
    }
