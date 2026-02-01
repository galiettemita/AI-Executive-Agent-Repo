from fastapi import APIRouter, Depends
from sqlalchemy.orm import Session

from app.db.database import get_db
from app.db.models import WatchItem
from app.schemas.watch import WatchCreateRequest, WatchListResponse, WatchItemOut
from app.api.deps import get_or_create_user

from fastapi import HTTPException
from pydantic import BaseModel

router = APIRouter()
def _is_google_link(url: str) -> bool:
    u = (url or "").lower()
    return "google.com" in u or "googleusercontent.com" in u

def _direct_buy_link(url: str):
    if not url or _is_google_link(url):
        return None
    return url


@router.post("")
def add_watch(req: WatchCreateRequest, db: Session = Depends(get_db)):
    get_or_create_user(db, req.user_id)

    existing = (
        db.query(WatchItem)
        .filter(WatchItem.user_id == req.user_id, WatchItem.url == req.url)
        .first()
    )

    if existing is None:
        db.add(
            WatchItem(
                user_id=req.user_id,
                url=req.url,
                title_hint=req.title_hint,

                # ✅ new fields
                upc=req.upc,
                product_key=req.product_key,

                desired_price=req.desired_price,
                currency=req.currency,
            )
        )
    else:
        existing.title_hint = req.title_hint

        # ✅ new fields
        existing.upc = req.upc
        existing.product_key = req.product_key

        existing.desired_price = req.desired_price
        existing.currency = req.currency

    db.commit()
    return {"ok": True}


@router.get("", response_model=WatchListResponse)
def list_watch(user_id: str, db: Session = Depends(get_db)):
    get_or_create_user(db, user_id)

    items = (
        db.query(WatchItem)
        .filter(WatchItem.user_id == user_id)
        .order_by(WatchItem.id.desc())
        .all()
    )

    return WatchListResponse(
        items=[
            WatchItemOut(
                url=i.url,
                title_hint=i.title_hint,

                # ✅ new fields
                upc=getattr(i, "upc", None),
                product_key=getattr(i, "product_key", None),

                desired_price=i.desired_price,
                currency=i.currency,
                last_seen_price=i.last_seen_price,

                best_price=getattr(i, "best_price", None),
                best_retailer=getattr(i, "best_retailer", None),
                best_offer_url=_direct_buy_link(getattr(i, "best_offer_url", None)),
                last_checked_at=getattr(i, "last_checked_at", None),

                # ✅ best product details
                best_title=getattr(i, "best_title", None),
                best_description=getattr(i, "best_description", None),
                best_rating=getattr(i, "best_rating", None),
                best_reviews_count=getattr(i, "best_reviews_count", None),
                best_condition=getattr(i, "best_condition", None),
                best_seller_type=getattr(i, "best_seller_type", None),
            )
            for i in items
        ]
    )

class TrackFromChatRequest(BaseModel):
    user_id: str
    url: str
    title_hint: str | None = None
    desired_price: float | None = None
    currency: str = "USD"
    product_key: str | None = None
    upc: str | None = None

@router.post("/from-chat")
def add_watch_from_chat(req: TrackFromChatRequest, db: Session = Depends(get_db)):
    get_or_create_user(db, req.user_id)

    if not req.url:
        raise HTTPException(status_code=400, detail="url is required")

    existing = (
        db.query(WatchItem)
        .filter(WatchItem.user_id == req.user_id, WatchItem.url == req.url)
        .first()
    )

    if existing is None:
        db.add(
            WatchItem(
                user_id=req.user_id,
                url=req.url,
                title_hint=req.title_hint,
                upc=req.upc,
                product_key=req.product_key,
                desired_price=req.desired_price,
                currency=req.currency,
            )
        )
    else:
        # update fields if provided
        if req.title_hint is not None:
            existing.title_hint = req.title_hint
        if req.upc is not None:
            existing.upc = req.upc
        if req.product_key is not None:
            existing.product_key = req.product_key
        if req.desired_price is not None:
            existing.desired_price = req.desired_price
        existing.currency = req.currency

    db.commit()
    return {"ok": True}
