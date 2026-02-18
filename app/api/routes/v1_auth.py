from __future__ import annotations

import json
import uuid
from datetime import datetime
from typing import Any

from fastapi import APIRouter, Depends, HTTPException, Request
from pydantic import BaseModel, Field
from sqlalchemy import text
from sqlalchemy.orm import Session

from app.api.deps import get_db, get_or_create_user
from app.blueprint.db import normalize_e164
from app.core.config import settings
from app.middleware.rate_limiter import rate_limit_user
from app.services.phone_verification import request_phone_verification, verify_phone_code


router = APIRouter(prefix="/api/v1/auth", tags=["auth-v1"])


def _ensure_channel_connections_sqlite(db: Session) -> None:
    dialect = db.bind.dialect.name if db.bind is not None else ""
    if dialect != "sqlite":
        return
    db.execute(
        text(
            """
            create table if not exists channel_connections (
              id text primary key,
              user_id text,
              channel text not null,
              channel_identifier text not null,
              is_primary integer default 0,
              metadata_json text default '{}',
              created_at text,
              unique(channel, channel_identifier)
            )
            """
        )
    )
    db.commit()


def _link_channel_connection(
    db: Session,
    *,
    user_id: str,
    channel: str,
    channel_identifier: str,
    metadata: dict[str, Any] | None = None,
) -> dict[str, Any]:
    """
    Inserts a channel connection row with conflict detection.
    """
    _ensure_channel_connections_sqlite(db)
    dialect = db.bind.dialect.name if db.bind is not None else ""
    meta_json = json.dumps(metadata or {}, ensure_ascii=False)

    if dialect == "sqlite":
        existing = db.execute(
            text(
                "select user_id from channel_connections where channel = :channel and channel_identifier = :ident limit 1"
            ),
            {"channel": channel, "ident": channel_identifier},
        ).mappings().first()
        if existing and str(existing.get("user_id") or "") != user_id:
            raise HTTPException(status_code=409, detail="Channel identifier already linked to another user")
        db.execute(
            text(
                """
                insert or ignore into channel_connections (id, user_id, channel, channel_identifier, is_primary, metadata_json, created_at)
                values (:id, :user_id, :channel, :ident, 0, :meta, :created_at)
                """
            ),
            {
                "id": str(uuid.uuid4()),
                "user_id": user_id,
                "channel": channel,
                "ident": channel_identifier,
                "meta": meta_json,
                "created_at": datetime.utcnow().isoformat(),
            },
        )
        db.commit()
        return {"ok": True, "channel": channel, "channel_identifier": channel_identifier}

    # Postgres v5 schema uses channel_type enum.
    existing = db.execute(
        text(
            """
            select user_id::text as user_id
            from channel_connections
            where channel = (:channel)::channel_type and channel_identifier = :ident
            limit 1
            """
        ),
        {"channel": channel, "ident": channel_identifier},
    ).mappings().first()
    if existing and str(existing.get("user_id") or "") != user_id:
        raise HTTPException(status_code=409, detail="Channel identifier already linked to another user")

    db.execute(
        text(
            """
            insert into channel_connections (user_id, channel, channel_identifier, is_primary, metadata)
            values ((:user_id)::uuid, (:channel)::channel_type, :ident, false, (:meta)::jsonb)
            on conflict (channel, channel_identifier) do nothing
            """
        ),
        {"user_id": user_id, "channel": channel, "ident": channel_identifier, "meta": meta_json},
    )
    db.commit()
    return {"ok": True, "channel": channel, "channel_identifier": channel_identifier}


class LinkChannelRequest(BaseModel):
    user_id: str | None = Field(default=None, description="Optional in dev; in prod use auth and omit.")
    channel: str = Field(default="imessage")
    channel_identifier: str = Field(..., description="Phone number or channel identifier to link")
    code: str | None = Field(default=None, description="If provided, verify + link; otherwise send OTP")


@rate_limit_user()
@router.post("/link-channel")
def link_channel(request: Request, payload: LinkChannelRequest, db: Session = Depends(get_db)):
    user_id = (payload.user_id or getattr(request.state, "user_id", None) or "").strip()
    if not user_id:
        raise HTTPException(status_code=401, detail="Authentication required")

    channel = (payload.channel or "").strip().lower()
    if channel not in {"imessage"}:
        raise HTTPException(status_code=400, detail="Unsupported channel for linking (expected imessage)")

    ident = normalize_e164(payload.channel_identifier) or payload.channel_identifier.strip()
    if not ident:
        raise HTTPException(status_code=400, detail="channel_identifier is required")

    # Ensure legacy user row exists for FK-backed verification tables.
    get_or_create_user(db, user_id)

    # Step 1: Send OTP
    if not payload.code:
        try:
            return request_phone_verification(db, user_id, ident)
        except ValueError as exc:
            raise HTTPException(status_code=400, detail=str(exc))

    # Step 2: Verify OTP (do not mutate profile/preferences; linking is separate)
    try:
        _ = verify_phone_code(db, user_id, ident, payload.code, apply_profile_updates=False)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc))

    # Link to the authenticated user.
    return _link_channel_connection(
        db,
        user_id=user_id,
        channel=channel,
        channel_identifier=ident,
        metadata={"linked_via": "otp", "env": settings.ENV},
    )
