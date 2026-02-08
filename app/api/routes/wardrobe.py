# backend/app/api/routes/wardrobe.py

from __future__ import annotations

from typing import Optional

from fastapi import APIRouter, Depends, File, Form, HTTPException, Request, UploadFile
from sqlalchemy.orm import Session

from app.api.deps import get_db, get_or_create_user
from app.middleware.rate_limiter import rate_limit_user
from app.schemas.wardrobe import (
    WardrobeItemCreate,
    WardrobeItemUpdate,
    WardrobePhotoAttach,
    WardrobeWearLogCreate,
)
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
from app.services.wardrobe_wear_service import log_wear_event, list_wear_events, get_wear_stats
from app.services.wardrobe_rotation import run_rotation_for_user
from app.services.wardrobe_intelligence import (
    build_context,
    suggest_outfits,
    shopping_recommendations,
    DiscoverNotConfiguredError,
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


@rate_limit_user()
@router.post("/items/{item_id}/wear")
def log_item_wear(
    request: Request,
    item_id: int,
    payload: WardrobeWearLogCreate,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, payload.user_id)
    try:
        result = log_wear_event(
            db=db,
            user_id=payload.user_id,
            item_id=item_id,
            worn_at=payload.worn_at,
            source=payload.source or "manual",
            notes=payload.notes,
        )
    except ValueError as exc:
        raise HTTPException(status_code=404, detail=str(exc))
    return {"ok": True, **result}


@rate_limit_user()
@router.get("/items/{item_id}/wear")
def list_item_wear(
    request: Request,
    item_id: int,
    user_id: str,
    limit: int = 50,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    events = list_wear_events(db=db, user_id=user_id, item_id=item_id, limit=limit)
    return {"ok": True, "events": events}


@rate_limit_user()
@router.get("/stats")
def wardrobe_stats(
    request: Request,
    user_id: str,
    lookback_days: int = 90,
    limit: int = 10,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    stats = get_wear_stats(db=db, user_id=user_id, lookback_days=lookback_days, limit=limit)
    return {"ok": True, "stats": stats}


@rate_limit_user()
@router.get("/rotation")
def wardrobe_rotation(
    request: Request,
    user_id: str,
    min_days_since_worn: int | None = None,
    limit: int | None = None,
    cooldown_days: int | None = None,
    notify: bool = False,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    result = run_rotation_for_user(
        db=db,
        user_id=user_id,
        min_days_since_worn=min_days_since_worn,
        limit=limit,
        cooldown_days=cooldown_days,
        notify=notify,
    )
    return {"ok": True, "rotation": result}


@rate_limit_user()
@router.get("/suggestions")
def wardrobe_suggestions(
    request: Request,
    user_id: str,
    date: Optional[str] = None,
    location: Optional[str] = None,
    calendar_provider: Optional[str] = None,
    max_suggestions: int = 3,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    context = build_context(
        db=db,
        user_id=user_id,
        date_str=date,
        location=location,
        calendar_provider=calendar_provider,
    )
    suggestions = suggest_outfits(db=db, user_id=user_id, context=context, max_suggestions=max_suggestions)
    return {
        "ok": True,
        "context": {
            "date": context.date,
            "timezone": context.timezone,
            "weather": context.weather,
            "events": context.events,
            "event_tags": context.event_tags,
        },
        "suggestions": suggestions,
    }


@rate_limit_user()
@router.get("/recommendations")
async def wardrobe_recommendations(
    request: Request,
    user_id: str,
    date: Optional[str] = None,
    location: Optional[str] = None,
    calendar_provider: Optional[str] = None,
    max_results: int | None = None,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    context = build_context(
        db=db,
        user_id=user_id,
        date_str=date,
        location=location,
        calendar_provider=calendar_provider,
    )
    try:
        results = await shopping_recommendations(
            db=db,
            user_id=user_id,
            context=context,
            max_results=max_results,
        )
    except DiscoverNotConfiguredError as exc:
        raise HTTPException(status_code=501, detail=str(exc))
    except Exception as exc:
        raise HTTPException(status_code=502, detail=str(exc))
    return {
        "ok": True,
        "context": {
            "date": context.date,
            "timezone": context.timezone,
            "weather": context.weather,
            "event_tags": context.event_tags,
        },
        "recommendations": results,
    }
