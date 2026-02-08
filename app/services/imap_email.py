from __future__ import annotations

import imaplib
import json
import logging
import smtplib
from datetime import datetime, timedelta
from email import message_from_bytes
from email.header import decode_header
from email.message import EmailMessage, Message
from email.utils import parseaddr, parsedate_to_datetime
from typing import Any, Dict, List, Optional, Tuple
from uuid import uuid4

from sqlalchemy.orm import Session

from app.db.models import IntegrationCredential
from app.services.integration_credentials import get_integration_credential, get_decrypted_secret

logger = logging.getLogger(__name__)

DEFAULT_PROVIDER_SETTINGS: Dict[str, Dict[str, Any]] = {
    "icloud": {
        "imap_host": "imap.mail.me.com",
        "imap_port": 993,
        "imap_ssl": True,
        "smtp_host": "smtp.mail.me.com",
        "smtp_port": 587,
        "smtp_ssl": False,
    },
    "yahoo": {
        "imap_host": "imap.mail.yahoo.com",
        "imap_port": 993,
        "imap_ssl": True,
        "smtp_host": "smtp.mail.yahoo.com",
        "smtp_port": 587,
        "smtp_ssl": False,
    },
}


class IMAPConfigError(RuntimeError):
    pass


def _load_metadata(row: IntegrationCredential) -> Dict[str, Any]:
    if not row or not row.metadata_json:
        return {}
    try:
        data = json.loads(row.metadata_json)
        return data if isinstance(data, dict) else {}
    except Exception:
        return {}


def resolve_imap_settings(provider: str, row: IntegrationCredential) -> Dict[str, Any]:
    defaults = DEFAULT_PROVIDER_SETTINGS.get(provider, {})
    metadata = _load_metadata(row)

    imap_host = metadata.get("imap_host") or row.server_url or defaults.get("imap_host")
    smtp_host = metadata.get("smtp_host") or defaults.get("smtp_host")
    if not imap_host or not smtp_host:
        raise IMAPConfigError("IMAP/SMTP host missing. Provide server settings in the connection metadata.")

    imap_port = int(metadata.get("imap_port") or defaults.get("imap_port") or 993)
    smtp_port = int(metadata.get("smtp_port") or defaults.get("smtp_port") or 587)

    if "imap_ssl" in metadata:
        imap_ssl = bool(metadata.get("imap_ssl"))
    else:
        imap_ssl = bool(defaults.get("imap_ssl", True))

    if "smtp_ssl" in metadata:
        smtp_ssl = bool(metadata.get("smtp_ssl"))
    else:
        smtp_ssl = bool(defaults.get("smtp_ssl", False))

    smtp_user = metadata.get("smtp_user") or row.username
    from_email = metadata.get("from_email") or row.username
    mailbox = metadata.get("mailbox") or "INBOX"

    return {
        "imap_host": imap_host,
        "imap_port": imap_port,
        "imap_ssl": imap_ssl,
        "smtp_host": smtp_host,
        "smtp_port": smtp_port,
        "smtp_ssl": smtp_ssl,
        "smtp_user": smtp_user,
        "from_email": from_email,
        "mailbox": mailbox,
    }


def _get_credentials(db: Session, user_id: str, provider: str) -> Tuple[IntegrationCredential, str, Dict[str, Any]]:
    row = get_integration_credential(db, user_id, provider)
    if not row or not row.username:
        raise RuntimeError("Email credentials not configured. Connect iCloud/Yahoo first.")
    secret = get_decrypted_secret(row)
    if not secret:
        raise RuntimeError("Email credentials missing password/app-specific password.")
    settings = resolve_imap_settings(provider, row)
    return row, secret, settings


def _imap_connect(username: str, password: str, settings: Dict[str, Any]) -> imaplib.IMAP4:
    host = settings["imap_host"]
    port = settings["imap_port"]
    if settings.get("imap_ssl", True):
        client = imaplib.IMAP4_SSL(host, port)
    else:
        client = imaplib.IMAP4(host, port)
    client.login(username, password)
    return client


def _smtp_send(
    username: str,
    password: str,
    settings: Dict[str, Any],
    message: EmailMessage,
) -> None:
    host = settings["smtp_host"]
    port = settings["smtp_port"]
    use_ssl = settings.get("smtp_ssl", False)

    if use_ssl:
        server: smtplib.SMTP = smtplib.SMTP_SSL(host, port)
    else:
        server = smtplib.SMTP(host, port)
        server.ehlo()
        server.starttls()

    server.login(settings.get("smtp_user") or username, password)
    server.send_message(message)
    server.quit()


def _decode_header_value(value: Optional[str]) -> str:
    if not value:
        return ""
    parts = decode_header(value)
    out = []
    for part, encoding in parts:
        if isinstance(part, bytes):
            try:
                out.append(part.decode(encoding or "utf-8", errors="ignore"))
            except Exception:
                out.append(part.decode("utf-8", errors="ignore"))
        else:
            out.append(part)
    return "".join(out)


def _extract_body(msg: Message) -> str:
    if msg.is_multipart():
        for part in msg.walk():
            if part.get_content_type() == "text/plain" and not part.get("Content-Disposition"):
                payload = part.get_payload(decode=True)
                if payload:
                    charset = part.get_content_charset() or "utf-8"
                    return payload.decode(charset, errors="ignore")
        for part in msg.walk():
            if part.get_content_type() == "text/html" and not part.get("Content-Disposition"):
                payload = part.get_payload(decode=True)
                if payload:
                    charset = part.get_content_charset() or "utf-8"
                    return payload.decode(charset, errors="ignore")
        return ""

    payload = msg.get_payload(decode=True)
    if payload:
        charset = msg.get_content_charset() or "utf-8"
        return payload.decode(charset, errors="ignore")
    return ""


def _parse_message(uid: str, raw_bytes: bytes, include_body: bool) -> Dict[str, Any]:
    msg = message_from_bytes(raw_bytes)
    subject = _decode_header_value(msg.get("Subject"))
    from_name, from_email = parseaddr(_decode_header_value(msg.get("From")))
    date_header = msg.get("Date")
    date_iso = None
    if date_header:
        try:
            dt = parsedate_to_datetime(date_header)
            date_iso = dt.isoformat()
        except Exception:
            date_iso = None

    body_text = _extract_body(msg) if include_body else ""
    snippet_source = body_text or subject or ""
    snippet = snippet_source.strip().replace("\r", " ").replace("\n", " ")[:280]

    return {
        "id": uid,
        "from": f"{from_name} <{from_email}>".strip() if from_email else from_name,
        "subject": subject,
        "date": date_iso,
        "snippet": snippet,
        "body": body_text if include_body else None,
    }


def verify_imap_connection(db: Session, user_id: str, provider: str) -> bool:
    row, secret, settings = _get_credentials(db, user_id, provider)
    client = _imap_connect(row.username or "", secret, settings)
    try:
        status, _ = client.select(settings.get("mailbox") or "INBOX")
        return status == "OK"
    finally:
        try:
            client.logout()
        except Exception:
            pass


def list_recent_imap_messages(
    db: Session,
    user_id: str,
    max_results: int = 10,
    hours_back: int = 24,
    unread_only: bool = True,
    provider: str = "icloud",
    include_body: bool = False,
) -> List[Dict[str, Any]]:
    row, secret, settings = _get_credentials(db, user_id, provider)
    client = _imap_connect(row.username or "", secret, settings)
    mailbox = settings.get("mailbox") or "INBOX"

    try:
        client.select(mailbox)
        since_date = (datetime.utcnow() - timedelta(hours=hours_back)).strftime("%d-%b-%Y")
        criteria: List[str] = []
        if unread_only:
            criteria.append("UNSEEN")
        criteria += ["SINCE", since_date]

        typ, data = client.uid("search", None, *criteria)
        if typ != "OK":
            return []
        raw_ids = data[0].split() if data and data[0] else []
        if not raw_ids:
            return []

        ids = [i.decode() if isinstance(i, bytes) else str(i) for i in raw_ids]
        ids = ids[-max_results:]
        results: List[Dict[str, Any]] = []
        for uid in reversed(ids):
            fetch_typ, msg_data = client.uid("fetch", uid, "(BODY.PEEK[])" if include_body else "(BODY.PEEK[HEADER])")
            if fetch_typ != "OK" or not msg_data:
                continue
            raw = msg_data[0][1] if isinstance(msg_data[0], tuple) else None
            if not raw:
                continue
            results.append(_parse_message(uid, raw, include_body))
        return results
    finally:
        try:
            client.logout()
        except Exception:
            pass


def search_imap_messages(
    db: Session,
    user_id: str,
    query: str,
    max_results: int = 10,
    provider: str = "icloud",
    include_body: bool = False,
) -> List[Dict[str, Any]]:
    if not query:
        return []
    row, secret, settings = _get_credentials(db, user_id, provider)
    client = _imap_connect(row.username or "", secret, settings)
    mailbox = settings.get("mailbox") or "INBOX"

    try:
        client.select(mailbox)
        typ, data = client.uid("search", None, "TEXT", f"\"{query}\"")
        if typ != "OK":
            return []
        raw_ids = data[0].split() if data and data[0] else []
        if not raw_ids:
            return []
        ids = [i.decode() if isinstance(i, bytes) else str(i) for i in raw_ids]
        ids = ids[-max_results:]
        results: List[Dict[str, Any]] = []
        for uid in reversed(ids):
            fetch_typ, msg_data = client.uid("fetch", uid, "(BODY.PEEK[])" if include_body else "(BODY.PEEK[HEADER])")
            if fetch_typ != "OK" or not msg_data:
                continue
            raw = msg_data[0][1] if isinstance(msg_data[0], tuple) else None
            if not raw:
                continue
            results.append(_parse_message(uid, raw, include_body))
        return results
    finally:
        try:
            client.logout()
        except Exception:
            pass


def get_imap_message(
    db: Session,
    user_id: str,
    message_id: str,
    provider: str = "icloud",
    include_body: bool = True,
) -> Optional[Dict[str, Any]]:
    if not message_id:
        return None
    row, secret, settings = _get_credentials(db, user_id, provider)
    client = _imap_connect(row.username or "", secret, settings)
    mailbox = settings.get("mailbox") or "INBOX"

    try:
        client.select(mailbox)
        fetch_typ, msg_data = client.uid("fetch", message_id, "(BODY.PEEK[])" if include_body else "(BODY.PEEK[HEADER])")
        if fetch_typ != "OK" or not msg_data:
            return None
        raw = msg_data[0][1] if isinstance(msg_data[0], tuple) else None
        if not raw:
            return None
        return _parse_message(str(message_id), raw, include_body)
    finally:
        try:
            client.logout()
        except Exception:
            pass


def send_imap_email(
    db: Session,
    user_id: str,
    to_email: str,
    subject: str,
    body_text: str,
    cc: Optional[str] = None,
    bcc: Optional[str] = None,
    provider: str = "icloud",
) -> Dict[str, Any]:
    row, secret, settings = _get_credentials(db, user_id, provider)

    msg = EmailMessage()
    msg["To"] = to_email
    msg["From"] = settings.get("from_email") or row.username
    msg["Subject"] = subject
    if cc:
        msg["Cc"] = cc
    if bcc:
        msg["Bcc"] = bcc
    msg.set_content(body_text)

    _smtp_send(row.username or "", secret, settings, msg)
    return {"id": f"imap-{uuid4().hex}"}
