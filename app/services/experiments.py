from __future__ import annotations

import hashlib
import json
import threading
import uuid
from datetime import datetime
from typing import Any

from sqlalchemy import text
from sqlalchemy.orm import Session


_LOCK = threading.Lock()
_READY = False


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


def ensure_experiments_table(db: Session) -> None:
    global _READY
    if _READY and _table_exists(db, "experiments"):
        return
    with _LOCK:
        if _READY and _table_exists(db, "experiments"):
            return
        dialect = db.bind.dialect.name if db.bind is not None else ""
        if dialect == "sqlite":
            db.execute(
                text(
                    """
                    create table if not exists experiments (
                      id text primary key,
                      name text not null,
                      description text,
                      status text not null default 'draft',
                      prompt_group text,
                      allocation_json text not null default '{}',
                      config_json text not null default '{}',
                      created_by text,
                      created_at datetime default current_timestamp,
                      updated_at datetime default current_timestamp
                    )
                    """
                )
            )
        else:
            db.execute(
                text(
                    """
                    create table if not exists experiments (
                      id text primary key,
                      name text not null,
                      description text,
                      status text not null default 'draft',
                      prompt_group text,
                      allocation_json jsonb not null default '{}'::jsonb,
                      config_json jsonb not null default '{}'::jsonb,
                      created_by text,
                      created_at timestamptz default now(),
                      updated_at timestamptz default now()
                    )
                    """
                )
            )
        db.execute(text("create index if not exists idx_experiments_status_updated on experiments(status, updated_at)"))
        db.commit()
        _READY = True


def _normalize_allocation(allocation: dict[str, int] | None) -> dict[str, int]:
    raw = allocation or {"control": 50, "candidate": 50}
    out: dict[str, int] = {}
    total = 0
    for key, value in raw.items():
        name = str(key or "").strip()
        if not name:
            continue
        pct = max(0, int(value or 0))
        if pct <= 0:
            continue
        out[name] = pct
        total += pct
    if not out:
        return {"control": 100}
    if total == 100:
        return out
    scaled: dict[str, int] = {}
    consumed = 0
    keys = list(out.keys())
    for idx, key in enumerate(keys):
        if idx == len(keys) - 1:
            scaled[key] = max(0, 100 - consumed)
            break
        pct = int(round((out[key] / total) * 100))
        pct = max(0, min(100, pct))
        scaled[key] = pct
        consumed += pct
    if sum(scaled.values()) == 0:
        return {"control": 100}
    return scaled


def create_experiment(
    db: Session,
    *,
    name: str,
    description: str = "",
    status: str = "draft",
    prompt_group: str | None = None,
    allocation: dict[str, int] | None = None,
    config: dict[str, Any] | None = None,
    created_by: str | None = None,
) -> dict[str, Any]:
    ensure_experiments_table(db)
    if not str(name or "").strip():
        raise ValueError("name is required")

    allocation_norm = _normalize_allocation(allocation)
    exp_id = str(uuid.uuid4())
    params = {
        "id": exp_id,
        "name": str(name).strip(),
        "description": str(description or "").strip(),
        "status": str(status or "draft").strip().lower() or "draft",
        "prompt_group": str(prompt_group or "").strip() or None,
        "allocation_json": json.dumps(allocation_norm, ensure_ascii=False),
        "config_json": json.dumps(config or {}, ensure_ascii=False),
        "created_by": str(created_by or "").strip() or None,
        "updated_at": datetime.utcnow(),
    }
    dialect = db.bind.dialect.name if db.bind is not None else ""
    if dialect == "sqlite":
        db.execute(
            text(
                """
                insert into experiments (
                  id, name, description, status, prompt_group, allocation_json, config_json, created_by, updated_at
                ) values (
                  :id, :name, :description, :status, :prompt_group, :allocation_json, :config_json, :created_by, :updated_at
                )
                """
            ),
            params,
        )
    else:
        db.execute(
            text(
                """
                insert into experiments (
                  id, name, description, status, prompt_group, allocation_json, config_json, created_by, updated_at
                ) values (
                  :id, :name, :description, :status, :prompt_group,
                  cast(:allocation_json as jsonb), cast(:config_json as jsonb), :created_by, :updated_at
                )
                """
            ),
            params,
        )
    db.commit()
    return get_experiment(db, experiment_id=exp_id) or {"id": exp_id}


def list_experiments(
    db: Session,
    *,
    status: str | None = None,
    limit: int = 100,
) -> list[dict[str, Any]]:
    ensure_experiments_table(db)
    params: dict[str, Any] = {"limit": max(1, min(500, int(limit or 100)))}
    where_clause = ""
    if status:
        where_clause = "where status = :status"
        params["status"] = str(status).strip().lower()
    rows = db.execute(
        text(
            f"""
            select *
            from experiments
            {where_clause}
            order by updated_at desc, created_at desc
            limit :limit
            """
        ),
        params,
    ).mappings().all()
    return [_serialize_row(dict(row)) for row in rows]


def get_experiment(db: Session, *, experiment_id: str) -> dict[str, Any] | None:
    ensure_experiments_table(db)
    row = db.execute(
        text("select * from experiments where id = :id limit 1"),
        {"id": experiment_id},
    ).mappings().first()
    if not row:
        return None
    return _serialize_row(dict(row))


def assign_variant(*, experiment_id: str, user_id: str, allocation: dict[str, int] | None) -> str:
    normalized = _normalize_allocation(allocation)
    digest = hashlib.sha256(f"{experiment_id}:{user_id}".encode("utf-8")).hexdigest()
    bucket = int(digest[:8], 16) % 100
    cursor = 0
    for variant, pct in normalized.items():
        cursor += int(pct)
        if bucket < cursor:
            return variant
    return next(iter(normalized.keys()))


def _parse_json_field(value: Any) -> dict[str, Any]:
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


def _serialize_row(row: dict[str, Any]) -> dict[str, Any]:
    out: dict[str, Any] = {}
    for key, value in row.items():
        if key in {"allocation_json", "config_json"}:
            out["allocation" if key == "allocation_json" else "config"] = _parse_json_field(value)
            continue
        if isinstance(value, datetime):
            out[key] = value.isoformat()
        else:
            out[key] = value
    return out
