# app/api/routes/watch_refresh.py

from datetime import datetime, timedelta

from fastapi import APIRouter, Depends, HTTPException, Query
from sqlalchemy.orm import Session

from app.db.database import get_db
from app.db.models import WatchItem, WatchOffer, NotificationQueue
from app.services.price_lookup import lookup_price_google_shopping, PriceLookupError

router = APIRouter(prefix="/watch", tags=["watch"])


@router.post("/refresh")
async def refresh_prices(
    user_id: str = Query(...),
    db: Session = Depends(get_db),
):
    items = db.query(WatchItem).filter(WatchItem.user_id == user_id).all()

    updated = 0
    for it in items:
        q = it.title_hint or it.url

        prev_best_price = it.best_price
        prev_last_seen = it.last_seen_price

        try:
            best_price, currency, best_retailer, best_offer_url, best_meta, offers = await lookup_price_google_shopping(q)
        except PriceLookupError as e:
            raise HTTPException(status_code=502, detail=str(e))

        # always record we checked
        it.last_checked_at = datetime.utcnow()

        it.product_key = best_meta.get("product_key") if isinstance(best_meta, dict) else None

        if best_price is not None:
            it.last_seen_price = best_price
            it.best_price = best_price
            it.best_retailer = best_retailer
            it.best_offer_url = best_offer_url
            it.currency = currency

            if isinstance(best_meta, dict):
                it.best_title = best_meta.get("title")
                it.best_description = best_meta.get("description")
                it.best_rating = best_meta.get("rating")
                it.best_reviews_count = best_meta.get("reviews_count")
                it.best_condition = best_meta.get("condition")
                it.best_seller_type = best_meta.get("seller_type")

            updated += 1

            # ----------------------------
            # PRICE DROP DETECTION → QUEUE
            # ----------------------------

            did_drop = (prev_best_price is not None and best_price < prev_best_price)

            # ✅ Only trigger when:
            # 1) desired_price exists
            # 2) desired_price is BELOW the previous price (meaning user expects a drop)
            # 3) price crosses from above target → at/below target on this refresh
            target_hit = False
            if it.desired_price is not None:
                target = float(it.desired_price)

                if prev_best_price is not None:
                    target_hit = (target < prev_best_price) and (prev_best_price > target) and (best_price <= target)
                else:
                    # First-ever refresh: generally should not trigger
                    target_hit = (target < best_price) and (best_price <= target)

            # Avoid duplicate queued rows for same item + same new price + same event type
            def _already_queued(event_type: str) -> bool:
                existing = (
                    db.query(NotificationQueue)
                    .filter(
                        NotificationQueue.user_id == user_id,
                        NotificationQueue.watch_item_id == it.id,
                        NotificationQueue.event_type == event_type,
                        NotificationQueue.sent_at.is_(None),
                        NotificationQueue.new_price == best_price,
                    )
                    .first()
                )
                return existing is not None

            if did_drop and not _already_queued("price_drop"):
                db.add(
                    NotificationQueue(
                        user_id=user_id,
                        watch_item_id=it.id,
                        event_type="price_drop",
                        title="Price dropped",
                        message=f"{it.title_hint or 'Item'} dropped from {prev_best_price} → {best_price} {currency}",
                        deep_link_url=best_offer_url or it.url,
                        prev_price=prev_best_price,
                        new_price=best_price,
                        currency=currency,
                        is_sent=False,
                    )
                )

            if target_hit and not _already_queued("target_hit"):
                db.add(
                    NotificationQueue(
                        user_id=user_id,
                        watch_item_id=it.id,
                        event_type="target_hit",
                        title="Target price hit",
                        message=f"{it.title_hint or 'Item'} is now {best_price} {currency} (target {it.desired_price})",
                        deep_link_url=best_offer_url or it.url,
                        prev_price=prev_last_seen,
                        new_price=best_price,
                        currency=currency,
                        is_sent=False,
                    )
                )

        # -----------------------
        # OFFER HISTORY SNAPSHOT
        # -----------------------

        # Use ONE timestamp per refresh per item so "batches" are real
        batch_ts = datetime.utcnow()

        for o in (offers or []):
            db.add(
                WatchOffer(
                    user_id=user_id,
                    watch_item_id=it.id,
                    fetched_at=batch_ts,
                    price=o.get("price"),
                    currency=o.get("currency") or "USD",
                    retailer=o.get("retailer"),
                    offer_url=o.get("offer_url"),
                    product_key=o.get("product_key"),
                    title=o.get("title"),
                    description=o.get("description"),
                    rating=o.get("rating"),
                    reviews_count=o.get("reviews_count"),
                    condition=o.get("condition"),
                    seller_type=o.get("seller_type"),
                )
            )

        # keep DB small: prune old offer history (14 days)
        cutoff = datetime.utcnow() - timedelta(days=14)
        


    db.commit()
    return {"ok": True, "updated": updated}
