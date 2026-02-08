# backend/app/api/routes/files.py

from __future__ import annotations

from typing import Optional

from fastapi import APIRouter, Depends, File, Form, HTTPException, UploadFile
from sqlalchemy.orm import Session

from app.api.deps import get_db, get_or_create_user
from app.services.assets_service import (
    save_file_asset,
    list_file_assets,
    search_file_assets,
    get_file_asset,
    get_asset_url,
)
from app.services.file_semantic_search import semantic_search_files

router = APIRouter(prefix="/files", tags=["files"])


@router.post("/upload")
async def upload_file(
    user_id: str = Form(...),
    file: UploadFile = File(...),
    tags: Optional[str] = Form(None),
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    data = await file.read()
    if not data:
        raise HTTPException(status_code=400, detail="Empty upload")

    tag_list = [t.strip() for t in (tags or "").split(",") if t.strip()]
    asset = save_file_asset(
        db=db,
        user_id=user_id,
        filename=file.filename or "upload.bin",
        content_type=file.content_type,
        data=data,
        tags=tag_list or None,
    )
    return {
        "ok": True,
        "id": asset.id,
        "filename": asset.filename,
        "content_type": asset.content_type,
        "size_bytes": asset.size_bytes,
        "created_at": asset.created_at.isoformat() if asset.created_at else None,
    }


@router.get("")
def list_files(
    user_id: str,
    limit: int = 50,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    assets = list_file_assets(db, user_id, limit=limit)
    return {
        "ok": True,
        "files": [
            {
                "id": a.id,
                "filename": a.filename,
                "content_type": a.content_type,
                "size_bytes": a.size_bytes,
                "tags": a.tags_json,
                "created_at": a.created_at.isoformat() if a.created_at else None,
            }
            for a in assets
        ],
    }


@router.get("/search")
def search_files(
    user_id: str,
    q: str,
    limit: int = 20,
    semantic: bool = False,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    if semantic:
        try:
            results = semantic_search_files(db, user_id, q, top_k=limit)
        except Exception as exc:
            raise HTTPException(status_code=400, detail=str(exc))
        return {
            "ok": True,
            "results": [
                {
                    "id": item["asset"].id,
                    "filename": item["asset"].filename,
                    "content_type": item["asset"].content_type,
                    "size_bytes": item["asset"].size_bytes,
                    "tags": item["asset"].tags_json,
                    "score": item.get("score"),
                    "created_at": item["asset"].created_at.isoformat() if item["asset"].created_at else None,
                }
                for item in results
            ],
        }

    assets = search_file_assets(db, user_id, q, limit=limit)
    return {
        "ok": True,
        "results": [
            {
                "id": a.id,
                "filename": a.filename,
                "content_type": a.content_type,
                "size_bytes": a.size_bytes,
                "tags": a.tags_json,
                "created_at": a.created_at.isoformat() if a.created_at else None,
            }
            for a in assets
        ],
    }


@router.get("/{asset_id}/url")
def get_file_url(
    asset_id: int,
    user_id: str,
    expires_seconds: int = 3600,
    db: Session = Depends(get_db),
):
    asset = get_file_asset(db, user_id, asset_id)
    if not asset:
        raise HTTPException(status_code=404, detail="File not found")
    return {"ok": True, "url": get_asset_url(asset.storage_key, expires_seconds=expires_seconds)}
