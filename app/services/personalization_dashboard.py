from __future__ import annotations

from datetime import datetime, timedelta
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
        row = db.execute(text("select name from sqlite_master where type='table' and name=:name"), {"name": table_name}).first()
        return bool(row)
    except Exception:
        return False


def _table_columns(db: Session, table_name: str) -> set[str]:
    cols: set[str] = set()
    try:
        rows = db.execute(
            text(
                "select column_name from information_schema.columns "
                "where table_schema = current_schema() and table_name = :name"
            ),
            {"name": table_name},
        ).all()
        cols.update(str(row[0]) for row in rows if row and row[0])
    except Exception:
        pass

    if cols:
        return cols

    try:
        rows = db.execute(text(f"pragma table_info({table_name})")).all()
        for row in rows:
            name = row[1] if len(row) > 1 else None
            if name:
                cols.add(str(name))
    except Exception:
        pass
    return cols


def _time_clause(db: Session, table_name: str, since: datetime) -> tuple[str, dict[str, Any]]:
    cols = _table_columns(db, table_name)
    if "created_at" in cols:
        return "created_at >= :since", {"since": since.isoformat()}
    return "1=1", {}


def _safe_row(db: Session, sql: str, params: dict[str, Any]) -> dict[str, Any]:
    try:
        row = db.execute(text(sql), params).mappings().first()
        return dict(row or {})
    except Exception:
        return {}


def get_personalization_dashboard(db: Session, *, days: int = 30) -> dict[str, Any]:
    window_days = max(1, int(days or 30))
    since = datetime.utcnow() - timedelta(days=window_days)

    profiling = {
        "available": False,
        "total_sessions": 0,
        "completed_sessions": 0,
        "coverage_pct": 0.0,
    }
    if _table_exists(db, "profiling_sessions"):
        where, params = _time_clause(db, "profiling_sessions", since)
        cols = _table_columns(db, "profiling_sessions")
        progress_clause = "progress_pct >= 100" if "progress_pct" in cols else "1=0"
        sql = (
            "select count(*) as total, "
            "sum(case when status = 'completed' or " + progress_clause + " then 1 else 0 end) as completed "
            "from profiling_sessions where " + where
        )
        row = _safe_row(db, sql, params)
        total = int(row.get("total") or 0)
        completed = int(row.get("completed") or 0)
        profiling.update(
            {
                "available": True,
                "total_sessions": total,
                "completed_sessions": completed,
                "coverage_pct": round((completed / max(1, total)) * 100.0, 2),
            }
        )

    corrections = {
        "available": False,
        "total_signals": 0,
        "correction_signals": 0,
        "correction_rate_pct": 0.0,
        "knowledge_accuracy_estimate": 0.0,
    }
    if _table_exists(db, "feedback_signals"):
        where, params = _time_clause(db, "feedback_signals", since)
        sql = (
            "select count(*) as total, "
            "sum(case when signal_type in ("
            "'correction','edit','override','complaint','outcome_failed'"
            ") then 1 else 0 end) as corrections "
            "from feedback_signals where " + where
        )
        row = _safe_row(db, sql, params)
        total = int(row.get("total") or 0)
        correction_count = int(row.get("corrections") or 0)
        correction_rate = (correction_count / max(1, total)) * 100.0
        accuracy_estimate = max(0.0, 1.0 - (correction_count / max(1, total)))
        corrections.update(
            {
                "available": True,
                "total_signals": total,
                "correction_signals": correction_count,
                "correction_rate_pct": round(correction_rate, 2),
                "knowledge_accuracy_estimate": round(accuracy_estimate, 3),
            }
        )

    satisfaction = {
        "available": False,
        "total_feedback": 0,
        "positive": 0,
        "negative": 0,
        "satisfaction_rate_pct": 0.0,
    }
    if _table_exists(db, "user_feedback"):
        where, params = _time_clause(db, "user_feedback", since)
        sql = (
            "select count(*) as total, "
            "sum(case when feedback in ('up','positive') then 1 else 0 end) as positive, "
            "sum(case when feedback in ('down','negative') then 1 else 0 end) as negative "
            "from user_feedback where " + where
        )
        row = _safe_row(db, sql, params)
        total = int(row.get("total") or 0)
        pos = int(row.get("positive") or 0)
        neg = int(row.get("negative") or 0)
        satisfaction_rate = (pos / max(1, total)) * 100.0
        satisfaction.update(
            {
                "available": True,
                "total_feedback": total,
                "positive": pos,
                "negative": neg,
                "satisfaction_rate_pct": round(satisfaction_rate, 2),
            }
        )

    message_volume = {
        "available": False,
        "message_received": 0,
        "corrections_per_100_messages": 0.0,
    }
    if _table_exists(db, "analytics_events"):
        where, params = _time_clause(db, "analytics_events", since)
        sql = (
            "select count(*) as total "
            "from analytics_events where event_name = 'message_received' and " + where
        )
        row = _safe_row(db, sql, params)
        msg_count = int(row.get("total") or 0)
        correction_count = int(corrections.get("correction_signals") or 0)
        per_100 = (correction_count / max(1, msg_count)) * 100.0
        message_volume.update(
            {
                "available": True,
                "message_received": msg_count,
                "corrections_per_100_messages": round(per_100, 2),
            }
        )

    return {
        "window_days": window_days,
        "profiling": profiling,
        "knowledge_accuracy": corrections,
        "satisfaction": satisfaction,
        "correction_frequency": message_volume,
    }
