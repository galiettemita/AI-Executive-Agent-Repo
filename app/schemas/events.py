from __future__ import annotations

from datetime import date, datetime
from typing import Any, Dict, Optional

from pydantic import BaseModel


class EventDiscoverRequest(BaseModel):
    user_id: str
    query: Optional[str] = None
    location: Optional[str] = None
    start_date: Optional[date] = None
    end_date: Optional[date] = None
    max_results: Optional[int] = None
    save: Optional[bool] = False


class EventCreate(BaseModel):
    user_id: str
    title: str
    event_type: Optional[str] = None
    venue: Optional[str] = None
    location: Optional[str] = None
    starts_at: Optional[datetime] = None
    ends_at: Optional[datetime] = None
    external_url: Optional[str] = None
    provider: Optional[str] = None
    provider_event_id: Optional[str] = None
    price_min: Optional[float] = None
    price_max: Optional[float] = None
    currency: Optional[str] = None
    status: Optional[str] = None
    metadata: Optional[Dict[str, Any]] = None


class EventUpdate(BaseModel):
    user_id: str
    title: Optional[str] = None
    event_type: Optional[str] = None
    venue: Optional[str] = None
    location: Optional[str] = None
    starts_at: Optional[datetime] = None
    ends_at: Optional[datetime] = None
    external_url: Optional[str] = None
    provider: Optional[str] = None
    provider_event_id: Optional[str] = None
    price_min: Optional[float] = None
    price_max: Optional[float] = None
    currency: Optional[str] = None
    status: Optional[str] = None
    metadata: Optional[Dict[str, Any]] = None


class EventBookingCreate(BaseModel):
    user_id: str
    event_id: Optional[int] = None
    quantity: Optional[int] = 1
    total_price: Optional[float] = None
    currency: Optional[str] = None
    ticket_delivery: Optional[str] = None
    notes: Optional[str] = None
    require_approval: Optional[bool] = True


class EventBookingUpdate(BaseModel):
    user_id: str
    status: Optional[str] = None
    ticket_delivery: Optional[str] = None
    notes: Optional[str] = None
    metadata: Optional[Dict[str, Any]] = None
