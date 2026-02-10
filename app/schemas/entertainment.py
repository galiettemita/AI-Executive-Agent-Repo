from __future__ import annotations

from datetime import datetime
from typing import Any, Dict, List, Optional

from pydantic import BaseModel


class EntertainmentItemCreate(BaseModel):
    user_id: str
    title: str
    content_type: str
    status: Optional[str] = None
    rating: Optional[float] = None
    external_url: Optional[str] = None
    source: Optional[str] = None
    tags: Optional[List[str]] = None
    metadata: Optional[Dict[str, Any]] = None
    notes: Optional[str] = None


class EntertainmentItemUpdate(BaseModel):
    user_id: str
    title: Optional[str] = None
    content_type: Optional[str] = None
    status: Optional[str] = None
    rating: Optional[float] = None
    external_url: Optional[str] = None
    source: Optional[str] = None
    tags: Optional[List[str]] = None
    metadata: Optional[Dict[str, Any]] = None
    notes: Optional[str] = None
    last_consumed_at: Optional[datetime] = None


class EntertainmentConsumptionCreate(BaseModel):
    user_id: str
    item_id: int
    event_type: Optional[str] = None
    duration_minutes: Optional[int] = None
    notes: Optional[str] = None
    occurred_at: Optional[datetime] = None
    metadata: Optional[Dict[str, Any]] = None


class EntertainmentRecommendationRequest(BaseModel):
    user_id: str
    query: str
    content_type: Optional[str] = None
    max_results: Optional[int] = 6
    save: Optional[bool] = False
