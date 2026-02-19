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
        row = db.execute(text("select name from sqlite_master where type='table' and name=:name"), {"name": table_name}).first()
        return bool(row)
    except Exception:
        return False


def ensure_prompt_versions_table(db: Session) -> None:
    global _READY
    if _READY and _table_exists(db, "prompt_versions"):
        return
    with _LOCK:
        if _READY and _table_exists(db, "prompt_versions"):
            return
        dialect = db.bind.dialect.name if db.bind is not None else ""
        if dialect == "sqlite":
            db.execute(
                text(
                    """
                    create table if not exists prompt_versions (
                      id text primary key,
                      prompt_group text not null,
                      version_label text not null,
                      content text not null,
                      status text not null default 'draft',
                      rollout_percentage integer default 0,
                      metadata_json text,
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
                    create table if not exists prompt_versions (
                      id text primary key,
                      prompt_group text not null,
                      version_label text not null,
                      content text not null,
                      status text not null default 'draft',
                      rollout_percentage integer default 0,
                      metadata_json jsonb,
                      created_at timestamptz default now(),
                      updated_at timestamptz default now()
                    )
                    """
                )
            )
        db.execute(
            text(
                "create index if not exists idx_prompt_versions_group_status "
                "on prompt_versions(prompt_group, status, created_at)"
            )
        )
        db.commit()
        _READY = True


def _bucket_for_user(*, user_id: str, prompt_group: str) -> int:
    digest = hashlib.sha256(f"{prompt_group}:{user_id}".encode("utf-8")).hexdigest()
    return int(digest[:8], 16) % 100


def create_prompt_version(
    db: Session,
    *,
    prompt_group: str,
    version_label: str,
    content: str,
    status: str = "draft",
    rollout_percentage: int = 0,
    metadata: dict[str, Any] | None = None,
) -> str:
    ensure_prompt_versions_table(db)
    version_id = str(uuid.uuid4())
    payload = {
        "id": version_id,
        "prompt_group": prompt_group,
        "version_label": version_label,
        "content": content,
        "status": status,
        "rollout_percentage": max(0, min(100, int(rollout_percentage or 0))),
        "metadata_json": json.dumps(metadata or {}, ensure_ascii=False),
        "updated_at": datetime.utcnow(),
    }
    dialect = db.bind.dialect.name if db.bind is not None else ""
    if dialect == "sqlite":
        db.execute(
            text(
                """
                insert into prompt_versions (
                  id, prompt_group, version_label, content, status, rollout_percentage, metadata_json, updated_at
                ) values (
                  :id, :prompt_group, :version_label, :content, :status, :rollout_percentage, :metadata_json, :updated_at
                )
                """
            ),
            payload,
        )
    else:
        db.execute(
            text(
                """
                insert into prompt_versions (
                  id, prompt_group, version_label, content, status, rollout_percentage, metadata_json, updated_at
                ) values (
                  :id, :prompt_group, :version_label, :content, :status, :rollout_percentage, cast(:metadata_json as jsonb), :updated_at
                )
                """
            ),
            payload,
        )
    db.commit()
    return version_id


def _row_to_dict(row: Any) -> dict[str, Any]:
    if row is None:
        return {}
    return dict(row)


def select_prompt_version(
    db: Session,
    *,
    user_id: str | None,
    prompt_group: str,
    default_content: str,
) -> dict[str, Any]:
    ensure_prompt_versions_table(db)
    active = db.execute(
        text(
            "select * from prompt_versions "
            "where prompt_group = :prompt_group and status = 'active' "
            "order by updated_at desc, created_at desc limit 1"
        ),
        {"prompt_group": prompt_group},
    ).mappings().first()

    candidate = db.execute(
        text(
            "select * from prompt_versions "
            "where prompt_group = :prompt_group and status in ('canary', 'rolling') "
            "order by updated_at desc, created_at desc limit 1"
        ),
        {"prompt_group": prompt_group},
    ).mappings().first()

    if candidate and user_id:
        percent = int(candidate.get("rollout_percentage") or (5 if str(candidate.get("status")) == "canary" else 25))
        if _bucket_for_user(user_id=user_id, prompt_group=prompt_group) < max(0, min(100, percent)):
            return {
                "content": str(candidate.get("content") or default_content),
                "prompt_version_id": str(candidate.get("id")),
                "status": str(candidate.get("status") or "rolling"),
            }

    if active:
        return {
            "content": str(active.get("content") or default_content),
            "prompt_version_id": str(active.get("id")),
            "status": "active",
        }

    return {"content": default_content, "prompt_version_id": None, "status": "default"}


def resolve_prompt_content(
    db: Session,
    *,
    user_id: str | None,
    prompt_group: str,
    default_content: str,
) -> tuple[str, str | None, str]:
    selected = select_prompt_version(
        db,
        user_id=user_id,
        prompt_group=prompt_group,
        default_content=default_content,
    )
    content = str(selected.get("content") or default_content)
    raw_id = selected.get("prompt_version_id")
    version_id = str(raw_id) if raw_id else None
    status = str(selected.get("status") or "default")
    return content, version_id, status


def rollback_prompt(
    db: Session,
    *,
    prompt_group: str,
    target_version_id: str | None = None,
) -> str | None:
    ensure_prompt_versions_table(db)
    if target_version_id:
        chosen = db.execute(
            text("select id from prompt_versions where id = :id and prompt_group = :prompt_group limit 1"),
            {"id": target_version_id, "prompt_group": prompt_group},
        ).first()
        if not chosen:
            return None
        chosen_id = str(chosen[0])
    else:
        row = db.execute(
            text(
                "select id from prompt_versions "
                "where prompt_group = :prompt_group "
                "order by updated_at desc, created_at desc limit 2"
            ),
            {"prompt_group": prompt_group},
        ).all()
        if not row:
            return None
        chosen_id = str(row[-1][0])

    db.execute(
        text("update prompt_versions set status = 'draft' where prompt_group = :prompt_group and status = 'active'"),
        {"prompt_group": prompt_group},
    )
    db.execute(
        text(
            "update prompt_versions set status = 'active', rollout_percentage = 100, updated_at = :updated_at "
            "where id = :id"
        ),
        {"id": chosen_id, "updated_at": datetime.utcnow()},
    )
    db.commit()
    return chosen_id
