# backend/app/api/routes/photos.py

from __future__ import annotations

from typing import Optional

from fastapi import APIRouter, Depends, File, Form, HTTPException, UploadFile
from sqlalchemy.orm import Session

from app.api.deps import get_db, get_or_create_user
from app.services.assets_service import (
    save_photo_asset,
    list_photo_assets,
    search_photo_assets,
    get_photo_asset,
    get_asset_url,
)

router = APIRouter(prefix="/photos", tags=["photos"])


@router.post("/upload")
async def upload_photo(
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
    asset = save_photo_asset(
        db=db,
        user_id=user_id,
        filename=file.filename or "photo.jpg",
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
def list_photos(
    user_id: str,
    limit: int = 50,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    assets = list_photo_assets(db, user_id, limit=limit)
    return {
        "ok": True,
        "photos": [
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
def search_photos(
    user_id: str,
    q: str,
    limit: int = 20,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    assets = search_photo_assets(db, user_id, q, limit=limit)
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
def get_photo_url(
    asset_id: int,
    user_id: str,
    expires_seconds: int = 3600,
    db: Session = Depends(get_db),
):
    asset = get_photo_asset(db, user_id, asset_id)
    if not asset:
        raise HTTPException(status_code=404, detail="Photo not found")
    return {"ok": True, "url": get_asset_url(asset.storage_key, expires_seconds=expires_seconds)}
