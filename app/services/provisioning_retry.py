from __future__ import annotations

import json
from typing import Any

from sqlalchemy import text
from sqlalchemy.orm import Session

from app.blueprint.brain.responder import generate_reply
from app.blueprint.brain.tier_router import route_tier
from app.blueprint.contracts import ContentProvenance, ServerProvisionedEvent
from app.blueprint.progress import append_progress, set_run_result
from app.db.database import SessionLocal


def _table_exists(db: Session, table_name: str) -> bool:
    try:
        row = db.execute(
            text(
                "select 1 from information_schema.tables "
                "where table_schema = current_schema() and table_name = :name"
            ),
            {"name": table_name},
        ).first()
        if row:
            return True
    except Exception:
        pass
    try:
        row = db.execute(
            text("select name from sqlite_master where type='table' and name=:name"),
            {"name": table_name},
        ).first()
        return bool(row)
    except Exception:
        return False


def _extract_text(content_value: Any) -> str:
    if isinstance(content_value, dict):
        return str(content_value.get("text") or "").strip()
    if isinstance(content_value, str):
        raw = content_value.strip()
        if not raw:
            return ""
        try:
            parsed = json.loads(raw)
            if isinstance(parsed, dict):
                return str(parsed.get("text") or "").strip() or raw
        except Exception:
            return raw
        return raw
    return ""


def _load_original_task_context(db: Session, *, user_id: str, run_id: str) -> dict[str, Any] | None:
    if not _table_exists(db, "messages"):
        return None
    dialect = db.bind.dialect.name if db.bind is not None else ""
    if dialect == "sqlite":
        row = db.execute(
            text(
                """
                select conversation_id, channel, content, tier, metadata
                from messages
                where user_id = :user_id
                  and run_id = :run_id
                  and direction = 'inbound'
                order by created_at desc
                limit 1
                """
            ),
            {"user_id": user_id, "run_id": run_id},
        ).mappings().first()
    else:
        row = db.execute(
            text(
                """
                select conversation_id::text as conversation_id, channel::text as channel, content, tier, metadata
                from messages
                where user_id::text = :user_id
                  and run_id::text = :run_id
                  and direction = 'inbound'::message_direction
                order by created_at desc
                limit 1
                """
            ),
            {"user_id": user_id, "run_id": run_id},
        ).mappings().first()
    if not row:
        return None
    item = dict(row)
    content_value = item.get("content")
    item["text"] = _extract_text(content_value)
    return item


def retry_original_task_after_provisioning(event: ServerProvisionedEvent) -> dict[str, Any]:
    if not event.original_task_id:
        return {"ok": False, "reason": "missing_original_task_id"}

    db = SessionLocal()
    try:
        context = _load_original_task_context(
            db,
            user_id=event.user_id,
            run_id=event.original_task_id,
        )
        if not context:
            return {"ok": False, "reason": "original_task_not_found"}

        user_text = str(context.get("text") or "").strip()
        if not user_text:
            return {"ok": False, "reason": "original_task_empty"}

        tier_value = context.get("tier")
        tier = int(tier_value) if isinstance(tier_value, int) else route_tier(user_text)
        conversation_id = str(context.get("conversation_id") or "").strip() or None

        reply, meta = generate_reply(
            user_text=user_text,
            tier=tier,
            user_id=event.user_id,
            conversation_id=conversation_id,
            run_id=event.original_task_id,
            input_provenance=ContentProvenance.USER_DIRECT.value,
        )
        prefixed = f"Server connected! ✓ {reply}".strip()
        payload = {
            "ok": True,
            "retry_from_run_id": event.original_task_id,
            "server_id": event.server_id,
            "reply": prefixed,
            "metadata": meta or {},
        }
        set_run_result(event.original_task_id, payload)
        append_progress(
            event.original_task_id,
            step="provisioning_retry",
            status="completed",
            partial_result={"reply": prefixed, "server_id": event.server_id},
        )
        return payload
    except Exception as exc:
        append_progress(
            event.original_task_id,
            step="provisioning_retry",
            status="failed",
            partial_result={"error": str(exc), "server_id": event.server_id},
        )
        return {"ok": False, "reason": "retry_failed", "error": str(exc)}
    finally:
        try:
            db.close()
        except Exception:
            pass
