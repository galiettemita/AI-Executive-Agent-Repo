# backend/app/services/google_gmail.py

from __future__ import annotations

import base64
import re
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


def _decode_base64(data: str) -> str:
    if not data:
        return ""
    padding = "=" * (-len(data) % 4)
    decoded = base64.urlsafe_b64decode((data + padding).encode("utf-8"))
    return decoded.decode("utf-8", errors="replace")


def _strip_html(html: str) -> str:
    if not html:
        return ""
    text = re.sub(r"<[^>]+>", " ", html)
    text = re.sub(r"\s+", " ", text).strip()
    return text


def _extract_body(payload: Dict[str, Any]) -> str:
    if not payload:
        return ""
    mime = payload.get("mimeType")
    body = payload.get("body") or {}
    data = body.get("data")
    if data:
        content = _decode_base64(data)
        if mime == "text/html":
            return _strip_html(content)
        return content

    parts = payload.get("parts") or []
    plain_parts = []
    html_parts = []
    for part in parts:
        part_mime = part.get("mimeType")
        if part_mime == "text/plain":
            plain_parts.append(_extract_body(part))
        elif part_mime == "text/html":
            html_parts.append(_extract_body(part))
        else:
            if part.get("parts"):
                nested = _extract_body(part)
                if nested:
                    plain_parts.append(nested)
    if plain_parts:
        return "\n".join(p for p in plain_parts if p)
    if html_parts:
        return "\n".join(p for p in html_parts if p)
    return ""


def _fetch_gmail_messages(
    db: Session,
    user_id: str,
    query: str,
    max_results: int = 10,
    include_body: bool = False,
) -> List[Dict[str, Any]]:
    creds = get_valid_google_credentials(db=db, user_id=user_id)
    if not creds:
        return []

    service = build("gmail", "v1", credentials=creds)
    results = service.users().messages().list(userId="me", q=query, maxResults=max_results).execute()
    messages = results.get("messages", [])

    email_summaries = []
    for msg in messages:
        msg_data = (
            service.users()
            .messages()
            .get(
                userId="me",
                id=msg["id"],
                format="full" if include_body else "metadata",
                metadataHeaders=["From", "Subject", "Date"],
            )
            .execute()
        )

        headers = {h["name"]: h["value"] for h in msg_data.get("payload", {}).get("headers", [])}
        body_text = _extract_body(msg_data.get("payload") or {}) if include_body else None

        email_summaries.append(
            {
                "id": msg_data.get("id"),
                "thread_id": msg_data.get("threadId"),
                "from": headers.get("From", "Unknown"),
                "subject": headers.get("Subject", "No subject"),
                "date": headers.get("Date"),
                "snippet": msg_data.get("snippet", ""),
                "body": body_text,
                "labels": msg_data.get("labelIds", []),
            }
        )
    return email_summaries


def search_gmail_messages(
    db: Session,
    user_id: str,
    query: str,
    max_results: int = 10,
    include_body: bool = False,
) -> List[Dict[str, Any]]:
    try:
        return _fetch_gmail_messages(
            db=db,
            user_id=user_id,
            query=query,
            max_results=max_results,
            include_body=include_body,
        )
    except Exception as e:
        logger.error("Error searching Gmail (user %s): %s", user_id, e)
        return []


def list_recent_gmail_messages(
    db: Session,
    user_id: str,
    max_results: int = 10,
    hours_back: int = 24,
    unread_only: bool = True,
    include_body: bool = False,
) -> List[Dict[str, Any]]:
    query_parts = ["in:inbox", f"newer_than:{hours_back}h"]
    if unread_only:
        query_parts.append("is:unread")
    query = " ".join(query_parts)
    try:
        return _fetch_gmail_messages(
            db=db,
            user_id=user_id,
            query=query,
            max_results=max_results,
            include_body=include_body,
        )
    except Exception as e:
        logger.error("Error fetching Gmail messages (user %s): %s", user_id, e)
        return []


def get_recent_emails_for_daily_brief(
    db: Session,
    user_id: str,
    max_results: int = 10,
    hours_back: int = 24,
) -> List[Dict[str, Any]]:
    return list_recent_gmail_messages(
        db=db,
        user_id=user_id,
        max_results=max_results,
        hours_back=hours_back,
        unread_only=True,
        include_body=False,
    )
