from __future__ import annotations

import json
from datetime import datetime, timedelta, timezone
from typing import Any

import stripe
from sqlalchemy import text
from sqlalchemy.orm import Session

from app.blueprint.db import _column_exists, _table_exists
from app.core.config import settings
from app.core.redis import get_redis
from app.services.provisioning_sessions import delete_provisioning_sessions_for_user
from app.services.gdpr_service import delete_user_data, get_user_data_summary


STAGE_PURGE_24H = "purge_24h"
STAGE_PROVIDERS_7D = "providers_7d"
STAGE_VERIFY_30D = "verify_30d"


def _now_utc() -> datetime:
    return datetime.now(timezone.utc)


def _ensure_pipeline_tables(db: Session) -> None:
    dialect = db.bind.dialect.name if db.bind is not None else ""
    if dialect == "sqlite":
        db.execute(
            text(
                """
                create table if not exists account_deletion_jobs (
                  id text primary key,
                  user_id text not null,
                  stage text not null,
                  due_at text not null,
                  status text not null default 'pending',
                  details_json text default '{}',
                  created_at text,
                  updated_at text,
                  unique(user_id, stage)
                )
                """
            )
        )
        db.commit()
        return

    db.execute(
        text(
            """
            create table if not exists account_deletion_jobs (
              id uuid primary key default gen_random_uuid(),
              user_id text not null,
              stage text not null,
              due_at timestamptz not null,
              status text not null default 'pending',
              details jsonb default '{}'::jsonb,
              created_at timestamptz default now(),
              updated_at timestamptz default now(),
              unique(user_id, stage)
            )
            """
        )
    )
    db.commit()


def _mark_user_deleted_requested(db: Session, *, user_id: str, now: datetime) -> None:
    if _table_exists(db, "users") and _column_exists(db, "users", "deletion_requested_at"):
        if (db.bind and db.bind.dialect.name == "sqlite"):
            db.execute(
                text("update users set deletion_requested_at = :ts where id = :user_id"),
                {"ts": now.replace(tzinfo=None).isoformat(sep=" "), "user_id": user_id},
            )
        else:
            db.execute(
                text("update users set deletion_requested_at = :ts where id::text = :user_id"),
                {"ts": now.replace(tzinfo=None), "user_id": user_id},
            )

    # Blueprint `accounts` table uses account_status enum (active/expired/revoked).
    if _table_exists(db, "accounts") and _column_exists(db, "accounts", "status"):
        if (db.bind and db.bind.dialect.name == "sqlite"):
            db.execute(
                text("update accounts set status = 'revoked' where id = :user_id"),
                {"user_id": user_id},
            )
        else:
            db.execute(
                text("update accounts set status = 'revoked' where id::text = :user_id"),
                {"user_id": user_id},
            )


def _revoke_immediate_tokens(db: Session, *, user_id: str) -> dict[str, Any]:
    out: dict[str, Any] = {"oauth_tokens": 0, "integration_credentials": 0, "redis_keys_deleted": 0, "provisioning_sessions_deleted": 0}
    dialect = db.bind.dialect.name if db.bind is not None else ""

    if _table_exists(db, "oauth_tokens"):
        if dialect == "sqlite":
            res = db.execute(text("delete from oauth_tokens where user_id = :user_id"), {"user_id": user_id})
        else:
            res = db.execute(text("delete from oauth_tokens where user_id::text = :user_id"), {"user_id": user_id})
        out["oauth_tokens"] = int(getattr(res, "rowcount", 0) or 0)

    if _table_exists(db, "integration_credentials"):
        if dialect == "sqlite":
            res = db.execute(
                text("delete from integration_credentials where user_id = :user_id"),
                {"user_id": user_id},
            )
        else:
            res = db.execute(
                text("delete from integration_credentials where user_id::text = :user_id"),
                {"user_id": user_id},
            )
        out["integration_credentials"] = int(getattr(res, "rowcount", 0) or 0)

    r = get_redis()
    if r is not None:
        patterns = [
            f"billing:sub:{user_id}",
            f"billing:daily:{user_id}",
            f"billing:burst:{user_id}",
            f"billing:mcp:monthly:*:{user_id}",
            f"bp:v1:session:*:{user_id}",
        ]
        deleted = 0
        try:
            for pattern in patterns:
                keys = list(r.scan_iter(match=pattern, count=200))
                if keys:
                    deleted += int(r.delete(*keys) or 0)
        except Exception:
            pass
        out["redis_keys_deleted"] = deleted
    try:
        out["provisioning_sessions_deleted"] = int(delete_provisioning_sessions_for_user(user_id))
    except Exception:
        out["provisioning_sessions_deleted"] = 0
    return out


def _delete_rows_for_user(db: Session, *, table_name: str, user_id: str) -> int:
    if not _table_exists(db, table_name):
        return 0
    dialect = db.bind.dialect.name if db.bind is not None else ""
    try:
        if dialect == "sqlite":
            res = db.execute(text(f"delete from {table_name} where user_id = :user_id"), {"user_id": user_id})
        else:
            res = db.execute(text(f"delete from {table_name} where user_id::text = :user_id"), {"user_id": user_id})
        return int(getattr(res, "rowcount", 0) or 0)
    except Exception:
        return 0


def _purge_operational_artifacts(db: Session, *, user_id: str) -> dict[str, Any]:
    tables = (
        "provisioning_requests",
        "provisioning_declined",
        "mcp_user_servers",
    )
    summary: dict[str, Any] = {"deleted": {}, "errors": []}
    for table_name in tables:
        try:
            summary["deleted"][table_name] = _delete_rows_for_user(db, table_name=table_name, user_id=user_id)
        except Exception as exc:
            summary["deleted"][table_name] = 0
            summary["errors"].append(f"{table_name}:{exc}")
    try:
        db.commit()
    except Exception:
        db.rollback()
    return summary


def _upsert_job(db: Session, *, user_id: str, stage: str, due_at: datetime) -> None:
    dialect = db.bind.dialect.name if db.bind is not None else ""
    now = _now_utc().replace(tzinfo=None)
    details_json = json.dumps({}, ensure_ascii=False)
    if dialect == "sqlite":
        existing = db.execute(
            text(
                """
                select id from account_deletion_jobs
                where user_id = :user_id and stage = :stage
                limit 1
                """
            ),
            {"user_id": user_id, "stage": stage},
        ).mappings().first()
        if existing:
            db.execute(
                text(
                    """
                    update account_deletion_jobs
                    set due_at = :due_at, status = 'pending', details_json = :details_json, updated_at = :updated_at
                    where user_id = :user_id and stage = :stage
                    """
                ),
                {
                    "due_at": due_at.replace(tzinfo=None).isoformat(sep=" "),
                    "details_json": details_json,
                    "updated_at": now.isoformat(sep=" "),
                    "user_id": user_id,
                    "stage": stage,
                },
            )
            return

        db.execute(
            text(
                """
                insert into account_deletion_jobs (id, user_id, stage, due_at, status, details_json, created_at, updated_at)
                values (:id, :user_id, :stage, :due_at, 'pending', :details_json, :created_at, :updated_at)
                """
            ),
            {
                "id": f"{user_id}:{stage}",
                "user_id": user_id,
                "stage": stage,
                "due_at": due_at.replace(tzinfo=None).isoformat(sep=" "),
                "details_json": details_json,
                "created_at": now.isoformat(sep=" "),
                "updated_at": now.isoformat(sep=" "),
            },
        )
        return

    existing = db.execute(
        text(
            """
            select id::text as id
            from account_deletion_jobs
            where user_id = :user_id and stage = :stage
            limit 1
            """
        ),
        {"user_id": user_id, "stage": stage},
    ).mappings().first()
    if existing:
        db.execute(
            text(
                """
                update account_deletion_jobs
                set due_at = :due_at,
                    status = 'pending',
                    details = (:details)::jsonb,
                    updated_at = now()
                where user_id = :user_id and stage = :stage
                """
            ),
            {"due_at": due_at, "details": details_json, "user_id": user_id, "stage": stage},
        )
        return

    db.execute(
        text(
            """
            insert into account_deletion_jobs (user_id, stage, due_at, status, details)
            values (:user_id, :stage, :due_at, 'pending', (:details)::jsonb)
            """
        ),
        {"user_id": user_id, "stage": stage, "due_at": due_at, "details": details_json},
    )


def start_account_deletion_pipeline(db: Session, *, user_id: str) -> dict[str, Any]:
    _ensure_pipeline_tables(db)
    now = _now_utc()

    _mark_user_deleted_requested(db, user_id=user_id, now=now)
    immediate = _revoke_immediate_tokens(db, user_id=user_id)

    _upsert_job(db, user_id=user_id, stage=STAGE_PURGE_24H, due_at=now + timedelta(hours=24))
    _upsert_job(db, user_id=user_id, stage=STAGE_PROVIDERS_7D, due_at=now + timedelta(days=7))
    _upsert_job(db, user_id=user_id, stage=STAGE_VERIFY_30D, due_at=now + timedelta(days=30))

    db.commit()
    return {
        "ok": True,
        "user_id": user_id,
        "scheduled_stages": [STAGE_PURGE_24H, STAGE_PROVIDERS_7D, STAGE_VERIFY_30D],
        "immediate": immediate,
        "started_at": now.isoformat(),
    }


def _delete_stripe_customer(db: Session, *, user_id: str) -> dict[str, Any]:
    if not (settings.STRIPE_SECRET_KEY or "").strip():
        return {"ok": False, "reason": "stripe_not_configured"}

    stripe.api_key = settings.STRIPE_SECRET_KEY
    customer_id = None
    if _table_exists(db, "subscriptions") and _column_exists(db, "subscriptions", "provider_customer_id"):
        if (db.bind and db.bind.dialect.name == "sqlite"):
            row = db.execute(
                text(
                    """
                    select provider_customer_id
                    from subscriptions
                    where user_id = :user_id
                    limit 1
                    """
                ),
                {"user_id": user_id},
            ).mappings().first()
        else:
            row = db.execute(
                text(
                    """
                    select provider_customer_id
                    from subscriptions
                    where user_id::text = :user_id
                    limit 1
                    """
                ),
                {"user_id": user_id},
            ).mappings().first()
        customer_id = str((row or {}).get("provider_customer_id") or "").strip() or None

    if not customer_id:
        return {"ok": False, "reason": "no_customer_id"}

    try:
        stripe.Customer.delete(customer_id)
        return {"ok": True, "customer_id": customer_id}
    except Exception as exc:
        return {"ok": False, "reason": f"stripe_delete_failed:{exc}"}


def _process_stage(db: Session, *, user_id: str, stage: str) -> dict[str, Any]:
    if stage == STAGE_PURGE_24H:
        result = delete_user_data(
            db=db,
            user_id=user_id,
            dry_run=False,
            keep_anonymized_transactions=True,
        )
        cleanup = _purge_operational_artifacts(db, user_id=user_id)
        if isinstance(result, dict):
            result["operational_cleanup"] = cleanup
            result["ok"] = bool(result.get("ok")) and not bool(cleanup.get("errors"))
        return result

    if stage == STAGE_PROVIDERS_7D:
        stripe_result = _delete_stripe_customer(db, user_id=user_id)
        # Keep vault clean even if provider deletion fails.
        if _table_exists(db, "oauth_tokens"):
            if db.bind and db.bind.dialect.name == "sqlite":
                db.execute(
                    text("delete from oauth_tokens where user_id = :user_id and provider = 'stripe_billing'"),
                    {"user_id": user_id},
                )
            else:
                db.execute(
                    text("delete from oauth_tokens where user_id::text = :user_id and provider = 'stripe_billing'"),
                    {"user_id": user_id},
                )
        db.commit()
        return {"ok": bool(stripe_result.get("ok")), "stripe": stripe_result}

    if stage == STAGE_VERIFY_30D:
        summary = get_user_data_summary(db=db, user_id=user_id)
        for table_name in ("provisioning_requests", "provisioning_declined", "mcp_user_servers"):
            if not _table_exists(db, table_name):
                summary[table_name] = 0
                continue
            dialect = db.bind.dialect.name if db.bind is not None else ""
            if dialect == "sqlite":
                row = db.execute(
                    text(f"select count(1) as c from {table_name} where user_id = :user_id"),
                    {"user_id": user_id},
                ).mappings().first()
            else:
                row = db.execute(
                    text(f"select count(1) as c from {table_name} where user_id::text = :user_id"),
                    {"user_id": user_id},
                ).mappings().first()
            summary[table_name] = int((row or {}).get("c") or 0)
        total = int(sum(summary.values()))
        return {"ok": total == 0, "remaining_records": total, "summary": summary}

    return {"ok": False, "error": f"unknown_stage:{stage}"}


def run_due_account_deletion_jobs(db: Session, *, limit: int = 50) -> dict[str, Any]:
    _ensure_pipeline_tables(db)
    now = _now_utc().replace(tzinfo=None)
    dialect = db.bind.dialect.name if db.bind is not None else ""

    if dialect == "sqlite":
        rows = db.execute(
            text(
                """
                select id, user_id, stage
                from account_deletion_jobs
                where status = 'pending' and due_at <= :now
                order by due_at asc
                limit :limit
                """
            ),
            {"now": now.isoformat(sep=" "), "limit": int(limit)},
        ).mappings().all()
    else:
        rows = db.execute(
            text(
                """
                select id::text as id, user_id, stage
                from account_deletion_jobs
                where status = 'pending' and due_at <= :now
                order by due_at asc
                limit :limit
                """
            ),
            {"now": now, "limit": int(limit)},
        ).mappings().all()

    processed = 0
    failed = 0
    for row in rows:
        user_id = str(row.get("user_id") or "")
        stage = str(row.get("stage") or "")
        result = _process_stage(db, user_id=user_id, stage=stage)
        status = "completed" if bool(result.get("ok")) else "failed"
        if status == "completed":
            processed += 1
        else:
            failed += 1

        details_json = json.dumps(result, ensure_ascii=False)
        if dialect == "sqlite":
            db.execute(
                text(
                    """
                    update account_deletion_jobs
                    set status = :status,
                        details_json = :details_json,
                        updated_at = :updated_at
                    where id = :id
                    """
                ),
                {
                    "status": status,
                    "details_json": details_json,
                    "updated_at": now.isoformat(sep=" "),
                    "id": str(row.get("id") or ""),
                },
            )
        else:
            db.execute(
                text(
                    """
                    update account_deletion_jobs
                    set status = :status,
                        details = (:details)::jsonb,
                        updated_at = now()
                    where id::text = :id
                    """
                ),
                {
                    "status": status,
                    "details": details_json,
                    "id": str(row.get("id") or ""),
                },
            )
        db.commit()

    return {"processed": processed, "failed": failed, "total_due": len(rows)}
