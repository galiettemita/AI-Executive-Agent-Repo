#stores and retrieves tracked items in the DB for users

from fastapi import APIRouter, Depends
from sqlalchemy.orm import Session
from app.api.deps import get_or_create_user
from app.db.database import get_db
from app.db.models import TrackedItem
from app.schemas.tracked import TrackProductRequest, ListTrackedResponse, TrackedItemOut

router = APIRouter()

@router.post("")
def track_product(req: TrackProductRequest, db: Session = Depends(get_db)):
    get_or_create_user(db, req.user_id)

    # Upsert by (user_id, walmart_item_id)
    item = (
        db.query(TrackedItem)
        .filter(TrackedItem.user_id == req.user_id, TrackedItem.walmart_item_id == req.walmart_item_id)
        .first()
    )

    if item is None:
        item = TrackedItem(
            user_id=req.user_id,
            walmart_item_id=req.walmart_item_id,
            target_price=req.target_price,
            zip_code=req.zip_code,
        )
        db.add(item)
    else:
        item.target_price = req.target_price
        item.zip_code = req.zip_code

    db.commit()
    return {"ok": True}

@router.get("", response_model=ListTrackedResponse)
def list_tracked(user_id: str, db: Session = Depends(get_db)):
    get_or_create_user(db, user_id)

    items = (
        db.query(TrackedItem)
        .filter(TrackedItem.user_id == user_id)
        .order_by(TrackedItem.created_at.desc())
        .all()
    )

    return ListTrackedResponse(
        items=[
            TrackedItemOut(
                walmart_item_id=i.walmart_item_id,
                last_price=i.last_price,
                target_price=i.target_price,
                zip_code=i.zip_code,
            )
            for i in items
        ]
    )
