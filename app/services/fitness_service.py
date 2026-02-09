from __future__ import annotations

import json
from datetime import datetime, timedelta
from typing import Any, Dict, List, Optional

from sqlalchemy.orm import Session

from app.core.config import settings
from app.db.models import FitnessWorkout, FitnessMealPlan, NutritionLog


def _dump_json(value: Optional[Any]) -> str:
    if value is None:
        return "{}"
    return json.dumps(value, ensure_ascii=False)


def _load_json(value: Optional[str], default: Any) -> Any:
    if not value:
        return default
    try:
        data = json.loads(value)
        return data
    except Exception:
        return default


def serialize_workout(row: FitnessWorkout) -> Dict[str, Any]:
    return {
        "id": row.id,
        "user_id": row.user_id,
        "workout_type": row.workout_type,
        "duration_minutes": row.duration_minutes,
        "calories_burned": row.calories_burned,
        "intensity": row.intensity,
        "notes": row.notes,
        "occurred_at": row.occurred_at.isoformat() if row.occurred_at else None,
        "created_at": row.created_at.isoformat() if row.created_at else None,
        "updated_at": row.updated_at.isoformat() if row.updated_at else None,
    }


def serialize_meal_plan(row: FitnessMealPlan) -> Dict[str, Any]:
    return {
        "id": row.id,
        "user_id": row.user_id,
        "plan_date": row.plan_date.isoformat() if row.plan_date else None,
        "meals": _load_json(row.meals_json, []),
        "calorie_target": row.calorie_target,
        "macros": _load_json(row.macros_json, {}),
        "notes": row.notes,
        "created_at": row.created_at.isoformat() if row.created_at else None,
        "updated_at": row.updated_at.isoformat() if row.updated_at else None,
    }


def serialize_nutrition_log(row: NutritionLog) -> Dict[str, Any]:
    return {
        "id": row.id,
        "user_id": row.user_id,
        "meal_type": row.meal_type,
        "calories": row.calories,
        "protein_g": row.protein_g,
        "carbs_g": row.carbs_g,
        "fat_g": row.fat_g,
        "notes": row.notes,
        "occurred_at": row.occurred_at.isoformat() if row.occurred_at else None,
        "created_at": row.created_at.isoformat() if row.created_at else None,
        "updated_at": row.updated_at.isoformat() if row.updated_at else None,
    }


def create_workout(
    db: Session,
    user_id: str,
    workout_type: str,
    duration_minutes: Optional[int] = None,
    calories_burned: Optional[float] = None,
    intensity: Optional[str] = None,
    notes: Optional[str] = None,
    occurred_at: Optional[datetime] = None,
) -> FitnessWorkout:
    row = FitnessWorkout(
        user_id=user_id,
        workout_type=workout_type,
        duration_minutes=duration_minutes,
        calories_burned=calories_burned,
        intensity=intensity,
        notes=notes,
        occurred_at=occurred_at or datetime.utcnow(),
        created_at=datetime.utcnow(),
        updated_at=datetime.utcnow(),
    )
    db.add(row)
    db.commit()
    db.refresh(row)
    return row


def list_workouts(db: Session, user_id: str, limit: int = 50) -> List[FitnessWorkout]:
    return (
        db.query(FitnessWorkout)
        .filter(FitnessWorkout.user_id == user_id)
        .order_by(FitnessWorkout.occurred_at.desc())
        .limit(limit)
        .all()
    )


def update_workout(db: Session, user_id: str, workout_id: int, **fields: Any) -> Optional[FitnessWorkout]:
    row = (
        db.query(FitnessWorkout)
        .filter(FitnessWorkout.user_id == user_id, FitnessWorkout.id == workout_id)
        .one_or_none()
    )
    if not row:
        return None

    for key, value in fields.items():
        if value is None:
            continue
        if hasattr(row, key):
            setattr(row, key, value)

    row.updated_at = datetime.utcnow()
    db.commit()
    db.refresh(row)
    return row


def delete_workout(db: Session, user_id: str, workout_id: int) -> bool:
    row = (
        db.query(FitnessWorkout)
        .filter(FitnessWorkout.user_id == user_id, FitnessWorkout.id == workout_id)
        .one_or_none()
    )
    if not row:
        return False
    db.delete(row)
    db.commit()
    return True


def create_meal_plan(
    db: Session,
    user_id: str,
    plan_date: datetime,
    meals: Optional[List[Dict[str, Any]]] = None,
    calorie_target: Optional[int] = None,
    macros: Optional[Dict[str, Any]] = None,
    notes: Optional[str] = None,
) -> FitnessMealPlan:
    row = FitnessMealPlan(
        user_id=user_id,
        plan_date=plan_date,
        meals_json=_dump_json(meals or []),
        calorie_target=calorie_target,
        macros_json=_dump_json(macros or {}),
        notes=notes,
        created_at=datetime.utcnow(),
        updated_at=datetime.utcnow(),
    )
    db.add(row)
    db.commit()
    db.refresh(row)
    return row


def list_meal_plans(
    db: Session,
    user_id: str,
    limit: int = 30,
) -> List[FitnessMealPlan]:
    return (
        db.query(FitnessMealPlan)
        .filter(FitnessMealPlan.user_id == user_id)
        .order_by(FitnessMealPlan.plan_date.desc())
        .limit(limit)
        .all()
    )


def update_meal_plan(db: Session, user_id: str, plan_id: int, **fields: Any) -> Optional[FitnessMealPlan]:
    row = (
        db.query(FitnessMealPlan)
        .filter(FitnessMealPlan.user_id == user_id, FitnessMealPlan.id == plan_id)
        .one_or_none()
    )
    if not row:
        return None

    for key, value in fields.items():
        if value is None:
            continue
        if key == "meals":
            row.meals_json = _dump_json(value)
            continue
        if key == "macros":
            row.macros_json = _dump_json(value)
            continue
        if hasattr(row, key):
            setattr(row, key, value)

    row.updated_at = datetime.utcnow()
    db.commit()
    db.refresh(row)
    return row


def delete_meal_plan(db: Session, user_id: str, plan_id: int) -> bool:
    row = (
        db.query(FitnessMealPlan)
        .filter(FitnessMealPlan.user_id == user_id, FitnessMealPlan.id == plan_id)
        .one_or_none()
    )
    if not row:
        return False
    db.delete(row)
    db.commit()
    return True


def create_nutrition_log(
    db: Session,
    user_id: str,
    meal_type: Optional[str] = None,
    calories: Optional[int] = None,
    protein_g: Optional[float] = None,
    carbs_g: Optional[float] = None,
    fat_g: Optional[float] = None,
    notes: Optional[str] = None,
    occurred_at: Optional[datetime] = None,
) -> NutritionLog:
    row = NutritionLog(
        user_id=user_id,
        meal_type=meal_type,
        calories=calories,
        protein_g=protein_g,
        carbs_g=carbs_g,
        fat_g=fat_g,
        notes=notes,
        occurred_at=occurred_at or datetime.utcnow(),
        created_at=datetime.utcnow(),
        updated_at=datetime.utcnow(),
    )
    db.add(row)
    db.commit()
    db.refresh(row)
    return row


def list_nutrition_logs(db: Session, user_id: str, limit: int = 50) -> List[NutritionLog]:
    return (
        db.query(NutritionLog)
        .filter(NutritionLog.user_id == user_id)
        .order_by(NutritionLog.occurred_at.desc())
        .limit(limit)
        .all()
    )


def update_nutrition_log(db: Session, user_id: str, log_id: int, **fields: Any) -> Optional[NutritionLog]:
    row = (
        db.query(NutritionLog)
        .filter(NutritionLog.user_id == user_id, NutritionLog.id == log_id)
        .one_or_none()
    )
    if not row:
        return None

    for key, value in fields.items():
        if value is None:
            continue
        if hasattr(row, key):
            setattr(row, key, value)

    row.updated_at = datetime.utcnow()
    db.commit()
    db.refresh(row)
    return row


def delete_nutrition_log(db: Session, user_id: str, log_id: int) -> bool:
    row = (
        db.query(NutritionLog)
        .filter(NutritionLog.user_id == user_id, NutritionLog.id == log_id)
        .one_or_none()
    )
    if not row:
        return False
    db.delete(row)
    db.commit()
    return True


def get_workout_suggestions(db: Session, user_id: str) -> Dict[str, Any]:
    recent = (
        db.query(FitnessWorkout)
        .filter(FitnessWorkout.user_id == user_id)
        .order_by(FitnessWorkout.occurred_at.desc())
        .limit(5)
        .all()
    )
    last_type = (recent[0].workout_type.lower() if recent else "")

    if "strength" in last_type:
        suggestions = [
            {"type": "cardio", "duration_minutes": 30, "intensity": "moderate"},
            {"type": "mobility", "duration_minutes": 20, "intensity": "light"},
        ]
    elif "cardio" in last_type or "run" in last_type or "cycle" in last_type:
        suggestions = [
            {"type": "strength", "duration_minutes": 40, "intensity": "moderate"},
            {"type": "core", "duration_minutes": 20, "intensity": "light"},
        ]
    else:
        suggestions = [
            {"type": "strength", "duration_minutes": 35, "intensity": "moderate"},
            {"type": "cardio", "duration_minutes": 25, "intensity": "moderate"},
        ]

    return {
        "last_workout_type": recent[0].workout_type if recent else None,
        "recommendations": suggestions,
    }


def _average_calories(logs: List[NutritionLog]) -> Optional[int]:
    values = [log.calories for log in logs if log.calories is not None]
    if not values:
        return None
    return int(sum(values) / len(values))


def get_meal_plan_suggestions(db: Session, user_id: str) -> Dict[str, Any]:
    since = datetime.utcnow() - timedelta(days=7)
    recent_logs = (
        db.query(NutritionLog)
        .filter(NutritionLog.user_id == user_id, NutritionLog.occurred_at >= since)
        .all()
    )
    avg_calories = _average_calories(recent_logs)
    target = avg_calories or settings.FITNESS_DEFAULT_CALORIES

    macros = {
        "protein_g": int(target * settings.FITNESS_PROTEIN_RATIO / 4),
        "carbs_g": int(target * settings.FITNESS_CARBS_RATIO / 4),
        "fat_g": int(target * settings.FITNESS_FAT_RATIO / 9),
    }

    meals = [
        {"meal": "breakfast", "target_calories": int(target * 0.25)},
        {"meal": "lunch", "target_calories": int(target * 0.35)},
        {"meal": "dinner", "target_calories": int(target * 0.3)},
        {"meal": "snack", "target_calories": int(target * 0.1)},
    ]

    return {
        "target_calories": target,
        "macros": macros,
        "suggested_meals": meals,
        "based_on": "recent_logs" if avg_calories else "default_targets",
    }
