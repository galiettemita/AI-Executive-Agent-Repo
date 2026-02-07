# backend/app/services/history.py

from __future__ import annotations

from typing import Dict, List

from sqlalchemy.orm import Session
from sqlalchemy import func

from app.db.models import Conversation, ChatMessage
from app.core.config import settings
from app.core.redis import cache_get_json, cache_set_json


def _history_turns_limit() -> int:
    return max(1, settings.HISTORY_TURNS)


def get_or_create_conversation(db: Session, user_id: str) -> Conversation:
    convo = (
        db.query(Conversation)
        .filter(Conversation.user_id == user_id)
        .order_by(Conversation.id.desc())
        .first()
    )
    if convo:
        return convo
    convo = Conversation(user_id=user_id)
    db.add(convo)
    db.commit()
    db.refresh(convo)
    return convo


def get_recent_history(db: Session, user_id: str) -> List[Dict[str, str]]:
    limit_turns = _history_turns_limit()
    limit_msgs = limit_turns * 2
    cache_key = f"history:{user_id}"
    cached = cache_get_json(cache_key)
    if isinstance(cached, list):
        return cached[-limit_msgs:]

    msgs = (
        db.query(ChatMessage)
        .filter(ChatMessage.user_id == user_id)
        .order_by(ChatMessage.id.desc())
        .limit(limit_msgs)
        .all()
    )
    msgs = list(reversed(msgs))
    payload = [{"role": m.role, "content": m.content} for m in msgs]
    cache_set_json(cache_key, payload, ttl_seconds=settings.REDIS_SESSION_TTL_SECONDS)
    return payload


def store_message(db: Session, user_id: str, conversation_id: int, role: str, content: str) -> None:
    db.add(
        ChatMessage(
            conversation_id=conversation_id,
            user_id=user_id,
            role=role,
            content=content,
        )
    )
    db.commit()
    cache_key = f"history:{user_id}"
    cached = cache_get_json(cache_key)
    if not isinstance(cached, list):
        cached = []
    cached.append({"role": role, "content": content})
    limit_msgs = _history_turns_limit() * 2
    if len(cached) > limit_msgs:
        cached = cached[-limit_msgs:]
    cache_set_json(cache_key, cached, ttl_seconds=settings.REDIS_SESSION_TTL_SECONDS)


def trim_history(db: Session, user_id: str) -> None:
    limit_turns = _history_turns_limit()
    limit_msgs = limit_turns * 2
    total = db.query(func.count(ChatMessage.id)).filter(ChatMessage.user_id == user_id).scalar() or 0
    if total <= limit_msgs:
        return

    # Delete oldest messages beyond the limit
    to_delete = total - limit_msgs
    old_ids = (
        db.query(ChatMessage.id)
        .filter(ChatMessage.user_id == user_id)
        .order_by(ChatMessage.id.asc())
        .limit(to_delete)
        .all()
    )
    if not old_ids:
        return
    id_list = [row[0] for row in old_ids]
    db.query(ChatMessage).filter(ChatMessage.id.in_(id_list)).delete(synchronize_session=False)
    db.commit()
    cache_key = f"history:{user_id}"
    cached = cache_get_json(cache_key)
    if isinstance(cached, list):
        cached = cached[-limit_msgs:]
        cache_set_json(cache_key, cached, ttl_seconds=settings.REDIS_SESSION_TTL_SECONDS)
