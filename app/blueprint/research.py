from __future__ import annotations

import asyncio
import json
from datetime import datetime, timedelta, timezone
from typing import Any
from uuid import uuid4

from sqlalchemy import text
from sqlalchemy.orm import Session

from app.blueprint.knowledge_files import get_latest_knowledge_file, put_knowledge_file_version
from app.services.tavily_client import tavily_search


def _table_exists(db: Session, table_name: str) -> bool:
    try:
        row = db.execute(
            text(
                "select 1 from information_schema.tables where table_schema = current_schema() and table_name = :name"
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


def _dialect_name(db: Session) -> str:
    if db.bind is None or db.bind.dialect is None:
        return ""
    return str(db.bind.dialect.name or "")


def _loads_json(value: Any, default: Any) -> Any:
    if isinstance(value, (dict, list)):
        return value
    if isinstance(value, str):
        try:
            return json.loads(value)
        except Exception:
            return default
    return default


def _normalize_sources(value: Any) -> list[str]:
    if isinstance(value, list):
        return [str(v).strip() for v in value if str(v).strip()]
    if isinstance(value, str):
        loaded = _loads_json(value, None)
        if isinstance(loaded, list):
            return [str(v).strip() for v in loaded if str(v).strip()]
        return [s.strip() for s in value.split(",") if s.strip()]
    return []


def _refresh_heartbeat_research(db: Session, *, user_id: str) -> None:
    rows = db.execute(
        text(
            """
            select title, status, last_run_at, next_run_at
            from research_jobs
            where user_id = :user_id
            order by updated_at desc
            limit 20
            """
        ),
        {"user_id": user_id},
    ).mappings().all()
    lines = ["## Research Tracker"]
    if rows:
        for row in rows:
            last_run = row.get("last_run_at")
            next_run = row.get("next_run_at")
            last_text = last_run.isoformat() if isinstance(last_run, datetime) else str(last_run or "-")
            next_text = next_run.isoformat() if isinstance(next_run, datetime) else str(next_run or "-")
            lines.append(f"- {row.get('title') or 'Untitled'}: {row.get('status')} (last={last_text}, next={next_text})")
    else:
        lines.append("- No recurring research jobs configured.")

    latest = get_latest_knowledge_file(db, user_id=user_id, file_path="HEARTBEAT.md")
    current = str((latest or {}).get("content") or "").strip()
    merged = current
    marker = "## Research Tracker"
    if marker in current:
        head = current.split(marker)[0].rstrip()
        merged = (head + "\n\n" + "\n".join(lines)).strip()
    else:
        merged = (current + "\n\n" + "\n".join(lines)).strip() if current else "\n".join(["# HEARTBEAT.md", "", *lines]).strip()
    if current == merged:
        return
    put_knowledge_file_version(
        db,
        user_id=user_id,
        file_path="HEARTBEAT.md",
        content=merged,
        metadata={"source": "research_engine"},
    )


def _queue_research_delivery(
    db: Session,
    *,
    user_id: str,
    title: str,
    summary: str,
    findings: list[dict[str, Any]],
    delivery_channel: str | None,
    delivery_format: str | None,
) -> dict[str, Any]:
    if not _table_exists(db, "notification_queue"):
        return {"queued": False, "reason": "notification_queue_missing"}

    channel = str(delivery_channel or "whatsapp").strip().lower()
    fmt = str(delivery_format or "summary").strip().lower()
    heading = f"Research: {title or 'Update'}"
    if fmt == "digest":
        message = summary.strip() or "Research digest is ready."
    else:
        top = findings[:3]
        bullets = [f"- {item.get('title') or item.get('url')}" for item in top]
        message = (summary.strip() + "\n" if summary else "") + ("\n".join(bullets) if bullets else "No findings.")

    db.execute(
        text(
            """
            insert into notification_queue (
                user_id, watch_item_id, event_type, title, message, deep_link_url,
                prev_price, new_price, currency, is_sent, sent_at, created_at
            ) values (
                :user_id, null, :event_type, :title, :message, :deep_link_url,
                null, null, null, false, null, :created_at
            )
            """
        ),
        {
            "user_id": user_id,
            "event_type": f"research_{channel}",
            "title": heading,
            "message": message[:3000],
            "deep_link_url": findings[0].get("url") if findings else None,
            "created_at": datetime.utcnow(),
        },
    )
    return {"queued": True, "channel": channel}


def list_research_jobs(db: Session, *, user_id: str) -> list[dict[str, Any]]:
    if not _table_exists(db, "research_jobs"):
        raise RuntimeError("research_jobs table not found")
    rows = db.execute(
        text(
            """
            select id, user_id, title, query, sources, schedule, status, last_run_at, next_run_at,
                   findings, delivery_channel, delivery_format, max_cost_per_run, total_cost,
                   created_at, updated_at
            from research_jobs
            where user_id = :user_id
            order by updated_at desc
            """
        ),
        {"user_id": user_id},
    ).mappings().all()
    out: list[dict[str, Any]] = []
    for row in rows:
        item = dict(row)
        item["sources"] = _normalize_sources(item.get("sources"))
        item["findings"] = _loads_json(item.get("findings"), [])
        out.append(item)
    return out


def create_research_job(
    db: Session,
    *,
    user_id: str,
    title: str,
    query: str,
    sources: list[str] | None = None,
    schedule: str | None = None,
    status: str = "active",
    delivery_channel: str | None = "whatsapp",
    delivery_format: str = "summary",
    max_cost_per_run: float = 0.5,
) -> dict[str, Any]:
    if not _table_exists(db, "research_jobs"):
        raise RuntimeError("research_jobs table not found")
    now = datetime.now(timezone.utc).replace(tzinfo=None)
    next_run = now + timedelta(days=1)
    normalized_sources = [s for s in (sources or []) if str(s).strip()]
    dialect = _dialect_name(db)
    sources_value: Any = normalized_sources if dialect != "sqlite" else json.dumps(normalized_sources, ensure_ascii=False)
    row = db.execute(
        text(
            """
            insert into research_jobs (
              id, user_id, title, query, sources, schedule, status,
              last_run_at, next_run_at, findings, delivery_channel, delivery_format,
              max_cost_per_run, total_cost, created_at, updated_at
            ) values (
              :id, :user_id, :title, :query, :sources, :schedule, :status,
              null, :next_run_at, :findings, :delivery_channel, :delivery_format,
              :max_cost_per_run, 0, :now, :now
            )
            returning id, user_id, title, query, sources, schedule, status,
                      last_run_at, next_run_at, findings, delivery_channel, delivery_format,
                      max_cost_per_run, total_cost, created_at, updated_at
            """
        ),
        {
            "id": str(uuid4()),
            "user_id": user_id,
            "title": title,
            "query": query,
            "sources": sources_value,
            "schedule": schedule,
            "status": status,
            "next_run_at": next_run,
            "findings": json.dumps([], ensure_ascii=False),
            "delivery_channel": delivery_channel,
            "delivery_format": delivery_format,
            "max_cost_per_run": float(max_cost_per_run),
            "now": now,
        },
    ).mappings().first()
    db.commit()
    _refresh_heartbeat_research(db, user_id=user_id)
    item = dict(row or {})
    item["sources"] = _normalize_sources(item.get("sources"))
    item["findings"] = _loads_json(item.get("findings"), [])
    return item


def get_research_job(db: Session, *, user_id: str, research_id: str) -> dict[str, Any] | None:
    if not _table_exists(db, "research_jobs"):
        raise RuntimeError("research_jobs table not found")
    row = db.execute(
        text(
            """
            select id, user_id, title, query, sources, schedule, status, last_run_at, next_run_at,
                   findings, delivery_channel, delivery_format, max_cost_per_run, total_cost,
                   created_at, updated_at
            from research_jobs
            where user_id = :user_id and id = :research_id
            """
        ),
        {"user_id": user_id, "research_id": research_id},
    ).mappings().first()
    if not row:
        return None
    item = dict(row)
    item["sources"] = _normalize_sources(item.get("sources"))
    item["findings"] = _loads_json(item.get("findings"), [])
    return item


def update_research_job(
    db: Session,
    *,
    user_id: str,
    research_id: str,
    fields: dict[str, Any],
) -> dict[str, Any] | None:
    if not _table_exists(db, "research_jobs"):
        raise RuntimeError("research_jobs table not found")
    updates: list[str] = []
    params: dict[str, Any] = {"user_id": user_id, "research_id": research_id, "updated_at": datetime.utcnow()}
    allowed = {
        "title",
        "query",
        "sources",
        "schedule",
        "status",
        "delivery_channel",
        "delivery_format",
        "max_cost_per_run",
        "next_run_at",
    }
    dialect = _dialect_name(db)
    for key, value in fields.items():
        if key not in allowed:
            continue
        if key == "sources":
            normalized = [str(s).strip() for s in (value or []) if str(s).strip()]
            params[key] = normalized if dialect != "sqlite" else json.dumps(normalized, ensure_ascii=False)
        else:
            params[key] = value
        updates.append(f"{key} = :{key}")
    if not updates:
        return get_research_job(db, user_id=user_id, research_id=research_id)
    updated = db.execute(
        text(
            f"update research_jobs set {', '.join(updates)}, updated_at = :updated_at "
            "where id = :research_id and user_id = :user_id"
        ),
        params,
    ).rowcount
    if not updated:
        db.rollback()
        return None
    db.commit()
    _refresh_heartbeat_research(db, user_id=user_id)
    return get_research_job(db, user_id=user_id, research_id=research_id)


def delete_research_job(db: Session, *, user_id: str, research_id: str) -> bool:
    if not _table_exists(db, "research_jobs"):
        raise RuntimeError("research_jobs table not found")
    deleted = db.execute(
        text("delete from research_jobs where id = :research_id and user_id = :user_id"),
        {"research_id": research_id, "user_id": user_id},
    ).rowcount
    db.commit()
    _refresh_heartbeat_research(db, user_id=user_id)
    return bool(deleted)


async def _run_tavily(query: str, max_results: int = 6) -> dict[str, Any]:
    return await tavily_search(query, max_results=max_results, include_answer=True, include_raw_content=False)


def run_research_job(db: Session, *, user_id: str, research_id: str) -> dict[str, Any]:
    job = get_research_job(db, user_id=user_id, research_id=research_id)
    if not job:
        raise RuntimeError("Research job not found")
    query = str(job.get("query") or "").strip()
    if not query:
        raise RuntimeError("Research query is empty")

    try:
        findings_payload = asyncio.run(_run_tavily(query))
    except RuntimeError:
        loop = asyncio.new_event_loop()
        try:
            findings_payload = loop.run_until_complete(_run_tavily(query))
        finally:
            loop.close()

    findings: list[dict[str, Any]] = []
    for item in findings_payload.get("results") or []:
        if not isinstance(item, dict):
            continue
        findings.append(
            {
                "title": str(item.get("title") or ""),
                "url": str(item.get("url") or ""),
                "snippet": str(item.get("content") or item.get("snippet") or ""),
                "score": float(item.get("score") or 0.0),
            }
        )

    max_cost = float(job.get("max_cost_per_run") or 0.5)
    run_cost = min(max_cost, round(0.01 * max(1, len(findings)), 4))
    now = datetime.utcnow()
    next_run = now + timedelta(days=1)
    db.execute(
        text(
            """
            update research_jobs
            set findings = :findings,
                last_run_at = :last_run_at,
                next_run_at = :next_run_at,
                total_cost = coalesce(total_cost, 0) + :run_cost,
                updated_at = :updated_at
            where id = :research_id and user_id = :user_id
            """
        ),
        {
            "findings": json.dumps(findings, ensure_ascii=False),
            "last_run_at": now,
            "next_run_at": next_run,
            "run_cost": run_cost,
            "updated_at": now,
            "research_id": research_id,
            "user_id": user_id,
        },
    )
    delivery = _queue_research_delivery(
        db,
        user_id=user_id,
        title=str(job.get("title") or ""),
        summary=str(findings_payload.get("answer") or ""),
        findings=findings,
        delivery_channel=job.get("delivery_channel"),
        delivery_format=job.get("delivery_format"),
    )
    db.commit()
    _refresh_heartbeat_research(db, user_id=user_id)
    updated = get_research_job(db, user_id=user_id, research_id=research_id) or {}
    return {
        "job": updated,
        "summary": findings_payload.get("answer") or "",
        "findings_count": len(findings),
        "run_cost": run_cost,
        "delivery": delivery,
    }
