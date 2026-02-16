from __future__ import annotations

import json
from datetime import datetime
from typing import Any

from sqlalchemy import text
from sqlalchemy.orm import Session


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


def _infer_rule_key(signal_type: str, corrected_output: str | None, context: dict[str, Any]) -> str:
    explicit = str((context or {}).get("rule_key") or "").strip()
    if explicit:
        return explicit
    text = str(corrected_output or "").lower()
    if any(k in text for k in ("concise", "shorter", "brief")):
        return "style.conciseness"
    if any(k in text for k in ("formal", "professional", "tone")):
        return "style.tone"
    if signal_type in {"approved", "praise", "outcome_success"}:
        return "autonomy.default"
    if signal_type in {"override", "edit", "correction", "complaint", "outcome_failed"}:
        return "autonomy.guardrails"
    return "style.default"


def _confidence_for_signal(signal_type: str) -> float:
    if signal_type in {"approved", "praise", "outcome_success"}:
        return 0.65
    if signal_type in {"override", "edit", "correction"}:
        return 0.8
    if signal_type in {"complaint", "outcome_failed"}:
        return 0.9
    return 0.55


def record_feedback_signal(
    db: Session,
    *,
    user_id: str,
    signal_type: str,
    original_output: str | None = None,
    corrected_output: str | None = None,
    context: dict[str, Any] | None = None,
) -> dict[str, Any]:
    ctx = context or {}
    created_at = datetime.utcnow().isoformat()
    if not _table_exists(db, "feedback_signals"):
        return {"ok": False, "reason": "feedback_signals_table_missing"}

    db.execute(
        text(
            """
            insert into feedback_signals (
                user_id, signal_type, original_output, corrected_output, context, created_at
            ) values (
                :user_id, :signal_type, :original_output, :corrected_output, :context, :created_at
            )
            """
        ),
        {
            "user_id": user_id,
            "signal_type": signal_type,
            "original_output": original_output,
            "corrected_output": corrected_output,
            "context": json.dumps(ctx, ensure_ascii=False),
            "created_at": created_at,
        },
    )

    learning_applied = False
    if _table_exists(db, "behavioral_rules"):
        tracked_types = {
            "correction",
            "override",
            "edit",
            "complaint",
            "approved",
            "praise",
            "outcome_success",
            "outcome_failed",
        }
        if signal_type in tracked_types:
            file_path = str(ctx.get("file_path") or "AGENTS.md")
            category = str(ctx.get("category") or ("outcome" if signal_type.startswith("outcome_") else "correction"))
            rule_key = _infer_rule_key(signal_type, corrected_output, ctx)
            candidate_value = (corrected_output or original_output or str(ctx.get("note") or "")).strip()
            if not candidate_value:
                if signal_type in {"approved", "praise", "outcome_success"}:
                    candidate_value = "Preference reinforced: keep current behavior."
                elif signal_type in {"outcome_failed", "complaint"}:
                    candidate_value = "Preference reinforced: increase caution and ask for confirmation."
                else:
                    candidate_value = "Preference updated from user feedback."

            db.execute(
                text(
                    """
                    insert into behavioral_rules (
                        user_id, file_path, category, rule_key, rule_value, source, confidence, created_at
                    ) values (
                        :user_id, :file_path, :category, :rule_key, :rule_value, :source, :confidence, :created_at
                    )
                    """
                ),
                {
                    "user_id": user_id,
                    "file_path": file_path,
                    "category": category,
                    "rule_key": rule_key,
                    "rule_value": candidate_value[:400],
                    "source": "inferred",
                    "confidence": _confidence_for_signal(signal_type),
                    "created_at": created_at,
                },
            )
            learning_applied = True

    db.commit()
    return {"ok": True, "learning_applied": learning_applied}
