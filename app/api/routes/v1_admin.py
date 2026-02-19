from __future__ import annotations

import json
from dataclasses import dataclass
from datetime import datetime
from typing import Any

import jwt
from fastapi import APIRouter, Depends, HTTPException, Query, Request
from pydantic import BaseModel, Field
from sqlalchemy import text
from sqlalchemy.orm import Session

from app.api.deps import get_db
from app.core.config import settings
from app.services.prompt_versions import create_prompt_version, ensure_prompt_versions_table, rollback_prompt

router = APIRouter(prefix="/api/v1/admin", tags=["admin-v1"])


class SuspendUserRequest(BaseModel):
    reason: str = Field(default="", max_length=500)


class ResolveModerationRequest(BaseModel):
    status: str = Field(default="resolved")
    resolution_notes: str = Field(default="", max_length=1000)


class PromptRollbackRequest(BaseModel):
    prompt_group: str | None = None
    target_version_id: str | None = None


class PromptVersionCreateRequest(BaseModel):
    prompt_group: str = "system_prompt"
    version_label: str
    content: str
    status: str = "draft"
    rollout_percentage: int = 0
    metadata: dict[str, Any] = Field(default_factory=dict)


class PromptVersionStatusRequest(BaseModel):
    status: str
    rollout_percentage: int | None = None


@dataclass
class AdminContext:
    admin_id: str
    claims: dict[str, Any]


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


def _column_names(db: Session, table_name: str) -> set[str]:
    try:
        rows = db.execute(
            text(
                "select column_name from information_schema.columns "
                "where table_schema = current_schema() and table_name = :name"
            ),
            {"name": table_name},
        ).all()
        if rows:
            return {str(row[0]) for row in rows}
    except Exception:
        pass
    try:
        rows = db.execute(text(f"pragma table_info({table_name})")).mappings().all()
        return {str(row.get("name")) for row in rows}
    except Exception:
        return set()


def _decode_admin_claims(request: Request) -> AdminContext:
    auth = (request.headers.get("Authorization") or "").strip()
    if not auth.startswith("Bearer "):
        raise HTTPException(status_code=401, detail="Admin auth required")
    token = auth.split(" ", 1)[1].strip()
    try:
        claims = jwt.decode(token, settings.JWT_SECRET, algorithms=["HS256"])
    except Exception:
        raise HTTPException(status_code=401, detail="Invalid token")

    role = str(claims.get("role") or "").strip().lower()
    roles_raw = claims.get("roles")
    role_set: set[str] = set()
    if role:
        role_set.add(role)
    if isinstance(roles_raw, (list, tuple)):
        role_set.update(str(item).strip().lower() for item in roles_raw if str(item).strip())
    elif isinstance(roles_raw, str):
        role_set.update(part.strip().lower() for part in roles_raw.split(",") if part.strip())

    if "admin" not in role_set:
        raise HTTPException(status_code=403, detail="Admin role required")

    admin_id = str(claims.get("user_id") or claims.get("sub") or claims.get("email") or "admin")
    return AdminContext(admin_id=admin_id, claims=claims)


def _log_admin_action(
    db: Session,
    *,
    admin_id: str,
    action: str,
    resource_type: str,
    resource_id: str | None,
    metadata: dict[str, Any] | None = None,
) -> None:
    if not _table_exists(db, "audit_logs"):
        return
    payload = json.dumps(metadata or {}, ensure_ascii=False)
    now = datetime.utcnow()
    db.execute(
        text(
            """
            insert into audit_logs (
              user_id, actor_type, action, resource_type, resource_id, method, path, status_code, metadata_json, created_at
            ) values (
              :user_id, 'admin', :action, :resource_type, :resource_id, null, null, null, :metadata_json, :created_at
            )
            """
        ),
        {
            "user_id": admin_id,
            "action": action,
            "resource_type": resource_type,
            "resource_id": resource_id,
            "metadata_json": payload,
            "created_at": now,
        },
    )
    db.commit()


def _identity_table(db: Session) -> str:
    if _table_exists(db, "users"):
        return "users"
    if _table_exists(db, "accounts"):
        return "accounts"
    raise HTTPException(status_code=404, detail="No users/accounts table found")


@router.get("/users")
def admin_list_users(
    request: Request,
    q: str | None = Query(default=None),
    limit: int = Query(default=50, ge=1, le=200),
    offset: int = Query(default=0, ge=0),
    db: Session = Depends(get_db),
):
    _decode_admin_claims(request)
    table = _identity_table(db)
    cols = _column_names(db, table)
    select_cols = [c for c in ("id", "email", "phone", "full_name", "name", "account_status", "created_at", "updated_at") if c in cols]
    if "id" not in select_cols:
        raise HTTPException(status_code=500, detail=f"{table}.id column missing")

    filters: list[str] = []
    params: dict[str, Any] = {"limit": int(limit), "offset": int(offset)}
    if q and q.strip():
        token = f"%{q.strip().lower()}%"
        params["q"] = token
        search_cols = [c for c in ("id", "email", "phone", "full_name", "name") if c in cols]
        if search_cols:
            filters.append(
                "(" + " or ".join([f"lower(cast({col} as text)) like :q" for col in search_cols]) + ")"
            )

    where_sql = f"where {' and '.join(filters)}" if filters else ""
    sql = (
        f"select {', '.join(select_cols)} from {table} "
        f"{where_sql} order by created_at desc nulls last, id asc limit :limit offset :offset"
    )
    rows = db.execute(text(sql), params).mappings().all()
    return {"ok": True, "table": table, "count": len(rows), "users": [dict(row) for row in rows]}


@router.get("/users/{user_id}")
def admin_user_detail(request: Request, user_id: str, db: Session = Depends(get_db)):
    _decode_admin_claims(request)
    table = _identity_table(db)
    row = db.execute(text(f"select * from {table} where id = :id limit 1"), {"id": user_id}).mappings().first()
    if not row:
        raise HTTPException(status_code=404, detail="User not found")

    channels: list[dict[str, Any]] = []
    for ch_table in ("channel_connections", "user_channels"):
        if _table_exists(db, ch_table):
            channel_rows = db.execute(
                text(f"select * from {ch_table} where user_id = :id order by created_at desc"),
                {"id": user_id},
            ).mappings().all()
            channels.extend(dict(item) for item in channel_rows)

    subscription = None
    if _table_exists(db, "subscriptions"):
        subscription = db.execute(
            text("select * from subscriptions where user_id = :id order by updated_at desc limit 1"),
            {"id": user_id},
        ).mappings().first()

    return {
        "ok": True,
        "user": dict(row),
        "channels": channels,
        "subscription": dict(subscription) if subscription else None,
    }


@router.post("/users/{user_id}/suspend")
def admin_suspend_user(request: Request, user_id: str, payload: SuspendUserRequest, db: Session = Depends(get_db)):
    admin = _decode_admin_claims(request)
    table = _identity_table(db)
    cols = _column_names(db, table)
    if "account_status" not in cols:
        raise HTTPException(status_code=400, detail=f"{table}.account_status column missing")
    updated = db.execute(
        text(f"update {table} set account_status = 'suspended' where id = :id"),
        {"id": user_id},
    ).rowcount
    db.commit()
    if not updated:
        raise HTTPException(status_code=404, detail="User not found")
    _log_admin_action(
        db,
        admin_id=admin.admin_id,
        action="suspend_user",
        resource_type="user",
        resource_id=user_id,
        metadata={"reason": payload.reason},
    )
    return {"ok": True, "user_id": user_id, "status": "suspended"}


@router.get("/mcp/health")
def admin_mcp_health(request: Request, db: Session = Depends(get_db)):
    _decode_admin_claims(request)
    if not _table_exists(db, "mcp_servers"):
        return {"ok": True, "servers": [], "note": "mcp_servers table not found"}
    rows = db.execute(
        text("select * from mcp_servers order by updated_at desc nulls last, id asc"),
    ).mappings().all()
    return {"ok": True, "servers": [dict(row) for row in rows], "count": len(rows)}


@router.get("/moderation/queue")
def admin_moderation_queue(
    request: Request,
    status: str = Query(default="pending"),
    limit: int = Query(default=100, ge=1, le=500),
    db: Session = Depends(get_db),
):
    _decode_admin_claims(request)
    if not _table_exists(db, "moderation_queue"):
        return {"ok": True, "items": [], "count": 0, "note": "moderation_queue table not found"}
    rows = db.execute(
        text(
            "select * from moderation_queue "
            "where (:status = '' or status = :status) "
            "order by created_at desc "
            "limit :limit"
        ),
        {"status": status or "", "limit": int(limit)},
    ).mappings().all()
    return {"ok": True, "count": len(rows), "items": [dict(row) for row in rows]}


@router.post("/moderation/queue/{item_id}/resolve")
def admin_resolve_moderation(
    request: Request,
    item_id: str,
    payload: ResolveModerationRequest,
    db: Session = Depends(get_db),
):
    admin = _decode_admin_claims(request)
    if not _table_exists(db, "moderation_queue"):
        raise HTTPException(status_code=404, detail="moderation_queue table not found")
    updated = db.execute(
        text(
            "update moderation_queue "
            "set status = :status, resolved_at = :resolved_at, resolver_id = :resolver_id, resolution_notes = :notes "
            "where id = :id"
        ),
        {
            "id": item_id,
            "status": payload.status,
            "resolved_at": datetime.utcnow(),
            "resolver_id": admin.admin_id,
            "notes": payload.resolution_notes,
        },
    ).rowcount
    db.commit()
    if not updated:
        raise HTTPException(status_code=404, detail="Moderation item not found")
    _log_admin_action(
        db,
        admin_id=admin.admin_id,
        action="resolve_moderation_item",
        resource_type="moderation_queue",
        resource_id=item_id,
        metadata={"status": payload.status},
    )
    return {"ok": True, "id": item_id, "status": payload.status}


@router.get("/prompts/versions")
def admin_list_prompt_versions(
    request: Request,
    prompt_group: str = Query(default="system_prompt"),
    limit: int = Query(default=100, ge=1, le=500),
    db: Session = Depends(get_db),
):
    _decode_admin_claims(request)
    ensure_prompt_versions_table(db)
    rows = db.execute(
        text(
            "select * from prompt_versions "
            "where prompt_group = :prompt_group "
            "order by updated_at desc, created_at desc limit :limit"
        ),
        {"prompt_group": prompt_group, "limit": int(limit)},
    ).mappings().all()
    return {"ok": True, "count": len(rows), "versions": [dict(row) for row in rows]}


@router.post("/prompts/versions")
def admin_create_prompt_version(request: Request, payload: PromptVersionCreateRequest, db: Session = Depends(get_db)):
    admin = _decode_admin_claims(request)
    ensure_prompt_versions_table(db)
    version_id = create_prompt_version(
        db,
        prompt_group=payload.prompt_group,
        version_label=payload.version_label,
        content=payload.content,
        status=payload.status,
        rollout_percentage=payload.rollout_percentage,
        metadata=payload.metadata,
    )
    _log_admin_action(
        db,
        admin_id=admin.admin_id,
        action="create_prompt_version",
        resource_type="prompt_versions",
        resource_id=version_id,
        metadata={"prompt_group": payload.prompt_group, "status": payload.status},
    )
    return {"ok": True, "id": version_id}


@router.post("/prompts/versions/{version_id}/status")
def admin_update_prompt_version_status(
    request: Request,
    version_id: str,
    payload: PromptVersionStatusRequest,
    db: Session = Depends(get_db),
):
    admin = _decode_admin_claims(request)
    ensure_prompt_versions_table(db)
    updates = ["status = :status", "updated_at = :updated_at"]
    params: dict[str, Any] = {"id": version_id, "status": payload.status, "updated_at": datetime.utcnow()}
    if payload.rollout_percentage is not None:
        updates.append("rollout_percentage = :rollout_percentage")
        params["rollout_percentage"] = max(0, min(100, int(payload.rollout_percentage)))
    updated = db.execute(
        text(f"update prompt_versions set {', '.join(updates)} where id = :id"),
        params,
    ).rowcount
    db.commit()
    if not updated:
        raise HTTPException(status_code=404, detail="Prompt version not found")
    _log_admin_action(
        db,
        admin_id=admin.admin_id,
        action="update_prompt_version_status",
        resource_type="prompt_versions",
        resource_id=version_id,
        metadata={"status": payload.status, "rollout_percentage": payload.rollout_percentage},
    )
    return {"ok": True, "id": version_id, "status": payload.status}


@router.post("/prompts/rollback")
def admin_prompt_rollback(request: Request, payload: PromptRollbackRequest, db: Session = Depends(get_db)):
    admin = _decode_admin_claims(request)
    if not _table_exists(db, "prompt_versions"):
        raise HTTPException(status_code=404, detail="prompt_versions table not found")
    prompt_group = str(payload.prompt_group or "system_prompt")
    active_id = rollback_prompt(
        db,
        prompt_group=prompt_group,
        target_version_id=(str(payload.target_version_id or "").strip() or None),
    )
    if not active_id:
        raise HTTPException(status_code=404, detail="No prompt versions found for rollback")

    _log_admin_action(
        db,
        admin_id=admin.admin_id,
        action="rollback_prompt",
        resource_type="prompt_versions",
        resource_id=active_id,
        metadata={"prompt_group": prompt_group},
    )
    return {"ok": True, "active_prompt_version_id": active_id}


@router.get("/analytics/daily")
def admin_analytics_daily(
    request: Request,
    days: int = Query(default=30, ge=1, le=365),
    db: Session = Depends(get_db),
):
    _decode_admin_claims(request)
    if not _table_exists(db, "analytics_daily"):
        return {"ok": True, "rows": [], "count": 0, "note": "analytics_daily table not found"}
    rows = db.execute(
        text("select * from analytics_daily order by day desc limit :limit"),
        {"limit": int(days)},
    ).mappings().all()
    return {"ok": True, "count": len(rows), "rows": [dict(row) for row in rows]}
