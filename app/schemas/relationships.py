from __future__ import annotations

from datetime import datetime
from typing import Any, Dict, List, Optional

from pydantic import BaseModel


class RelationshipProfileCreate(BaseModel):
    user_id: str
    contact_id: int
    relationship: Optional[str] = None
    priority: Optional[int] = None
    cadence_days: Optional[int] = None
    preferred_channel: Optional[str] = None
    tags: Optional[List[str]] = None
    notes: Optional[str] = None
    metadata: Optional[Dict[str, Any]] = None


class RelationshipProfileUpdate(BaseModel):
    user_id: str
    relationship: Optional[str] = None
    priority: Optional[int] = None
    cadence_days: Optional[int] = None
    preferred_channel: Optional[str] = None
    tags: Optional[List[str]] = None
    notes: Optional[str] = None
    metadata: Optional[Dict[str, Any]] = None
    next_checkin_at: Optional[datetime] = None


class RelationshipInteractionCreate(BaseModel):
    user_id: str
    contact_id: int
    direction: str = "outbound"
    channel: Optional[str] = None
    summary: Optional[str] = None
    occurred_at: Optional[datetime] = None
    metadata: Optional[Dict[str, Any]] = None
