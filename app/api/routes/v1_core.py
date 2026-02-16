from __future__ import annotations

from datetime import datetime
from typing import Any

from fastapi import APIRouter, Depends, HTTPException, Query
from pydantic import BaseModel, Field
from sqlalchemy import text
from sqlalchemy.orm import Session

from app.api.deps import get_db
from app.blueprint.knowledge_files import (
    get_latest_knowledge_file,
    knowledge_completeness,
    list_knowledge_files,
    put_knowledge_file_version,
)
from app.blueprint.preferences_learning import record_feedback_signal
from app.services.google_oauth import build_google_auth_url, get_google_connection_status
from app.services.microsoft_oauth import build_microsoft_auth_url, get_microsoft_connection_status
from app.core.config import settings


router = APIRouter(prefix="/api/v1", tags=["core-v1"])


class RunApprovalRequest(BaseModel):
    approved: bool = True
    note: str | None = None


class KnowledgePutRequest(BaseModel):
    user_id: str
    content: str = Field(default="", min_length=1)
    metadata: dict[str, Any] = Field(default_factory=dict)


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


def _serialize_row(row: dict[str, Any]) -> dict[str, Any]:
    out: dict[str, Any] = {}
    for k, v in (row or {}).items():
        if isinstance(v, datetime):
            out[k] = v.isoformat()
        else:
            out[k] = v
    return out


@router.get("/runs/{run_id}")
def get_run(run_id: str, user_id: str | None = Query(default=None), db: Session = Depends(get_db)):
    if not _table_exists(db, "runs"):
        raise HTTPException(status_code=404, detail="runs table not found")

    if user_id:
        row = db.execute(
            text("select * from runs where id = :run_id and user_id = :user_id limit 1"),
            {"run_id": run_id, "user_id": user_id},
        ).mappings().first()
    else:
        row = db.execute(
            text("select * from runs where id = :run_id limit 1"),
            {"run_id": run_id},
        ).mappings().first()

    if not row:
        raise HTTPException(status_code=404, detail="Run not found")
    return {"ok": True, "run": _serialize_row(dict(row))}


@router.post("/runs/{run_id}/approve")
def approve_run(
    run_id: str,
    payload: RunApprovalRequest,
    user_id: str | None = Query(default=None),
    db: Session = Depends(get_db),
):
    if not _table_exists(db, "runs"):
        raise HTTPException(status_code=404, detail="runs table not found")
    if not payload.approved:
        new_state = "cancelled"
    else:
        # Continue execution after approval.
        new_state = "executing"

    params: dict[str, Any] = {"run_id": run_id, "new_state": new_state}
    where = "id = :run_id"
    if user_id:
        where += " and user_id = :user_id"
        params["user_id"] = user_id

    updated = db.execute(
        text(f"update runs set state = :new_state where {where}"),
        params,
    ).rowcount
    db.commit()
    if not updated:
        raise HTTPException(status_code=404, detail="Run not found")

    # Preference-learning signal path: approval/denial feeds behavioral adaptation.
    if user_id:
        try:
            record_feedback_signal(
                db,
                user_id=user_id,
                signal_type="approved" if payload.approved else "override",
                original_output="run_approval",
                corrected_output=payload.note or "",
                context={
                    "run_id": run_id,
                    "approved": payload.approved,
                    "state": new_state,
                    "source": "api_v1_runs_approve",
                    "category": "approval",
                },
            )
        except Exception:
            db.rollback()

    return {"ok": True, "run_id": run_id, "approved": payload.approved, "state": new_state, "note": payload.note}


@router.get("/connectors")
def list_connectors(user_id: str = Query(...), db: Session = Depends(get_db)):
    connectors: dict[str, Any] = {
        "google": {"connected": False},
        "microsoft": {"connected": False},
        "tavily": {"connected": bool(settings.TAVILY_API_KEY)},
        "whatsapp": {"connected": bool(settings.WHATSAPP_TOKEN and settings.WHATSAPP_PHONE_NUMBER_ID)},
        "clerk": {"connected": bool(settings.CLERK_SECRET_KEY)},
    }

    try:
        connectors["google"] = get_google_connection_status(db=db, user_id=user_id)
    except Exception:
        connectors["google"] = {"connected": False}

    try:
        connectors["microsoft"] = get_microsoft_connection_status(db=db, user_id=user_id)
    except Exception:
        connectors["microsoft"] = {"connected": False}

    return {"ok": True, "connectors": connectors}


@router.get("/connectors/{provider}/auth")
def connector_auth(provider: str, user_id: str = Query(...), db: Session = Depends(get_db)):
    p = provider.strip().lower()
    if p == "google":
        try:
            return {"ok": True, "provider": p, "auth_url": build_google_auth_url(user_id=user_id)}
        except Exception as exc:
            raise HTTPException(status_code=400, detail=f"Google auth URL failed: {exc}")
    if p == "microsoft":
        try:
            return {"ok": True, "provider": p, "auth_url": build_microsoft_auth_url(user_id=user_id)}
        except Exception as exc:
            raise HTTPException(status_code=400, detail=f"Microsoft auth URL failed: {exc}")
    raise HTTPException(status_code=404, detail=f"Connector '{provider}' is not supported")


@router.get("/knowledge/{file_path:path}")
def knowledge_get(file_path: str, user_id: str = Query(...), db: Session = Depends(get_db)):
    normalized = file_path.strip().replace("\\", "/")
    item = get_latest_knowledge_file(db, user_id=user_id, file_path=normalized)
    if not item:
        raise HTTPException(status_code=404, detail="Knowledge file not found")
    return {"ok": True, "file": item}


@router.put("/knowledge/{file_path:path}")
def knowledge_put(file_path: str, payload: KnowledgePutRequest, db: Session = Depends(get_db)):
    normalized = file_path.strip().replace("\\", "/")
    try:
        item = put_knowledge_file_version(
            db,
            user_id=payload.user_id,
            file_path=normalized,
            content=payload.content,
            metadata=payload.metadata,
        )
        return {"ok": True, "file": item}
    except Exception as exc:
        raise HTTPException(status_code=500, detail=f"Knowledge file update failed: {exc}")


@router.get("/knowledge")
def knowledge_index(user_id: str = Query(...), db: Session = Depends(get_db)):
    files = list_knowledge_files(db, user_id=user_id)
    return {"ok": True, "files": files, "completeness": knowledge_completeness(files)}


@router.get("/knowledge/graph")
def knowledge_graph(user_id: str = Query(...), limit: int = Query(default=200, ge=1, le=1000), db: Session = Depends(get_db)):
    if not _table_exists(db, "knowledge_graph_edges"):
        raise HTTPException(status_code=404, detail="knowledge_graph_edges table not found")
    rows = db.execute(
        text(
            """
            select id, source_node, target_node, relation, weight, metadata, created_at
            from knowledge_graph_edges
            where user_id = :user_id
            order by created_at desc
            limit :limit
            """
        ),
        {"user_id": user_id, "limit": int(limit)},
    ).mappings().all()
    return {"ok": True, "edges": [_serialize_row(dict(r)) for r in rows]}
