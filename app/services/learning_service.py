from __future__ import annotations

import json
from datetime import date, datetime, timedelta
from typing import Any, Dict, List, Optional

from sqlalchemy.orm import Session

from app.db.models import (
    LanguageGoal,
    LanguagePracticeSession,
    LearningResource,
    LearningSchedule,
)
from app.services.discover_provider import discover_search


def _dump_json(value: Optional[Any]) -> str:
    if value is None:
        return "{}"
    return json.dumps(value, ensure_ascii=False)


def _load_json(value: Optional[str], default: Any) -> Any:
    if not value:
        return default
    try:
        return json.loads(value)
    except Exception:
        return default


def _dump_tags(tags: Optional[List[str]]) -> str:
    if not tags:
        return "[]"
    cleaned = [t.strip() for t in tags if t and t.strip()]
    return json.dumps(cleaned, ensure_ascii=False)


def _load_tags(value: Optional[str]) -> List[str]:
    if not value:
        return []
    try:
        data = json.loads(value)
        if isinstance(data, list):
            return [str(item) for item in data]
    except Exception:
        return []
    return []


def serialize_language_goal(row: LanguageGoal) -> Dict[str, Any]:
    return {
        "id": row.id,
        "user_id": row.user_id,
        "language": row.language,
        "daily_minutes": row.daily_minutes,
        "weekly_sessions": row.weekly_sessions,
        "target_level": row.target_level,
        "active": row.active,
        "start_date": row.start_date.isoformat() if row.start_date else None,
        "end_date": row.end_date.isoformat() if row.end_date else None,
        "notes": row.notes,
        "created_at": row.created_at.isoformat() if row.created_at else None,
        "updated_at": row.updated_at.isoformat() if row.updated_at else None,
    }


def serialize_language_session(row: LanguagePracticeSession) -> Dict[str, Any]:
    return {
        "id": row.id,
        "user_id": row.user_id,
        "language": row.language,
        "session_type": row.session_type,
        "duration_minutes": row.duration_minutes,
        "accuracy_score": row.accuracy_score,
        "notes": row.notes,
        "metadata": _load_json(row.metadata_json, {}),
        "occurred_at": row.occurred_at.isoformat() if row.occurred_at else None,
        "created_at": row.created_at.isoformat() if row.created_at else None,
    }


def serialize_resource(row: LearningResource) -> Dict[str, Any]:
    return {
        "id": row.id,
        "user_id": row.user_id,
        "topic": row.topic,
        "title": row.title,
        "url": row.url,
        "source": row.source,
        "resource_type": row.resource_type,
        "difficulty": row.difficulty,
        "status": row.status,
        "tags": _load_tags(row.tags_json),
        "metadata": _load_json(row.metadata_json, {}),
        "notes": row.notes,
        "created_at": row.created_at.isoformat() if row.created_at else None,
        "updated_at": row.updated_at.isoformat() if row.updated_at else None,
    }


def serialize_schedule(row: LearningSchedule) -> Dict[str, Any]:
    return {
        "id": row.id,
        "user_id": row.user_id,
        "resource_id": row.resource_id,
        "scheduled_for": row.scheduled_for.isoformat() if row.scheduled_for else None,
        "duration_minutes": row.duration_minutes,
        "status": row.status,
        "notes": row.notes,
        "created_at": row.created_at.isoformat() if row.created_at else None,
        "updated_at": row.updated_at.isoformat() if row.updated_at else None,
    }


def create_language_goal(
    db: Session,
    user_id: str,
    language: str,
    daily_minutes: Optional[int] = None,
    weekly_sessions: Optional[int] = None,
    target_level: Optional[str] = None,
    active: Optional[bool] = True,
    start_date: Optional[date] = None,
    end_date: Optional[date] = None,
    notes: Optional[str] = None,
) -> LanguageGoal:
    row = (
        db.query(LanguageGoal)
        .filter(LanguageGoal.user_id == user_id, LanguageGoal.language == language)
        .one_or_none()
    )
    if row:
        row.daily_minutes = daily_minutes if daily_minutes is not None else row.daily_minutes
        row.weekly_sessions = weekly_sessions if weekly_sessions is not None else row.weekly_sessions
        row.target_level = target_level if target_level is not None else row.target_level
        row.active = active if active is not None else row.active
        row.start_date = start_date if start_date is not None else row.start_date
        row.end_date = end_date if end_date is not None else row.end_date
        row.notes = notes if notes is not None else row.notes
        row.updated_at = datetime.utcnow()
        db.commit()
        db.refresh(row)
        return row

    row = LanguageGoal(
        user_id=user_id,
        language=language,
        daily_minutes=daily_minutes or 15,
        weekly_sessions=weekly_sessions or 3,
        target_level=target_level,
        active=True if active is None else active,
        start_date=start_date,
        end_date=end_date,
        notes=notes,
        created_at=datetime.utcnow(),
        updated_at=datetime.utcnow(),
    )
    db.add(row)
    db.commit()
    db.refresh(row)
    return row


def update_language_goal(db: Session, user_id: str, goal_id: int, **fields: Any) -> Optional[LanguageGoal]:
    row = (
        db.query(LanguageGoal)
        .filter(LanguageGoal.user_id == user_id, LanguageGoal.id == goal_id)
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


def list_language_goals(db: Session, user_id: str, active: Optional[bool] = None) -> List[LanguageGoal]:
    q = db.query(LanguageGoal).filter(LanguageGoal.user_id == user_id)
    if active is not None:
        q = q.filter(LanguageGoal.active == active)
    return q.order_by(LanguageGoal.updated_at.desc()).all()


def delete_language_goal(db: Session, user_id: str, goal_id: int) -> bool:
    row = (
        db.query(LanguageGoal)
        .filter(LanguageGoal.user_id == user_id, LanguageGoal.id == goal_id)
        .one_or_none()
    )
    if not row:
        return False
    db.delete(row)
    db.commit()
    return True


def log_language_session(
    db: Session,
    user_id: str,
    language: str,
    session_type: Optional[str] = None,
    duration_minutes: Optional[int] = None,
    accuracy_score: Optional[float] = None,
    notes: Optional[str] = None,
    metadata: Optional[Dict[str, Any]] = None,
    occurred_at: Optional[datetime] = None,
) -> LanguagePracticeSession:
    row = LanguagePracticeSession(
        user_id=user_id,
        language=language,
        session_type=session_type,
        duration_minutes=duration_minutes,
        accuracy_score=accuracy_score,
        notes=notes,
        metadata_json=_dump_json(metadata),
        occurred_at=occurred_at or datetime.utcnow(),
        created_at=datetime.utcnow(),
    )
    db.add(row)
    db.commit()
    db.refresh(row)
    return row


def list_language_sessions(
    db: Session,
    user_id: str,
    language: Optional[str] = None,
    limit: int = 100,
) -> List[LanguagePracticeSession]:
    q = db.query(LanguagePracticeSession).filter(LanguagePracticeSession.user_id == user_id)
    if language:
        q = q.filter(LanguagePracticeSession.language == language)
    return q.order_by(LanguagePracticeSession.occurred_at.desc()).limit(limit).all()


def compute_language_progress(
    db: Session,
    user_id: str,
    language: Optional[str] = None,
    window_days: int = 30,
) -> Dict[str, Any]:
    since = datetime.utcnow() - timedelta(days=window_days)
    q = db.query(LanguagePracticeSession).filter(
        LanguagePracticeSession.user_id == user_id,
        LanguagePracticeSession.occurred_at >= since,
    )
    if language:
        q = q.filter(LanguagePracticeSession.language == language)

    sessions = q.order_by(LanguagePracticeSession.occurred_at.desc()).all()
    total_minutes = sum(s.duration_minutes or 0 for s in sessions)
    total_sessions = len(sessions)
    last_session = sessions[0].occurred_at if sessions else None

    date_set = {s.occurred_at.date() for s in sessions}
    streak = 0
    cursor = datetime.utcnow().date()
    while cursor in date_set:
        streak += 1
        cursor -= timedelta(days=1)

    return {
        "language": language,
        "window_days": window_days,
        "total_minutes": total_minutes,
        "total_sessions": total_sessions,
        "last_session_at": last_session.isoformat() if last_session else None,
        "streak_days": streak,
    }


def create_resource(
    db: Session,
    user_id: str,
    title: str,
    topic: Optional[str] = None,
    url: Optional[str] = None,
    source: Optional[str] = None,
    resource_type: Optional[str] = None,
    difficulty: Optional[str] = None,
    status: Optional[str] = None,
    tags: Optional[List[str]] = None,
    metadata: Optional[Dict[str, Any]] = None,
    notes: Optional[str] = None,
) -> LearningResource:
    row = LearningResource(
        user_id=user_id,
        topic=topic,
        title=title,
        url=url,
        source=source,
        resource_type=resource_type,
        difficulty=difficulty,
        status=status or "planned",
        tags_json=_dump_tags(tags),
        metadata_json=_dump_json(metadata),
        notes=notes,
        created_at=datetime.utcnow(),
        updated_at=datetime.utcnow(),
    )
    db.add(row)
    db.commit()
    db.refresh(row)
    return row


def update_resource(db: Session, user_id: str, resource_id: int, **fields: Any) -> Optional[LearningResource]:
    row = (
        db.query(LearningResource)
        .filter(LearningResource.user_id == user_id, LearningResource.id == resource_id)
        .one_or_none()
    )
    if not row:
        return None

    for key, value in fields.items():
        if value is None:
            continue
        if key == "tags":
            row.tags_json = _dump_tags(value)
            continue
        if key == "metadata":
            existing = _load_json(row.metadata_json, {})
            existing.update(value)
            row.metadata_json = _dump_json(existing)
            continue
        if hasattr(row, key):
            setattr(row, key, value)

    row.updated_at = datetime.utcnow()
    db.commit()
    db.refresh(row)
    return row


def list_resources(
    db: Session,
    user_id: str,
    status: Optional[str] = None,
    topic: Optional[str] = None,
    limit: int = 50,
) -> List[LearningResource]:
    q = db.query(LearningResource).filter(LearningResource.user_id == user_id)
    if status:
        q = q.filter(LearningResource.status == status)
    if topic:
        q = q.filter(LearningResource.topic == topic)
    return q.order_by(LearningResource.updated_at.desc()).limit(limit).all()


def delete_resource(db: Session, user_id: str, resource_id: int) -> bool:
    row = (
        db.query(LearningResource)
        .filter(LearningResource.user_id == user_id, LearningResource.id == resource_id)
        .one_or_none()
    )
    if not row:
        return False
    db.delete(row)
    db.commit()
    return True


def create_schedule(
    db: Session,
    user_id: str,
    resource_id: Optional[int] = None,
    scheduled_for: Optional[datetime] = None,
    duration_minutes: Optional[int] = None,
    status: Optional[str] = None,
    notes: Optional[str] = None,
) -> LearningSchedule:
    row = LearningSchedule(
        user_id=user_id,
        resource_id=resource_id,
        scheduled_for=scheduled_for,
        duration_minutes=duration_minutes,
        status=status or "scheduled",
        notes=notes,
        created_at=datetime.utcnow(),
        updated_at=datetime.utcnow(),
    )
    db.add(row)
    db.commit()
    db.refresh(row)
    return row


def update_schedule(db: Session, user_id: str, schedule_id: int, **fields: Any) -> Optional[LearningSchedule]:
    row = (
        db.query(LearningSchedule)
        .filter(LearningSchedule.user_id == user_id, LearningSchedule.id == schedule_id)
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


def list_schedule(db: Session, user_id: str, status: Optional[str] = None, limit: int = 50) -> List[LearningSchedule]:
    q = db.query(LearningSchedule).filter(LearningSchedule.user_id == user_id)
    if status:
        q = q.filter(LearningSchedule.status == status)
    return q.order_by(LearningSchedule.scheduled_for.desc()).limit(limit).all()


def delete_schedule(db: Session, user_id: str, schedule_id: int) -> bool:
    row = (
        db.query(LearningSchedule)
        .filter(LearningSchedule.user_id == user_id, LearningSchedule.id == schedule_id)
        .one_or_none()
    )
    if not row:
        return False
    db.delete(row)
    db.commit()
    return True


async def recommend_resources(
    db: Session,
    user_id: str,
    query: str,
    topic: Optional[str] = None,
    max_results: int = 6,
    save: bool = False,
) -> Dict[str, Any]:
    search_query = query.strip()
    if topic:
        search_query = f"{topic} {search_query}".strip()
    search_query = f"{search_query} learning resources".strip()

    results = await discover_search(search_query, max_results=max_results)
    payload = {
        "query": search_query,
        "results": [r.model_dump() for r in results],
    }

    created_ids: List[int] = []
    if save and results:
        for r in results:
            data = r.model_dump() if hasattr(r, "model_dump") else {}
            url = getattr(r, "url", None) or data.get("url")
            title = getattr(r, "title", None) or data.get("title") or "Resource"
            snippet = getattr(r, "snippet", None) or data.get("snippet")
            source = getattr(r, "source", None) or data.get("source")

            existing = None
            if url:
                existing = (
                    db.query(LearningResource)
                    .filter(LearningResource.user_id == user_id, LearningResource.url == url)
                    .one_or_none()
                )
            if existing:
                continue

            row = create_resource(
                db,
                user_id=user_id,
                title=title,
                topic=topic,
                url=url,
                source=source,
                resource_type="recommendation",
                metadata={"snippet": snippet},
            )
            created_ids.append(row.id)

    payload["saved_ids"] = created_ids
    return payload
