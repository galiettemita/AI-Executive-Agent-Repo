from __future__ import annotations

import base64
import os
from datetime import datetime, timedelta
from typing import Any, Dict, Optional
from urllib.parse import urlencode

import httpx
from sqlalchemy.orm import Session

from app.db.models import OAuthToken
from app.db.user_compat import ensure_user_row
from app.services.oauth_state import make_state, parse_state
from app.services.token_crypto import decrypt_token, encrypt_token


DEFAULT_SCOPES = [
    "activity",
]


def _require_env(name: str) -> str:
    val = os.getenv(name)
    if not val:
        raise RuntimeError(f"Missing required env var: {name}")
    return val


def _decrypt_refresh_token(refresh_token_enc: str) -> str:
    return decrypt_token(refresh_token_enc)


def _authorize_url() -> str:
    return "https://www.fitbit.com/oauth2/authorize"


def _token_url() -> str:
    return "https://api.fitbit.com/oauth2/token"


def _basic_auth_header(client_id: str, client_secret: str) -> Dict[str, str]:
    token = base64.b64encode(f"{client_id}:{client_secret}".encode("utf-8")).decode("utf-8")
    return {"Authorization": f"Basic {token}"}


def build_fitbit_auth_url(user_id: str) -> str:
    client_id = _require_env("FITBIT_CLIENT_ID")
    redirect_uri = _require_env("FITBIT_REDIRECT_URI")
    state = make_state(user_id)

    params = {
        "client_id": client_id,
        "response_type": "code",
        "redirect_uri": redirect_uri,
        "scope": " ".join(DEFAULT_SCOPES),
        "state": state,
    }
    return f"{_authorize_url()}?{urlencode(params)}"


def _store_tokens(
    db: Session,
    user_id: str,
    access_token: Optional[str],
    refresh_token: Optional[str],
    expires_in: Optional[int],
    scopes: Optional[str],
) -> None:
    row = (
        db.query(OAuthToken)
        .filter(OAuthToken.user_id == user_id, OAuthToken.provider == "fitbit")
        .one_or_none()
    )
    if row is None:
        row = OAuthToken(user_id=user_id, provider="fitbit")
        db.add(row)

    row.access_token = access_token or ""
    row.refresh_token_enc = encrypt_token(refresh_token or "")
    row.scopes = scopes
    if expires_in:
        row.expiry_utc = datetime.utcnow() + timedelta(seconds=int(expires_in))
    row.updated_at = datetime.utcnow()

    db.commit()


def exchange_code_and_store_tokens(db: Session, code: str, state: str) -> str:
    user_id = parse_state(state)
    ensure_user_row(db, user_id)

    client_id = _require_env("FITBIT_CLIENT_ID")
    client_secret = _require_env("FITBIT_CLIENT_SECRET")
    redirect_uri = _require_env("FITBIT_REDIRECT_URI")

    headers = {
        **_basic_auth_header(client_id, client_secret),
        "Content-Type": "application/x-www-form-urlencoded",
    }
    data = {
        "code": code,
        "grant_type": "authorization_code",
        "redirect_uri": redirect_uri,
    }
    resp = httpx.post(_token_url(), headers=headers, data=data, timeout=15.0)
    if resp.status_code >= 400:
        raise RuntimeError(f"Fitbit token exchange failed: {resp.text}")

    payload = resp.json()
    access_token = payload.get("access_token")
    refresh_token = payload.get("refresh_token")
    expires_in = payload.get("expires_in")
    scopes = payload.get("scope") or " ".join(DEFAULT_SCOPES)

    _store_tokens(
        db=db,
        user_id=user_id,
        access_token=access_token,
        refresh_token=refresh_token,
        expires_in=expires_in,
        scopes=scopes,
    )
    return user_id


def _refresh_fitbit_token(db: Session, user_id: str, refresh_token: str) -> Optional[str]:
    client_id = _require_env("FITBIT_CLIENT_ID")
    client_secret = _require_env("FITBIT_CLIENT_SECRET")

    headers = {
        **_basic_auth_header(client_id, client_secret),
        "Content-Type": "application/x-www-form-urlencoded",
    }
    data = {
        "grant_type": "refresh_token",
        "refresh_token": refresh_token,
    }
    resp = httpx.post(_token_url(), headers=headers, data=data, timeout=15.0)
    if resp.status_code >= 400:
        raise RuntimeError(f"Fitbit token refresh failed: {resp.text}")

    payload = resp.json()
    access_token = payload.get("access_token")
    refresh_token_new = payload.get("refresh_token") or refresh_token
    expires_in = payload.get("expires_in")
    scopes = payload.get("scope")

    _store_tokens(
        db=db,
        user_id=user_id,
        access_token=access_token,
        refresh_token=refresh_token_new,
        expires_in=expires_in,
        scopes=scopes,
    )
    return access_token


def get_valid_fitbit_access_token(db: Session, user_id: str) -> Optional[str]:
    row = (
        db.query(OAuthToken)
        .filter(OAuthToken.user_id == user_id, OAuthToken.provider == "fitbit")
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

    return _refresh_fitbit_token(db, user_id, refresh_token)


def get_fitbit_connection_status(db: Session, user_id: str) -> Dict[str, Any]:
    row = (
        db.query(OAuthToken)
        .filter(OAuthToken.user_id == user_id, OAuthToken.provider == "fitbit")
        .one_or_none()
    )
    if row is None:
        return {"connected": False}

    connected = bool(row.refresh_token_enc) or bool(row.access_token)
    return {
        "connected": connected,
        "scopes": (row.scopes.split() if row.scopes else []),
        "expiry_utc": row.expiry_utc.isoformat() if row.expiry_utc else None,
        "updated_at": row.updated_at.isoformat() if row.updated_at else None,
    }


def disconnect_fitbit(db: Session, user_id: str) -> None:
    row = (
        db.query(OAuthToken)
        .filter(OAuthToken.user_id == user_id, OAuthToken.provider == "fitbit")
        .one_or_none()
    )
    if row is None:
        return
    db.delete(row)
    db.commit()
