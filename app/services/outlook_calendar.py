# backend/app/services/outlook_calendar.py

from __future__ import annotations

from datetime import datetime, timedelta, timezone
import logging
from typing import Any, Dict, List, Optional

import httpx
from sqlalchemy.orm import Session

from app.services.microsoft_oauth import get_valid_microsoft_access_token

logger = logging.getLogger(__name__)


GRAPH_BASE_URL = "https://graph.microsoft.com/v1.0"


def _auth_headers(access_token: str) -> Dict[str, str]:
    return {
        "Authorization": f"Bearer {access_token}",
        "Prefer": 'outlook.timezone="UTC"',
    }


def create_outlook_event(
    db: Session,
    user_id: str,
    title: str,
    start_utc: datetime,
    end_utc: datetime,
    description: Optional[str] = None,
    location: Optional[str] = None,
    reminders_minutes: Optional[List[int]] = None,
    attendees: Optional[List[str]] = None,
) -> Dict[str, Any]:
    access_token = get_valid_microsoft_access_token(db=db, user_id=user_id)
    if not access_token:
        raise RuntimeError("Microsoft not connected. Ask the user to connect first.")

    body: Dict[str, Any] = {
        "subject": title,
        "start": {"dateTime": start_utc.astimezone(timezone.utc).isoformat(), "timeZone": "UTC"},
        "end": {"dateTime": end_utc.astimezone(timezone.utc).isoformat(), "timeZone": "UTC"},
    }
    if description:
        body["body"] = {"contentType": "HTML", "content": description}
    if location:
        body["location"] = {"displayName": location}
    if reminders_minutes:
        body["isReminderOn"] = True
        body["reminderMinutesBeforeStart"] = int(reminders_minutes[0])
    if attendees:
        body["attendees"] = [
            {"emailAddress": {"address": email}, "type": "required"} for email in attendees
        ]

    resp = httpx.post(
        f"{GRAPH_BASE_URL}/me/events",
        headers=_auth_headers(access_token),
        json=body,
        timeout=15.0,
    )
    if resp.status_code >= 400:
        raise RuntimeError(f"Microsoft calendar create failed: {resp.text}")

    event = resp.json()
    return {
        "id": event.get("id"),
        "webLink": event.get("webLink"),
        "summary": event.get("subject"),
        "start": event.get("start"),
        "end": event.get("end"),
        "location": (event.get("location") or {}).get("displayName"),
    }


def update_outlook_event(
    db: Session,
    user_id: str,
    event_id: str,
    title: Optional[str] = None,
    start_utc: Optional[datetime] = None,
    end_utc: Optional[datetime] = None,
    description: Optional[str] = None,
    location: Optional[str] = None,
    reminders_minutes: Optional[List[int]] = None,
    attendees: Optional[List[str]] = None,
) -> Dict[str, Any]:
    access_token = get_valid_microsoft_access_token(db=db, user_id=user_id)
    if not access_token:
        raise RuntimeError("Microsoft not connected. Ask the user to connect first.")

    body: Dict[str, Any] = {}
    if title is not None:
        body["subject"] = title
    if start_utc is not None:
        body["start"] = {"dateTime": start_utc.astimezone(timezone.utc).isoformat(), "timeZone": "UTC"}
    if end_utc is not None:
        body["end"] = {"dateTime": end_utc.astimezone(timezone.utc).isoformat(), "timeZone": "UTC"}
    if description is not None:
        body["body"] = {"contentType": "HTML", "content": description}
    if location is not None:
        body["location"] = {"displayName": location}
    if reminders_minutes is not None:
        body["isReminderOn"] = True
        body["reminderMinutesBeforeStart"] = int(reminders_minutes[0]) if reminders_minutes else 15
    if attendees is not None:
        body["attendees"] = [
            {"emailAddress": {"address": email}, "type": "required"} for email in attendees
        ]

    if not body:
        raise ValueError("No fields provided for update.")

    headers = _auth_headers(access_token)
    headers["Prefer"] = 'outlook.timezone="UTC", return=representation'
    resp = httpx.patch(
        f"{GRAPH_BASE_URL}/me/events/{event_id}",
        headers=headers,
        json=body,
        timeout=15.0,
    )
    if resp.status_code >= 400:
        raise RuntimeError(f"Microsoft calendar update failed: {resp.text}")

    if resp.status_code == 204:
        return {"id": event_id, "summary": title}

    event = resp.json()
    return {
        "id": event.get("id"),
        "webLink": event.get("webLink"),
        "summary": event.get("subject"),
        "start": event.get("start"),
        "end": event.get("end"),
        "location": (event.get("location") or {}).get("displayName"),
    }


def list_events_in_range(
    db: Session,
    user_id: str,
    start_utc: datetime,
    end_utc: datetime,
    max_results: int = 20,
) -> List[Dict[str, Any]]:
    access_token = get_valid_microsoft_access_token(db=db, user_id=user_id)
    if not access_token:
        raise RuntimeError("Microsoft not connected. Ask the user to connect first.")

    params = {
        "startDateTime": start_utc.astimezone(timezone.utc).isoformat(),
        "endDateTime": end_utc.astimezone(timezone.utc).isoformat(),
        "$top": max_results,
        "$orderby": "start/dateTime",
    }
    resp = httpx.get(
        f"{GRAPH_BASE_URL}/me/calendarView",
        headers=_auth_headers(access_token),
        params=params,
        timeout=15.0,
    )
    if resp.status_code >= 400:
        raise RuntimeError(f"Microsoft calendar list failed: {resp.text}")

    items = (resp.json() or {}).get("value", [])
    out = []
    for e in items:
        out.append(
            {
                "id": e.get("id"),
                "summary": e.get("subject"),
                "start": e.get("start"),
                "end": e.get("end"),
                "webLink": e.get("webLink"),
                "location": (e.get("location") or {}).get("displayName"),
                "attendees": [
                    {
                        "email": (a.get("emailAddress") or {}).get("address"),
                        "responseStatus": (a.get("status") or {}).get("response"),
                    }
                    for a in (e.get("attendees") or [])
                ],
            }
        )
    return out


def list_upcoming_outlook_events(
    db: Session,
    user_id: str,
    max_results: int = 10,
) -> List[Dict[str, Any]]:
    now = datetime.now(timezone.utc)
    window_end = now + timedelta(days=30)
    return list_events_in_range(
        db=db,
        user_id=user_id,
        start_utc=now,
        end_utc=window_end,
        max_results=max_results,
    )


def get_outlook_events_for_daily_brief(
    db: Session,
    user_id: str,
    days_ahead: int = 1,
) -> List[Dict[str, Any]]:
    access_token = get_valid_microsoft_access_token(db=db, user_id=user_id)
    if not access_token:
        return []


def get_outlook_event(
    db: Session,
    user_id: str,
    event_id: str,
) -> Optional[Dict[str, Any]]:
    access_token = get_valid_microsoft_access_token(db=db, user_id=user_id)
    if not access_token:
        raise RuntimeError("Microsoft not connected. Ask the user to connect first.")

    resp = httpx.get(
        f"{GRAPH_BASE_URL}/me/events/{event_id}",
        headers=_auth_headers(access_token),
        timeout=15.0,
    )
    if resp.status_code >= 400:
        logger.error("Microsoft event fetch failed: %s", resp.text)
        return None

    event = resp.json() or {}
    return {
        "id": event.get("id"),
        "summary": event.get("subject"),
        "start": event.get("start"),
        "end": event.get("end"),
        "webLink": event.get("webLink"),
        "location": (event.get("location") or {}).get("displayName"),
        "description": (event.get("body") or {}).get("content"),
        "attendees": [
            (a.get("emailAddress") or {}).get("address")
            for a in (event.get("attendees") or [])
            if isinstance(a, dict)
        ],
        "organizer": (event.get("organizer") or {}).get("emailAddress"),
        "provider": "microsoft",
    }

    try:
        now = datetime.now(timezone.utc)
        window_end = now + timedelta(days=days_ahead)
        events = list_events_in_range(
            db=db,
            user_id=user_id,
            start_utc=now,
            end_utc=window_end,
            max_results=20,
        )
        simplified = []
        for event in events:
            start = event.get("start") or {}
            end = event.get("end") or {}
            simplified.append(
                {
                    "id": event.get("id"),
                    "summary": event.get("summary") or "No title",
                    "description": None,
                    "start": start.get("dateTime"),
                    "end": end.get("dateTime"),
                    "location": event.get("location"),
                    "attendees": event.get("attendees") or [],
                    "link": event.get("webLink"),
                    "is_all_day": False,
                }
            )
        return simplified
    except Exception as e:
        logger.error("Error fetching Outlook events for daily brief (user %s): %s", user_id, e)
        return []
