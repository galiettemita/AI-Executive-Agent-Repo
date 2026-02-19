from __future__ import annotations

import json
from datetime import datetime, timedelta
from typing import Any

from sqlalchemy import text
from sqlalchemy.orm import Session

_OPT_IN_KEY = "share_anonymized_insights"


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
        row = db.execute(text("select name from sqlite_master where type='table' and name=:name"), {"name": table_name}).first()
        return bool(row)
    except Exception:
        return False


def _parse_json(value: Any, default: Any) -> Any:
    if isinstance(value, (dict, list)):
        return value
    if isinstance(value, str):
        try:
            return json.loads(value)
        except Exception:
            return default
    return default


def opted_in_user_ids(db: Session) -> list[str]:
    if not _table_exists(db, "preferences"):
        return []
    rows = db.execute(text("select user_id, data_json from preferences")).mappings().all()
    out: list[str] = []
    for row in rows:
        user_id = str(row.get("user_id") or "").strip()
        if not user_id:
            continue
        data = _parse_json(row.get("data_json"), {})
        if isinstance(data, dict) and bool(data.get(_OPT_IN_KEY)):
            out.append(user_id)
    return sorted(set(out))


def summarize_anonymized_insights(db: Session, *, days: int = 30) -> dict[str, Any]:
    window_days = max(1, int(days or 30))
    since = datetime.utcnow() - timedelta(days=window_days)
    opted_in = set(opted_in_user_ids(db))

    summary = {
        "window_days": window_days,
        "opted_in_users": len(opted_in),
        "events_total": 0,
        "top_events": {},
        "top_servers": {},
        "feedback": {
            "available": False,
            "total": 0,
            "positive": 0,
            "negative": 0,
            "positive_rate_pct": 0.0,
        },
    }

    if not opted_in:
        return summary

    if _table_exists(db, "analytics_events"):
        rows = db.execute(
            text(
                "select user_id, event_name, payload_json, created_at "
                "from analytics_events where created_at >= :since"
            ),
            {"since": since.isoformat()},
        ).mappings().all()
        event_counts: dict[str, int] = {}
        server_counts: dict[str, int] = {}
        total = 0
        for row in rows:
            user_id = str(row.get("user_id") or "").strip()
            if user_id not in opted_in:
                continue
            total += 1
            event_name = str(row.get("event_name") or "unknown").strip().lower()
            event_counts[event_name] = event_counts.get(event_name, 0) + 1
            payload = _parse_json(row.get("payload_json"), {})
            if isinstance(payload, dict):
                server_id = str(payload.get("server_id") or payload.get("mcp_server_id") or "").strip()
                if server_id:
                    server_counts[server_id] = server_counts.get(server_id, 0) + 1
        summary["events_total"] = total
        summary["top_events"] = dict(sorted(event_counts.items(), key=lambda item: item[1], reverse=True)[:10])
        summary["top_servers"] = dict(sorted(server_counts.items(), key=lambda item: item[1], reverse=True)[:10])

    if _table_exists(db, "user_feedback"):
        rows = db.execute(
            text(
                "select user_id, feedback from user_feedback where created_at >= :since"
            ),
            {"since": since.isoformat()},
        ).mappings().all()
        total = 0
        pos = 0
        neg = 0
        for row in rows:
            user_id = str(row.get("user_id") or "").strip()
            if user_id not in opted_in:
                continue
            total += 1
            fb = str(row.get("feedback") or "").strip().lower()
            if fb in {"up", "positive"}:
                pos += 1
            elif fb in {"down", "negative"}:
                neg += 1
        rate = (pos / max(1, total)) * 100.0
        summary["feedback"] = {
            "available": True,
            "total": total,
            "positive": pos,
            "negative": neg,
            "positive_rate_pct": round(rate, 2),
        }

    return summary


def opt_in_key() -> str:
    return _OPT_IN_KEY
