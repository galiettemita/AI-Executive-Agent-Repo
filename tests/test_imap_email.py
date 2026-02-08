from email.message import EmailMessage

from app.db.database import SessionLocal
from app.services.imap_email import (
    resolve_imap_settings,
    list_recent_imap_messages,
    send_imap_email,
)
from app.services.integration_credentials import upsert_integration_credential
import app.services.imap_email as imap_email


def _make_email_bytes(subject: str, sender: str, body: str) -> bytes:
    msg = EmailMessage()
    msg["Subject"] = subject
    msg["From"] = sender
    msg["To"] = "user@example.com"
    msg.set_content(body)
    return msg.as_bytes()


class StubIMAP:
    def __init__(self, raw_bytes: bytes):
        self.raw_bytes = raw_bytes

    def login(self, username, password):
        return "OK", []

    def select(self, mailbox):
        return "OK", []

    def uid(self, command, *args):
        if command == "search":
            return "OK", [b"1 2"]
        if command == "fetch":
            return "OK", [(b"1", self.raw_bytes)]
        return "OK", []

    def logout(self):
        return "OK", []


def test_resolve_imap_settings_defaults():
    db = SessionLocal()
    row = upsert_integration_credential(
        db=db,
        user_id="user_imap_defaults",
        provider="icloud",
        username="user@icloud.com",
        secret="app-pass",
        server_url=None,
        metadata={},
    )
    settings = resolve_imap_settings("icloud", row)
    assert settings["imap_host"] == "imap.mail.me.com"
    assert settings["smtp_host"] == "smtp.mail.me.com"
    db.close()


def test_list_recent_imap_messages(monkeypatch):
    db = SessionLocal()
    upsert_integration_credential(
        db=db,
        user_id="user_imap_list",
        provider="icloud",
        username="user@icloud.com",
        secret="app-pass",
        server_url="imap.mail.me.com",
        metadata={"smtp_host": "smtp.mail.me.com"},
    )

    raw_bytes = _make_email_bytes("Test Subject", "sender@example.com", "Hello world")
    monkeypatch.setattr(imap_email, "_imap_connect", lambda *args, **kwargs: StubIMAP(raw_bytes))

    results = list_recent_imap_messages(
        db=db,
        user_id="user_imap_list",
        max_results=2,
        hours_back=24,
        unread_only=True,
        provider="icloud",
        include_body=True,
    )
    assert results
    assert results[0]["subject"] == "Test Subject"
    assert "Hello world" in (results[0]["body"] or "")
    db.close()


def test_send_imap_email(monkeypatch):
    db = SessionLocal()
    upsert_integration_credential(
        db=db,
        user_id="user_imap_send",
        provider="yahoo",
        username="user@yahoo.com",
        secret="app-pass",
        server_url="imap.mail.yahoo.com",
        metadata={"smtp_host": "smtp.mail.yahoo.com"},
    )

    sent = {"called": False}

    def fake_send(username, password, settings, message):
        sent["called"] = True
        assert settings["smtp_host"] == "smtp.mail.yahoo.com"

    monkeypatch.setattr(imap_email, "_smtp_send", fake_send)

    result = send_imap_email(
        db=db,
        user_id="user_imap_send",
        to_email="dest@example.com",
        subject="Hello",
        body_text="Body",
        provider="yahoo",
    )
    assert sent["called"] is True
    assert result.get("id")
    db.close()
