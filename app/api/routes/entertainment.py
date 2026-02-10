from __future__ import annotations

from typing import Optional

from fastapi import APIRouter, Depends, HTTPException, Request
from sqlalchemy.orm import Session

from app.api.deps import get_db, get_or_create_user
from app.middleware.rate_limiter import rate_limit_user
from app.schemas.entertainment import (
    EntertainmentItemCreate,
    EntertainmentItemUpdate,
    EntertainmentConsumptionCreate,
    EntertainmentRecommendationRequest,
)
from app.services.entertainment_service import (
    create_item,
    update_item,
    list_items,
    delete_item,
    log_consumption,
    list_consumption,
    serialize_item,
    serialize_consumption,
    get_recommendations,
)
from app.services.discover_provider import DiscoverNotConfiguredError


router = APIRouter(prefix="/entertainment", tags=["entertainment"])


@rate_limit_user()
@router.post("/items")
def add_item(request: Request, payload: EntertainmentItemCreate, db: Session = Depends(get_db)):
    get_or_create_user(db, payload.user_id)
    row = create_item(db, **payload.model_dump())
    return {"ok": True, "item": serialize_item(row)}


@rate_limit_user()
@router.get("/items")
def list_items_endpoint(
    request: Request,
    user_id: str,
    limit: int = 50,
    status: Optional[str] = None,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    rows = list_items(db, user_id, limit=limit, status=status)
    return {"ok": True, "items": [serialize_item(r) for r in rows]}


@rate_limit_user()
@router.patch("/items/{item_id}")
def update_item_endpoint(
    request: Request,
    item_id: int,
    payload: EntertainmentItemUpdate,
    db: Session = Depends(get_db),
):
    row = update_item(db, payload.user_id, item_id, **payload.model_dump(exclude={"user_id"}))
    if not row:
        raise HTTPException(status_code=404, detail="Item not found")
    return {"ok": True, "item": serialize_item(row)}


@rate_limit_user()
@router.delete("/items/{item_id}")
def delete_item_endpoint(request: Request, item_id: int, user_id: str, db: Session = Depends(get_db)):
    ok = delete_item(db, user_id, item_id)
    if not ok:
        raise HTTPException(status_code=404, detail="Item not found")
    return {"ok": True}


@rate_limit_user()
@router.post("/consumption")
def log_consumption_endpoint(
    request: Request,
    payload: EntertainmentConsumptionCreate,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, payload.user_id)
    row = log_consumption(db, **payload.model_dump())
    if not row:
        raise HTTPException(status_code=404, detail="Item not found")
    return {"ok": True, "consumption": serialize_consumption(row)}


@rate_limit_user()
@router.get("/consumption")
def list_consumption_endpoint(
    request: Request,
    user_id: str,
    item_id: Optional[int] = None,
    limit: int = 100,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    rows = list_consumption(db, user_id, item_id=item_id, limit=limit)
    return {"ok": True, "consumption": [serialize_consumption(r) for r in rows]}


@rate_limit_user()
@router.post("/recommendations")
async def recommend_content(
    request: Request,
    payload: EntertainmentRecommendationRequest,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, payload.user_id)
    try:
        results = await get_recommendations(
            db,
            user_id=payload.user_id,
            query=payload.query,
            content_type=payload.content_type,
            max_results=payload.max_results or 6,
            save=bool(payload.save),
        )
    except DiscoverNotConfiguredError as exc:
        raise HTTPException(status_code=400, detail=str(exc))
    return {"ok": True, "recommendations": results}
