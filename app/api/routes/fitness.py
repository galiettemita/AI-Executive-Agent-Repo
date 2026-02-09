from __future__ import annotations

from datetime import datetime as dt
from typing import Optional

from fastapi import APIRouter, Depends, HTTPException, Request
from sqlalchemy.orm import Session

from app.api.deps import get_db, get_or_create_user
from app.middleware.rate_limiter import rate_limit_user
from app.schemas.fitness import (
    WorkoutCreate,
    WorkoutUpdate,
    NutritionLogCreate,
    NutritionLogUpdate,
    MealPlanCreate,
    MealPlanUpdate,
)
from app.services.fitness_service import (
    create_workout,
    list_workouts,
    update_workout,
    delete_workout,
    create_nutrition_log,
    list_nutrition_logs,
    update_nutrition_log,
    delete_nutrition_log,
    create_meal_plan,
    list_meal_plans,
    update_meal_plan,
    delete_meal_plan,
    get_workout_suggestions,
    get_meal_plan_suggestions,
    serialize_workout,
    serialize_nutrition_log,
    serialize_meal_plan,
)
from app.services.fitbit_steps import get_daily_steps


router = APIRouter(prefix="/fitness", tags=["fitness"])


@rate_limit_user()
@router.post("/workouts")
def add_workout(request: Request, payload: WorkoutCreate, db: Session = Depends(get_db)):
    get_or_create_user(db, payload.user_id)
    row = create_workout(db, **payload.model_dump())
    return {"ok": True, "workout": serialize_workout(row)}


@rate_limit_user()
@router.get("/workouts")
def list_workouts_endpoint(request: Request, user_id: str, limit: int = 50, db: Session = Depends(get_db)):
    get_or_create_user(db, user_id)
    rows = list_workouts(db, user_id, limit=limit)
    return {"ok": True, "workouts": [serialize_workout(r) for r in rows]}


@rate_limit_user()
@router.patch("/workouts/{workout_id}")
def update_workout_endpoint(
    request: Request,
    workout_id: int,
    payload: WorkoutUpdate,
    db: Session = Depends(get_db),
):
    row = update_workout(db, payload.user_id, workout_id, **payload.model_dump(exclude={"user_id"}))
    if not row:
        raise HTTPException(status_code=404, detail="Workout not found")
    return {"ok": True, "workout": serialize_workout(row)}


@rate_limit_user()
@router.delete("/workouts/{workout_id}")
def delete_workout_endpoint(request: Request, workout_id: int, user_id: str, db: Session = Depends(get_db)):
    ok = delete_workout(db, user_id, workout_id)
    if not ok:
        raise HTTPException(status_code=404, detail="Workout not found")
    return {"ok": True}


@rate_limit_user()
@router.post("/nutrition/logs")
def add_nutrition_log(request: Request, payload: NutritionLogCreate, db: Session = Depends(get_db)):
    get_or_create_user(db, payload.user_id)
    row = create_nutrition_log(db, **payload.model_dump())
    return {"ok": True, "log": serialize_nutrition_log(row)}


@rate_limit_user()
@router.get("/nutrition/logs")
def list_nutrition_logs_endpoint(request: Request, user_id: str, limit: int = 50, db: Session = Depends(get_db)):
    get_or_create_user(db, user_id)
    rows = list_nutrition_logs(db, user_id, limit=limit)
    return {"ok": True, "logs": [serialize_nutrition_log(r) for r in rows]}


@rate_limit_user()
@router.patch("/nutrition/logs/{log_id}")
def update_nutrition_log_endpoint(
    request: Request,
    log_id: int,
    payload: NutritionLogUpdate,
    db: Session = Depends(get_db),
):
    row = update_nutrition_log(db, payload.user_id, log_id, **payload.model_dump(exclude={"user_id"}))
    if not row:
        raise HTTPException(status_code=404, detail="Nutrition log not found")
    return {"ok": True, "log": serialize_nutrition_log(row)}


@rate_limit_user()
@router.delete("/nutrition/logs/{log_id}")
def delete_nutrition_log_endpoint(request: Request, log_id: int, user_id: str, db: Session = Depends(get_db)):
    ok = delete_nutrition_log(db, user_id, log_id)
    if not ok:
        raise HTTPException(status_code=404, detail="Nutrition log not found")
    return {"ok": True}


@rate_limit_user()
@router.post("/meal-plans")
def add_meal_plan(request: Request, payload: MealPlanCreate, db: Session = Depends(get_db)):
    get_or_create_user(db, payload.user_id)
    row = create_meal_plan(db, **payload.model_dump())
    return {"ok": True, "meal_plan": serialize_meal_plan(row)}


@rate_limit_user()
@router.get("/meal-plans")
def list_meal_plans_endpoint(request: Request, user_id: str, limit: int = 30, db: Session = Depends(get_db)):
    get_or_create_user(db, user_id)
    rows = list_meal_plans(db, user_id, limit=limit)
    return {"ok": True, "meal_plans": [serialize_meal_plan(r) for r in rows]}


@rate_limit_user()
@router.patch("/meal-plans/{plan_id}")
def update_meal_plan_endpoint(
    request: Request,
    plan_id: int,
    payload: MealPlanUpdate,
    db: Session = Depends(get_db),
):
    row = update_meal_plan(db, payload.user_id, plan_id, **payload.model_dump(exclude={"user_id"}))
    if not row:
        raise HTTPException(status_code=404, detail="Meal plan not found")
    return {"ok": True, "meal_plan": serialize_meal_plan(row)}


@rate_limit_user()
@router.delete("/meal-plans/{plan_id}")
def delete_meal_plan_endpoint(request: Request, plan_id: int, user_id: str, db: Session = Depends(get_db)):
    ok = delete_meal_plan(db, user_id, plan_id)
    if not ok:
        raise HTTPException(status_code=404, detail="Meal plan not found")
    return {"ok": True}


@rate_limit_user()
@router.get("/suggestions/workouts")
def workout_suggestions(request: Request, user_id: str, db: Session = Depends(get_db)):
    get_or_create_user(db, user_id)
    return {"ok": True, "suggestions": get_workout_suggestions(db, user_id)}


@rate_limit_user()
@router.get("/suggestions/meals")
def meal_suggestions(request: Request, user_id: str, db: Session = Depends(get_db)):
    get_or_create_user(db, user_id)
    return {"ok": True, "suggestions": get_meal_plan_suggestions(db, user_id)}


@rate_limit_user()
@router.get("/steps")
def get_steps(
    request: Request,
    user_id: str,
    date: Optional[str] = None,
    refresh: bool = False,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    try:
        step_date = dt.fromisoformat(date).date() if date else dt.utcnow().date()
    except Exception:
        raise HTTPException(status_code=400, detail="Invalid date format. Use YYYY-MM-DD.")

    try:
        result = get_daily_steps(db, user_id, step_date, refresh=refresh)
    except RuntimeError as exc:
        raise HTTPException(status_code=400, detail=str(exc))

    return {"ok": True, **result}
