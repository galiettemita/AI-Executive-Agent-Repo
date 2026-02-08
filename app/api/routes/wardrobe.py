# backend/app/api/routes/wardrobe.py

from __future__ import annotations

from typing import Optional

from fastapi import APIRouter, Depends, File, Form, HTTPException, Request, UploadFile
from sqlalchemy.orm import Session

from app.api.deps import get_db, get_or_create_user
from app.middleware.rate_limiter import rate_limit_user
from app.schemas.wardrobe import WardrobeItemCreate, WardrobeItemUpdate, WardrobePhotoAttach
from app.services.assets_service import save_photo_asset
from app.services.wardrobe_service import (
    create_wardrobe_item,
    list_wardrobe_items,
    get_wardrobe_item,
    update_wardrobe_item,
    delete_wardrobe_item,
    attach_photo_to_item,
    detach_photo_from_item,
    list_item_photos,
    serialize_item,
)

router = APIRouter(prefix="/wardrobe", tags=["wardrobe"])


@rate_limit_user()
@router.post("/items")
def create_item(request: Request, payload: WardrobeItemCreate, db: Session = Depends(get_db)):
    get_or_create_user(db, payload.user_id)
    item = create_wardrobe_item(
        db=db,
        user_id=payload.user_id,
        name=payload.name,
        category=payload.category,
        subcategory=payload.subcategory,
        brand=payload.brand,
        color=payload.color,
        size=payload.size,
        material=payload.material,
        season=payload.season,
        condition=payload.condition,
        purchase_date=payload.purchase_date,
        price=payload.price,
        currency=payload.currency,
        notes=payload.notes,
        tags=payload.tags,
        metadata=payload.metadata,
    )

    attached = []
    if payload.photo_asset_ids:
        for pid in payload.photo_asset_ids:
            ok = attach_photo_to_item(
                db=db,
                user_id=payload.user_id,
                item_id=item.id,
                photo_asset_id=pid,
                is_primary=payload.primary_photo_id == pid,
            )
            if ok:
                attached.append(pid)

    return {
        "ok": True,
        "item": serialize_item(item),
        "attached_photo_ids": attached,
    }


@rate_limit_user()
@router.get("/items")
def list_items(
    request: Request,
    user_id: str,
    limit: int = 50,
    category: Optional[str] = None,
    season: Optional[str] = None,
    brand: Optional[str] = None,
    q: Optional[str] = None,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    items = list_wardrobe_items(
        db=db,
        user_id=user_id,
        limit=limit,
        category=category,
        season=season,
        brand=brand,
        query=q,
    )
    return {"ok": True, "items": [serialize_item(item) for item in items]}


@rate_limit_user()
@router.get("/items/{item_id}")
def get_item(
    request: Request,
    item_id: int,
    user_id: str,
    db: Session = Depends(get_db),
):
    item = get_wardrobe_item(db, user_id, item_id)
    if not item:
        raise HTTPException(status_code=404, detail="Wardrobe item not found")
    return {"ok": True, "item": serialize_item(item), "photos": list_item_photos(db, user_id, item_id)}


@rate_limit_user()
@router.patch("/items/{item_id}")
def update_item(
    request: Request,
    item_id: int,
    payload: WardrobeItemUpdate,
    db: Session = Depends(get_db),
):
    item = update_wardrobe_item(
        db=db,
        user_id=payload.user_id,
        item_id=item_id,
        name=payload.name,
        category=payload.category,
        subcategory=payload.subcategory,
        brand=payload.brand,
        color=payload.color,
        size=payload.size,
        material=payload.material,
        season=payload.season,
        condition=payload.condition,
        purchase_date=payload.purchase_date,
        price=payload.price,
        currency=payload.currency,
        notes=payload.notes,
        tags=payload.tags,
        metadata=payload.metadata,
    )
    if not item:
        raise HTTPException(status_code=404, detail="Wardrobe item not found")
    return {"ok": True, "item": serialize_item(item)}


@rate_limit_user()
@router.delete("/items/{item_id}")
def delete_item(request: Request, item_id: int, user_id: str, db: Session = Depends(get_db)):
    ok = delete_wardrobe_item(db, user_id, item_id)
    if not ok:
        raise HTTPException(status_code=404, detail="Wardrobe item not found")
    return {"ok": True}


@rate_limit_user()
@router.get("/items/{item_id}/photos")
def list_item_photos_endpoint(
    request: Request,
    item_id: int,
    user_id: str,
    db: Session = Depends(get_db),
):
    return {"ok": True, "photos": list_item_photos(db, user_id, item_id)}


@rate_limit_user()
@router.post("/items/{item_id}/photos")
def attach_item_photos(
    request: Request,
    item_id: int,
    payload: WardrobePhotoAttach,
    db: Session = Depends(get_db),
):
    item = get_wardrobe_item(db, payload.user_id, item_id)
    if not item:
        raise HTTPException(status_code=404, detail="Wardrobe item not found")
    attached = []
    for pid in payload.photo_asset_ids:
        ok = attach_photo_to_item(
            db=db,
            user_id=payload.user_id,
            item_id=item_id,
            photo_asset_id=pid,
            is_primary=payload.primary_photo_id == pid,
        )
        if ok:
            attached.append(pid)
    if not attached:
        raise HTTPException(status_code=400, detail="No photos attached")
    return {"ok": True, "attached_photo_ids": attached}


@rate_limit_user()
@router.delete("/items/{item_id}/photos/{photo_id}")
def detach_item_photo(
    request: Request,
    item_id: int,
    photo_id: int,
    user_id: str,
    db: Session = Depends(get_db),
):
    ok = detach_photo_from_item(db, user_id, item_id, photo_id)
    if not ok:
        raise HTTPException(status_code=404, detail="Photo link not found")
    return {"ok": True}


@rate_limit_user()
@router.post("/items/{item_id}/photos/upload")
async def upload_item_photo(
    request: Request,
    item_id: int,
    user_id: str = Form(...),
    file: UploadFile = File(...),
    tags: Optional[str] = Form(None),
    primary: bool = Form(False),
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    if not get_wardrobe_item(db, user_id, item_id):
        raise HTTPException(status_code=404, detail="Wardrobe item not found")
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

    ok = attach_photo_to_item(
        db=db,
        user_id=user_id,
        item_id=item_id,
        photo_asset_id=asset.id,
        is_primary=primary,
    )
    if not ok:
        raise HTTPException(status_code=404, detail="Wardrobe item not found")

    return {
        "ok": True,
        "photo_id": asset.id,
        "item_id": item_id,
    }
