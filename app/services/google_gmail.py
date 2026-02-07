# backend/app/services/google_gmail.py

from __future__ import annotations

import base64
from email.message import EmailMessage
import logging
from typing import Any, Dict, List, Optional

from googleapiclient.discovery import build
from sqlalchemy.orm import Session

from app.services.google_oauth import get_valid_google_credentials

logger = logging.getLogger(__name__)


def send_email(
    db: Session,
    user_id: str,
    to_email: str,
    subject: str,
    body_text: str,
    cc: Optional[str] = None,
    bcc: Optional[str] = None,
) -> Dict[str, Any]:
    """
    Sends an email immediately using the user's connected Google account.
    Requires the user to have completed OAuth connection first.
    """
    creds = get_valid_google_credentials(db=db, user_id=user_id)
    if creds is None:
        raise RuntimeError("Google not connected. Ask the user to connect first.")

    service = build("gmail", "v1", credentials=creds)

    msg = EmailMessage()
    msg["To"] = to_email
    msg["Subject"] = subject
    if cc:
        msg["Cc"] = cc
    if bcc:
        msg["Bcc"] = bcc
    msg.set_content(body_text)

    encoded = base64.urlsafe_b64encode(msg.as_bytes()).decode("utf-8")
    sent = service.users().messages().send(userId="me", body={"raw": encoded}).execute()
    return {"id": sent.get("id"), "threadId": sent.get("threadId")}


def create_draft(
    db: Session,
    user_id: str,
    to_email: str,
    subject: str,
    body_text: str,
    cc: Optional[str] = None,
    bcc: Optional[str] = None,
) -> Dict[str, Any]:
    """
    Creates a Gmail draft (does NOT send).
    Requires the user to have completed OAuth connection first.
    """
    creds = get_valid_google_credentials(db=db, user_id=user_id)
    if creds is None:
        raise RuntimeError("Google not connected. Ask the user to connect first.")

    service = build("gmail", "v1", credentials=creds)

    msg = EmailMessage()
    msg["To"] = to_email
    msg["Subject"] = subject
    if cc:
        msg["Cc"] = cc
    if bcc:
        msg["Bcc"] = bcc
    msg.set_content(body_text)

    encoded = base64.urlsafe_b64encode(msg.as_bytes()).decode("utf-8")
    draft = service.users().drafts().create(userId="me", body={"message": {"raw": encoded}}).execute()
    return {"id": draft.get("id"), "messageId": (draft.get("message") or {}).get("id")}


def get_recent_emails_for_daily_brief(
    db: Session,
    user_id: str,
    max_results: int = 10,
    hours_back: int = 24,
) -> List[Dict[str, Any]]:
    """
    Fetch recent emails for the daily brief.

    Args:
        db: Database session
        user_id: User ID
        max_results: Maximum number of emails to fetch
        hours_back: How many hours back to look for emails

    Returns:
        List of email summaries with sender, subject, snippet
    """
    creds = get_valid_google_credentials(db=db, user_id=user_id)
    if not creds:
        return []

    try:
        service = build("gmail", "v1", credentials=creds)

        # Query for unread emails in inbox from last N hours
        # You can customize this query based on your needs
        query = f"in:inbox is:unread newer_than:{hours_back}h"

        # List messages
        results = (
            service.users()
            .messages()
            .list(userId="me", q=query, maxResults=max_results)
            .execute()
        )

        messages = results.get("messages", [])

        email_summaries = []
        for msg in messages:
            # Get full message details
            msg_data = (
                service.users()
                .messages()
                .get(userId="me", id=msg["id"], format="metadata", metadataHeaders=["From", "Subject", "Date"])
                .execute()
            )

            headers = {h["name"]: h["value"] for h in msg_data.get("payload", {}).get("headers", [])}

            email_summaries.append({
                "id": msg_data.get("id"),
                "thread_id": msg_data.get("threadId"),
                "from": headers.get("From", "Unknown"),
                "subject": headers.get("Subject", "No subject"),
                "date": headers.get("Date"),
                "snippet": msg_data.get("snippet", ""),
                "labels": msg_data.get("labelIds", []),
            })

        return email_summaries

    except Exception as e:
        logger.error("Error fetching emails for daily brief (user %s): %s", user_id, e)
        return []
