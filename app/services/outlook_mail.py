# backend/app/services/outlook_mail.py

from __future__ import annotations

from datetime import datetime, timedelta, timezone
import logging
from typing import Any, Dict, List, Optional

import httpx
from sqlalchemy.orm import Session

from app.services.microsoft_oauth import get_valid_microsoft_access_token

logger = logging.getLogger(__name__)


GRAPH_BASE_URL = "https://graph.microsoft.com/v1.0"


def _auth_headers(access_token: str, include_body: bool = False) -> Dict[str, str]:
    headers = {
        "Authorization": f"Bearer {access_token}",
        "ConsistencyLevel": "eventual",
    }
    if include_body:
        headers["Prefer"] = 'outlook.body-content-type="text"'
    return headers


def _build_message(
    to_email: str,
    subject: str,
    body_text: str,
    cc: Optional[str] = None,
    bcc: Optional[str] = None,
) -> Dict[str, Any]:
    msg: Dict[str, Any] = {
        "subject": subject,
        "body": {"contentType": "Text", "content": body_text},
        "toRecipients": [{"emailAddress": {"address": to_email}}],
    }
    if cc:
        msg["ccRecipients"] = [{"emailAddress": {"address": e.strip()}} for e in cc.split(",") if e.strip()]
    if bcc:
        msg["bccRecipients"] = [{"emailAddress": {"address": e.strip()}} for e in bcc.split(",") if e.strip()]
    return msg


def send_outlook_email(
    db: Session,
    user_id: str,
    to_email: str,
    subject: str,
    body_text: str,
    cc: Optional[str] = None,
    bcc: Optional[str] = None,
) -> Dict[str, Any]:
    access_token = get_valid_microsoft_access_token(db=db, user_id=user_id)
    if not access_token:
        raise RuntimeError("Microsoft not connected. Ask the user to connect first.")

    payload = {
        "message": _build_message(to_email, subject, body_text, cc=cc, bcc=bcc),
        "saveToSentItems": True,
    }
    resp = httpx.post(
        f"{GRAPH_BASE_URL}/me/sendMail",
        headers=_auth_headers(access_token),
        json=payload,
        timeout=15.0,
    )
    if resp.status_code >= 400:
        raise RuntimeError(f"Microsoft sendMail failed: {resp.text}")
    return {"status": "accepted"}


def create_outlook_draft(
    db: Session,
    user_id: str,
    to_email: str,
    subject: str,
    body_text: str,
    cc: Optional[str] = None,
    bcc: Optional[str] = None,
) -> Dict[str, Any]:
    access_token = get_valid_microsoft_access_token(db=db, user_id=user_id)
    if not access_token:
        raise RuntimeError("Microsoft not connected. Ask the user to connect first.")

    msg = _build_message(to_email, subject, body_text, cc=cc, bcc=bcc)
    resp = httpx.post(
        f"{GRAPH_BASE_URL}/me/messages",
        headers=_auth_headers(access_token, include_body=True),
        json=msg,
        timeout=15.0,
    )
    if resp.status_code >= 400:
        raise RuntimeError(f"Microsoft draft create failed: {resp.text}")
    data = resp.json()
    return {"id": data.get("id"), "conversationId": data.get("conversationId")}


def _parse_message(msg: Dict[str, Any], include_body: bool = False) -> Dict[str, Any]:
    sender = msg.get("from") or {}
    sender_addr = (sender.get("emailAddress") or {}).get("address")
    body = None
    if include_body:
        body = (msg.get("body") or {}).get("content")
    return {
        "id": msg.get("id"),
        "thread_id": msg.get("conversationId"),
        "from": sender_addr or "Unknown",
        "subject": msg.get("subject") or "No subject",
        "date": msg.get("receivedDateTime"),
        "snippet": msg.get("bodyPreview") or "",
        "body": body,
        "labels": ["unread"] if msg.get("isRead") is False else [],
    }


def search_outlook_messages(
    db: Session,
    user_id: str,
    query: str,
    max_results: int = 10,
    include_body: bool = False,
) -> List[Dict[str, Any]]:
    access_token = get_valid_microsoft_access_token(db=db, user_id=user_id)
    if not access_token:
        raise RuntimeError("Microsoft not connected. Ask the user to connect first.")

    params = {
        "$search": f"\"{query}\"",
        "$top": max_results,
    }
    if include_body:
        params["$select"] = "id,conversationId,from,subject,receivedDateTime,bodyPreview,isRead,body"

    resp = httpx.get(
        f"{GRAPH_BASE_URL}/me/messages",
        headers=_auth_headers(access_token, include_body=include_body),
        params=params,
        timeout=15.0,
    )
    if resp.status_code >= 400:
        raise RuntimeError(f"Microsoft message search failed: {resp.text}")

    items = (resp.json() or {}).get("value", [])
    return [_parse_message(m, include_body=include_body) for m in items]


def list_recent_outlook_messages(
    db: Session,
    user_id: str,
    max_results: int = 10,
    hours_back: int = 24,
    unread_only: bool = True,
    include_body: bool = False,
) -> List[Dict[str, Any]]:
    access_token = get_valid_microsoft_access_token(db=db, user_id=user_id)
    if not access_token:
        return []

    since = (datetime.now(timezone.utc) - timedelta(hours=hours_back)).isoformat()
    filters = [f"receivedDateTime ge {since}"]
    if unread_only:
        filters.append("isRead eq false")

    params = {
        "$filter": " and ".join(filters),
        "$orderby": "receivedDateTime desc",
        "$top": max_results,
    }
    if include_body:
        params["$select"] = "id,conversationId,from,subject,receivedDateTime,bodyPreview,isRead,body"

    resp = httpx.get(
        f"{GRAPH_BASE_URL}/me/mailFolders/inbox/messages",
        headers=_auth_headers(access_token, include_body=include_body),
        params=params,
        timeout=15.0,
    )
    if resp.status_code >= 400:
        logger.error("Microsoft recent messages failed: %s", resp.text)
        return []

    items = (resp.json() or {}).get("value", [])
    return [_parse_message(m, include_body=include_body) for m in items]


def get_recent_outlook_emails_for_daily_brief(
    db: Session,
    user_id: str,
    max_results: int = 10,
    hours_back: int = 24,
) -> List[Dict[str, Any]]:
    return list_recent_outlook_messages(
        db=db,
        user_id=user_id,
        max_results=max_results,
        hours_back=hours_back,
        unread_only=True,
        include_body=False,
    )
