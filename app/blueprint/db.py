from __future__ import annotations

import hashlib
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
    run_id: str | None = None,
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
                "(id, conversation_id, user_id, direction, content, intent, tier, run_id, cost_cents, latency_ms, channel_msg_id, created_at) "
                "values "
                "(:id::uuid, :conversation_id::uuid, :user_id::uuid, :direction::message_direction, :content::jsonb, :intent, :tier, :run_id::uuid, :cost_cents, :latency_ms, :channel_msg_id, now()) "
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
                "run_id": run_id,
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


def attach_run_to_message(
    db: Session,
    *,
    user_id: str,
    message_id: str,
    run_id: str,
) -> None:
    """
    Attach a run_id to an existing message row.
    """
    set_app_user_id(db, user_id)
    db.execute(
        text("update messages set run_id = :run_id::uuid where id = :id::uuid"),
        {"id": message_id, "run_id": run_id},
    )
    db.commit()


def _json_hash(payload: dict[str, Any]) -> str:
    encoded = json.dumps(payload or {}, sort_keys=True, ensure_ascii=False).encode("utf-8")
    return hashlib.sha256(encoded).hexdigest()


def create_run(
    db: Session,
    *,
    user_id: str,
    conversation_id: str,
    envelope: dict[str, Any],
    intent: str,
    tier: int,
    parent_run_id: str | None = None,
) -> str:
    """
    Create a Blueprint `runs` row and return its UUID.
    """
    run_id = str(uuid.uuid4())
    set_app_user_id(db, user_id)
    db.execute(
        text(
            "insert into runs "
            "(id, user_id, conversation_id, parent_run_id, intent, tier, plan, state, envelope, created_at) "
            "values "
            "(:id::uuid, :user_id::uuid, :conversation_id::uuid, :parent_run_id::uuid, :intent, :tier, '[]'::jsonb, 'pending'::run_state, :envelope::jsonb, now())"
        ),
        {
            "id": run_id,
            "user_id": user_id,
            "conversation_id": conversation_id,
            "parent_run_id": parent_run_id,
            "intent": intent,
            "tier": tier,
            "envelope": json.dumps(envelope or {}, ensure_ascii=False),
        },
    )
    db.commit()
    return run_id


def complete_run(
    db: Session,
    *,
    run_id: str,
    user_id: str,
    state: str = "completed",
    total_cost_cents: int = 0,
    total_latency_ms: int = 0,
    error: dict[str, Any] | None = None,
) -> None:
    """
    Mark a run terminal (completed/failed/cancelled).
    """
    set_app_user_id(db, user_id)
    db.execute(
        text(
            "update runs "
            "set state = :state::run_state, "
            "    error = :error::jsonb, "
            "    total_cost_cents = :total_cost_cents, "
            "    total_latency_ms = :total_latency_ms, "
            "    completed_at = now() "
            "where id = :id::uuid"
        ),
        {
            "id": run_id,
            "state": state,
            "error": json.dumps(error, ensure_ascii=False) if error else None,
            "total_cost_cents": total_cost_cents,
            "total_latency_ms": total_latency_ms,
        },
    )
    db.commit()


def list_conversation_messages(
    db: Session,
    *,
    user_id: str,
    conversation_id: str,
    limit: int = 12,
) -> list[dict[str, str]]:
    """
    Fetch recent Blueprint messages for context compilation.

    Returns OpenAI-style message dicts: {"role": "user"|"assistant", "content": "..."}.
    """
    set_app_user_id(db, user_id)
    rows = (
        db.execute(
            text(
                "select direction, content "
                "from messages "
                "where conversation_id = :conversation_id::uuid "
                "order by created_at desc "
                "limit :limit"
            ),
            {"conversation_id": conversation_id, "limit": int(limit)},
        )
        .mappings()
        .all()
    )

    items: list[dict[str, str]] = []
    for row in reversed(rows or []):
        direction = str(row.get("direction") or "")
        raw_content = row.get("content")
        content: dict[str, Any]
        if isinstance(raw_content, dict):
            content = raw_content
        else:
            try:
                content = json.loads(raw_content or "{}")
            except Exception:
                content = {}

        text_val = content.get("text")
        if isinstance(text_val, str) and text_val.strip():
            msg_text = text_val.strip()
        elif "location" in content:
            msg_text = "[Location shared]"
        else:
            continue

        role = "user" if direction == MessageDirection.INBOUND.value else "assistant"
        items.append({"role": role, "content": msg_text})
    return items


def get_tool_execution_by_idempotency(
    db: Session,
    *,
    user_id: str,
    idempotency_key: str,
) -> dict[str, Any] | None:
    set_app_user_id(db, user_id)
    row = (
        db.execute(
            text(
                "select tool_name, status, output, error "
                "from tool_executions "
                "where idempotency_key = :k "
                "limit 1"
            ),
            {"k": idempotency_key},
        )
        .mappings()
        .first()
    )
    if not row:
        return None
    return {
        "tool_name": row.get("tool_name"),
        "status": row.get("status"),
        "output": row.get("output"),
        "error": row.get("error"),
    }


def insert_tool_execution(
    db: Session,
    *,
    run_id: str,
    user_id: str,
    tool_name: str,
    input_payload: dict[str, Any],
    output_payload: dict[str, Any] | None,
    status: str,
    error_payload: dict[str, Any] | None,
    idempotency_key: str,
    risk_level: str = "none",
    cost_cents: int = 0,
    latency_ms: int,
) -> str | None:
    """
    Insert tool execution row. Returns id, or None if deduped by idempotency_key.
    """
    set_app_user_id(db, user_id)
    tool_id = str(uuid.uuid4())
    try:
        row = db.execute(
            text(
                "insert into tool_executions "
                "(id, run_id, user_id, tool_name, input_hash, input, output, status, error, idempotency_key, risk_level, cost_cents, latency_ms, created_at) "
                "values "
                "(:id::uuid, :run_id::uuid, :user_id::uuid, :tool_name, :input_hash, :input::jsonb, :output::jsonb, :status::tool_exec_status, :error::jsonb, :idempotency_key, :risk_level::risk_level, :cost_cents, :latency_ms, now()) "
                "returning id"
            ),
            {
                "id": tool_id,
                "run_id": run_id,
                "user_id": user_id,
                "tool_name": tool_name,
                "input_hash": _json_hash(input_payload),
                "input": json.dumps(input_payload, ensure_ascii=False),
                "output": json.dumps(output_payload, ensure_ascii=False) if output_payload else None,
                "status": status,
                "error": json.dumps(error_payload, ensure_ascii=False) if error_payload else None,
                "idempotency_key": idempotency_key,
                "risk_level": risk_level,
                "cost_cents": int(cost_cents),
                "latency_ms": int(latency_ms),
            },
        ).mappings().first()
        db.commit()
        return str((row or {}).get("id") or tool_id)
    except IntegrityError:
        db.rollback()
        return None
