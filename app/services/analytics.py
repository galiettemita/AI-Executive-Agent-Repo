from __future__ import annotations

import json
import threading
import uuid
from concurrent.futures import ThreadPoolExecutor
from datetime import date, datetime, timedelta, timezone
from typing import Any

from sqlalchemy import text
from sqlalchemy.orm import Session

from app.db.database import SessionLocal

_LOCK = threading.Lock()
_READY = False
_EXECUTOR = ThreadPoolExecutor(max_workers=3, thread_name_prefix="analytics-events")

_WAVE56_SERVER_IDS: tuple[str, ...] = (
    "duffel-mcp",
    "zoom-mcp",
    "calendly-mcp",
    "plaid-mcp",
    "crunchbase-mcp",
    "booking-com-mcp",
    "docusign-mcp",
    "canva-mcp",
    "instacart-mcp",
    "tesla-mcp",
)

_WAVE14_SERVER_IDS: tuple[str, ...] = (
    "google-calendar-mcp",
    "google-drive-mcp",
    "gmail-mcp",
    "notion-mcp",
    "todoist-mcp",
    "brave-search-mcp",
    "github-mcp",
    "apple-reminders-mcp",
    "slack-mcp",
    "outlook-mcp",
    "teams-mcp",
    "linear-mcp",
    "asana-mcp",
    "discord-mcp",
    "whatsapp-business-mcp",
    "stripe-mcp",
    "quickbooks-mcp",
    "hubspot-mcp",
    "salesforce-mcp",
    "google-sheets-mcp",
    "airtable-mcp",
    "jira-mcp",
    "sentry-mcp",
    "google-maps-mcp",
    "uber-lyft-mcp",
    "opentable-resy-mcp",
    "homeassistant-mcp",
    "spotify-mcp",
    "evernote-mcp",
    "dropbox-mcp",
)

_WAVE56_WEIGHTS: dict[str, int] = {
    "provisioning_requested": 5,
    "provisioning_declined": 3,
    "awaiting_auth": 2,
    "provisioning_failed": 2,
    "server_provisioned": 4,
    "mcp_server_connected": 4,
    "tool_invoked": 1,
}

_WAVE14_WEIGHTS: dict[str, int] = {
    "provisioning_requested": 5,
    "provisioning_declined": 3,
    "awaiting_auth": 2,
    "provisioning_failed": 2,
    "server_provisioned": 4,
    "mcp_server_connected": 4,
    "tool_invoked": 1,
    "message_received": 1,
}


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
            # SQLite pragma row shape: (cid, name, type, notnull, dflt_value, pk)
            name = row[1] if len(row) > 1 else None
            if name:
                cols.add(str(name))
    except Exception:
        pass
    return cols


def ensure_analytics_tables(db: Session) -> None:
    global _READY
    if _READY and _table_exists(db, "analytics_events") and _table_exists(db, "analytics_daily"):
        return
    with _LOCK:
        if _READY and _table_exists(db, "analytics_events") and _table_exists(db, "analytics_daily"):
            return
        dialect = db.bind.dialect.name if db.bind is not None else ""
        if dialect == "sqlite":
            db.execute(
                text(
                    """
                    create table if not exists analytics_events (
                      id text primary key,
                      user_id text,
                      event_name text not null,
                      source text,
                      payload_json text,
                      created_at datetime default current_timestamp
                    )
                    """
                )
            )
            db.execute(
                text(
                    """
                    create table if not exists analytics_daily (
                      id text primary key,
                      day text not null,
                      dau integer default 0,
                      mau integer default 0,
                      message_volume integer default 0,
                      tool_calls integer default 0,
                      avg_quality_score real default 0,
                      revenue_cents integer default 0,
                      metadata_json text,
                      created_at datetime default current_timestamp
                    )
                    """
                )
            )
            db.execute(text("create index if not exists idx_analytics_events_created on analytics_events(created_at)"))
            db.execute(text("create index if not exists idx_analytics_events_name on analytics_events(event_name, created_at)"))
            db.execute(text("create unique index if not exists uq_analytics_daily_day on analytics_daily(day)"))
        else:
            db.execute(
                text(
                    """
                    create table if not exists analytics_events (
                      id text primary key,
                      user_id text,
                      event_name text not null,
                      source text,
                      payload_json jsonb,
                      created_at timestamptz default now()
                    )
                    """
                )
            )
            db.execute(
                text(
                    """
                    create table if not exists analytics_daily (
                      id text primary key,
                      day date not null,
                      dau integer default 0,
                      mau integer default 0,
                      message_volume integer default 0,
                      tool_calls integer default 0,
                      avg_quality_score double precision default 0,
                      revenue_cents bigint default 0,
                      metadata_json jsonb,
                      created_at timestamptz default now()
                    )
                    """
                )
            )
            db.execute(text("create index if not exists idx_analytics_events_created on analytics_events(created_at)"))
            db.execute(text("create index if not exists idx_analytics_events_name on analytics_events(event_name, created_at)"))
            db.execute(text("create unique index if not exists uq_analytics_daily_day on analytics_daily(day)"))
        db.commit()
        _READY = True


def emit_event(
    db: Session,
    *,
    event_name: str,
    user_id: str | None = None,
    source: str | None = None,
    payload: dict[str, Any] | None = None,
) -> str:
    ensure_analytics_tables(db)
    event_id = str(uuid.uuid4())
    payload_json = json.dumps(payload or {}, ensure_ascii=False)
    dialect = db.bind.dialect.name if db.bind is not None else ""
    params = {
        "id": event_id,
        "user_id": user_id,
        "event_name": event_name,
        "source": source,
        "payload_json": payload_json,
    }
    if dialect == "sqlite":
        db.execute(
            text(
                """
                insert into analytics_events (
                  id, user_id, event_name, source, payload_json
                ) values (
                  :id, :user_id, :event_name, :source, :payload_json
                )
                """
            ),
            params,
        )
    else:
        db.execute(
            text(
                """
                insert into analytics_events (
                  id, user_id, event_name, source, payload_json
                ) values (
                  :id, :user_id, :event_name, :source, cast(:payload_json as jsonb)
                )
                """
            ),
            params,
        )
    db.commit()
    return event_id


def emit_event_async(
    *,
    event_name: str,
    user_id: str | None = None,
    source: str | None = None,
    payload: dict[str, Any] | None = None,
) -> None:
    def _job() -> None:
        db = SessionLocal()
        try:
            emit_event(db, event_name=event_name, user_id=user_id, source=source, payload=payload)
        finally:
            db.close()

    _EXECUTOR.submit(_job)


def aggregate_daily(db: Session, *, for_day: date | None = None) -> dict[str, Any]:
    ensure_analytics_tables(db)
    day = for_day or datetime.now(timezone.utc).date()
    start = datetime.combine(day, datetime.min.time(), tzinfo=timezone.utc)
    end = start + timedelta(days=1)
    month_start = day - timedelta(days=30)

    dau_row = db.execute(
        text(
            "select count(distinct user_id) as c from analytics_events "
            "where created_at >= :start and created_at < :end and user_id is not null"
        ),
        {"start": start, "end": end},
    ).mappings().first()
    mau_row = db.execute(
        text(
            "select count(distinct user_id) as c from analytics_events "
            "where created_at >= :month_start and created_at < :end and user_id is not null"
        ),
        {"month_start": datetime.combine(month_start, datetime.min.time(), tzinfo=timezone.utc), "end": end},
    ).mappings().first()
    volume_row = db.execute(
        text("select count(*) as c from analytics_events where created_at >= :start and created_at < :end"),
        {"start": start, "end": end},
    ).mappings().first()
    tool_row = db.execute(
        text(
            "select count(*) as c from analytics_events "
            "where created_at >= :start and created_at < :end and event_name = 'tool_invoked'"
        ),
        {"start": start, "end": end},
    ).mappings().first()
    quality_row = None
    if _table_exists(db, "eval_results"):
        quality_row = db.execute(
            text("select avg(overall_score) as avg_quality from eval_results where created_at >= :start and created_at < :end"),
            {"start": start, "end": end},
        ).mappings().first()
    revenue_row = None
    if _table_exists(db, "invoices"):
        invoice_cols = _table_columns(db, "invoices")
        amount_col = next(
            (
                col
                for col in (
                    "amount_paid_cents",
                    "amount_paid",
                    "amount_due_cents",
                    "amount_due",
                    "total_cents",
                    "total_amount_cents",
                )
                if col in invoice_cols
            ),
            None,
        )
        if amount_col:
            where_parts: list[str] = []
            params: dict[str, Any] = {}
            if "created_at" in invoice_cols:
                where_parts.append("created_at >= :start and created_at < :end")
                params["start"] = start
                params["end"] = end
            elif "paid_at" in invoice_cols:
                where_parts.append("paid_at >= :start and paid_at < :end")
                params["start"] = start
                params["end"] = end
            if "status" in invoice_cols:
                where_parts.append("status in ('paid', 'succeeded')")
            sql = f"select coalesce(sum({amount_col}), 0) as revenue from invoices"
            if where_parts:
                sql += " where " + " and ".join(where_parts)
            revenue_row = db.execute(text(sql), params).mappings().first()

    data = {
        "id": str(uuid.uuid4()),
        "day": day.isoformat(),
        "dau": int((dau_row or {}).get("c") or 0),
        "mau": int((mau_row or {}).get("c") or 0),
        "message_volume": int((volume_row or {}).get("c") or 0),
        "tool_calls": int((tool_row or {}).get("c") or 0),
        "avg_quality_score": float((quality_row or {}).get("avg_quality") or 0.0),
        "revenue_cents": int((revenue_row or {}).get("revenue") or 0),
        "metadata_json": json.dumps({"generated_at": datetime.utcnow().isoformat()}, ensure_ascii=False),
    }

    dialect = db.bind.dialect.name if db.bind is not None else ""
    if dialect == "sqlite":
        db.execute(text("delete from analytics_daily where day = :day"), {"day": data["day"]})
        db.execute(
            text(
                """
                insert into analytics_daily (
                  id, day, dau, mau, message_volume, tool_calls, avg_quality_score, revenue_cents, metadata_json
                ) values (
                  :id, :day, :dau, :mau, :message_volume, :tool_calls, :avg_quality_score, :revenue_cents, :metadata_json
                )
                """
            ),
            data,
        )
    else:
        db.execute(text("delete from analytics_daily where day = :day"), {"day": data["day"]})
        db.execute(
            text(
                """
                insert into analytics_daily (
                  id, day, dau, mau, message_volume, tool_calls, avg_quality_score, revenue_cents, metadata_json
                ) values (
                  :id, cast(:day as date), :dau, :mau, :message_volume, :tool_calls, :avg_quality_score, :revenue_cents, cast(:metadata_json as jsonb)
                )
                """
            ),
            data,
        )
    db.commit()
    return data


def _parse_payload_json(value: Any) -> dict[str, Any]:
    if isinstance(value, dict):
        return value
    if isinstance(value, str) and value.strip():
        try:
            parsed = json.loads(value)
            if isinstance(parsed, dict):
                return parsed
        except Exception:
            return {}
    return {}


def _server_prioritization(
    db: Session,
    *,
    days: int,
    server_ids: tuple[str, ...],
    weights: dict[str, int],
) -> dict[str, Any]:
    ensure_analytics_tables(db)
    window_days = max(1, min(365, int(days or 30)))
    cutoff = datetime.now(timezone.utc) - timedelta(days=window_days)
    rows = db.execute(
        text(
            "select user_id, event_name, payload_json "
            "from analytics_events "
            "where created_at >= :cutoff"
        ),
        {"cutoff": cutoff},
    ).mappings().all()

    state: dict[str, dict[str, Any]] = {
        server_id: {
            "server_id": server_id,
            "demand_score": 0,
            "event_count": 0,
            "weighted_events": {},
            "distinct_users": set(),
            "requested_count": 0,
            "provisioned_count": 0,
        }
        for server_id in server_ids
    }
    server_id_set = set(server_ids)

    for row in rows:
        event_name = str(row.get("event_name") or "").strip()
        if not event_name:
            continue
        payload = _parse_payload_json(row.get("payload_json"))
        target_server = str(
            payload.get("server_id")
            or payload.get("mcp_server_id")
            or payload.get("provider")
            or ""
        ).strip().lower()
        if target_server not in server_id_set:
            continue

        weight = int(weights.get(event_name, 0))
        bucket = state[target_server]
        bucket["event_count"] = int(bucket["event_count"] or 0) + 1
        if event_name == "provisioning_requested":
            bucket["requested_count"] = int(bucket.get("requested_count") or 0) + 1
        elif event_name == "server_provisioned":
            bucket["provisioned_count"] = int(bucket.get("provisioned_count") or 0) + 1
        if weight > 0:
            bucket["demand_score"] = int(bucket["demand_score"] or 0) + weight
            weighted = dict(bucket.get("weighted_events") or {})
            weighted[event_name] = int(weighted.get(event_name) or 0) + 1
            bucket["weighted_events"] = weighted
        user_id = str(row.get("user_id") or "").strip()
        if user_id:
            users = bucket.get("distinct_users")
            if isinstance(users, set):
                users.add(user_id)

    prioritized_rows: list[dict[str, Any]] = []
    onboarding_tuning: list[dict[str, Any]] = []
    for server_id in server_ids:
        bucket = state[server_id]
        users = bucket.get("distinct_users")
        requested = int(bucket.get("requested_count") or 0)
        provisioned = int(bucket.get("provisioned_count") or 0)
        conversion = (provisioned / requested) if requested > 0 else 0.0
        prioritized_rows.append(
            {
                "server_id": server_id,
                "demand_score": int(bucket.get("demand_score") or 0),
                "event_count": int(bucket.get("event_count") or 0),
                "distinct_users": len(users) if isinstance(users, set) else 0,
                "weighted_events": dict(bucket.get("weighted_events") or {}),
                "requested_count": requested,
                "provisioned_count": provisioned,
                "provision_conversion_rate": round(conversion, 3),
            }
        )
        if requested >= 3 and conversion < 0.5:
            onboarding_tuning.append(
                {
                    "server_id": server_id,
                    "requested_count": requested,
                    "provisioned_count": provisioned,
                    "provision_conversion_rate": round(conversion, 3),
                    "recommendation": "Improve onboarding card copy and OAuth guidance for this server.",
                }
            )

    prioritized_rows.sort(
        key=lambda item: (
            -int(item.get("demand_score") or 0),
            -int(item.get("event_count") or 0),
            -int(item.get("distinct_users") or 0),
            str(item.get("server_id") or ""),
        )
    )
    onboarding_tuning.sort(
        key=lambda item: (
            float(item.get("provision_conversion_rate") or 0),
            -int(item.get("requested_count") or 0),
            str(item.get("server_id") or ""),
        )
    )
    return {
        "window_days": window_days,
        "servers": prioritized_rows,
        "recommended_order": [str(item.get("server_id") or "") for item in prioritized_rows],
        "onboarding_tuning": onboarding_tuning[:10],
    }


def wave56_server_prioritization(db: Session, *, days: int = 30) -> dict[str, Any]:
    return _server_prioritization(
        db,
        days=days,
        server_ids=_WAVE56_SERVER_IDS,
        weights=_WAVE56_WEIGHTS,
    )


def wave14_server_prioritization(db: Session, *, days: int = 30) -> dict[str, Any]:
    return _server_prioritization(
        db,
        days=days,
        server_ids=_WAVE14_SERVER_IDS,
        weights=_WAVE14_WEIGHTS,
    )
