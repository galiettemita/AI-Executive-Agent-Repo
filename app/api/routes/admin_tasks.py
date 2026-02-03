# backend/app/api/routes/admin_tasks.py

from __future__ import annotations

from datetime import datetime
from typing import Optional

from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel, Field
from sqlalchemy.orm import Session

from app.api.deps import get_db, get_or_create_user
from app.db.models import TaskItem
from app.services.daily_brief import generate_and_store_daily_brief

router = APIRouter(prefix="/admin/tasks", tags=["admin"])


class TaskCreate(BaseModel):
    user_id: str
    title: str = Field(..., min_length=1, max_length=300)
    due_at: Optional[datetime] = None


class TaskUpdate(BaseModel):
    completed: bool


@router.post("")
def create_task(payload: TaskCreate, db: Session = Depends(get_db)):
    get_or_create_user(db, payload.user_id)
    t = TaskItem(user_id=payload.user_id, title=payload.title, due_at=payload.due_at)
    db.add(t)
    db.commit()
    db.refresh(t)
    return {"id": t.id, "title": t.title, "due_at": t.due_at, "completed": t.completed}


@router.get("")
def list_tasks(user_id: str, include_completed: bool = False, db: Session = Depends(get_db)):
    get_or_create_user(db, user_id)
    q = db.query(TaskItem).filter(TaskItem.user_id == user_id)
    if not include_completed:
        q = q.filter(TaskItem.completed == False)  # noqa: E712
    tasks = q.order_by(TaskItem.due_at.is_(None), TaskItem.due_at.asc(), TaskItem.id.desc()).all()
    return {
        "tasks": [
            {"id": t.id, "title": t.title, "due_at": t.due_at, "completed": t.completed}
            for t in tasks
        ]
    }


@router.patch("/{task_id}")
def update_task(task_id: int, payload: TaskUpdate, user_id: str, db: Session = Depends(get_db)):
    get_or_create_user(db, user_id)
    t = db.get(TaskItem, task_id)
    if not t or t.user_id != user_id:
        raise HTTPException(status_code=404, detail="Task not found")
    t.completed = payload.completed
    if payload.completed:
        t.completed_at = datetime.utcnow()
    db.commit()
    return {"ok": True}


@router.post("/daily-brief")
def trigger_daily_brief(user_id: str, db: Session = Depends(get_db)):
    """
    Manually trigger a daily brief for a user.
    Fetches calendar events and emails, generates AI summary, and stores it.

    Args:
        user_id: User ID (query param)

    Returns:
        Daily brief result with text, events, emails, and storage info
    """
    get_or_create_user(db, user_id)

    result = generate_and_store_daily_brief(db=db, user_id=user_id)

    if not result.get("success"):
        raise HTTPException(
            status_code=500,
            detail=result.get("error", "Failed to generate daily brief")
        )

    return {
        "ok": True,
        "brief_text": result["brief_text"],
        "calendar_events_count": len(result.get("calendar_events", [])),
        "emails_count": len(result.get("emails", [])),
        "message_id": result.get("message_id"),
        "stored": result.get("stored", False),
    }