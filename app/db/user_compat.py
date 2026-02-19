from __future__ import annotations

import uuid
from datetime import datetime
from typing import Any

from sqlalchemy import text
from sqlalchemy.orm import Session


def _dialect_name(db: Session) -> str:
    bind = getattr(db, "bind", None)
    dialect = getattr(bind, "dialect", None)
    return str(getattr(dialect, "name", "") or "")


def _table_exists(db: Session, table_name: str) -> bool:
    try:
        dialect = _dialect_name(db)
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
        dialect = _dialect_name(db)
        if dialect == "sqlite":
            rows = db.execute(text(f"PRAGMA table_info({table_name})")).mappings().all()
            return any(str(r.get("name") or "") == column_name for r in rows)
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


def users_id_is_uuid(db: Session) -> bool:
    if _dialect_name(db) == "sqlite":
        return False
    try:
        row = db.execute(
            text(
                "select data_type, udt_name "
                "from information_schema.columns "
                "where table_schema = current_schema() and table_name = 'users' and column_name = 'id' "
                "limit 1"
            )
        ).mappings().first()
        if not row:
            return False
        data_type = str(row.get("data_type") or "").lower()
        udt_name = str(row.get("udt_name") or "").lower()
        return data_type == "uuid" or udt_name == "uuid"
    except Exception:
        return False


def user_exists(db: Session, user_id: str) -> bool:
    uid = str(user_id or "").strip()
    if not uid or not _table_exists(db, "users"):
        return False
    try:
        if _dialect_name(db) == "sqlite":
            row = db.execute(
                text("select id from users where id = :id limit 1"),
                {"id": uid},
            ).first()
        else:
            row = db.execute(
                text("select id::text as id from users where id::text = :id limit 1"),
                {"id": uid},
            ).first()
        return bool(row)
    except Exception:
        return False


def _set_app_user_id(db: Session, user_id: str) -> None:
    if _dialect_name(db) == "sqlite":
        return
    try:
        db.execute(text("select set_config('app.user_id', :user_id, true)"), {"user_id": user_id})
    except Exception:
        pass


def ensure_user_row(db: Session, user_id: str) -> None:
    uid = str(user_id or "").strip()
    if not uid or not _table_exists(db, "users"):
        return
    if user_exists(db, uid):
        return

    dialect = _dialect_name(db)
    has_created_at = _column_exists(db, "users", "created_at")
    values: dict[str, Any] = {"id": uid, "created_at": datetime.utcnow()}

    if dialect != "sqlite" and users_id_is_uuid(db):
        try:
            uuid.UUID(uid)
        except ValueError:
            # Mixed-schema environments may send non-UUID user ids on legacy paths.
            # Don't hard-fail the request if this legacy row cannot be materialized.
            return
    _set_app_user_id(db, uid)

    try:
        if dialect == "sqlite":
            if has_created_at:
                db.execute(
                    text(
                        "insert into users (id, created_at) values (:id, :created_at) "
                        "on conflict(id) do nothing"
                    ),
                    values,
                )
            else:
                db.execute(
                    text("insert into users (id) values (:id) on conflict(id) do nothing"),
                    {"id": uid},
                )
        else:
            id_expr = "(:id)::uuid" if users_id_is_uuid(db) else ":id"
            if has_created_at:
                db.execute(
                    text(
                        f"insert into users (id, created_at) values ({id_expr}, :created_at) "
                        "on conflict (id) do nothing"
                    ),
                    values,
                )
            else:
                db.execute(
                    text(f"insert into users (id) values ({id_expr}) on conflict (id) do nothing"),
                    {"id": uid},
                )
        db.commit()
    except Exception:
        db.rollback()
        # Best-effort fallback for unexpected legacy schemas.
        try:
            if dialect == "sqlite":
                db.execute(
                    text("insert into users (id) values (:id) on conflict(id) do nothing"),
                    {"id": uid},
                )
            else:
                id_expr = "(:id)::uuid" if users_id_is_uuid(db) else ":id"
                db.execute(
                    text(f"insert into users (id) values ({id_expr}) on conflict (id) do nothing"),
                    {"id": uid},
                )
            db.commit()
        except Exception:
            db.rollback()
