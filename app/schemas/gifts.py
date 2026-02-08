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
