# backend/app/services/google_oauth.py

from __future__ import annotations

import base64
import hashlib
import os
from datetime import datetime, timedelta, timezone
from typing import Any, Dict, Optional

from cryptography.fernet import Fernet, InvalidToken
from google_auth_oauthlib.flow import Flow
from google.oauth2.credentials import Credentials
from google.auth.transport.requests import Request as GoogleRequest
from sqlalchemy.orm import Session
from app.db.models import User
from app.db.models import OAuthToken
from app.core.config import settings
from app.services.oauth_state import make_state, parse_state

def _to_naive_utc(dt):
    """
    Returns a naive UTC datetime (tzinfo=None).
    Google auth libs commonly use naive UTC datetimes internally.
    """
    if dt is None:
        return None
    # If timezone-aware, drop tzinfo after converting to UTC
    if getattr(dt, "tzinfo", None) is not None and dt.tzinfo is not None:
        return dt.astimezone(timezone.utc).replace(tzinfo=None)
    # If already naive, assume it's UTC
    return dt

DEFAULT_SCOPES = [
    # Calendar
    "https://www.googleapis.com/auth/calendar.events",
    # Gmail (draft + send). If you want drafts only, change to gmail.compose
    "https://www.googleapis.com/auth/gmail.send",
    "https://www.googleapis.com/auth/gmail.modify",
    # Identity (optional but useful)
    "openid",
    "https://www.googleapis.com/auth/userinfo.email",
]

def _require_env(name: str) -> str:
    val = os.getenv(name)
    if not val:
        raise RuntimeError(f"Missing required env var: {name}")
    return val


def _fernet() -> Fernet:
    """
    Uses TOKEN_ENCRYPTION_KEY (preferred) or derives a stable key from STATE_SIGNING_SECRET.
    TOKEN_ENCRYPTION_KEY must be a urlsafe base64-encoded 32-byte key (Fernet key).
    """
    key = settings.TOKEN_ENCRYPTION_KEY
    if key:
        return Fernet(key.encode("utf-8"))

    secret = _require_env("STATE_SIGNING_SECRET").encode("utf-8")
    digest = hashlib.sha256(secret).digest()
    return Fernet(base64.urlsafe_b64encode(digest))


def _build_flow(state: str) -> Flow:
    client_id = _require_env("GOOGLE_CLIENT_ID")
    client_secret = _require_env("GOOGLE_CLIENT_SECRET")
    redirect_uri = _require_env("GOOGLE_REDIRECT_URI")

    flow = Flow.from_client_config(
        {
            "web": {
                "client_id": client_id,
                "client_secret": client_secret,
                "auth_uri": "https://accounts.google.com/o/oauth2/auth",
                "token_uri": "https://oauth2.googleapis.com/token",
                "redirect_uris": [redirect_uri],
            }
        },
        scopes=DEFAULT_SCOPES,
        state=state,
    )
    flow.redirect_uri = redirect_uri
    return flow


def build_google_auth_url(user_id: str) -> str:
    state = make_state(user_id)
    flow = _build_flow(state)
    auth_url, _ = flow.authorization_url(
        access_type="offline",
        include_granted_scopes="true",
        prompt="consent",
    )
    return auth_url


def exchange_code_and_store_tokens(db: Session, code: str, state: str) -> str:
    user_id = parse_state(state)
    # Ensure user exists before writing oauth token row (prevents FK issues)
    user = db.get(User, user_id)
    if user is None:
        user = User(id=user_id)
        db.add(user)
        db.commit()

    flow = _build_flow(state)
    flow.fetch_token(code=code)
    creds: Credentials = flow.credentials

    refresh_token = creds.refresh_token
    access_token = creds.token
    expiry = creds.expiry

    f = _fernet()
    enc_refresh = f.encrypt((refresh_token or "").encode("utf-8")).decode("utf-8")

    scopes = " ".join(creds.scopes or DEFAULT_SCOPES)

    row = (
        db.query(OAuthToken)
        .filter(OAuthToken.user_id == user_id, OAuthToken.provider == "google")
        .one_or_none()
    )
    if row is None:
        row = OAuthToken(user_id=user_id, provider="google")
        db.add(row)

    row.scopes = scopes
    row.access_token = access_token or ""
    row.refresh_token_enc = enc_refresh
    row.expiry_utc = _to_naive_utc(expiry)
    row.updated_at = datetime.utcnow()

    db.commit()
    return user_id


def _decrypt_refresh_token(refresh_token_enc: str) -> str:
    if not refresh_token_enc:
        return ""
    f = _fernet()
    try:
        return f.decrypt(refresh_token_enc.encode("utf-8")).decode("utf-8")
    except InvalidToken:
        raise RuntimeError("Could not decrypt refresh token. TOKEN_ENCRYPTION_KEY changed?")


def get_valid_google_credentials(db: Session, user_id: str) -> Optional[Credentials]:
    row = (
        db.query(OAuthToken)
        .filter(OAuthToken.user_id == user_id, OAuthToken.provider == "google")
        .one_or_none()
    )
    if row is None:
        return None

    refresh_token = _decrypt_refresh_token(row.refresh_token_enc)
    creds = Credentials(
        token=row.access_token or None,
        refresh_token=refresh_token or None,
        token_uri="https://oauth2.googleapis.com/token",
        client_id=_require_env("GOOGLE_CLIENT_ID"),
        client_secret=_require_env("GOOGLE_CLIENT_SECRET"),
        scopes=(row.scopes.split() if row.scopes else DEFAULT_SCOPES),
    )

    if row.expiry_utc:
        creds.expiry = _to_naive_utc(row.expiry_utc)

    if creds and creds.expired and creds.refresh_token:
        creds.refresh(GoogleRequest())
        row.access_token = creds.token or ""
        row.expiry_utc = _to_naive_utc(creds.expiry)
        row.updated_at = datetime.utcnow()
        db.commit()

    return creds


def get_google_connection_status(db: Session, user_id: str) -> Dict[str, Any]:
    row = (
        db.query(OAuthToken)
        .filter(OAuthToken.user_id == user_id, OAuthToken.provider == "google")
        .one_or_none()
    )
    if row is None:
        return {"connected": False}

    connected = bool(row.refresh_token_enc) or bool(row.access_token)
    return {
        "connected": connected,
        "scopes": (row.scopes.split() if row.scopes else []),
        "expiry_utc": row.expiry_utc.isoformat() if row.expiry_utc else None,
        "email": row.email,
        "updated_at": row.updated_at.isoformat() if row.updated_at else None,
    }


def disconnect_google(db: Session, user_id: str) -> None:
    row = (
        db.query(OAuthToken)
        .filter(OAuthToken.user_id == user_id, OAuthToken.provider == "google")
        .one_or_none()
    )
    if row is None:
        return
    db.delete(row)
    db.commit()
