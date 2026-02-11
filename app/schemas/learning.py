from __future__ import annotations

from datetime import date, datetime
from typing import Any, Dict, List, Optional

from pydantic import BaseModel


class LanguageGoalCreate(BaseModel):
    user_id: str
    language: str
    daily_minutes: Optional[int] = None
    weekly_sessions: Optional[int] = None
    target_level: Optional[str] = None
    active: Optional[bool] = True
    start_date: Optional[date] = None
    end_date: Optional[date] = None
    notes: Optional[str] = None


class LanguageGoalUpdate(BaseModel):
    user_id: str
    language: Optional[str] = None
    daily_minutes: Optional[int] = None
    weekly_sessions: Optional[int] = None
    target_level: Optional[str] = None
    active: Optional[bool] = None
    start_date: Optional[date] = None
    end_date: Optional[date] = None
    notes: Optional[str] = None


class LanguageSessionCreate(BaseModel):
    user_id: str
    language: str
    session_type: Optional[str] = None
    duration_minutes: Optional[int] = None
    accuracy_score: Optional[float] = None
    notes: Optional[str] = None
    metadata: Optional[Dict[str, Any]] = None
    occurred_at: Optional[datetime] = None


class LearningResourceCreate(BaseModel):
    user_id: str
    topic: Optional[str] = None
    title: str
    url: Optional[str] = None
    source: Optional[str] = None
    resource_type: Optional[str] = None
    difficulty: Optional[str] = None
    status: Optional[str] = None
    tags: Optional[List[str]] = None
    metadata: Optional[Dict[str, Any]] = None
    notes: Optional[str] = None


class LearningResourceUpdate(BaseModel):
    user_id: str
    topic: Optional[str] = None
    title: Optional[str] = None
    url: Optional[str] = None
    source: Optional[str] = None
    resource_type: Optional[str] = None
    difficulty: Optional[str] = None
    status: Optional[str] = None
    tags: Optional[List[str]] = None
    metadata: Optional[Dict[str, Any]] = None
    notes: Optional[str] = None


class LearningScheduleCreate(BaseModel):
    user_id: str
    resource_id: Optional[int] = None
    scheduled_for: Optional[datetime] = None
    duration_minutes: Optional[int] = None
    status: Optional[str] = None
    notes: Optional[str] = None


class LearningScheduleUpdate(BaseModel):
    user_id: str
    scheduled_for: Optional[datetime] = None
    duration_minutes: Optional[int] = None
    status: Optional[str] = None
    notes: Optional[str] = None


class LearningRecommendationRequest(BaseModel):
    user_id: str
    query: str
    topic: Optional[str] = None
    max_results: Optional[int] = None
    save: Optional[bool] = False
