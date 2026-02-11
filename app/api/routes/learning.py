from __future__ import annotations

from typing import Optional

from fastapi import APIRouter, Depends, HTTPException, Request
from sqlalchemy.orm import Session

from app.api.deps import get_db, get_or_create_user
from app.middleware.rate_limiter import rate_limit_user
from app.schemas.learning import (
    LanguageGoalCreate,
    LanguageGoalUpdate,
    LanguageSessionCreate,
    LearningResourceCreate,
    LearningResourceUpdate,
    LearningScheduleCreate,
    LearningScheduleUpdate,
    LearningRecommendationRequest,
)
from app.services.learning_service import (
    create_language_goal,
    update_language_goal,
    list_language_goals,
    delete_language_goal,
    serialize_language_goal,
    log_language_session,
    list_language_sessions,
    serialize_language_session,
    compute_language_progress,
    create_resource,
    update_resource,
    list_resources,
    delete_resource,
    serialize_resource,
    create_schedule,
    update_schedule,
    list_schedule,
    delete_schedule,
    serialize_schedule,
    recommend_resources,
)
from app.services.discover_provider import DiscoverNotConfiguredError


router = APIRouter(prefix="/learning", tags=["learning"])


@rate_limit_user()
@router.post("/language/goals")
def add_language_goal(request: Request, payload: LanguageGoalCreate, db: Session = Depends(get_db)):
    get_or_create_user(db, payload.user_id)
    row = create_language_goal(db, **payload.model_dump())
    return {"ok": True, "goal": serialize_language_goal(row)}


@rate_limit_user()
@router.get("/language/goals")
def list_language_goals_endpoint(
    request: Request,
    user_id: str,
    active: Optional[bool] = None,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    rows = list_language_goals(db, user_id, active=active)
    return {"ok": True, "goals": [serialize_language_goal(r) for r in rows]}


@rate_limit_user()
@router.patch("/language/goals/{goal_id}")
def update_language_goal_endpoint(
    request: Request,
    goal_id: int,
    payload: LanguageGoalUpdate,
    db: Session = Depends(get_db),
):
    row = update_language_goal(db, payload.user_id, goal_id, **payload.model_dump(exclude={"user_id"}))
    if not row:
        raise HTTPException(status_code=404, detail="Goal not found")
    return {"ok": True, "goal": serialize_language_goal(row)}


@rate_limit_user()
@router.delete("/language/goals/{goal_id}")
def delete_language_goal_endpoint(request: Request, goal_id: int, user_id: str, db: Session = Depends(get_db)):
    ok = delete_language_goal(db, user_id, goal_id)
    if not ok:
        raise HTTPException(status_code=404, detail="Goal not found")
    return {"ok": True}


@rate_limit_user()
@router.post("/language/sessions")
def log_language_session_endpoint(
    request: Request,
    payload: LanguageSessionCreate,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, payload.user_id)
    row = log_language_session(db, **payload.model_dump())
    return {"ok": True, "session": serialize_language_session(row)}


@rate_limit_user()
@router.get("/language/sessions")
def list_language_sessions_endpoint(
    request: Request,
    user_id: str,
    language: Optional[str] = None,
    limit: int = 100,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    rows = list_language_sessions(db, user_id, language=language, limit=limit)
    return {"ok": True, "sessions": [serialize_language_session(r) for r in rows]}


@rate_limit_user()
@router.get("/language/progress")
def language_progress_endpoint(
    request: Request,
    user_id: str,
    language: Optional[str] = None,
    window_days: int = 30,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    data = compute_language_progress(db, user_id, language=language, window_days=window_days)
    return {"ok": True, "progress": data}


@rate_limit_user()
@router.post("/resources")
def add_learning_resource(request: Request, payload: LearningResourceCreate, db: Session = Depends(get_db)):
    get_or_create_user(db, payload.user_id)
    row = create_resource(db, **payload.model_dump())
    return {"ok": True, "resource": serialize_resource(row)}


@rate_limit_user()
@router.get("/resources")
def list_learning_resources_endpoint(
    request: Request,
    user_id: str,
    status: Optional[str] = None,
    topic: Optional[str] = None,
    limit: int = 50,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    rows = list_resources(db, user_id, status=status, topic=topic, limit=limit)
    return {"ok": True, "resources": [serialize_resource(r) for r in rows]}


@rate_limit_user()
@router.patch("/resources/{resource_id}")
def update_learning_resource_endpoint(
    request: Request,
    resource_id: int,
    payload: LearningResourceUpdate,
    db: Session = Depends(get_db),
):
    row = update_resource(db, payload.user_id, resource_id, **payload.model_dump(exclude={"user_id"}))
    if not row:
        raise HTTPException(status_code=404, detail="Resource not found")
    return {"ok": True, "resource": serialize_resource(row)}


@rate_limit_user()
@router.delete("/resources/{resource_id}")
def delete_learning_resource_endpoint(
    request: Request,
    resource_id: int,
    user_id: str,
    db: Session = Depends(get_db),
):
    ok = delete_resource(db, user_id, resource_id)
    if not ok:
        raise HTTPException(status_code=404, detail="Resource not found")
    return {"ok": True}


@rate_limit_user()
@router.post("/resources/recommendations")
async def recommend_learning_resources(
    request: Request,
    payload: LearningRecommendationRequest,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, payload.user_id)
    try:
        results = await recommend_resources(
            db,
            user_id=payload.user_id,
            query=payload.query,
            topic=payload.topic,
            max_results=payload.max_results or 6,
            save=bool(payload.save),
        )
    except DiscoverNotConfiguredError as exc:
        raise HTTPException(status_code=400, detail=str(exc))
    return {"ok": True, "recommendations": results}


@rate_limit_user()
@router.post("/schedule")
def create_learning_schedule(request: Request, payload: LearningScheduleCreate, db: Session = Depends(get_db)):
    get_or_create_user(db, payload.user_id)
    row = create_schedule(db, **payload.model_dump())
    return {"ok": True, "schedule": serialize_schedule(row)}


@rate_limit_user()
@router.get("/schedule")
def list_learning_schedule(
    request: Request,
    user_id: str,
    status: Optional[str] = None,
    limit: int = 50,
    db: Session = Depends(get_db),
):
    get_or_create_user(db, user_id)
    rows = list_schedule(db, user_id, status=status, limit=limit)
    return {"ok": True, "schedule": [serialize_schedule(r) for r in rows]}


@rate_limit_user()
@router.patch("/schedule/{schedule_id}")
def update_learning_schedule_endpoint(
    request: Request,
    schedule_id: int,
    payload: LearningScheduleUpdate,
    db: Session = Depends(get_db),
):
    row = update_schedule(db, payload.user_id, schedule_id, **payload.model_dump(exclude={"user_id"}))
    if not row:
        raise HTTPException(status_code=404, detail="Schedule not found")
    return {"ok": True, "schedule": serialize_schedule(row)}


@rate_limit_user()
@router.delete("/schedule/{schedule_id}")
def delete_learning_schedule_endpoint(
    request: Request,
    schedule_id: int,
    user_id: str,
    db: Session = Depends(get_db),
):
    ok = delete_schedule(db, user_id, schedule_id)
    if not ok:
        raise HTTPException(status_code=404, detail="Schedule not found")
    return {"ok": True}
