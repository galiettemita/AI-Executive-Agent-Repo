from __future__ import annotations

import json
from datetime import datetime, timezone
from typing import Any

from sqlalchemy import text
from sqlalchemy.orm import Session

from app.blueprint.db import _column_exists, _table_exists
from app.services.token_crypto import encrypt_token


def _now_naive_utc() -> datetime:
    return datetime.now(timezone.utc).replace(tzinfo=None)


def _normalize_expiry(expiry_utc: datetime | None) -> datetime | None:
    if expiry_utc is None:
        return None
    if expiry_utc.tzinfo is None:
        return expiry_utc
    return expiry_utc.astimezone(timezone.utc).replace(tzinfo=None)


def _serialize_scopes(scopes: list[str] | None, *, as_array: bool) -> str | list[str] | None:
    if scopes is None:
        return None
    cleaned = [str(s).strip() for s in scopes if str(s).strip()]
    if as_array:
        return cleaned
    return " ".join(cleaned)


def _serialize_metadata(metadata: dict[str, Any] | None) -> str:
    return json.dumps(metadata or {}, ensure_ascii=False)


def store_provider_tokens(
    db: Session,
    *,
    user_id: str,
    provider: str,
    access_token: str | None = None,
    refresh_token: str | None = None,
    expiry_utc: datetime | None = None,
    scopes: list[str] | None = None,
    metadata: dict[str, Any] | None = None,
    email: str | None = None,
) -> None:
    """
    Unified OAuth vault writer that supports both legacy and v5 schemas:
    - legacy: access_token / refresh_token_enc / expiry_utc / scopes(text)
    - v5: encrypted_access_token / encrypted_refresh_token / token_expiry / scopes(text[])
    """
    if not _table_exists(db, "oauth_tokens"):
        raise RuntimeError("oauth_tokens table not found")

    dialect = db.bind.dialect.name if db.bind is not None else ""
    has_enc_access = _column_exists(db, "oauth_tokens", "encrypted_access_token")
    has_access = _column_exists(db, "oauth_tokens", "access_token")
    has_enc_refresh = _column_exists(db, "oauth_tokens", "encrypted_refresh_token")
    has_refresh = _column_exists(db, "oauth_tokens", "refresh_token_enc")
    has_token_expiry = _column_exists(db, "oauth_tokens", "token_expiry")
    has_expiry_utc = _column_exists(db, "oauth_tokens", "expiry_utc")
    has_metadata = _column_exists(db, "oauth_tokens", "metadata")
    has_email = _column_exists(db, "oauth_tokens", "email")
    has_scopes = _column_exists(db, "oauth_tokens", "scopes")

    scopes_as_array = False
    if has_scopes and dialect != "sqlite":
        # v5 schema uses text[]; legacy commonly uses text.
        try:
            row = db.execute(
                text(
                    """
                    select data_type, udt_name
                    from information_schema.columns
                    where table_schema = current_schema()
                      and table_name = 'oauth_tokens'
                      and column_name = 'scopes'
                    """
                )
            ).mappings().first()
            udt_name = str((row or {}).get("udt_name") or "")
            scopes_as_array = udt_name.startswith("_")
        except Exception:
            scopes_as_array = False

    access_plain = str(access_token or "")
    access_encrypted = encrypt_token(access_plain) if access_plain else ""
    refresh_plain = str(refresh_token or "")
    refresh_encrypted = encrypt_token(refresh_plain) if refresh_plain else ""
    metadata_json = _serialize_metadata(metadata)
    expiry_norm = _normalize_expiry(expiry_utc)
    scopes_value = _serialize_scopes(scopes, as_array=scopes_as_array)

    if dialect == "sqlite":
        existing = db.execute(
            text(
                """
                select id
                from oauth_tokens
                where user_id = :user_id and provider = :provider
                limit 1
                """
            ),
            {"user_id": user_id, "provider": provider},
        ).mappings().first()
    else:
        existing = db.execute(
            text(
                """
                select id::text as id
                from oauth_tokens
                where user_id::text = :user_id and provider = :provider
                limit 1
                """
            ),
            {"user_id": user_id, "provider": provider},
        ).mappings().first()

    set_columns: list[str] = []
    params: dict[str, Any] = {
        "user_id": user_id,
        "provider": provider,
        "updated_at": _now_naive_utc(),
    }

    if has_enc_access:
        set_columns.append("encrypted_access_token = :encrypted_access_token")
        params["encrypted_access_token"] = access_encrypted
    elif has_access:
        set_columns.append("access_token = :access_token")
        params["access_token"] = access_plain

    if has_enc_refresh:
        set_columns.append("encrypted_refresh_token = :encrypted_refresh_token")
        params["encrypted_refresh_token"] = refresh_encrypted
    elif has_refresh:
        set_columns.append("refresh_token_enc = :refresh_token_enc")
        params["refresh_token_enc"] = refresh_encrypted

    if has_token_expiry:
        set_columns.append("token_expiry = :token_expiry")
        params["token_expiry"] = expiry_norm
    elif has_expiry_utc:
        set_columns.append("expiry_utc = :expiry_utc")
        params["expiry_utc"] = expiry_norm

    if has_scopes:
        set_columns.append("scopes = :scopes")
        params["scopes"] = scopes_value

    if has_metadata:
        if dialect == "sqlite":
            set_columns.append("metadata = :metadata")
            params["metadata"] = metadata_json
        else:
            set_columns.append("metadata = (:metadata)::jsonb")
            params["metadata"] = metadata_json

    if has_email and email is not None:
        set_columns.append("email = :email")
        params["email"] = str(email)

    if _column_exists(db, "oauth_tokens", "updated_at"):
        set_columns.append("updated_at = :updated_at")

    if existing:
        if not set_columns:
            return
        where_clause = "user_id = :user_id and provider = :provider" if dialect == "sqlite" else "user_id::text = :user_id and provider = :provider"
        db.execute(
            text(f"update oauth_tokens set {', '.join(set_columns)} where {where_clause}"),
            params,
        )
        db.commit()
        return

    insert_cols = ["user_id", "provider"]
    insert_vals = [":user_id", ":provider"]

    if has_enc_access:
        insert_cols.append("encrypted_access_token")
        insert_vals.append(":encrypted_access_token")
    elif has_access:
        insert_cols.append("access_token")
        insert_vals.append(":access_token")

    if has_enc_refresh:
        insert_cols.append("encrypted_refresh_token")
        insert_vals.append(":encrypted_refresh_token")
    elif has_refresh:
        insert_cols.append("refresh_token_enc")
        insert_vals.append(":refresh_token_enc")

    if has_token_expiry:
        insert_cols.append("token_expiry")
        insert_vals.append(":token_expiry")
    elif has_expiry_utc:
        insert_cols.append("expiry_utc")
        insert_vals.append(":expiry_utc")

    if has_scopes:
        insert_cols.append("scopes")
        insert_vals.append(":scopes")

    if has_metadata:
        insert_cols.append("metadata")
        if dialect == "sqlite":
            insert_vals.append(":metadata")
        else:
            insert_vals.append("(:metadata)::jsonb")

    if has_email and email is not None:
        insert_cols.append("email")
        insert_vals.append(":email")

    if _column_exists(db, "oauth_tokens", "created_at"):
        insert_cols.append("created_at")
        insert_vals.append(":created_at")
        params["created_at"] = _now_naive_utc()
    if _column_exists(db, "oauth_tokens", "updated_at"):
        insert_cols.append("updated_at")
        insert_vals.append(":updated_at")

    db.execute(
        text(
            f"""
            insert into oauth_tokens ({', '.join(insert_cols)})
            values ({', '.join(insert_vals)})
            """
        ),
        params,
    )
    db.commit()


def store_stripe_billing_tokens(
    db: Session,
    *,
    user_id: str,
    customer_id: str,
    subscription_id: str | None = None,
    plan: str | None = None,
    status: str | None = None,
) -> None:
    metadata = {
        "kind": "stripe_billing",
        "plan": plan,
        "status": status,
    }
    store_provider_tokens(
        db,
        user_id=user_id,
        provider="stripe_billing",
        access_token=str(customer_id or ""),
        refresh_token=str(subscription_id or ""),
        metadata=metadata,
    )
