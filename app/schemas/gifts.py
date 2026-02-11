# app/schemas/gifts.py

from __future__ import annotations

from datetime import date, datetime
from typing import Any, Dict, List, Optional

from pydantic import BaseModel


class GiftOccasionCreate(BaseModel):
    user_id: str
    recipient_name: str
    relationship: Optional[str] = None
    occasion_type: Optional[str] = None
    occasion_date: Optional[date] = None
    recurrence: Optional[str] = "annual"
    reminder_days_before: Optional[int] = 14
    budget: Optional[float] = None
    currency: Optional[str] = None
    preferences: Optional[Dict[str, Any]] = None
    notes: Optional[str] = None


class GiftOccasionUpdate(BaseModel):
    user_id: str
    recipient_name: Optional[str] = None
    relationship: Optional[str] = None
    occasion_type: Optional[str] = None
    occasion_date: Optional[date] = None
    recurrence: Optional[str] = None
    reminder_days_before: Optional[int] = None
    budget: Optional[float] = None
    currency: Optional[str] = None
    preferences: Optional[Dict[str, Any]] = None
    notes: Optional[str] = None


class GiftIdeaCreate(BaseModel):
    user_id: str
    occasion_id: Optional[int] = None
    title: str
    description: Optional[str] = None
    link_url: Optional[str] = None
    price: Optional[float] = None
    currency: Optional[str] = None
    status: Optional[str] = "idea"
    tags: Optional[List[str]] = None
    source: Optional[str] = None


class GiftIdeaUpdate(BaseModel):
    user_id: str
    title: Optional[str] = None
    description: Optional[str] = None
    link_url: Optional[str] = None
    price: Optional[float] = None
    currency: Optional[str] = None
    status: Optional[str] = None
    tags: Optional[List[str]] = None


class GiftRecommendationRequest(BaseModel):
    user_id: str
    occasion_id: Optional[int] = None
    query: Optional[str] = None
    max_results: Optional[int] = None


class GiftThankYouRequest(BaseModel):
    user_id: str
    occasion_id: Optional[int] = None
    gift_idea_id: Optional[int] = None
    tone: Optional[str] = "grateful"
    length: Optional[str] = "short"
    extra_notes: Optional[str] = None


class GiftPurchaseProposalRequest(BaseModel):
    user_id: str
    gift_idea_id: int
    quantity: Optional[int] = 1
    notes: Optional[str] = None


class GiftRetailerCreate(BaseModel):
    user_id: str
    domain: str
    status: Optional[str] = "allowed"
    notes: Optional[str] = None


class GiftRetailerUpdate(BaseModel):
    user_id: str
    status: Optional[str] = None
    notes: Optional[str] = None


class GiftOrderCreate(BaseModel):
    user_id: str
    gift_idea_id: Optional[int] = None
    occasion_id: Optional[int] = None
    title: Optional[str] = None
    product_url: Optional[str] = None
    retailer_domain: Optional[str] = None
    quantity: Optional[int] = 1
    unit_price: Optional[float] = None
    total_price: Optional[float] = None
    currency: Optional[str] = None
    shipping_address: Optional[Dict[str, Any]] = None
    notes: Optional[str] = None
    require_approval: Optional[bool] = True
    enforce_allowlist: Optional[bool] = True
    payment_method_id: Optional[int] = None


class GiftOrderUpdate(BaseModel):
    user_id: str
    status: Optional[str] = None
    tracking_number: Optional[str] = None
    tracking_url: Optional[str] = None
    notes: Optional[str] = None
    metadata: Optional[Dict[str, Any]] = None


class GiftOrderAuthorize(BaseModel):
    user_id: str
    payment_method_id: Optional[int] = None
    authorized_total: Optional[float] = None
    currency: Optional[str] = None


class GiftOrderEventCreate(BaseModel):
    user_id: str
    status: str
    message: Optional[str] = None
    metadata: Optional[Dict[str, Any]] = None
    occurred_at: Optional[datetime] = None


class GiftOrderRefundRequest(BaseModel):
    user_id: str
    reason: Optional[str] = None
    amount: Optional[float] = None
