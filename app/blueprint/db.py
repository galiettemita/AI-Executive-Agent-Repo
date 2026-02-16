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


def _table_exists(db: Session, table_name: str) -> bool:
    try:
        dialect = db.bind.dialect.name if db.bind is not None else ""
        if dialect == "sqlite":
            row = db.execute(
                text("select name from sqlite_master where type='table' and name=:name"),
                {"name": table_name},
            ).first()
            return bool(row)

        row = db.execute(
            text(
                "select 1 from information_schema.tables "
                "where table_schema = current_schema() and table_name = :name"
            ),
            {"name": table_name},
        ).first()
        return bool(row)
    except Exception:
        return False


def _column_exists(db: Session, table_name: str, column_name: str) -> bool:
    try:
        dialect = db.bind.dialect.name if db.bind is not None else ""
        if dialect == "sqlite":
            rows = db.execute(text(f"PRAGMA table_info({table_name})")).mappings().all()
            return any(str(r.get("name")) == column_name for r in rows)

        row = db.execute(
            text(
                "select 1 from information_schema.columns "
                "where table_schema = current_schema() and table_name = :table and column_name = :column"
            ),
            {"table": table_name, "column": column_name},
        ).first()
        return bool(row)
    except Exception:
        return False


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
    if _table_exists(db, "users"):
        row = db.execute(
            text("select id, phone_number from users where phone_number = :phone limit 1"),
            {"phone": phone},
        ).mappings().first()
        if row:
            return BlueprintUser(id=str(row["id"]), phone_number=str(row["phone_number"]))

    if _table_exists(db, "accounts") and _table_exists(db, "channel_connections"):
        row = db.execute(
            text(
                """
                select a.id as id, cc.channel_identifier as phone_number
                from accounts a
                join channel_connections cc on cc.user_id = a.id
                where cc.channel = 'whatsapp'::channel_type
                  and cc.channel_identifier = :phone
                limit 1
                """
            ),
            {"phone": phone},
        ).mappings().first()
        if row:
            return BlueprintUser(id=str(row["id"]), phone_number=str(row["phone_number"]))
    return None


def create_user(db: Session, phone_number: str) -> BlueprintUser:
    phone = normalize_e164(phone_number)
    if not phone:
        raise ValueError("phone_number is required")

    # To remain compatible with strict RLS setups, generate the UUID ourselves
    # and set app.user_id before inserting.
    user_uuid = str(uuid.uuid4())
    set_app_user_id(db, user_uuid)
    if _table_exists(db, "users"):
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

    if _table_exists(db, "accounts") and _table_exists(db, "channel_connections"):
        row = db.execute(
            text(
                """
                insert into accounts (id, clerk_user_id, display_name)
                values (:id::uuid, :clerk_user_id, :display_name)
                returning id
                """
            ),
            {
                "id": user_uuid,
                "clerk_user_id": f"wa:{phone}",
                "display_name": "WhatsApp User",
            },
        ).mappings().first()
        db.execute(
            text(
                """
                insert into channel_connections (user_id, channel, channel_identifier, is_primary)
                values (:user_id::uuid, 'whatsapp'::channel_type, :phone, true)
                on conflict (channel, channel_identifier) do nothing
                """
            ),
            {"user_id": user_uuid, "phone": phone},
        )
        db.commit()
        if not row:
            raise RuntimeError("failed to create account")
        return BlueprintUser(id=str(row["id"]), phone_number=phone)

    raise RuntimeError("No supported account table found")


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
    if _table_exists(db, "conversations"):
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

    # v5 schema removed conversations table; fallback to a deterministic synthetic session ID.
    return f"{user_id}:{channel.value}"


def touch_conversation(db: Session, *, conversation_id: str) -> None:
    if not _table_exists(db, "conversations"):
        return
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
        if _column_exists(db, "messages", "conversation_id"):
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
            if _table_exists(db, "conversations"):
                db.execute(
                    text("update conversations set last_active_at = now() where id = :id::uuid"),
                    {"id": conversation_id},
                )
            db.commit()
            return str((row or {}).get("id") or msg_id)

        # v5 schema message shape (content is text, no conversation_id)
        text_content = str((content or {}).get("text") or payload)
        channel_value = str((content or {}).get("channel") or "whatsapp")
        row = db.execute(
            text(
                """
                insert into messages (
                    id, user_id, run_id, channel, direction, content,
                    input_modality, original_media_url, transcription_confidence,
                    extracted_entities, content_provenance, emotion_detected, wa_message_id,
                    metadata, created_at
                ) values (
                    :id::uuid,
                    :user_id::uuid,
                    :run_id::uuid,
                    :channel::channel_type,
                    :direction::message_direction,
                    :content,
                    :input_modality::input_modality,
                    :original_media_url,
                    :transcription_confidence,
                    :extracted_entities::jsonb,
                    :content_provenance::content_provenance,
                    :emotion_detected::emotion_state,
                    :wa_message_id,
                    :metadata::jsonb,
                    now()
                )
                returning id
                """
            ),
            {
                "id": msg_id,
                "user_id": user_id,
                "run_id": run_id,
                "channel": channel_value,
                "direction": direction.value,
                "content": text_content,
                "input_modality": str((content or {}).get("input_modality") or "text"),
                "original_media_url": (content or {}).get("media_url"),
                "transcription_confidence": (content or {}).get("transcription_confidence"),
                "extracted_entities": json.dumps((content or {}).get("extracted_entities") or {}, ensure_ascii=False),
                "content_provenance": str((content or {}).get("content_provenance") or "user_direct"),
                "emotion_detected": str((content or {}).get("emotion_detected") or "neutral"),
                "wa_message_id": channel_msg_id,
                "metadata": json.dumps({"intent": intent, "tier": tier, "latency_ms": latency_ms}, ensure_ascii=False),
            },
        ).mappings().first()
        db.commit()
        return str((row or {}).get("id") or msg_id)
    except IntegrityError:
        # Likely message dedup conflict.
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
    if not _column_exists(db, "messages", "run_id"):
        return
    db.execute(text("update messages set run_id = :run_id where id = :id"), {"id": message_id, "run_id": run_id})
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

    has_conversation = _column_exists(db, "runs", "conversation_id")
    has_envelope = _column_exists(db, "runs", "envelope")
    has_llm_provider = _column_exists(db, "runs", "llm_provider")
    has_cost_cents = _column_exists(db, "runs", "cost_cents")
    has_latency_ms = _column_exists(db, "runs", "latency_ms")

    if has_conversation and has_envelope:
        db.execute(
            text(
                "insert into runs "
                "(id, user_id, conversation_id, parent_run_id, intent, tier, plan, state, envelope, created_at) "
                "values "
                "(:id, :user_id, :conversation_id, :parent_run_id, :intent, :tier, '[]', 'pending', :envelope, now())"
            ),
            {
                "id": run_id,
                "user_id": user_id,
                "conversation_id": conversation_id if is_uuid(conversation_id) else None,
                "parent_run_id": parent_run_id if (parent_run_id and is_uuid(parent_run_id)) else None,
                "intent": intent,
                "tier": tier,
                "envelope": json.dumps(envelope or {}, ensure_ascii=False),
            },
        )
        db.commit()
        return run_id

    columns = ["id", "user_id", "intent", "tier", "state", "created_at"]
    values = [":id", ":user_id", ":intent", ":tier", "'pending'", "now()"]
    params: dict[str, Any] = {
        "id": run_id,
        "user_id": user_id,
        "intent": intent,
        "tier": tier,
    }

    if _column_exists(db, "runs", "parent_run_id"):
        columns.append("parent_run_id")
        values.append(":parent_run_id")
        params["parent_run_id"] = parent_run_id if (parent_run_id and is_uuid(parent_run_id)) else None

    if _column_exists(db, "runs", "plan"):
        columns.append("plan")
        values.append(":plan")
        params["plan"] = json.dumps(envelope or {}, ensure_ascii=False)

    if has_llm_provider:
        columns.append("llm_provider")
        values.append(":llm_provider")
        params["llm_provider"] = str((envelope or {}).get("llm_provider") or "openai")

    if has_cost_cents:
        columns.append("cost_cents")
        values.append(":cost_cents")
        params["cost_cents"] = 0

    if has_latency_ms:
        columns.append("latency_ms")
        values.append(":latency_ms")
        params["latency_ms"] = 0

    db.execute(
        text(
            f"insert into runs ({', '.join(columns)}) "
            f"values ({', '.join(values)})"
        ),
        params,
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
    llm_provider: str | None = None,
    knowledge_files_injected: list[str] | None = None,
) -> None:
    """
    Mark a run terminal (completed/failed/cancelled).
    """
    set_app_user_id(db, user_id)
    has_legacy_totals = _column_exists(db, "runs", "total_cost_cents") and _column_exists(db, "runs", "total_latency_ms")

    if has_legacy_totals:
        db.execute(
            text(
                "update runs "
                "set state = :state, "
                "    error = :error, "
                "    total_cost_cents = :total_cost_cents, "
                "    total_latency_ms = :total_latency_ms, "
                "    completed_at = now() "
                "where id = :id"
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
        return

    assignments = ["state = :state", "completed_at = now()"]
    params: dict[str, Any] = {"id": run_id, "state": state}

    if _column_exists(db, "runs", "error"):
        assignments.append("error = :error")
        params["error"] = json.dumps(error, ensure_ascii=False) if error else None
    if _column_exists(db, "runs", "cost_cents"):
        assignments.append("cost_cents = :cost_cents")
        params["cost_cents"] = total_cost_cents
    if _column_exists(db, "runs", "latency_ms"):
        assignments.append("latency_ms = :latency_ms")
        params["latency_ms"] = total_latency_ms
    if llm_provider and _column_exists(db, "runs", "llm_provider"):
        assignments.append("llm_provider = :llm_provider")
        params["llm_provider"] = llm_provider
    if knowledge_files_injected is not None and _column_exists(db, "runs", "knowledge_files_injected"):
        assignments.append("knowledge_files_injected = :knowledge_files_injected")
        params["knowledge_files_injected"] = knowledge_files_injected

    db.execute(text(f"update runs set {', '.join(assignments)} where id = :id"), params)
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
    if _column_exists(db, "messages", "conversation_id"):
        rows = (
            db.execute(
                text(
                    "select direction, content "
                    "from messages "
                    "where conversation_id = :conversation_id "
                    "order by created_at desc "
                    "limit :limit"
                ),
                {"conversation_id": conversation_id if is_uuid(conversation_id) else None, "limit": int(limit)},
            )
            .mappings()
            .all()
        )
    else:
        rows = (
            db.execute(
                text(
                    "select direction, content "
                    "from messages "
                    "where user_id = :user_id "
                    "order by created_at desc "
                    "limit :limit"
                ),
                {"user_id": user_id, "limit": int(limit)},
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
        elif isinstance(raw_content, str) and raw_content and raw_content.lstrip().startswith("{"):
            try:
                content = json.loads(raw_content or "{}")
            except Exception:
                content = {"text": raw_content}
        elif isinstance(raw_content, str):
            content = {"text": raw_content}
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
    output_col = "output" if _column_exists(db, "tool_executions", "output") else "result"
    error_col = "error" if _column_exists(db, "tool_executions", "error") else "null"
    row = (
        db.execute(
            text(
                f"select tool_name, status, {output_col} as output, {error_col} as error "
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
    compensating_action: dict[str, Any] | None = None,
) -> str | None:
    """
    Insert tool execution row. Returns id, or None if deduped by idempotency_key.
    """
    set_app_user_id(db, user_id)
    tool_id = str(uuid.uuid4())
    try:
        is_legacy = _column_exists(db, "tool_executions", "input_hash") and _column_exists(db, "tool_executions", "input")

        if is_legacy:
            row = db.execute(
                text(
                    "insert into tool_executions "
                    "(id, run_id, user_id, tool_name, input_hash, input, output, status, error, idempotency_key, risk_level, cost_cents, latency_ms, created_at) "
                    "values "
                    "(:id, :run_id, :user_id, :tool_name, :input_hash, :input, :output, :status, :error, :idempotency_key, :risk_level, :cost_cents, :latency_ms, now()) "
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
        else:
            execution_result = output_payload if output_payload is not None else ({"error": error_payload} if error_payload else None)
            row = db.execute(
                text(
                    "insert into tool_executions "
                    "(id, run_id, user_id, tool_name, is_mcp, mcp_server_id, arguments, input_provenance, result, status, risk_level, idempotency_key, compensating_action, cost_cents, latency_ms, created_at) "
                    "values "
                    "(:id, :run_id, :user_id, :tool_name, :is_mcp, :mcp_server_id, :arguments, :input_provenance, :result, :status, :risk_level, :idempotency_key, :compensating_action, :cost_cents, :latency_ms, now()) "
                    "returning id"
                ),
                {
                    "id": tool_id,
                    "run_id": run_id,
                    "user_id": user_id,
                    "tool_name": tool_name,
                    "is_mcp": bool(input_payload.get("is_mcp")) if isinstance(input_payload, dict) else False,
                    "mcp_server_id": (input_payload.get("mcp_server_id") if isinstance(input_payload, dict) else None),
                    "arguments": json.dumps(input_payload, ensure_ascii=False),
                    "input_provenance": str((input_payload or {}).get("input_provenance") or "user_direct"),
                    "result": json.dumps(execution_result, ensure_ascii=False) if execution_result is not None else None,
                    "status": status,
                    "risk_level": risk_level,
                    "idempotency_key": idempotency_key,
                    "compensating_action": json.dumps((error_payload or {}).get("compensating_action"), ensure_ascii=False)
                    if isinstance(error_payload, dict) and error_payload.get("compensating_action") is not None
                    else (json.dumps(compensating_action, ensure_ascii=False) if compensating_action is not None else None),
                    "cost_cents": int(cost_cents),
                    "latency_ms": int(latency_ms),
                },
            ).mappings().first()
        db.commit()
        return str((row or {}).get("id") or tool_id)
    except IntegrityError:
        db.rollback()
        return None


def record_side_effect(
    db: Session,
    *,
    run_id: str | None,
    user_id: str,
    effect_type: str,
    description: str,
    metadata: dict[str, Any] | None = None,
    reversible: bool = False,
) -> str | None:
    if not _table_exists(db, "side_effects"):
        return None
    side_effect_id = str(uuid.uuid4())
    try:
        db.execute(
            text(
                """
                insert into side_effects (
                    id, run_id, user_id, effect_type, description, reversible, metadata, created_at
                ) values (
                    :id, :run_id, :user_id, :effect_type, :description, :reversible, :metadata, now()
                )
                """
            ),
            {
                "id": side_effect_id,
                "run_id": run_id if (run_id and is_uuid(run_id)) else None,
                "user_id": user_id,
                "effect_type": effect_type,
                "description": description,
                "reversible": bool(reversible),
                "metadata": json.dumps(metadata or {}, ensure_ascii=False),
            },
        )
        db.commit()
        return side_effect_id
    except Exception:
        db.rollback()
        return None
