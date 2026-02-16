# backend/app/services/microsoft_oauth.py

from __future__ import annotations

import os
from datetime import datetime, timedelta
from typing import Any, Dict, Optional
from urllib.parse import urlencode

import httpx
from sqlalchemy.orm import Session

from app.db.models import OAuthToken, User
from app.services.oauth_state import make_state, parse_state
from app.services.token_crypto import decrypt_token, encrypt_token


DEFAULT_SCOPES = [
    "openid",
    "profile",
    "email",
    "offline_access",
    "User.Read",
    "Calendars.ReadWrite",
    "Mail.ReadWrite",
]


def _require_env(name: str) -> str:
    val = os.getenv(name)
    if not val:
        raise RuntimeError(f"Missing required env var: {name}")
    return val


def _tenant() -> str:
    return os.getenv("MS_TENANT_ID") or "common"


def _authorize_url() -> str:
    return f"https://login.microsoftonline.com/{_tenant()}/oauth2/v2.0/authorize"


def _token_url() -> str:
    return f"https://login.microsoftonline.com/{_tenant()}/oauth2/v2.0/token"


def _decrypt_refresh_token(refresh_token_enc: str) -> str:
    return decrypt_token(refresh_token_enc)


def build_microsoft_auth_url(user_id: str) -> str:
    client_id = _require_env("MS_CLIENT_ID")
    redirect_uri = _require_env("MS_REDIRECT_URI")
    state = make_state(user_id)

    params = {
        "client_id": client_id,
        "response_type": "code",
        "redirect_uri": redirect_uri,
        "response_mode": "query",
        "scope": " ".join(DEFAULT_SCOPES),
        "state": state,
        "prompt": "consent",
    }
    return f"{_authorize_url()}?{urlencode(params)}"


def _store_tokens(
    db: Session,
    user_id: str,
    access_token: Optional[str],
    refresh_token: Optional[str],
    expires_in: Optional[int],
    scopes: Optional[str],
    email: Optional[str] = None,
) -> None:
    row = (
        db.query(OAuthToken)
        .filter(OAuthToken.user_id == user_id, OAuthToken.provider == "microsoft")
        .one_or_none()
    )
    if row is None:
        row = OAuthToken(user_id=user_id, provider="microsoft")
        db.add(row)

    row.access_token = access_token or ""
    row.refresh_token_enc = encrypt_token(refresh_token or "")
    row.scopes = scopes
    if expires_in:
        row.expiry_utc = datetime.utcnow() + timedelta(seconds=int(expires_in))
    row.updated_at = datetime.utcnow()
    if email:
        row.email = email

    db.commit()


def _fetch_microsoft_email(access_token: str) -> Optional[str]:
    if not access_token:
        return None
    try:
        resp = httpx.get(
            "https://graph.microsoft.com/v1.0/me",
            headers={"Authorization": f"Bearer {access_token}"},
            timeout=10.0,
        )
        if resp.status_code >= 400:
            return None
        data = resp.json()
        return data.get("mail") or data.get("userPrincipalName")
    except Exception:
        return None


def exchange_code_and_store_tokens(db: Session, code: str, state: str) -> str:
    user_id = parse_state(state)

    user = db.get(User, user_id)
    if user is None:
        user = User(id=user_id)
        db.add(user)
        db.commit()

    data = {
        "client_id": _require_env("MS_CLIENT_ID"),
        "client_secret": _require_env("MS_CLIENT_SECRET"),
        "redirect_uri": _require_env("MS_REDIRECT_URI"),
        "grant_type": "authorization_code",
        "code": code,
        "scope": " ".join(DEFAULT_SCOPES),
    }
    resp = httpx.post(_token_url(), data=data, timeout=15.0)
    if resp.status_code >= 400:
        raise RuntimeError(f"Microsoft token exchange failed: {resp.text}")
    payload = resp.json()

    access_token = payload.get("access_token")
    refresh_token = payload.get("refresh_token")
    expires_in = payload.get("expires_in")
    scopes = payload.get("scope") or " ".join(DEFAULT_SCOPES)
    email = _fetch_microsoft_email(access_token or "")

    _store_tokens(
        db=db,
        user_id=user_id,
        access_token=access_token,
        refresh_token=refresh_token,
        expires_in=expires_in,
        scopes=scopes,
        email=email,
    )
    return user_id


def get_valid_microsoft_access_token(db: Session, user_id: str) -> Optional[str]:
    row = (
        db.query(OAuthToken)
        .filter(OAuthToken.user_id == user_id, OAuthToken.provider == "microsoft")
        .one_or_none()
    )
    if row is None:
        return None

    now = datetime.utcnow()
    if row.access_token and row.expiry_utc and row.expiry_utc > now:
        return row.access_token

    refresh_token = _decrypt_refresh_token(row.refresh_token_enc)
    if not refresh_token:
        return None

    data = {
        "client_id": _require_env("MS_CLIENT_ID"),
        "client_secret": _require_env("MS_CLIENT_SECRET"),
        "grant_type": "refresh_token",
        "refresh_token": refresh_token,
        "redirect_uri": _require_env("MS_REDIRECT_URI"),
        "scope": " ".join(DEFAULT_SCOPES),
    }
    resp = httpx.post(_token_url(), data=data, timeout=15.0)
    if resp.status_code >= 400:
        raise RuntimeError(f"Microsoft token refresh failed: {resp.text}")
    payload = resp.json()

    access_token = payload.get("access_token")
    refresh_token_new = payload.get("refresh_token") or refresh_token
    expires_in = payload.get("expires_in")
    scopes = payload.get("scope") or row.scopes

    _store_tokens(
        db=db,
        user_id=user_id,
        access_token=access_token,
        refresh_token=refresh_token_new,
        expires_in=expires_in,
        scopes=scopes,
        email=row.email,
    )
    return access_token


def get_microsoft_connection_status(db: Session, user_id: str) -> Dict[str, Any]:
    row = (
        db.query(OAuthToken)
        .filter(OAuthToken.user_id == user_id, OAuthToken.provider == "microsoft")
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


def disconnect_microsoft(db: Session, user_id: str) -> None:
    row = (
        db.query(OAuthToken)
        .filter(OAuthToken.user_id == user_id, OAuthToken.provider == "microsoft")
        .one_or_none()
    )
    if row is None:
        return
    db.delete(row)
    db.commit()
