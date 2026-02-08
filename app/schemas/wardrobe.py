# app/schemas/wardrobe.py

from __future__ import annotations

from datetime import datetime
from typing import Any, Dict, List, Optional

from pydantic import BaseModel, Field


class WardrobeItemCreate(BaseModel):
    user_id: str
    name: str
    category: Optional[str] = None
    subcategory: Optional[str] = None
    brand: Optional[str] = None
    color: Optional[str] = None
    size: Optional[str] = None
    material: Optional[str] = None
    season: Optional[str] = None
    condition: Optional[str] = None
    purchase_date: Optional[datetime] = None
    price: Optional[float] = None
    currency: Optional[str] = None
    notes: Optional[str] = None
    tags: Optional[List[str]] = None
    metadata: Optional[Dict[str, Any]] = None
    photo_asset_ids: Optional[List[int]] = None
    primary_photo_id: Optional[int] = None


class WardrobeItemUpdate(BaseModel):
    user_id: str
    name: Optional[str] = None
    category: Optional[str] = None
    subcategory: Optional[str] = None
    brand: Optional[str] = None
    color: Optional[str] = None
    size: Optional[str] = None
    material: Optional[str] = None
    season: Optional[str] = None
    condition: Optional[str] = None
    purchase_date: Optional[datetime] = None
    price: Optional[float] = None
    currency: Optional[str] = None
    notes: Optional[str] = None
    tags: Optional[List[str]] = None
    metadata: Optional[Dict[str, Any]] = None


class WardrobePhotoAttach(BaseModel):
    user_id: str
    photo_asset_ids: List[int] = Field(default_factory=list)
    primary_photo_id: Optional[int] = None


class WardrobeWearLogCreate(BaseModel):
    user_id: str
    worn_at: Optional[datetime] = None
    source: Optional[str] = "manual"
    notes: Optional[str] = None
