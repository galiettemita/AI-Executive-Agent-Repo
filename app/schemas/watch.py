from __future__ import annotations
from pydantic import BaseModel
from typing import Optional
from datetime import datetime


class WatchCreateRequest(BaseModel):
    user_id: str
    url: str
    title_hint: Optional[str] = None

    # ✅ NEW: lets user track by UPC (optional)
    upc: Optional[str] = None

    # ✅ NEW: stable identity key (optional; backend can fill later)
    product_key: Optional[str] = None

    desired_price: Optional[float] = None
    currency: str = "USD"


class WatchItemOut(BaseModel):
    url: str
    title_hint: Optional[str] = None

    # ✅ NEW fields the AI will use
    upc: Optional[str] = None
    product_key: Optional[str] = None

    desired_price: Optional[float] = None
    currency: str = "USD"
    last_seen_price: Optional[float] = None

    # ✅ best-offer provenance fields
    best_price: Optional[float] = None
    best_retailer: Optional[str] = None
    best_offer_url: Optional[str] = None
    last_checked_at: Optional[datetime] = None

    # ✅ NEW: best product details (quality reasoning)
    best_title: Optional[str] = None
    best_description: Optional[str] = None
    best_rating: Optional[float] = None
    best_reviews_count: Optional[int] = None
    best_condition: Optional[str] = None
    best_seller_type: Optional[str] = None


class WatchListResponse(BaseModel):
    items: list[WatchItemOut]
