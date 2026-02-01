# backend/app/services/google_gmail.py

from __future__ import annotations

import base64
from email.message import EmailMessage
from typing import Any, Dict, Optional

from googleapiclient.discovery import build
from sqlalchemy.orm import Session

from app.services.google_oauth import get_valid_google_credentials


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