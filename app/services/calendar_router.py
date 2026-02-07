# backend/app/services/calendar_router.py

from __future__ import annotations

import logging
from datetime import datetime
from typing import Any, Dict, List, Optional

from sqlalchemy.orm import Session

from app.services.google_oauth import get_google_connection_status
from app.services.microsoft_oauth import get_microsoft_connection_status
from app.services.integration_credentials import get_connection_status as get_integration_status
from app.services.preferences import get_preferences
from app.services.google_calendar import create_calendar_event, update_calendar_event, list_events_in_range as google_list_range
from app.services.outlook_calendar import create_outlook_event, update_outlook_event, list_events_in_range as outlook_list_range
from app.services.caldav_calendar import create_caldav_event, list_events_in_range as caldav_list_range
from app.services.google_calendar import get_events_for_daily_brief as google_brief
from app.services.outlook_calendar import get_outlook_events_for_daily_brief as outlook_brief
from app.services.caldav_calendar import get_caldav_events_for_daily_brief as caldav_brief


logger = logging.getLogger(__name__)


PROVIDER_ORDER = ["google", "microsoft", "caldav"]


def get_calendar_connections(db: Session, user_id: str) -> Dict[str, bool]:
    google_status = get_google_connection_status(db=db, user_id=user_id)
    ms_status = get_microsoft_connection_status(db=db, user_id=user_id)
    caldav_status = get_integration_status(db=db, user_id=user_id, provider="caldav")
    return {
        "google": bool(google_status.get("connected")),
        "microsoft": bool(ms_status.get("connected")),
        "caldav": bool(caldav_status.get("connected")),
    }


def pick_calendar_provider(db: Session, user_id: str, preferred: Optional[str] = None) -> Optional[str]:
    connections = get_calendar_connections(db, user_id)
    if preferred and connections.get(preferred):
        return preferred

    prefs = get_preferences(db, user_id)
    preferred_pref = prefs.get("calendar_provider")
    if preferred_pref and connections.get(preferred_pref):
        return preferred_pref

    for provider in PROVIDER_ORDER:
        if connections.get(provider):
            return provider
    return None


def create_event(
    db: Session,
    user_id: str,
    title: str,
    start_utc: datetime,
    end_utc: datetime,
    description: Optional[str] = None,
    location: Optional[str] = None,
    reminders_minutes: Optional[List[int]] = None,
    attendees: Optional[List[str]] = None,
    provider: Optional[str] = None,
) -> Dict[str, Any]:
    provider = pick_calendar_provider(db, user_id, preferred=provider)
    if not provider:
        raise RuntimeError("No calendar connected. Ask the user to connect Google, Outlook, or CalDAV.")

    if provider == "google":
        return create_calendar_event(
            db=db,
            user_id=user_id,
            title=title,
            start_utc=start_utc,
            end_utc=end_utc,
            description=description,
            location=location,
            reminders_minutes=reminders_minutes,
        )
    if provider == "microsoft":
        return create_outlook_event(
            db=db,
            user_id=user_id,
            title=title,
            start_utc=start_utc,
            end_utc=end_utc,
            description=description,
            location=location,
            reminders_minutes=reminders_minutes,
            attendees=attendees,
        )
    if provider == "caldav":
        return create_caldav_event(
            db=db,
            user_id=user_id,
            title=title,
            start_utc=start_utc,
            end_utc=end_utc,
            description=description,
            location=location,
        )

    raise RuntimeError(f"Unsupported calendar provider: {provider}")


def list_events_in_range(
    db: Session,
    user_id: str,
    start_utc: datetime,
    end_utc: datetime,
    provider: Optional[str] = None,
    max_results: int = 20,
) -> List[Dict[str, Any]]:
    if provider:
        if provider == "google":
            return google_list_range(db=db, user_id=user_id, start_utc=start_utc, end_utc=end_utc, max_results=max_results)
        if provider == "microsoft":
            return outlook_list_range(db=db, user_id=user_id, start_utc=start_utc, end_utc=end_utc, max_results=max_results)
        if provider == "caldav":
            return caldav_list_range(db=db, user_id=user_id, start_utc=start_utc, end_utc=end_utc)
        raise RuntimeError(f"Unsupported calendar provider: {provider}")

    connections = get_calendar_connections(db, user_id)
    events: List[Dict[str, Any]] = []
    if connections.get("google"):
        try:
            events.extend(google_list_range(db=db, user_id=user_id, start_utc=start_utc, end_utc=end_utc, max_results=max_results))
        except Exception as e:
            logger.warning("Google calendar fetch failed: %s", e)
    if connections.get("microsoft"):
        try:
            events.extend(outlook_list_range(db=db, user_id=user_id, start_utc=start_utc, end_utc=end_utc, max_results=max_results))
        except Exception as e:
            logger.warning("Microsoft calendar fetch failed: %s", e)
    if connections.get("caldav"):
        try:
            events.extend(caldav_list_range(db=db, user_id=user_id, start_utc=start_utc, end_utc=end_utc))
        except Exception as e:
            logger.warning("CalDAV calendar fetch failed: %s", e)
    return events


def update_event(
    db: Session,
    user_id: str,
    event_id: str,
    provider: Optional[str] = None,
    title: Optional[str] = None,
    start_utc: Optional[datetime] = None,
    end_utc: Optional[datetime] = None,
    description: Optional[str] = None,
    location: Optional[str] = None,
    reminders_minutes: Optional[List[int]] = None,
    attendees: Optional[List[str]] = None,
) -> Dict[str, Any]:
    provider = pick_calendar_provider(db, user_id, preferred=provider)
    if not provider:
        raise RuntimeError("No calendar connected. Ask the user to connect Google, Outlook, or CalDAV.")

    if provider == "google":
        return update_calendar_event(
            db=db,
            user_id=user_id,
            event_id=event_id,
            title=title,
            start_utc=start_utc,
            end_utc=end_utc,
            description=description,
            location=location,
            reminders_minutes=reminders_minutes,
        )
    if provider == "microsoft":
        return update_outlook_event(
            db=db,
            user_id=user_id,
            event_id=event_id,
            title=title,
            start_utc=start_utc,
            end_utc=end_utc,
            description=description,
            location=location,
            reminders_minutes=reminders_minutes,
            attendees=attendees,
        )
    if provider == "caldav":
        raise RuntimeError("CalDAV update is not supported yet.")

    raise RuntimeError(f"Unsupported calendar provider: {provider}")


def get_events_for_daily_brief(
    db: Session,
    user_id: str,
    days_ahead: int = 1,
) -> List[Dict[str, Any]]:
    events: List[Dict[str, Any]] = []
    try:
        events.extend(google_brief(db=db, user_id=user_id, days_ahead=days_ahead))
    except Exception as e:
        logger.warning("Google daily brief events failed: %s", e)
    try:
        events.extend(outlook_brief(db=db, user_id=user_id, days_ahead=days_ahead))
    except Exception as e:
        logger.warning("Outlook daily brief events failed: %s", e)
    try:
        events.extend(caldav_brief(db=db, user_id=user_id, days_ahead=days_ahead))
    except Exception as e:
        logger.warning("CalDAV daily brief events failed: %s", e)
    return events
