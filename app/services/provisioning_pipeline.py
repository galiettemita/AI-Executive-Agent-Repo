from __future__ import annotations

import json
import threading
import uuid
from datetime import datetime, timedelta, timezone
from typing import Any

from sqlalchemy import text
from sqlalchemy.orm import Session

from app.blueprint.contracts import (
    AuthType,
    ProvisionTrigger,
    ProvisioningRequest,
    ProvisioningState,
)
from app.services.provisioning_catalog import available_servers_for_user

_TABLE_LOCK = threading.Lock()
_TABLE_READY = False

_ACTIVE_STATES = {
    ProvisioningState.INITIATED.value,
    ProvisioningState.AWAITING_AUTH.value,
    ProvisioningState.AUTH_RECEIVED.value,
    ProvisioningState.PROVISIONING.value,
}

_ALLOWED_TRANSITIONS: dict[ProvisioningState, set[ProvisioningState]] = {
    ProvisioningState.INITIATED: {
        ProvisioningState.AWAITING_AUTH,
        ProvisioningState.CANCELED,
        ProvisioningState.FAILED,
        ProvisioningState.EXPIRED,
    },
    ProvisioningState.AWAITING_AUTH: {
        ProvisioningState.AUTH_RECEIVED,
        ProvisioningState.EXPIRED,
        ProvisioningState.CANCELED,
        ProvisioningState.FAILED,
    },
    ProvisioningState.AUTH_RECEIVED: {
        ProvisioningState.PROVISIONING,
        ProvisioningState.CANCELED,
        ProvisioningState.FAILED,
    },
    ProvisioningState.PROVISIONING: {
        ProvisioningState.ACTIVE,
        ProvisioningState.FAILED,
        ProvisioningState.EXPIRED,
    },
    ProvisioningState.FAILED: {
        ProvisioningState.PROVISIONING,
        ProvisioningState.CANCELED,
    },
    ProvisioningState.EXPIRED: {
        ProvisioningState.INITIATED,
        ProvisioningState.CANCELED,
    },
    ProvisioningState.CANCELED: {
        ProvisioningState.INITIATED,
    },
    ProvisioningState.ACTIVE: set(),
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


def ensure_provisioning_tables(db: Session) -> None:
    global _TABLE_READY
    if _TABLE_READY and _table_exists(db, "provisioning_requests") and _table_exists(db, "server_catalog"):
        return
    with _TABLE_LOCK:
        if _TABLE_READY and _table_exists(db, "provisioning_requests") and _table_exists(db, "server_catalog"):
            return
        dialect = db.bind.dialect.name if db.bind is not None else ""
        if dialect == "sqlite":
            db.execute(
                text(
                    """
                    create table if not exists provisioning_requests (
                      id text primary key,
                      user_id text not null,
                      server_id text not null,
                      state text not null default 'initiated',
                      trigger text not null default 'capability_gap',
                      auth_type text,
                      reason text,
                      original_task_id text,
                      retry_count integer default 0,
                      state_history text,
                      error_message text,
                      expires_at datetime,
                      created_at datetime default current_timestamp,
                      updated_at datetime default current_timestamp,
                      completed_at datetime
                    )
                    """
                )
            )
            db.execute(
                text(
                    """
                    create table if not exists server_catalog (
                      server_id text primary key,
                      display_name text,
                      description text,
                      auth_type text,
                      min_plan text default 'free',
                      setup_seconds integer default 30,
                      capabilities text,
                      keywords text,
                      hosting_model text,
                      oauth_config text,
                      container_image text,
                      source text default 'local',
                      signature text,
                      status text default 'active',
                      updated_at datetime default current_timestamp,
                      created_at datetime default current_timestamp
                    )
                    """
                )
            )
            db.execute(
                text(
                    """
                    create table if not exists provisioning_declined (
                      id text primary key,
                      user_id text not null,
                      server_id text not null,
                      reason text,
                      declined_at datetime default current_timestamp,
                      cooldown_until datetime,
                      created_at datetime default current_timestamp
                    )
                    """
                )
            )
        else:
            db.execute(
                text(
                    """
                    create table if not exists provisioning_requests (
                      id text primary key,
                      user_id text not null,
                      server_id text not null,
                      state text not null default 'initiated',
                      trigger text not null default 'capability_gap',
                      auth_type text,
                      reason text,
                      original_task_id text,
                      retry_count integer default 0,
                      state_history jsonb,
                      error_message text,
                      expires_at timestamptz,
                      created_at timestamptz default now(),
                      updated_at timestamptz default now(),
                      completed_at timestamptz
                    )
                    """
                )
            )
            db.execute(
                text(
                    """
                    create table if not exists server_catalog (
                      server_id text primary key,
                      display_name text,
                      description text,
                      auth_type text,
                      min_plan text default 'free',
                      setup_seconds integer default 30,
                      capabilities jsonb,
                      keywords jsonb,
                      hosting_model text,
                      oauth_config jsonb,
                      container_image text,
                      source text default 'local',
                      signature text,
                      status text default 'active',
                      updated_at timestamptz default now(),
                      created_at timestamptz default now()
                    )
                    """
                )
            )
            db.execute(
                text(
                    """
                    create table if not exists provisioning_declined (
                      id text primary key,
                      user_id text not null,
                      server_id text not null,
                      reason text,
                      declined_at timestamptz default now(),
                      cooldown_until timestamptz,
                      created_at timestamptz default now()
                    )
                    """
                )
            )
        db.execute(text("create index if not exists idx_provisioning_requests_user_state on provisioning_requests(user_id, state, updated_at)"))
        db.execute(text("create index if not exists idx_provisioning_requests_server_state on provisioning_requests(server_id, state, updated_at)"))
        db.execute(text("create index if not exists idx_provisioning_declined_user_server on provisioning_declined(user_id, server_id, declined_at)"))
        db.commit()
        _TABLE_READY = True


def _as_history(value: Any) -> list[dict[str, Any]]:
    if isinstance(value, list):
        return [item for item in value if isinstance(item, dict)]
    if isinstance(value, str) and value.strip():
        try:
            parsed = json.loads(value)
            if isinstance(parsed, list):
                return [item for item in parsed if isinstance(item, dict)]
        except Exception:
            return []
    return []


def _to_request(row: dict[str, Any]) -> ProvisioningRequest:
    state_value = str(row.get("state") or ProvisioningState.INITIATED.value)
    trigger_value = str(row.get("trigger") or ProvisionTrigger.CAPABILITY_GAP.value)
    auth_value = str(row.get("auth_type") or AuthType.OAUTH2.value)
    return ProvisioningRequest(
        id=str(row.get("id") or ""),
        user_id=str(row.get("user_id") or ""),
        server_id=str(row.get("server_id") or ""),
        state=ProvisioningState(state_value),
        trigger=ProvisionTrigger(trigger_value),
        auth_type=AuthType(auth_value),
        reason=str(row.get("reason") or ""),
        original_task_id=(str(row.get("original_task_id") or "").strip() or None),
        retry_count=int(row.get("retry_count") or 0),
        state_history=_as_history(row.get("state_history")),
        error_message=(str(row.get("error_message") or "").strip() or None),
        expires_at=row.get("expires_at"),
        created_at=row.get("created_at") or datetime.utcnow(),
        updated_at=row.get("updated_at") or datetime.utcnow(),
        completed_at=row.get("completed_at"),
    )


def get_request(db: Session, *, request_id: str) -> ProvisioningRequest | None:
    ensure_provisioning_tables(db)
    row = db.execute(
        text("select * from provisioning_requests where id = :id limit 1"),
        {"id": request_id},
    ).mappings().first()
    if not row:
        return None
    return _to_request(dict(row))


def _find_active_request(db: Session, *, user_id: str, server_id: str, now_utc: datetime) -> ProvisioningRequest | None:
    ensure_provisioning_tables(db)
    row = db.execute(
        text(
            """
            select * from provisioning_requests
            where user_id = :user_id
              and server_id = :server_id
              and state in ('initiated', 'awaiting_auth', 'auth_received', 'provisioning')
              and (expires_at is null or expires_at > :now_utc)
            order by updated_at desc, created_at desc
            limit 1
            """
        ),
        {"user_id": user_id, "server_id": server_id, "now_utc": now_utc},
    ).mappings().first()
    if not row:
        return None
    return _to_request(dict(row))


def begin_request(
    db: Session,
    *,
    user_id: str,
    server_id: str,
    reason: str,
    trigger: ProvisionTrigger = ProvisionTrigger.CAPABILITY_GAP,
    auth_type: AuthType = AuthType.OAUTH2,
    original_task_id: str | None = None,
    expires_in_minutes: int = 15,
) -> ProvisioningRequest:
    ensure_provisioning_tables(db)
    now_utc = datetime.now(timezone.utc)
    existing = _find_active_request(db, user_id=user_id, server_id=server_id, now_utc=now_utc)
    if existing:
        return existing

    request_id = str(uuid.uuid4())
    expires_at = now_utc + timedelta(minutes=max(5, int(expires_in_minutes or 15)))
    history = [{"state": ProvisioningState.INITIATED.value, "at": now_utc.isoformat(), "note": "request_created"}]
    dialect = db.bind.dialect.name if db.bind is not None else ""
    params = {
        "id": request_id,
        "user_id": user_id,
        "server_id": server_id,
        "state": ProvisioningState.INITIATED.value,
        "trigger": trigger.value,
        "auth_type": auth_type.value,
        "reason": reason,
        "original_task_id": original_task_id,
        "retry_count": 0,
        "state_history": json.dumps(history, ensure_ascii=False),
        "expires_at": expires_at,
        "updated_at": now_utc,
    }
    if dialect == "sqlite":
        db.execute(
            text(
                """
                insert into provisioning_requests (
                  id, user_id, server_id, state, trigger, auth_type, reason, original_task_id,
                  retry_count, state_history, expires_at, updated_at
                ) values (
                  :id, :user_id, :server_id, :state, :trigger, :auth_type, :reason, :original_task_id,
                  :retry_count, :state_history, :expires_at, :updated_at
                )
                """
            ),
            params,
        )
    else:
        db.execute(
            text(
                """
                insert into provisioning_requests (
                  id, user_id, server_id, state, trigger, auth_type, reason, original_task_id,
                  retry_count, state_history, expires_at, updated_at
                ) values (
                  :id, :user_id, :server_id, :state, :trigger, :auth_type, :reason, :original_task_id,
                  :retry_count, cast(:state_history as jsonb), :expires_at, :updated_at
                )
                """
            ),
            params,
        )
    db.commit()
    created = get_request(db, request_id=request_id)
    if not created:
        raise RuntimeError("failed to create provisioning request")
    return created


def transition_request(
    db: Session,
    *,
    request_id: str,
    new_state: ProvisioningState,
    note: str = "",
    error_message: str | None = None,
) -> ProvisioningRequest:
    ensure_provisioning_tables(db)
    current = get_request(db, request_id=request_id)
    if not current:
        raise ValueError("provisioning request not found")

    allowed = _ALLOWED_TRANSITIONS.get(current.state, set())
    if new_state not in allowed and new_state != current.state:
        raise ValueError(f"invalid transition: {current.state.value} -> {new_state.value}")

    now_utc = datetime.now(timezone.utc)
    history = list(current.state_history or [])
    history.append({"state": new_state.value, "at": now_utc.isoformat(), "note": note or "state_transition"})
    completed_at = now_utc if new_state in {ProvisioningState.ACTIVE, ProvisioningState.CANCELED, ProvisioningState.EXPIRED, ProvisioningState.FAILED} else None
    retry_count = int(current.retry_count or 0)
    if new_state == ProvisioningState.PROVISIONING and current.state == ProvisioningState.FAILED:
        retry_count += 1

    dialect = db.bind.dialect.name if db.bind is not None else ""
    params = {
        "id": request_id,
        "state": new_state.value,
        "history": json.dumps(history, ensure_ascii=False),
        "error_message": error_message,
        "retry_count": retry_count,
        "updated_at": now_utc,
        "completed_at": completed_at,
    }
    if dialect == "sqlite":
        db.execute(
            text(
                """
                update provisioning_requests
                set state = :state,
                    state_history = :history,
                    error_message = :error_message,
                    retry_count = :retry_count,
                    updated_at = :updated_at,
                    completed_at = :completed_at
                where id = :id
                """
            ),
            params,
        )
    else:
        db.execute(
            text(
                """
                update provisioning_requests
                set state = :state,
                    state_history = cast(:history as jsonb),
                    error_message = :error_message,
                    retry_count = :retry_count,
                    updated_at = :updated_at,
                    completed_at = :completed_at
                where id = :id
                """
            ),
            params,
        )
    db.commit()
    updated = get_request(db, request_id=request_id)
    if not updated:
        raise RuntimeError("failed to update provisioning request")
    return updated


def expire_timed_out_requests(db: Session, *, now_utc: datetime | None = None) -> int:
    ensure_provisioning_tables(db)
    current = now_utc or datetime.now(timezone.utc)
    rows = db.execute(
        text(
            """
            select id from provisioning_requests
            where state in ('initiated', 'awaiting_auth', 'auth_received', 'provisioning')
              and expires_at is not null
              and expires_at <= :now_utc
            """
        ),
        {"now_utc": current},
    ).mappings().all()
    count = 0
    for row in rows:
        request_id = str((row or {}).get("id") or "")
        if not request_id:
            continue
        transition_request(
            db,
            request_id=request_id,
            new_state=ProvisioningState.EXPIRED,
            note="expired_timeout",
        )
        count += 1
    return count


def record_declined(
    db: Session,
    *,
    user_id: str,
    server_id: str,
    reason: str = "not_now",
    cooldown_days: int = 7,
) -> str:
    ensure_provisioning_tables(db)
    declined_id = str(uuid.uuid4())
    now_utc = datetime.now(timezone.utc)
    cooldown_until = now_utc + timedelta(days=max(1, int(cooldown_days or 7)))
    db.execute(
        text(
            """
            insert into provisioning_declined (
              id, user_id, server_id, reason, declined_at, cooldown_until, created_at
            ) values (
              :id, :user_id, :server_id, :reason, :declined_at, :cooldown_until, :created_at
            )
            """
        ),
        {
            "id": declined_id,
            "user_id": user_id,
            "server_id": server_id,
            "reason": reason,
            "declined_at": now_utc,
            "cooldown_until": cooldown_until,
            "created_at": now_utc,
        },
    )
    db.commit()
    return declined_id


def search_catalog_entries(
    db: Session,
    *,
    user_id: str | None,
    query: str,
    limit: int = 10,
    connected_server_ids: set[str] | None = None,
) -> list[dict[str, Any]]:
    entries = available_servers_for_user(
        db,
        user_id=user_id,
        connected_server_ids=connected_server_ids or set(),
    )
    q = str(query or "").strip().lower()
    if not q:
        return entries[: max(1, int(limit or 10))]

    tokens = {token for token in q.replace("-", " ").split() if token}
    scored: list[tuple[int, dict[str, Any]]] = []
    for entry in entries:
        haystack = " ".join(
            [
                str(entry.get("server_id") or "").lower(),
                str(entry.get("description") or "").lower(),
                " ".join(str(item).lower() for item in (entry.get("capabilities") or [])),
                " ".join(str(item).lower() for item in (entry.get("keywords") or [])),
            ]
        )
        score = 0
        for token in tokens:
            if token in haystack:
                score += 1
        if score > 0:
            scored.append((score, entry))
    scored.sort(key=lambda item: (-item[0], str(item[1].get("server_id") or "")))
    return [item[1] for item in scored[: max(1, int(limit or 10))]]


class ProvisioningPipeline:
    def __init__(self, db: Session) -> None:
        self.db = db
        ensure_provisioning_tables(self.db)

    def begin(
        self,
        *,
        user_id: str,
        server_id: str,
        reason: str,
        trigger: ProvisionTrigger = ProvisionTrigger.CAPABILITY_GAP,
        auth_type: AuthType = AuthType.OAUTH2,
        original_task_id: str | None = None,
    ) -> ProvisioningRequest:
        return begin_request(
            self.db,
            user_id=user_id,
            server_id=server_id,
            reason=reason,
            trigger=trigger,
            auth_type=auth_type,
            original_task_id=original_task_id,
        )

    def transition(
        self,
        *,
        request_id: str,
        new_state: ProvisioningState,
        note: str = "",
        error_message: str | None = None,
    ) -> ProvisioningRequest:
        return transition_request(
            self.db,
            request_id=request_id,
            new_state=new_state,
            note=note,
            error_message=error_message,
        )

    def expire_timeouts(self) -> int:
        return expire_timed_out_requests(self.db)

    def search_catalog(
        self,
        *,
        user_id: str | None,
        query: str,
        limit: int = 10,
        connected_server_ids: set[str] | None = None,
    ) -> list[dict[str, Any]]:
        return search_catalog_entries(
            self.db,
            user_id=user_id,
            query=query,
            limit=limit,
            connected_server_ids=connected_server_ids,
        )
