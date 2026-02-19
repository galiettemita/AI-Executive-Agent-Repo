from __future__ import annotations

import logging
import re
import uuid
from datetime import datetime
from typing import Any

from sqlalchemy import text
from sqlalchemy.orm import Session

logger = logging.getLogger(__name__)

_IDENT_RE = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*$")


def _ident(name: str) -> str:
    n = str(name or "").strip()
    if not _IDENT_RE.match(n):
        raise ValueError(f"unsafe SQL identifier: {name!r}")
    return n


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


def _column_is_uuid(db: Session, table_name: str, column_name: str) -> bool:
    if _dialect_name(db) == "sqlite":
        return False
    try:
        row = db.execute(
            text(
                "select data_type, udt_name "
                "from information_schema.columns "
                "where table_schema = current_schema() and table_name = :table and column_name = :column "
                "limit 1"
            ),
            {"table": table_name, "column": column_name},
        ).mappings().first()
        if not row:
            return False
        data_type = str(row.get("data_type") or "").lower()
        udt_name = str(row.get("udt_name") or "").lower()
        return data_type == "uuid" or udt_name == "uuid"
    except Exception:
        return False


def users_id_is_uuid(db: Session) -> bool:
    return _column_is_uuid(db, "users", "id")


def _row_exists(db: Session, table_name: str, id_column: str, value: str) -> bool:
    if not _table_exists(db, table_name):
        return False
    table = _ident(table_name)
    col = _ident(id_column)
    uid = str(value or "").strip()
    if not uid:
        return False
    try:
        if _dialect_name(db) == "sqlite":
            row = db.execute(
                text(f"select {col} from {table} where {col} = :id limit 1"),
                {"id": uid},
            ).first()
        else:
            row = db.execute(
                text(f"select {col}::text as id from {table} where {col}::text = :id limit 1"),
                {"id": uid},
            ).first()
        return bool(row)
    except Exception:
        return False


def user_exists(db: Session, user_id: str) -> bool:
    return _row_exists(db, "users", "id", user_id)


def _set_app_user_id(db: Session, user_id: str) -> None:
    if _dialect_name(db) == "sqlite":
        return
    try:
        db.execute(text("select set_config('app.user_id', :user_id, true)"), {"user_id": user_id})
    except Exception:
        pass


def _columns_info(db: Session, table_name: str) -> list[dict[str, Any]]:
    if not _table_exists(db, table_name):
        return []
    table = _ident(table_name)
    if _dialect_name(db) == "sqlite":
        rows = db.execute(text(f"PRAGMA table_info({table})")).mappings().all()
        out: list[dict[str, Any]] = []
        for row in rows:
            out.append(
                {
                    "column_name": str(row.get("name") or ""),
                    "is_nullable": "NO" if int(row.get("notnull") or 0) else "YES",
                    "data_type": str(row.get("type") or "").lower(),
                    "udt_name": str(row.get("type") or "").lower(),
                    "column_default": row.get("dflt_value"),
                }
            )
        return out
    rows = db.execute(
        text(
            "select column_name, is_nullable, data_type, udt_name, column_default "
            "from information_schema.columns "
            "where table_schema = current_schema() and table_name = :table "
            "order by ordinal_position"
        ),
        {"table": table_name},
    ).mappings().all()
    return [dict(r) for r in rows]


def _first_enum_label(db: Session, enum_name: str) -> str | None:
    if _dialect_name(db) == "sqlite":
        return None
    try:
        row = db.execute(
            text(
                "select e.enumlabel "
                "from pg_type t "
                "join pg_enum e on e.enumtypid = t.oid "
                "where t.typname = :typ "
                "order by e.enumsortorder asc "
                "limit 1"
            ),
            {"typ": enum_name},
        ).first()
        return str(row[0]) if row and row[0] is not None else None
    except Exception:
        return None


def _guess_required_value(
    db: Session,
    *,
    column_name: str,
    data_type: str,
    udt_name: str,
    user_id: str,
) -> Any | None:
    now = datetime.utcnow()
    name = str(column_name or "").lower()
    dt = str(data_type or "").lower()
    udt = str(udt_name or "").lower()

    if name in {"id", "user_id", "account_id", "owner_id"}:
        return user_id
    if name.endswith("_id"):
        if dt == "uuid" or udt == "uuid":
            try:
                uuid.UUID(user_id)
                return user_id
            except ValueError:
                return str(uuid.uuid4())
        return user_id

    if name in {
        "created_at",
        "updated_at",
        "deletion_requested_at",
        "last_sent_at",
        "verified_at",
        "expires_at",
        "onboarding_completed_at",
    }:
        return now

    if "timestamp" in dt or dt == "datetime":
        return now
    if dt == "date":
        return now.date()
    if dt == "boolean":
        return False
    if any(token in dt for token in ("int", "numeric", "real", "double", "decimal")):
        return 0
    if dt in {"character varying", "character", "text", "varchar", "char"}:
        return ""
    if dt == "uuid" or udt == "uuid":
        try:
            uuid.UUID(user_id)
            return user_id
        except ValueError:
            return str(uuid.uuid4())
    if dt == "user-defined":
        return _first_enum_label(db, udt)
    return None


def _insert_minimal_row(
    db: Session,
    *,
    table_name: str,
    id_column: str,
    user_id: str,
    explicit_values: dict[str, Any] | None = None,
) -> bool:
    table = _ident(table_name)
    id_col = _ident(id_column)
    cols = _columns_info(db, table)
    if not cols:
        return False

    values: dict[str, Any] = dict(explicit_values or {})
    if id_col not in values:
        values[id_col] = user_id

    by_name = {str(c.get("column_name") or ""): c for c in cols}
    now = datetime.utcnow()
    for maybe_ts in ("created_at", "updated_at"):
        if maybe_ts in by_name and maybe_ts not in values:
            values[maybe_ts] = now

    for c in cols:
        name = str(c.get("column_name") or "")
        if not name or name in values:
            continue
        required = str(c.get("is_nullable") or "").upper() == "NO" and c.get("column_default") is None
        if not required:
            continue
        guessed = _guess_required_value(
            db,
            column_name=name,
            data_type=str(c.get("data_type") or ""),
            udt_name=str(c.get("udt_name") or ""),
            user_id=user_id,
        )
        if guessed is None:
            logger.warning("Cannot infer required value for %s.%s while ensuring identity row", table, name)
            return False
        values[name] = guessed

    ordered_cols = [k for k in values.keys() if k in by_name]
    if not ordered_cols:
        return False

    params = {k: values[k] for k in ordered_cols}
    dialect = _dialect_name(db)
    placeholders: list[str] = []
    for col in ordered_cols:
        if dialect != "sqlite" and _column_is_uuid(db, table, col):
            placeholders.append(f"(:{col})::uuid")
        else:
            placeholders.append(f":{col}")

    sql = (
        f"insert or ignore into {table} ({', '.join(ordered_cols)}) values ({', '.join(placeholders)})"
        if dialect == "sqlite"
        else f"insert into {table} ({', '.join(ordered_cols)}) values ({', '.join(placeholders)}) on conflict do nothing"
    )
    _set_app_user_id(db, user_id)
    db.execute(text(sql), params)
    db.commit()
    return True


def ensure_table_row(
    db: Session,
    *,
    table_name: str,
    id_column: str,
    user_id: str,
    explicit_values: dict[str, Any] | None = None,
) -> bool:
    uid = str(user_id or "").strip()
    if not uid or not _table_exists(db, table_name):
        return False
    if _row_exists(db, table_name, id_column, uid):
        return True
    try:
        return _insert_minimal_row(
            db,
            table_name=table_name,
            id_column=id_column,
            user_id=uid,
            explicit_values=explicit_values,
        )
    except Exception:
        db.rollback()
        logger.exception("Failed to ensure row for %s(%s=%s)", table_name, id_column, uid)
        return False


def ensure_account_row(db: Session, user_id: str) -> bool:
    uid = str(user_id or "").strip()
    if not uid:
        return False
    if _column_is_uuid(db, "accounts", "id"):
        try:
            uuid.UUID(uid)
        except ValueError:
            return False
    return ensure_table_row(
        db,
        table_name="accounts",
        id_column="id",
        user_id=uid,
        explicit_values={"id": uid, "created_at": datetime.utcnow(), "updated_at": datetime.utcnow()},
    )


def ensure_user_row(db: Session, user_id: str) -> bool:
    uid = str(user_id or "").strip()
    if not uid or not _table_exists(db, "users"):
        return False
    if user_exists(db, uid):
        return True

    if users_id_is_uuid(db):
        try:
            uuid.UUID(uid)
        except ValueError:
            # Mixed-schema environments may send non-UUID user ids on legacy paths.
            return False

    explicit: dict[str, Any] = {"id": uid}
    if _column_exists(db, "users", "created_at"):
        explicit["created_at"] = datetime.utcnow()
    if _column_exists(db, "users", "updated_at"):
        explicit["updated_at"] = datetime.utcnow()

    # Some schemas model users as a profile table attached to accounts.
    if _column_exists(db, "users", "account_id"):
        if _column_is_uuid(db, "users", "account_id"):
            try:
                uuid.UUID(uid)
                ensure_account_row(db, uid)
                explicit["account_id"] = uid
            except ValueError:
                pass
        else:
            explicit["account_id"] = uid

    return ensure_table_row(
        db,
        table_name="users",
        id_column="id",
        user_id=uid,
        explicit_values=explicit,
    )


def _fk_parent_for(
    db: Session,
    *,
    child_table: str,
    fk_column: str,
) -> tuple[str, str] | None:
    if not _table_exists(db, child_table):
        return
    try:
        if _dialect_name(db) == "sqlite":
            table = _ident(child_table)
            rows = db.execute(text(f"PRAGMA foreign_key_list({table})")).mappings().all()
            for row in rows:
                if str(row.get("from") or "") == fk_column:
                    parent_table = str(row.get("table") or "")
                    parent_col = str(row.get("to") or "id")
                    if parent_table and parent_col:
                        return parent_table, parent_col
            return None

        row = db.execute(
            text(
                """
                select ccu.table_name as parent_table, ccu.column_name as parent_column
                from information_schema.table_constraints tc
                join information_schema.key_column_usage kcu
                  on tc.constraint_name = kcu.constraint_name
                 and tc.table_schema = kcu.table_schema
                join information_schema.constraint_column_usage ccu
                  on ccu.constraint_name = tc.constraint_name
                 and ccu.table_schema = tc.table_schema
                where tc.constraint_type = 'FOREIGN KEY'
                  and tc.table_schema = current_schema()
                  and tc.table_name = :child
                  and kcu.column_name = :col
                limit 1
                """
            ),
            {"child": child_table, "col": fk_column},
        ).mappings().first()
        if row:
            parent_table = str(row.get("parent_table") or "")
            parent_col = str(row.get("parent_column") or "id")
            if parent_table and parent_col:
                return parent_table, parent_col
    except Exception:
        return None
    return None


def ensure_fk_parent_row(
    db: Session,
    *,
    child_table: str,
    fk_column: str,
    user_id: str,
) -> bool:
    uid = str(user_id or "").strip()
    if not uid:
        return False
    fk = _fk_parent_for(db, child_table=child_table, fk_column=fk_column)
    if not fk:
        return True
    parent_table, parent_col = fk
    if parent_table == "users":
        return ensure_user_row(db, uid)
    if parent_table == "accounts":
        return ensure_account_row(db, uid)
    return ensure_table_row(db, table_name=parent_table, id_column=parent_col, user_id=uid, explicit_values={parent_col: uid})
