# backend/app/services/history.py

from __future__ import annotations

import os
from typing import Dict, List

from sqlalchemy.orm import Session
from sqlalchemy import func

from app.db.models import Conversation, ChatMessage


def _history_turns_limit() -> int:
    try:
        return max(1, int(os.getenv("HISTORY_TURNS", "6")))
    except Exception:
        return 6


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
    msgs = (
        db.query(ChatMessage)
        .filter(ChatMessage.user_id == user_id)
        .order_by(ChatMessage.id.desc())
        .limit(limit_msgs)
        .all()
    )
    msgs = list(reversed(msgs))
    return [{"role": m.role, "content": m.content} for m in msgs]


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
