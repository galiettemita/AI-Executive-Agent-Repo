from __future__ import annotations

import io
import json
import zipfile
from datetime import datetime
from typing import Any
from urllib.parse import urlencode

from fastapi import APIRouter, Depends, HTTPException, Query
from fastapi.responses import StreamingResponse
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
from app.services.document_generation import generate_document
from app.services.docs_search import search_connected_docs
from app.services.gdpr_service import export_user_data
from app.services.preferences import update_preferences
from app.core.config import settings


router = APIRouter(prefix="/api/v1", tags=["core-v1"])


class RunApprovalRequest(BaseModel):
    approved: bool = True
    note: str | None = None


class KnowledgePutRequest(BaseModel):
    user_id: str
    content: str = Field(default="", min_length=1)
    metadata: dict[str, Any] = Field(default_factory=dict)


class DocumentGenerateRequest(BaseModel):
    user_id: str
    title: str
    markdown: str
    output_format: str = Field(default="pdf", pattern="^(pdf|docx)$")
    template: str = "default"
    metadata: dict[str, Any] = Field(default_factory=dict)


class InsightsOptInRequest(BaseModel):
    user_id: str
    opt_in: bool = True
    source: str | None = None


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


def _build_export_zip_payload(*, user_id: str, payload: dict[str, Any]) -> bytes:
    bio = io.BytesIO()
    with zipfile.ZipFile(bio, mode="w", compression=zipfile.ZIP_DEFLATED) as zf:
        zf.writestr(
            "manifest.json",
            json.dumps({"user_id": user_id, "generated_at": datetime.utcnow().isoformat()}, ensure_ascii=False, indent=2),
        )
        zf.writestr("export.json", json.dumps(payload, ensure_ascii=False, indent=2))
        rows = payload.get("data") if isinstance(payload, dict) else {}
        if isinstance(rows, dict):
            for table_name, table_rows in rows.items():
                if not isinstance(table_name, str):
                    continue
                zf.writestr(f"tables/{table_name}.json", json.dumps(table_rows, ensure_ascii=False, indent=2))
    bio.seek(0)
    return bio.getvalue()


def _build_slack_auth_url(*, user_id: str) -> str:
    client_id = str(settings.SLACK_CLIENT_ID or "").strip()
    redirect_uri = str(settings.SLACK_REDIRECT_URI or "").strip()
    if not client_id or not redirect_uri:
        raise RuntimeError("Slack OAuth is not configured")
    params = urlencode(
        {
            "client_id": client_id,
            "scope": "channels:history,chat:write,groups:history,im:history,users:read",
            "redirect_uri": redirect_uri,
            "state": user_id,
        }
    )
    return f"https://slack.com/oauth/v2/authorize?{params}"


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
        "slack": {
            "connected": bool(settings.SLACK_BOT_TOKEN),
            "configured": bool(settings.SLACK_CLIENT_ID and settings.SLACK_REDIRECT_URI),
        },
        "plaid": {
            "connected": False,
            "configured": bool(settings.PLAID_CLIENT_ID and settings.PLAID_SECRET_STAGING),
            "env": settings.PLAID_ENV_STAGING,
        },
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
    if p == "slack":
        try:
            return {"ok": True, "provider": p, "auth_url": _build_slack_auth_url(user_id=user_id)}
        except Exception as exc:
            raise HTTPException(status_code=400, detail=f"Slack auth URL failed: {exc}")
    if p == "plaid":
        # Plaid Link token minting is handled by /api/v1/research + workflow actions for now.
        return {
            "ok": True,
            "provider": p,
            "auth_url": f"{settings.APP_BASE_URL.rstrip('/')}/api/v1/plaid/link/start?user_id={user_id}",
            "mode": settings.PLAID_ENV_STAGING,
        }
    raise HTTPException(status_code=404, detail=f"Connector '{provider}' is not supported")


@router.get("/connectors/docs/search")
def connector_docs_search(
    user_id: str = Query(...),
    query: str = Query(..., min_length=1),
    limit: int = Query(default=8, ge=1, le=50),
    db: Session = Depends(get_db),
):
    try:
        result = search_connected_docs(
            db,
            user_id=user_id,
            query=query,
            max_results=limit,
        )
        return {"ok": True, **result}
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc))
    except Exception as exc:
        raise HTTPException(status_code=500, detail=f"docs search failed: {exc}")


@router.post("/documents/generate")
def document_generate(payload: DocumentGenerateRequest):
    if not settings.FEATURE_DOCUMENT_GENERATION:
        raise HTTPException(status_code=403, detail="Document generation feature is disabled")
    try:
        result = generate_document(
            user_id=payload.user_id,
            title=payload.title,
            markdown=payload.markdown,
            output_format=payload.output_format,
            template=payload.template,
            metadata=payload.metadata,
        )
        return {"ok": True, "document": result}
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc))
    except Exception as exc:
        raise HTTPException(status_code=500, detail=f"document generation failed: {exc}")


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


@router.get("/export")
def export_data(
    user_id: str = Query(...),
    format: str = Query(default="zip"),
    db: Session = Depends(get_db),
):
    payload = export_user_data(db=db, user_id=user_id)
    fmt = str(format or "zip").strip().lower()
    if fmt == "json":
        return {"ok": True, **payload}
    if fmt != "zip":
        raise HTTPException(status_code=400, detail="format must be one of: zip, json")
    zip_bytes = _build_export_zip_payload(user_id=user_id, payload=payload)
    headers = {
        "Content-Disposition": f'attachment; filename="user-export-{user_id}.zip"',
        "Cache-Control": "no-store",
    }
    return StreamingResponse(
        io.BytesIO(zip_bytes),
        media_type="application/zip",
        headers=headers,
    )


@router.post("/insights/opt-in")
def insights_opt_in(payload: InsightsOptInRequest, db: Session = Depends(get_db)):
    if not _table_exists(db, "preferences"):
        raise HTTPException(status_code=404, detail="preferences table not found")
    patch: dict[str, Any] = {
        "share_anonymized_insights": bool(payload.opt_in),
        "share_anonymized_insights_at": datetime.utcnow().isoformat(),
    }
    if payload.source:
        patch["share_anonymized_insights_source"] = payload.source
    updated = update_preferences(db, payload.user_id, patch)
    return {"ok": True, "user_id": payload.user_id, "opt_in": bool(payload.opt_in), "preferences": updated}
