from __future__ import annotations

import json
from datetime import date
from typing import Any

import httpx
from sqlalchemy import text
from sqlalchemy.orm import Session

from app.blueprint.db import _column_exists, _table_exists
from app.core.config import settings
from app.services.token_crypto import decrypt_token, encrypt_token


class PlaidNotConfiguredError(RuntimeError):
    pass


def _plaid_env(stage: str = "staging") -> str:
    if stage == "prod":
        return str(settings.PLAID_ENV_PROD or "production")
    return str(settings.PLAID_ENV_STAGING or "sandbox")


def _plaid_secret(stage: str = "staging") -> str:
    if stage == "prod":
        return str(settings.PLAID_SECRET_PROD or "").strip()
    return str(settings.PLAID_SECRET_STAGING or "").strip()


def _base_url(stage: str = "staging") -> str:
    env = _plaid_env(stage)
    if env == "production":
        return "https://production.plaid.com"
    if env == "development":
        return "https://development.plaid.com"
    return "https://sandbox.plaid.com"


def _plaid_request(stage: str, endpoint: str, payload: dict[str, Any]) -> dict[str, Any]:
    client_id = str(settings.PLAID_CLIENT_ID or "").strip()
    secret = _plaid_secret(stage)
    if not client_id or not secret:
        raise PlaidNotConfiguredError("Plaid client id/secret is missing")
    body = {"client_id": client_id, "secret": secret, **(payload or {})}
    with httpx.Client(timeout=20.0) as client:
        resp = client.post(f"{_base_url(stage)}{endpoint}", json=body)
        resp.raise_for_status()
        data = resp.json()
    if data.get("error_code"):
        raise RuntimeError(f"Plaid API error: {data.get('error_code')} {data.get('error_message')}")
    return data


def _provider_name(stage: str) -> str:
    env = _plaid_env(stage)
    return f"plaid-{env}"


def store_access_token(db: Session, *, user_id: str, access_token: str, stage: str = "staging") -> None:
    if not _table_exists(db, "oauth_tokens"):
        raise RuntimeError("oauth_tokens table not found")
    provider = _provider_name(stage)
    now_fn = "CURRENT_TIMESTAMP" if (db.bind and db.bind.dialect.name == "sqlite") else "now()"
    encrypted = encrypt_token(access_token)
    if _column_exists(db, "oauth_tokens", "encrypted_access_token"):
        db.execute(
            text(
                f"""
                insert into oauth_tokens (id, user_id, provider, encrypted_access_token, token_expiry, metadata, created_at, updated_at)
                values (:id, :user_id, :provider, :access_token, null, :metadata, {now_fn}, {now_fn})
                on conflict(user_id, provider) do update set
                  encrypted_access_token=excluded.encrypted_access_token,
                  updated_at={now_fn}
                """
            ),
            {
                "id": user_id + "-" + provider,
                "user_id": user_id,
                "provider": provider,
                "access_token": encrypted,
                "metadata": json.dumps({"source": "plaid_connector"}, ensure_ascii=False),
            },
        )
    else:
        db.execute(
            text(
                f"""
                insert into oauth_tokens (user_id, provider, access_token, refresh_token_enc, expiry_utc, updated_at, created_at)
                values (:user_id, :provider, :access_token, '', null, {now_fn}, {now_fn})
                on conflict(user_id, provider) do update set access_token=excluded.access_token, updated_at={now_fn}
                """
            ),
            {"user_id": user_id, "provider": provider, "access_token": access_token},
        )
    db.commit()


def get_access_token(db: Session, *, user_id: str, stage: str = "staging") -> str:
    if not _table_exists(db, "oauth_tokens"):
        raise RuntimeError("oauth_tokens table not found")
    provider = _provider_name(stage)
    fields = []
    if _column_exists(db, "oauth_tokens", "encrypted_access_token"):
        fields.append("encrypted_access_token")
    if _column_exists(db, "oauth_tokens", "access_token"):
        fields.append("access_token")
    if not fields:
        raise RuntimeError("oauth_tokens table missing access token fields")
    row = db.execute(
        text(f"select {', '.join(fields)} from oauth_tokens where user_id = :user_id and provider = :provider"),
        {"user_id": user_id, "provider": provider},
    ).mappings().first()
    if not row:
        raise RuntimeError("Plaid token is not connected")
    if "encrypted_access_token" in fields:
        token_enc = str(row.get("encrypted_access_token") or "").strip()
        if token_enc:
            try:
                return decrypt_token(token_enc)
            except Exception:
                pass
    token = str(row.get("access_token") or "").strip()
    if not token:
        raise RuntimeError("Plaid token is empty")
    return token


def create_link_token(*, user_id: str, stage: str = "staging") -> dict[str, Any]:
    redirect_uri = settings.PLAID_REDIRECT_URI_STAGING if stage != "prod" else settings.PLAID_REDIRECT_URI_PROD
    payload = {
        "client_name": "Executive OS",
        "country_codes": ["US"],
        "language": "en",
        "products": ["transactions"],
        "user": {"client_user_id": user_id},
    }
    if redirect_uri:
        payload["redirect_uri"] = redirect_uri
    return _plaid_request(stage, "/link/token/create", payload)


def exchange_public_token(db: Session, *, user_id: str, public_token: str, stage: str = "staging") -> dict[str, Any]:
    resp = _plaid_request(stage, "/item/public_token/exchange", {"public_token": public_token})
    access_token = str(resp.get("access_token") or "")
    if not access_token:
        raise RuntimeError("Plaid response missing access_token")
    store_access_token(db, user_id=user_id, access_token=access_token, stage=stage)
    return {"item_id": resp.get("item_id"), "request_id": resp.get("request_id")}


def list_accounts(db: Session, *, user_id: str, stage: str = "staging") -> dict[str, Any]:
    access_token = get_access_token(db, user_id=user_id, stage=stage)
    return _plaid_request(stage, "/accounts/get", {"access_token": access_token})


def list_transactions(
    db: Session,
    *,
    user_id: str,
    start_date: date,
    end_date: date,
    stage: str = "staging",
) -> dict[str, Any]:
    access_token = get_access_token(db, user_id=user_id, stage=stage)
    return _plaid_request(
        stage,
        "/transactions/get",
        {
            "access_token": access_token,
            "start_date": start_date.isoformat(),
            "end_date": end_date.isoformat(),
            "options": {"count": 100, "offset": 0},
        },
    )
