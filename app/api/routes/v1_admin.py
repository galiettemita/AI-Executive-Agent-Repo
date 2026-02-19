from __future__ import annotations

import json
from dataclasses import dataclass
from datetime import datetime, timedelta
from typing import Any

import jwt
from fastapi import APIRouter, Depends, HTTPException, Query, Request
from fastapi.responses import HTMLResponse
from pydantic import BaseModel, Field
from sqlalchemy import text
from sqlalchemy.orm import Session

from app.api.deps import get_db
from app.core.config import settings
from app.services.analytics import wave56_server_prioritization
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


@router.get("/analytics/wave56-prioritization")
def admin_wave56_prioritization(
    request: Request,
    days: int = Query(default=30, ge=1, le=365),
    db: Session = Depends(get_db),
):
    _decode_admin_claims(request)
    payload = wave56_server_prioritization(db, days=int(days))
    return {"ok": True, **payload}


@router.get("/provisioning/requests")
def admin_provisioning_requests(
    request: Request,
    state: str = Query(default=""),
    user_id: str = Query(default=""),
    server_id: str = Query(default=""),
    limit: int = Query(default=100, ge=1, le=1000),
    offset: int = Query(default=0, ge=0),
    db: Session = Depends(get_db),
):
    _decode_admin_claims(request)
    if not _table_exists(db, "provisioning_requests"):
        return {"ok": True, "count": 0, "items": [], "note": "provisioning_requests table not found"}

    where_parts = ["1=1"]
    params: dict[str, Any] = {"limit": int(limit), "offset": int(offset)}
    if state.strip():
        where_parts.append("state = :state")
        params["state"] = state.strip().lower()
    if user_id.strip():
        where_parts.append("user_id = :user_id")
        params["user_id"] = user_id.strip()
    if server_id.strip():
        where_parts.append("server_id = :server_id")
        params["server_id"] = server_id.strip().lower()

    rows = db.execute(
        text(
            "select * from provisioning_requests "
            f"where {' and '.join(where_parts)} "
            "order by updated_at desc, created_at desc "
            "limit :limit offset :offset"
        ),
        params,
    ).mappings().all()
    return {"ok": True, "count": len(rows), "items": [dict(row) for row in rows]}


@router.get("/provisioning/stats")
def admin_provisioning_stats(
    request: Request,
    days: int = Query(default=30, ge=1, le=365),
    db: Session = Depends(get_db),
):
    _decode_admin_claims(request)
    if not _table_exists(db, "provisioning_requests"):
        return {
            "ok": True,
            "window_days": int(days),
            "totals": {"requests": 0, "active": 0, "success": 0, "failed": 0, "expired": 0, "canceled": 0},
            "success_rate": 0.0,
            "avg_completion_seconds": 0.0,
            "by_state": {},
            "note": "provisioning_requests table not found",
        }

    dialect = db.bind.dialect.name if db.bind is not None else ""
    cutoff = datetime.utcnow() - timedelta(days=int(days))
    cutoff_value: Any = cutoff if dialect != "sqlite" else cutoff.isoformat(sep=" ")

    state_rows = db.execute(
        text(
            """
            select state, count(*) as c
            from provisioning_requests
            where created_at >= :cutoff
            group by state
            """
        ),
        {"cutoff": cutoff_value},
    ).mappings().all()
    by_state: dict[str, int] = {
        str(row.get("state") or "unknown"): int(row.get("c") or 0) for row in state_rows
    }

    total = int(sum(by_state.values()))
    success = int(by_state.get("active", 0))
    failed = int(by_state.get("failed", 0))
    expired = int(by_state.get("expired", 0))
    canceled = int(by_state.get("canceled", 0))
    active = int(
        by_state.get("initiated", 0)
        + by_state.get("awaiting_auth", 0)
        + by_state.get("auth_received", 0)
        + by_state.get("provisioning", 0)
    )
    terminal = success + failed + expired + canceled
    success_rate = round((success / terminal), 4) if terminal > 0 else 0.0

    if dialect == "sqlite":
        avg_row = db.execute(
            text(
                """
                select avg((julianday(completed_at) - julianday(created_at)) * 86400.0) as avg_seconds
                from provisioning_requests
                where created_at >= :cutoff
                  and completed_at is not null
                  and state = 'active'
                """
            ),
            {"cutoff": cutoff_value},
        ).mappings().first()
    else:
        avg_row = db.execute(
            text(
                """
                select avg(extract(epoch from (completed_at - created_at))) as avg_seconds
                from provisioning_requests
                where created_at >= :cutoff
                  and completed_at is not null
                  and state = 'active'
                """
            ),
            {"cutoff": cutoff_value},
        ).mappings().first()
    avg_completion_seconds = round(float((avg_row or {}).get("avg_seconds") or 0.0), 3)

    return {
        "ok": True,
        "window_days": int(days),
        "totals": {
            "requests": total,
            "active": active,
            "success": success,
            "failed": failed,
            "expired": expired,
            "canceled": canceled,
        },
        "success_rate": success_rate,
        "avg_completion_seconds": avg_completion_seconds,
        "by_state": by_state,
    }


@router.get("/dashboard/provisioning", response_class=HTMLResponse)
def admin_provisioning_dashboard(
    request: Request,
    days: int = Query(default=30, ge=1, le=365),
    limit: int = Query(default=50, ge=1, le=500),
    db: Session = Depends(get_db),
):
    _decode_admin_claims(request)
    stats_payload = admin_provisioning_stats(request=request, days=days, db=db)
    history_payload = admin_provisioning_requests(
        request=request,
        state="",
        user_id="",
        server_id="",
        limit=limit,
        offset=0,
        db=db,
    )
    mcp_payload = admin_mcp_health(request=request, db=db)
    stats = stats_payload.get("totals") or {}
    success_rate = float(stats_payload.get("success_rate") or 0.0) * 100.0
    mcp_servers = list(mcp_payload.get("servers") or [])
    healthy_mcp = sum(1 for item in mcp_servers if str(item.get("health_status") or "").strip().lower() in {"healthy", "ok"})
    rows_html = "".join(
        (
            "<tr>"
            f"<td>{str(item.get('created_at') or '')}</td>"
            f"<td>{str(item.get('user_id') or '')}</td>"
            f"<td>{str(item.get('server_id') or '')}</td>"
            f"<td>{str(item.get('state') or '')}</td>"
            f"<td>{str(item.get('updated_at') or '')}</td>"
            "</tr>"
        )
        for item in (history_payload.get("items") or [])
    )
    if not rows_html:
        rows_html = "<tr><td colspan='5'>No provisioning requests in this window.</td></tr>"

    html = (
        "<html><body style='font-family: sans-serif; padding: 20px;'>"
        "<h2>System Health</h2>"
        "<ul>"
        "<li>API status: healthy</li>"
        f"<li>MCP servers total: {len(mcp_servers)}</li>"
        f"<li>MCP servers healthy: {healthy_mcp}</li>"
        "</ul>"
        "<h2>Provisioning Dashboard</h2>"
        f"<p>Window: last {int(days)} day(s)</p>"
        "<ul>"
        f"<li>Total requests: {int(stats.get('requests') or 0)}</li>"
        f"<li>Success: {int(stats.get('success') or 0)}</li>"
        f"<li>Failed: {int(stats.get('failed') or 0)}</li>"
        f"<li>Expired: {int(stats.get('expired') or 0)}</li>"
        f"<li>Canceled: {int(stats.get('canceled') or 0)}</li>"
        f"<li>Active: {int(stats.get('active') or 0)}</li>"
        f"<li>Success rate: {success_rate:.2f}%</li>"
        "</ul>"
        "<table border='1' cellspacing='0' cellpadding='6'>"
        "<thead><tr><th>Created</th><th>User</th><th>Server</th><th>State</th><th>Updated</th></tr></thead>"
        f"<tbody>{rows_html}</tbody>"
        "</table>"
        "</body></html>"
    )
    return HTMLResponse(content=html)


@router.get("/dashboard", response_class=HTMLResponse)
def admin_dashboard(
    request: Request,
    days: int = Query(default=30, ge=1, le=365),
    db: Session = Depends(get_db),
):
    _decode_admin_claims(request)

    users_payload = admin_list_users(request=request, q=None, limit=25, offset=0, db=db)
    moderation_payload = admin_moderation_queue(request=request, status="", limit=25, db=db)
    mcp_payload = admin_mcp_health(request=request, db=db)
    analytics_payload = admin_analytics_daily(request=request, days=days, db=db)
    provisioning_payload = admin_provisioning_stats(request=request, days=days, db=db)

    eval_avg_overall = 0.0
    eval_avg_safety = 0.0
    if _table_exists(db, "eval_results"):
        row = db.execute(
            text(
                "select avg(overall_score) as avg_overall, avg(safety_score) as avg_safety "
                "from eval_results"
            )
        ).mappings().first()
        eval_avg_overall = round(float((row or {}).get("avg_overall") or 0.0), 3)
        eval_avg_safety = round(float((row or {}).get("avg_safety") or 0.0), 3)

    users = list(users_payload.get("users") or [])
    moderation_items = list(moderation_payload.get("items") or [])
    mcp_servers = list(mcp_payload.get("servers") or [])
    analytics_rows = list(analytics_payload.get("rows") or [])

    latest_analytics = analytics_rows[0] if analytics_rows else {}
    dau = int(latest_analytics.get("dau") or 0)
    mau = int(latest_analytics.get("mau") or 0)
    revenue_cents = int(latest_analytics.get("revenue_cents") or 0)
    message_volume = int(latest_analytics.get("message_volume") or 0)
    tool_calls = int(latest_analytics.get("tool_calls") or 0)

    healthy_mcp = sum(1 for item in mcp_servers if str(item.get("health_status") or "").strip().lower() in {"healthy", "ok"})
    prov_totals = provisioning_payload.get("totals") or {}
    prov_success_rate = float(provisioning_payload.get("success_rate") or 0.0) * 100.0

    users_rows_html = "".join(
        f"<tr><td>{str(item.get('id') or '')}</td><td>{str(item.get('email') or item.get('phone') or '')}</td><td>{str(item.get('account_status') or '')}</td></tr>"
        for item in users
    ) or "<tr><td colspan='3'>No users found</td></tr>"

    moderation_rows_html = "".join(
        f"<tr><td>{str(item.get('id') or '')}</td><td>{str(item.get('status') or '')}</td><td>{str(item.get('risk_score') or '')}</td></tr>"
        for item in moderation_items
    ) or "<tr><td colspan='3'>No moderation items</td></tr>"

    html = (
        "<html><body style='font-family: sans-serif; padding: 20px;'>"
        "<h1>Admin Dashboard</h1>"
        "<h2>System Health</h2>"
        "<ul>"
        "<li>API status: healthy</li>"
        f"<li>MCP healthy: {healthy_mcp}/{len(mcp_servers)}</li>"
        f"<li>Provisioning success rate: {prov_success_rate:.2f}%</li>"
        "</ul>"
        "<h2>User Management</h2>"
        "<table border='1' cellspacing='0' cellpadding='6'>"
        "<thead><tr><th>User ID</th><th>Contact</th><th>Status</th></tr></thead>"
        f"<tbody>{users_rows_html}</tbody></table>"
        "<h2>Moderation Queue</h2>"
        "<table border='1' cellspacing='0' cellpadding='6'>"
        "<thead><tr><th>Item ID</th><th>Status</th><th>Risk</th></tr></thead>"
        f"<tbody>{moderation_rows_html}</tbody></table>"
        "<h2>Financial Dashboard</h2>"
        "<ul>"
        f"<li>Revenue (latest day): ${revenue_cents / 100.0:.2f}</li>"
        f"<li>Message volume (latest day): {message_volume}</li>"
        f"<li>Tool calls (latest day): {tool_calls}</li>"
        "</ul>"
        "<h2>Eval & Quality</h2>"
        "<ul>"
        f"<li>Average overall score: {eval_avg_overall:.3f}</li>"
        f"<li>Average safety score: {eval_avg_safety:.3f}</li>"
        f"<li>Provisioning successful: {int(prov_totals.get('success') or 0)}</li>"
        f"<li>Provisioning failed: {int(prov_totals.get('failed') or 0)}</li>"
        "</ul>"
        "<h2>Analytics Panel</h2>"
        "<ul>"
        f"<li>DAU: {dau}</li>"
        f"<li>MAU: {mau}</li>"
        f"<li>Feature adoption rows: {len(analytics_rows)}</li>"
        "</ul>"
        "</body></html>"
    )
    return HTMLResponse(content=html)
