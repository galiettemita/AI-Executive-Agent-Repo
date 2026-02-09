from __future__ import annotations

from datetime import datetime
from typing import Any, Dict, List, Optional

from pydantic import BaseModel


class WorkoutCreate(BaseModel):
    user_id: str
    workout_type: str
    duration_minutes: Optional[int] = None
    calories_burned: Optional[float] = None
    intensity: Optional[str] = None
    notes: Optional[str] = None
    occurred_at: Optional[datetime] = None


class WorkoutUpdate(BaseModel):
    user_id: str
    workout_type: Optional[str] = None
    duration_minutes: Optional[int] = None
    calories_burned: Optional[float] = None
    intensity: Optional[str] = None
    notes: Optional[str] = None
    occurred_at: Optional[datetime] = None


class NutritionLogCreate(BaseModel):
    user_id: str
    meal_type: Optional[str] = None
    calories: Optional[int] = None
    protein_g: Optional[float] = None
    carbs_g: Optional[float] = None
    fat_g: Optional[float] = None
    notes: Optional[str] = None
    occurred_at: Optional[datetime] = None


class NutritionLogUpdate(BaseModel):
    user_id: str
    meal_type: Optional[str] = None
    calories: Optional[int] = None
    protein_g: Optional[float] = None
    carbs_g: Optional[float] = None
    fat_g: Optional[float] = None
    notes: Optional[str] = None
    occurred_at: Optional[datetime] = None


class MealPlanCreate(BaseModel):
    user_id: str
    plan_date: datetime
    meals: Optional[List[Dict[str, Any]]] = None
    calorie_target: Optional[int] = None
    macros: Optional[Dict[str, Any]] = None
    notes: Optional[str] = None


class MealPlanUpdate(BaseModel):
    user_id: str
    plan_date: Optional[datetime] = None
    meals: Optional[List[Dict[str, Any]]] = None
    calorie_target: Optional[int] = None
    macros: Optional[Dict[str, Any]] = None
    notes: Optional[str] = None
