from __future__ import annotations

import json
import re
import uuid
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
from typing import Any, Optional

from sqlalchemy import text
from sqlalchemy.exc import IntegrityError
from sqlalchemy.orm import Session

from app.blueprint.contracts import Channel, MessageDirection


_UUID_RE = re.compile(r"^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$")


def is_uuid(value: str) -> bool:
    return bool(value and _UUID_RE.match(value))


def normalize_e164(phone: str | None) -> str | None:
    if not phone:
        return None
    p = phone.strip()
    if not p:
        return None
    if not p.startswith("+"):
        p = f"+{p}"
    return p


def set_app_user_id(db: Session, user_id: str) -> None:
    """
    Set the RLS session variable for this transaction/connection.
    Safe no-op in SQLite.
    """
    try:
        db.execute(text("select set_config('app.user_id', :user_id, true)"), {"user_id": user_id})
    except Exception:
        # Don’t break runtime if RLS isn’t available / DB is SQLite.
        return


@dataclass(frozen=True)
class BlueprintUser:
    id: str  # UUID string
    phone_number: str


def get_user_by_phone(db: Session, phone_number: str) -> BlueprintUser | None:
    phone = normalize_e164(phone_number)
    if not phone:
        return None
    row = db.execute(
        text("select id, phone_number from users where phone_number = :phone limit 1"),
        {"phone": phone},
    ).mappings().first()
    if not row:
        return None
    return BlueprintUser(id=str(row["id"]), phone_number=str(row["phone_number"]))


def create_user(db: Session, phone_number: str) -> BlueprintUser:
    phone = normalize_e164(phone_number)
    if not phone:
        raise ValueError("phone_number is required")

    # To remain compatible with strict RLS setups, generate the UUID ourselves
    # and set app.user_id before inserting.
    user_uuid = str(uuid.uuid4())
    set_app_user_id(db, user_uuid)
    row = db.execute(
        text(
            "insert into users (id, phone_number) values (:id::uuid, :phone) "
            "returning id, phone_number"
        ),
        {"id": user_uuid, "phone": phone},
    ).mappings().first()
    db.commit()
    if not row:
        raise RuntimeError("failed to create user")
    return BlueprintUser(id=str(row["id"]), phone_number=str(row["phone_number"]))


def get_or_create_user_by_phone(db: Session, phone_number: str) -> BlueprintUser:
    existing = get_user_by_phone(db, phone_number)
    if existing:
        set_app_user_id(db, existing.id)
        return existing
    try:
        return create_user(db, phone_number)
    except IntegrityError:
        # Race: another worker created the same phone_number.
        db.rollback()
        existing = get_user_by_phone(db, phone_number)
        if not existing:
            raise
        set_app_user_id(db, existing.id)
        return existing


def get_or_create_conversation(
    db: Session,
    *,
    user_id: str,
    channel: Channel,
    max_inactive_minutes: int = 30,
) -> str:
    """
    Create a new conversation after inactivity, otherwise return most recent active.
    """
    cutoff = datetime.now(timezone.utc) - timedelta(minutes=max_inactive_minutes)
    row = db.execute(
        text(
            "select id, last_active_at "
            "from conversations "
            "where user_id = :user_id::uuid and is_active = true and last_active_at >= :cutoff "
            "order by last_active_at desc "
            "limit 1"
        ),
        {"user_id": user_id, "cutoff": cutoff},
    ).mappings().first()
    if row:
        return str(row["id"])

    convo_id = str(uuid.uuid4())
    db.execute(
        text(
            "insert into conversations (id, user_id, channel, state, summary, started_at, last_active_at, is_active) "
            "values (:id::uuid, :user_id::uuid, :channel::channel_type, '{}'::jsonb, null, now(), now(), true)"
        ),
        {"id": convo_id, "user_id": user_id, "channel": channel.value},
    )
    db.commit()
    return convo_id


def touch_conversation(db: Session, *, conversation_id: str) -> None:
    db.execute(
        text("update conversations set last_active_at = now() where id = :id::uuid"),
        {"id": conversation_id},
    )
    db.commit()


def insert_message(
    db: Session,
    *,
    conversation_id: str,
    user_id: str,
    direction: MessageDirection,
    content: dict[str, Any],
    channel_msg_id: str | None = None,
    intent: str | None = None,
    tier: int | None = None,
    latency_ms: int | None = None,
    cost_cents: int = 0,
) -> str | None:
    """
    Insert into blueprint `messages`.
    Returns message UUID string, or None if deduped.
    """
    payload = json.dumps(content, ensure_ascii=False)
    msg_id = str(uuid.uuid4())
    try:
        row = db.execute(
            text(
                "insert into messages "
                "(id, conversation_id, user_id, direction, content, intent, tier, cost_cents, latency_ms, channel_msg_id, created_at) "
                "values "
                "(:id::uuid, :conversation_id::uuid, :user_id::uuid, :direction::message_direction, :content::jsonb, :intent, :tier, :cost_cents, :latency_ms, :channel_msg_id, now()) "
                "returning id"
            ),
            {
                "id": msg_id,
                "conversation_id": conversation_id,
                "user_id": user_id,
                "direction": direction.value,
                "content": payload,
                "intent": intent,
                "tier": tier,
                "cost_cents": cost_cents,
                "latency_ms": latency_ms,
                "channel_msg_id": channel_msg_id,
            },
        ).mappings().first()
        db.execute(
            text("update conversations set last_active_at = now() where id = :id::uuid"),
            {"id": conversation_id},
        )
        db.commit()
        return str((row or {}).get("id") or msg_id)
    except IntegrityError:
        # Likely channel_msg_id dedup.
        db.rollback()
        return None

