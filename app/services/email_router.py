# backend/app/services/email_router.py

from __future__ import annotations

import logging
from typing import Any, Dict, List, Optional

from sqlalchemy.orm import Session

from app.services.google_oauth import get_google_connection_status
from app.services.microsoft_oauth import get_microsoft_connection_status
from app.services.preferences import get_preferences
from app.services.google_gmail import (
    send_email as gmail_send_email,
    create_draft as gmail_create_draft,
    search_gmail_messages,
    list_recent_gmail_messages,
)
from app.services.outlook_mail import (
    send_outlook_email,
    create_outlook_draft,
    search_outlook_messages,
    list_recent_outlook_messages,
    get_recent_outlook_emails_for_daily_brief,
)

logger = logging.getLogger(__name__)


PROVIDER_ORDER = ["google", "microsoft"]


def get_email_connections(db: Session, user_id: str) -> Dict[str, bool]:
    google_status = get_google_connection_status(db=db, user_id=user_id)
    ms_status = get_microsoft_connection_status(db=db, user_id=user_id)
    return {
        "google": bool(google_status.get("connected")),
        "microsoft": bool(ms_status.get("connected")),
    }


def pick_email_provider(db: Session, user_id: str, preferred: Optional[str] = None) -> Optional[str]:
    connections = get_email_connections(db, user_id)
    if preferred and connections.get(preferred):
        return preferred

    prefs = get_preferences(db, user_id)
    preferred_pref = prefs.get("email_provider")
    if preferred_pref and connections.get(preferred_pref):
        return preferred_pref

    for provider in PROVIDER_ORDER:
        if connections.get(provider):
            return provider
    return None


def send_email(
    db: Session,
    user_id: str,
    to_email: str,
    subject: str,
    body_text: str,
    cc: Optional[str] = None,
    bcc: Optional[str] = None,
    provider: Optional[str] = None,
) -> Dict[str, Any]:
    provider = pick_email_provider(db, user_id, preferred=provider)
    if not provider:
        raise RuntimeError("No email provider connected. Ask the user to connect Google or Outlook.")

    if provider == "google":
        return gmail_send_email(
            db=db,
            user_id=user_id,
            to_email=to_email,
            subject=subject,
            body_text=body_text,
            cc=cc,
            bcc=bcc,
        )
    if provider == "microsoft":
        return send_outlook_email(
            db=db,
            user_id=user_id,
            to_email=to_email,
            subject=subject,
            body_text=body_text,
            cc=cc,
            bcc=bcc,
        )
    raise RuntimeError(f"Unsupported email provider: {provider}")


def create_draft(
    db: Session,
    user_id: str,
    to_email: str,
    subject: str,
    body_text: str,
    cc: Optional[str] = None,
    bcc: Optional[str] = None,
    provider: Optional[str] = None,
) -> Dict[str, Any]:
    provider = pick_email_provider(db, user_id, preferred=provider)
    if not provider:
        raise RuntimeError("No email provider connected. Ask the user to connect Google or Outlook.")

    if provider == "google":
        return gmail_create_draft(
            db=db,
            user_id=user_id,
            to_email=to_email,
            subject=subject,
            body_text=body_text,
            cc=cc,
            bcc=bcc,
        )
    if provider == "microsoft":
        return create_outlook_draft(
            db=db,
            user_id=user_id,
            to_email=to_email,
            subject=subject,
            body_text=body_text,
            cc=cc,
            bcc=bcc,
        )
    raise RuntimeError(f"Unsupported email provider: {provider}")


def search_emails(
    db: Session,
    user_id: str,
    query: str,
    max_results: int = 10,
    provider: Optional[str] = None,
    include_body: bool = False,
) -> List[Dict[str, Any]]:
    provider = pick_email_provider(db, user_id, preferred=provider)
    if not provider:
        raise RuntimeError("No email provider connected. Ask the user to connect Google or Outlook.")

    if provider == "google":
        results = search_gmail_messages(
            db=db,
            user_id=user_id,
            query=query,
            max_results=max_results,
            include_body=include_body,
        )
        for msg in results:
            msg["provider"] = "google"
        return results
    if provider == "microsoft":
        results = search_outlook_messages(
            db=db,
            user_id=user_id,
            query=query,
            max_results=max_results,
            include_body=include_body,
        )
        for msg in results:
            msg["provider"] = "microsoft"
        return results
    raise RuntimeError(f"Unsupported email provider: {provider}")


def list_recent_emails(
    db: Session,
    user_id: str,
    max_results: int = 10,
    hours_back: int = 24,
    unread_only: bool = True,
    provider: Optional[str] = None,
    include_body: bool = False,
) -> List[Dict[str, Any]]:
    provider = pick_email_provider(db, user_id, preferred=provider)
    if not provider:
        return []

    if provider == "google":
        results = list_recent_gmail_messages(
            db=db,
            user_id=user_id,
            max_results=max_results,
            hours_back=hours_back,
            unread_only=unread_only,
            include_body=include_body,
        )
        for msg in results:
            msg["provider"] = "google"
        return results
    if provider == "microsoft":
        results = list_recent_outlook_messages(
            db=db,
            user_id=user_id,
            max_results=max_results,
            hours_back=hours_back,
            unread_only=unread_only,
            include_body=include_body,
        )
        for msg in results:
            msg["provider"] = "microsoft"
        return results
    return []


def get_recent_emails_for_daily_brief(
    db: Session,
    user_id: str,
    max_results: int = 10,
    hours_back: int = 24,
) -> List[Dict[str, Any]]:
    emails: List[Dict[str, Any]] = []
    try:
        results = list_recent_gmail_messages(
                db=db,
                user_id=user_id,
                max_results=max_results,
                hours_back=hours_back,
                unread_only=True,
                include_body=False,
        )
        for msg in results:
            msg["provider"] = "google"
        emails.extend(results)
    except Exception as e:
        logger.warning("Gmail daily brief fetch failed: %s", e)
    try:
        results = get_recent_outlook_emails_for_daily_brief(
                db=db,
                user_id=user_id,
                max_results=max_results,
                hours_back=hours_back,
        )
        for msg in results:
            msg["provider"] = "microsoft"
        emails.extend(results)
    except Exception as e:
        logger.warning("Outlook daily brief fetch failed: %s", e)
    return emails
